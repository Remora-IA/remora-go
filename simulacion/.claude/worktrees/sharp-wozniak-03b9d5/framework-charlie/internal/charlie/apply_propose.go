package charlie

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyProposePlan is the outcome of `charlie apply-propose [--push]`. It
// closes the long-standing gap between `propose` (proposes but does not
// commit) and the contract rule "Charlie does not execute manual git".
// The contract stays intact: every git call here goes through
// runGitControlled, respects rejectDangerousGit, and emits an audit entry.
type ApplyProposePlan struct {
	Mode          string   // "plan" or "apply"
	Version       string
	CommitMessage string
	Changelog     string
	StagedFiles   []string
	SkippedFiles  []string // files excluded because of ShouldIgnore/.charlieignore
	Blockers      []string
	Warnings      []string
	Applied       bool
	Pushed        bool
	HeadBefore    string
	HeadAfter     string
	TagCreated    string
}

// BuildApplyProposePlan computes what would happen, without mutating anything.
func BuildApplyProposePlan() (*ApplyProposePlan, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}
	plan := &ApplyProposePlan{Mode: "plan"}

	// Pre-flight through doctor. If the repo is structurally broken, bail
	// with a clear code instead of letting git errors cascade.
	if blockers := PreflightHealthBlockers(); len(blockers) > 0 {
		plan.Blockers = append(plan.Blockers, blockers...)
		plan.Blockers = append(plan.Blockers, "ejecuta 'charlie doctor --apply' antes de apply-propose")
	}

	report, err := BuildReport()
	if err != nil {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("no se pudo construir Report: %v", err))
		return plan, nil
	}
	if len(report.Changes) == 0 {
		plan.Warnings = append(plan.Warnings, "repo limpio; nada que commitear")
		return plan, nil
	}

	plan.Version = report.NextVersion
	plan.CommitMessage = report.CommitMessage
	plan.Changelog = report.Changelog

	// Partition changes: staged vs skipped. ClassifyChanges already filters
	// ShouldIgnore but we double check (and untracked directories may have
	// leaked in older versions).
	seen := map[string]bool{}
	for _, c := range report.Changes {
		if seen[c.FilePath] {
			continue
		}
		seen[c.FilePath] = true
		if ShouldIgnore(c.FilePath) {
			plan.SkippedFiles = append(plan.SkippedFiles, c.FilePath)
			continue
		}
		plan.StagedFiles = append(plan.StagedFiles, c.FilePath)
	}
	sort.Strings(plan.StagedFiles)
	sort.Strings(plan.SkippedFiles)

	// Block the rest of the flow if validate() would reject the commit.
	if issues := ValidateReport(report); len(issues) > 0 {
		plan.Blockers = append(plan.Blockers, issues...)
	}

	return plan, nil
}

// ApplyApplyPropose executes the plan: writes CHANGELOG.md, stages files,
// commits, creates the tag. When push=true, also invokes publish-draft
// semantics for the push (respecting force-with-lease).
func ApplyApplyPropose(push bool) (*ApplyProposePlan, error) {
	plan, err := BuildApplyProposePlan()
	if err != nil {
		return nil, err
	}
	if len(plan.Blockers) > 0 {
		return plan, nil
	}
	plan.Mode = "apply"

	// Snapshot HEAD for audit.
	if sha, err := HeadCommit(); err == nil {
		plan.HeadBefore = sha
	}

	// Always back up before mutating.
	if _, err := BackupWorkingTree(); err != nil {
		return plan, fmt.Errorf("backup fallo: %v", err)
	}

	// Write (prepend) CHANGELOG.md entry for this release if not already there.
	if err := ensureChangelogEntry(plan.Version, plan.Changelog); err != nil {
		return plan, fmt.Errorf("no pude actualizar CHANGELOG.md: %v", err)
	}

	// Stage files explicitly to honor SkippedFiles. We use `git add --` with
	// the concrete list so untracked binaries that slipped past .gitignore
	// still get excluded when they match ShouldIgnore.
	//
	// Also always stage CHANGELOG.md (it may be new in this release).
	toStage := append([]string{}, plan.StagedFiles...)
	toStage = append(toStage, "CHANGELOG.md")
	args := append([]string{"add", "--"}, dedup(toStage)...)
	if _, err := runGitControlled(args...); err != nil {
		return plan, fmt.Errorf("git add fallo: %v", err)
	}

	if _, err := runGitControlled("commit", "-m", plan.CommitMessage); err != nil {
		return plan, fmt.Errorf("git commit fallo: %v", err)
	}
	plan.Applied = true

	if _, err := runGitControlled("tag", plan.Version); err != nil {
		// Tag already exists -> this is a release rewrite, not a linear bump.
		// Apply-propose stays conservative: surfaces a warning and recommends
		// repair-release vVERSION --apply, which already has the canonical path.
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("tag %s ya existia; usa repair-release para reescribir", plan.Version))
	} else {
		plan.TagCreated = plan.Version
	}

	if sha, err := HeadCommit(); err == nil {
		plan.HeadAfter = sha
	}

	appendAudit("apply-propose", map[string]string{
		"version":     plan.Version,
		"message":     plan.CommitMessage,
		"head_before": plan.HeadBefore,
		"head_after":  plan.HeadAfter,
		"files":       fmt.Sprintf("%d", len(plan.StagedFiles)),
		"skipped":     fmt.Sprintf("%d", len(plan.SkippedFiles)),
	})

	if push {
		// Delegate to publish-draft for strategy (push / force-with-lease).
		publish, perr := ApplyPublishDraft()
		if perr != nil {
			return plan, fmt.Errorf("publish-draft fallo: %v", perr)
		}
		if len(publish.Blockers) == 0 {
			plan.Pushed = true
			// Also push the tag. publish-draft does not push tags.
			if plan.TagCreated != "" {
				if _, err := runGitControlled("push", "origin", plan.TagCreated); err != nil {
					plan.Warnings = append(plan.Warnings, fmt.Sprintf("no pude pushear tag %s: %v", plan.TagCreated, err))
				}
			}
		} else {
			plan.Warnings = append(plan.Warnings, publish.Blockers...)
		}
		appendAudit("apply-propose.push", map[string]string{
			"version": plan.Version,
			"pushed":  fmt.Sprintf("%v", plan.Pushed),
		})
	}

	return plan, nil
}

// ensureChangelogEntry prepends the generated section to CHANGELOG.md (after
// the keep-a-changelog header). Idempotent: if a section for the version
// already exists, it merges bullets via appendGeneratedBulletsToRelease.
func ensureChangelogEntry(version string, generated string) error {
	path := filepath.Join(RepoRoot, "CHANGELOG.md")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	current := string(existing)

	if strings.Contains(current, versionHeader(version)) {
		merged := appendGeneratedBulletsToRelease(current, version, generated)
		return os.WriteFile(path, []byte(merged), 0o644)
	}

	var out string
	if current == "" {
		out = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\nThe format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),\nand this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n\n" + strings.TrimRight(generated, "\n") + "\n"
	} else {
		out = prependSection(current, generated)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func prependSection(existing string, generated string) string {
	// Insert after the keep-a-changelog header (first blank line after
	// "semver" line), or after the first blank line if not found.
	lines := strings.Split(existing, "\n")
	insertAt := -1
	for i, l := range lines {
		if strings.Contains(l, "semver.org") {
			// Look for the next blank line after this.
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					insertAt = j + 1
					break
				}
			}
			break
		}
	}
	if insertAt == -1 {
		insertAt = 0
	}
	head := strings.Join(lines[:insertAt], "\n")
	tail := strings.Join(lines[insertAt:], "\n")
	gen := strings.TrimRight(generated, "\n")
	if head != "" {
		head += "\n"
	}
	return head + gen + "\n\n" + tail
}

func versionHeader(version string) string {
	// "## [0.1.7]"
	v := strings.TrimPrefix(version, "v")
	return "## [" + v + "]"
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ----------------------------------------------------------------------------
// Format
// ----------------------------------------------------------------------------

func FormatApplyProposePlan(plan *ApplyProposePlan) string {
	var b strings.Builder
	b.WriteString("=== CHARLIE APPLY-PROPOSE ===\n\n")
	fmt.Fprintf(&b, "modo: %s\n", plan.Mode)
	fmt.Fprintf(&b, "version: %s\n", valueOrDash(plan.Version))
	fmt.Fprintf(&b, "commit: %s\n", valueOrDash(plan.CommitMessage))
	fmt.Fprintf(&b, "archivos a stagear: %d\n", len(plan.StagedFiles))
	if len(plan.SkippedFiles) > 0 {
		fmt.Fprintf(&b, "archivos ignorados (.charlieignore/ShouldIgnore): %d\n", len(plan.SkippedFiles))
	}
	if plan.Applied {
		fmt.Fprintf(&b, "\ncommit aplicado: %s -> %s\n", safeSha(plan.HeadBefore), safeSha(plan.HeadAfter))
		if plan.TagCreated != "" {
			fmt.Fprintf(&b, "tag creado: %s\n", plan.TagCreated)
		}
		if plan.Pushed {
			b.WriteString("push a origin/draft: OK\n")
		}
	}
	if len(plan.Warnings) > 0 {
		b.WriteString("\nwarnings:\n")
		for _, w := range plan.Warnings {
			fmt.Fprintf(&b, "  - %s\n", w)
		}
	}
	if len(plan.Blockers) > 0 {
		b.WriteString("\nBLOQUEADO:\n")
		for _, bl := range plan.Blockers {
			fmt.Fprintf(&b, "  - %s\n", bl)
		}
		b.WriteString("\nCharlie no avanza con apply-propose hasta que se resuelvan los bloqueos.\n")
	}
	if !plan.Applied && len(plan.Blockers) == 0 {
		b.WriteString("\nsiguiente: go run ./cmd/charlie apply-propose --apply [--push]\n")
	}
	return b.String()
}

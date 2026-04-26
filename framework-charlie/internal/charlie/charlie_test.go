package charlie

import (
	"strings"
	"testing"
)

func TestNextVersionIncrementsLastSegment(t *testing.T) {
	cases := map[string]string{
		"v0.1.0":   "v0.1.1",
		"v0.1.1.1": "v0.1.1.2",
		"0.9.9":    "v0.9.10",
		"no-tags":  "v0.0.1",
	}

	for input, want := range cases {
		if got := NextVersion(input); got != want {
			t.Fatalf("NextVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGenerateCommitMessageUsesOnlyCharlieFormat(t *testing.T) {
	changes := []Change{
		{FilePath: "framework-paladin/paladin/context.go", Type: TypeFeat},
		{FilePath: "framework-quine/INITIAL_PROMPT.md", Type: TypeDocs},
	}

	got := GenerateCommitMessage(changes, "v0.1.4")
	if !strings.HasPrefix(got, "chore: commit v0.1.4 - ") {
		t.Fatalf("commit message = %q", got)
	}
	if strings.HasPrefix(got, "feat(") || strings.HasPrefix(got, "docs:") {
		t.Fatalf("commit message must not use conventional per-change format: %q", got)
	}
}

func TestRejectDangerousGit(t *testing.T) {
	if err := rejectDangerousGit("reset", "--hard", "HEAD"); err == nil {
		t.Fatal("expected git reset --hard to be blocked")
	}
	if err := rejectDangerousGit("push", "--force"); err == nil {
		t.Fatal("expected git push --force to be blocked")
	}
	if err := rejectDangerousGit("status", "--porcelain"); err != nil {
		t.Fatalf("safe command rejected: %v", err)
	}
}

func TestShouldIgnoreGeneratedArtifacts(t *testing.T) {
	ignored := []string{
		".DS_Store",
		"framework-charlie/charlie",
		"framework-quine/temp/paladin/trace.json",
		"framework-paladin/bin/paladin",
		"framework-quine/cmd/quine/main.go.bak",
	}
	for _, path := range ignored {
		if !ShouldIgnore(path) {
			t.Fatalf("expected %s to be ignored", path)
		}
	}

	if ShouldIgnore("framework-charlie/internal/charlie/charlie.go") {
		t.Fatal("source file should not be ignored")
	}
}

func TestBackupSkipsGitAndGeneratedArtifacts(t *testing.T) {
	cases := []string{
		".git",
		".git/objects/abc",
		"framework-charlie/charlie",
		"framework-foco/temp/foco/today.json",
		"framework-paladin/bin/server",
	}
	for _, path := range cases {
		if !shouldSkipBackup(path, nil) {
			t.Fatalf("expected backup to skip %s", path)
		}
	}

	if shouldSkipBackup("framework-foco/cmd/foco/main.go", nil) {
		t.Fatal("backup should keep source files")
	}
}

func TestFormatAmendPlanBlocksUnsafeReleaseRewrite(t *testing.T) {
	plan := &AmendPlan{
		Version:   "v0.1.4",
		Branch:    "main",
		Head:      "abc123",
		TagCommit: "def456",
		Behind:    1,
		Blockers: []string{
			`branch actual "main"; cambiar a draft fuera de Charlie`,
			"HEAD abc123 no coincide con v0.1.4 def456; solo se puede amendar la release si el tag apunta al HEAD",
		},
	}

	got := FormatAmendPlan(plan)
	for _, want := range []string{"estado: BLOQUEADO", "No amendar la release", "branch actual"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatAmendPlan missing %q in:\n%s", want, got)
		}
	}
}

func TestSameReleaseCommitMessage(t *testing.T) {
	a := "chore: commit v0.1.4 - expandir charlie, foco"
	b := "chore: commit v0.1.4 - expandir charlie, echo"
	if !sameReleaseCommitMessage(a, b) {
		t.Fatal("expected same release version to be detected")
	}
	if sameReleaseCommitMessage(a, "chore: commit v0.1.5 - expandir charlie") {
		t.Fatal("different release versions must not match")
	}
}

func TestCommitMessageVersion(t *testing.T) {
	got := commitMessageVersion("chore: commit v0.1.4 - expandir charlie")
	if got != "v0.1.4" {
		t.Fatalf("commitMessageVersion = %q", got)
	}
	if commitMessageVersion("feat: other") != "" {
		t.Fatal("non Charlie commit should not return a version")
	}
}

func TestAppendGeneratedBulletsToRelease(t *testing.T) {
	existing := "# Changelog\n\n## [0.1.4] - 2026-04-26\n\n### Charlie\n\n- old\n\n---\n\n## [0.1.3] - 2026-04-25\n"
	generated := "## [0.1.4] - 2026-04-26\n\n### Charlie\n\n- **repair-release**: comando nuevo\n"
	got := appendGeneratedBulletsToRelease(existing, "v0.1.4", generated)
	if !strings.Contains(got, "- **repair-release**: comando nuevo") {
		t.Fatalf("generated bullet was not merged:\n%s", got)
	}
	if !strings.Contains(got, "## [0.1.3]") {
		t.Fatalf("next release section was lost:\n%s", got)
	}
}

func TestFormatRepairReleasePlanShowsApplyCommand(t *testing.T) {
	plan := &RepairReleasePlan{
		Version:       "v0.1.4",
		Mode:          "plan",
		Branch:        "draft",
		BaseRef:       "origin/draft",
		BaseCommit:    "e4cf1b5",
		Head:          "29f44c2",
		CommitMessage: "chore: commit v0.1.4 - expandir charlie",
		Changes:       []Change{{FilePath: "framework-charlie/README.md"}},
	}
	got := FormatRepairReleasePlan(plan)
	if !strings.Contains(got, "repair-release v0.1.4 --apply") {
		t.Fatalf("apply command missing:\n%s", got)
	}
}

func TestFormatPublishDraftPlanShowsApplyCommand(t *testing.T) {
	plan := &PublishDraftPlan{
		Mode:          "plan",
		Branch:        "draft",
		Head:          "abc123",
		Upstream:      "def456",
		Version:       "v0.1.4",
		CommitMessage: "chore: commit v0.1.4 - expandir charlie",
		Ahead:         1,
		Behind:        1,
		Strategy:      "force-with-lease",
	}
	got := FormatPublishDraftPlan(plan)
	if !strings.Contains(got, "publish-draft --apply") {
		t.Fatalf("publish apply command missing:\n%s", got)
	}
}

func TestFormatPublishTagPlanShowsApplyCommand(t *testing.T) {
	plan := &PublishTagPlan{
		Mode:      "plan",
		Version:   "v0.1.4",
		LocalTag:  "local",
		RemoteTag: "remote",
		Strategy:  "force-with-lease-tag",
	}
	got := FormatPublishTagPlan(plan)
	if !strings.Contains(got, "publish-tag v0.1.4 --apply") {
		t.Fatalf("publish tag apply command missing:\n%s", got)
	}
}

func TestFormatPublishMainPlanShowsApplyCommand(t *testing.T) {
	plan := &PublishMainPlan{
		Mode:          "plan",
		Branch:        "draft",
		Head:          "abc123",
		DraftRemote:   "abc123",
		MainLocal:     "def456",
		MainRemote:    "def456",
		Version:       "v0.1.4",
		CommitMessage: "chore: commit v0.1.4 - expandir charlie",
		Strategy:      "force-with-lease-main",
		TagStrategy:   "noop",
	}
	got := FormatPublishMainPlan(plan)
	if !strings.Contains(got, "publish-main --apply") {
		t.Fatalf("publish main apply command missing:\n%s", got)
	}
}

func TestFormatReconcilePlanBlocksDivergedReleaseWithoutAskingOptions(t *testing.T) {
	plan := &ReconcilePlan{
		State:         "DIVERGENCIA_RELEASE",
		Decision:      "dos commits distintos declaran la misma release; conservar ambos y requerir reparacion de release controlada",
		Branch:        "draft",
		Ahead:         1,
		Behind:        1,
		Head:          "29f44c2",
		Upstream:      "e4cf1b5",
		HeadMessage:   "chore: commit v0.1.4 - expandir charlie, foco",
		RemoteMessage: "chore: commit v0.1.4 - expandir charlie, echo",
		HeadTags:      []string{"v0.1.4"},
		Blockers:      []string{"draft y upstream tienen commits distintos; Charlie no debe escoger force push ni merge manual"},
	}

	got := FormatReconcilePlan(plan)
	for _, want := range []string{"DIVERGENCIA_RELEASE", "No preguntes A/B", "no debe escoger force push"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatReconcilePlan missing %q in:\n%s", want, got)
		}
	}
}

func TestValidateReportRequiresChangelogPerFile(t *testing.T) {
	report := &Report{
		Changes:       []Change{{FilePath: "framework-charlie/internal/charlie/charlie.go", Type: TypeFeat}},
		NextVersion:   "v0.1.4",
		CommitMessage: "chore: commit v0.1.4 - expandir charlie",
		Changelog:     "## [0.1.4]\n\n### Cambios por archivo\n\n- `framework-charlie/internal/charlie/charlie.go`: cambio",
	}
	if issues := ValidateReport(report); len(issues) != 0 {
		t.Fatalf("unexpected validation issues: %v", issues)
	}

	report.Changelog = "## [0.1.4]\n\nsin detalle"
	if issues := ValidateReport(report); len(issues) == 0 {
		t.Fatal("expected incomplete changelog to fail")
	}
}

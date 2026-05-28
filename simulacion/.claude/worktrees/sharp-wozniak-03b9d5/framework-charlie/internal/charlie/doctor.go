package charlie

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Doctor: diagnoses and repairs repo-level corruption that would make the rest
// of Charlie crash with raw git errors. Added in v0.1.8 after the v0.1.6
// incident where `git gc` purged reachable-but-recent objects and every Charlie
// command failed with "fatal: bad object HEAD".
//
// Design principles:
//   - Inspect is read-only; never mutates the repo.
//   - Diagnose classifies the state into Charlie-level codes.
//   - Recipes are ordered by safety (level 1 = read-only, 5 = destructive).
//   - Apply always creates a backup before running a recipe of level >= 3.

// DoctorSeverity grades the health findings.
type DoctorSeverity string

const (
	SeverityOK       DoctorSeverity = "ok"
	SeverityInfo     DoctorSeverity = "info"
	SeverityWarning  DoctorSeverity = "warning"
	SeverityBlocker  DoctorSeverity = "blocker"
	SeverityCritical DoctorSeverity = "critical"
)

// Diagnosis codes are stable identifiers that recipes and the intent router
// can pattern-match against.
const (
	CodeRepoOK                     = "REPO_OK"
	CodeMissingObject              = "REPO_CORRUPT_MISSING_OBJECT"
	CodeDetachedHead               = "REPO_DETACHED_HEAD"
	CodeUpstreamDiverged           = "UPSTREAM_DIVERGED"
	CodeUpstreamUnconfigured       = "UPSTREAM_UNCONFIGURED"
	CodeUnresolvedMerge            = "MERGE_UNRESOLVED"
	CodeGcAggressive               = "GC_AGGRESSIVE_CONFIG"
	CodeGcLogPresent               = "GC_LOG_PRESENT"
	CodeNotDraft                   = "BRANCH_NOT_DRAFT"
	CodeDanglingReflogOnlyRecovery = "RECOVERY_VIA_REFLOG_ONLY"
)

// RepoState is a point-in-time snapshot of the repo collected by Inspect.
// Every field is raw data - no policy, no classification.
type RepoState struct {
	RepoRoot         string
	Branch           string            // empty if detached
	HeadRef          string            // e.g. "refs/heads/draft"
	HeadSHA          string            // output of git rev-parse HEAD, even if object is missing
	HeadObjectExists bool              // whether .git has the object HeadSHA points to
	LooseRefs        map[string]string // ref -> sha (only refs/heads/* and refs/remotes/origin/*)
	ReflogTail       []string          // last 10 reflog entries for current branch
	UpstreamRef      string            // e.g. "origin/draft"
	UpstreamSHA      string            // empty if no upstream
	UpstreamAhead    int
	UpstreamBehind   int
	HasUnmerged      bool
	FsckErrors       []string
	GcLogPath        string // ".git/gc.log" if present
	GcAuto           string // value of config gc.auto; empty means default
	RemoteURL        string // origin fetch url
}

// Diagnosis is a single health finding. A Doctor run returns N diagnoses.
type Diagnosis struct {
	Code      string
	Severity  DoctorSeverity
	Summary   string
	Evidence  []string
	RecipeIDs []string // recipe ids that can remediate this diagnosis, ordered by safety
}

// Recipe is a deterministic, idempotent recovery procedure. Recipes never
// bypass rejectDangerousGit: every git call goes through runGitControlled so
// that the audit log and Charlie's policy stays intact.
type Recipe struct {
	ID             string
	Description    string
	SafetyLevel    int // 1 = read-only, 2 = reversible config, 3 = fetch/backup, 4 = ref rewrite, 5 = destructive
	RequiresBackup bool
	Run            func(state *RepoState) (log []string, newState *RepoState, err error)
}

// DoctorReport is what the CLI prints.
type DoctorReport struct {
	State          *RepoState
	Diagnoses      []Diagnosis
	Applied        []string // recipe ids that were applied (only when --apply)
	AppliedLog     [][]string
	PostState      *RepoState // state after applying recipes (nil if no apply)
	OverallHealth  DoctorSeverity
	BackupPath     string
	Recommendation string
}

// ----------------------------------------------------------------------------
// Inspect
// ----------------------------------------------------------------------------

// Inspect collects repo state without mutating anything. It uses direct exec
// (not runGit) because runGit itself can fail on a corrupt repo and we need
// partial success here.
func Inspect() (*RepoState, error) {
	state := &RepoState{
		RepoRoot:  RepoRoot,
		LooseRefs: map[string]string{},
	}

	// HeadRef: read .git/HEAD directly. Works even when HEAD object is missing.
	if head, err := os.ReadFile(filepath.Join(RepoRoot, ".git", "HEAD")); err == nil {
		raw := strings.TrimSpace(string(head))
		if strings.HasPrefix(raw, "ref: ") {
			state.HeadRef = strings.TrimPrefix(raw, "ref: ")
			state.Branch = strings.TrimPrefix(state.HeadRef, "refs/heads/")
		} else {
			// Detached HEAD pointing to a SHA
			state.HeadSHA = raw
		}
	}

	// Resolve HEAD SHA through the ref chain.
	if state.HeadSHA == "" && state.HeadRef != "" {
		if sha, err := readRefSHA(state.HeadRef); err == nil {
			state.HeadSHA = sha
		}
	}

	// Verify HEAD object exists in the object store.
	if state.HeadSHA != "" {
		state.HeadObjectExists = gitObjectExists(state.HeadSHA)
	}

	// Loose refs: heads/<branch> and remotes/origin/*.
	collectLooseRefs(state.LooseRefs, filepath.Join(RepoRoot, ".git", "refs", "heads"), "refs/heads")
	collectLooseRefs(state.LooseRefs, filepath.Join(RepoRoot, ".git", "refs", "remotes", "origin"), "refs/remotes/origin")

	// Reflog tail (safe: reads a file).
	if state.Branch != "" {
		if data, err := os.ReadFile(filepath.Join(RepoRoot, ".git", "logs", "refs", "heads", state.Branch)); err == nil {
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			start := 0
			if len(lines) > 10 {
				start = len(lines) - 10
			}
			state.ReflogTail = lines[start:]
		}
	}

	// Upstream. These calls might fail if HEAD is corrupt; treat as "unknown".
	if out, err := execGit("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		state.UpstreamRef = strings.TrimSpace(out)
	}
	if state.UpstreamRef != "" {
		if out, err := execGit("rev-parse", state.UpstreamRef); err == nil {
			state.UpstreamSHA = strings.TrimSpace(out)
		}
		if out, err := execGit("rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
			fields := strings.Fields(strings.TrimSpace(out))
			if len(fields) == 2 {
				fmt.Sscanf(fields[0], "%d", &state.UpstreamBehind)
				fmt.Sscanf(fields[1], "%d", &state.UpstreamAhead)
			}
		}
	}

	// Unmerged paths.
	if out, err := execGit("status", "--porcelain"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if len(line) >= 2 && (line[0] == 'U' || line[1] == 'U' || (line[0] == 'A' && line[1] == 'A') || (line[0] == 'D' && line[1] == 'D')) {
				state.HasUnmerged = true
				break
			}
		}
	}

	// Fsck - we only flag errors, not warnings/dangling. Cheap-ish call.
	if out, err := execGit("fsck", "--connectivity-only", "--no-dangling", "--no-reflogs"); err != nil || out != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if line == "" {
				continue
			}
			state.FsckErrors = append(state.FsckErrors, line)
		}
	}

	// gc artifacts.
	if _, err := os.Stat(filepath.Join(RepoRoot, ".git", "gc.log")); err == nil {
		state.GcLogPath = ".git/gc.log"
	}
	if out, err := execGit("config", "--get", "gc.auto"); err == nil {
		state.GcAuto = strings.TrimSpace(out)
	}

	// Remote url.
	if out, err := execGit("config", "--get", "remote.origin.url"); err == nil {
		state.RemoteURL = strings.TrimSpace(out)
	}

	return state, nil
}

func readRefSHA(ref string) (string, error) {
	// Try loose ref first.
	loose := filepath.Join(RepoRoot, ".git", ref)
	if data, err := os.ReadFile(loose); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	// Fall back to packed-refs.
	packed, err := os.ReadFile(filepath.Join(RepoRoot, ".git", "packed-refs"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(packed), "\n") {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == ref {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("ref %s not found", ref)
}

func collectLooseRefs(dst map[string]string, dir string, prefix string) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		dst[prefix+"/"+filepath.ToSlash(rel)] = strings.TrimSpace(string(data))
		return nil
	})
}

func gitObjectExists(sha string) bool {
	cmd := exec.Command("git", "cat-file", "-e", sha)
	cmd.Dir = RepoRoot
	return cmd.Run() == nil
}

// execGit is a bare exec without rejectDangerousGit. Only used for read-only
// inspection. All write operations in recipes must go through runGitControlled.
func execGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = RepoRoot
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ----------------------------------------------------------------------------
// Diagnose
// ----------------------------------------------------------------------------

// Diagnose classifies a RepoState into a list of findings. An empty or
// REPO_OK-only result means the repo is healthy for Charlie to operate.
func Diagnose(state *RepoState) []Diagnosis {
	var out []Diagnosis

	// Most urgent: HEAD points to an object that doesn't exist.
	if state.HeadSHA != "" && !state.HeadObjectExists {
		d := Diagnosis{
			Code:     CodeMissingObject,
			Severity: SeverityCritical,
			Summary:  fmt.Sprintf("HEAD (%s) apunta a un objeto que no existe en el object store", shortSHA(state.HeadSHA)),
			Evidence: []string{
				"git cat-file -e " + state.HeadSHA + " => exit 1",
			},
			RecipeIDs: []string{"fetch-missing-objects", "rewind-head-to-reflog"},
		}
		if state.GcLogPath != "" {
			d.Evidence = append(d.Evidence, "detectado .git/gc.log: git gc reciente es sospechoso de haber purgado el objeto")
		}
		if state.RemoteURL != "" {
			d.Evidence = append(d.Evidence, "remote origin configurado: "+state.RemoteURL+" => fetch puede recuperar")
		}
		out = append(out, d)
	}

	// Detached HEAD.
	if state.Branch == "" && state.HeadSHA != "" {
		out = append(out, Diagnosis{
			Code:      CodeDetachedHead,
			Severity:  SeverityBlocker,
			Summary:   "HEAD detached: Charlie requiere estar en draft",
			Evidence:  []string{"HEAD apunta directamente a " + shortSHA(state.HeadSHA)},
			RecipeIDs: []string{"reattach-to-draft"},
		})
	}

	// Branch is not draft (but attached).
	if state.Branch != "" && state.Branch != "draft" {
		out = append(out, Diagnosis{
			Code:     CodeNotDraft,
			Severity: SeverityBlocker,
			Summary:  fmt.Sprintf("branch actual %q; Charlie solo versiona en draft", state.Branch),
			Evidence: []string{"HEAD ref: " + state.HeadRef},
		})
	}

	// Upstream divergence.
	if state.UpstreamSHA != "" && state.UpstreamBehind > 0 && state.UpstreamAhead > 0 {
		out = append(out, Diagnosis{
			Code:      CodeUpstreamDiverged,
			Severity:  SeverityBlocker,
			Summary:   fmt.Sprintf("draft ha divergido del upstream: ahead=%d behind=%d", state.UpstreamAhead, state.UpstreamBehind),
			Evidence:  []string{"HEAD=" + shortSHA(state.HeadSHA), "upstream=" + shortSHA(state.UpstreamSHA)},
			RecipeIDs: []string{"suggest-reconcile-draft"},
		})
	} else if state.UpstreamSHA == "" && state.HeadObjectExists {
		out = append(out, Diagnosis{
			Code:     CodeUpstreamUnconfigured,
			Severity: SeverityWarning,
			Summary:  "no hay upstream configurado para este branch",
			Evidence: []string{"ref HEAD: " + state.HeadRef},
		})
	}

	// Merge in progress.
	if state.HasUnmerged {
		out = append(out, Diagnosis{
			Code:     CodeUnresolvedMerge,
			Severity: SeverityBlocker,
			Summary:  "hay archivos con conflictos sin resolver",
			Evidence: []string{"git status --porcelain detecta UU/AA/DD/UX/XU"},
		})
	}

	// gc aggressive.
	if state.GcAuto != "" && state.GcAuto != "0" {
		out = append(out, Diagnosis{
			Code:      CodeGcAggressive,
			Severity:  SeverityWarning,
			Summary:   "gc.auto != 0 puede purgar objetos recientes",
			Evidence:  []string{"gc.auto = " + state.GcAuto},
			RecipeIDs: []string{"disable-gc-auto"},
		})
	}
	if state.GcLogPath != "" {
		out = append(out, Diagnosis{
			Code:     CodeGcLogPresent,
			Severity: SeverityInfo,
			Summary:  "detectado .git/gc.log (historial de un git gc previo)",
			Evidence: []string{state.GcLogPath},
		})
	}

	// fsck errors (not dangling, not warning).
	if len(state.FsckErrors) > 0 {
		// Filter out benign "dangling" mentions just in case.
		var real []string
		for _, l := range state.FsckErrors {
			if strings.HasPrefix(strings.TrimSpace(l), "dangling") {
				continue
			}
			real = append(real, l)
		}
		if len(real) > 0 {
			out = append(out, Diagnosis{
				Code:     "FSCK_ERROR",
				Severity: SeverityBlocker,
				Summary:  "git fsck reporta errores de integridad",
				Evidence: real,
			})
		}
	}

	if len(out) == 0 {
		out = append(out, Diagnosis{
			Code:     CodeRepoOK,
			Severity: SeverityOK,
			Summary:  "repo saludable: HEAD existe, sin divergencia destructiva, sin conflictos",
		})
	}
	return out
}

// ----------------------------------------------------------------------------
// Recipes
// ----------------------------------------------------------------------------

// RegisteredRecipes returns the recipe catalog. Adding a recipe here makes it
// automatically addressable from Diagnose by ID.
func RegisteredRecipes() map[string]Recipe {
	return map[string]Recipe{
		"fetch-missing-objects": {
			ID:             "fetch-missing-objects",
			Description:    "git fetch origin para recuperar objetos faltantes desde el remoto",
			SafetyLevel:    3,
			RequiresBackup: false, // fetch is additive, no destructive
			Run: func(state *RepoState) ([]string, *RepoState, error) {
				var log []string
				out, err := runGitControlled("fetch", "origin", "--prune")
				log = append(log, "git fetch origin --prune", out)
				if err != nil {
					return log, state, fmt.Errorf("fetch fallo: %v", err)
				}
				post, perr := Inspect()
				if perr != nil {
					return log, state, perr
				}
				if post.HeadSHA != "" && !post.HeadObjectExists {
					return log, post, fmt.Errorf("el objeto %s sigue faltando tras fetch; el remoto no lo tiene", shortSHA(post.HeadSHA))
				}
				log = append(log, "HEAD object recuperado: "+shortSHA(post.HeadSHA))
				return log, post, nil
			},
		},
		"rewind-head-to-reflog": {
			ID:             "rewind-head-to-reflog",
			Description:    "retrocede HEAD al ultimo SHA del reflog cuyo objeto si existe (no destructivo: working tree intacto)",
			SafetyLevel:    4,
			RequiresBackup: true,
			Run: func(state *RepoState) ([]string, *RepoState, error) {
				var log []string
				if state.Branch == "" {
					return log, state, fmt.Errorf("no hay branch: no se puede retroceder HEAD")
				}
				target := findLastValidReflogSHA(state)
				if target == "" {
					return log, state, fmt.Errorf("no encontre en el reflog ningun SHA con objeto presente")
				}
				log = append(log, "target reflog SHA (con objeto presente): "+target)
				out, err := runGitControlled("update-ref", "refs/heads/"+state.Branch, target)
				log = append(log, fmt.Sprintf("git update-ref refs/heads/%s %s", state.Branch, target), out)
				if err != nil {
					return log, state, err
				}
				post, _ := Inspect()
				return log, post, nil
			},
		},
		"disable-gc-auto": {
			ID:             "disable-gc-auto",
			Description:    "git config gc.auto 0 para evitar que gc purgue objetos recientes",
			SafetyLevel:    2,
			RequiresBackup: false,
			Run: func(state *RepoState) ([]string, *RepoState, error) {
				out, err := runGitControlled("config", "gc.auto", "0")
				log := []string{"git config gc.auto 0", out}
				if err != nil {
					return log, state, err
				}
				post, _ := Inspect()
				return log, post, nil
			},
		},
		"suggest-reconcile-draft": {
			ID:             "suggest-reconcile-draft",
			Description:    "delega a charlie reconcile-draft (no ejecuta: sugiere)",
			SafetyLevel:    1,
			RequiresBackup: false,
			Run: func(state *RepoState) ([]string, *RepoState, error) {
				return []string{"SIGUIENTE COMANDO CHARLIE: go run ./cmd/charlie reconcile-draft"}, state, nil
			},
		},
		"reattach-to-draft": {
			ID:             "reattach-to-draft",
			Description:    "sugiere git switch draft; Charlie no cambia branch automaticamente",
			SafetyLevel:    1,
			RequiresBackup: false,
			Run: func(state *RepoState) ([]string, *RepoState, error) {
				return []string{
					"Charlie no ejecuta git switch/checkout.",
					"El humano debe ejecutar: git switch draft",
				}, state, nil
			},
		},
	}
}

// findLastValidReflogSHA walks the reflog in reverse and returns the first
// SHA whose git object exists locally. Reflog format:
//
//	<old> <new> <author> <timestamp> <msg>
func findLastValidReflogSHA(state *RepoState) string {
	for i := len(state.ReflogTail) - 1; i >= 0; i-- {
		fields := strings.Fields(state.ReflogTail[i])
		if len(fields) < 2 {
			continue
		}
		// Check "new" (col 1) first, then "old" (col 0).
		for _, candidate := range []string{fields[1], fields[0]} {
			if gitObjectExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// ----------------------------------------------------------------------------
// Orchestration
// ----------------------------------------------------------------------------

// RunDoctor inspects + diagnoses. Read-only. Equivalent to `charlie doctor`.
func RunDoctor() (*DoctorReport, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}
	state, err := Inspect()
	if err != nil {
		return nil, err
	}
	diags := Diagnose(state)
	return &DoctorReport{
		State:         state,
		Diagnoses:     diags,
		OverallHealth: overallSeverity(diags),
	}, nil
}

// ApplyDoctor runs RunDoctor and then executes, in order, the highest-priority
// recipe for each non-OK diagnosis. Takes a backup first if any recipe has
// SafetyLevel >= 3.
func ApplyDoctor() (*DoctorReport, error) {
	report, err := RunDoctor()
	if err != nil {
		return nil, err
	}
	if report.OverallHealth == SeverityOK || report.OverallHealth == SeverityInfo {
		return report, nil
	}

	recipes := RegisteredRecipes()
	needsBackup := false
	var queue []Recipe
	for _, d := range report.Diagnoses {
		if len(d.RecipeIDs) == 0 {
			continue
		}
		r, ok := recipes[d.RecipeIDs[0]]
		if !ok {
			continue
		}
		if r.RequiresBackup {
			needsBackup = true
		}
		queue = append(queue, r)
	}

	if needsBackup {
		path, berr := BackupWorkingTree()
		if berr != nil {
			return report, fmt.Errorf("no pude crear backup antes de aplicar recetas: %v", berr)
		}
		report.BackupPath = path
	}

	state := report.State
	for _, r := range queue {
		log, newState, rerr := r.Run(state)
		report.Applied = append(report.Applied, r.ID)
		report.AppliedLog = append(report.AppliedLog, log)
		if rerr != nil {
			log = append(log, "ERROR: "+rerr.Error())
			report.AppliedLog[len(report.AppliedLog)-1] = log
			break
		}
		if newState != nil {
			state = newState
		}
		appendAudit("doctor.apply", map[string]string{
			"recipe":      r.ID,
			"head_before": safeSha(report.State.HeadSHA),
			"head_after":  safeSha(state.HeadSHA),
		})
	}
	report.PostState = state

	// Re-run diagnose on the post-state to summarize recovery outcome.
	if state != nil {
		post := Diagnose(state)
		report.OverallHealth = overallSeverity(post)
		report.Recommendation = recommendationFor(post)
		// Replace diagnoses with the post-state ones so the user sees whether
		// the apply succeeded.
		report.Diagnoses = post
	}
	return report, nil
}

func overallSeverity(diags []Diagnosis) DoctorSeverity {
	rank := map[DoctorSeverity]int{
		SeverityOK: 0, SeverityInfo: 1, SeverityWarning: 2, SeverityBlocker: 3, SeverityCritical: 4,
	}
	max := SeverityOK
	for _, d := range diags {
		if rank[d.Severity] > rank[max] {
			max = d.Severity
		}
	}
	return max
}

func recommendationFor(diags []Diagnosis) string {
	for _, d := range diags {
		if d.Severity == SeverityCritical || d.Severity == SeverityBlocker {
			return "bloqueo no resuelto: " + d.Code + " - " + d.Summary
		}
	}
	return "repo saludable. Puedes continuar con preflight / propose."
}

func safeSha(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// ----------------------------------------------------------------------------
// Formatting
// ----------------------------------------------------------------------------

func FormatDoctorReport(report *DoctorReport) string {
	var b strings.Builder
	b.WriteString("=== CHARLIE DOCTOR ===\n\n")
	fmt.Fprintf(&b, "estado: %s\n", strings.ToUpper(string(report.OverallHealth)))
	if report.State != nil {
		fmt.Fprintf(&b, "branch: %s\n", valueOrDash(report.State.Branch))
		fmt.Fprintf(&b, "HEAD: %s (objeto presente=%v)\n", safeSha(report.State.HeadSHA), report.State.HeadObjectExists)
		if report.State.UpstreamRef != "" {
			fmt.Fprintf(&b, "upstream: %s (ahead=%d behind=%d)\n", report.State.UpstreamRef, report.State.UpstreamAhead, report.State.UpstreamBehind)
		}
		if report.State.GcAuto != "" {
			fmt.Fprintf(&b, "gc.auto: %s\n", report.State.GcAuto)
		}
	}
	if report.BackupPath != "" {
		fmt.Fprintf(&b, "backup: %s\n", report.BackupPath)
	}
	b.WriteString("\n")

	for _, d := range report.Diagnoses {
		fmt.Fprintf(&b, "[%s] %s\n  %s\n", strings.ToUpper(string(d.Severity)), d.Code, d.Summary)
		for _, e := range d.Evidence {
			fmt.Fprintf(&b, "    - %s\n", e)
		}
		if len(d.RecipeIDs) > 0 {
			fmt.Fprintf(&b, "    recetas disponibles: %s\n", strings.Join(d.RecipeIDs, ", "))
		}
	}

	if len(report.Applied) > 0 {
		b.WriteString("\n--- recetas aplicadas ---\n")
		for i, id := range report.Applied {
			fmt.Fprintf(&b, "\n• %s\n", id)
			for _, line := range report.AppliedLog[i] {
				if strings.TrimSpace(line) == "" {
					continue
				}
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
	}

	if report.Recommendation != "" {
		fmt.Fprintf(&b, "\nsiguiente paso: %s\n", report.Recommendation)
	}
	return b.String()
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// PreflightHealthBlockers returns the health-related blockers that should
// prevent preflight/propose/apply-propose from continuing. Used by Preflight
// to add doctor-level awareness without running mutations.
func PreflightHealthBlockers() []string {
	state, err := Inspect()
	if err != nil {
		return []string{fmt.Sprintf("doctor inspect fallo: %v", err)}
	}
	var blockers []string
	for _, d := range Diagnose(state) {
		if d.Severity == SeverityBlocker || d.Severity == SeverityCritical {
			blockers = append(blockers, fmt.Sprintf("[%s] %s (code=%s)", strings.ToUpper(string(d.Severity)), d.Summary, d.Code))
		}
	}
	return blockers
}

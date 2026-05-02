package charlie

import (
	"strings"
	"testing"
)

func TestDiagnoseMissingObjectIsCritical(t *testing.T) {
	state := &RepoState{
		HeadRef:          "refs/heads/draft",
		Branch:           "draft",
		HeadSHA:          "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		HeadObjectExists: false,
		RemoteURL:        "https://github.com/example/repo.git",
		GcLogPath:        ".git/gc.log",
	}
	diags := Diagnose(state)

	var found *Diagnosis
	for i := range diags {
		if diags[i].Code == CodeMissingObject {
			found = &diags[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("Diagnose should surface REPO_CORRUPT_MISSING_OBJECT, got %+v", diags)
	}
	if found.Severity != SeverityCritical {
		t.Fatalf("missing object should be critical, got %q", found.Severity)
	}
	if len(found.RecipeIDs) == 0 || found.RecipeIDs[0] != "fetch-missing-objects" {
		t.Fatalf("first recipe for missing object should be fetch-missing-objects, got %v", found.RecipeIDs)
	}
}

func TestDiagnoseHealthyRepoReturnsOk(t *testing.T) {
	state := &RepoState{
		HeadRef:          "refs/heads/draft",
		Branch:           "draft",
		HeadSHA:          "abc",
		HeadObjectExists: true,
		UpstreamRef:      "origin/draft",
		UpstreamSHA:      "abc",
	}
	diags := Diagnose(state)
	if len(diags) != 1 || diags[0].Code != CodeRepoOK {
		t.Fatalf("healthy repo should return a single REPO_OK diagnosis, got %+v", diags)
	}
}

func TestDiagnoseDetachedHeadIsBlocker(t *testing.T) {
	state := &RepoState{
		HeadSHA:          "abc123",
		HeadObjectExists: true,
	}
	diags := Diagnose(state)
	hasDetached := false
	for _, d := range diags {
		if d.Code == CodeDetachedHead && d.Severity == SeverityBlocker {
			hasDetached = true
		}
	}
	if !hasDetached {
		t.Fatalf("detached HEAD should be a blocker, got %+v", diags)
	}
}

func TestDiagnoseNotDraftIsBlocker(t *testing.T) {
	state := &RepoState{
		HeadRef:          "refs/heads/feature-x",
		Branch:           "feature-x",
		HeadSHA:          "abc",
		HeadObjectExists: true,
	}
	diags := Diagnose(state)
	hasNotDraft := false
	for _, d := range diags {
		if d.Code == CodeNotDraft {
			hasNotDraft = true
		}
	}
	if !hasNotDraft {
		t.Fatal("non-draft branch must raise BRANCH_NOT_DRAFT")
	}
}

func TestDiagnoseGcAggressiveIsWarning(t *testing.T) {
	state := &RepoState{
		HeadRef:          "refs/heads/draft",
		Branch:           "draft",
		HeadSHA:          "abc",
		HeadObjectExists: true,
		GcAuto:           "6700",
	}
	diags := Diagnose(state)
	hasGc := false
	for _, d := range diags {
		if d.Code == CodeGcAggressive {
			hasGc = true
			if d.Severity != SeverityWarning {
				t.Fatalf("gc.auto != 0 should be warning, got %q", d.Severity)
			}
		}
	}
	if !hasGc {
		t.Fatal("gc.auto != 0 should surface GC_AGGRESSIVE_CONFIG")
	}
}

func TestDiagnoseUnmergedIsBlocker(t *testing.T) {
	state := &RepoState{
		Branch:           "draft",
		HeadSHA:          "abc",
		HeadObjectExists: true,
		HasUnmerged:      true,
	}
	diags := Diagnose(state)
	found := false
	for _, d := range diags {
		if d.Code == CodeUnresolvedMerge && d.Severity == SeverityBlocker {
			found = true
		}
	}
	if !found {
		t.Fatal("unmerged paths must raise MERGE_UNRESOLVED blocker")
	}
}

func TestOverallSeverityPicksHighest(t *testing.T) {
	diags := []Diagnosis{
		{Code: "A", Severity: SeverityInfo},
		{Code: "B", Severity: SeverityBlocker},
		{Code: "C", Severity: SeverityWarning},
	}
	if overallSeverity(diags) != SeverityBlocker {
		t.Fatalf("overall should be blocker, got %v", overallSeverity(diags))
	}
}

func TestFormatDoctorReportHighlightsCriticalCode(t *testing.T) {
	report := &DoctorReport{
		State: &RepoState{
			Branch:           "draft",
			HeadSHA:          "abcd1234",
			HeadObjectExists: false,
		},
		Diagnoses: []Diagnosis{
			{
				Code:      CodeMissingObject,
				Severity:  SeverityCritical,
				Summary:   "HEAD apunta a objeto faltante",
				RecipeIDs: []string{"fetch-missing-objects"},
			},
		},
		OverallHealth: SeverityCritical,
	}
	got := FormatDoctorReport(report)
	for _, want := range []string{"CRITICAL", "REPO_CORRUPT_MISSING_OBJECT", "fetch-missing-objects"} {
		if !strings.Contains(got, want) {
			t.Fatalf("report must include %q:\n%s", want, got)
		}
	}
}

func TestBuildIntentPlanMapsCommitAndPush(t *testing.T) {
	plan := BuildIntentPlan("commitea todos los cambios y hace push")
	if len(plan.Steps) < 3 {
		t.Fatalf("commit-and-push intent should yield >=3 steps, got %d: %+v", len(plan.Steps), plan.Steps)
	}
	last := plan.Steps[len(plan.Steps)-1].Command
	if !strings.Contains(last, "apply-propose --apply --push") {
		t.Fatalf("last step should be apply-propose --apply --push, got %q", last)
	}
}

func TestBuildIntentPlanMapsRecovery(t *testing.T) {
	plan := BuildIntentPlan("necesito recuperar el repo")
	if len(plan.Steps) < 1 {
		t.Fatal("recover intent should yield steps")
	}
	if !strings.Contains(plan.Steps[0].Command, "doctor") {
		t.Fatalf("recover intent should start with doctor, got %q", plan.Steps[0].Command)
	}
}

func TestBuildIntentPlanReportsAmbiguity(t *testing.T) {
	plan := BuildIntentPlan("xyz123")
	if len(plan.Steps) != 0 {
		t.Fatal("unknown intent should return no steps")
	}
	if len(plan.Ambiguity) == 0 {
		t.Fatal("unknown intent should report ambiguity")
	}
}

func TestPrependSectionInsertsAfterKeepAChangelogHeader(t *testing.T) {
	existing := "# Changelog\n\nThe format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),\nand this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n\n## [0.1.7]\n\nold\n"
	generated := "## [0.1.8]\n\nnew entry\n"
	got := prependSection(existing, generated)
	if !strings.Contains(got, "## [0.1.8]") {
		t.Fatalf("new section missing:\n%s", got)
	}
	// v0.1.8 must appear before v0.1.7.
	if strings.Index(got, "## [0.1.8]") > strings.Index(got, "## [0.1.7]") {
		t.Fatalf("new section must come before older ones:\n%s", got)
	}
}

func TestVersionHeaderStripsV(t *testing.T) {
	if versionHeader("v0.1.8") != "## [0.1.8]" {
		t.Fatal("versionHeader must strip leading v")
	}
	if versionHeader("0.1.8") != "## [0.1.8]" {
		t.Fatal("versionHeader must handle missing v")
	}
}

package deployer

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls []string
}

func (f *fakeRunner) Run(ctx context.Context, cwd, name string, args ...string) CommandResult {
	cmd := shellQuote(append([]string{name}, args...))
	f.calls = append(f.calls, cmd)
	if strings.Contains(cmd, "git rev-parse --short HEAD") {
		return CommandResult{Command: cmd, OK: true, Output: "9a39514"}
	}
	return CommandResult{Command: cmd, OK: true, Output: "ok"}
}

func (f *fakeRunner) RunShell(ctx context.Context, cwd, script string) CommandResult {
	f.calls = append(f.calls, script)
	return CommandResult{Command: script, OK: true, Output: "ok"}
}

func testRunbook(fr *fakeRunner) *Runbook {
	return &Runbook{
		Config: DefaultConfig(),
		Root:   "/repo",
		Runner: fr,
		Now: func() time.Time {
			return time.Date(2026, 5, 6, 17, 13, 5, 0, time.UTC)
		},
	}
}

func TestPlanDevUsesOfficialTargetAndForbidsProd(t *testing.T) {
	r := testRunbook(&fakeRunner{})
	plan, err := r.Plan(PlanInput{Intent: "deploy dev", RequestedEnv: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Env != Dev || plan.Service != "flujo-api-dev" {
		t.Fatalf("unexpected dev plan: %+v", plan)
	}
	if plan.RequiresConfirmation {
		t.Fatalf("dev should not require confirmation")
	}
	if len(plan.ForbiddenServices) != 1 || plan.ForbiddenServices[0] != "flujo-api" {
		t.Fatalf("prod service must be forbidden during dev: %+v", plan.ForbiddenServices)
	}
}

func TestProdBuildBlockedWithoutExactConfirmation(t *testing.T) {
	fr := &fakeRunner{}
	r := testRunbook(fr)
	rep := r.Build(context.Background(), Prod, "confirmo")
	if rep.Status != "blocked" {
		t.Fatalf("expected blocked, got %+v", rep)
	}
	if len(fr.calls) != 0 {
		t.Fatalf("blocked prod build must not run commands, got %v", fr.calls)
	}
}

func TestBuildDevUsesTraceableTagAndDevService(t *testing.T) {
	fr := &fakeRunner{}
	r := testRunbook(fr)
	rep := r.Build(context.Background(), Dev, "")
	if rep.Status != "needs_attention" && rep.Status != "ok" {
		t.Fatalf("unexpected status: %+v", rep)
	}
	if rep.Tag != "9a39514-dev-20260506171305" {
		t.Fatalf("unexpected tag: %q", rep.Tag)
	}
	joined := strings.Join(fr.calls, "\n")
	if !strings.Contains(joined, "_SERVICE_NAME=flujo-api-dev") {
		t.Fatalf("build must target dev service, calls:\n%s", joined)
	}
	if strings.Contains(joined, "_SERVICE_NAME=flujo-api,") {
		t.Fatalf("dev build must not target prod service, calls:\n%s", joined)
	}
}

func TestDeployProdBlockedWithoutExactConfirmationAndTagRequired(t *testing.T) {
	fr := &fakeRunner{}
	r := testRunbook(fr)
	blocked := r.Deploy(context.Background(), Prod, "tag", "")
	if blocked.Status != "blocked" || len(fr.calls) != 0 {
		t.Fatalf("prod deploy should block without commands: %+v calls=%v", blocked, fr.calls)
	}
	missingTag := r.Deploy(context.Background(), Dev, "", "")
	if missingTag.Status != "blocked" {
		t.Fatalf("dev deploy without tag should block: %+v", missingTag)
	}
}

func TestDiagnoseKnownPaladinWhitelistError(t *testing.T) {
	d := DiagnoseText("command not allowed: ./frameworkpaladin")
	if !strings.Contains(d.Remediation, "whitelist") {
		t.Fatalf("expected whitelist remediation, got %+v", d)
	}
}

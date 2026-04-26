package paladin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRepoFindsSemanticCoverage(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "main.go", `package main

import "github.com/remora-go/framework-paladin/paladin"

func main() {
	trace := paladin.NewTrace("demo")
	ctx := trace.Start()
	defer trace.Flush()
	child := ctx.Child("flow")
	child.Actor("echo", "discover")
	child.Goal("handoff")
	child.Rule("echo_to_alfa", "after 2 answers", nil)
	child.Check("echo_to_alfa", "answers >= 2", "answers = 2", true)
	child.Decision("handoff", "rule passed")
	child.Expect("next_actor", "alfa")
	child.Handoff("echo", "alfa", "ready")
}
`)

	result, err := AuditRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Calls["Rule"] != 1 || result.Calls["Check"] != 1 || result.Calls["Handoff"] != 1 {
		t.Fatalf("expected semantic calls, got %#v", result.Calls)
	}
	for _, finding := range result.Findings {
		if finding.Level == "fail" {
			t.Fatalf("unexpected fail finding: %#v", finding)
		}
	}
}

func TestAuditRepoFlagsTechnicalOnlyTracing(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "main.go", `package main

import "github.com/remora-go/framework-paladin/paladin"

func main() {
	trace := paladin.NewTrace("demo")
	ctx := trace.Start()
	defer trace.Flush()
	child := ctx.Child("flow")
	child.Var("turn_count", 2)
	child.Decision("handoff", "turn_count >= 2")
}
`)

	result, err := AuditRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(result, "no_semantic_layer") {
		t.Fatalf("expected no_semantic_layer finding, got %#v", result.Findings)
	}
	if !hasFinding(result, "decisions_without_rules") {
		t.Fatalf("expected decisions_without_rules finding, got %#v", result.Findings)
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func hasFinding(result AuditResult, code string) bool {
	for _, finding := range result.Findings {
		if strings.EqualFold(finding.Code, code) {
			return true
		}
	}
	return false
}

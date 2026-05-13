package pingpong

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileCheckGoSingleFileOutsideModule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rep := CompileCheck(path, DefaultLangConfigs["go"])
	if !rep.CompileOK {
		t.Fatalf("expected go compile ok outside module, got %q", rep.CompileLog)
	}
}

func TestReviewAutoAcceptsCurrentStepAndAdvances(t *testing.T) {
	c, dir := setupTestClient(t)
	writeGoFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	if _, err := c.Start("goal", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetSteps("[main.go]Crear función main;[main.go]Crear función helper"); err != nil {
		t.Fatal(err)
	}

	res, err := c.Review("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %q", res.Message)
	}

	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", res.Data)
	}
	if accepted, _ := data["accepted"].(bool); !accepted {
		t.Fatalf("expected review to auto-accept current step: %#v", data)
	}

	p := loadProgress(t)
	if !p.Steps[0].Done {
		t.Fatal("expected first step done after review")
	}
	if p.CurrentStep != 2 {
		t.Fatalf("expected currentStep=2 after review, got %d", p.CurrentStep)
	}

	next, err := c.Next()
	if err != nil {
		t.Fatal(err)
	}
	state, ok := next.Data.(TutorState)
	if !ok {
		t.Fatalf("expected TutorState, got %T", next.Data)
	}
	if !strings.Contains(state.Say, "Crear función helper") {
		t.Fatalf("expected next step to mention helper, got %q", state.Say)
	}
}

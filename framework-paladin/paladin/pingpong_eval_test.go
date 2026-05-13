package paladin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunPingPongScenarioCompletesE2E(t *testing.T) {
	result, err := RunPingPongScenario(PingPongScenario{
		Name:        "pingpong complete external e2e flow",
		PingPongDir: pingPongRepoDir(t),
		InitialFiles: []PingPongFile{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
			},
		},
		Actions: []PingPongAction{
			{Label: "reset", Args: []string{"reset"}},
			{Label: "start", Args: []string{"start", "--goal", "practicar go con una función helper"}},
			{Label: "set-steps", Args: []string{"set-steps", "--steps", "[main.go]Crear función main;[main.go]Crear función helper"}},
			{Label: "next-1", Args: []string{"next"}},
			{Label: "review-main", Args: []string{"review", "--file", "main.go"}},
			{
				Label: "user-adds-helper",
				Files: []PingPongFile{
					{
						Path:    "main.go",
						Content: "package main\n\nfunc helper() {}\n\nfunc main() {\n\thelper()\n}\n",
					},
				},
			},
			{Label: "next-2", Args: []string{"next"}},
			{Label: "review-helper", Args: []string{"review", "--file", "main.go"}},
			{
				Label: "user-rewrites-minitest",
				Files: []PingPongFile{
					{
						Path:    "main.go",
						Content: "package main\n\nimport \"fmt\"\n\nfunc helper() string {\n\treturn \"hola\"\n}\n\nfunc main() {\n\tfmt.Println(helper())\n}\n",
					},
				},
			},
			{Label: "next-minitest-1", Args: []string{"next"}},
			{Label: "review-minitest-main", Args: []string{"review", "--file", "main.go"}},
			{Label: "next-minitest-2", Args: []string{"next"}},
			{Label: "review-minitest-helper", Args: []string{"review", "--file", "main.go"}},
			{Label: "run-final", Args: []string{"run", "--file", "main.go", "--expect", "hola"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Steps) < 13 {
		t.Fatalf("expected multiple transcript steps, got %d", len(result.Steps))
	}

	next1 := findStepByLabel(t, result, "next-1")
	if !strings.Contains(next1.Message, "Crear función main") {
		t.Fatalf("expected first next message to mention main, got %q", next1.Message)
	}

	review1 := findStepByLabel(t, result, "review-main")
	if review1.Success == nil || !*review1.Success {
		t.Fatalf("expected first review success, got %#v", review1)
	}
	if !strings.Contains(review1.Raw, `"accepted": true`) {
		t.Fatalf("expected first review to auto-accept, got %s", review1.Raw)
	}

	next2 := findStepByLabel(t, result, "next-2")
	if !strings.Contains(next2.Message, "Crear función helper") {
		t.Fatalf("expected second next message to mention helper, got %q", next2.Message)
	}

	review2 := findStepByLabel(t, result, "review-helper")
	if !strings.Contains(review2.Message, "Mini-test paso 1/2") {
		t.Fatalf("expected review-helper to enter mini-test, got %q", review2.Message)
	}

	nextM1 := findStepByLabel(t, result, "next-minitest-1")
	if !strings.Contains(nextM1.Message, "Mini-test paso 1/2") {
		t.Fatalf("expected first mini-test next message, got %q", nextM1.Message)
	}

	reviewM1 := findStepByLabel(t, result, "review-minitest-main")
	if reviewM1.Success == nil || !*reviewM1.Success {
		t.Fatalf("expected first mini-test review success, got %#v", reviewM1)
	}

	nextM2 := findStepByLabel(t, result, "next-minitest-2")
	if !strings.Contains(nextM2.Message, "Mini-test paso 2/2") {
		t.Fatalf("expected second mini-test next message, got %q", nextM2.Message)
	}

	reviewM2 := findStepByLabel(t, result, "review-minitest-helper")
	if reviewM2.Success == nil || !*reviewM2.Success {
		t.Fatalf("expected final mini-test review success, got %#v", reviewM2)
	}
	if !strings.Contains(reviewM2.Message, "fase final con run") && !strings.Contains(reviewM2.Raw, `"completedAll": true`) {
		t.Fatalf("expected final mini-test review to complete all steps, got %q", reviewM2.Message)
	}

	runFinal := findStepByLabel(t, result, "run-final")
	if runFinal.Success == nil || !*runFinal.Success {
		t.Fatalf("expected final run success, got %#v", runFinal)
	}
	if !strings.Contains(runFinal.Message, "Output correcto") {
		t.Fatalf("expected final run success message, got %q", runFinal.Message)
	}

	if _, err := os.Stat(result.ArtifactPath); err != nil {
		t.Fatalf("expected artifact.json, got %v", err)
	}
	if _, err := os.Stat(result.ConversationPath); err != nil {
		t.Fatalf("expected conversation.txt, got %v", err)
	}
	if _, err := os.Stat(result.TranscriptPath); err != nil {
		t.Fatalf("expected transcript.txt, got %v", err)
	}
}

func pingPongRepoDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "framework-pingpong")
}

func findStepByLabel(t *testing.T, result *PingPongRunResult, label string) PingPongStepResult {
	t.Helper()
	for _, step := range result.Steps {
		if step.Label == label {
			return step
		}
	}
	t.Fatalf("step %q not found", label)
	return PingPongStepResult{}
}

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEcommercePedidosRunsAndWritesArtifacts(t *testing.T) {
	exampleDir := testExampleDir(t)
	tempDir := filepath.Join(exampleDir, "temp")

	if err := os.RemoveAll(tempDir); err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	cmd := exec.Command(testGoBinary(t), "run", ".")
	cmd.Dir = exampleDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run . failed: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "=== RESUMEN FINAL ===") {
		t.Fatalf("example output missing summary marker:\n%s", output)
	}

	for _, name := range []string{"ideal_flow.json", "IDEAL_FLOW.md"} {
		path := filepath.Join(tempDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}

	traceFiles, err := filepath.Glob(filepath.Join(tempDir, "trace_*.json"))
	if err != nil {
		t.Fatalf("glob trace files: %v", err)
	}
	if len(traceFiles) == 0 {
		t.Fatalf("expected at least one trace_*.json in %s", tempDir)
	}
}

func testExampleDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Dir(filename)
}

func testGoBinary(t *testing.T) string {
	t.Helper()

	candidate := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	if path, err := exec.LookPath("go"); err == nil {
		return path
	}

	t.Fatalf("go binary not found in GOROOT=%q or PATH", runtime.GOROOT())
	return ""
}

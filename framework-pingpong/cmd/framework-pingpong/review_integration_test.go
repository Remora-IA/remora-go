package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewCLIAdvancesWithoutManualAccept(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "pingpong")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = pkgDir
	build.Env = append(os.Environ(), "DISABLE_TRACES=1")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	workDir := t.TempDir()
	src := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(workDir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	runCLI(t, bin, workDir, "reset")
	runCLI(t, bin, workDir, "start", "--goal", "probar review")
	runCLI(t, bin, workDir, "set-steps", "--steps", "[main.go]Crear función main;[main.go]Crear función helper")
	out := runCLI(t, bin, workDir, "review", "--file", "main.go")

	if !strings.Contains(out, `"accepted": true`) {
		t.Fatalf("expected CLI review to auto-accept step, got:\n%s", out)
	}
	if !strings.Contains(out, "Crear función helper") {
		t.Fatalf("expected CLI review to advance to helper step, got:\n%s", out)
	}
	if strings.Contains(out, "ejecutá ./pingpong accept") {
		t.Fatalf("expected CLI review to avoid manual accept guidance, got:\n%s", out)
	}
}

func runCLI(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "DISABLE_TRACES=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

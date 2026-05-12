package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"channel/manifest"
)

func TestResolveManifestRuntimeFallsBackToGoRunWhenBinaryIsStale(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "framework-hosting")
	if err := os.MkdirAll(filepath.Join(cwd, "cmd", "frameworkhosting"), 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(cwd, "frameworkhosting")
	if err := os.WriteFile(binPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(binPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(cwd, "cmd", "frameworkhosting", "main.go")
	if err := os.WriteFile(srcPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(srcPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	rt := resolveManifestRuntime(root, &manifest.Manifest{
		Name: "hosting",
		Build: manifest.BuildSpec{
			Command: "go",
			Args:    []string{"build", "-buildvcs=false", "-o", "frameworkhosting", "./cmd/frameworkhosting"},
		},
		Binary: manifest.BinarySpec{Command: "./frameworkhosting"},
		Cwd:    "framework-hosting",
	})

	if rt.Command != "go" {
		t.Fatalf("command=%q want go (%#v)", rt.Command, rt)
	}
	if rt.Mode != "go_run_fallback" {
		t.Fatalf("mode=%q want go_run_fallback (%#v)", rt.Mode, rt)
	}
}

func TestResolveManifestRuntimeUsesFreshBuiltBinary(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "framework-mensajero")
	if err := os.MkdirAll(filepath.Join(cwd, "cmd", "frameworkmensajero"), 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(cwd, "cmd", "frameworkmensajero", "main.go")
	if err := os.WriteFile(srcPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srcTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(srcPath, srcTime, srcTime); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(cwd, "frameworkmensajero")
	if err := os.WriteFile(binPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	binTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(binPath, binTime, binTime); err != nil {
		t.Fatal(err)
	}

	rt := resolveManifestRuntime(root, &manifest.Manifest{
		Name: "mensajero",
		Build: manifest.BuildSpec{
			Command: "go",
			Args:    []string{"build", "-buildvcs=false", "-o", "frameworkmensajero", "./cmd/frameworkmensajero"},
		},
		Binary: manifest.BinarySpec{Command: "./frameworkmensajero"},
		Cwd:    "framework-mensajero",
	})

	if rt.Command != "./frameworkmensajero" {
		t.Fatalf("command=%q want ./frameworkmensajero (%#v)", rt.Command, rt)
	}
	if rt.Mode != "manifest_binary" {
		t.Fatalf("mode=%q want manifest_binary (%#v)", rt.Mode, rt)
	}
	if rt.Freshness != "fresh" {
		t.Fatalf("freshness=%q want fresh (%#v)", rt.Freshness, rt)
	}
}

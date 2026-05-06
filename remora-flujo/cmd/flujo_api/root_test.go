package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRemoraRootFromSubdir(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"channel", "remora-flujo", "framework-echo", "framework-alfa"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	subdir := filepath.Join(root, "remora-flujo", "cmd", "flujo_api")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	got, ok := findRemoraRoot(subdir)
	if !ok {
		t.Fatal("expected remora root")
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestResolveRemoraRootPrefersEnv(t *testing.T) {
	t.Setenv("REMORA_ROOT", "/tmp/remora-root-test")
	t.Setenv("CHANNEL_BASE_DIR", "/tmp/channel-root-test")
	if got := resolveRemoraRoot(); got != "/tmp/remora-root-test" {
		t.Fatalf("root = %q", got)
	}
}

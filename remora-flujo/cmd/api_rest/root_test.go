package main

import (
	"io"
	"log"
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
	subdir := filepath.Join(root, "remora-flujo", "cmd", "api_rest")
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

func TestLocalDiscoveryFindsRepoFrameworkManifests(t *testing.T) {
	t.Setenv("REMORA_ROOT", "")
	t.Setenv("CHANNEL_BASE_DIR", "")
	root := resolveRemoraRoot()
	oldRegistry := driverRegistry
	driverRegistry = map[string]FrameworkDriver{}
	t.Cleanup(func() {
		driverRegistry = oldRegistry
	})

	loaded, skipped := initDriverRegistry(root, log.New(io.Discard, "", 0))
	if len(loaded) <= 2 {
		t.Fatalf("loaded manifests = %d, skipped = %d, root = %q; expected more than hardcoded alfa/echo", len(loaded), len(skipped), root)
	}
	testable := 0
	for _, m := range loaded {
		if frameworkManifestTestable(m) {
			testable++
		}
	}
	if testable <= 2 {
		t.Fatalf("testable manifests = %d, loaded = %d, root = %q; expected more than alfa/echo", testable, len(loaded), root)
	}
}

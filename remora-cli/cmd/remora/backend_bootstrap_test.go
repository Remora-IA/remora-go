package main

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestEnsureBackendReadySkipsAutoBootstrapForRemoteAPI(t *testing.T) {
	err := ensureBackendReadyWithDeps(backendBootstrapDeps{
		apiURL: "https://remora.example/api/v1",
		stderr: io.Discard,
		findRepoRoot: func() string {
			t.Fatal("findRepoRoot should not be called for remote backends")
			return ""
		},
		healthCheck: func(url string) error {
			t.Fatalf("healthCheck should not run for remote backend: %s", url)
			return nil
		},
		runCommand: func(spec backendCommandSpec) error {
			t.Fatalf("runCommand should not run for remote backend: %#v", spec)
			return nil
		},
		startDetachedCommand: func(spec backendCommandSpec) error {
			t.Fatalf("startDetachedCommand should not run for remote backend: %#v", spec)
			return nil
		},
		waitForHealth: func(url string, attempts int, delay time.Duration) error {
			t.Fatalf("waitForHealth should not run for remote backend: %s", url)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureBackendReadyChecksChannelAndAPIBeforeSkippingBootstrap(t *testing.T) {
	var healthChecks []string
	err := ensureBackendReadyWithDeps(backendBootstrapDeps{
		stderr: io.Discard,
		healthCheck: func(url string) error {
			healthChecks = append(healthChecks, url)
			return nil
		},
		findRepoRoot: func() string {
			t.Fatal("findRepoRoot should not be called when backend is already ready")
			return ""
		},
		runCommand: func(spec backendCommandSpec) error {
			t.Fatalf("runCommand should not run when backend is already ready: %#v", spec)
			return nil
		},
		startDetachedCommand: func(spec backendCommandSpec) error {
			t.Fatalf("startDetachedCommand should not run when backend is already ready: %#v", spec)
			return nil
		},
		waitForHealth: func(url string, attempts int, delay time.Duration) error {
			t.Fatalf("waitForHealth should not run when backend is already ready: %s", url)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(healthChecks, localAPIHealthURL) {
		t.Fatalf("expected api health check in %#v", healthChecks)
	}
	if !containsString(healthChecks, localChannelHealthURL) {
		t.Fatalf("expected channel health check in %#v", healthChecks)
	}
}

func TestEnsureBackendReadyBootstrapsInlineWithoutTerminalApp(t *testing.T) {
	body := mustReadRemoraFunctionBody(t, "main.go", "func ensureBackendReadyWithDeps(")
	for _, forbidden := range []string{"osascript", "dev-local.sh"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("ensureBackendReadyWithDeps should not depend on %q:\n%s", forbidden, body)
		}
	}

	ready := map[string]bool{}
	var started []backendCommandSpec
	var ran []backendCommandSpec
	err := ensureBackendReadyWithDeps(backendBootstrapDeps{
		stderr:       io.Discard,
		findRepoRoot: func() string { return "/repo" },
		healthCheck: func(url string) error {
			if ready[url] {
				return nil
			}
			return errors.New("down")
		},
		startDetachedCommand: func(spec backendCommandSpec) error {
			started = append(started, spec)
			ready[localChannelHealthURL] = true
			return nil
		},
		runCommand: func(spec backendCommandSpec) error {
			ran = append(ran, spec)
			ready[localAPIHealthURL] = true
			return nil
		},
		waitForHealth: func(url string, attempts int, delay time.Duration) error {
			if ready[url] {
				return nil
			}
			return errors.New("still down")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(started) != 1 {
		t.Fatalf("expected one detached command, got %#v", started)
	}
	if started[0].Name != "go" {
		t.Fatalf("channel bootstrap should use go directly, got %#v", started[0])
	}
	if started[0].Dir != "/repo/channel" {
		t.Fatalf("channel bootstrap dir = %q", started[0].Dir)
	}
	if !strings.Contains(strings.Join(started[0].Args, " "), "./cmd/channel") {
		t.Fatalf("channel bootstrap args = %#v", started[0].Args)
	}
	assertCommandHasNoForbiddenBootstrapTokens(t, started[0])

	if len(ran) != 1 {
		t.Fatalf("expected one blocking command, got %#v", ran)
	}
	if ran[0].Name != "make" || len(ran[0].Args) != 1 || ran[0].Args[0] != "restart-api" {
		t.Fatalf("api bootstrap should use make restart-api, got %#v", ran[0])
	}
	assertCommandHasNoForbiddenBootstrapTokens(t, ran[0])
}

func assertCommandHasNoForbiddenBootstrapTokens(t *testing.T, spec backendCommandSpec) {
	t.Helper()
	flat := spec.Name + " " + strings.Join(spec.Args, " ") + " " + spec.Dir
	for _, forbidden := range []string{"osascript", "dev-local.sh"} {
		if strings.Contains(flat, forbidden) {
			t.Fatalf("unexpected forbidden bootstrap token %q in %#v", forbidden, spec)
		}
	}
}

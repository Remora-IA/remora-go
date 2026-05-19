package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleFlowWorkbenchDelegatesToCanonicalFlujoCLI(t *testing.T) {
	body := mustReadRemoraFunctionBody(t, filepath.Join("flow_workbench.go"), "func handleFlowWorkbench()")
	if !strings.Contains(body, "delegateToCanonicalFlowWorkbench(") {
		t.Fatalf("handleFlowWorkbench should delegate to canonical flujo CLI:\n%s", body)
	}
	for _, forbidden := range []string{
		"handleFlowCreate(",
		"handleFlowDraft(",
		"handleFlowInspect(",
		"handleFlowSimulate(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("legacy flow subcommands should not remain primary in handleFlowWorkbench: %s\n%s", forbidden, body)
		}
	}
}

func TestPrintFlowWorkbenchUsageMentionsCanonicalCommandSurface(t *testing.T) {
	body := mustReadRemoraFunctionBody(t, filepath.Join("flow_workbench.go"), "func printFlowWorkbenchUsage()")
	for _, want := range []string{
		"remora flow compile --id <flow_id>",
		"remora flow validate --id <flow_id>",
		"remora flow run --id <flow_id>",
		"remora flow install --id <flow_id>",
		"remora flow replay --run <run_id>",
		"remora flow debug --id <flow_id>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("usage should mention canonical flow command %q:\n%s", want, body)
		}
	}
}

func mustReadRemoraFunctionBody(t *testing.T, fileName, signature string) string {
	t.Helper()
	path := filepath.Join("/Users/alcless_a1234_cursor/remora-go-lite/remora-cli/cmd/remora", fileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("signature %q not found in %s", signature, path)
	}
	open := strings.Index(src[start:], "{")
	if open < 0 {
		t.Fatalf("opening brace for %q not found in %s", signature, path)
	}
	open += start
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : i+1]
			}
		}
	}
	t.Fatalf("function body for %q not closed in %s", signature, path)
	return ""
}

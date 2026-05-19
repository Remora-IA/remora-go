package main

import (
	"os"
	"strings"
	"testing"
)

func TestBuilderSaveFlowPostsAuthorialManifestOnly(t *testing.T) {
	html := mustReadStaticIndexHTML(t)
	saveFlow := extractFunctionSource(t, html, "async function saveFlow()")

	for _, banned := range []string{
		"adoptedNodes",
		"adoptedEdges",
		"saveMode === 'adopt_derived'",
		"nodes: adoptedNodes",
		"edges: adoptedEdges",
	} {
		if strings.Contains(saveFlow, banned) {
			t.Fatalf("saveFlow should persist authored manifest only; found forbidden derived-save fragment %q in %s", banned, saveFlow)
		}
	}
	if !strings.Contains(saveFlow, "const manifest = authoredManifest;") {
		t.Fatalf("saveFlow should persist authored manifest directly, got %s", saveFlow)
	}
}

func TestBuilderUIKeepsDerivedVisibleButNotDirectlySaveable(t *testing.T) {
	html := mustReadStaticIndexHTML(t)

	for _, banned := range []string{
		"Incorporar enmiendas derivadas",
		"Guardar con enmiendas derivadas",
		"flows-adopt-derived-btn",
		"flows-summary-adopt-derived",
	} {
		if strings.Contains(html, banned) {
			t.Fatalf("builder UI should not offer direct derived save, found %q", banned)
		}
	}
	for _, required := range []string{
		"<strong>Plan derivado</strong>",
		"JSON.stringify(manifest, null, 2)",
		"JSON.stringify(compiled.flow, null, 2)",
	} {
		if !strings.Contains(html, required) {
			t.Fatalf("builder UI should still expose authored vs compiled comparison, missing %q", required)
		}
	}
}

func mustReadStaticIndexHTML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("read static index: %v", err)
	}
	return string(data)
}

func extractFunctionSource(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("missing function signature %q", signature)
	}
	bodyStart := strings.Index(src[start:], "{")
	if bodyStart < 0 {
		t.Fatalf("missing function body for %q", signature)
	}
	bodyStart += start
	depth := 0
	for i := bodyStart; i < len(src); i++ {
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
	t.Fatalf("unterminated function body for %q", signature)
	return ""
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadJSONArgPrefersPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gaps.json")
	if err := os.WriteFile(path, []byte(`[{"type":"missing_contact","description":"email; > faltante"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := loadJSONArg(path, `[{"type":"inline"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `[{"type":"missing_contact","description":"email; > faltante"}]` {
		t.Fatalf("expected path payload, got %q", got)
	}
}

func TestLoadFindingsAndDatasetPathFallbacks(t *testing.T) {
	dir := t.TempDir()
	findingsPath := filepath.Join(dir, "findings.json")
	datasetPath := filepath.Join(dir, "dataset.json")
	findingsJSON := `{"artifact_type":"auditor.findings.v1","findings":[{"id":"F-1","rule":"missing_email","description":"sin email","severity":"high","auto_fixable":false}]}`
	datasetJSON := `{"artifact_type":"dataset.raw.v1","tables":{"clients":[{"id":"1","name":"Cliente Uno"}]}}`
	if err := os.WriteFile(findingsPath, []byte(findingsJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(datasetPath, []byte(datasetJSON), 0644); err != nil {
		t.Fatal(err)
	}
	findings, err := loadFindingsOrJSONPath("", findingsPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "F-1" {
		t.Fatalf("unexpected findings %#v", findings)
	}
	dataset, err := loadDatasetOrJSONPath("", datasetPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if dataset == nil || len(dataset.Endpoints["clients"]) != 1 {
		t.Fatalf("unexpected dataset %#v", dataset)
	}
}

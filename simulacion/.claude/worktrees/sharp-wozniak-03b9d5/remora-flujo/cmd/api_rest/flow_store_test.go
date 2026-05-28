package main

import (
	"path/filepath"
	"testing"
)

func TestFlowStoreIndexesArtifactAndInstallationState(t *testing.T) {
	fs, err := openFlowStore(filepath.Join(t.TempDir(), "flows.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fs.close()

	result := flowRunResult{
		RunID:      "run_1",
		FlowID:     "flow_1",
		BusinessID: "biz_1",
		Status:     "completed",
		CreatedAt:  "2026-01-01T00:00:00Z",
		FinishedAt: "2026-01-01T00:00:01Z",
	}
	if err := fs.recordRun(result); err != nil {
		t.Fatal(err)
	}
	if err := fs.recordArtifact(result.RunID, result.FlowID, result.BusinessID, "radar", "analysis.plan.v1", "framework_stdout", "/tmp/old.json", "2026-01-01T00:00:01Z"); err != nil {
		t.Fatal(err)
	}
	if err := fs.recordArtifact("run_2", result.FlowID, result.BusinessID, "radar", "analysis.plan.v1", "framework_stdout", "/tmp/new.json", "2026-01-02T00:00:01Z"); err != nil {
		t.Fatal(err)
	}
	if got := fs.latestArtifactPath("biz_1", "analysis.plan.v1"); got != "/tmp/new.json" {
		t.Fatalf("latest path = %q", got)
	}

	want := &flowInstalledSnapshot{
		Installed:      true,
		AnalysisPlan:   "/tmp/new.json",
		AnalysisSchema: "/tmp/schema.json",
		SchemaID:       "schema_1",
		Weights:        map[string]interface{}{"mora": float64(0.8)},
		UpdatedAt:      "2026-01-02T00:00:01Z",
	}
	if err := fs.upsertInstallation("flow_1", "biz_1", want); err != nil {
		t.Fatal(err)
	}
	got := fs.installation("flow_1")
	if got == nil || !got.Installed || got.AnalysisPlan != want.AnalysisPlan || got.SchemaID != want.SchemaID {
		t.Fatalf("installation = %#v", got)
	}
	if got.Weights["mora"] != float64(0.8) {
		t.Fatalf("weights = %#v", got.Weights)
	}
}

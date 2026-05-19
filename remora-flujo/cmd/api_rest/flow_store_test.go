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

func TestCreateFlowPreservesAuthorialManifestAndDropsDerivedPlan(t *testing.T) {
	fs, err := openFlowStore(filepath.Join(t.TempDir(), "flows.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fs.close()

	manifest := &flowManifest{
		Intent: flowIntent{Goal: "enviar correos a la gente que me debe"},
		Derivation: &flowDerivation{
			Executable: flowExecutablePlan{
				Nodes: []flowNode{{ID: "node_foco_entry", Framework: "foco", Capability: "focus.next_collection_task", Role: flowRoleEntry}},
			},
		},
		Nodes: []flowNode{
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}

	created, err := fs.createFlow("Cobranza simple", "desc", "biz-1", manifest)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := fs.getFlow(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Manifest == nil {
		t.Fatal("expected stored manifest")
	}
	if stored.Manifest.Derivation != nil {
		t.Fatalf("stored manifest should not persist derived plan: %#v", stored.Manifest.Derivation)
	}
	if len(stored.Manifest.Nodes) != 2 {
		t.Fatalf("expected authorial nodes preserved, got %#v", stored.Manifest.Nodes)
	}
	if stored.Manifest.Nodes[0].ID != "prioritize" || stored.Manifest.Nodes[1].ID != "draft" {
		t.Fatalf("unexpected node order %#v", stored.Manifest.Nodes)
	}
	for _, node := range stored.Manifest.Nodes {
		if node.ID == "node_foco_entry" || node.Role != "" {
			t.Fatalf("authorial manifest should remain unnormalized, got %#v", stored.Manifest.Nodes)
		}
	}
	if stored.Manifest.Intent.Goal != "enviar correos a la gente que me debe" {
		t.Fatalf("intent=%#v", stored.Manifest.Intent)
	}
}

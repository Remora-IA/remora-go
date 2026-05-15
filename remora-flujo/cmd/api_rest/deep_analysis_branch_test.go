package main

import (
	"context"
	"testing"
)

func deepAnalysisBranchArtifacts() map[string]flowRunArtifact {
	return map[string]flowRunArtifact{
		"data.sqlite_db.v1":         {Type: "data.sqlite_db.v1", Source: "test"},
		"business.semantic_pack.v1": {Type: "business.semantic_pack.v1", Source: "test"},
		"dataset.raw.v1":            {Type: "dataset.raw.v1", Source: "test", Payload: map[string]interface{}{"artifact_type": "dataset.raw.v1"}},
		"collection.priority_list.v1": {
			Type:   "collection.priority_list.v1",
			Source: "test",
			Payload: map[string]interface{}{
				"artifact_type": "collection.priority_list.v1",
				"items": []interface{}{
					map[string]interface{}{"rank": 1, "deudor": "Cliente Uno", "saldo_total": 1000, "dias_mora_max": 45},
				},
				"selected": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "cust_1", "name": "Cliente Uno"},
			},
		},
		"entity.ref.v1": {Type: "entity.ref.v1", Source: "test", Payload: map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "cust_1", "name": "Cliente Uno"}},
		"strategy.recommendation.v1": {Type: "strategy.recommendation.v1", Source: "test", Payload: map[string]interface{}{
			"artifact_type": "strategy.recommendation.v1",
			"recommendations": []interface{}{
				map[string]interface{}{"action_id": "deep_analysis", "label": "Ver análisis profundo", "description": "Investigar antes de actuar"},
			},
		}},
		"action.options.v1": {Type: "action.options.v1", Source: "test", Payload: map[string]interface{}{
			"artifact_type": "action.options.v1",
			"action_options": []interface{}{
				map[string]interface{}{"id": "deep_analysis", "label": "Ver análisis profundo", "description": "Investigar antes de actuar", "bound_id": "escalate"},
			},
		}},
	}
}

func TestDeepAnalysisDimensionDefaultDoesNotSimulateConversation(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()
	t.Setenv("REMORA_DEEP_ANALYSIS_STRESS_TURNS", "6")

	req := collectionSmokeRequest(false, false)
	branches := s.runFlowActionBranches(context.Background(), req, deepAnalysisBranchArtifacts())
	if len(branches) != 1 {
		t.Fatalf("expected one branch, got %#v", branches)
	}
	branch := branches[0]
	if branch.Action["id"] != "deep_analysis" {
		t.Fatalf("expected deep_analysis branch, got %#v", branch.Action)
	}
	for _, step := range branch.Timeline {
		if step.Node == "deep_analysis_simulated_user" || step.Node == "radar_deep_analysis_followup" || step.Node == "analysis_review_pending" {
			t.Fatalf("default branch must not auto-simulate deep analysis conversation, got timeline=%#v", branch.Timeline)
		}
	}
	if containsString(branch.Artifacts, "analysis.simulation.preview.v1") || containsString(branch.Artifacts, "analysis.followup.v1") {
		t.Fatalf("default branch must not persist simulation artifacts, got %#v", branch.Artifacts)
	}
	if path := s.latestFlowArtifactPath(req.Flow.BusinessID, "analysis.simulation.preview.v1"); path != "" {
		t.Fatalf("default branch must not persist analysis.simulation.preview.v1, got %s", path)
	}
}

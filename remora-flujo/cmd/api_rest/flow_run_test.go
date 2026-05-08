package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunFlowManifestDryRunPersistsRunAndArtifacts(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root, allManifests: flowTestManifests()}
	req := flowRunRequest{
		DryRun: true,
		Flow: flowManifest{
			ID:       "staff_dry_run",
			Policies: []string{"approval_required"},
			ProvidedArtifacts: []string{
				"data.sqlite_db.v1",
				"business.semantic_pack.v1",
				"message.draft.v1",
			},
			Nodes: []flowNode{
				{ID: "radar", Framework: "radar", Capability: "collection.priority_list"},
				{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"},
				{ID: "entity", Framework: "sabio", Capability: "data.entity_360"},
				{ID: "smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
				{ID: "send", Framework: "mensajero", Capability: "message.send"},
			},
			Edges: []flowEdge{
				{From: "radar", To: "focus"},
				{From: "focus", To: "entity"},
				{From: "entity", To: "send"},
				{From: "smtp", To: "send"},
			},
		},
	}

	result := s.runFlowManifest(context.Background(), req)
	if result.Status != "completed" || !result.Valid {
		t.Fatalf("status=%s valid=%v errors=%#v", result.Status, result.Valid, result.Validation.Errors)
	}
	if len(result.Timeline) != 5 {
		t.Fatalf("timeline len = %d", len(result.Timeline))
	}
	for _, step := range result.Timeline {
		if step.Status != "would_run" {
			t.Fatalf("expected dry-run step, got %#v", step)
		}
	}
	if !containsString(result.ExecutionOrder, "send") {
		t.Fatalf("missing execution order: %#v", result.ExecutionOrder)
	}
	for _, want := range []string{"collection.priority_list.v1", "focus.next_task.v1", "entity.ref.v1", "entity_360.v1", "message.draft.v1", "credentials.smtp", "message.sent.v1"} {
		if _, ok := result.Artifacts[want]; !ok {
			t.Fatalf("missing artifact %s in %#v", want, result.Artifacts)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "temp", "flow_runs", result.RunID, "run.json")); err != nil {
		t.Fatalf("expected persisted run: %v", err)
	}
}

func TestRunFlowManifestRequiresApprovalForRealSideEffect(t *testing.T) {
	s := &server{rootDir: t.TempDir(), allManifests: flowTestManifests()}
	req := flowRunRequest{
		Flow: flowManifest{
			ID: "real_send",
			ProvidedArtifacts: []string{
				"message.draft.v1",
				"credentials.smtp",
			},
			Nodes: []flowNode{
				{ID: "send", Framework: "mensajero", Capability: "message.send"},
			},
		},
	}

	result := s.runFlowManifest(context.Background(), req)
	if result.Status != "needs_approval" {
		t.Fatalf("status = %q want needs_approval; result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 1 || result.Timeline[0].Status != "awaiting_approval" {
		t.Fatalf("expected awaiting approval step, got %#v", result.Timeline)
	}
}

func TestResolveFlowParamTemplateUsesArtifactPayload(t *testing.T) {
	artifacts := map[string]flowRunArtifact{
		"entity.ref.v1": {
			Type: "entity.ref.v1",
			Payload: map[string]interface{}{
				"id":   "cust_123",
				"type": "customer",
			},
		},
	}

	got, err := resolveFlowParamTemplate("cliente={artifacts.entity.ref.v1.id}", artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "cliente=cust_123" {
		t.Fatalf("got %q", got)
	}
}

func TestRecordNodeArtifactsSplitsSelectedArtifactPayload(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	available := map[string]bool{}
	artifacts := map[string]flowRunArtifact{}
	stdout := `{
		"artifact_type":"collection.priority_list.v1",
		"artifacts":["collection.priority_list.v1","entity.ref.v1"],
		"items":[{"rank":1,"deudor":"Cliente Uno"}],
		"selected":{"artifact_type":"entity.ref.v1","id":"cust_1","name":"Cliente Uno"}
	}`

	s.recordNodeArtifacts("run_1", "priorities", nodeContract{Produces: []string{"collection.priority_list.v1"}}, stdout, available, artifacts)
	entity, ok := artifacts["entity.ref.v1"]
	if !ok {
		t.Fatalf("expected entity.ref.v1 in %#v", artifacts)
	}
	payload, ok := entity.Payload.(map[string]interface{})
	if !ok || payload["id"] != "cust_1" {
		t.Fatalf("unexpected entity payload %#v", entity.Payload)
	}
}

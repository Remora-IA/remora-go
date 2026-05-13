package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"channel/adapter"
	"channel/manifest"
)

func TestRunFlowManifestDryRunExecutesSafeNodesAndStopsBeforeSideEffect(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:    true,
			ExitCode:   0,
			Stdout:     `{"artifact_type":"message.draft.v1","subject":"Prueba real","body":"Contenido real"}`,
			DurationMs: 4,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: dryRunExecutionTestManifests(), channel: adapter.New(channel.URL, "test-key")}
	req := flowRunRequest{
		DryRun: true,
		Flow: flowManifest{
			ID: "staff_dry_run",
			Nodes: []flowNode{
				{ID: "draft", Framework: "safe", Capability: "message.draft.test"},
				{ID: "send", Framework: "sender", Capability: "message.send"},
			},
			Edges: []flowEdge{
				{From: "draft", To: "send"},
			},
		},
	}

	result := s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "needs_approval" || !result.Valid {
		t.Fatalf("status=%s valid=%v errors=%#v timeline=%#v", result.Status, result.Valid, result.Validation.Errors, result.Timeline)
	}
	if len(result.Timeline) != 2 {
		t.Fatalf("timeline len = %d", len(result.Timeline))
	}
	if result.Timeline[0].Status != "completed" {
		t.Fatalf("expected safe node to run, got %#v", result.Timeline[0])
	}
	if result.Timeline[1].Status != "awaiting_approval" {
		t.Fatalf("expected side effect to stop before execution, got %#v", result.Timeline[1])
	}
	draft, ok := result.Artifacts["message.draft.v1"]
	if !ok {
		t.Fatalf("missing real draft artifact in %#v", result.Artifacts)
	}
	if draft.Source != "framework_stdout" {
		t.Fatalf("draft source = %q want framework_stdout", draft.Source)
	}
	payload, ok := draft.Payload.(map[string]interface{})
	if !ok || payload["subject"] != "Prueba real" {
		t.Fatalf("unexpected draft payload %#v", draft.Payload)
	}
	if _, ok := result.Artifacts["message.sent.v1"]; ok {
		t.Fatalf("dry run should not synthesize message.sent.v1: %#v", result.Artifacts["message.sent.v1"])
	}
	if _, err := os.Stat(filepath.Join(root, "temp", "flow_runs", result.RunID, "run.json")); err != nil {
		t.Fatalf("expected persisted run: %v", err)
	}
}

func TestSummarizeAuditorGapsCompactsMissingContactsAndCounts(t *testing.T) {
	artifacts := map[string]flowRunArtifact{
		"data.gaps.v1": {
			Payload: []interface{}{
				map[string]interface{}{"rule": "missing_contact_destination", "endpoint": "clients", "field": "email", "message": "Falta email/contacto operativo: clients[1].email"},
				map[string]interface{}{"rule": "missing_contact_destination", "endpoint": "clients", "field": "email", "message": "Falta email/contacto operativo: clients[2].email"},
				map[string]interface{}{"rule": "empty_required", "endpoint": "agreements", "field": "name", "count": float64(599), "message": "599 registros en agreements con campo name vacío"},
			},
		},
	}
	got := summarizeAuditorGaps(artifacts)
	if !strings.Contains(got, "2 registros en clients: sin email de contacto") {
		t.Fatalf("expected compact clients contact summary, got %q", got)
	}
	if !strings.Contains(got, "599 registros en agreements: campo name vacío") {
		t.Fatalf("expected count-aware agreements summary, got %q", got)
	}
	if strings.Contains(got, "clients[1].email") || strings.Contains(got, "clients[2].email") {
		t.Fatalf("summary should not list individual contact records: %q", got)
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

	result := s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "needs_approval" {
		t.Fatalf("status = %q want needs_approval; result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 1 || result.Timeline[0].Status != "awaiting_approval" {
		t.Fatalf("expected awaiting approval step, got %#v", result.Timeline)
	}
}

func TestRunFlowManifestTestModeExecutesSideEffectAgainstTestRecipient(t *testing.T) {
	t.Setenv("TEST_EMAIL_RECIPIENT", "test-recipient@example.com")
	var capturedArgs []string
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Params map[string]interface{} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if raw, ok := body.Params["args"].([]interface{}); ok {
			for _, item := range raw {
				capturedArgs = append(capturedArgs, fmt.Sprint(item))
			}
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifact_type":"message.sent.v1","message_id":"test_msg","channel":"email"}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: map[string]*manifest.Manifest{
		"sender": {
			Name: "sender",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"send": {
					Args:   []string{"send", "--to", "{params.to}", "--subject", "{params.subject}"},
					Params: []string{"to", "subject"},
				},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "message.send",
				Command:  "send",
				Requires: []string{"message.draft.v1", "contact.destination.v1"},
				Produces: []string{"message.sent.v1"},
				Policies: []string{"external_side_effect", "approval_required"},
			}},
		},
	}}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		TestMode: true,
		Flow: flowManifest{
			ID:                "test_mode_send",
			ProvidedArtifacts: []string{"message.draft.v1", "contact.destination.v1"},
			Nodes:             []flowNode{{ID: "send", Framework: "sender", Capability: "message.send"}},
		},
		InitialArtifacts: map[string]interface{}{
			"message.draft.v1": map[string]interface{}{"artifact_type": "message.draft.v1", "subject": "Cobranza"},
			"contact.destination.v1": map[string]interface{}{
				"artifact_type": "contact.destination.v1",
				"destination":   "real-client@example.com",
			},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s timeline=%#v", result.Status, result.Timeline)
	}
	if !containsString(capturedArgs, "test-recipient@example.com") {
		t.Fatalf("expected test recipient in args, got %#v", capturedArgs)
	}
	if !containsString(capturedArgs, "[TEST → real-client@example.com] Cobranza") {
		t.Fatalf("expected test subject prefix in args, got %#v", capturedArgs)
	}
}

func TestRunFlowManifestUsesInteractiveModeForApprovalPolicy(t *testing.T) {
	s := &server{rootDir: t.TempDir(), allManifests: map[string]*manifest.Manifest{
		"hosting": {
			Name: "hosting",
			Commands: map[string]manifest.Command{
				"import-smtp": {Args: []string{"import-smtp"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "credentials.smtp.import",
				Command:  "import-smtp",
				Requires: []string{"credentials.smtp.input.v1"},
				Produces: []string{"credentials.smtp"},
				Policies: []string{"approval_required", "resolution_interactive"},
			}},
		},
	}}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "interactive_approval",
			ProvidedArtifacts: []string{"credentials.smtp.input.v1"},
			Nodes:             []flowNode{{ID: "smtp", Framework: "hosting", Capability: "credentials.smtp.import"}},
		},
	}, nil)
	if result.Status != "needs_approval" {
		t.Fatalf("status = %q want needs_approval; result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 1 || result.Timeline[0].Status != "awaiting_approval" {
		t.Fatalf("expected awaiting approval, got %#v", result.Timeline)
	}
	if result.Timeline[0].ResolutionMode != resolutionInteractive {
		t.Fatalf("resolution mode = %q want interactive", result.Timeline[0].ResolutionMode)
	}
}

func TestRunFlowManifestUsesHybridModeForStateMutation(t *testing.T) {
	s := &server{rootDir: t.TempDir(), allManifests: map[string]*manifest.Manifest{
		"mecanico": {
			Name: "mecanico",
			Commands: map[string]manifest.Command{
				"apply": {Args: []string{"apply"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "action.fix.apply",
				Command:  "apply",
				Requires: []string{"mecanico.proposal.v1"},
				Produces: []string{"mecanico.applied.v1"},
				Policies: []string{"state_mutation", "approval_required", "resolution_hybrid"},
			}},
		},
	}}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "hybrid_mutation",
			ProvidedArtifacts: []string{"mecanico.proposal.v1"},
			Nodes:             []flowNode{{ID: "apply", Framework: "mecanico", Capability: "action.fix.apply"}},
		},
	}, nil)
	if result.Status != "needs_approval" {
		t.Fatalf("status = %q want needs_approval; result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 1 || result.Timeline[0].Status != "awaiting_approval" {
		t.Fatalf("expected awaiting approval, got %#v", result.Timeline)
	}
	if result.Timeline[0].ResolutionMode != resolutionHybrid {
		t.Fatalf("resolution mode = %q want hybrid", result.Timeline[0].ResolutionMode)
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

func TestRecordNodeArtifactsDoesNotSynthesizeDataRequestFromContract(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	available := map[string]bool{}
	artifacts := map[string]flowRunArtifact{}
	stdout := `{
		"artifact_type":"collection.priority_list.v1",
		"artifacts":["collection.priority_list.v1","entity.ref.v1"],
		"selected":{"artifact_type":"entity.ref.v1","id":"cust_1","name":"Cliente Uno"}
	}`

	types := s.recordNodeArtifacts("run_1", "priorities", nodeContract{Produces: []string{"collection.priority_list.v1", "data.request.v1"}}, stdout, available, artifacts)
	if containsString(types, "data.request.v1") || available["data.request.v1"] {
		t.Fatalf("data.request.v1 should only exist when stdout declares request, types=%#v artifacts=%#v", types, artifacts)
	}
}

func TestRunFlowManifestSkipsInstalledRadarAnalysis(t *testing.T) {
	root := t.TempDir()
	planPath := filepath.Join(root, "framework-radar", "temp", "radar", "biz-1", "collection_analysis_plan.json")
	if err := os.MkdirAll(filepath.Dir(planPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte(`{"model":{"entity_table":"clients","item_table":"charges"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"focus.briefing.v1"}`})
	}))
	defer channel.Close()
	store, err := openFlowStore(filepath.Join(root, "flows.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	fm := flowManifest{
		ID:                "flow_collection",
		BusinessID:        "biz-1",
		ProvidedArtifacts: []string{"business.semantic_pack.v1"},
		Nodes:             []flowNode{{ID: "configure", Framework: "radar", Capability: "analysis.configure", Role: flowRoleBootstrap}},
	}
	created, err := store.createFlow("Cobranza", "", "biz-1", &fm)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.updateFlowStatus(created.ID, "installed"); err != nil {
		t.Fatal(err)
	}
	s := &server{rootDir: root, flows: store, channel: adapter.New(channel.URL, "test-key"), allManifests: map[string]*manifest.Manifest{
		"radar": {
			Name: "radar", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{"configure-analysis": {Args: []string{"-c", "exit 99"}}},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "analysis.configure",
				Command:  "configure-analysis",
				Requires: []string{"business.semantic_pack.v1"},
				Produces: []string{"analysis.schema.v1", "analysis.plan.v1"},
				Policies: []string{"install_once"},
			}},
		},
		"foco": {
			Name: "foco", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{"session-start": {Args: []string{"-c", "true"}}},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "focus.entry_briefing",
				Command:  "session-start",
				Requires: []string{"session.context.v1"},
				Produces: []string{"focus.briefing.v1"},
			}},
		},
	}}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: fm,
	}, nil)

	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.Timeline) == 0 || result.Timeline[0].Status != "skipped" {
		t.Fatalf("expected installed analysis skip, timeline=%#v", result.Timeline)
	}
	if _, ok := result.Artifacts["flow.installation.v1"]; !ok {
		t.Fatalf("missing flow.installation.v1 in %#v", result.Artifacts)
	}
}

func TestRunFlowManifestPausesForRadarAnalysisAcceptance(t *testing.T) {
	root := t.TempDir()
	var calls int
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		stdout := `{"artifact_type":"analysis.schema.v1","artifacts":["analysis.schema.v1","analysis.proposal.v1"],"text":"Radar propone 40/30/30"}`
		if calls == 3 {
			stdout = `{"artifact_type":"collection.priority_list.v1","artifacts":["collection.priority_list.v1"],"items":[]}`
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: root, channel: adapter.New(channel.URL, "test-key"), allManifests: map[string]*manifest.Manifest{
		"radar": {
			Name: "radar", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"configure-analysis": {Args: []string{"-c", "radar"}},
				"prioritize":         {Args: []string{"-c", "prioritize"}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "analysis.configure", Command: "configure-analysis", Requires: []string{"business.semantic_pack.v1"}, Produces: []string{"analysis.schema.v1", "analysis.proposal.v1"}, Policies: []string{"human_acceptance_before_continue"}},
				{ID: "collection.priority_list", Command: "prioritize", Requires: []string{"business.semantic_pack.v1"}, Produces: []string{"collection.priority_list.v1"}},
			},
		},
	}}
	req := flowRunRequest{Flow: flowManifest{
		ID:                "bootstrap_gate",
		ProvidedArtifacts: []string{"business.semantic_pack.v1"},
		Nodes: []flowNode{
			{ID: "configure", Framework: "radar", Capability: "analysis.configure", Role: flowRoleBootstrap},
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list", Role: flowRolePipeline},
		},
	}}

	result := s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.NeedsInput) == 0 || result.NeedsInput[0].Artifact != "analysis.accepted.v1" {
		t.Fatalf("expected analysis acceptance need, got %#v", result.NeedsInput)
	}
	if result.NeedsInput[0].Node != "configure" || result.NeedsInput[0].Visibility != flowStepVisibilityUserFacing {
		t.Fatalf("expected analysis acceptance anchored to configure, got %#v", result.NeedsInput[0])
	}
	if _, ok := result.Artifacts["collection.priority_list.v1"]; ok {
		t.Fatalf("prioritize should not run before acceptance: %#v", result.Artifacts)
	}

	req.InitialArtifacts = map[string]interface{}{"analysis.accepted.v1": map[string]interface{}{"accepted": true}}
	result = s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "completed" {
		t.Fatalf("accepted status=%s result=%#v", result.Status, result)
	}
	if _, ok := result.Artifacts["collection.priority_list.v1"]; !ok {
		t.Fatalf("expected priority list after acceptance: %#v", result.Artifacts)
	}
}

func TestInstallFlowAnalysisRunsRadarAndMarksInstalled(t *testing.T) {
	root := t.TempDir()
	packPath := filepath.Join(root, "framework-sabio", "businesses", "biz-1", "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(packPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte(`{"business_id":"biz-1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifact_type":"analysis.schema.v1","artifacts":["analysis.schema.v1","analysis.plan.v1"],"plan_path":"temp/radar/biz-1/collection_analysis_plan.json"}`,
		})
	}))
	defer channel.Close()
	store, err := openFlowStore(filepath.Join(root, "flows.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	s := &server{rootDir: root, flows: store, channel: adapter.New(channel.URL, "test-key"), allManifests: map[string]*manifest.Manifest{
		"radar": {
			Name: "radar", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{"configure-analysis": {Args: []string{"-c", "radar"}, Params: []string{"business_id", "semantic_pack", "db"}, Defaults: map[string]string{"db": ""}}},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "analysis.configure",
				Command:  "configure-analysis",
				Requires: []string{"business.semantic_pack.v1"},
				Produces: []string{"analysis.schema.v1", "analysis.plan.v1"},
				Policies: []string{"install_once", "human_acceptance_before_continue"},
			}},
		},
	}}
	created, err := store.createFlow("Cobranza", "Cobranza", "biz-1", &flowManifest{Nodes: []flowNode{{ID: "configure", Framework: "radar", Capability: "analysis.configure"}}})
	if err != nil {
		t.Fatal(err)
	}
	flow, err := store.getFlow(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.installFlowAnalysis(context.Background(), flow, flowInstallOptions{})
	if err != nil {
		t.Fatalf("installFlowAnalysis: %v", err)
	}
	if result.Status != "installed" || result.ArtifactType != "flow.installation.v1" {
		t.Fatalf("unexpected result %#v", result)
	}
	updated, err := store.getFlow(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "installed" {
		t.Fatalf("flow status=%s want installed", updated.Status)
	}
}

func TestFlowInstalledSnapshotReadsRadarSchema(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "framework-radar", "temp", "radar", "biz-1")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "collection_analysis_plan.json"), []byte(`{"artifact_type":"analysis.plan.v1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "collection_analysis_schema.json"), []byte(`{"schema_id":"collection_priority_40_30_30_v1","updated_at":"2026-05-10T00:00:00Z","weights":{"materialidad":40,"comportamiento":30}}`), 0644); err != nil {
		t.Fatal(err)
	}
	s := &server{rootDir: root}
	snapshot := s.flowInstalledSnapshot("biz-1")
	if snapshot == nil || !snapshot.Installed {
		t.Fatalf("expected installed snapshot, got %#v", snapshot)
	}
	if snapshot.SchemaID != "collection_priority_40_30_30_v1" {
		t.Fatalf("unexpected schema id %#v", snapshot)
	}
	if snapshot.Weights["materialidad"].(float64) != 40 {
		t.Fatalf("unexpected weights %#v", snapshot.Weights)
	}
}

func TestRunFlowManifestEmitsFlowIntentArtifact(t *testing.T) {
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"intent.consumer.v1"}`})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: flowIntentTestManifests(false)}
	result := s.runFlowManifest(context.Background(), flowRunRequest{Flow: flowManifest{
		ID: "postventa_whatsapp",
		Intent: flowIntent{
			Goal:            "hacer seguimiento post-venta",
			OperatorRole:    "vendedor",
			SuccessCriteria: "respuesta del cliente registrada",
			Constraints:     []string{"no contactar fuera de horario laboral"},
		},
		Nodes: []flowNode{{ID: "first", Framework: "consumer", Capability: "intent.consume"}},
	}}, nil)

	artifact, ok := result.Artifacts["flow.intent.v1"]
	if !ok {
		t.Fatalf("expected flow.intent.v1 in artifacts: %#v", result.Artifacts)
	}
	payload, ok := artifact.Payload.(map[string]interface{})
	if !ok || payload["goal"] != "hacer seguimiento post-venta" || payload["operator_role"] != "vendedor" {
		t.Fatalf("unexpected intent payload %#v", artifact.Payload)
	}
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
}

func TestRunFlowManifestWithoutIntentDoesNotEmitFlowIntentArtifact(t *testing.T) {
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"intent.consumer.v1"}`})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: flowIntentTestManifests(false)}
	result := s.runFlowManifest(context.Background(), flowRunRequest{Flow: flowManifest{
		ID:    "sin_intent",
		Nodes: []flowNode{{ID: "first", Framework: "consumer", Capability: "intent.consume"}},
	}}, nil)

	if _, ok := result.Artifacts["flow.intent.v1"]; ok {
		t.Fatalf("did not expect flow.intent.v1 in %#v", result.Artifacts["flow.intent.v1"])
	}
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
}

func TestFlowIntentAvailableToFirstNode(t *testing.T) {
	var args []string
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Params struct {
				Args []string `json:"args"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		args = append([]string{}, rpc.Params.Args...)
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"intent.consumer.v1"}`})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: flowIntentTestManifests(true)}
	result := s.runFlowManifest(context.Background(), flowRunRequest{Flow: flowManifest{
		ID: "intent_first_node",
		Intent: flowIntent{
			Goal:         "actualizar CRM",
			OperatorRole: "soporte",
		},
		Nodes: []flowNode{{ID: "first", Framework: "consumer", Capability: "intent.consume", Params: map[string]string{"goal": "{artifacts.flow.intent.v1.goal}"}}},
	}}, nil)

	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if !containsString(args, "actualizar CRM") {
		t.Fatalf("first node did not receive intent goal, args=%#v", args)
	}
}

func flowIntentTestManifests(requireIntent bool) map[string]*manifest.Manifest {
	requires := []string{}
	args := []string{"-c", "true"}
	params := []string{}
	if requireIntent {
		requires = []string{"flow.intent.v1"}
		args = []string{"--goal", "{params.goal}"}
		params = []string{"goal"}
	}
	return map[string]*manifest.Manifest{
		"consumer": {
			Name: "consumer",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"consume": {
					Args:   args,
					Params: params,
				},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:       "intent.consume",
					Command:  "consume",
					Requires: requires,
					Produces: []string{"intent.consumer.v1"},
				},
			},
		},
	}
}

func TestWorkContextCreatedAfterEntryNode(t *testing.T) {
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: workContextEntryStdout("task_1", "cust_1")})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:     "work_context_flow",
			Intent: flowIntent{SuccessCriteria: "email enviado"},
			Nodes:  []flowNode{{ID: "entry", Framework: "entry", Capability: "focus.next", Role: flowRoleEntry}},
		},
		InitialArtifacts: map[string]interface{}{
			"action.selection.v1": map[string]interface{}{"artifact_type": "action.selection.v1", "id": "send_email", "label": "Enviar email"},
		},
	}, nil)

	artifact, ok := result.Artifacts["work.context.v1"]
	if !ok {
		t.Fatalf("expected work.context.v1 in artifacts: %#v", result.Artifacts)
	}
	payload, ok := artifact.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected work.context payload %#v", artifact.Payload)
	}
	if payload["task_id"] != "task_1" || payload["entity_ref"] != "cust_1" {
		t.Fatalf("unexpected work.context payload %#v", payload)
	}
}

func TestWorkContextContainsTaskEntityAndSelectedAction(t *testing.T) {
	var pipelineArgs []string
	call := 0
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		var rpc struct {
			Params struct {
				Args []string `json:"args"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		if len(rpc.Params.Args) > 0 && rpc.Params.Args[0] == "pipeline" {
			pipelineArgs = append([]string{}, rpc.Params.Args...)
		}
		stdout := `{"artifact_type":"pipeline.done.v1"}`
		if call == 1 {
			stdout = workContextEntryStdout("task_1", "cust_1")
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextCycleTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:         "work_context_pipeline",
			BusinessID: "biz_1",
			Intent:     flowIntent{SuccessCriteria: "mensaje enviado"},
			Edges:      []flowEdge{{From: "entry", To: "pipeline"}},
			Nodes: []flowNode{
				{ID: "entry", Framework: "entry", Capability: "focus.next", Role: flowRoleEntry},
				{ID: "pipeline", Framework: "pipeline", Capability: "pipeline.consume", Params: map[string]string{"task_id": "{artifacts.work.context.v1.task_id}"}},
			},
		},
		InitialArtifacts: map[string]interface{}{
			"action.selection.v1": map[string]interface{}{"artifact_type": "action.selection.v1", "id": "send_email", "label": "Enviar email"},
		},
	}, nil)

	payload := result.Artifacts["work.context.v1"].Payload.(map[string]interface{})
	selected, ok := payload["selected_action"].(map[string]interface{})
	if !ok || selected["id"] != "send_email" {
		t.Fatalf("unexpected selected action %#v in %#v", payload["selected_action"], payload)
	}
	if payload["expected_outcome"] != "mensaje enviado" {
		t.Fatalf("unexpected expected_outcome %#v", payload)
	}
	if !containsString(pipelineArgs, "task_1") {
		t.Fatalf("pipeline did not receive work.context task_id, args=%#v", pipelineArgs)
	}
}

func TestWorkContextResetsBetweenCycles(t *testing.T) {
	entryCalls := 0
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Params struct {
				Args []string `json:"args"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		stdout := `{"artifact_type":"message.sent.v1","message_id":"msg_1"}`
		if len(rpc.Params.Args) > 0 && rpc.Params.Args[0] == "entry" {
			entryCalls++
			stdout = workContextEntryStdoutNoActions(fmt.Sprintf("task_%d", entryCalls), fmt.Sprintf("cust_%d", entryCalls))
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "work_context_cycles",
			Edges: []flowEdge{{From: "entry", To: "terminal"}},
			Nodes: []flowNode{
				{ID: "entry", Framework: "entry", Capability: "focus.next", Role: flowRoleEntry},
				{ID: "terminal", Framework: "terminal", Capability: "message.send"},
			},
		},
		MaxCycles: 2,
	}, nil)

	payload := result.Artifacts["work.context.v1"].Payload.(map[string]interface{})
	if payload["task_id"] != "task_2" || payload["entity_ref"] != "cust_2" {
		t.Fatalf("work.context should reflect second cycle only, got %#v", payload)
	}
	if _, ok := payload["selected_action"]; ok {
		t.Fatalf("work.context should not infer selected action from options: %#v", payload)
	}
}

func TestWorkContextNotCreatedWithoutEntryTaskContext(t *testing.T) {
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"pipeline.done.v1"}`})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "no_entry_context",
			Nodes: []flowNode{{ID: "pipeline", Framework: "pipeline", Capability: "pipeline.consume"}},
		},
	}, nil)

	if _, ok := result.Artifacts["work.context.v1"]; ok {
		t.Fatalf("did not expect work.context.v1, got %#v", result.Artifacts["work.context.v1"])
	}
}

func TestCycleResultCreatedWhenCycleCompletes(t *testing.T) {
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Params struct {
				Args []string `json:"args"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		stdout := `{"artifact_type":"message.sent.v1","message_id":"msg_1","to":"cliente@example.com","channel":"email","summary":"Email enviado al cliente"}`
		if len(rpc.Params.Args) > 0 && rpc.Params.Args[0] == "entry" {
			stdout = workContextEntryStdoutNoActions("task_1", "cust_1")
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextCycleTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "cycle_result_flow",
			Edges: []flowEdge{{From: "entry", To: "terminal"}},
			Nodes: []flowNode{
				{ID: "entry", Framework: "entry", Capability: "focus.next", Role: flowRoleEntry},
				{ID: "terminal", Framework: "terminal", Capability: "message.send"},
			},
		},
	}, nil)

	artifact, ok := result.Artifacts["cycle.result.v1"]
	if !ok {
		t.Fatalf("expected cycle.result.v1 in %#v", result.Artifacts)
	}
	payload, ok := artifact.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected cycle.result payload %#v", artifact.Payload)
	}
	if payload["status"] != "done" {
		t.Fatalf("expected done status, got %#v", payload)
	}
	if payload["task_id"] != "task_1" || payload["entity_ref"] != "cust_1" || payload["completed_by_capability"] != "message.send" {
		t.Fatalf("unexpected cycle.result context %#v", payload)
	}
	evidence, ok := payload["evidence"].(map[string]interface{})
	if !ok || evidence["message_id"] != "msg_1" || evidence["to"] != "cliente@example.com" {
		t.Fatalf("unexpected cycle.result evidence %#v in %#v", payload["evidence"], payload)
	}
	if !containsString(result.Timeline[len(result.Timeline)-1].ArtifactTypes, "cycle.result.v1") {
		t.Fatalf("terminal step should include cycle.result.v1: %#v", result.Timeline)
	}
}

func TestCycleResultResetsBetweenCycles(t *testing.T) {
	entryCalls := 0
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Params struct {
				Args []string `json:"args"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		stdout := fmt.Sprintf(`{"artifact_type":"message.sent.v1","message_id":"msg_%d","to":"cliente_%d@example.com","channel":"email"}`, entryCalls, entryCalls)
		if len(rpc.Params.Args) > 0 && rpc.Params.Args[0] == "entry" {
			entryCalls++
			stdout = workContextEntryStdoutNoActions(fmt.Sprintf("task_%d", entryCalls), fmt.Sprintf("cust_%d", entryCalls))
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextCycleTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "cycle_result_cycles",
			Edges: []flowEdge{{From: "entry", To: "terminal"}},
			Nodes: []flowNode{
				{ID: "entry", Framework: "entry", Capability: "focus.next", Role: flowRoleEntry},
				{ID: "terminal", Framework: "terminal", Capability: "message.send"},
			},
		},
		MaxCycles: 2,
	}, nil)

	payload := result.Artifacts["cycle.result.v1"].Payload.(map[string]interface{})
	if payload["cycle_index"] != float64(1) && payload["cycle_index"] != 1 {
		t.Fatalf("expected second cycle index, got %#v", payload)
	}
	if payload["task_id"] != "task_2" || payload["entity_ref"] != "cust_2" {
		t.Fatalf("cycle.result should reflect second cycle only, got %#v", payload)
	}
	evidence, _ := payload["evidence"].(map[string]interface{})
	if evidence["message_id"] != "msg_2" {
		t.Fatalf("cycle.result evidence should reflect second cycle, got %#v", payload)
	}
}

func TestCycleResultNotCreatedWithoutCycleTerminalStep(t *testing.T) {
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"pipeline.done.v1"}`})
	}))
	defer channel.Close()
	s := &server{rootDir: t.TempDir(), channel: adapter.New(channel.URL, "test-key"), allManifests: workContextTestManifests()}
	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "cycle_result_no_terminal",
			Nodes: []flowNode{{ID: "pipeline", Framework: "pipeline", Capability: "pipeline.consume"}},
		},
	}, nil)

	if _, ok := result.Artifacts["cycle.result.v1"]; ok {
		t.Fatalf("did not expect cycle.result.v1, got %#v", result.Artifacts["cycle.result.v1"])
	}
}

func workContextTestManifests() map[string]*manifest.Manifest {
	return map[string]*manifest.Manifest{
		"entry": {
			Name: "entry",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"next": {Args: []string{"entry"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "focus.next",
				Command:  "next",
				Produces: []string{"focus.next_task.v1", "task.next", "entity.ref.v1", "action.options.v1"},
			}},
		},
		"pipeline": {
			Name: "pipeline",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"consume": {Args: []string{"pipeline", "{params.task_id}"}, Params: []string{"task_id"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "pipeline.consume",
				Command:  "consume",
				Requires: []string{"work.context.v1"},
				Produces: []string{"pipeline.done.v1"},
			}},
		},
		"terminal": {
			Name: "terminal",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"send": {Args: []string{"terminal"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "message.send",
				Command:  "send",
				Produces: []string{"message.sent.v1"},
				Policies: []string{"cycle_terminal"},
			}},
		},
	}
}

func workContextCycleTestManifests() map[string]*manifest.Manifest {
	manifests := workContextTestManifests()
	entry := *manifests["entry"]
	entry.Capabilities = []manifest.CapabilitySpec{{
		ID:       "focus.next",
		Command:  "next",
		Produces: []string{"focus.next_task.v1", "task.next", "entity.ref.v1"},
	}}
	manifests["entry"] = &entry
	return manifests
}

func workContextEntryStdout(taskID, entityID string) string {
	return fmt.Sprintf(`{
		"artifact_type":"focus.next_task.v1",
		"artifacts":["focus.next_task.v1","task.next","entity.ref.v1","action.options.v1"],
		"task_id":%q,
		"task":{"id":%q,"title":"Contactar cliente","expected":"mensaje enviado"},
		"selected":{"artifact_type":"entity.ref.v1","type":"client","id":%q,"name":"Cliente Uno"},
		"action_options":[{"id":"send_email","label":"Enviar email","description":"Contactar por email"}]
	}`, taskID, taskID, entityID)
}

func workContextEntryStdoutNoActions(taskID, entityID string) string {
	return fmt.Sprintf(`{
		"artifact_type":"focus.next_task.v1",
		"artifacts":["focus.next_task.v1","task.next","entity.ref.v1"],
		"task_id":%q,
		"task":{"id":%q,"title":"Contactar cliente","expected":"mensaje enviado"},
		"selected":{"artifact_type":"entity.ref.v1","type":"client","id":%q,"name":"Cliente Uno"}
	}`, taskID, taskID, entityID)
}

func dryRunExecutionTestManifests() map[string]*manifest.Manifest {
	return map[string]*manifest.Manifest{
		"safe": {
			Name: "safe",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"draft": {
					Args: []string{"-c", `printf '\173"artifact_type":"message.draft.v1","subject":"Prueba real","body":"Contenido real"\175'`},
				},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:       "message.draft.test",
					Command:  "draft",
					Produces: []string{"message.draft.v1"},
					Policies: []string{"no_external_side_effect"},
				},
			},
		},
		"sender": {
			Name: "sender",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"send": {
					Args: []string{"-c", `printf '\173"artifact_type":"message.sent.v1"\175'`},
				},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:       "message.send",
					Command:  "send",
					Requires: []string{"message.draft.v1"},
					Produces: []string{"message.sent.v1"},
					Policies: []string{"external_side_effect", "approval_required"},
				},
			},
		},
	}
}

func TestRunFlowManifestEmitsReadinessForMissingContact(t *testing.T) {
	s := &server{rootDir: t.TempDir(), allManifests: map[string]*manifest.Manifest{
		"sender": {
			Name: "sender",
			Commands: map[string]manifest.Command{
				"prepare": {Args: []string{"prepare"}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:       "message.prepare",
					Command:  "prepare",
					Requires: []string{"contact.destination.v1"},
					Produces: []string{"message.draft.v1"},
				},
			},
		},
	}}
	req := flowRunRequest{
		Flow: flowManifest{
			ID: "missing_contact",
			ProvidedArtifacts: []string{
				"entity.ref.v1",
			},
			Nodes: []flowNode{{ID: "prepare", Framework: "sender", Capability: "message.prepare"}},
		},
		InitialArtifacts: map[string]interface{}{
			"entity.ref.v1": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "customer", "id": "184", "name": "Thiel-Effertz"},
		},
	}

	result := s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status = %q want needs_input; result=%#v", result.Status, result)
	}
	if len(result.NeedsInput) != 1 || result.NeedsInput[0].Kind != "conversational_question" {
		t.Fatalf("unexpected needs_input %#v", result.NeedsInput)
	}
	// The JIT pipeline asks via Mecanico's conversational gap resolver, not Sabio directly.
	ni := result.NeedsInput[0]
	if ni.Artifact != "contact.destination.v1" {
		t.Fatalf("expected artifact contact.destination.v1, got %q in %#v", ni.Artifact, ni)
	}
	if ni.EntityRef != "184" && (ni.Context == nil || ni.Context["entity_ref"] != "184") {
		t.Fatalf("entity context missing in %#v", ni)
	}
	readiness, ok := result.Artifacts["flow.readiness.v1"]
	if !ok {
		t.Fatalf("missing flow.readiness.v1 in %#v", result.Artifacts)
	}
	payload, ok := readiness.Payload.(map[string]interface{})
	if !ok || payload["ready"] != false {
		t.Fatalf("unexpected readiness payload %#v", readiness.Payload)
	}
	blockers, ok := payload["blockers"].([]map[string]interface{})
	if !ok || len(blockers) != 1 || blockers[0]["required_artifact"] != "contact.destination.v1" {
		t.Fatalf("unexpected blockers %#v", payload["blockers"])
	}
}

func TestRunFlowManifestMediatesSQLiteThroughSabioBeforePipeline(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifacts":["dataset.raw.v1","external.api.dump.v1"],"rows":[{"id":"cust_1"}]}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"sabio": {
			Name: "sabio",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"dataset-export": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "dataset.export",
				Command:  "dataset-export",
				Inputs:   []string{"data.sqlite_db.v1"},
				Requires: []string{"data.sqlite_db.v1"},
				Produces: []string{"dataset.raw.v1", "external.api.dump.v1"},
			}},
		},
		"radar": {
			Name: "radar",
			Cwd:  ".",
			Binary: manifest.BinarySpec{
				Command: "/bin/sh",
			},
			Commands: map[string]manifest.Command{
				"rank": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "collection.rank",
				Command:  "rank",
				Requires: []string{"dataset.raw.v1"},
				Produces: []string{"collection.priority_list.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "sabio_mediation",
			ProvidedArtifacts: []string{"data.sqlite_db.v1"},
			Nodes: []flowNode{
				{ID: "rank", Framework: "radar", Capability: "collection.rank", Role: flowRolePipeline},
			},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if _, ok := result.Artifacts["external.api.dump.v1"]; !ok {
		t.Fatalf("missing mediated external.api.dump.v1 in %#v", result.Artifacts)
	}
	if len(result.Timeline) != 2 || result.Timeline[0].Node != "sabio_data_mediation" || result.Timeline[0].Framework != "sabio" {
		t.Fatalf("expected Sabio mediation before pipeline, got %#v", result.Timeline)
	}
	if result.Timeline[0].Visibility != flowStepVisibilityInfrastructure || result.Timeline[0].TriggeredBy == nil || result.Timeline[0].TriggeredBy.Node != "rank" {
		t.Fatalf("expected infrastructure mediation tied to rank, got %#v", result.Timeline[0])
	}
	if len(result.DynamicNodes) != 1 || result.DynamicNodes[0].ID != "sabio_data_mediation" || result.DynamicNodes[0].Role != flowRoleResolution {
		t.Fatalf("expected Sabio mediation as dynamic node, got %#v", result.DynamicNodes)
	}
	if len(result.ExecutionOrder) < 2 || result.ExecutionOrder[0] != "sabio_data_mediation" || result.ExecutionOrder[1] != "rank" {
		t.Fatalf("expected dynamic execution order, got %#v", result.ExecutionOrder)
	}
	if result.Timeline[1].Node != "rank" || result.Timeline[1].Status != "completed" {
		t.Fatalf("expected pipeline node to complete after mediation, got %#v", result.Timeline)
	}
}

func TestRunFlowManifestMediatesSQLiteThroughSabioBeforeBootstrap(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifacts":["dataset.raw.v1","external.api.dump.v1"],"rows":[{"id":"cust_1"}]}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"sabio": {
			Name:   "sabio",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"dataset-export": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "dataset.export",
				Command:  "dataset-export",
				Inputs:   []string{"data.sqlite_db.v1"},
				Requires: []string{"data.sqlite_db.v1"},
				Produces: []string{"dataset.raw.v1", "external.api.dump.v1"},
			}},
		},
		"radar": {
			Name:   "radar",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"rank": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "collection.rank",
				Command:  "rank",
				Requires: []string{"dataset.raw.v1"},
				Produces: []string{"collection.priority_list.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "sabio_mediation_bootstrap",
			ProvidedArtifacts: []string{"data.sqlite_db.v1"},
			Nodes: []flowNode{
				{ID: "rank", Framework: "radar", Capability: "collection.rank", Role: flowRoleBootstrap},
			},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 2 || result.Timeline[0].Node != "sabio_data_mediation" || result.Timeline[1].Node != "rank" {
		t.Fatalf("expected Sabio mediation before bootstrap rank, got %#v", result.Timeline)
	}
}

func TestRunFlowManifestResolvesDataRequestThroughSabioAndRerunsNode(t *testing.T) {
	root := t.TempDir()
	var radarCalls int32
	var sabioCalls int32
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		args, _ := params["args"].([]interface{})
		marker := ""
		for _, arg := range args {
			if s, _ := arg.(string); s == "radar" || s == "sabio" {
				marker = s
				break
			}
		}
		switch marker {
		case "sabio":
			atomic.AddInt32(&sabioCalls, 1)
			_ = json.NewEncoder(w).Encode(adapter.Response{
				Success:  true,
				ExitCode: 0,
				Stdout:   `{"artifacts":["dataset.raw.v1","external.api.dump.v1"],"tables":{"clients":[{"id":"cust_1"}]}}`,
			})
		case "radar":
			call := atomic.AddInt32(&radarCalls, 1)
			if call == 1 {
				_ = json.NewEncoder(w).Encode(adapter.Response{
					Success:  true,
					ExitCode: 0,
					Stdout: `{
						"artifact_type":"collection.priority_list.v1",
						"artifacts":["collection.priority_list.v1","data.request.v1"],
						"items":[],
						"request":{"artifact_type":"data.request.v1","target":"sabio","capability":"dataset.export","reason":"necesito dataset canónico"}
					}`,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(adapter.Response{
				Success:  true,
				ExitCode: 0,
				Stdout: `{
					"artifact_type":"collection.priority_list.v1",
					"artifacts":["collection.priority_list.v1","entity.ref.v1"],
					"items":[{"rank":1,"name":"Cliente Uno"}],
					"selected":{"artifact_type":"entity.ref.v1","type":"client","id":"cust_1","name":"Cliente Uno"}
				}`,
			})
		default:
			t.Fatalf("unexpected command args: %#v", args)
		}
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"sabio": {
			Name:   "sabio",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"dataset-export": {Args: []string{"-c", "sabio"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "dataset.export",
				Command:  "dataset-export",
				Requires: []string{"data.sqlite_db.v1"},
				Produces: []string{"dataset.raw.v1", "external.api.dump.v1"},
			}},
		},
		"radar": {
			Name:   "radar",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"rank": {Args: []string{"-c", "radar"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "collection.rank",
				Command:  "rank",
				Produces: []string{"collection.priority_list.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "data_request",
			ProvidedArtifacts: []string{"data.sqlite_db.v1"},
			Nodes:             []flowNode{{ID: "rank", Framework: "radar", Capability: "collection.rank"}},
		},
	}, nil)

	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if got := atomic.LoadInt32(&radarCalls); got != 2 {
		t.Fatalf("expected radar to run twice, got %d timeline=%#v", got, result.Timeline)
	}
	if got := atomic.LoadInt32(&sabioCalls); got != 1 {
		t.Fatalf("expected sabio to run once, got %d timeline=%#v", got, result.Timeline)
	}
	if _, ok := result.Artifacts["dataset.raw.v1"]; !ok {
		t.Fatalf("missing dataset.raw.v1 after data request: %#v", result.Artifacts)
	}
	if _, ok := result.Artifacts["entity.ref.v1"]; !ok {
		t.Fatalf("missing rerun entity.ref.v1: %#v", result.Artifacts)
	}
	if len(result.DynamicNodes) != 2 || result.DynamicNodes[0].ID != "sabio_data_mediation" || result.DynamicNodes[1].ID != "rank_after_data_request_1" {
		t.Fatalf("expected sabio mediation and rerun dynamic nodes, got %#v", result.DynamicNodes)
	}
}

func TestRunFlowManifestResolvesMultipleDataRequestsThroughSabio(t *testing.T) {
	root := t.TempDir()
	var radarCalls int32
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		args, _ := params["args"].([]interface{})
		marker := ""
		for _, arg := range args {
			if s, _ := arg.(string); s == "radar" || s == "sabio" {
				marker = s
				break
			}
		}
		if marker == "sabio" {
			_ = json.NewEncoder(w).Encode(adapter.Response{
				Success:  true,
				ExitCode: 0,
				Stdout:   `{"artifacts":["dataset.raw.v1","external.api.dump.v1"],"tables":{"clients":[{"id":"cust_1"}]}}`,
			})
			return
		}
		if marker != "radar" {
			t.Fatalf("unexpected command args: %#v", args)
		}
		call := atomic.AddInt32(&radarCalls, 1)
		if call <= 2 {
			_ = json.NewEncoder(w).Encode(adapter.Response{
				Success:  true,
				ExitCode: 0,
				Stdout: `{
					"artifact_type":"collection.priority_list.v1",
					"artifacts":["collection.priority_list.v1","data.request.v1"],
					"items":[],
					"request":{"artifact_type":"data.request.v1","target":"sabio","capability":"dataset.export","reason":"necesito otra vista del dataset"}
				}`,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout: `{
				"artifact_type":"collection.priority_list.v1",
				"artifacts":["collection.priority_list.v1","entity.ref.v1"],
				"selected":{"artifact_type":"entity.ref.v1","type":"client","id":"cust_1","name":"Cliente Uno"}
			}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"sabio": {
			Name:   "sabio",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"dataset-export": {Args: []string{"-c", "sabio"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "dataset.export",
				Command:  "dataset-export",
				Requires: []string{"data.sqlite_db.v1"},
				Produces: []string{"dataset.raw.v1", "external.api.dump.v1"},
			}},
		},
		"radar": {
			Name:   "radar",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"rank": {Args: []string{"-c", "radar"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "collection.rank",
				Command:  "rank",
				Produces: []string{"collection.priority_list.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "multi_data_request",
			ProvidedArtifacts: []string{"data.sqlite_db.v1"},
			Nodes:             []flowNode{{ID: "rank", Framework: "radar", Capability: "collection.rank"}},
		},
	}, nil)

	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if got := atomic.LoadInt32(&radarCalls); got != 3 {
		t.Fatalf("expected radar to run three times, got %d timeline=%#v", got, result.Timeline)
	}
	if _, ok := result.Artifacts["entity.ref.v1"]; !ok {
		t.Fatalf("missing final entity.ref.v1: %#v", result.Artifacts)
	}
	if _, ok := result.Artifacts["data.request.limit_reached.v1"]; ok {
		t.Fatalf("did not expect data request limit artifact: %#v", result.Artifacts["data.request.limit_reached.v1"])
	}
}

func TestRunFlowManifestRequestsContactInputForUnresolvedAuditorContactGap(t *testing.T) {
	root := t.TempDir()
	sabioBin := filepath.Join(root, "sabio-contact-lookup")
	if err := os.WriteFile(sabioBin, []byte("#!/bin/sh\nprintf '%s' '{\"found\":false}'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REMORA_SABIO_BIN", sabioBin)
	t.Setenv("REMORA_PROFILE", "test-no-contact")
	calls := 0
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifact_type":"auditor.findings.v1","artifacts":["auditor.findings.v1","data.gaps.v1"],"findings":[{"rule":"schema_contact_gap","severity":"critical","endpoint":"clients","field":"email","message":"clients no tiene email"}],"data_gaps":[{"rule":"schema_contact_gap","severity":"critical","endpoint":"clients","field":"email","message":"clients no tiene email","fix_hint":{"required_artifact":"contact.destination.v1"}}]}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"auditor": {
			Name:   "auditor",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"scan": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "data.quality.audit",
				Command:  "scan",
				Requires: []string{"external.api.dump.v1"},
				Produces: []string{"auditor.findings.v1", "data.gaps.v1"},
			}},
		},
		"sender": {
			Name:   "sender",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"send": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "message.send",
				Command:  "send",
				Requires: []string{"message.draft.v1", "contact.destination.v1"},
				Produces: []string{"message.sent.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "auditor_contact_gap",
			ProvidedArtifacts: []string{"external.api.dump.v1", "message.draft.v1", "entity.ref.v1"},
			Nodes: []flowNode{
				{ID: "audit", Framework: "auditor", Capability: "data.quality.audit"},
				{ID: "send", Framework: "sender", Capability: "message.send"},
			},
		},
		InitialArtifacts: map[string]interface{}{
			"entity.ref.v1": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "184", "name": "Thiel-Effertz"},
		},
	}, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s want needs_input; result=%#v", result.Status, result)
	}
	if calls != 1 {
		t.Fatalf("expected only auditor to execute, got %d channel calls", calls)
	}
	if len(result.NeedsInput) != 1 || result.NeedsInput[0].Kind != "contact_email" {
		t.Fatalf("expected contact_email input, got %#v", result.NeedsInput)
	}
	if result.NeedsInput[0].Context["data_gap"] != "clients no tiene email" {
		t.Fatalf("expected auditor gap context, got %#v", result.NeedsInput[0].Context)
	}
}

func TestRunFlowManifestInjectsAuditorPreflightBeforeExternalSideEffect(t *testing.T) {
	root := t.TempDir()
	sabioBin := filepath.Join(root, "sabio-contact-lookup")
	if err := os.WriteFile(sabioBin, []byte("#!/bin/sh\nprintf '%s' '{\"found\":false}'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REMORA_SABIO_BIN", sabioBin)
	calls := []string{}
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		params, _ := req["params"].(map[string]interface{})
		args, _ := params["args"].([]interface{})
		if len(args) > 0 {
			calls = append(calls, args[0].(string))
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifact_type":"auditor.findings.v1","artifacts":["auditor.findings.v1","data.gaps.v1"],"findings":[{"rule":"schema_contact_gap","severity":"critical","endpoint":"customers","field":"email","message":"customers no tiene email"}],"data_gaps":[{"rule":"schema_contact_gap","severity":"critical","endpoint":"customers","field":"email","message":"customers no tiene email","fix_hint":{"required_artifact":"contact.destination.v1"}}]}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"auditor": {
			Name:   "auditor",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"scan": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "data.quality.audit",
				Command:  "scan",
				Requires: []string{"external.api.dump.v1"},
				Produces: []string{"auditor.findings.v1", "data.gaps.v1"},
			}},
		},
		"sender": {
			Name:   "sender",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"send": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "message.send",
				Command:  "send",
				Requires: []string{"message.draft.v1", "contact.destination.v1"},
				Produces: []string{"message.sent.v1"},
				Policies: []string{"external_side_effect", "approval_required"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "generic_preflight",
			ProvidedArtifacts: []string{"external.api.dump.v1", "message.draft.v1", "entity.ref.v1"},
			Nodes: []flowNode{
				{ID: "send", Framework: "sender", Capability: "message.send"},
			},
		},
		InitialArtifacts: map[string]interface{}{
			"entity.ref.v1": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "customer", "id": "C-1", "name": "Generic Customer"},
		},
	}, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s want needs_input; result=%#v", result.Status, result)
	}
	if len(calls) != 1 {
		t.Fatalf("expected only auditor preflight command to run, got %#v", calls)
	}
	if len(result.DynamicNodes) != 1 || result.DynamicNodes[0].Framework != "auditor" {
		t.Fatalf("expected auditor dynamic preflight node, got %#v", result.DynamicNodes)
	}
	if len(result.Timeline) == 0 || result.Timeline[0].Visibility != flowStepVisibilityInfrastructure || result.Timeline[0].TriggeredBy == nil || result.Timeline[0].TriggeredBy.Node != "send" {
		t.Fatalf("expected infrastructure preflight tied to sender, got %#v", result.Timeline)
	}
	if len(result.NeedsInput) != 1 || result.NeedsInput[0].Artifact != "contact.destination.v1" || result.NeedsInput[0].Context["reported_by"] != "auditor" {
		t.Fatalf("expected auditor-driven contact input, got %#v", result.NeedsInput)
	}
	if result.NeedsInput[0].Node == "" || result.NeedsInput[0].Visibility != flowStepVisibilityUserFacing {
		t.Fatalf("expected visible node anchor for contact input, got %#v", result.NeedsInput[0])
	}
	if _, ok := result.Artifacts["message.sent.v1"]; ok {
		t.Fatalf("sender should not run before preflight gaps are resolved")
	}
	preflight, ok := result.Artifacts["flow.preflight.v1"]
	if !ok {
		t.Fatalf("missing flow.preflight.v1 in %#v", result.Artifacts)
	}
	payload, ok := preflight.Payload.(map[string]interface{})
	if !ok || payload["ready"] != false || payload["target_node"] != "send" {
		t.Fatalf("unexpected preflight payload %#v", preflight.Payload)
	}
}

func TestRunFlowManifestRequestsApprovalForMecanicoProposals(t *testing.T) {
	root := t.TempDir()
	call := 0
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		stdout := `{"artifact_type":"auditor.findings.v1","artifacts":["auditor.findings.v1","data.gaps.v1"],"findings":[{"id":"F-001","rule":"empty_required","auto_fixable":true}],"data_gaps":[{"kind":"empty_required","description":"cliente sin nombre"}]}`
		if call == 2 {
			stdout = `{"artifact_type":"mecanico.proposals.v1","artifacts":["mecanico.proposals.v1","mecanico.proposal.v1"],"proposals":[{"id":"P-001","finding_id":"F-001"}]}`
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"auditor": {
			Name:   "auditor",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"scan": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "data.quality.audit",
				Command:  "scan",
				Requires: []string{"external.api.dump.v1"},
				Produces: []string{"auditor.findings.v1", "data.gaps.v1"},
			}},
		},
		"mecanico": {
			Name:   "mecanico",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"propose-all-auto": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "action.fix.propose_all_auto",
				Command:  "propose-all-auto",
				Requires: []string{"auditor.findings.v1"},
				Produces: []string{"mecanico.proposals.v1", "mecanico.proposal.v1"},
				Policies: []string{"no_external_side_effect", "resolution_hybrid", "approval_required_before_apply"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "mecanico_proposals",
			ProvidedArtifacts: []string{"external.api.dump.v1"},
			Nodes:             []flowNode{{ID: "audit", Framework: "auditor", Capability: "data.quality.audit"}},
		},
	}, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s want needs_input; result=%#v", result.Status, result)
	}
	if len(result.NeedsInput) != 1 || result.NeedsInput[0].Framework != "mecanico" || result.NeedsInput[0].Kind != "approval" {
		t.Fatalf("expected mecanico approval input, got %#v", result.NeedsInput)
	}
	if result.NeedsInput[0].Node != "gap_resolve_mecanico_0" {
		t.Fatalf("expected mecanico approval anchored to resolution node, got %#v", result.NeedsInput[0])
	}
	if _, ok := result.Artifacts["mecanico.proposals.v1"]; !ok {
		t.Fatalf("missing mecanico proposals artifact in %#v", result.Artifacts)
	}
	if len(result.DynamicNodes) != 1 || result.DynamicNodes[0].Framework != "mecanico" {
		t.Fatalf("expected mecanico resolution dynamic node, got %#v", result.DynamicNodes)
	}
}

func TestRunFlowManifestAppliesApprovedMecanicoProposalsBeforeReaudit(t *testing.T) {
	root := t.TempDir()
	var stdoutByCall []string
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdout := `{"artifact_type":"auditor.findings.v1","artifacts":["auditor.findings.v1","data.gaps.v1"],"findings":[],"data_gaps":[]}`
		if len(stdoutByCall) == 0 {
			stdout = `{"artifact_type":"mecanico.applied.v1","artifacts":["mecanico.applied.v1","dataset.raw.v1","external.api.dump.v1"],"applied":[{"proposal_id":"P-001"}],"updated_dataset":{"artifact_type":"dataset.raw.v1","tables":{"clients":[{"id":"1","name":"Cliente Corregido"}]}},"human_summary":"Mecánico aplicó 1 propuesta."}`
		}
		stdoutByCall = append(stdoutByCall, stdout)
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"mecanico": {
			Name:   "mecanico",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"apply-all": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "action.fix.apply_all",
				Command:  "apply-all",
				Requires: []string{"external.api.dump.v1"},
				Produces: []string{"mecanico.applied.v1", "dataset.raw.v1", "external.api.dump.v1"},
				Policies: []string{"state_mutation", "approval_required"},
			}},
		},
		"auditor": {
			Name:   "auditor",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"scan": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "data.quality.audit",
				Command:  "scan",
				Requires: []string{"external.api.dump.v1"},
				Produces: []string{"auditor.findings.v1", "data.gaps.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Approved: true,
		Flow: flowManifest{
			ID:                "mecanico_apply_then_audit",
			ProvidedArtifacts: []string{"external.api.dump.v1"},
			Nodes:             []flowNode{{ID: "audit", Framework: "auditor", Capability: "data.quality.audit"}},
		},
		InitialArtifacts: map[string]interface{}{
			"action.selection.v1":  map[string]interface{}{"artifact_type": "action.selection.v1", "id": "apply_mecanico_proposals"},
			"external.api.dump.v1": map[string]interface{}{"artifact_type": "dataset.raw.v1", "tables": map[string]interface{}{"clients": []interface{}{map[string]interface{}{"id": "1", "name": ""}}}},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s want completed; result=%#v", result.Status, result)
	}
	if len(stdoutByCall) != 2 {
		t.Fatalf("expected apply + audit calls, got %d", len(stdoutByCall))
	}
	if len(result.DynamicNodes) != 1 || result.DynamicNodes[0].ID != "mecanico_apply_approved" {
		t.Fatalf("expected mecanico apply dynamic node, got %#v", result.DynamicNodes)
	}
	if _, ok := result.Artifacts["mecanico.applied.v1"]; !ok {
		t.Fatalf("missing mecanico.applied.v1 in %#v", result.Artifacts)
	}
	updated := result.Artifacts["external.api.dump.v1"].Payload.(map[string]interface{})
	tables := updated["tables"].(map[string]interface{})
	clients := tables["clients"].([]interface{})
	if clients[0].(map[string]interface{})["name"] != "Cliente Corregido" {
		t.Fatalf("dataset was not updated: %#v", updated)
	}
}

func TestRunFlowManifestExposesStructuredActionOptions(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout: `{
				"artifact_type":"focus.next_task.v1",
				"artifacts":["focus.next_task.v1","action.options.v1"],
				"action_options":[
					{"id":"send_email","label":"Enviar email","description":"Preparar y enviar correo"},
					{"id":"call","label":"Llamar","description":"Contactar por teléfono"}
				]
			}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"foco": {
			Name:   "foco",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"next-task": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "focus.next_collection_task",
				Command:  "next-task",
				Produces: []string{"focus.next_task.v1", "action.options.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{Flow: flowManifest{ID: "options", Nodes: []flowNode{{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"}}}}, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 1 || len(result.Timeline[0].ActionOptions) != 2 {
		t.Fatalf("expected structured action options on timeline, got %#v", result.Timeline)
	}
	if result.Timeline[0].ActionOptions[0]["id"] != "send_email" {
		t.Fatalf("unexpected action options %#v", result.Timeline[0].ActionOptions)
	}
	if len(result.NeedsInput) != 1 || result.NeedsInput[0].Kind != "action_selection" {
		t.Fatalf("expected action selection pause, got %#v", result.NeedsInput)
	}
	if len(result.NeedsInput[0].Actions) != 2 || result.NeedsInput[0].Actions[0].ID != "send_email" {
		t.Fatalf("unexpected action selection actions %#v", result.NeedsInput[0].Actions)
	}
}

func TestRunFlowManifestValidatesActionOptionsAgainstManifestBounds(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout: `{
				"artifact_type":"focus.next_task.v1",
				"artifacts":["focus.next_task.v1","action.options.v1"],
				"action_options":[
					{"id":"send_email","bound_id":"proceed","label":"Enviar email","description":"Preparar y enviar correo"},
					{"id":"invented","bound_id":"outside","label":"Borrar deuda","description":"Fuera de límites"}
				]
			}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"foco": {
			Name:   "foco",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"next-task": {Args: []string{"-c", "true"}},
			},
			ActionBounds: []manifest.ActionBoundSpec{
				{Type: "proceed", Description: "Avanzar"},
				{Type: "postpone", Description: "Postergar"},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "focus.next_collection_task",
				Command:  "next-task",
				Produces: []string{"focus.next_task.v1", "action.options.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{Flow: flowManifest{ID: "bounds", Nodes: []flowNode{{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"}}}}, nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.Timeline) != 1 {
		t.Fatalf("unexpected timeline %#v", result.Timeline)
	}
	options := result.Timeline[0].ActionOptions
	if len(options) != 2 {
		t.Fatalf("expected valid option plus fallback, got %#v", options)
	}
	if options[0]["bound_id"] != "proceed" || options[1]["bound_id"] != "postpone" {
		t.Fatalf("unexpected bounded options %#v", options)
	}
	if _, ok := result.Artifacts["action.bounds.validation.v1"]; !ok {
		t.Fatalf("missing action bounds validation artifact")
	}
	if len(result.NeedsInput) != 1 || len(result.NeedsInput[0].Actions) != 2 {
		t.Fatalf("expected paused bounded action selection, got %#v", result.NeedsInput)
	}
}

func TestRunFlowManifestEmitsCycleCompletedOnMessageSent(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:    true,
			ExitCode:   0,
			Stdout:     `{"artifact_type":"message.sent.v1","message_id":"msg_123","to":"cliente@example.com","channel":"email"}`,
			DurationMs: 2,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"sender": {
			Name:   "sender",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"send": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "message.send",
				Command:  "send",
				Requires: []string{"message.draft.v1", "credentials.smtp"},
				Produces: []string{"message.sent.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Approved: true,
		Flow: flowManifest{
			ID:                "cycle_complete",
			ProvidedArtifacts: []string{"message.draft.v1", "credentials.smtp", "entity.ref.v1"},
			Nodes:             []flowNode{{ID: "send", Framework: "sender", Capability: "message.send"}},
		},
		InitialArtifacts: map[string]interface{}{
			"entity.ref.v1": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "184", "name": "Thiel-Effertz"},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	cycle, ok := result.Artifacts["flow.cycle.completed.v1"]
	if !ok {
		t.Fatalf("missing flow.cycle.completed.v1 in %#v", result.Artifacts)
	}
	payload, ok := cycle.Payload.(map[string]interface{})
	if !ok || payload["cycle_kind"] != "message_sent" || payload["entity_ref"] != "184" {
		t.Fatalf("unexpected cycle payload %#v", cycle.Payload)
	}
	if !containsString(result.Timeline[0].ArtifactTypes, "flow.cycle.completed.v1") {
		t.Fatalf("timeline should mention cycle artifact: %#v", result.Timeline[0])
	}
}

func TestCycleTerminalPolicyClosesCycle(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:    true,
			ExitCode:   0,
			Stdout:     `{"artifact_type":"review.completed.v1","review_id":"rev_123"}`,
			DurationMs: 2,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"reviewer": {
			Name:   "reviewer",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"complete": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "review.complete",
				Command:  "complete",
				Produces: []string{"review.completed.v1"},
				Policies: []string{"cycle_terminal"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "cycle_terminal_policy",
			Nodes: []flowNode{{ID: "review", Framework: "reviewer", Capability: "review.complete"}},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	cycle, ok := result.Artifacts["flow.cycle.completed.v1"]
	if !ok {
		t.Fatalf("missing flow.cycle.completed.v1 in %#v", result.Artifacts)
	}
	payload, ok := cycle.Payload.(map[string]interface{})
	if !ok || payload["cycle_kind"] != "review.complete" {
		t.Fatalf("unexpected cycle payload %#v", cycle.Payload)
	}
	evidence, ok := payload["evidence"].(map[string]interface{})
	if !ok || evidence["completed_by"] != "review" || evidence["capability"] != "review.complete" {
		t.Fatalf("unexpected cycle evidence %#v", payload["evidence"])
	}
	if !containsString(result.Timeline[0].ArtifactTypes, "flow.cycle.completed.v1") {
		t.Fatalf("timeline should mention cycle artifact: %#v", result.Timeline[0])
	}
}

func TestNoCycleWithoutTerminalPolicy(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:    true,
			ExitCode:   0,
			Stdout:     `{"artifact_type":"review.completed.v1","review_id":"rev_123"}`,
			DurationMs: 2,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"reviewer": {
			Name:   "reviewer",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"complete": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "review.complete",
				Command:  "complete",
				Produces: []string{"review.completed.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:    "no_cycle_without_terminal_policy",
			Nodes: []flowNode{{ID: "review", Framework: "reviewer", Capability: "review.complete"}},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if _, ok := result.Artifacts["flow.cycle.completed.v1"]; ok {
		t.Fatalf("unexpected flow.cycle.completed.v1 in %#v", result.Artifacts)
	}
	if containsString(result.Timeline[0].ArtifactTypes, "flow.cycle.completed.v1") {
		t.Fatalf("timeline should not mention cycle artifact: %#v", result.Timeline[0])
	}
}

func TestRunFlowManifestExecutesActionBranchesInDryRun(t *testing.T) {
	root := t.TempDir()
	var activeBranches int32
	var maxActiveBranches int32
	var branchDryRuns int32
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		args, _ := params["args"].([]interface{})
		isBranch := false
		for _, arg := range args {
			switch arg {
			case "send_email", "call", "plan", "legal":
				isBranch = true
			}
		}
		if isBranch {
			for _, arg := range args {
				if arg == "dry_run=true" {
					atomic.AddInt32(&branchDryRuns, 1)
				}
				if arg == "dry_run=false" {
					t.Fatalf("branch executed without dry_run: %#v", args)
				}
			}
			now := atomic.AddInt32(&activeBranches, 1)
			for {
				prev := atomic.LoadInt32(&maxActiveBranches)
				if now <= prev || atomic.CompareAndSwapInt32(&maxActiveBranches, prev, now) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			defer atomic.AddInt32(&activeBranches, -1)
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout: `{
				"artifact_type":"focus.next_task.v1",
				"artifacts":["focus.next_task.v1","action.options.v1"],
				"action_options":[
					{"id":"send_email","label":"Enviar email","description":"Preparar y enviar correo"},
					{"id":"call","label":"Llamar","description":"Contactar por teléfono"},
					{"id":"plan","label":"Plan de pagos","description":"Negociar cuotas"},
					{"id":"legal","label":"Escalar legal","description":"Derivar a legal"}
				]
			}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"foco": {
			Name:   "foco",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"next-task": {Args: []string{"-c", "true", "{params.action_id}", "dry_run={params.dry_run}"}, Params: []string{"action_id", "dry_run"}, Defaults: map[string]string{"action_id": "", "dry_run": "false"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "focus.next_collection_task",
				Command:  "next-task",
				Produces: []string{"focus.next_task.v1", "action.options.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	events := []string{}
	result := s.runFlowManifest(context.Background(), flowRunRequest{MaxBranches: 3, Flow: flowManifest{ID: "branches", Nodes: []flowNode{{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task", Role: flowRoleEntry}}}}, func(event string, step flowRunStep, totalSteps int) {
		events = append(events, event)
	})
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.Branches) != 3 {
		t.Fatalf("expected 3 branch runs, got %#v", result.Branches)
	}
	for i, branch := range result.Branches {
		if branch.BranchID == "" || branch.Action["id"] == "" {
			t.Fatalf("bad branch %d: %#v", i, branch)
		}
		if branch.Status != "completed" || len(branch.Timeline) == 0 {
			t.Fatalf("branch did not run: %#v", branch)
		}
	}
	if !containsString(events, "branch_runs") {
		t.Fatalf("expected branch_runs event, got %#v", events)
	}
	if atomic.LoadInt32(&maxActiveBranches) < 2 {
		t.Fatalf("expected branch runs to overlap, max concurrency=%d", atomic.LoadInt32(&maxActiveBranches))
	}
	if atomic.LoadInt32(&branchDryRuns) != 3 {
		t.Fatalf("expected 3 dry-run branches, got %d", atomic.LoadInt32(&branchDryRuns))
	}
	if containsString(events, "branch_simulation") || containsString(events, "human_acceptance") {
		t.Fatalf("did not expect simulated human events without simulate_human=true, got %#v", events)
	}
}

func TestFlowBranchLimitUsesEnvironmentCap(t *testing.T) {
	t.Setenv("REMORA_FLOW_MAX_BRANCHES", "2")

	limit := flowBranchLimit(flowRunRequest{MaxBranches: 10})
	if limit != 2 {
		t.Fatalf("limit=%d want 2", limit)
	}
}

func TestApplyArtifactParamDefaultsMapsActionSelection(t *testing.T) {
	cmd := manifest.Command{
		Args: []string{"test"},
		Params: []string{
			"action_id",
			"action_label",
		},
	}
	params := map[string]string{}
	artifacts := map[string]flowRunArtifact{
		"action.selection.v1": {
			Type: "action.selection.v1",
			Payload: map[string]interface{}{
				"id":    "skip_case",
				"label": "Pasar al siguiente caso",
			},
		},
	}
	applyArtifactParamDefaults(cmd, params, artifacts)
	if params["action_id"] != "skip_case" {
		t.Fatalf("expected action_id=skip_case, got %q", params["action_id"])
	}
	if params["action_label"] != "Pasar al siguiente caso" {
		t.Fatalf("expected action_label=\"Pasar al siguiente caso\", got %q", params["action_label"])
	}
}

func TestApplyArtifactParamDefaultsMapsStrategyRecommendation(t *testing.T) {
	cmd := manifest.Command{
		Args:   []string{"test", "--strategy-json", "{params.strategy_json}"},
		Params: []string{"strategy_json"},
		Defaults: map[string]string{
			"strategy_json": "",
		},
	}
	params := map[string]string{}
	artifacts := map[string]flowRunArtifact{
		"strategy.recommendation.v1": {
			Type: "strategy.recommendation.v1",
			Payload: map[string]interface{}{
				"recommendations": []interface{}{
					map[string]interface{}{"action_id": "email_priority", "label": "Enviar email prioritario"},
				},
			},
		},
	}
	applyArtifactParamDefaults(cmd, params, artifacts)
	if params["strategy_json"] == "" {
		t.Fatalf("expected strategy_json to be set, got empty")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(params["strategy_json"]), &parsed); err != nil {
		t.Fatalf("expected valid JSON in strategy_json, got %q: %v", params["strategy_json"], err)
	}
	if _, ok := parsed["recommendations"]; !ok {
		t.Fatalf("expected recommendations in strategy_json, got %q", params["strategy_json"])
	}
}

func TestApplyArtifactParamDefaultsMapsStrategyPath(t *testing.T) {
	cmd := manifest.Command{
		Args:   []string{"test", "--strategy-path", "{params.strategy_path}"},
		Params: []string{"strategy_path"},
		Defaults: map[string]string{
			"strategy_path": "",
		},
	}
	params := map[string]string{}
	artifacts := map[string]flowRunArtifact{
		"strategy.recommendation.v1": {
			Type: "strategy.recommendation.v1",
			Path: "/tmp/strategy.json",
		},
	}
	applyArtifactParamDefaults(cmd, params, artifacts)
	if params["strategy_path"] != "/tmp/strategy.json" {
		t.Fatalf("expected strategy_path to be set, got %q", params["strategy_path"])
	}
}

func TestMaterializeLargeInlineParamsPrefersArtifactPath(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	cmd := manifest.Command{Params: []string{"dataset_json", "dataset_artifact"}, Defaults: map[string]string{"dataset_json": "", "dataset_artifact": ""}}
	existingPath := filepath.Join(root, "dataset.json")
	if err := os.WriteFile(existingPath, []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	params := map[string]string{
		"dataset_json":     strings.Repeat("x", inlineArtifactArgMaxBytes+1),
		"dataset_artifact": existingPath,
	}
	s.materializePortableArtifactParams("run_1", "radar", cmd, params)
	if params["dataset_json"] != "" {
		t.Fatalf("expected dataset_json to be cleared, len=%d", len(params["dataset_json"]))
	}
	if params["dataset_artifact"] != existingPath {
		t.Fatalf("expected existing artifact path to be preserved, got %q", params["dataset_artifact"])
	}
}

func TestMaterializeLargeInlineParamsWritesPathWhenNeeded(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	cmd := manifest.Command{Params: []string{"findings_json", "findings_path"}, Defaults: map[string]string{"findings_json": "", "findings_path": ""}}
	payload := strings.Repeat("y", inlineArtifactArgMaxBytes+1)
	params := map[string]string{"findings_json": payload}
	s.materializePortableArtifactParams("run_2", "mecanico", cmd, params)
	if params["findings_json"] != "" {
		t.Fatalf("expected findings_json to be cleared")
	}
	if params["findings_path"] == "" {
		t.Fatalf("expected findings_path to be set")
	}
	raw, err := os.ReadFile(params["findings_path"])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != payload {
		t.Fatalf("materialized payload mismatch")
	}
}

func TestMaterializePortableArtifactParamsMovesSmallJSONWhenPathExists(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	cmd := manifest.Command{Params: []string{"strategy_json", "strategy_path"}, Defaults: map[string]string{"strategy_json": "", "strategy_path": ""}}
	payload := `{"reason":"Saldo alto; requiere seguimiento"}`
	params := map[string]string{"strategy_json": payload}
	s.materializePortableArtifactParams("run_3", "foco", cmd, params)
	if params["strategy_json"] != "" {
		t.Fatalf("expected strategy_json to be cleared, got %q", params["strategy_json"])
	}
	if params["strategy_path"] == "" {
		t.Fatalf("expected strategy_path to be set")
	}
	raw, err := os.ReadFile(params["strategy_path"])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != payload {
		t.Fatalf("materialized strategy mismatch")
	}
}

func TestMaterializePortableArtifactParamsKeepsInlineWhenNoPathDeclared(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	cmd := manifest.Command{Params: []string{"strategy_json"}, Defaults: map[string]string{"strategy_json": ""}}
	payload := `{"reason":"Saldo alto; requiere seguimiento"}`
	params := map[string]string{"strategy_json": payload}
	s.materializePortableArtifactParams("run_4", "radar", cmd, params)
	if params["strategy_json"] != payload {
		t.Fatalf("expected inline JSON to remain when no path/artifact is declared, got %q", params["strategy_json"])
	}
}

func TestRunFlowManifestDeepDiveUsesMaterializedPaths(t *testing.T) {
	root := t.TempDir()
	var capturedArgs []string
	var sawRadar bool
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		argsRaw, _ := params["args"].([]interface{})
		args := make([]string, 0, len(argsRaw))
		for _, arg := range argsRaw {
			s, _ := arg.(string)
			args = append(args, s)
		}
		if containsString(args, "radar-deep-dive") {
			sawRadar = true
			capturedArgs = append([]string(nil), args...)
			for _, arg := range args {
				if strings.Contains(arg, ";") || strings.Contains(arg, ">") {
					_ = json.NewEncoder(w).Encode(adapter.Response{Success: false, ExitCode: 1, Error: "path not safe: " + arg})
					return
				}
			}
			_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"analysis.case_review.v1","artifacts":["analysis.case_review.v1","answer.grounded.v1"],"summary":"Radar takeover ok"}`})
			return
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: `{"artifact_type":"focus.next_task.v1","artifacts":["focus.next_task.v1","task.next","entity.ref.v1"],"selected":{"artifact_type":"entity.ref.v1","type":"client","id":"cust_1","name":"Cliente Uno"},"transfer_control":true}`})
	}))
	defer channel.Close()

	s := &server{rootDir: root, channel: adapter.New(channel.URL, "test-key"), allManifests: map[string]*manifest.Manifest{
		"radar": {
			Name:   "radar",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"deep-dive": {
					Args: []string{
						"-c", "radar-deep-dive",
						"--priority-list-path", "{params.priority_list_path}",
						"--priority-list-json", "{params.priority_list_json}",
						"--strategy-path", "{params.strategy_path}",
						"--strategy-json", "{params.strategy_json}",
						"--context-b64", "{params.context_b64}",
					},
					Params: []string{"priority_list_path", "priority_list_json", "strategy_path", "strategy_json", "context_b64"},
					Defaults: map[string]string{
						"priority_list_path": "",
						"priority_list_json": "",
						"strategy_path":      "",
						"strategy_json":      "",
						"context_b64":        "",
					},
				},
			},
			Capabilities: []manifest.CapabilitySpec{{ID: "analysis.deep_dive", Command: "deep-dive", Requires: []string{"collection.priority_list.v1", "entity.ref.v1"}, Produces: []string{"analysis.case_review.v1", "answer.grounded.v1"}}},
		},
		"foco": {
			Name:   "foco",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"next-task": {Args: []string{"-c", "focus-next-task"}, Params: []string{"action_id"}, Defaults: map[string]string{"action_id": ""}},
			},
			Capabilities: []manifest.CapabilitySpec{{ID: "focus.next_collection_task", Command: "next-task", Requires: []string{"collection.priority_list.v1"}, Produces: []string{"focus.next_task.v1", "task.next", "entity.ref.v1", "action.options.v1"}}},
		},
	}}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Flow: flowManifest{
			ID:                "deep_dive_transport",
			BusinessID:        "biz_test",
			ProvidedArtifacts: []string{"collection.priority_list.v1", "entity.ref.v1", "strategy.recommendation.v1"},
			Nodes:             []flowNode{{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task", Role: flowRoleEntry}},
		},
		InitialArtifacts: map[string]interface{}{
			"action.selection.v1": map[string]interface{}{"artifact_type": "action.selection.v1", "id": "deep_analysis", "bound_id": "escalate", "label": "Ver análisis profundo"},
			"entity.ref.v1":       map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "cust_1", "name": "Cliente Uno"},
			"collection.priority_list.v1": map[string]interface{}{
				"artifact_type": "collection.priority_list.v1",
				"items": []interface{}{
					map[string]interface{}{"rank": 1, "deudor": "Cliente Uno", "razon": "Saldo vencido; requiere revisión > 30 días"},
				},
				"selected": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "cust_1", "name": "Cliente Uno"},
			},
			"strategy.recommendation.v1": map[string]interface{}{
				"artifact_type": "strategy.recommendation.v1",
				"recommendations": []interface{}{
					map[string]interface{}{"action_id": "deep_analysis", "description": "Investigar; revisar > enviar"},
				},
			},
		},
	}, nil)
	if result.Status != "completed" {
		t.Fatalf("expected completed flow, got %s %#v", result.Status, result)
	}
	if !sawRadar {
		t.Fatalf("expected radar deep-dive execution, got args=%v", capturedArgs)
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--priority-list-path") || !strings.Contains(joined, "--strategy-path") {
		t.Fatalf("expected materialized path args, got %v", capturedArgs)
	}
	if strings.Contains(joined, "Saldo vencido;") || strings.Contains(joined, "revisar > enviar") {
		t.Fatalf("expected no inline structured payload in args, got %v", capturedArgs)
	}
}

func TestRunFlowManifestRespectsMaxCycles(t *testing.T) {
	root := t.TempDir()
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifact_type":"message.sent.v1","message_id":"msg_123","to":"a@example.com","channel":"email"}`,
		})
	}))
	defer channel.Close()
	s := &server{rootDir: root, allManifests: map[string]*manifest.Manifest{
		"sender": {
			Name:   "sender",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"send": {Args: []string{"-c", "true"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "message.send",
				Command:  "send",
				Requires: []string{"message.draft.v1", "credentials.smtp"},
				Produces: []string{"message.sent.v1"},
			}},
		},
	}, channel: adapter.New(channel.URL, "test-key")}

	result := s.runFlowManifest(context.Background(), flowRunRequest{
		Approved:  true,
		MaxCycles: 1,
		Flow: flowManifest{
			ID:                "max_cycles",
			ProvidedArtifacts: []string{"message.draft.v1", "credentials.smtp", "entity.ref.v1"},
			Nodes: []flowNode{
				{ID: "send1", Framework: "sender", Capability: "message.send"},
				{ID: "send2", Framework: "sender", Capability: "message.send"},
			},
		},
		InitialArtifacts: map[string]interface{}{
			"entity.ref.v1": map[string]interface{}{"artifact_type": "entity.ref.v1", "type": "client", "id": "184", "name": "Thiel-Effertz"},
		},
	}, nil)

	if result.Status != "max_cycles_reached" {
		t.Fatalf("expected max_cycles_reached, got status=%s result=%#v", result.Status, result)
	}
	if _, ok := result.Artifacts["flow.cycle.limit_reached.v1"]; !ok {
		t.Fatalf("missing flow.cycle.limit_reached.v1 in %#v", result.Artifacts)
	}
	if len(result.Timeline) != 1 {
		t.Fatalf("expected 1 timeline entry (stopped at max_cycles), got %d", len(result.Timeline))
	}
	if result.Timeline[0].Status != "max_cycles_reached" {
		t.Fatalf("expected max_cycles_reached on the step, got %#v", result.Timeline[0])
	}
	if result.Timeline[0].Node != "send1" {
		t.Fatalf("expected node send1 to be the one stopped, got %s", result.Timeline[0].Node)
	}
}

func TestCollectionFlowStopsAtMecanicoQuestionWhenContactMissing(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()
	result := s.runFlowManifest(context.Background(), collectionSmokeRequest(false, false), nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.NeedsInput) == 0 {
		t.Fatalf("expected at least one needs_input item, got none")
	}
	// When both contact and SMTP are missing, the flow may stop at either
	// mecanico (contact gap) or hosting (SMTP gap) first, depending on
	// which resolution check runs first in the pipeline.
	fw := result.NeedsInput[0].Framework
	if fw != "mecanico" && fw != "hosting" {
		t.Fatalf("expected mecanico or hosting question, got framework=%q: %#v", fw, result.NeedsInput)
	}
	if _, ok := result.Artifacts["message.sent.v1"]; ok {
		t.Fatalf("flow should not send without contact: %#v", result.Artifacts["message.sent.v1"])
	}
}

func TestCollectionFlowActivatesHostingWhenSMTPMissing(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()
	result := s.runFlowManifest(context.Background(), collectionSmokeRequest(true, false), nil)
	if result.Status != "needs_input" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.NeedsInput) == 0 {
		t.Fatalf("expected needs_input for missing SMTP, got none")
	}
	ni := result.NeedsInput[0]
	if ni.Kind != "conversational_question" {
		t.Fatalf("expected conversational_question for missing SMTP, got kind=%q: %#v", ni.Kind, result.NeedsInput)
	}
	if ni.Framework != "hosting" {
		t.Fatalf("expected Hosting to own missing outbound email, got framework=%q: %#v", ni.Framework, ni)
	}
	if ni.Capability != "credentials.cpanel.connect" {
		t.Fatalf("expected Hosting connect capability, got %q", ni.Capability)
	}
	if ni.Field != "domain" && ni.Step != "cpanel_domain" {
		t.Fatalf("expected sequential Hosting setup to start with domain/cpanel discovery, got %#v", ni)
	}
	if strings.Contains(strings.ToLower(ni.Message), "smtp") && strings.Contains(strings.ToLower(ni.Message), "host smtp") {
		t.Fatalf("unexpected generic SMTP prompt in Hosting question: %#v", ni)
	}
	if ni.Artifact != "credentials.smtp" {
		t.Fatalf("expected artifact credentials.smtp, got %q: %#v", ni.Artifact, ni)
	}
	if _, ok := result.Artifacts["message.draft.v1"]; !ok {
		t.Fatalf("expected draft before SMTP block: %#v", result.Artifacts)
	}
	if ni.SegmentMode != "operational" || ni.SegmentOwner != "foco" || ni.SegmentRole != "delegate" {
		t.Fatalf("expected operational segment metadata on hosting need, got %#v", ni)
	}
}

func TestCollectionFlowRoutesNodeViewAnswerToHosting(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()

	req := collectionSmokeRequest(true, false)
	first := s.runFlowManifest(context.Background(), req, nil)
	if first.Status != "needs_input" || len(first.NeedsInput) == 0 {
		t.Fatalf("expected hosting needs_input, got %#v", first)
	}
	need := first.NeedsInput[0]
	if need.Framework != "hosting" {
		t.Fatalf("expected Hosting owner, got %#v", need)
	}

	req.InitialArtifacts["flow.interaction.answer.v1"] = map[string]interface{}{
		"artifact_type": "flow.interaction.answer.v1",
		"framework":     need.Framework,
		"capability":    need.Capability,
		"artifact":      need.Artifact,
		"question_id":   need.QuestionID,
		"field":         need.Field,
		"step":          need.Step,
		"value":         "tudominio.com",
	}
	req.Approved = true
	second := s.runFlowManifest(context.Background(), req, nil)
	if second.Status != "max_cycles_reached" {
		t.Fatalf("expected flow to continue after routing answer to Hosting, got status=%s result=%#v", second.Status, second)
	}
	if _, ok := second.Artifacts["message.sent.v1"]; !ok {
		t.Fatalf("expected send after Hosting answer, got %#v", second.Artifacts)
	}
}

func TestCollectionFlowCompletesCycleWhenContactAndSMTPExist(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()
	req := collectionSmokeRequest(true, true)
	req.Approved = true
	result := s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "max_cycles_reached" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	for _, artifact := range []string{"dataset.raw.v1", "collection.priority_list.v1", "focus.next_task.v1", "entity_360.v1", "message.draft.v1", "message.sent.v1", "flow.cycle.completed.v1", "flow.cycle.limit_reached.v1"} {
		if _, ok := result.Artifacts[artifact]; !ok {
			t.Fatalf("missing %s in %#v", artifact, result.Artifacts)
		}
	}
	if result.CyclesDone != 1 {
		t.Fatalf("cycles_done=%d want 1", result.CyclesDone)
	}
}

func TestCollectionFlowDeepAnalysisStaysInAnalyticalSegment(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()

	req := collectionSmokeRequest(false, false)
	req.InitialArtifacts["action.selection.v1"] = map[string]interface{}{
		"artifact_type": "action.selection.v1",
		"id":            "deep_analysis",
		"bound_id":      "escalate",
		"label":         "Ver análisis profundo",
	}
	result := s.runFlowManifest(context.Background(), req, nil)
	if result.Status != "completed" {
		t.Fatalf("status=%s result=%#v", result.Status, result)
	}
	if len(result.NeedsInput) != 0 {
		t.Fatalf("expected no needs_input in analytical segment, got %#v", result.NeedsInput)
	}
	for _, artifact := range []string{"dataset.raw.v1", "collection.priority_list.v1", "analysis.case_review.v1", "entity_360.v1", "auditor.findings.v1", "segment.active.v1", "segment.constraints.v1", "segment.owner.v1"} {
		if _, ok := result.Artifacts[artifact]; !ok {
			t.Fatalf("missing %s in %#v", artifact, result.Artifacts)
		}
	}
	for _, artifact := range []string{"message.draft.v1", "message.sent.v1", "contact.destination.v1", "credentials.smtp", "action.options.v1"} {
		if _, ok := result.Artifacts[artifact]; ok {
			t.Fatalf("did not expect %s in analytical segment: %#v", artifact, result.Artifacts[artifact])
		}
	}
	active, _ := result.Artifacts["segment.active.v1"].Payload.(map[string]interface{})
	if active["mode"] != "analytical" {
		t.Fatalf("expected analytical segment, got %#v", active)
	}
	if active["owner_framework"] != "radar" || active["owner_capability"] != "analysis.deep_dive" {
		t.Fatalf("expected radar deep-dive owner, got %#v", active)
	}
	if active["subject_ref"] != "cust_1" {
		t.Fatalf("expected segment subject_ref=cust_1, got %#v", active)
	}
	owner, _ := result.Artifacts["segment.owner.v1"].Payload.(map[string]interface{})
	if owner["framework"] != "radar" || owner["capability"] != "analysis.deep_dive" {
		t.Fatalf("expected segment.owner.v1 owned by radar deep dive, got %#v", owner)
	}
	var radarStep, focusStep, draftStep, sendStep flowRunStep
	var radarIdx, focusIdx int = -1, -1
	for _, step := range result.Timeline {
		if step.Node == "radar_deep_analysis" {
			radarStep = step
		}
		if step.Node == "focus" {
			focusStep = step
		}
		if step.Node == "draft" {
			draftStep = step
		}
		if step.Node == "send" {
			sendStep = step
		}
	}
	for idx, step := range result.Timeline {
		if step.Node == "radar_deep_analysis" {
			radarIdx = idx
		}
		if step.Node == "focus" {
			focusIdx = idx
		}
	}
	if radarIdx < 0 || focusIdx < 0 || radarIdx >= focusIdx {
		t.Fatalf("expected radar takeover before focus reentry, got timeline %#v", result.Timeline)
	}
	if radarStep.Status != "completed" || radarStep.SegmentRole != "owner" || radarStep.SegmentOwner != "radar" {
		t.Fatalf("expected radar deep-dive owner step, got %#v", radarStep)
	}
	if focusStep.Status != "skipped" || focusStep.SegmentRole != "delegate" || focusStep.SegmentOwner != "radar" {
		t.Fatalf("expected foco skipped under radar ownership, got %#v", focusStep)
	}
	if draftStep.Status != "skipped" || draftStep.SegmentMode != "analytical" {
		t.Fatalf("expected draft skipped in analytical segment, got %#v", draftStep)
	}
	if draftStep.SegmentRole != "delegate" || draftStep.SegmentOwner != "radar" {
		t.Fatalf("expected draft step to remain delegated under radar ownership, got %#v", draftStep)
	}
	if sendStep.Status != "skipped" || sendStep.SegmentMode != "analytical" {
		t.Fatalf("expected send skipped in analytical segment, got %#v", sendStep)
	}
	if sendStep.SegmentRole != "delegate" || sendStep.SegmentOwner != "radar" {
		t.Fatalf("expected send step to remain delegated under radar ownership, got %#v", sendStep)
	}
	if len(result.DynamicNodes) == 0 || result.DynamicNodes[0].ID != "radar_deep_analysis" {
		t.Fatalf("expected radar deep-dive as dynamic owner node, got %#v", result.DynamicNodes)
	}
	if _, ok := result.Artifacts["segment.delegate.v1"]; !ok {
		t.Fatalf("expected segment.delegate.v1, got %#v", result.Artifacts)
	}
	if _, ok := result.Artifacts["segment.return.v1"]; !ok {
		t.Fatalf("expected segment.return.v1, got %#v", result.Artifacts)
	}
}

func TestDeepAnalysisDimensionRunsConversationalFollowupsAndHandoff(t *testing.T) {
	s, closeFn := newCollectionSmokeServer(t)
	defer closeFn()
	t.Setenv("REMORA_DEEP_ANALYSIS_STRESS_TURNS", "6")

	req := collectionSmokeRequest(false, false)
	artifacts := map[string]flowRunArtifact{
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

	branches := s.runFlowActionBranches(context.Background(), req, artifacts)
	if len(branches) != 1 {
		t.Fatalf("expected one branch, got %#v", branches)
	}
	branch := branches[0]
	if branch.Action["id"] != "deep_analysis" {
		t.Fatalf("expected deep_analysis branch, got %#v", branch.Action)
	}
	countUser := 0
	countFollowup := 0
	countOperational := 0
	countHandoff := 0
	countDelegationPlan := 0
	countSabioDelegate := 0
	countAuditorDelegate := 0
	integratedDelegatedEvidence := false
	synthesizedDelegatedFollowup := false
	sawCompletedComparativePlan := false
	for _, step := range branch.Timeline {
		if step.Node == "deep_analysis_simulated_user" {
			countUser++
		}
		if step.Node == "radar_deep_analysis_followup" && step.Status == "completed" {
			countFollowup++
		}
		if step.Node == "analysis_handoff_to_foco" && step.Status == "completed" {
			countHandoff++
			if step.SegmentMode == segmentModeOperational && step.SegmentOwner == "foco" {
				countOperational++
			}
		}
		if step.Node == "radar_deep_analysis_delegation_plan" {
			countDelegationPlan++
			if strings.Contains(step.HumanSummary, "evidence.case_360") && strings.Contains(step.HumanSummary, "evidence.portfolio_comparison") {
				sawCompletedComparativePlan = true
			}
		}
		if strings.Contains(step.Node, "deep_analysis_delegate_data.query.sql") || step.Framework == "sabio" && step.Role == "delegate" {
			countSabioDelegate++
		}
		if strings.Contains(step.Node, "deep_analysis_delegate_data.quality.audit") || step.Framework == "auditor" && step.Role == "delegate" {
			countAuditorDelegate++
		}
		if strings.Contains(step.HumanSummary, "delegation_results_json") || strings.Contains(step.HumanSummary, "percentil") {
			integratedDelegatedEvidence = true
		}
		if step.Node == "radar_deep_analysis_followup" && step.Synthesized && step.AnalysisPhase == "synthesis" && (strings.Contains(step.HumanSummary, "delegation_results_path") || strings.Contains(step.HumanSummary, "percentil") || strings.Contains(step.HumanSummary, "auditoría") || strings.Contains(step.HumanSummary, "detalle del caso")) {
			synthesizedDelegatedFollowup = true
		}
	}
	if countUser < 3 {
		t.Fatalf("expected at least 3 simulated user turns, got %d timeline=%#v", countUser, branch.Timeline)
	}
	if countFollowup < 6 {
		t.Fatalf("expected stress mode to run at least 6 real Radar followups, got %d timeline=%#v", countFollowup, branch.Timeline)
	}
	if countDelegationPlan < 2 {
		t.Fatalf("expected Radar delegation plans for stress questions, got %d timeline=%#v", countDelegationPlan, branch.Timeline)
	}
	if !sawCompletedComparativePlan {
		t.Fatalf("expected comparative planning turn to be completed with both case baseline and portfolio evidence, timeline=%#v", branch.Timeline)
	}
	if countSabioDelegate == 0 {
		t.Fatalf("expected portfolio comparison to delegate to Sabio, timeline=%#v", branch.Timeline)
	}
	if countAuditorDelegate == 0 {
		t.Fatalf("expected contradiction/gap question to delegate to Auditor, timeline=%#v", branch.Timeline)
	}
	if !integratedDelegatedEvidence {
		t.Fatalf("expected final Radar response to integrate delegated evidence, timeline=%#v", branch.Timeline)
	}
	if !synthesizedDelegatedFollowup {
		t.Fatalf("expected at least one delegated followup to finish with synthesized=true and analysis_phase=synthesis, timeline=%#v", branch.Timeline)
	}
	if countOperational != 1 || countHandoff != 1 {
		t.Fatalf("expected one operational handoff to Foco, operational=%d handoff=%d timeline=%#v", countOperational, countHandoff, branch.Timeline)
	}
	if !containsString(branch.Artifacts, "segment.session.v1") {
		t.Fatalf("expected branch artifacts to include segment.session.v1, got %#v", branch.Artifacts)
	}
	if !containsString(branch.Artifacts, "analysis.handoff.v1") {
		t.Fatalf("expected branch artifacts to include analysis.handoff.v1, got %#v", branch.Artifacts)
	}
	if containsString(branch.Artifacts, "message.sent.v1") {
		t.Fatalf("deep analysis dimension must not send before handoff, artifacts=%#v", branch.Artifacts)
	}
	handoffPath := s.latestFlowArtifactPath(req.Flow.BusinessID, "analysis.handoff.v1")
	if handoffPath == "" {
		t.Fatalf("expected persisted analysis.handoff.v1")
	}
	rawHandoff, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read handoff: %v", err)
	}
	var handoff map[string]interface{}
	if err := json.Unmarshal(rawHandoff, &handoff); err != nil {
		t.Fatalf("parse handoff: %v", err)
	}
	if strings.TrimSpace(fmt.Sprint(handoff["analytical_summary"])) == "" {
		t.Fatalf("expected handoff to preserve non-empty analytical summary, got %#v", handoff)
	}
	if !strings.Contains(fmt.Sprint(handoff["accepted_gaps"]), "contactabilidad") && !strings.Contains(fmt.Sprint(handoff["accepted_gaps"]), "inconsistencias") {
		t.Fatalf("expected handoff to preserve analytical gaps/findings, got %#v", handoff)
	}
	sawSynthesizedArtifact := false
	for _, path := range s.allFlowArtifactPaths(req.Flow.BusinessID, "analysis.followup.v1") {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read followup artifact %s: %v", path, err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("parse followup artifact %s: %v", path, err)
		}
		caps := fmt.Sprint(payload["delegated_capabilities"])
		if !strings.Contains(caps, "evidence.portfolio_comparison") {
			continue
		}
		if payload["analysis_phase"] == "synthesis" {
			if synthesized, _ := payload["synthesized"].(bool); synthesized {
				if strings.TrimSpace(fmt.Sprint(payload["delegation_results_path"])) == "" {
					t.Fatalf("expected delegated followup artifact to persist delegation_results_path, got %+v", payload)
				}
				sawSynthesizedArtifact = true
				break
			}
		}
	}
	if !sawSynthesizedArtifact {
		t.Fatalf("expected a persisted delegated followup artifact with analysis_phase=synthesis and synthesized=true")
	}
}

func collectionSmokeRequest(withContact, withSMTP bool) flowRunRequest {
	req := flowRunRequest{
		BranchMode: true,
		MaxCycles:  1,
		Flow: flowManifest{
			ID:                "collection_smoke",
			BusinessID:        "test_business",
			ProvidedArtifacts: []string{"data.sqlite_db.v1", "business.semantic_pack.v1"},
			Nodes: []flowNode{
				{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list", Role: flowRoleBootstrap},
				{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task", Role: flowRoleEntry},
				{ID: "entity360", Framework: "sabio", Capability: "data.entity_360", Role: flowRolePipeline},
				{ID: "audit", Framework: "auditor", Capability: "data.quality.audit", Role: flowRolePipeline},
				{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: flowRolePipeline},
				{ID: "send", Framework: "mensajero", Capability: "message.send", Role: flowRolePipeline},
			},
		},
		InitialArtifacts: map[string]interface{}{
			"action.selection.v1": map[string]interface{}{"artifact_type": "action.selection.v1", "id": "send_email", "label": "Enviar email"},
		},
	}
	if withContact {
		req.InitialArtifacts["contact.destination.v1"] = map[string]interface{}{"artifact_type": "contact.destination.v1", "channel": "email", "destination": "cliente@example.com", "value": "cliente@example.com", "to": "cliente@example.com", "entity_type": "client", "entity_ref": "cust_1"}
	}
	if withSMTP {
		req.InitialArtifacts["credentials.smtp"] = map[string]interface{}{"from_vault": true}
	}
	return req
}

func interfaceSliceToStrings(items []interface{}) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func newCollectionSmokeServer(t *testing.T) (*server, func()) {
	t.Helper()
	root := t.TempDir()
	expectedDB := filepath.Join(root, "framework-indexa", "data", "test_business.db")
	if err := os.MkdirAll(filepath.Dir(expectedDB), 0755); err != nil {
		t.Fatalf("mkdir smoke db dir: %v", err)
	}
	if err := os.WriteFile(expectedDB, []byte("sqlite fixture"), 0644); err != nil {
		t.Fatalf("write smoke db fixture: %v", err)
	}
	expectedPack := filepath.Join(root, "framework-sabio", "businesses", "test_business", "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(expectedPack), 0755); err != nil {
		t.Fatalf("mkdir smoke pack dir: %v", err)
	}
	pack := `{"business_id":"test_business","data_source":{"default_path":"../framework-indexa/data/test_business.db"}}`
	if err := os.WriteFile(expectedPack, []byte(pack), 0644); err != nil {
		t.Fatalf("write smoke semantic pack: %v", err)
	}
	smtpConfigured := false
	channel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		args, _ := params["args"].([]interface{})
		marker := ""
		for _, arg := range args {
			if s, _ := arg.(string); strings.HasPrefix(s, "smoke-") {
				marker = s
				break
			}
		}
		joinedArgs := fmt.Sprint(args)
		stdout := `{"artifact_type":"noop.v1"}`
		switch marker {
		case "smoke-sabio-dataset":
			stdout = `{"artifacts":["dataset.raw.v1","external.api.dump.v1"],"tables":{"clients":[{"id":"cust_1","name":"Cliente Uno"}]}}`
		case "smoke-radar":
			stdout = `{"artifact_type":"collection.priority_list.v1","artifacts":["collection.priority_list.v1","collection.priority_item.v1","entity.ref.v1","strategy.recommendation.v1"],"items":[{"rank":1,"deudor":"Cliente Uno","saldo_total":1000,"dias_mora_max":45}],"priority_item":{"artifact_type":"collection.priority_item.v1","rank":1,"deudor":"Cliente Uno","saldo_total":1000,"dias_mora_max":45},"selected":{"artifact_type":"entity.ref.v1","type":"client","id":"cust_1","name":"Cliente Uno"},"recommendations":[{"action_id":"send_email","label":"Enviar email","description":"Cobrar por email"}]}`
		case "smoke-radar-deep":
			stdout = `{"artifact_type":"analysis.case_review.v1","artifacts":["analysis.case_review.v1","answer.grounded.v1"],"summary":"Radar toma control del análisis profundo de Cliente Uno antes de cualquier acción operativa.","selected":{"artifact_type":"entity.ref.v1","type":"client","id":"cust_1","name":"Cliente Uno"},"owner":{"framework":"radar","capability":"analysis.deep_dive","transfer_control":true}}`
		case "smoke-radar-followup":
			delegationPath := testArgValue(interfaceSliceToStrings(args), "--delegation-results-path")
			delegationInline := testArgValue(interfaceSliceToStrings(args), "--delegation-results-json")
			delegationPayload := delegationInline
			if delegationPath != "" {
				raw, err := os.ReadFile(delegationPath)
				if err != nil {
					t.Fatalf("read materialized delegation results: %v", err)
				}
				delegationPayload = string(raw)
			}
			switch {
			case delegationInline != "" && delegationPath == "":
				t.Fatalf("expected delegated synthesis to use delegation_results_path, args=%s", joinedArgs)
			case delegationPath != "" && strings.Contains(strings.ToLower(delegationPayload), "quality"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"synthesis","text":"Radar integra delegation_results_path: la auditoría confirmó gaps de calidad y mantiene cautela por inconsistencias.","findings":["auditoría integrada"],"evidence":["delegation_results_path"],"confidence":"partial","data_gaps":["inconsistencias pendientes"]}`
			case delegationPath != "" && strings.Contains(strings.ToLower(delegationPayload), "entity_360.v1"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"synthesis","text":"Radar integra delegation_results_path: el detalle del caso aporta open_amount y payments_count para evaluar hipótesis.","findings":["detalle de caso integrado"],"evidence":["delegation_results_path"],"confidence":"moderate"}`
			case strings.Contains(delegationPayload, "percentil"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"synthesis","text":"Radar integra delegation_results_path: comparación de cartera con percentiles recibidos. Confianza: moderada. Gap: falta contactabilidad.","findings":["comparación integrada"],"evidence":["delegation_results_path"],"confidence":"moderate","data_gaps":["contactabilidad"],"recommendation":"avanzar con cautela tras handoff"}`
			case strings.Contains(strings.ToLower(joinedArgs), "compara") || strings.Contains(strings.ToLower(joinedArgs), "cartera"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"plan","analysis_intent":"portfolio_comparison","needs_delegation":true,"evidence_needed":["percentiles de cartera"],"reason":"comparar requiere datos de cartera","text":"Radar necesita comparar el caso contra la cartera antes de sintetizar.","delegation_requests":[{"framework":"sabio","capability":"evidence.case_360","params":{"entity_ref":"cust_1","entity_type":"client","analysis_intent":"case_baseline","question":"Construye baseline verificable del cliente cust_1"},"reason":"baseline del caso"}]}`
			case strings.Contains(strings.ToLower(joinedArgs), "contradic") || strings.Contains(strings.ToLower(joinedArgs), "gap"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"plan","analysis_intent":"data_reconciliation","needs_delegation":true,"evidence_needed":["auditoría de consistencia"],"reason":"reconciliar contradicciones requiere validar calidad","text":"Radar pide validación de calidad antes de concluir contradicciones.","delegation_requests":[{"type":"gaps_calidad","task":"validar contradicciones del caso"}]}`
			case strings.Contains(strings.ToLower(joinedArgs), "sensibilidad") || strings.Contains(strings.ToLower(joinedArgs), "contrafactual"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"plan","analysis_intent":"score_sensitivity","needs_delegation":true,"evidence_needed":["ranking contrafactual"],"reason":"sensibilidad requiere simulación con datos de cartera","text":"Radar necesita evidencia para medir sensibilidad del score.","delegation_requests":[{"type":"simulation","task":"analisis_de_sensibilidad","deudor_id":"cust_1"}]}`
			case strings.Contains(strings.ToLower(joinedArgs), "hipótesis") || strings.Contains(strings.ToLower(joinedArgs), "hipotesis"):
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"plan","analysis_intent":"alternative_hypothesis","needs_delegation":true,"evidence_needed":["detalle de documentos y pagos"],"reason":"las hipótesis alternativas requieren contexto del caso","text":"Radar quiere revisar detalle del caso antes de proponer hipótesis.","delegation_requests":[{"delegation_type":"obtener_detalles_de_caso","deudor_id":"cust_1","fields":["open_amount","payments_count","billing_documents"]}]}`
			default:
				stdout = `{"artifact_type":"analysis.followup.v1","artifacts":["analysis.followup.v1","answer.grounded.v1"],"analysis_phase":"synthesis","text":"Radar analiza el follow-up usando el contexto real recibido y mantiene el tramo analítico sin ejecutar operación.","grounded_answer":{"artifact_type":"answer.grounded.v1","text":"Radar analiza el follow-up usando el contexto real recibido y mantiene el tramo analítico sin ejecutar operación."},"confidence":"partial","data_gaps":["datos insuficientes para afirmar sin delegación"]}`
			}
		case "smoke-foco":
			stdout = `{"artifact_type":"focus.next_task.v1","artifacts":["focus.next_task.v1","task.next","entity.ref.v1","action.options.v1"],"task_id":"task_1","selected":{"artifact_type":"entity.ref.v1","type":"client","id":"cust_1","name":"Cliente Uno"},"action_options":[{"id":"send_email","label":"Enviar email","description":"Cobrar por email"},{"id":"skip","label":"Saltar","description":"Pasar al siguiente"}]}`
		case "smoke-sabio-query":
			if !strings.Contains(joinedArgs, expectedDB) {
				t.Fatalf("expected sabio query to receive canonical db path %q, args=%s", expectedDB, joinedArgs)
			}
			if !strings.Contains(joinedArgs, expectedPack) {
				t.Fatalf("expected sabio query to receive semantic pack %q, args=%s", expectedPack, joinedArgs)
			}
			stdout = `{"artifact_type":"entity_360.v1","artifacts":["entity_360.v1","answer.grounded.v1"],"summary":"Cliente Uno tiene deuda vencida","percentil_mora":90,"percentil_saldo":70,"text":"Cartera: percentil mora 90, percentil saldo 70, clientes similares con mora menor."}`
		case "smoke-auditor":
			stdout = `{"artifact_type":"auditor.findings.v1","artifacts":["auditor.findings.v1","data.gaps.v1"],"findings":[{"rule":"missing_contact_destination","severity":"high","description":"Falta email de contacto"}],"data_gaps":[{"type":"missing_contact_destination","field":"email","description":"Falta email de contacto"}]}`
		case "smoke-mecanico-resolve":
			stdout = `{"artifact_type":"mecanico.resolution_plan.v1","questions":[{"id":"q_email","gap_type":"missing_contact_destination","text":"¿Cuál es el email de contacto para Cliente Uno?"}]}`
		case "smoke-mecanico-draft":
			stdout = `{"artifact_type":"message.draft.v1","subject":"Recordatorio de pago","body":"Hola Cliente Uno, registramos una deuda pendiente.","to":"cliente@example.com","channel":"email"}`
		case "smoke-hosting-next":
			stdout = `{"id":"hosting_domain","text":"Veo que todavía no tengo conectado el hosting del negocio. Primero necesito el dominio principal o el host de cPanel.","framework":"hosting","capability":"credentials.cpanel.connect","field":"domain","field_label":"Dominio principal o host de cPanel","input_type":"text","placeholder":"tudominio.com","step":"cpanel_domain","required_artifact":"credentials.smtp","next_transition":"discover_cpanel"}`
		case "smoke-hosting-ingest":
			smtpConfigured = true
			stdout = `{"ok":true}`
		case "smoke-hosting-has":
			if smtpConfigured {
				stdout = `{"artifact_type":"credentials.status.v1","available":true,"ready":true,"verified":true,"capability":"credentials.smtp"}`
			} else {
				stdout = `{"artifact_type":"credentials.status.v1","available":false,"capability":"credentials.smtp"}`
			}
		case "smoke-mensajero":
			stdout = `{"artifact_type":"message.sent.v1","message_id":"msg_1","to":"cliente@example.com","channel":"email"}`
		default:
			t.Fatalf("unexpected smoke command args: %#v", args)
		}
		_ = json.NewEncoder(w).Encode(adapter.Response{Success: true, ExitCode: 0, Stdout: stdout})
	}))
	return &server{rootDir: root, allManifests: collectionSmokeManifests(), channel: adapter.New(channel.URL, "test-key")}, channel.Close
}

func collectionSmokeManifests() map[string]*manifest.Manifest {
	return map[string]*manifest.Manifest{
		"sabio": {
			Name: "sabio", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"dataset-export": {Args: []string{"-c", "smoke-sabio-dataset"}},
				"query":          {Args: []string{"-c", "smoke-sabio-query", "--business-id", "{params.business_id}", "--db", "{params.db}", "--semantic-pack", "{params.semantic_pack}", "--entity-ref", "{params.entity_ref}", "--entity-type", "{params.entity_type}"}, Params: []string{"business_id", "db", "semantic_pack", "entity_ref", "entity_type"}},
				"contact-lookup": {Args: []string{"-c", "smoke-contact-lookup"}, Params: []string{"entity_type", "entity_ref", "channel", "profile"}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "dataset.export", Command: "dataset-export", Requires: []string{"data.sqlite_db.v1"}, Produces: []string{"dataset.raw.v1", "external.api.dump.v1"}},
				{ID: "data.query.sql", Command: "query", Requires: []string{"data.sqlite_db.v1", "business.semantic_pack.v1"}, Produces: []string{"query.result.v1", "answer.grounded.v1"}},
				{ID: "data.entity_360", Command: "query", Requires: []string{"entity.ref.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"}, Produces: []string{"entity_360.v1", "answer.grounded.v1"}},
				{ID: "contact.lookup", Command: "contact-lookup", Requires: []string{"entity.ref.v1"}, Produces: []string{"contact.lookup.v1", "contact.destination.v1"}},
			},
		},
		"radar": {
			Name: "radar", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"prioritize":       {Args: []string{"-c", "smoke-radar"}},
				"deep-dive":        {Args: []string{"-c", "smoke-radar-deep", "--entity-ref", "{params.entity_ref}", "--entity-type", "{params.entity_type}", "--priority-list-path", "{params.priority_list_path}", "--priority-list-json", "{params.priority_list_json}", "--strategy-path", "{params.strategy_path}", "--strategy-json", "{params.strategy_json}", "--context-b64", "{params.context_b64}"}, Params: []string{"entity_ref", "entity_type", "priority_list_path", "priority_list_json", "strategy_path", "strategy_json", "context_b64"}, Defaults: map[string]string{"entity_ref": "", "entity_type": "", "priority_list_path": "", "priority_list_json": "", "strategy_path": "", "strategy_json": "", "context_b64": ""}},
				"analyze-followup": {Args: []string{"-c", "smoke-radar-followup", "--input", "{params.input}", "--turn-count", "{params.turn_count}", "--delegation-results-path", "{params.delegation_results_path}", "--delegation-results-json", "{params.delegation_results_json}", "--llm-followup-path", "{params.llm_followup_path}", "--llm-followup-json", "{params.llm_followup_json}"}, Params: []string{"input", "turn_count", "delegation_results_path", "delegation_results_json", "llm_followup_path", "llm_followup_json"}, Defaults: map[string]string{"input": "", "turn_count": "0", "delegation_results_path": "", "delegation_results_json": "", "llm_followup_path": "", "llm_followup_json": ""}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "collection.priority_list", Command: "prioritize", Requires: []string{"dataset.raw.v1", "business.semantic_pack.v1"}, Produces: []string{"collection.priority_list.v1", "collection.priority_item.v1", "entity.ref.v1", "strategy.recommendation.v1"}},
				{ID: "analysis.deep_dive", Command: "deep-dive", Requires: []string{"collection.priority_list.v1", "entity.ref.v1"}, Produces: []string{"analysis.case_review.v1", "answer.grounded.v1"}, Session: &manifest.CapabilitySession{Conversational: true, FollowupCommand: "analyze-followup", ContinueSignals: []string{"explica", "por qué", "evidencia", "contradicciones", "gaps"}, OperationalSignals: []string{"con eso basta", "avanza"}, ExitSignals: []string{"siguiente caso"}, AllowedDelegates: []string{"data.entity_360", "data.query.sql", "data.quality.audit"}, MaxTurns: 10}},
			},
		},
		"foco":     {Name: "foco", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"}, Commands: map[string]manifest.Command{"next-task": {Args: []string{"-c", "smoke-foco"}}}, Capabilities: []manifest.CapabilitySpec{{ID: "focus.next_collection_task", Command: "next-task", Requires: []string{"collection.priority_list.v1"}, Produces: []string{"focus.next_task.v1", "task.next", "entity.ref.v1", "action.options.v1"}}}},
		"auditor":  {Name: "auditor", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"}, Commands: map[string]manifest.Command{"scan": {Args: []string{"-c", "smoke-auditor"}}}, Capabilities: []manifest.CapabilitySpec{{ID: "data.quality.audit", Command: "scan", Requires: []string{"dataset.raw.v1"}, Produces: []string{"auditor.findings.v1", "data.gaps.v1"}}}},
		"mecanico": {Name: "mecanico", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"}, Commands: map[string]manifest.Command{"resolve-gaps": {Args: []string{"-c", "smoke-mecanico-resolve", "--data-gaps-path", "{params.data_gaps_path}", "--data-gaps-json", "{params.data_gaps_json}", "--findings-path", "{params.findings_path}", "--findings-json", "{params.findings_json}", "--entity-ref-path", "{params.entity_ref_path}", "--entity-ref-json", "{params.entity_ref_json}", "--scope-tables-path", "{params.scope_tables_path}", "--scope-tables-json", "{params.scope_tables_json}"}, Params: []string{"data_gaps_path", "data_gaps_json", "findings_path", "findings_json", "entity_ref_path", "entity_ref_json", "scope_tables_path", "scope_tables_json"}, Defaults: map[string]string{"data_gaps_path": "", "data_gaps_json": "", "findings_path": "", "findings_json": "", "entity_ref_path": "", "entity_ref_json": "", "scope_tables_path": "", "scope_tables_json": ""}}, "draft-email": {Args: []string{"-c", "smoke-mecanico-draft"}, Params: []string{"deudor", "to", "saldo", "dias_mora"}}}, Capabilities: []manifest.CapabilitySpec{{ID: "message.draft.collection_email", Command: "draft-email", Requires: []string{"entity.ref.v1", "contact.destination.v1"}, Produces: []string{"message.draft.v1"}}}},
		"hosting": {
			Name: "hosting", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"next-question": {Args: []string{"-c", "smoke-hosting-next"}},
				"ingest-answer": {Args: []string{"-c", "smoke-hosting-ingest"}, Params: []string{"question_id", "answer", "conv_id"}},
				"has-smtp":      {Args: []string{"-c", "smoke-hosting-has"}, Params: []string{"conv_id"}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "credentials.cpanel.connect", Command: "next-question", Produces: []string{"credentials.cpanel"}},
				{ID: "credentials.smtp.check", Command: "has-smtp", Requires: []string{"session.context.v1"}, Produces: []string{"credentials.status.v1"}},
			},
		},
		"mensajero": {Name: "mensajero", Cwd: ".", Binary: manifest.BinarySpec{Command: "/bin/sh"}, Commands: map[string]manifest.Command{"send": {Args: []string{"-c", "smoke-mensajero"}}}, Capabilities: []manifest.CapabilitySpec{{ID: "message.send", Command: "send", Requires: []string{"message.draft.v1", "credentials.smtp"}, Produces: []string{"message.sent.v1"}, Policies: []string{"external_side_effect"}}}},
	}
}

func TestMecanicoResolveGapsGeneratesConversationalQuestions(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "frameworkmecanico")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/frameworkmecanico")
	cmd.Dir = filepath.Join("..")
	if err := cmd.Run(); err != nil {
		t.Skipf("no se pudo compilar framework-mecanico: %v", err)
	}
	gapsJSON := `[{"type":"missing_contact","description":"falta email"},{"type":"missing_email","description":"sin destinatario"}]`
	entityJSON := `{"name":"Thiel-Effertz"}`
	out, err := exec.Command(bin, "resolve-gaps", "--data-gaps-json", gapsJSON, "--entity-ref-json", entityJSON).Output()
	if err != nil {
		t.Fatalf("resolve-gaps failed: %v, stderr: %s", err, string(err.(*exec.ExitError).Stderr))
	}
	var parsed map[string]interface{}
	if uerr := json.Unmarshal(out, &parsed); uerr != nil {
		t.Fatalf("parse failed: %v, stdout: %s", uerr, string(out))
	}
	if parsed["artifact_type"] != "mecanico.resolution_plan.v1" {
		t.Fatalf("expected artifact_type=mecanico.resolution_plan.v1, got %v", parsed["artifact_type"])
	}
	qs, _ := parsed["questions"].([]interface{})
	if len(qs) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(qs))
	}
	q0, _ := qs[0].(map[string]interface{})
	if q0["gap_type"] != "missing_contact" {
		t.Fatalf("expected first question gap_type=missing_contact, got %v", q0["gap_type"])
	}
}

func TestFocoGeneratesDynamicActionsFromStrategyBinary(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "foco")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/foco")
	cmd.Dir = filepath.Join("..")
	if err := cmd.Run(); err != nil {
		t.Skipf("no se pudo compilar framework-foco: %v", err)
	}
	strategyJSON := `{"recommendations":[{"action_id":"email_priority","label":"Enviar email prioritario","description":"Generado desde estrategia"}]}`
	out, err := exec.Command(bin, "next-task", "--entity-ref", "184", "--deudor", "Thiel-Effertz", "--strategy-json", strategyJSON).Output()
	if err != nil {
		t.Fatalf("next-task failed: %v, stderr: %s", err, string(err.(*exec.ExitError).Stderr))
	}
	var parsed map[string]interface{}
	if uerr := json.Unmarshal(out, &parsed); uerr != nil {
		t.Fatalf("parse failed: %v, stdout: %s", uerr, string(out))
	}
	if parsed["artifact_type"] != "focus.next_task.v1" {
		t.Fatalf("expected artifact_type=focus.next_task.v1, got %v", parsed["artifact_type"])
	}
	opts, _ := parsed["action_options"].([]interface{})
	if len(opts) != 2 {
		t.Fatalf("expected 2 action_options, got %d", len(opts))
	}
	first, _ := opts[0].(map[string]interface{})
	if first["id"] != "email_priority" {
		t.Fatalf("expected first option id=email_priority, got %v", first["id"])
	}
	if fromStrat, _ := first["from_strategy"].(bool); !fromStrat {
		t.Fatalf("expected first option from_strategy=true, got %v", first)
	}
}

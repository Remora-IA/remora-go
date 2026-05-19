package main

import (
	"strings"
	"testing"
)

func TestFormatFlowWorkbenchShowsAuthoredDerivedAndAmendments(t *testing.T) {
	text := formatFlowWorkbench(
		"Cobranza desde terminal",
		"Preparar cobranza asistida",
		&cliFlowManifest{
			ID:         "flow_cobranza_terminal",
			BusinessID: "biz-1",
			Lifecycle: cliFlowLifecycle{
				Entry:  cliFlowLifecycleBinding{Framework: "radar", Capability: "collection.priority_list"},
				Tutela: cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
			},
			Nodes: []cliFlowNode{
				{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
				{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
			},
			Derivation: &cliFlowDerivation{
				Amendments: []cliFlowAmendment{
					{Kind: "node_inserted", Summary: "Se propuso insertar foco.focus.next_collection_task como paso visible del plan ejecutable."},
				},
				Contracts: []cliFlowDerivedContract{
					{NodeID: "draft", Requires: []string{"entity.ref.v1"}, Produces: []string{"message.draft.v1"}},
				},
				Handoffs: []cliFlowDerivedHandoff{
					{FromNode: "prioritize", ToNode: "draft", Artifacts: []string{"entity.ref.v1"}},
				},
				Install: cliFlowInstallPreview{RequiresInstall: true, Capabilities: []string{"analysis.configure"}},
				Executable: cliFlowExecutablePlan{
					Lifecycle: cliFlowLifecycle{
						Entry:  cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
						Tutela: cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
					},
					Nodes: []cliFlowNode{
						{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list", Role: "bootstrap"},
						{ID: "node_foco_entry", Framework: "foco", Capability: "focus.next_collection_task", Role: "entry"},
						{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
					},
				},
			},
		},
		nil,
		&cliInstalledSnapshot{Installed: true, SchemaID: "schema_1"},
	)

	for _, want := range []string{"Authored", "Derived", "Enmiendas", "Contratos", "Handoffs", "Instalacion", "Lifecycle autoral", "Lifecycle derivado", "entry: radar.collection.priority_list", "tutela: foco.focus.next_collection_task", "node_foco_entry"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestParseCSVListDropsEmptyValues(t *testing.T) {
	got := parseCSVList(" message.draft.v1, , credentials.smtp ,,")
	if len(got) != 2 || got[0] != "message.draft.v1" || got[1] != "credentials.smtp" {
		t.Fatalf("got %#v", got)
	}
}

func TestBuildFlowCreateSuggestPayloadSubordinatesCapabilityHintToIntent(t *testing.T) {
	payload := buildFlowCreateSuggestPayload(flowCreateAnswers{
		BusinessID:      "biz-1",
		Name:            "Cobranza asistida",
		Description:     "Analizar cartera y preparar correos para revisión humana",
		CapabilityHint:  "message.send",
		SuccessCriteria: "cada caso termina con correo listo",
		AutonomyMode:    "approval",
	}, 6)

	if payload["description"] == "Capacidad inicial: message.send." {
		t.Fatalf("description should not collapse into capability hint: %#v", payload)
	}
	intentRaw, ok := payload["intent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected explicit intent payload, got %#v", payload)
	}
	if intentRaw["goal"] != "Cobranza asistida" {
		t.Fatalf("goal = %#v", intentRaw["goal"])
	}
	if intentRaw["capability_hint"] != "message.send" {
		t.Fatalf("capability hint = %#v", intentRaw["capability_hint"])
	}
	roles, ok := intentRaw["roles"].([]string)
	if !ok {
		t.Fatalf("roles = %#v", intentRaw["roles"])
	}
	if len(roles) == 0 || roles[0] != "analizar" || !containsString(roles, "redactar") {
		t.Fatalf("expected role-first ordering, got %#v", roles)
	}
}

func TestApplyFlowCreateIntentModelKeepsGoalIntentFirst(t *testing.T) {
	manifest := &cliFlowManifest{}
	applyFlowCreateIntentModel(manifest, flowCreateAnswers{
		Name:            "Cobranza",
		CapabilityHint:  "message.send",
		SuccessCriteria: "correo listo para cada caso",
		AutonomyMode:    "advisory",
		Description:     "Analizar cartera y preparar cobranza",
	})
	if manifest.Intent.Goal != "Cobranza" {
		t.Fatalf("goal = %q", manifest.Intent.Goal)
	}
	if manifest.Intent.SuccessCriteria != "correo listo para cada caso" {
		t.Fatalf("success = %q", manifest.Intent.SuccessCriteria)
	}
	if !containsString(manifest.Intent.Roles, "analizar") || !containsString(manifest.Intent.Roles, "redactar") {
		t.Fatalf("roles = %#v", manifest.Intent.Roles)
	}
	if !strings.Contains(strings.Join(manifest.Intent.Constraints, ","), "no_external_side_effect") {
		t.Fatalf("constraints = %#v", manifest.Intent.Constraints)
	}
	if strings.Contains(manifest.Intent.Description, "Capacidad inicial:") {
		t.Fatalf("description should remain intent-first, got %q", manifest.Intent.Description)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

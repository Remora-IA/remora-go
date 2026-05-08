package main

import (
	"io"
	"log"
	"strings"
	"testing"

	"channel/manifest"
)

func TestValidateFlowManifestValidChain(t *testing.T) {
	manifests := flowTestManifests()
	flow := flowManifest{
		ID: "cobranza",
		ProvidedArtifacts: []string{
			"data.sqlite_db.v1",
			"business.semantic_pack.v1",
			"message.draft.v1",
		},
		Policies: []string{
			"approval_required",
		},
		Nodes: []flowNode{
			{ID: "radar", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "foco", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "sabio", Framework: "sabio", Capability: "data.entity_360"},
			{ID: "hosting", Framework: "hosting", Capability: "credentials.smtp.import"},
			{ID: "mensajero", Framework: "mensajero", Capability: "message.send"},
		},
	}

	result := validateFlowManifest(flow, manifests)
	if !result.Valid {
		t.Fatalf("expected valid flow, errors=%#v", result.Errors)
	}
	wantProduced := map[string]bool{
		"collection.priority_list.v1": true,
		"focus.next_task.v1":          true,
		"entity.ref.v1":               true,
		"entity_360.v1":               true,
		"message.draft.v1":            true,
		"credentials.smtp":            true,
		"message.sent.v1":             true,
	}
	for _, artifact := range result.ProducedArtifacts {
		delete(wantProduced, artifact)
	}
	if len(wantProduced) > 0 {
		t.Fatalf("missing produced artifacts: %#v; got %#v", wantProduced, result.ProducedArtifacts)
	}
}

func TestValidateFlowManifestMissingRequirementWithProviderHint(t *testing.T) {
	manifests := flowTestManifests()
	flow := flowManifest{
		ID: "cobranza",
		Nodes: []flowNode{
			{ID: "mensajero", Framework: "mensajero", Capability: "message.send"},
		},
	}

	result := validateFlowManifest(flow, manifests)
	if result.Valid {
		t.Fatal("expected invalid flow")
	}
	var foundDraft, foundSMTP bool
	for _, issue := range result.Errors {
		if issue.Code != "node.requirement_missing" {
			continue
		}
		if issue.Message == "artifact/capability requerido no disponible antes del nodo: message.draft.v1" {
			foundDraft = true
		}
		if issue.Message == "artifact/capability requerido no disponible antes del nodo: credentials.smtp" {
			foundSMTP = true
			if len(issue.Hints) == 0 || !strings.HasPrefix(issue.Hints[0], "hosting.") {
				t.Fatalf("expected hosting provider hint, got %#v", issue.Hints)
			}
		}
	}
	if !foundDraft || !foundSMTP {
		t.Fatalf("expected missing draft and smtp errors, got %#v", result.Errors)
	}
}

func TestBuildCapabilityRegistryIncludesSemanticProduces(t *testing.T) {
	registry := buildCapabilityRegistry(flowTestManifests())
	providers := registry["message.sent"]
	if len(providers) == 0 {
		t.Fatal("expected message.sent provider")
	}
	if providers[0].Framework != "mensajero" {
		t.Fatalf("provider = %#v", providers[0])
	}
	entityProvider := registry["entity.ref.v1"]
	if len(entityProvider) == 0 || !providerListContainsFramework(entityProvider, "radar") {
		t.Fatalf("expected radar entity provider, got %#v", entityProvider)
	}
}

func TestValidateCobranzaFlowWithRepoManifests(t *testing.T) {
	t.Setenv("REMORA_ROOT", "")
	t.Setenv("CHANNEL_BASE_DIR", "")
	root := resolveRemoraRoot()
	loaded, skipped := initDriverRegistry(root, log.New(io.Discard, "", 0))
	if len(skipped) > 0 {
		t.Fatalf("expected repo manifests to load cleanly, skipped=%#v", skipped)
	}
	flow := flowManifest{
		ID:         "cobranza_chile_v1",
		BusinessID: "panalbit",
		Audience:   "collector",
		ProvidedArtifacts: []string{
			"data.sqlite_db.v1",
			"business.semantic_pack.v1",
			"message.draft.v1",
			"message.channel.v1",
			"credentials.smtp.input.v1",
		},
		Policies: []string{"approval_required", "trace_required"},
		Nodes: []flowNode{
			{ID: "priorizar", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "foco", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "analizar_deudor", Framework: "sabio", Capability: "data.entity_360"},
			{ID: "importar_smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
			{ID: "enviar", Framework: "mensajero", Capability: "message.send"},
		},
		Edges: []flowEdge{
			{From: "priorizar", To: "foco"},
			{From: "foco", To: "analizar_deudor"},
			{From: "analizar_deudor", To: "enviar"},
			{From: "importar_smtp", To: "enviar"},
		},
	}

	result := validateFlowManifest(flow, loaded)
	if !result.Valid {
		t.Fatalf("expected cobranza flow valid, errors=%#v", result.Errors)
	}
	for _, want := range []string{
		"collection.priority_list.v1",
		"focus.next_task.v1",
		"entity.ref.v1",
		"entity_360.v1",
		"message.draft.v1",
		"credentials.smtp",
		"message.sent.v1",
	} {
		if !containsString(result.ProducedArtifacts, want) {
			t.Fatalf("expected produced artifact %s, got %#v", want, result.ProducedArtifacts)
		}
	}
}

func TestValidateStaffScenariosAcrossBusinesses(t *testing.T) {
	t.Setenv("REMORA_ROOT", "")
	t.Setenv("CHANNEL_BASE_DIR", "")
	root := resolveRemoraRoot()
	loaded, skipped := initDriverRegistry(root, log.New(io.Discard, "", 0))
	if len(skipped) > 0 {
		t.Fatalf("expected repo manifests to load cleanly, skipped=%#v", skipped)
	}

	cases := []struct {
		name        string
		flow        flowManifest
		wantValid   bool
		wantMissing string
	}{
		{
			name: "collections flow prioritizes with Radar, focuses with Foco and sends approved draft",
			flow: flowManifest{
				ID:         "panalbit_collections",
				BusinessID: "panalbit",
				Audience:   "collector",
				ProvidedArtifacts: []string{
					"data.sqlite_db.v1",
					"business.semantic_pack.v1",
					"message.draft.v1",
					"message.channel.v1",
					"credentials.smtp.input.v1",
				},
				Policies: []string{"approval_required", "trace_required"},
				Nodes: []flowNode{
					{ID: "priorizar", Framework: "radar", Capability: "collection.priority_list"},
					{ID: "foco", Framework: "foco", Capability: "focus.next_collection_task"},
					{ID: "analizar_deudor", Framework: "sabio", Capability: "data.entity_360"},
					{ID: "importar_smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
					{ID: "enviar", Framework: "mensajero", Capability: "message.send"},
				},
			},
			wantValid: true,
		},
		{
			name: "retail support flow cannot use Sabio without semantic pack",
			flow: flowManifest{
				ID:         "retail_support",
				BusinessID: "retail_demo",
				Audience:   "support_agent",
				ProvidedArtifacts: []string{
					"data.sqlite_db.v1",
				},
				Nodes: []flowNode{
					{ID: "lookup_customer", Framework: "sabio", Capability: "data.entity_360"},
				},
			},
			wantValid:   false,
			wantMissing: "business.semantic_pack.v1",
		},
		{
			name: "retail notification flow can send provided draft without Sabio data layer",
			flow: flowManifest{
				ID:         "retail_notification",
				BusinessID: "retail_demo",
				Audience:   "support_agent",
				ProvidedArtifacts: []string{
					"business.context.v1",
					"message.channel.v1",
					"message.draft.v1",
					"credentials.smtp.input.v1",
				},
				Policies: []string{"approval_required"},
				Nodes: []flowNode{
					{ID: "smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
					{ID: "send", Framework: "mensajero", Capability: "message.send"},
				},
			},
			wantValid: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateFlowManifest(tc.flow, loaded)
			if result.Valid != tc.wantValid {
				t.Fatalf("valid=%v want %v errors=%#v", result.Valid, tc.wantValid, result.Errors)
			}
			if tc.wantMissing != "" && !validationErrorsContain(result.Errors, tc.wantMissing) {
				t.Fatalf("expected missing %q in errors %#v", tc.wantMissing, result.Errors)
			}
		})
	}
}

func TestValidateFlowCanUseAutoBusinessArtifacts(t *testing.T) {
	flow := flowManifest{
		ID:         "retail_support",
		BusinessID: "retail_demo",
		Audience:   "support_agent",
		Nodes: []flowNode{
			{ID: "lookup_customer", Framework: "sabio", Capability: "data.entity_360"},
		},
	}

	withoutArtifacts := validateFlowManifest(flow, flowTestManifests())
	if withoutArtifacts.Valid {
		t.Fatal("expected flow invalid without business artifacts")
	}

	withArtifacts := validateFlowManifestWithArtifacts(flow, flowTestManifests(), []string{
		"entity.ref.v1",
		"data.sqlite_db.v1",
		"business.semantic_pack.v1",
	})
	if !withArtifacts.Valid {
		t.Fatalf("expected flow valid with auto business artifacts, errors=%#v", withArtifacts.Errors)
	}
	for _, want := range []string{"data.sqlite_db.v1", "business.semantic_pack.v1"} {
		if !containsString(withArtifacts.ProvidedArtifacts, want) {
			t.Fatalf("expected provided artifact %s, got %#v", want, withArtifacts.ProvidedArtifacts)
		}
	}
}

func TestStaffScenarioFailsWhenBusinessDataIsNotReady(t *testing.T) {
	flow := flowManifest{
		ID:         "logistics_collections",
		BusinessID: "logistics_demo",
		Audience:   "ops_agent",
		Nodes: []flowNode{
			{ID: "priorizar", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "foco", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "analizar", Framework: "sabio", Capability: "data.entity_360"},
		},
	}

	result := validateFlowManifestWithArtifacts(flow, flowTestManifests(), []string{})
	if result.Valid {
		t.Fatal("expected invalid flow for business without data artifacts")
	}
	for _, want := range []string{"data.sqlite_db.v1", "business.semantic_pack.v1"} {
		if !validationErrorsContain(result.Errors, want) {
			t.Fatalf("expected missing %s in errors %#v", want, result.Errors)
		}
	}
}

func TestValidateFlowUsesEdgesForExecutionOrder(t *testing.T) {
	flow := flowManifest{
		ID: "staff_dag",
		ProvidedArtifacts: []string{
			"data.sqlite_db.v1",
			"business.semantic_pack.v1",
			"message.draft.v1",
		},
		Policies: []string{"approval_required"},
		Nodes: []flowNode{
			{ID: "send", Framework: "mensajero", Capability: "message.send"},
			{ID: "smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
			{ID: "entity", Framework: "sabio", Capability: "data.entity_360"},
			{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "radar", Framework: "radar", Capability: "collection.priority_list"},
		},
		Edges: []flowEdge{
			{From: "radar", To: "focus"},
			{From: "focus", To: "entity"},
			{From: "entity", To: "send"},
			{From: "smtp", To: "send"},
		},
	}

	result := validateFlowManifest(flow, flowTestManifests())
	if !result.Valid {
		t.Fatalf("expected DAG valid independent of node array order, errors=%#v", result.Errors)
	}
}

func TestValidateFlowRejectsGraphCycle(t *testing.T) {
	flow := flowManifest{
		ID: "cyclic",
		Nodes: []flowNode{
			{ID: "a", Framework: "hosting", Capability: "credentials.smtp.provision"},
			{ID: "b", Framework: "hosting", Capability: "credentials.smtp.provision"},
		},
		Edges: []flowEdge{
			{From: "a", To: "b"},
			{From: "b", To: "a"},
		},
	}

	result := validateFlowManifest(flow, flowTestManifests())
	if result.Valid {
		t.Fatal("expected cyclic graph to be invalid")
	}
	if !validationErrorsContainCode(result.Errors, "flow.graph_cycle") {
		t.Fatalf("expected graph cycle error, got %#v", result.Errors)
	}
}

func TestSimulateFlowManifestBuildsDryRunTimeline(t *testing.T) {
	req := flowSimulationRequest{
		Flow: flowManifest{
			ID:       "collections_email_flow",
			Policies: []string{"approval_required"},
			ProvidedArtifacts: []string{
				"message.draft.v1",
			},
			Nodes: []flowNode{
				{ID: "radar", Framework: "radar", Capability: "collection.priority_list"},
				{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"},
				{ID: "entity", Framework: "sabio", Capability: "data.entity_360"},
				{ID: "send", Framework: "mensajero", Capability: "message.send"},
				{ID: "smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
			},
			Edges: []flowEdge{
				{From: "radar", To: "focus"},
				{From: "focus", To: "entity"},
				{From: "entity", To: "send"},
				{From: "smtp", To: "send"},
			},
		},
		Input: "prepara seguimiento",
		FixtureArtifacts: []string{
			"data.sqlite_db.v1",
			"business.semantic_pack.v1",
		},
	}

	result := simulateFlowManifest(req, flowTestManifests(), nil)
	if !result.Valid {
		t.Fatalf("expected simulation valid, validation=%#v", result.Validation.Errors)
	}
	if len(result.Timeline) != 5 {
		t.Fatalf("timeline len = %d", len(result.Timeline))
	}
	if !containsString(result.ExecutionOrder, "send") {
		t.Fatalf("expected execution order, got %#v", result.ExecutionOrder)
	}
	for _, want := range []string{"collection.priority_list.v1", "focus.next_task.v1", "entity.ref.v1", "entity_360.v1", "message.draft.v1", "credentials.smtp", "message.sent.v1"} {
		if !containsString(result.Artifacts, want) {
			t.Fatalf("expected artifact %s, got %#v", want, result.Artifacts)
		}
	}
	for _, step := range result.Timeline {
		if step.Status != "would_run" {
			t.Fatalf("expected would_run step, got %#v", step)
		}
	}
}

func TestSimulateFlowManifestBlocksMissingArtifacts(t *testing.T) {
	req := flowSimulationRequest{
		Flow: flowManifest{
			ID: "blocked_send",
			Nodes: []flowNode{
				{ID: "send", Framework: "mensajero", Capability: "message.send"},
			},
		},
	}

	result := simulateFlowManifest(req, flowTestManifests(), nil)
	if result.Valid {
		t.Fatal("expected invalid simulation")
	}
	if len(result.Timeline) != 1 || result.Timeline[0].Status != "blocked" {
		t.Fatalf("expected blocked step, got %#v", result.Timeline)
	}
	for _, want := range []string{"message.draft.v1", "credentials.smtp"} {
		if !containsString(result.Timeline[0].MissingArtifacts, want) {
			t.Fatalf("expected missing %s, got %#v", want, result.Timeline[0].MissingArtifacts)
		}
	}
}

func flowTestManifests() map[string]*manifest.Manifest {
	return map[string]*manifest.Manifest{
		"sabio": {
			Name: "sabio",
			Commands: map[string]manifest.Command{
				"query": {Args: []string{"query"}, Params: []string{}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:        "data.entity_360",
					Command:   "query",
					Inputs:    []string{"entity.ref.v1", "user.question", "business.context.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"},
					Requires:  []string{"entity.ref.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"},
					Produces:  []string{"entity_360.v1", "answer.grounded.v1"},
					Execution: "scoped_readonly_sqlite",
					Policies:  []string{"readonly_sql", "scope_required"},
				},
			},
		},
		"radar": {
			Name: "radar",
			Commands: map[string]manifest.Command{
				"prioritize": {Args: []string{"prioritize"}, Params: []string{}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:        "collection.priority_list",
					Command:   "prioritize",
					Inputs:    []string{"business.context.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"},
					Requires:  []string{"data.sqlite_db.v1", "business.semantic_pack.v1"},
					Produces:  []string{"collection.priority_list.v1", "collection.priority_item.v1", "entity.ref.v1"},
					Execution: "deterministic_business_priority_scoring",
					Policies:  []string{"readonly_sql", "business_context_required", "no_silent_fallback"},
				},
			},
		},
		"foco": {
			Name: "foco",
			Commands: map[string]manifest.Command{
				"next-task": {Args: []string{"next-task"}, Params: []string{}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:        "focus.next_collection_task",
					Command:   "next-task",
					Inputs:    []string{"collection.priority_list.v1", "business.context.v1", "session.context.v1"},
					Requires:  []string{"collection.priority_list.v1"},
					Produces:  []string{"focus.next_task.v1", "task.next", "entity.ref.v1"},
					Execution: "operational_focus_selection",
					Policies:  []string{"trace_required"},
				},
			},
		},
		"hosting": {
			Name: "hosting",
			Commands: map[string]manifest.Command{
				"provision-smtp": {Args: []string{"provision-smtp"}, Params: []string{}},
				"import-smtp":    {Args: []string{"import-smtp"}, Params: []string{}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:        "credentials.smtp.provision",
					Command:   "provision-smtp",
					Produces:  []string{"credentials.smtp"},
					Execution: "vault_write",
					Policies:  []string{"admin_only"},
				},
				{
					ID:        "credentials.smtp.import",
					Command:   "import-smtp",
					Produces:  []string{"credentials.smtp"},
					Execution: "vault_write",
					Policies:  []string{"admin_only"},
				},
			},
		},
		"mensajero": {
			Name: "mensajero",
			Commands: map[string]manifest.Command{
				"send": {Args: []string{"send"}, Params: []string{}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:        "message.send",
					Command:   "send",
					Inputs:    []string{"message.draft.v1", "credentials.smtp"},
					Produces:  []string{"message.sent.v1"},
					Execution: "smtp_send",
					Policies:  []string{"external_side_effect"},
				},
			},
			CapabilitiesSemantic: manifest.CapabilitiesSemantic{
				Produces: []string{"message.sent"},
			},
		},
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

func providerListContainsFramework(items []capabilityProviderInfo, framework string) bool {
	for _, item := range items {
		if item.Framework == framework {
			return true
		}
	}
	return false
}

func validationErrorsContain(errors []flowValidationIssue, needle string) bool {
	for _, issue := range errors {
		if strings.Contains(issue.Message, needle) {
			return true
		}
	}
	return false
}

func validationErrorsContainCode(errors []flowValidationIssue, code string) bool {
	for _, issue := range errors {
		if issue.Code == code {
			return true
		}
	}
	return false
}

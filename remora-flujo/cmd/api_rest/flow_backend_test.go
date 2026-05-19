package main

import (
	"io"
	"log"
	"reflect"
	"strings"
	"testing"

	"channel/manifest"
)

func TestFindProviderForCapability(t *testing.T) {
	s := &server{allManifests: map[string]*manifest.Manifest{
		"alpha": {Name: "alpha", Capabilities: []manifest.CapabilitySpec{{ID: "data.read"}}},
		"beta":  {Name: "beta", Capabilities: []manifest.CapabilitySpec{{ID: "contact.lookup"}}},
		"gamma": {Name: "gamma", Capabilities: []manifest.CapabilitySpec{{ID: "message.send"}}},
	}}

	m, name, ok := s.findProviderForCapability("contact.lookup")
	if !ok {
		t.Fatal("expected provider")
	}
	if name != "beta" || m == nil || m.Name != "beta" {
		t.Fatalf("provider name=%q manifest=%#v", name, m)
	}
}

func TestFindProviderForCapabilityNotFound(t *testing.T) {
	s := &server{allManifests: map[string]*manifest.Manifest{
		"alpha": {Name: "alpha", Capabilities: []manifest.CapabilitySpec{{ID: "data.read"}}},
	}}

	if m, name, ok := s.findProviderForCapability("missing.capability"); ok || m != nil || name != "" {
		t.Fatalf("expected not found, got ok=%v name=%q manifest=%#v", ok, name, m)
	}
}

func TestResolutionModeFromPolicies(t *testing.T) {
	tests := []struct {
		name     string
		policies []string
		want     string
	}{
		{name: "interactive", policies: []string{"resolution_interactive"}, want: resolutionInteractive},
		{name: "hybrid", policies: []string{"resolution_hybrid"}, want: resolutionHybrid},
		{name: "autonomous", policies: []string{"resolution_autonomous"}, want: resolutionAutonomous},
		{name: "default", policies: nil, want: resolutionAutonomous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolutionModeFromPolicies(tt.policies); got != tt.want {
				t.Fatalf("mode=%q want %q", got, tt.want)
			}
		})
	}
}

func TestGapResolutionRegistryUsesCapabilityNotName(t *testing.T) {
	if _, ok := reflect.TypeOf(gapResolution{}).FieldByName("Framework"); ok {
		t.Fatal("gapResolution should not contain Framework")
	}
	for _, entry := range gapResolutionRegistry() {
		if strings.TrimSpace(entry.Capability) == "" {
			t.Fatalf("registry entry missing capability: %#v", entry)
		}
		if strings.TrimSpace(entry.Produces) == "" {
			t.Fatalf("registry entry missing produces: %#v", entry)
		}
	}
}

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
			{ID: "exportar", Framework: "sabio", Capability: "dataset.export"},
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

func TestPrepareFlowManifestLifecyclePromotesPriorityListBeforeFoco(t *testing.T) {
	flow := flowManifest{
		ID: "cobranza",
		Nodes: []flowNode{
			{ID: "configure", Framework: "radar", Capability: "analysis.configure", Role: flowRoleBootstrap},
			{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task", Role: flowRoleEntry},
			{ID: "hosting", Framework: "hosting", Capability: "credentials.smtp.check", Role: flowRolePipeline},
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list", Role: flowRolePipeline},
		},
	}

	prepareFlowManifestLifecycle(&flow, flowTestManifests())

	positions := map[string]int{}
	for i, node := range flow.Nodes {
		positions[node.ID] = i
	}
	if positions["prioritize"] > positions["focus"] {
		t.Fatalf("expected priority list before focus, nodes=%#v", flow.Nodes)
	}
	if flow.Nodes[positions["prioritize"]].Role != flowRoleBootstrap {
		t.Fatalf("expected priority list bootstrap role, got %#v", flow.Nodes[positions["prioritize"]])
	}
	if len(flow.Edges) == 0 || flow.Edges[positions["focus"]-1].To != "focus" {
		t.Fatalf("expected rebuilt edges into focus, edges=%#v nodes=%#v", flow.Edges, flow.Nodes)
	}
}

func TestPrepareFlowManifestLifecycleHonorsConfiguredEntry(t *testing.T) {
	flow := flowManifest{
		ID:         "cobranza",
		BusinessID: "biz-1",
		Lifecycle:  flowLifecycle{Entry: flowLifecycleEntry{Framework: "radar", Capability: "collection.priority_list"}},
		Nodes: []flowNode{
			{ID: "configure", Framework: "radar", Capability: "analysis.configure"},
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}

	prepareFlowManifestLifecycle(&flow, flowTestManifests())

	for _, node := range flow.Nodes {
		if node.ID == "foco_entry" {
			t.Fatalf("configured non-Foco entry should not inject foco_entry: %#v", flow.Nodes)
		}
		if node.ID == "prioritize" && node.Role != flowRoleEntry {
			t.Fatalf("configured entry role=%q want entry; nodes=%#v", node.Role, flow.Nodes)
		}
	}
}

func TestPrepareFlowManifestLifecycleHonorsConfiguredTutelaPreference(t *testing.T) {
	flow := flowManifest{
		ID:         "cobranza",
		BusinessID: "biz-1",
		Lifecycle: flowLifecycle{
			Tutela: flowLifecycleBinding{Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
		Nodes: []flowNode{
			{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}

	prepareFlowManifestLifecycle(&flow, flowTestManifests())

	var focusRole, draftRole string
	for _, node := range flow.Nodes {
		switch node.ID {
		case "focus":
			focusRole = node.Role
		case "draft":
			draftRole = node.Role
		}
	}
	if draftRole != flowRoleEntry {
		t.Fatalf("configured tutela should be respected as explicit entry preference, nodes=%#v", flow.Nodes)
	}
	if focusRole == flowRoleEntry {
		t.Fatalf("configured tutela should prevent foco from remaining implicit entry, nodes=%#v", flow.Nodes)
	}
	if flow.Lifecycle.Entry.Framework != "mecanico" || flow.Lifecycle.Entry.Capability != "message.draft.collection_email" {
		t.Fatalf("expected derived lifecycle entry to become explicit from tutela, got %#v", flow.Lifecycle)
	}
}

func TestDeriveFlowManifestReportsGroundingAndVisibleAmendments(t *testing.T) {
	flow := flowManifest{
		ID:         "cobranza",
		BusinessID: "biz-1",
		Intent:     flowIntent{Goal: "enviar correos a la gente que me debe"},
		Nodes: []flowNode{
			{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
		},
	}

	derivation := deriveFlowManifest(flow, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
		Sources:    map[string]string{"business.semantic_pack.v1": "/tmp/pack.json"},
	})
	if derivation == nil {
		t.Fatal("expected derivation")
	}
	if len(derivation.Executable.Nodes) != 3 {
		t.Fatalf("expected executable plan with preserved nodes, got %#v", derivation.Executable.Nodes)
	}
	if !containsString(derivation.Grounding.MissingArtifacts, "contact.destination.v1") {
		t.Fatalf("expected missing contact grounding, got %#v", derivation.Grounding)
	}
	if !containsString(derivation.Grounding.UniversalRoles, "redactar") || !containsString(derivation.Grounding.UniversalRoles, "analizar") {
		t.Fatalf("expected universal roles in grounding, got %#v", derivation.Grounding.UniversalRoles)
	}
	var promotedPriority, reordered bool
	for _, amendment := range derivation.Amendments {
		if amendment.Kind == "role_changed" && amendment.NodeID == "prioritize" && amendment.After == flowRoleBootstrap {
			promotedPriority = true
		}
		if amendment.Kind == "nodes_reordered" {
			reordered = true
		}
	}
	if !promotedPriority || !reordered {
		t.Fatalf("expected visible amendments, got %#v", derivation.Amendments)
	}
}

func TestDeriveFlowManifestMakesLifecycleDecisionExplicitAndVisible(t *testing.T) {
	flow := flowManifest{
		ID:         "cobranza_lifecycle",
		BusinessID: "biz-1",
		Intent:     flowIntent{Goal: "priorizar y preparar cobranza"},
		Nodes: []flowNode{
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}

	derivation := deriveFlowManifest(flow, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
	})
	if derivation == nil {
		t.Fatal("expected derivation")
	}
	if derivation.Executable.Lifecycle.Entry.Framework == "" || derivation.Executable.Lifecycle.Entry.Capability == "" {
		t.Fatalf("expected explicit derived entry lifecycle, got %#v", derivation.Executable.Lifecycle)
	}
	if derivation.Executable.Lifecycle.Tutela.Framework == "" || derivation.Executable.Lifecycle.Tutela.Capability == "" {
		t.Fatalf("expected explicit derived tutela lifecycle, got %#v", derivation.Executable.Lifecycle)
	}
	var sawEntryAmendment, sawTutelaAmendment bool
	for _, amendment := range derivation.Amendments {
		if amendment.Kind == "lifecycle_entry_changed" {
			sawEntryAmendment = true
		}
		if amendment.Kind == "lifecycle_tutela_changed" {
			sawTutelaAmendment = true
		}
	}
	if !sawEntryAmendment || !sawTutelaAmendment {
		t.Fatalf("expected lifecycle corrections as visible amendments, got %#v", derivation.Amendments)
	}
}

func TestDeriveFlowManifestExposesContractsHandoffsAndInstallPreview(t *testing.T) {
	manifests := flowTestManifests()
	manifests["radar"] = &manifest.Manifest{
		Name: "radar",
		Commands: map[string]manifest.Command{
			"configure-analysis": {Args: []string{"configure-analysis"}, Params: []string{}},
			"prioritize":         {Args: []string{"prioritize"}, Params: []string{}},
		},
		Capabilities: []manifest.CapabilitySpec{
			{
				ID:       "analysis.configure",
				Command:  "configure-analysis",
				Requires: []string{"business.semantic_pack.v1"},
				Produces: []string{"analysis.schema.v1", "analysis.plan.v1"},
				Policies: []string{"install_once"},
			},
			{
				ID:       "collection.priority_list",
				Command:  "prioritize",
				Requires: []string{"dataset.raw.v1", "business.semantic_pack.v1"},
				Produces: []string{"collection.priority_list.v1", "collection.priority_item.v1", "entity.ref.v1"},
			},
		},
	}
	flow := flowManifest{
		ID:         "cobranza_instalable",
		BusinessID: "biz-1",
		Nodes: []flowNode{
			{ID: "configure", Framework: "radar", Capability: "analysis.configure"},
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}

	derivation := deriveFlowManifest(flow, manifests, businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
	})
	if derivation == nil {
		t.Fatal("expected derivation")
	}
	if !derivation.Install.RequiresInstall || !containsString(derivation.Install.Capabilities, "analysis.configure") {
		t.Fatalf("expected install preview for analysis.configure, got %#v", derivation.Install)
	}
	if len(derivation.Contracts) == 0 {
		t.Fatal("expected derived contracts")
	}
	var sawDraftContract bool
	for _, contract := range derivation.Contracts {
		if contract.NodeID != "draft" {
			continue
		}
		sawDraftContract = true
		if !containsString(contract.Produces, "message.draft.v1") {
			t.Fatalf("expected draft contract produces message.draft.v1, got %#v", contract)
		}
	}
	if !sawDraftContract {
		t.Fatalf("missing draft contract in %#v", derivation.Contracts)
	}
	var sawPriorityHandoff bool
	for _, handoff := range derivation.Handoffs {
		if handoff.FromNode != "prioritize" {
			continue
		}
		sawPriorityHandoff = true
		if len(handoff.Artifacts) == 0 {
			t.Fatalf("expected explicit handoff artifacts, got %#v", handoff)
		}
	}
	if !sawPriorityHandoff {
		t.Fatalf("expected handoff leaving prioritize, got %#v", derivation.Handoffs)
	}
}

func TestCompileFlowManifestReturnsStableCompiledPlanIdentity(t *testing.T) {
	flow := flowManifest{
		ID:         "cobranza",
		BusinessID: "biz-1",
		Intent:     flowIntent{Goal: "priorizar y redactar"},
		Nodes: []flowNode{
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}
	first := compileFlowManifest(flow, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
	})
	second := compileFlowManifest(flow, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
	})
	if first.Compiled.ID == "" || first.Compiled.ID != second.Compiled.ID {
		t.Fatalf("compiled IDs should be stable, got %q and %q", first.Compiled.ID, second.Compiled.ID)
	}
	if len(first.Compiled.Flow.Nodes) == 0 {
		t.Fatalf("expected compiled executable nodes, got %#v", first.Compiled.Flow.Nodes)
	}
	if first.Derivation == nil || len(first.Derivation.Executable.Nodes) == 0 {
		t.Fatalf("expected derivation executable, got %#v", first.Derivation)
	}
}

func TestBuildFlowSuggestionProposalReturnsExplicitDesign(t *testing.T) {
	req := flowSuggestRequest{
		BusinessID:  "biz-1",
		Name:        "Analizar cobranzas",
		Description: "Quiero revisar cartera y luego preparar correos.",
	}
	suggestions := []flowCapabilitySuggestion{
		{Framework: "radar", Capability: "collection.priority_list"},
		{Framework: "mecanico", Capability: "message.draft.collection_email"},
	}
	proposal := buildFlowSuggestionProposal(req, suggestions, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
	})
	if proposal == nil {
		t.Fatal("expected proposal")
	}
	if proposal.Manifest.Intent.Goal != "Analizar cobranzas" {
		t.Fatalf("unexpected intent %#v", proposal.Manifest.Intent)
	}
	if len(proposal.Manifest.Nodes) != 2 {
		t.Fatalf("proposal manifest=%#v", proposal.Manifest)
	}
	if proposal.Derivation == nil || len(proposal.Derivation.Amendments) == 0 {
		t.Fatalf("expected explicit derivation with amendments, got %#v", proposal.Derivation)
	}
	if proposal.Compiled.ID == "" || len(proposal.Compiled.Flow.Nodes) == 0 {
		t.Fatalf("expected compiled plan in proposal, got %#v", proposal.Compiled)
	}
}

func TestBuildFlowSuggestionProposalCanComposeFromIntentRolesWithoutCapabilityHint(t *testing.T) {
	req := flowSuggestRequest{
		BusinessID:  "biz-1",
		Name:        "Cobranza asistida",
		Description: "Quiero analizar la cartera y redactar correos para los casos prioritarios.",
		Intent: flowIntent{
			Goal:         "Reducir mora con mensajes listos para revisión",
			OperatorRole: "collector",
			Roles:        []string{"analizar", "redactar"},
		},
	}
	suggestions := []flowCapabilitySuggestion{
		{Framework: "mensajero", Capability: "message.send"},
		{Framework: "sabio", Capability: "data.entity_360"},
		{Framework: "mecanico", Capability: "message.draft.collection_email"},
	}
	proposal := buildFlowSuggestionProposal(req, suggestions, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1", "contact.destination.v1"},
	})
	if proposal == nil {
		t.Fatal("expected proposal")
	}
	if proposal.IntentPlan.Goal != "Reducir mora con mensajes listos para revisión" {
		t.Fatalf("intent plan goal = %q", proposal.IntentPlan.Goal)
	}
	if got := proposal.Manifest.Intent.Goal; got != "Reducir mora con mensajes listos para revisión" {
		t.Fatalf("manifest goal = %q", got)
	}
	if !containsString(proposal.Manifest.Intent.Roles, "analizar") || !containsString(proposal.Manifest.Intent.Roles, "redactar") {
		t.Fatalf("expected authored manifest to preserve intent roles, got %#v", proposal.Manifest.Intent.Roles)
	}
	if len(proposal.Manifest.Nodes) < 2 {
		t.Fatalf("expected bound nodes, got %#v", proposal.Manifest.Nodes)
	}
	if proposal.Manifest.Nodes[0].Capability != "data.entity_360" || proposal.Manifest.Nodes[1].Capability != "message.draft.collection_email" {
		t.Fatalf("expected intent-first binding analyze->draft, got %#v", proposal.Manifest.Nodes)
	}
}

func TestBuildFlowSuggestionProposalSubordinatesCapabilityHintToIntentRoles(t *testing.T) {
	req := flowSuggestRequest{
		BusinessID:     "biz-1",
		Name:           "Cobranza guiada",
		Description:    "Necesito revisar cartera y preparar un borrador antes de cualquier envío.",
		CapabilityHint: "message.send",
		Intent: flowIntent{
			Goal:         "Preparar borradores revisables",
			OperatorRole: "collector",
			Roles:        []string{"analizar", "redactar"},
		},
	}
	suggestions := []flowCapabilitySuggestion{
		{Framework: "mensajero", Capability: "message.send"},
		{Framework: "mecanico", Capability: "message.draft.collection_email"},
		{Framework: "sabio", Capability: "data.entity_360"},
	}
	proposal := buildFlowSuggestionProposal(req, suggestions, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1", "contact.destination.v1"},
	})
	if proposal == nil {
		t.Fatal("expected proposal")
	}
	if proposal.IntentPlan.CapabilityHint != "message.send" {
		t.Fatalf("capability hint = %q", proposal.IntentPlan.CapabilityHint)
	}
	if proposal.Manifest.Intent.Goal == "message.send" {
		t.Fatalf("goal should remain intent-first, got %#v", proposal.Manifest.Intent)
	}
	for _, node := range proposal.Manifest.Nodes {
		if node.Capability == "message.send" {
			t.Fatalf("capability hint should not dominate authored proposal, got %#v", proposal.Manifest.Nodes)
		}
	}
}

func TestBuildFlowSuggestionProposalExposesRoleBindingsAheadOfNodes(t *testing.T) {
	req := flowSuggestRequest{
		BusinessID:  "biz-1",
		Name:        "Cobranza asistida",
		Description: "Analizar cartera y redactar correos para los casos prioritarios.",
		Intent: flowIntent{
			Goal:         "Reducir mora con mensajes listos para revisión",
			OperatorRole: "collector",
			Roles:        []string{"analizar", "redactar"},
		},
	}
	suggestions := []flowCapabilitySuggestion{
		{Framework: "mensajero", Capability: "message.send"},
		{Framework: "sabio", Capability: "data.entity_360"},
		{Framework: "mecanico", Capability: "message.draft.collection_email"},
	}
	proposal := buildFlowSuggestionProposal(req, suggestions, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1", "contact.destination.v1"},
	})
	if proposal == nil {
		t.Fatal("expected proposal")
	}
	if len(proposal.Bindings) != 2 {
		t.Fatalf("expected explicit role bindings before nodes, got %#v", proposal.Bindings)
	}
	if proposal.Bindings[0].Role != "analizar" || proposal.Bindings[0].Capability != "data.entity_360" {
		t.Fatalf("expected analyze role to bind first, got %#v", proposal.Bindings)
	}
	if proposal.Bindings[1].Role != "redactar" || proposal.Bindings[1].Capability != "message.draft.collection_email" {
		t.Fatalf("expected draft role binding second, got %#v", proposal.Bindings)
	}
	if proposal.Manifest.Nodes[0].Capability != proposal.Bindings[0].Capability || proposal.Manifest.Nodes[1].Capability != proposal.Bindings[1].Capability {
		t.Fatalf("authored nodes should be materialized from explicit bindings, bindings=%#v nodes=%#v", proposal.Bindings, proposal.Manifest.Nodes)
	}
}

func TestValidateFlowManifestMissingRequirementAllowsRuntimeCredentials(t *testing.T) {
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
	var foundDraft bool
	for _, issue := range result.Errors {
		if issue.Code != "node.requirement_missing" {
			continue
		}
		if issue.Message == "artifact/capability requerido no disponible antes del nodo: message.draft.v1" {
			foundDraft = true
		}
		if strings.Contains(issue.Message, "credentials.smtp") {
			t.Fatalf("credentials.smtp should be resolved at runtime via readiness/preflight, got %#v", issue)
		}
	}
	if !foundDraft {
		t.Fatalf("expected missing draft error, got %#v", result.Errors)
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
			{ID: "exportar", Framework: "sabio", Capability: "dataset.export"},
			{ID: "priorizar", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "foco", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "analizar_deudor", Framework: "sabio", Capability: "data.entity_360"},
			{ID: "importar_smtp", Framework: "hosting", Capability: "credentials.smtp.import"},
			{ID: "enviar", Framework: "mensajero", Capability: "message.send"},
		},
		Edges: []flowEdge{
			{From: "exportar", To: "priorizar"},
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
					{ID: "exportar", Framework: "sabio", Capability: "dataset.export"},
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
	for _, want := range []string{"dataset.raw.v1", "business.semantic_pack.v1"} {
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

func TestValidateFlowManifestUsesMecanicoForDraftArtifact(t *testing.T) {
	flow := flowManifest{
		ID: "cobranza_draft_real",
		ProvidedArtifacts: []string{
			"data.sqlite_db.v1",
			"business.semantic_pack.v1",
		},
		Nodes: []flowNode{
			{ID: "radar", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "focus", Framework: "foco", Capability: "focus.next_collection_task"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
			{ID: "send", Framework: "mensajero", Capability: "message.send"},
		},
		Edges: []flowEdge{
			{From: "radar", To: "focus"},
			{From: "focus", To: "draft"},
			{From: "draft", To: "send"},
		},
	}
	result := validateFlowManifest(flow, flowTestManifests())
	if !result.Valid {
		t.Fatalf("expected valid flow using mecanico draft capability, errors=%#v", result.Errors)
	}
	if !containsString(result.ProducedArtifacts, "message.draft.v1") {
		t.Fatalf("expected message.draft.v1 in produced artifacts, got %#v", result.ProducedArtifacts)
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
	if !containsString(result.Timeline[0].MissingArtifacts, "message.draft.v1") {
		t.Fatalf("expected missing message.draft.v1, got %#v", result.Timeline[0].MissingArtifacts)
	}
	if containsString(result.Timeline[0].MissingArtifacts, "credentials.smtp") {
		t.Fatalf("credentials.smtp should be treated as runtime-resolvable, got %#v", result.Timeline[0].MissingArtifacts)
	}
}

func flowTestManifests() map[string]*manifest.Manifest {
	return map[string]*manifest.Manifest{
		"sabio": {
			Name: "sabio",
			Commands: map[string]manifest.Command{
				"query":          {Args: []string{"query"}, Params: []string{}},
				"dataset-export": {Args: []string{"dataset-export"}, Params: []string{}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:        "data.entity_360",
					Command:   "query",
					Inputs:    []string{"entity.ref.v1", "user.question", "business.context.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"},
					Requires:  []string{"entity.ref.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"},
					Produces:  []string{"entity_360.v1", "answer.grounded.v1"},
					Execution: "scoped_readonly_sqlite",
					Policies:  []string{"readonly_sql", "scope_required", "business_sqlite_param"},
				},
				{
					ID:        "dataset.export",
					Command:   "dataset-export",
					Inputs:    []string{"data.sqlite_db.v1", "business.semantic_pack.v1"},
					Requires:  []string{"data.sqlite_db.v1"},
					Produces:  []string{"dataset.raw.v1", "external.api.dump.v1"},
					Execution: "deterministic_sqlite_readonly_export",
					Policies:  []string{"readonly_sql", "safe_for_runtime", "business_sqlite_param", "data_mediator"},
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
					Inputs:    []string{"business.context.v1", "dataset.raw.v1", "business.semantic_pack.v1"},
					Requires:  []string{"dataset.raw.v1", "business.semantic_pack.v1"},
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
					Policies:  []string{"trace_required", "entrypoint", "task_owner", "flow_state_scoped"},
				},
			},
		},
		"mecanico": {
			Name: "mecanico",
			Commands: map[string]manifest.Command{
				"draft-email": {Args: []string{"draft-email", "--deudor", "{params.deudor}", "--to", "{params.to}", "--saldo", "{params.saldo}", "--dias-mora", "{params.dias_mora}"}, Params: []string{"deudor", "to", "saldo", "dias_mora"}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{
					ID:       "message.draft.collection_email",
					Command:  "draft-email",
					Inputs:   []string{"entity.ref.v1", "collection.priority_item.v1", "contact.destination.v1"},
					Requires: []string{"entity.ref.v1", "contact.destination.v1"},
					Produces: []string{"message.draft.v1"},
					Policies: []string{"no_external_side_effect"},
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
					Policies:  []string{"admin_only", "vault_scoped"},
				},
				{
					ID:        "credentials.smtp.import",
					Command:   "import-smtp",
					Produces:  []string{"credentials.smtp"},
					Execution: "vault_write",
					Policies:  []string{"admin_only", "vault_scoped"},
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
					Policies:  []string{"external_side_effect", "vault_scoped"},
				},
			},
			CapabilitiesSemantic: manifest.CapabilitiesSemantic{
				Produces: []string{"message.sent"},
			},
		},
	}
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

func TestNormalizeFlowLifecycleRoles(t *testing.T) {
	flow := flowManifest{Nodes: []flowNode{
		{ID: "radar", Framework: "radar", Capability: "collection.priority_list"},
		{ID: "foco", Framework: "foco", Capability: "focus.next_collection_task"},
		{ID: "sabio", Framework: "sabio", Capability: "data.entity_360"},
	}}
	normalizeFlowLifecycleRoles(&flow)
	if flow.Nodes[0].Role != flowRoleBootstrap {
		t.Fatalf("first node role = %q want bootstrap", flow.Nodes[0].Role)
	}
	if flow.Nodes[1].Role != flowRoleEntry {
		t.Fatalf("second node role = %q want entry", flow.Nodes[1].Role)
	}
	if flow.Nodes[2].Role != flowRolePipeline {
		t.Fatalf("third node role = %q want pipeline", flow.Nodes[2].Role)
	}
}

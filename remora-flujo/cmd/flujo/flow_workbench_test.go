package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunFlowWorkbenchCompileShowsCompiledSections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/flows/flw_1":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowRecord{
				ID:          "flw_1",
				BusinessID:  "biz-1",
				Name:        "Cobranza terminal",
				Description: "Preparar cobranza",
				Status:      "draft",
				Manifest: &cliFlowManifest{
					ID:         "flow_cobranza",
					BusinessID: "biz-1",
					Intent:     cliFlowIntent{Goal: "cobranza"},
					Lifecycle: cliFlowLifecycle{
						Entry:  cliFlowLifecycleBinding{Framework: "radar", Capability: "collection.priority_list"},
						Tutela: cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
					},
					Nodes: []cliFlowNode{
						{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
						{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
					},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/workbench/compile":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowCompileResponse{
				Manifest: cliFlowManifest{
					ID:         "flow_cobranza",
					BusinessID: "biz-1",
					Intent:     cliFlowIntent{Goal: "cobranza"},
					Lifecycle: cliFlowLifecycle{
						Entry:  cliFlowLifecycleBinding{Framework: "radar", Capability: "collection.priority_list"},
						Tutela: cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
					},
					Nodes: []cliFlowNode{
						{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
						{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
					},
				},
				Compiled: cliFlowCompiledManifest{
					ID: "cmp_1234",
					Flow: cliFlowManifest{
						ID:         "flow_cobranza",
						BusinessID: "biz-1",
						Nodes: []cliFlowNode{
							{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list", Role: "bootstrap"},
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
						},
					},
				},
				Derivation: &cliFlowDerivation{
					Amendments: []cliFlowAmendment{{Kind: "role_changed", Summary: "Se promovió radar como bootstrap."}},
					Contracts:  []cliFlowDerivedContract{{NodeID: "draft", Command: "draft", Requires: []string{"entity.ref.v1"}, Produces: []string{"message.draft.v1"}, Policies: []string{"trace_required"}}},
					Handoffs:   []cliFlowDerivedHandoff{{FromNode: "prioritize", ToNode: "draft", Ownership: "pipeline", Artifacts: []string{"entity.ref.v1"}}},
					Install:    cliFlowInstallPreview{RequiresInstall: true, Capabilities: []string{"analysis.configure"}},
					Executable: cliFlowExecutablePlan{
						Lifecycle: cliFlowLifecycle{
							Entry:  cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
							Tutela: cliFlowLifecycleBinding{Framework: "foco", Capability: "focus.next_collection_task"},
						},
						Nodes: []cliFlowNode{
							{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list", Role: "bootstrap"},
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
						},
					},
				},
			})})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{"compile", "--id", "flw_1"}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Workbench", "Authored", "Derived", "Compiled", "cmp_1234", "Contratos", "Handoffs", "Instalacion", "Lifecycle autoral", "Lifecycle derivado", "entry: radar.collection.priority_list", "tutela: foco.focus.next_collection_task"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestRunFlowWorkbenchValidateShowsCompiledIDAndErrors(t *testing.T) {
	var validateBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/flows/flw_2":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowRecord{
				ID:         "flw_2",
				BusinessID: "biz-1",
				Name:       "Cobranza",
				Status:     "draft",
				Manifest:   &cliFlowManifest{ID: "flow_cobranza", BusinessID: "biz-1", Nodes: []cliFlowNode{{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"}}},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/workbench/compile":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowCompileResponse{
				Manifest: cliFlowManifest{ID: "flow_cobranza", BusinessID: "biz-1", Nodes: []cliFlowNode{{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"}}},
				Compiled: cliFlowCompiledManifest{ID: "cmp_validate", Flow: cliFlowManifest{ID: "flow_cobranza", BusinessID: "biz-1", Nodes: []cliFlowNode{{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"}}}},
				Derivation: &cliFlowDerivation{
					Executable: cliFlowExecutablePlan{Nodes: []cliFlowNode{{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"}}},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/validate":
			_ = json.NewDecoder(r.Body).Decode(&validateBody)
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliValidationResult{
				Valid:      false,
				CompiledID: "cmp_validate",
				Errors:     []cliValidationIssue{{Code: "node.requirement_missing", Message: "falta entity.ref.v1"}},
			})})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{"validate", "--id", "flw_2"}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Validacion", "cmp_validate", "node.requirement_missing", "falta entity.ref.v1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
	}
	if validateBody["compiled_id"] != "cmp_validate" {
		t.Fatalf("expected validate request to send compiled_id only, got %#v", validateBody)
	}
	if _, ok := validateBody["flow"]; ok {
		t.Fatalf("validate request should not include flow payload, got %#v", validateBody)
	}
}

func TestRunFlowWorkbenchReplayUsesRunEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/flows/runs/run_123":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliRunResult{
				RunID:      "run_123",
				Status:     "completed",
				CompiledID: "cmp_run",
				Valid:      true,
				Timeline:   []cliRunStep{{Node: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Status: "completed"}},
				Artifacts:  map[string]cliRunArtifact{"message.draft.v1": {Source: "framework_stdout"}},
			})})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{"replay", "--run", "run_123"}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Replay run run_123", "Run", "cmp_run", "message.draft.v1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestRunFlowDraftSendsExplicitIntentPayload(t *testing.T) {
	var suggestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/suggest":
			_ = json.NewDecoder(r.Body).Decode(&suggestBody)
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowSuggestResponse{
				Source: "heuristic",
				Proposal: &cliFlowSuggestionProposal{
					Manifest: cliFlowManifest{
						ID:         "flow_cobranza_intent",
						BusinessID: "biz-1",
						Intent: cliFlowIntent{
							Goal:        "Cobranza guiada",
							Description: "Analizar cartera y preparar correos",
							Roles:       []string{"analizar", "redactar"},
						},
						Nodes: []cliFlowNode{
							{ID: "analyze", Framework: "sabio", Capability: "data.entity_360"},
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
						},
					},
					Compiled: cliFlowCompiledManifest{ID: "cmp_role_first"},
				},
			})})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{"draft", "--business", "biz-1", "--name", "Cobranza guiada", "--description", "Analizar cartera y preparar correos"}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	intentRaw, ok := suggestBody["intent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected explicit intent payload, got %#v", suggestBody)
	}
	if intentRaw["goal"] != "Cobranza guiada" {
		t.Fatalf("intent goal = %#v", intentRaw["goal"])
	}
	roles, ok := intentRaw["roles"].([]interface{})
	if !ok || len(roles) == 0 {
		t.Fatalf("expected intent roles in suggest payload, got %#v", intentRaw["roles"])
	}
	var hasAnalyze, hasDraft bool
	for _, item := range roles {
		if item == "analizar" {
			hasAnalyze = true
		}
		if item == "redactar" {
			hasDraft = true
		}
	}
	if !hasAnalyze || !hasDraft {
		t.Fatalf("expected analyze+draft roles, got %#v", roles)
	}
}

func TestBuildFlowCreateIntentKeepsGoalIntentFirstWhenCapabilityHintExists(t *testing.T) {
	intent := buildFlowCreateIntent(flowCreateAnswers{
		Name:            "Cobranza guiada",
		Description:     "Analizar cartera y preparar correos para revisión humana",
		CapabilityHint:  "message.send",
		SuccessCriteria: "cada caso termina con correo listo",
		AutonomyMode:    "approval",
	})
	if intent.Goal != "Cobranza guiada" {
		t.Fatalf("goal should stay intent-first, got %#v", intent)
	}
	if strings.Contains(intent.Description, "Capacidad inicial:") {
		t.Fatalf("description should not be capability-first, got %#v", intent)
	}
	if len(intent.Roles) == 0 || intent.Roles[0] != "analizar" || !containsString(intent.Roles, "redactar") {
		t.Fatalf("expected role-first intent model, got %#v", intent.Roles)
	}
	if intent.CapabilityHint != "message.send" {
		t.Fatalf("capability hint should remain optional binding metadata, got %#v", intent)
	}
}

func TestRunFlowWorkbenchInspectPrefersPersistedCompiledRecord(t *testing.T) {
	compileCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/flows/flw_3":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowRecord{
				ID:          "flw_3",
				BusinessID:  "biz-1",
				Name:        "Cobranza compilada",
				Description: "Usar compiled persistido",
				Status:      "draft",
				CompiledID:  "cmp_cached",
				Manifest: &cliFlowManifest{
					ID:         "flow_cobranza_cached",
					BusinessID: "biz-1",
					Nodes: []cliFlowNode{
						{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
					},
				},
			})})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/flows/compiled/cmp_cached":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, map[string]interface{}{
				"authored": map[string]interface{}{
					"id":          "flow_cobranza_cached",
					"business_id": "biz-1",
					"nodes": []map[string]interface{}{
						{"id": "draft", "framework": "mecanico", "capability": "message.draft.collection_email"},
					},
				},
				"compiled": map[string]interface{}{
					"id": "cmp_cached",
					"flow": map[string]interface{}{
						"id":          "flow_cobranza_cached",
						"business_id": "biz-1",
						"nodes": []map[string]interface{}{
							{"id": "draft", "framework": "mecanico", "capability": "message.draft.collection_email", "role": "pipeline"},
						},
					},
				},
				"derivation": map[string]interface{}{
					"executable": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{"id": "draft", "framework": "mecanico", "capability": "message.draft.collection_email", "role": "pipeline"},
						},
					},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/workbench/compile":
			compileCalls++
			http.Error(w, `{"success":false,"error":"should not compile"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{"inspect", "--id", "flw_3"}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if compileCalls != 0 {
		t.Fatalf("expected inspect to reuse persisted compiled record, compileCalls=%d", compileCalls)
	}
	text := out.String()
	for _, want := range []string{"Workbench", "cmp_cached", "Compiled"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestRunFlowWorkbenchCreateUsesCapabilityFirstPipeline(t *testing.T) {
	var suggestBody map[string]interface{}
	var compileReq struct {
		Flow cliFlowManifest `json:"flow"`
	}
	var createBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/businesses/biz-1/artifacts":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliBusinessArtifactsResponse{
				BusinessID: "biz-1",
				Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/suggest":
			_ = json.NewDecoder(r.Body).Decode(&suggestBody)
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowSuggestResponse{
				Source: "heuristic",
				Proposal: &cliFlowSuggestionProposal{
					Manifest: cliFlowManifest{
						ID:         "flow_create_capability_first",
						BusinessID: "biz-1",
						Nodes: []cliFlowNode{
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
						},
					},
					Derivation: &cliFlowDerivation{
						Grounding: cliFlowDataGrounding{
							DesiredCapability: "message.send",
							BusinessArtifacts: []string{"data.sqlite_db.v1"},
						},
						Executable: cliFlowExecutablePlan{
							Nodes: []cliFlowNode{
								{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
							},
						},
					},
					Compiled: cliFlowCompiledManifest{
						ID:   "cmp_suggest",
						Flow: cliFlowManifest{ID: "flow_create_capability_first"},
					},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/workbench/compile":
			_ = json.NewDecoder(r.Body).Decode(&compileReq)
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowCompileResponse{
				Manifest: compileReq.Flow,
				Compiled: cliFlowCompiledManifest{
					ID: "cmp_create",
					Flow: cliFlowManifest{
						ID:         compileReq.Flow.ID,
						BusinessID: compileReq.Flow.BusinessID,
						Intent:     compileReq.Flow.Intent,
						Nodes: []cliFlowNode{
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
						},
					},
				},
				Derivation: &cliFlowDerivation{
					Grounding: cliFlowDataGrounding{
						DesiredCapability: "message.send",
						BusinessArtifacts: []string{"data.sqlite_db.v1"},
					},
					Executable: cliFlowExecutablePlan{
						Nodes: []cliFlowNode{
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
						},
					},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/businesses/biz-1/flows":
			_ = json.NewDecoder(r.Body).Decode(&createBody)
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowRecord{
				ID:          "flw_created",
				BusinessID:  "biz-1",
				Name:        "Cobranza capability-first",
				Description: "Preparar cobranza capability-first",
				Status:      "draft",
				CompiledID:  "cmp_create",
				Manifest:    &compileReq.Flow,
			})})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{
		"create",
		"--business", "biz-1",
		"--name", "Cobranza capability-first",
		"--description", "Preparar cobranza capability-first",
		"--capability", "message.send",
		"--success", "correo listo para cada caso",
		"--autonomy", "approval",
	}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}

	intentRaw, ok := suggestBody["intent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected suggest intent payload, got %#v", suggestBody)
	}
	if intentRaw["capability_hint"] != "message.send" {
		t.Fatalf("capability_hint = %#v", intentRaw["capability_hint"])
	}
	if intentRaw["goal"] != "Cobranza capability-first" {
		t.Fatalf("goal should stay intent-first, got %#v", intentRaw["goal"])
	}
	if description, _ := suggestBody["description"].(string); strings.Contains(description, "Capacidad inicial:") {
		t.Fatalf("suggest description should not be capability-first, got %#v", suggestBody)
	}
	if intentRaw["success_criteria"] != "correo listo para cada caso" {
		t.Fatalf("success_criteria = %#v", intentRaw["success_criteria"])
	}
	intentConstraints, ok := intentRaw["constraints"].([]interface{})
	if !ok || len(intentConstraints) == 0 {
		t.Fatalf("constraints = %#v", intentRaw["constraints"])
	}
	var hasApproval bool
	for _, item := range intentConstraints {
		if item == "approval_required" {
			hasApproval = true
		}
	}
	if !hasApproval {
		t.Fatalf("expected approval_required in constraints, got %#v", intentConstraints)
	}
	if compileReq.Flow.Intent.CapabilityHint != "message.send" {
		t.Fatalf("compile capability_hint = %q", compileReq.Flow.Intent.CapabilityHint)
	}
	if compileReq.Flow.Intent.Goal != "Cobranza capability-first" {
		t.Fatalf("compile goal should stay intent-first, got %#v", compileReq.Flow.Intent)
	}
	if compileReq.Flow.Intent.SuccessCriteria != "correo listo para cada caso" {
		t.Fatalf("compile success = %q", compileReq.Flow.Intent.SuccessCriteria)
	}
	if strings.Contains(compileReq.Flow.Intent.Description, "Capacidad inicial:") {
		t.Fatalf("compile description should not be capability-first, got %#v", compileReq.Flow.Intent)
	}
	manifestRaw, ok := createBody["manifest"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected manifest in create body, got %#v", createBody)
	}
	createIntent, ok := manifestRaw["intent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected manifest.intent in create body, got %#v", manifestRaw)
	}
	if createIntent["capability_hint"] != "message.send" {
		t.Fatalf("create capability_hint = %#v", createIntent["capability_hint"])
	}
	text := out.String()
	for _, want := range []string{
		"Flow create Cobranza capability-first",
		"artifacts: business.semantic_pack.v1, data.sqlite_db.v1",
		"Authored",
		"Derived",
		"Compiled",
		"cmp_create",
		"success: correo listo para cada caso",
		"constraints: approval_required",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestRunFlowWorkbenchCreateInteractiveNoCreate(t *testing.T) {
	previousInput := flowWorkbenchInput
	flowWorkbenchInput = strings.NewReader("biz-1\nCobranza interactiva\nPreparar cobranza interactiva\nmessage.send\ncorreo listo\nadvisory\n")
	t.Cleanup(func() { flowWorkbenchInput = previousInput })

	createCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/businesses/biz-1/artifacts":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliBusinessArtifactsResponse{
				BusinessID: "biz-1",
				Artifacts:  []string{"business.semantic_pack.v1"},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/suggest":
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowSuggestResponse{
				Source: "heuristic",
				Proposal: &cliFlowSuggestionProposal{
					Manifest: cliFlowManifest{
						ID:         "flow_interactive",
						BusinessID: "biz-1",
						Nodes: []cliFlowNode{
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
						},
					},
					Compiled: cliFlowCompiledManifest{
						ID: "cmp_interactive_suggest",
					},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/flows/workbench/compile":
			var req struct {
				Flow cliFlowManifest `json:"flow"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(flowAPIEnvelope{Success: true, Data: mustRawJSON(t, cliFlowCompileResponse{
				Manifest: req.Flow,
				Compiled: cliFlowCompiledManifest{
					ID:   "cmp_interactive",
					Flow: req.Flow,
				},
				Derivation: &cliFlowDerivation{
					Executable: cliFlowExecutablePlan{
						Nodes: []cliFlowNode{
							{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email", Role: "pipeline"},
						},
					},
				},
			})})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/businesses/biz-1/flows":
			createCalls++
			http.Error(w, `{"success":false,"error":"should not create"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runFlowWorkbench(&out, []string{"create", "--interactive", "--no-create"}, &flowWorkbenchClient{BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if createCalls != 0 {
		t.Fatalf("expected no create call, got %d", createCalls)
	}
	text := out.String()
	for _, want := range []string{
		"business_id:",
		"Qué quieres automatizar",
		"Artifacts detectados business.semantic_pack.v1",
		"Flow create Cobranza interactiva",
		"constraints: no_external_side_effect",
		"Compiled",
		"cmp_interactive",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output:\n%s", want, text)
		}
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

func mustRawJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

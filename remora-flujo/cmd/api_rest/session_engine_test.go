package main

import (
	"channel/adapter"
	"channel/manifest"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"remora-flujo/handoff"
	"strings"
	"testing"
)

func TestLoadActiveSessionScopedByConversation(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	payload := map[string]interface{}{
		"artifact_type":     segmentSessionArtifact,
		"business_id":       "biz-1",
		"status":            "active",
		"owner_framework":   "radar",
		"owner_capability":  "analysis.deep_dive",
		"followup_command":  "analyze-followup",
		"conversation_id":   "conv-a",
		"segment_id":        "seg-1",
		"allowed_delegates": []interface{}{"data.entity_360"},
	}
	s.persistFlowArtifact("run-a", "segment_session", segmentSessionArtifact, payload)

	got, err := s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil {
		t.Fatalf("load active session: %v", err)
	}
	if got == nil {
		t.Fatalf("expected session for owning conversation")
	}
	if got.ConversationID != "conv-a" || got.SegmentID != "seg-1" {
		t.Fatalf("unexpected scope: conversation=%q segment=%q", got.ConversationID, got.SegmentID)
	}

	other, err := s.loadActiveSessionFromDisk("biz-1", "conv-b")
	if err != nil {
		t.Fatalf("load active session for other conversation: %v", err)
	}
	if other != nil {
		t.Fatalf("expected no session for different conversation, got %+v", other)
	}
}

func TestUnclaimedSessionCanBeClaimedBySingleConversation(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	payload := map[string]interface{}{
		"artifact_type":    segmentSessionArtifact,
		"business_id":      "biz-1",
		"status":           "active",
		"owner_framework":  "radar",
		"owner_capability": "analysis.deep_dive",
		"followup_command": "analyze-followup",
		"segment_id":       "seg-1",
	}
	path := s.persistFlowArtifact("run-a", "segment_session", segmentSessionArtifact, payload)

	got, err := s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil {
		t.Fatalf("load unclaimed session: %v", err)
	}
	if got == nil || got.ConversationID != "" {
		t.Fatalf("expected unclaimed session, got %+v", got)
	}
	s.claimSessionForConversation(path, "conv-a")

	got, err = s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil || got == nil || got.ConversationID != "conv-a" {
		t.Fatalf("expected claimed session for conv-a, got session=%+v err=%v", got, err)
	}
	other, err := s.loadActiveSessionFromDisk("biz-1", "conv-b")
	if err != nil {
		t.Fatalf("load claimed session for other conversation: %v", err)
	}
	if other != nil {
		t.Fatalf("expected claimed session to be invisible to conv-b, got %+v", other)
	}
}

func TestLoadActiveSessionPrefersConversationClaimOverNewerUnclaimedSession(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	s.persistFlowArtifact("run-a", "segment_session", segmentSessionArtifact, map[string]interface{}{
		"artifact_type":    segmentSessionArtifact,
		"business_id":      "biz-1",
		"status":           "active",
		"owner_framework":  "radar",
		"owner_capability": "analysis.deep_dive",
		"followup_command": "analyze-followup",
		"conversation_id":  "conv-a",
		"segment_id":       "seg-a",
	})
	s.persistFlowArtifact("run-b", "segment_session", segmentSessionArtifact, map[string]interface{}{
		"artifact_type":    segmentSessionArtifact,
		"business_id":      "biz-1",
		"status":           "active",
		"owner_framework":  "radar",
		"owner_capability": "analysis.deep_dive",
		"followup_command": "analyze-followup",
		"segment_id":       "seg-b",
	})

	got, err := s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil {
		t.Fatalf("load active session: %v", err)
	}
	if got == nil || got.SegmentID != "seg-a" {
		t.Fatalf("expected conv-a to keep its claimed segment, got %+v", got)
	}
}

func TestExecuteDelegationsEnforcesAllowedDelegates(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	requests := []map[string]interface{}{
		{
			"framework":  "sabio",
			"capability": "evidence.portfolio_comparison",
			"params":     map[string]interface{}{"question": "portfolio"},
		},
	}
	results := s.executeDelegations(context.Background(), nil, &Conversation{Frameworks: []string{"sabio"}}, nil, requests, []string{"data.entity_360"})
	entries := delegationResultEntries(results)
	if len(entries) != 1 {
		t.Fatalf("expected one blocked delegation result, got %+v", results)
	}
	if verified, _ := entries[0]["verified"].(bool); verified {
		t.Fatalf("expected blocked delegation to be unverified, got %+v", entries[0])
	}
}

func TestNormalizeFollowupDelegationRequestsSimilarCustomers(t *testing.T) {
	payload := map[string]interface{}{
		"analysis_phase":   "plan",
		"analysis_intent":  "portfolio_comparison",
		"needs_delegation": true,
		"reason":           "comparar requiere datos de cartera",
		"delegation_requests": []interface{}{
			map[string]interface{}{
				"type":      "similar_customers",
				"deudor_id": "184",
				"criteria":  map[string]interface{}{"saldo": "cercano", "mora": "alta"},
			},
		},
	}

	got := normalizeFollowupDelegationRequests(payload, "Compáralo contra clientes similares de la cartera.")
	if len(got) != 1 {
		t.Fatalf("expected one normalized delegation, got %#v", got)
	}
	if got[0]["framework"] != "sabio" || got[0]["capability"] != "evidence.portfolio_comparison" {
		t.Fatalf("expected sabio/evidence.portfolio_comparison, got %#v", got[0])
	}
	params, _ := got[0]["params"].(map[string]interface{})
	if params["question"] == "" {
		t.Fatalf("expected non-empty question, got %#v", got[0])
	}
	if params["peer_strategy"] != "similar_clients" {
		t.Fatalf("expected peer_strategy similar_clients, got %#v", got[0])
	}
}

func TestNormalizeFollowupDelegationRequestsCaseDetails(t *testing.T) {
	payload := map[string]interface{}{
		"analysis_phase":   "plan",
		"analysis_intent":  "alternative_hypothesis",
		"needs_delegation": true,
		"delegation_requests": []interface{}{
			map[string]interface{}{
				"delegation_type": "obtener_detalles_de_caso",
				"deudor_id":       "184",
				"fields":          []interface{}{"open_amount", "payments_count"},
			},
		},
	}

	got := normalizeFollowupDelegationRequests(payload, "Profundiza el caso antes de asumir que la deuda es cobrable.")
	if len(got) != 1 {
		t.Fatalf("expected one normalized delegation, got %#v", got)
	}
	if got[0]["framework"] != "sabio" || got[0]["capability"] != "evidence.case_360" {
		t.Fatalf("expected sabio/evidence.case_360, got %#v", got[0])
	}
	params, _ := got[0]["params"].(map[string]interface{})
	question, _ := params["question"].(string)
	if question == "" || !strings.Contains(question, "open_amount") {
		t.Fatalf("expected entity_360 question to mention requested fields, got %#v", got[0])
	}
}

func TestNormalizeFollowupDelegationRequestsCompletesInsufficientPortfolioPlan(t *testing.T) {
	payload := map[string]interface{}{
		"analysis_phase":   "plan",
		"analysis_intent":  "portfolio_comparison",
		"needs_delegation": true,
		"reason":           "comparar requiere más contexto",
		"delegation_requests": []interface{}{
			map[string]interface{}{
				"framework":  "sabio",
				"capability": "evidence.case_360",
				"params": map[string]interface{}{
					"entity_ref":      "184",
					"entity_type":     "client",
					"analysis_intent": "case_baseline",
					"question":        "Construye baseline del caso",
				},
				"reason": "baseline",
			},
		},
	}

	got := normalizeFollowupDelegationRequests(payload, "Compáralo contra clientes similares de la cartera.")
	if len(got) < 2 {
		t.Fatalf("expected completed comparative plan, got %#v", got)
	}
	var caps []string
	for _, req := range got {
		caps = append(caps, jsonFirstString(req, "capability"))
	}
	if !containsString(caps, "evidence.case_360") {
		t.Fatalf("expected case baseline preserved, got %#v", got)
	}
	if !containsString(caps, "evidence.portfolio_comparison") {
		t.Fatalf("expected portfolio comparison added, got %#v", got)
	}
}

func TestNormalizeFollowupDelegationRequestsPreservesValidPortfolioCapabilityWithNonCanonicalFramework(t *testing.T) {
	payload := map[string]interface{}{
		"analysis_phase":   "plan",
		"analysis_intent":  "portfolio_comparison",
		"needs_delegation": true,
		"reason":           "comparar requiere cartera",
		"delegation_requests": []interface{}{
			map[string]interface{}{
				"framework":  "evidence",
				"capability": "evidence.portfolio_comparison",
				"params": map[string]interface{}{
					"entity_ref":      "184",
					"entity_type":     "client",
					"analysis_intent": "portfolio_comparison",
					"question":        "Compara el cliente contra la cartera",
				},
				"reason": "comparación de cartera",
			},
		},
	}

	got := normalizeFollowupDelegationRequests(payload, "Compáralo contra clientes similares de la cartera.")
	if len(got) == 0 {
		t.Fatalf("expected normalized delegation, got %#v", got)
	}
	first := got[0]
	if jsonFirstString(first, "capability") != "evidence.portfolio_comparison" {
		t.Fatalf("expected portfolio comparison capability preserved, got %#v", first)
	}
	if jsonFirstString(first, "framework") != "sabio" {
		t.Fatalf("expected framework canonicalized to sabio, got %#v", first)
	}
	if len(got) != 1 {
		t.Fatalf("expected no extra fallback when comparative capability already valid, got %#v", got)
	}
}

func TestNormalizeFollowupDelegationRequestsInfersSensitivityByIntent(t *testing.T) {
	payload := map[string]interface{}{
		"analysis_phase":   "plan",
		"analysis_intent":  "score_sensitivity",
		"needs_delegation": true,
		"reason":           "necesita ranking contrafactual",
	}

	got := normalizeFollowupDelegationRequests(payload, "Haz un análisis de sensibilidad del score.")
	if len(got) != 1 {
		t.Fatalf("expected one inferred delegation, got %#v", got)
	}
	if got[0]["framework"] != "sabio" || got[0]["capability"] != "evidence.score_sensitivity" {
		t.Fatalf("expected inferred sabio/evidence.score_sensitivity, got %#v", got[0])
	}
}

func TestExtractFollowupTextAndDelegationsHandlesNonExecutablePlan(t *testing.T) {
	stdout := `{"artifact_type":"analysis.followup.v1","analysis_phase":"plan","analysis_intent":"general_deep_analysis","needs_delegation":true,"reason":"faltan datos adicionales","text":"Radar interpreta la pregunta y solicita evidencia auxiliar () antes de sintetizar.","delegation_requests":[{"type":"desconocido"}]}`

	text, delegations := extractFollowupTextAndDelegations(stdout, "¿Qué evidencia falta?")
	if len(delegations) != 0 {
		t.Fatalf("expected no executable delegations, got %#v", delegations)
	}
	if text == "" || !strings.Contains(text, "no produjo delegaciones ejecutables") {
		t.Fatalf("expected fallback text warning about non-executable delegation, got %q", text)
	}
}

func TestExecuteDelegationsPassesStructuredDelegateParams(t *testing.T) {
	var joinedArgs string
	chServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		args, _ := params["args"].([]interface{})
		joinedArgs = fmt.Sprint(args)
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   `{"artifact_type":"entity_360.v1","text":"ok","verified":true}`,
		})
	}))
	defer chServer.Close()

	root := t.TempDir()
	s := &server{rootDir: root}
	dbPath := filepath.Join(root, "framework-indexa", "data", "biz-1.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite fixture"), 0644); err != nil {
		t.Fatalf("write db fixture: %v", err)
	}
	packPath := filepath.Join(root, "framework-sabio", "businesses", "biz-1", "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(packPath), 0755); err != nil {
		t.Fatalf("mkdir semantic pack dir: %v", err)
	}
	pack := `{"business_id":"biz-1","data_source":{"default_path":"../framework-indexa/data/biz-1.db"}}`
	if err := os.WriteFile(packPath, []byte(pack), 0644); err != nil {
		t.Fatalf("write semantic pack: %v", err)
	}
	manifests := map[string]*manifest.Manifest{
		"sabio": {
			Name:   "sabio",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"query": {
					Args:   []string{"-c", "echo", "--business-id", "{params.business_id}", "--db", "{params.db}", "--semantic-pack", "{params.semantic_pack}", "--question", "{params.question}", "--capability", "{params.capability}", "--entity-ref", "{params.entity_ref}", "--entity-type", "{params.entity_type}", "--analysis-intent", "{params.analysis_intent}", "--context-b64", "{params.context_b64}"},
					Params: []string{"question", "capability", "entity_ref", "entity_type", "analysis_intent", "context_b64", "business_id", "db", "semantic_pack"},
				},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "data.entity_360", Command: "query"},
			},
		},
	}
	requests := []map[string]interface{}{
		{
			"framework":       "sabio",
			"capability":      "evidence.case_360",
			"analysis_intent": "alternative_hypothesis",
			"params": map[string]interface{}{
				"question":        "Construye vista 360 del cliente activo",
				"entity_ref":      "184",
				"entity_type":     "customer",
				"analysis_intent": "alternative_hypothesis",
			},
		},
	}
	conv := &Conversation{
		BusinessID:     "biz-1",
		Frameworks:     []string{"sabio"},
		RuntimeContext: map[string]any{"active_entity": map[string]any{"id": "184", "type": "client"}},
	}
	results := s.executeDelegations(context.Background(), adapter.New(chServer.URL, "test-key"), conv, manifests, requests, []string{"data.entity_360"})
	if len(results) == 0 {
		t.Fatalf("expected delegation result, got %#v", results)
	}
	for _, want := range []string{
		"--business-id biz-1",
		"--db " + dbPath,
		"--semantic-pack " + packPath,
		"--capability data.entity_360",
		"--entity-ref 184",
		"--entity-type client",
		"--analysis-intent alternative_hypothesis",
		"--context-b64",
	} {
		if !strings.Contains(joinedArgs, want) {
			t.Fatalf("expected args to contain %q, got %s", want, joinedArgs)
		}
	}
}

func TestExecuteDelegationsPreservesMultipleResultsSameResolvedCapability(t *testing.T) {
	var callCount int
	chServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(adapter.Response{
			Success:  true,
			ExitCode: 0,
			Stdout:   fmt.Sprintf(`{"artifact_type":"entity_360.v1","text":"ok-%d","verified":true}`, callCount),
		})
	}))
	defer chServer.Close()
	s := &server{rootDir: t.TempDir()}
	manifests := map[string]*manifest.Manifest{
		"sabio": {
			Name:   "sabio",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"query": {Args: []string{"-c", "echo"}, Params: []string{"question", "capability", "entity_ref", "entity_type"}},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "data.entity_360", Command: "query"},
			},
		},
	}
	requests := []map[string]interface{}{
		{"request_id": "r1", "framework": "sabio", "capability": "evidence.case_360", "params": map[string]interface{}{"question": "uno", "entity_ref": "184", "entity_type": "client"}},
		{"request_id": "r2", "framework": "sabio", "capability": "evidence.case_360", "params": map[string]interface{}{"question": "dos", "entity_ref": "185", "entity_type": "client"}},
	}
	results := s.executeDelegations(context.Background(), adapter.New(chServer.URL, "test-key"), &Conversation{Frameworks: []string{"sabio"}}, manifests, requests, []string{"data.entity_360"})
	entries := delegationResultEntries(results)
	if len(entries) != 2 {
		t.Fatalf("expected two result entries, got %#v", results)
	}
	if entries[0]["request_id"] == entries[1]["request_id"] {
		t.Fatalf("expected distinct request_ids, got %#v", entries)
	}
}

func TestExecuteSessionFollowupDetailedMaterializesDelegationResultsPathAndSynthesizes(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "session.json")
	if err := os.WriteFile(sessionPath, []byte(`{"artifact_type":"segment.session.v1","business_id":"biz-1","status":"active","owner_framework":"radar","owner_capability":"analysis.deep_dive","followup_command":"analyze-followup","turn_count":0}`), 0644); err != nil {
		t.Fatalf("write session fixture: %v", err)
	}

	var sawPlan bool
	var sawSynthesis bool
	var capturedDelegationPath string
	chServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		params, _ := body["params"].(map[string]interface{})
		argsRaw, _ := params["args"].([]interface{})
		args := make([]string, 0, len(argsRaw))
		for _, raw := range argsRaw {
			args = append(args, fmt.Sprint(raw))
		}
		for _, arg := range args {
			if strings.Contains(arg, ";") || strings.Contains(arg, ">") {
				_ = json.NewEncoder(w).Encode(adapter.Response{Success: false, ExitCode: 1, Error: "path not safe: " + arg})
				return
			}
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "delegate-query"):
			_ = json.NewEncoder(w).Encode(adapter.Response{
				Success:  true,
				ExitCode: 0,
				Stdout:   `{"artifact_type":"query.result.v1","text":"percentil 95; materialidad > media","verified":true,"structured":{"open_amount_percentile":95,"materiality":"alta"}}`,
			})
		case strings.Contains(joined, "radar-followup"):
			delegationPath := testArgValue(args, "--delegation-results-path")
			delegationInline := testArgValue(args, "--delegation-results-json")
			if delegationPath == "" && delegationInline == "" {
				sawPlan = true
				_ = json.NewEncoder(w).Encode(adapter.Response{
					Success:  true,
					ExitCode: 0,
					Stdout:   `{"artifact_type":"analysis.followup.v1","analysis_phase":"plan","analysis_intent":"portfolio_comparison","needs_delegation":true,"reason":"comparar requiere percentiles","delegation_requests":[{"framework":"sabio","capability":"evidence.portfolio_comparison","params":{"question":"comparar cartera","entity_ref":"cust_1","entity_type":"customer"},"reason":"comparar percentiles"}],"text":"Radar necesita comparar el caso contra la cartera."}`,
				})
				return
			}
			sawSynthesis = true
			if delegationInline != "" {
				t.Fatalf("expected delegation results inline arg to be empty after materialization, got %q", delegationInline)
			}
			if delegationPath == "" {
				t.Fatalf("expected delegation results path for second pass, args=%#v", args)
			}
			capturedDelegationPath = delegationPath
			raw, err := os.ReadFile(delegationPath)
			if err != nil {
				t.Fatalf("read delegation results path: %v", err)
			}
			if !strings.Contains(string(raw), "percentil 95; materialidad") {
				t.Fatalf("expected materialized delegation payload to preserve evidence, got %s", string(raw))
			}
			_ = json.NewEncoder(w).Encode(adapter.Response{
				Success:  true,
				ExitCode: 0,
				Stdout:   `{"artifact_type":"analysis.followup.v1","analysis_phase":"synthesis","text":"Radar integra percentil 95 y materialidad alta desde la comparación de cartera.","findings":["comparación integrada"],"evidence":["delegation_results_path"],"confidence":"moderate"}`,
			})
			return
		default:
			t.Fatalf("unexpected args: %#v params=%#v", args, params)
		}
	}))
	defer chServer.Close()

	s := &server{rootDir: root}
	manifests := map[string]*manifest.Manifest{
		"radar": {
			Name:   "radar",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"analyze-followup": {
					Args: []string{
						"-c", "radar-followup",
						"--business-id", "{params.business_id}",
						"--input", "{params.input}",
						"--delegation-results-path", "{params.delegation_results_path}",
						"--delegation-results-json", "{params.delegation_results_json}",
						"--llm-followup-path", "{params.llm_followup_path}",
						"--llm-followup-json", "{params.llm_followup_json}",
					},
					Params: []string{"business_id", "input", "delegation_results_path", "delegation_results_json", "llm_followup_path", "llm_followup_json"},
					Defaults: map[string]string{
						"business_id":             "",
						"input":                   "",
						"delegation_results_path": "",
						"delegation_results_json": "",
						"llm_followup_path":       "",
						"llm_followup_json":       "",
					},
				},
			},
		},
		"sabio": {
			Name:   "sabio",
			Cwd:    ".",
			Binary: manifest.BinarySpec{Command: "/bin/sh"},
			Commands: map[string]manifest.Command{
				"query": {
					Args:   []string{"-c", "delegate-query", "--question", "{params.question}", "--capability", "{params.capability}", "--entity-ref", "{params.entity_ref}", "--entity-type", "{params.entity_type}"},
					Params: []string{"question", "capability", "entity_ref", "entity_type"},
				},
			},
			Capabilities: []manifest.CapabilitySpec{
				{ID: "data.query.sql", Command: "query"},
			},
		},
	}
	conv := &Conversation{
		ID:         "conv_followup_detailed",
		BusinessID: "biz-1",
		Frameworks: []string{"radar", "sabio"},
		RuntimeContext: map[string]any{
			"active_entity": map[string]any{"id": "cust_1", "type": "client"},
		},
	}
	queue := handoff.NewQuestionsQueue(conv.Frameworks...)
	session := &activeSessionInfo{
		Path:             sessionPath,
		Framework:        "radar",
		Capability:       "analysis.deep_dive",
		FollowupCmd:      "analyze-followup",
		AllowedDelegates: []string{"data.query.sql"},
		ContinueSignals:  []string{"profundiza"},
		ExitSignals:      []string{"siguiente caso"},
		MaxTurns:         10,
	}

	execution, err := s.executeSessionFollowupDetailed(context.Background(), adapter.New(chServer.URL, "test-key"), conv, manifests, queue, session, "Compáralo contra la cartera.", sessionFollowupModeRuntime)
	if err != nil {
		t.Fatalf("executeSessionFollowupDetailed: %v", err)
	}
	if !execution.OK {
		t.Fatalf("expected queued followup execution, got %+v", execution)
	}
	if !execution.Synthesized || execution.AnalysisPhase != "synthesis" {
		t.Fatalf("expected synthesized second pass, got %+v", execution)
	}
	if execution.SynthesisError != "" {
		t.Fatalf("unexpected synthesis error: %+v", execution)
	}
	if execution.DelegationResultsPath == "" || capturedDelegationPath == "" {
		t.Fatalf("expected materialized delegation path, got execution=%+v captured=%q", execution, capturedDelegationPath)
	}
	if execution.DelegationResultsPath != capturedDelegationPath {
		t.Fatalf("expected captured delegation path to match execution, got %q vs %q", capturedDelegationPath, execution.DelegationResultsPath)
	}
	if !sawPlan || !sawSynthesis {
		t.Fatalf("expected both plan and synthesis passes, plan=%v synthesis=%v", sawPlan, sawSynthesis)
	}
	if !strings.Contains(execution.Question.Text, "percentil 95") {
		t.Fatalf("expected final question to reflect synthesized evidence, got %q", execution.Question.Text)
	}

	followupPath := s.latestFlowArtifactPath("biz-1", "analysis.followup.v1")
	if followupPath == "" {
		t.Fatalf("expected persisted analysis.followup.v1")
	}
	raw, err := os.ReadFile(followupPath)
	if err != nil {
		t.Fatalf("read followup artifact: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse followup artifact: %v", err)
	}
	if payload["analysis_phase"] != "synthesis" {
		t.Fatalf("expected persisted phase=synthesis, got %+v", payload)
	}
	if synthesized, _ := payload["synthesized"].(bool); !synthesized {
		t.Fatalf("expected persisted synthesized=true, got %+v", payload)
	}
	if strings.TrimSpace(fmt.Sprint(payload["delegation_results_path"])) == "" {
		t.Fatalf("expected persisted delegation_results_path, got %+v", payload)
	}
}

func TestDeepAnalysisSimulatedConversationPreservesActiveEntity(t *testing.T) {
	req := flowRunRequest{Flow: flowManifest{BusinessID: "biz-1", Nodes: []flowNode{{Framework: "radar"}}}}
	result := &flowRunResult{
		RunID:      "run-1",
		BusinessID: "biz-1",
		Artifacts: map[string]flowRunArtifact{
			"entity.ref.v1": {
				Type:    "entity.ref.v1",
				Payload: map[string]interface{}{"id": "184", "type": "customer", "name": "Thiel-Effertz"},
			},
		},
	}
	conv := deepAnalysisSimulatedConversation(req, result, map[string]*manifest.Manifest{"radar": {Name: "radar"}})
	active, _ := conv.RuntimeContext["active_entity"].(map[string]any)
	if active["id"] != "184" || active["type"] != "client" {
		t.Fatalf("expected active_entity preserved, got %#v", conv.RuntimeContext)
	}
}

func testArgValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func TestClassifySegmentIntentSeparatesContinueOperationalExit(t *testing.T) {
	session := &activeSessionInfo{
		ContinueSignals:    []string{"profundiza", "detalle", "riesgo", "mora"},
		OperationalSignals: []string{"avanza", "con eso basta", "manda"},
		ExitSignals:        []string{"siguiente caso", "déjalo"},
	}
	cases := []struct {
		input string
		want  segmentIntentType
	}{
		{"ok, pero profundiza más el riesgo", segmentIntentContinue},
		{"dale más detalle de la mora", segmentIntentContinue},
		{"ok, con eso basta, avanza", segmentIntentOperational},
		{"déjalo ahí, siguiente caso", segmentIntentExit},
	}
	for _, tc := range cases {
		got := classifySegmentIntent(tc.input, session)
		if got != tc.want {
			t.Fatalf("classifySegmentIntent(%q)=%s, want %s", tc.input, got, tc.want)
		}
	}
}

func TestPersistAnalysisHandoffCreatesStructuredArtifact(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	s.persistFlowArtifact("review", "radar", "analysis.case_review.v1", map[string]interface{}{
		"artifact_type":  "analysis.case_review.v1",
		"business_id":    "biz-1",
		"text":           "El cliente tiene mora alta y riesgo moderado.",
		"recommendation": "Contactar con mensaje de cobranza empático.",
		"residual_risks": []string{"historial incompleto"},
		"data_gaps":      []string{"sin contacto alternativo"},
	})
	session := &activeSessionInfo{
		Framework:      "radar",
		Capability:     "analysis.deep_dive",
		ConversationID: "conv-a",
		TurnCount:      2,
	}
	conv := &Conversation{ID: "conv-a", BusinessID: "biz-1"}

	s.persistAnalysisHandoff(context.Background(), nil, conv, nil, session, "ok, con eso basta, avanza")

	path := filepath.Join(root, "temp", "flow_runs", "handoff_biz-1", "artifacts", "analysis_handoff__analysis.handoff.v1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read handoff artifact: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse handoff artifact: %v", err)
	}
	if payload["artifact_type"] != "analysis.handoff.v1" {
		t.Fatalf("unexpected artifact_type: %v", payload["artifact_type"])
	}
	if payload["conversation_id"] != "conv-a" {
		t.Fatalf("handoff not scoped to conversation: %v", payload["conversation_id"])
	}
	if payload["analytical_summary"] == "" || payload["recommendation"] == "" || payload["confidence"] == "" {
		t.Fatalf("handoff missing operational context: %+v", payload)
	}
}

func TestConcludeAnalysisTransitionPersistsReviewPendingWithoutExplicitReadiness(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	sessionPath := filepath.Join(root, "session.json")
	if err := os.WriteFile(sessionPath, []byte(`{"artifact_type":"segment.session.v1","business_id":"biz-1","status":"active","owner_framework":"radar","owner_capability":"analysis.deep_dive","followup_command":"analyze-followup","turn_count":2}`), 0644); err != nil {
		t.Fatalf("write session fixture: %v", err)
	}
	s.persistFlowArtifact("review", "radar", "analysis.case_review.v1", map[string]interface{}{
		"artifact_type":  "analysis.case_review.v1",
		"business_id":    "biz-1",
		"text":           "Radar cerró el análisis, pero todavía faltan definiciones para operar.",
		"recommendation": "Mantener stewardship en Foco hasta decisión humana.",
		"data_gaps":      []string{"falta aprobación humana"},
		"confidence":     "moderate",
	})
	session := &activeSessionInfo{
		Framework:      "radar",
		Capability:     "analysis.deep_dive",
		ConversationID: "conv-a",
		TurnCount:      2,
		Path:           sessionPath,
	}
	conv := &Conversation{ID: "conv-a", BusinessID: "biz-1"}

	result := s.concludeAnalysisTransition(context.Background(), nil, conv, nil, session, "ok, avanza", false)
	if result.State != "review_pending" || result.ReadyForOperation {
		t.Fatalf("expected review_pending without readiness, got %+v", result)
	}
	if result.ArtifactType != "analysis.review_pending.v1" || result.ArtifactPath == "" {
		t.Fatalf("expected review_pending artifact, got %+v", result)
	}
	if path := s.latestFlowArtifactPath("biz-1", "analysis.handoff.v1"); path != "" {
		t.Fatalf("did not expect handoff artifact, got %s", path)
	}
	raw, err := os.ReadFile(result.ArtifactPath)
	if err != nil {
		t.Fatalf("read review_pending artifact: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse review_pending artifact: %v", err)
	}
	if payload["state"] != "review_pending" || payload["ready_for_operation"] != false {
		t.Fatalf("unexpected review_pending payload: %+v", payload)
	}
	to, _ := payload["to"].(map[string]interface{})
	if to["framework"] != "foco" || to["role"] != "case_manager" {
		t.Fatalf("expected Foco case manager reentry, got %+v", payload)
	}
}

func TestConcludeAnalysisTransitionCreatesHandoffWithExplicitReadiness(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	sessionPath := filepath.Join(root, "session.json")
	if err := os.WriteFile(sessionPath, []byte(`{"artifact_type":"segment.session.v1","business_id":"biz-1","status":"active","owner_framework":"radar","owner_capability":"analysis.deep_dive","followup_command":"analyze-followup","turn_count":3}`), 0644); err != nil {
		t.Fatalf("write session fixture: %v", err)
	}
	session := &activeSessionInfo{
		Framework:      "radar",
		Capability:     "analysis.deep_dive",
		ConversationID: "conv-a",
		TurnCount:      3,
		Path:           sessionPath,
	}
	s.persistAnalysisReadinessArtifact("biz-1", "conv-a", session, `{"artifact_type":"analysis.followup.v1","text":"Radar considera suficiente la evidencia para operar.","recommendation":"Contactar al cliente hoy.","confidence":"high","ready_for_operation":true,"data_gaps":[]}`)
	conv := &Conversation{ID: "conv-a", BusinessID: "biz-1"}

	result := s.concludeAnalysisTransition(context.Background(), nil, conv, nil, session, "ok, avanza", false)
	if !result.ReadyForOperation || result.ArtifactType != "analysis.handoff.v1" || result.ArtifactPath == "" {
		t.Fatalf("expected operational handoff, got %+v", result)
	}
	if path := s.latestFlowArtifactPath("biz-1", "analysis.review_pending.v1"); path != "" {
		t.Fatalf("did not expect review_pending artifact, got %s", path)
	}
	raw, err := os.ReadFile(result.ArtifactPath)
	if err != nil {
		t.Fatalf("read handoff artifact: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse handoff artifact: %v", err)
	}
	if payload["artifact_type"] != "analysis.handoff.v1" || payload["conversation_id"] != "conv-a" {
		t.Fatalf("unexpected handoff payload: %+v", payload)
	}
}

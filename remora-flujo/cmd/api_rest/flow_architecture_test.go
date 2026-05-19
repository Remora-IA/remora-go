package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"channel/manifest"
)

func TestDeriveFlowManifestUsesExplicitExecutableDerivationPath(t *testing.T) {
	body := mustReadFunctionBody(t, filepath.Join("flow_derivation.go"), "func deriveFlowManifest(")
	if strings.Contains(body, "prepareFlowManifestLifecycle(") {
		t.Fatalf("deriveFlowManifest should not call prepareFlowManifestLifecycle directly:\n%s", body)
	}
	if !strings.Contains(body, "deriveExecutableFlow(") {
		t.Fatalf("deriveFlowManifest should use explicit executable derivation helper:\n%s", body)
	}
}

func TestValidateFlowManifestWithArtifactsDoesNotSilentlyNormalizeLifecycleRoles(t *testing.T) {
	body := mustReadFunctionBody(t, filepath.Join("flow_backend.go"), "func validateFlowManifestWithArtifacts(")
	if strings.Contains(body, "normalizeFlowLifecycleRoles(") {
		t.Fatalf("validateFlowManifestWithArtifacts should not normalize lifecycle roles silently:\n%s", body)
	}
}

func TestBuildFlowSuggestionProposalUsesExplicitIntentPlanBindingPath(t *testing.T) {
	body := mustReadFunctionBody(t, filepath.Join("flow_suggest.go"), "func buildFlowSuggestionProposal(")
	if !strings.Contains(body, "composeFlowSuggestIntentPlan(") {
		t.Fatalf("buildFlowSuggestionProposal should compose explicit intent plan before technical binding:\n%s", body)
	}
	if !strings.Contains(body, "bindIntentPlanToSuggestions(") {
		t.Fatalf("buildFlowSuggestionProposal should bind a precomposed intent plan to suggestions:\n%s", body)
	}
}

func TestBuildFlowSuggestionProposalUsesExplicitRoleBindingLayer(t *testing.T) {
	body := mustReadFunctionBody(t, filepath.Join("flow_suggest.go"), "func buildFlowSuggestionProposal(")
	if !strings.Contains(body, "buildFlowSuggestionBindings(") {
		t.Fatalf("buildFlowSuggestionProposal should expose an explicit role-to-technical binding layer before materializing nodes:\n%s", body)
	}
	if !strings.Contains(body, "buildFlowNodesFromBindings(") {
		t.Fatalf("buildFlowSuggestionProposal should materialize authored nodes from explicit bindings, not directly from suggestions:\n%s", body)
	}
}

func TestEnsureDefaultBusinessAssetsUsesTemplateStoreInsteadOfCreateFlow(t *testing.T) {
	body := mustReadFunctionBody(t, filepath.Join("business_defaults.go"), "func (s *server) ensureDefaultBusinessAssets(")
	if strings.Contains(body, ".createFlow(") {
		t.Fatalf("ensureDefaultBusinessAssets should not materialize authored flows directly anymore:\n%s", body)
	}
	if !strings.Contains(body, "createFlowTemplate(") {
		t.Fatalf("ensureDefaultBusinessAssets should register a template/proposal asset instead of a flow:\n%s", body)
	}
}

func TestValidateFlowUsesCompiledRecordDeterministically(t *testing.T) {
	s, token, businessID := newCompiledFlowHTTPTestServer(t)
	compilation := s.compileAndPersistFlowManifest(flowManifest{
		ID:         "flow_validate_compiled",
		BusinessID: businessID,
		Nodes:      []flowNode{{ID: "noop", Framework: "noop", Capability: "task.noop"}},
	}, s.allManifests, businessArtifactsResponse{BusinessID: businessID})

	body, err := json.Marshal(map[string]interface{}{
		"compiled_id": compilation.Compiled.ID,
		"flow": flowManifest{
			ID:         "broken_flow",
			BusinessID: businessID,
			Nodes:      []flowNode{{ID: "broken", Framework: "missing", Capability: "task.bad"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, apiBase+"/flows/validate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.validateFlow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool                 `json:"success"`
		Data    flowValidationResult `json:"data"`
		Error   string               `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if !resp.Data.Valid {
		t.Fatalf("expected compiled validation to ignore conflicting authored flow, errors=%#v", resp.Data.Errors)
	}
	if resp.Data.CompiledID != compilation.Compiled.ID {
		t.Fatalf("compiled_id=%q want %q", resp.Data.CompiledID, compilation.Compiled.ID)
	}
}

func TestSimulateFlowUsesCompiledRecordDeterministically(t *testing.T) {
	s, token, businessID := newCompiledFlowHTTPTestServer(t)
	compilation := s.compileAndPersistFlowManifest(flowManifest{
		ID:         "flow_simulate_compiled",
		BusinessID: businessID,
		Nodes:      []flowNode{{ID: "noop", Framework: "noop", Capability: "task.noop"}},
	}, s.allManifests, businessArtifactsResponse{BusinessID: businessID})

	body, err := json.Marshal(map[string]interface{}{
		"compiled_id": compilation.Compiled.ID,
		"flow": flowManifest{
			ID:         "broken_flow",
			BusinessID: businessID,
			Nodes:      []flowNode{{ID: "broken", Framework: "missing", Capability: "task.bad"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, apiBase+"/flows/simulate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.simulateFlow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool                 `json:"success"`
		Data    flowSimulationResult `json:"data"`
		Error   string               `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Data.CompiledID != compilation.Compiled.ID {
		t.Fatalf("compiled_id=%q want %q", resp.Data.CompiledID, compilation.Compiled.ID)
	}
	if len(resp.Data.Timeline) != 1 || resp.Data.Timeline[0].Node != "noop" {
		t.Fatalf("simulation should use compiled flow timeline, got %#v", resp.Data.Timeline)
	}
	if !resp.Data.Valid {
		t.Fatalf("expected compiled simulation valid, got %#v", resp.Data.Validation.Errors)
	}
}

func newCompiledFlowHTTPTestServer(t *testing.T) (*server, string, string) {
	t.Helper()
	s := newBusinessDefaultsServer(t)
	s.allManifests = map[string]*manifest.Manifest{
		"noop": {
			Name: "noop",
			Commands: map[string]manifest.Command{
				"noop": {Args: []string{"noop"}},
			},
			Capabilities: []manifest.CapabilitySpec{{
				ID:       "task.noop",
				Command:  "noop",
				Produces: []string{"noop.out.v1"},
			}},
		},
	}
	user, err := s.auth.createUser("owner@example.com", "password123", "Owner", "user")
	if err != nil {
		t.Fatal(err)
	}
	business, _, err := s.auth.createBusiness(user.ID, "Compiled Demo", "", "")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.auth.createSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, token, business.ID
}

func mustReadFunctionBody(t *testing.T, fileName, signature string) string {
	t.Helper()
	path := filepath.Join("/Users/alcless_a1234_cursor/remora-go-lite/remora-flujo/cmd/api_rest", fileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("signature %q not found in %s", signature, path)
	}
	open := strings.Index(src[start:], "{")
	if open < 0 {
		t.Fatalf("opening brace for %q not found in %s", signature, path)
	}
	open += start
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : i+1]
			}
		}
	}
	t.Fatalf("function body for %q not closed in %s", signature, path)
	return ""
}

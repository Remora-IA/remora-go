package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompileFlowManifestKeepsAuthoredManifestPure(t *testing.T) {
	flow := flowManifest{
		ID:         "flow_cobranza_authorial",
		BusinessID: "biz-1",
		Intent:     flowIntent{Goal: "enviar correos a la gente que me debe"},
		Nodes: []flowNode{
			{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
			{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
	}

	compilation := compileFlowManifest(flow, flowTestManifests(), businessArtifactsResponse{
		BusinessID: "biz-1",
		Artifacts:  []string{"business.semantic_pack.v1", "data.sqlite_db.v1"},
	})
	if compilation.Derivation == nil {
		t.Fatal("expected derivation")
	}
	if compilation.Compiled.ID == "" {
		t.Fatal("expected compiled id")
	}
	if len(compilation.Authored.Nodes) != 2 {
		t.Fatalf("authored nodes = %#v", compilation.Authored.Nodes)
	}
	for _, node := range compilation.Authored.Nodes {
		if node.ID == "node_foco_entry" || node.Role != "" {
			t.Fatalf("authored manifest should stay pure, got %#v", compilation.Authored.Nodes)
		}
	}
	if compilation.Authored.Nodes[0].ID != "prioritize" || compilation.Authored.Nodes[1].ID != "draft" {
		t.Fatalf("authored order = %#v", compilation.Authored.Nodes)
	}
	var sawDerivedRole bool
	for _, node := range compilation.Compiled.Flow.Nodes {
		if node.Role != "" {
			sawDerivedRole = true
		}
	}
	if !sawDerivedRole {
		t.Fatalf("compiled flow should contain derived executable structure, got %#v", compilation.Compiled.Flow.Nodes)
	}
	if len(compilation.Compiled.Flow.Nodes) != len(compilation.Authored.Nodes) {
		t.Fatalf("unexpected compiled/authored node count mismatch: authored=%#v compiled=%#v", compilation.Authored.Nodes, compilation.Compiled.Flow.Nodes)
	}
}

func TestCompileFlowWorkbenchReturnsAuthoredAndCompiledSeparately(t *testing.T) {
	s := newBusinessDefaultsServer(t)
	s.allManifests = flowTestManifests()

	user, err := s.auth.createUser("owner@example.com", "password123", "Owner", "user")
	if err != nil {
		t.Fatal(err)
	}
	business, _, err := s.auth.createBusiness(user.ID, "Panalbit", "", "")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.auth.createSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(flowWorkbenchCompileRequest{
		Flow: flowManifest{
			ID:         "flow_cobranza_http",
			BusinessID: business.ID,
			Intent:     flowIntent{Goal: "preparar correos de cobranza"},
			Nodes: []flowNode{
				{ID: "prioritize", Framework: "radar", Capability: "collection.priority_list"},
				{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, apiBase+"/flows/workbench/compile", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.compileFlowWorkbench(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    flowSuggestionProposal `json:"data"`
		Error   string                 `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("response error: %s", resp.Error)
	}
	if len(resp.Data.Manifest.Nodes) != 2 {
		t.Fatalf("authored manifest = %#v", resp.Data.Manifest)
	}
	for _, node := range resp.Data.Manifest.Nodes {
		if node.ID == "node_foco_entry" || node.Role != "" {
			t.Fatalf("endpoint should return authored manifest without derived staging, got %#v", resp.Data.Manifest.Nodes)
		}
	}
	var sawCompiledRole bool
	for _, node := range resp.Data.Compiled.Flow.Nodes {
		if node.Role != "" {
			sawCompiledRole = true
			break
		}
	}
	if !sawCompiledRole {
		t.Fatalf("expected compiled flow to include derived executable roles, got %#v", resp.Data.Compiled.Flow.Nodes)
	}
	if resp.Data.Compiled.ID == "" || resp.Data.Derivation == nil {
		t.Fatalf("expected compiled id and derivation, got %#v", resp.Data)
	}
}

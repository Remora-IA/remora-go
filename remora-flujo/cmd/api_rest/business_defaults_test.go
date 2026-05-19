package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
)

func newBusinessDefaultsServer(t *testing.T) *server {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(root, "auth.db")
	t.Setenv("REMORA_AUTH_DB", dbPath)
	t.Setenv("REMORA_BOOTSTRAP_EMAIL", "")
	t.Setenv("REMORA_BOOTSTRAP_PASSWORD", "")
	auth, err := openAuthStore()
	if err != nil {
		t.Fatal(err)
	}
	flows, err := openFlowStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = flows.close()
		_ = auth.db.Close()
	})
	return &server{rootDir: root, auth: auth, flows: flows}
}

func TestEnsureDefaultBusinessAssetsRegistersTemplateOnceWithoutMaterializingFlow(t *testing.T) {
	s := newBusinessDefaultsServer(t)
	user, err := s.auth.createUser("owner@example.com", "password123", "Owner", "user")
	if err != nil {
		t.Fatal(err)
	}
	business, _, err := s.auth.createBusiness(user.ID, "Panalbit", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ensureDefaultBusinessAssets(*business); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureDefaultBusinessAssets(*business); err != nil {
		t.Fatal(err)
	}
	flows, err := s.flows.listFlowsByBusiness(business.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(flows) != 0 {
		t.Fatalf("flows = %d, want 0 because defaults should stay as templates until explicit instantiation", len(flows))
	}
	var templateID, name, description, manifestRaw, status string
	if err := s.flows.db.QueryRow(
		`SELECT id, name, description, manifest_json, status FROM flow_templates WHERE business_id = ?`,
		business.ID,
	).Scan(&templateID, &name, &description, &manifestRaw, &status); err != nil {
		t.Fatal(err)
	}
	if templateID == "" {
		t.Fatal("expected stored default template id")
	}
	if name != defaultBusinessFlowName {
		t.Fatalf("name = %q", name)
	}
	if description != defaultBusinessFlowDescription {
		t.Fatalf("description = %q", description)
	}
	if status != "available" {
		t.Fatalf("status = %q", status)
	}
	var manifest flowManifest
	if err := json.Unmarshal([]byte(manifestRaw), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ID != defaultBusinessFlowManifestID() {
		t.Fatalf("manifest id = %q", manifest.ID)
	}
	if !manifest.Provenance.Template || manifest.Provenance.Source != "system_default_proposal" || manifest.Provenance.TemplateID != "default_business_collection" {
		t.Fatalf("expected template proposal provenance, got %#v", manifest.Provenance)
	}
	var count int
	if err := s.flows.db.QueryRow(`SELECT COUNT(*) FROM flow_templates WHERE business_id = ?`, business.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("template count = %d, want 1", count)
	}
}

func TestHandleListFlowTemplatesReturnsDefaultTemplateProposal(t *testing.T) {
	s := newBusinessDefaultsServer(t)
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
	if err := s.ensureDefaultBusinessAssets(*business); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, apiBase+"/businesses/"+business.ID+"/flow-templates", nil)
	req = mux.SetURLVars(req, map[string]string{"business_id": business.ID})
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.handleListFlowTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool                       `json:"success"`
		Data    []flowTemplateWithManifest `json:"data"`
		Error   string                     `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("templates=%d want 1", len(resp.Data))
	}
	if resp.Data[0].Manifest == nil {
		t.Fatal("expected template manifest")
	}
	if !resp.Data[0].Manifest.Provenance.Template || resp.Data[0].Manifest.Provenance.Source != "system_default_proposal" {
		t.Fatalf("expected visible template proposal provenance, got %#v", resp.Data[0].Manifest.Provenance)
	}
}

func TestHandleCreateFlowExplicitlyMaterializesTemplateIntoFlow(t *testing.T) {
	s := newBusinessDefaultsServer(t)
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
	body, err := json.Marshal(map[string]interface{}{
		"name":        "Cobranza instanciada",
		"description": "instanciación explícita desde template default",
		"manifest":    defaultBusinessFlowManifest(),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, apiBase+"/businesses/"+business.ID+"/flows", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"business_id": business.ID})
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.handleCreateFlow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool             `json:"success"`
		Data    flowWithManifest `json:"data"`
		Error   string           `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Data.Manifest == nil {
		t.Fatal("expected created manifest")
	}
	if resp.Data.Manifest.Provenance.Template {
		t.Fatalf("created flow should be an instantiated flow, got template provenance %#v", resp.Data.Manifest.Provenance)
	}
	if resp.Data.Manifest.Provenance.Source != "template_instantiation" {
		t.Fatalf("source = %q want template_instantiation", resp.Data.Manifest.Provenance.Source)
	}
	if resp.Data.Manifest.Provenance.TemplateID != "default_business_collection" {
		t.Fatalf("template_id = %q", resp.Data.Manifest.Provenance.TemplateID)
	}
	if resp.Data.Manifest.ID == defaultBusinessFlowManifestID() {
		t.Fatalf("instantiated flow must not keep template manifest id %q", resp.Data.Manifest.ID)
	}
}

func TestHandleCreateFlowKeepsAuthoredFlowsCompatible(t *testing.T) {
	s := newBusinessDefaultsServer(t)
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
	body, err := json.Marshal(map[string]interface{}{
		"name":        "Flow authored",
		"description": "creación manual compatible",
		"manifest": &flowManifest{
			Nodes: []flowNode{
				{ID: "draft", Framework: "mecanico", Capability: "message.draft.collection_email"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, apiBase+"/businesses/"+business.ID+"/flows", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"business_id": business.ID})
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.handleCreateFlow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool             `json:"success"`
		Data    flowWithManifest `json:"data"`
		Error   string           `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Data.Manifest == nil {
		t.Fatal("expected created manifest")
	}
	if resp.Data.Manifest.Provenance.Template {
		t.Fatalf("authored flow should not be classified as template: %#v", resp.Data.Manifest.Provenance)
	}
}

func TestBusinessArtifactsFallbackToBusinessNameSemanticPack(t *testing.T) {
	s := newBusinessDefaultsServer(t)
	user, err := s.auth.createUser("owner@example.com", "password123", "Owner", "user")
	if err != nil {
		t.Fatal(err)
	}
	business, _, err := s.auth.createBusiness(user.ID, "Panalbit", "", "")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(s.rootDir, "framework-indexa", "data", "panalbit.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	packPath := filepath.Join(s.rootDir, "framework-sabio", "businesses", "panalbit", "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(packPath), 0755); err != nil {
		t.Fatal(err)
	}
	pack := `{"business_id":"panalbit","data_source":{"default_path":"../framework-indexa/data/panalbit.db"}}`
	if err := os.WriteFile(packPath, []byte(pack), 0644); err != nil {
		t.Fatal(err)
	}
	resp := s.businessArtifacts(business.ID)
	if !containsString(resp.Artifacts, "business.semantic_pack.v1") {
		t.Fatalf("expected semantic pack, got %#v", resp.Artifacts)
	}
	if !containsString(resp.Artifacts, "data.sqlite_db.v1") {
		t.Fatalf("expected sqlite artifact, got %#v", resp.Artifacts)
	}
	if resp.Sources["business.semantic_pack.v1"] != packPath {
		t.Fatalf("semantic pack source = %q want %q", resp.Sources["business.semantic_pack.v1"], packPath)
	}
}

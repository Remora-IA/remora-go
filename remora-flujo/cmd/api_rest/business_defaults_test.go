package main

import (
	"os"
	"path/filepath"
	"testing"
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

func TestEnsureDefaultBusinessAssetsCreatesFlowOnce(t *testing.T) {
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
	if len(flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(flows))
	}
	if flows[0].Name != defaultBusinessFlowName {
		t.Fatalf("name = %q", flows[0].Name)
	}
	if flows[0].Manifest == nil || flows[0].Manifest.ID != defaultBusinessFlowManifestID() {
		t.Fatalf("manifest = %#v", flows[0].Manifest)
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

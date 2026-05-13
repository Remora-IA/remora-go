package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBusinessArtifactsDetectsSQLiteAndSemanticPack(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	businessID := "retail_demo"

	dbPath := businessDataDBPath(root, businessID)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE customers (id TEXT PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	packPath := filepath.Join(root, "framework-sabio", "businesses", businessID, "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(packPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte(`{"business_id":"retail_demo"}`), 0644); err != nil {
		t.Fatal(err)
	}

	resp := s.businessArtifacts(businessID)
	for _, want := range []string{"business.context.v1", "data.sqlite_db.v1", "business.semantic_pack.v1"} {
		if !containsString(resp.Artifacts, want) {
			t.Fatalf("expected artifact %s, got %#v", want, resp.Artifacts)
		}
		if resp.Sources[want] == "" {
			t.Fatalf("expected source for %s, got %#v", want, resp.Sources)
		}
	}
}

func TestBusinessArtifactsKeepsDataAndSemanticPackBusinessSpecific(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}

	packPath := filepath.Join(root, "framework-sabio", "businesses", "retail_demo", "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(packPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packPath, []byte(`{"business_id":"retail_demo"}`), 0644); err != nil {
		t.Fatal(err)
	}

	retail := s.businessArtifacts("retail_demo")
	logistics := s.businessArtifacts("logistics_demo")
	if !containsString(retail.Artifacts, "business.semantic_pack.v1") {
		t.Fatalf("expected retail semantic pack: %#v", retail.Artifacts)
	}
	if containsString(logistics.Artifacts, "business.semantic_pack.v1") {
		t.Fatalf("did not expect logistics semantic pack from retail config: %#v", logistics.Artifacts)
	}
}

func TestBusinessSQLitePathUsesSemanticPackDataSourceForAnyBusiness(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	businessID := "cliente_x"

	dbPath := filepath.Join(root, "framework-indexa", "data", businessID+".db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite fixture"), 0644); err != nil {
		t.Fatal(err)
	}

	packPath := filepath.Join(root, "framework-sabio", "businesses", businessID, "sabio.business.json")
	if err := os.MkdirAll(filepath.Dir(packPath), 0755); err != nil {
		t.Fatal(err)
	}
	pack := `{"business_id":"cliente_x","data_source":{"default_path":"../framework-indexa/data/cliente_x.db"}}`
	if err := os.WriteFile(packPath, []byte(pack), 0644); err != nil {
		t.Fatal(err)
	}

	resp := s.businessArtifacts(businessID)
	if !containsString(resp.Artifacts, "data.sqlite_db.v1") {
		t.Fatalf("expected generic data source artifact, got %#v", resp.Artifacts)
	}
	if resp.Sources["data.sqlite_db.v1"] != dbPath {
		t.Fatalf("source = %q want %q", resp.Sources["data.sqlite_db.v1"], dbPath)
	}
}

func TestRuntimeBusinessDBPathFallsBackToLegacyLocationWhenNoResolvedSQLiteExists(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	businessID := "cliente_x"

	want := businessDataDBPath(root, businessID)
	if got := s.runtimeBusinessDBPath(businessID); got != want {
		t.Fatalf("runtimeBusinessDBPath(%q) = %q, want %q", businessID, got, want)
	}
}

func TestBusinessSQLitePathDoesNotUseClientSpecificLegacyFallback(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}

	legacyPath := filepath.Join(root, "framework-indexa", "data", "panalbit.db")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("sqlite fixture"), 0644); err != nil {
		t.Fatal(err)
	}

	resp := s.businessArtifacts("panalbit")
	if containsString(resp.Artifacts, "data.sqlite_db.v1") {
		t.Fatalf("did not expect data artifact without standard business config: %#v", resp.Artifacts)
	}
}

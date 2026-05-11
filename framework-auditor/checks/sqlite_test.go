package checks

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func createTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a clients table WITHOUT email — mirrors panalbit schema.
	_, err = db.Exec(`CREATE TABLE clients (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		code TEXT,
		active TEXT DEFAULT '1'
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO clients (id, name, code, active) VALUES
		('1', 'Acme Corp', 'ACME', '1'),
		('2', 'Globex Inc', 'GLBX', '1'),
		('3', 'Initech', 'INIT', '0')`)
	if err != nil {
		t.Fatal(err)
	}

	// Create a users table WITH email — should NOT trigger schema gap.
	_, err = db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users (id, name, email) VALUES
		('u1', 'Admin', 'admin@test.com'),
		('u2', 'Staff', '')`)
	if err != nil {
		t.Fatal(err)
	}

	return dbPath
}

func TestLoadDatasetFromSQLite(t *testing.T) {
	dbPath := createTestDB(t)

	d, err := LoadDatasetFromSQLite(dbPath)
	if err != nil {
		t.Fatalf("LoadDatasetFromSQLite: %v", err)
	}

	if len(d.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(d.Endpoints))
	}
	clients := d.Endpoints["clients"]
	if len(clients) != 3 {
		t.Fatalf("expected 3 clients, got %d", len(clients))
	}
	users := d.Endpoints["users"]
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	// Verify field values are accessible.
	found := false
	for _, c := range clients {
		if c["name"] == "Acme Corp" {
			found = true
			if c["code"] != "ACME" {
				t.Errorf("expected code ACME, got %v", c["code"])
			}
		}
	}
	if !found {
		t.Error("Acme Corp not found in clients")
	}
}

func TestCheckSchemaContactCapability_MissingEmail(t *testing.T) {
	dbPath := createTestDB(t)

	d, err := LoadDatasetFromSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tableColumns, err := TableColumnsFromDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	findings := CheckSchemaContactCapability(d, tableColumns)

	// clients is a contact-entity endpoint without email column → should have a finding.
	foundClients := false
	for _, f := range findings {
		if f.Endpoint == "clients" && f.Rule == RuleSchemaContactGap {
			foundClients = true
			if f.Severity != SeverityCritical {
				t.Errorf("expected critical severity, got %s", f.Severity)
			}
		}
	}
	if !foundClients {
		t.Error("expected schema_contact_gap finding for clients table")
	}

	// users is NOT a contact-entity endpoint → should NOT appear.
	for _, f := range findings {
		if f.Endpoint == "users" {
			t.Errorf("users should not be flagged (not a contact-entity endpoint)")
		}
	}
}

func TestCheckSchemaContactCapability_WithEmail(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_with_email.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE clients (
		id TEXT PRIMARY KEY,
		name TEXT,
		email TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO clients (id, name, email) VALUES ('1', 'Test', 'test@x.com')`)
	if err != nil {
		t.Fatal(err)
	}

	d, err := LoadDatasetFromSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tableColumns, err := TableColumnsFromDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	findings := CheckSchemaContactCapability(d, tableColumns)
	for _, f := range findings {
		if f.Endpoint == "clients" {
			t.Error("clients with email column should NOT be flagged")
		}
	}
}

func TestRunAllWithSchema_IncludesSchemaCheck(t *testing.T) {
	dbPath := createTestDB(t)

	d, err := LoadDatasetFromSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	tableColumns, err := TableColumnsFromDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	findings := RunAllWithSchema(d, tableColumns)

	hasSchemaGap := false
	hasMissingContact := false
	for _, f := range findings {
		if f.Rule == RuleSchemaContactGap {
			hasSchemaGap = true
		}
		if f.Rule == RuleMissingContact {
			hasMissingContact = true
		}
	}
	if !hasSchemaGap {
		t.Error("RunAllWithSchema should include schema_contact_gap findings")
	}
	if !hasMissingContact {
		t.Error("RunAllWithSchema should include missing_contact_destination findings")
	}
}

func TestRunAll_BackwardCompatible(t *testing.T) {
	// RunAll (without schema) should still work and NOT include schema checks.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "compat.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO clients (id, name) VALUES ('1', 'Test')`)
	if err != nil {
		t.Fatal(err)
	}

	d, err := LoadDatasetFromSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	findings := RunAll(d) // no schema columns passed

	for _, f := range findings {
		if f.Rule == RuleSchemaContactGap {
			t.Error("RunAll (without schema) should NOT include schema_contact_gap findings")
		}
	}
}

func TestLoadDatasetAcceptsSabioTablesArtifact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dataset.raw.json")
	if err := os.WriteFile(path, []byte(`{
		"artifact_type": "dataset.raw.v1",
		"tables": {
			"clients": [
				{"id": "1", "name": "Cliente Uno"}
			]
		}
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDataset(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Endpoints["clients"]) != 1 {
		t.Fatalf("expected clients table from Sabio artifact, got %#v", d.Endpoints)
	}
	findings := RunAll(d)
	hasMissingContact := false
	for _, f := range findings {
		if f.Rule == RuleMissingContact {
			hasMissingContact = true
			break
		}
	}
	if !hasMissingContact {
		t.Fatalf("expected missing contact finding from Sabio tables artifact, got %#v", findings)
	}
}

func TestLoadDatasetFromSQLite_FileNotFound(t *testing.T) {
	_, err := LoadDatasetFromSQLite("/nonexistent/path.db")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTableColumnsFromDB(t *testing.T) {
	dbPath := createTestDB(t)

	cols, err := TableColumnsFromDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	clientCols := cols["clients"]
	if len(clientCols) != 4 {
		t.Fatalf("expected 4 columns for clients, got %d: %v", len(clientCols), clientCols)
	}
	expected := map[string]bool{"id": true, "name": true, "code": true, "active": true}
	for _, c := range clientCols {
		if !expected[c] {
			t.Errorf("unexpected column: %s", c)
		}
	}

	userCols := cols["users"]
	if len(userCols) != 3 {
		t.Fatalf("expected 3 columns for users, got %d: %v", len(userCols), userCols)
	}

	// Clean up temp db
	_ = os.Remove(dbPath)
}

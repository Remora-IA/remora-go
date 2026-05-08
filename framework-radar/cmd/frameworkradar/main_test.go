package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestScoreSQLiteUsesSemanticMappingsWithoutBusinessSpecificFallback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "acme.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE customers (id TEXT PRIMARY KEY, name TEXT)`,
		`CREATE TABLE invoices (id TEXT PRIMARY KEY, customer_id TEXT, status TEXT, due_date TEXT, amount REAL)`,
		`INSERT INTO customers (id, name) VALUES ('c1', 'Cliente Uno'), ('c2', 'Cliente Dos')`,
		`INSERT INTO invoices (id, customer_id, status, due_date, amount) VALUES ('i1', 'c1', 'open', '2026-01-01', 1000)`,
		`INSERT INTO invoices (id, customer_id, status, due_date, amount) VALUES ('i2', 'c2', 'open', '2025-01-01', 9000)`,
		`INSERT INTO invoices (id, customer_id, status, due_date, amount) VALUES ('i3', 'c2', 'paid', '2024-01-01', 50000)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	items, err := scoreSQLite(dbPath, collectionScoring{
		EntityTable:      "customers",
		EntityIDColumn:   "id",
		EntityNameColumn: "name",
		ItemTable:        "invoices",
		ItemEntityColumn: "customer_id",
		AmountColumn:     "amount",
		StatusColumn:     "status",
		DateColumn:       "due_date",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items len=%d items=%#v", len(items), items)
	}
	if got := items[0].EntityRef.ID; got != "c2" {
		t.Fatalf("selected=%s want c2; items=%#v", got, items)
	}
}

func TestInferScoringModelNeedsSemanticConfiguration(t *testing.T) {
	_, err := inferScoringModel(semanticPack{})
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestLoadSemanticPackAcceptsGenericBusiness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sabio.business.json")
	raw := `{
		"business_id": "acme",
		"primary_entities": {
			"customer": {"table": "customers", "scope_key": "id", "display_column": "name"},
			"invoice": {"table": "invoices", "scope_column": "customer_id"}
		},
		"scope_policies": {"scope_entity": "customer"}
	}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	pack, err := loadSemanticPack(path)
	if err != nil {
		t.Fatal(err)
	}
	model, err := inferScoringModel(pack)
	if err != nil {
		t.Fatal(err)
	}
	if model.EntityTable != "customers" || model.ItemTable != "invoices" {
		t.Fatalf("unexpected model %#v", model)
	}
}

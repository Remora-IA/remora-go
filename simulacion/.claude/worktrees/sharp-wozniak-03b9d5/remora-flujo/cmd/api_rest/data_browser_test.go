package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestParseAndImportCSV(t *testing.T) {
	sheets, err := parseUploadedTables("clientes.csv", []byte("nombre,deuda\nAna,100\nBeto,200\n"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	info, err := importSheet(db, "clientes.csv", sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Count != 2 {
		t.Fatalf("count = %d", info.Count)
	}
	if info.Name != "clientes_clientes" {
		t.Fatalf("table = %q", info.Name)
	}
	rows, err := dataTableRows(db, info.Name, info.Columns, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["nombre"] != "Beto" || rows[1]["deuda"] != "100" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestBusinessDataDBPathIsIsolatedPerBusiness(t *testing.T) {
	root := t.TempDir()
	panalbit := businessDataDBPath(root, "panalbit")
	retail := businessDataDBPath(root, "retail/demo")
	if panalbit == retail {
		t.Fatalf("expected separate paths, got %q", panalbit)
	}
	if !strings.HasPrefix(panalbit, filepath.Join(root, "temp", "business_data")) {
		t.Fatalf("panalbit path outside business_data: %q", panalbit)
	}
	if !strings.HasPrefix(retail, filepath.Join(root, "temp", "business_data")) {
		t.Fatalf("retail path outside business_data: %q", retail)
	}
	if strings.Contains(filepath.Base(retail), "/") || strings.Contains(filepath.Base(retail), "..") {
		t.Fatalf("unsafe business db path: %q", retail)
	}
}

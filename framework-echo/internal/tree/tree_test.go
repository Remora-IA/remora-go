package tree

import (
	"path/filepath"
	"testing"
)

func TestQALogRequiresConfigAndStoresEntry(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := tm.AddQALog("¿Qué haces hoy?", "Lo reviso en Excel", "mapear conducta actual"); err == nil {
		t.Fatal("expected qa log to require enabled config")
	}

	if err := tm.SetQALogEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := tm.AddQALog("¿Qué haces hoy?", "Lo reviso en Excel", "mapear conducta actual"); err != nil {
		t.Fatal(err)
	}
	if len(tm.QALog) != 1 {
		t.Fatalf("expected 1 qa log entry, got %d", len(tm.QALog))
	}
	if tm.QALog[0].Purpose != "mapear conducta actual" {
		t.Fatalf("unexpected purpose: %s", tm.QALog[0].Purpose)
	}
}

func TestSelectOpportunityRequiresValidatedOpportunity(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes["op_001"] = &Node{
		ID:     "op_001",
		Type:   TypeOpportunity,
		Status: StatusValidated,
		Title:  "Reporte automático",
	}
	tm.Nodes["op_002"] = &Node{
		ID:     "op_002",
		Type:   TypeOpportunity,
		Status: StatusPending,
		Title:  "Dashboard",
	}

	if err := tm.SelectOpportunity("op_002"); err == nil {
		t.Fatal("expected pending opportunity selection to fail")
	}
	if err := tm.SelectOpportunity("op_001"); err != nil {
		t.Fatal(err)
	}
	if err := tm.SelectOpportunity("op_001"); err != nil {
		t.Fatal(err)
	}
	if len(tm.SelectedOpportunityIDs) != 1 || tm.SelectedOpportunityIDs[0] != "op_001" {
		t.Fatalf("unexpected selected opportunities: %#v", tm.SelectedOpportunityIDs)
	}
}

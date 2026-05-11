package main

import (
	"testing"

	"framework-auditor/checks"
)

func TestDataGapsFromFindingsIncludesSchemaContactGap(t *testing.T) {
	gaps := dataGapsFromFindings([]checks.Finding{{
		Rule:     checks.RuleSchemaContactGap,
		Severity: checks.SeverityCritical,
		Endpoint: "clients",
		Field:    "email",
		Message:  "clients no tiene email",
		FixHint: map[string]interface{}{
			"required_artifact": "contact.destination.v1",
		},
	}})
	if len(gaps) != 1 {
		t.Fatalf("expected one gap, got %#v", gaps)
	}
	if gaps[0]["rule"] != checks.RuleSchemaContactGap {
		t.Fatalf("unexpected gap %#v", gaps[0])
	}
	fixHint, ok := gaps[0]["fix_hint"].(map[string]interface{})
	if !ok || fixHint["required_artifact"] != "contact.destination.v1" {
		t.Fatalf("missing fix hint %#v", gaps[0])
	}
}

func TestDataGapsFromFindingsGroupsMissingContactRows(t *testing.T) {
	gaps := dataGapsFromFindings([]checks.Finding{
		{Rule: checks.RuleMissingContact, Severity: checks.SeverityWarning, Endpoint: "clients", RecordID: "1", Field: "email", Message: "Falta email/contacto operativo: clients[1].email"},
		{Rule: checks.RuleMissingContact, Severity: checks.SeverityWarning, Endpoint: "clients", RecordID: "2", Field: "email", Message: "Falta email/contacto operativo: clients[2].email"},
	})
	if len(gaps) != 1 {
		t.Fatalf("expected grouped gap, got %#v", gaps)
	}
	if gaps[0]["count"] != 2 {
		t.Fatalf("expected count=2, got %#v", gaps[0])
	}
	if gaps[0]["type"] != "missing_contact" {
		t.Fatalf("unexpected gap type %#v", gaps[0])
	}
}

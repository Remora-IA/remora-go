package main

import (
	"os"
	"path/filepath"
	"testing"

	"channel/manifest"
)

func TestContractNeedsBusinessSQLitePathInfersFromRequiredArtifact(t *testing.T) {
	cmd := manifest.Command{Params: []string{"db"}}
	contract := nodeContract{
		Command:  "query",
		Requires: []string{"entity.ref.v1", "data.sqlite_db.v1"},
	}
	if !contractNeedsBusinessSQLitePath(cmd, contract) {
		t.Fatalf("expected db injection when capability requires data.sqlite_db.v1")
	}
}

func TestGenerateFlowPrerequisitesMarksDerivedFieldsAsUnresolvedWithoutEvidence(t *testing.T) {
	root := t.TempDir()
	writeTestBusinessPack(t, root, "biz_test")

	s := &server{rootDir: root}
	flow := testPrerequisiteFlow("biz_test")
	artifacts := map[string]flowRunArtifact{
		"entity.ref.v1": {Type: "entity.ref.v1", Payload: map[string]interface{}{"id": "184", "name": "Thiel-Effertz"}},
	}
	available := map[string]bool{
		"contact.destination.v1": true,
		"credentials.smtp":       true,
	}

	payload := s.generateFlowPrerequisites("run_1", flow, map[string]interface{}{"name": "Thiel-Effertz"}, artifacts, available, nil, nil)
	prereqs := payload["prerequisites"].([]flowPrerequisite)

	var derivedUnresolved int
	for _, prereq := range prereqs {
		if prereq.Status == prereqDerivable {
			derivedUnresolved++
		}
	}
	if derivedUnresolved != 3 {
		t.Fatalf("expected 3 unresolved derived fields, got %d (%#v)", derivedUnresolved, prereqs)
	}
	if ready, _ := payload["ready_for_terminal"].(bool); ready {
		t.Fatalf("ready_for_terminal should be false when derived fields are still unresolved")
	}
	if unresolved, _ := payload["unresolved_count"].(int); unresolved != 3 {
		t.Fatalf("unresolved_count = %d, want 3", unresolved)
	}
	for _, prereq := range prereqs {
		switch prereq.Field {
		case "amount", "date", "number":
			if prereq.Status != prereqDerivable {
				t.Fatalf("field %s should remain derivable without structured evidence: %#v", prereq.Field, prereq)
			}
			if prereq.Value != "" {
				t.Fatalf("field %s should not invent a value without evidence: %#v", prereq.Field, prereq)
			}
		}
	}
}

func TestGenerateFlowPrerequisitesIncludesExecutionBlockersFromFailedSteps(t *testing.T) {
	root := t.TempDir()
	writeTestBusinessPack(t, root, "biz_test")

	s := &server{rootDir: root}
	flow := testPrerequisiteFlow("biz_test")
	artifacts := map[string]flowRunArtifact{
		"entity.ref.v1":          {Type: "entity.ref.v1", Payload: map[string]interface{}{"id": "184", "name": "Thiel-Effertz"}},
		"message.draft.v1":       {Type: "message.draft.v1", Payload: map[string]interface{}{"to": "tom3bs@gmail.com", "value": "$7500"}},
		"contact.destination.v1": {Type: "contact.destination.v1", Payload: map[string]interface{}{"to": "tom3bs@gmail.com"}},
	}
	available := map[string]bool{
		"contact.destination.v1": true,
		"message.draft.v1":       true,
		"credentials.smtp":       true,
	}
	timeline := []flowRunStep{
		{
			Node:         "node_5_data_entity_360",
			Framework:    "sabio",
			Capability:   "data.entity_360",
			Role:         flowRolePipeline,
			Status:       "failed",
			HumanSummary: "Sabio no pudo leer la DB del negocio.",
		},
	}

	payload := s.generateFlowPrerequisites("run_2", flow, map[string]interface{}{"name": "Thiel-Effertz"}, artifacts, available, nil, timeline)
	if ready, _ := payload["ready_for_terminal"].(bool); ready {
		t.Fatalf("ready_for_terminal should be false when there are failed upstream steps")
	}
	if blockers, _ := payload["execution_blocker_count"].(int); blockers != 1 {
		t.Fatalf("execution_blocker_count = %d, want 1", blockers)
	}
	executionBlockers := payload["execution_blockers"].([]map[string]interface{})
	if len(executionBlockers) != 1 {
		t.Fatalf("expected 1 execution blocker, got %d", len(executionBlockers))
	}
	if got := executionBlockers[0]["capability"]; got != "data.entity_360" {
		t.Fatalf("blocker capability = %v, want data.entity_360", got)
	}
	prereqs := payload["prerequisites"].([]flowPrerequisite)
	for _, prereq := range prereqs {
		switch prereq.Field {
		case "amount", "date", "number":
			if prereq.Value != "" {
				t.Fatalf("field %s should not reuse unrelated artifact values: %#v", prereq.Field, prereq)
			}
		}
	}
}

func testPrerequisiteFlow(businessID string) flowManifest {
	return flowManifest{
		ID:         "flow_test",
		BusinessID: businessID,
		Nodes: []flowNode{
			{
				ID:         "send",
				Framework:  "mensajero",
				Capability: "message.send",
				Role:       flowRolePipeline,
				Requires:   []string{"message.draft.v1", "credentials.smtp", "contact.destination.v1"},
				Produces:   []string{"message.sent.v1"},
				Policies:   []string{"cycle_terminal"},
			},
		},
	}
}

func writeTestBusinessPack(t *testing.T, root, businessID string) {
	t.Helper()
	dir := filepath.Join(root, "framework-sabio", "businesses", businessID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir business pack dir: %v", err)
	}
	pack := `{
  "artifact_requirements": {
    "contact.destination.v1": {
      "fields": [
        {"table": "clients", "field": "email", "label": "Correo electronico del cliente/deudor", "reason": "Para enviar comunicacion al destinatario"}
      ]
    },
    "message.draft.v1": {
      "fields": [
        {"table": "clients", "field": "name", "label": "Nombre del cliente/deudor", "reason": "Para personalizar el mensaje"},
        {"table": "milestones", "field": "amount", "label": "Monto adeudado", "reason": "Para indicar el saldo pendiente en el mensaje", "derived": true},
        {"table": "milestones", "field": "date", "label": "Dias de mora", "reason": "Para indicar la antiguedad de la deuda", "derived": true},
        {"table": "billing_documents", "field": "number", "label": "Numero de factura", "reason": "Para referenciar documentos en el mensaje", "derived": true}
      ]
    }
  },
  "primary_entities": {
    "portfolio_client": {
      "table": "clients",
      "label": "cliente",
      "scope_key": "id",
      "display_column": "name"
    }
  },
  "scope_policies": {
    "scope_entity": "portfolio_client",
    "tables": {
      "clients": {"scope_column": "id"}
    }
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "sabio.business.json"), []byte(pack), 0o644); err != nil {
		t.Fatalf("write business pack: %v", err)
	}
}

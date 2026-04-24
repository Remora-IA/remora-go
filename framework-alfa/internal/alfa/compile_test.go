package alfa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompileDetectsOpenQuestions(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID: "test",
		Nodes: map[string]*Node{
			"ax_001": {
				ID:       "ax_001",
				Layer:    0,
				Type:     TypeAxiom,
				Title:    "Tiene datos en Excel",
				Evidence: []string{"datos de cobranza en Excel"},
				Status:   StatusValidated,
			},
			"th_001": {
				ID:               "th_001",
				Layer:            1,
				Type:             TypeTheory,
				Title:            "Debe priorizar por riesgo",
				Evidence:         []string{"fecha no basta"},
				Status:           StatusValidated,
				ParentID:         "ax_001",
				ValidationAnswer: "sí",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Factores de riesgo: tiempo, monto, comportamiento histórico",
				Evidence: []string{"factores mencionados"},
				Status:   "PENDING",
				ParentID: "th_001",
			},
			"pn_001": {
				ID:               "pn_001",
				Layer:            3,
				Type:             TypePain,
				Title:            "Contactos se acumulan",
				Evidence:         []string{"falta priorización"},
				Status:           StatusValidated,
				ParentID:         "tk_001",
				ValidationAnswer: "sí",
			},
			"op_001": {
				ID:               "op_001",
				Layer:            4,
				Type:             TypeOpportunity,
				Title:            "Dashboard de priorización diaria",
				Evidence:         []string{"prioriza por riesgo"},
				Status:           StatusValidated,
				ParentID:         "pn_001",
				ValidationAnswer: "sí",
			},
		},
	}
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(treePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := Compile(CompileOptions{EchoTreePath: treePath})
	if err != nil {
		t.Fatal(err)
	}
	if spec.ExportReady {
		t.Fatal("expected draft spec with open questions")
	}
	if len(spec.OpenQuestions) < 2 {
		t.Fatalf("expected at least 2 open questions, got %d", len(spec.OpenQuestions))
	}
}

func TestExportBravoIncludesOpenQuestionsAsRules(t *testing.T) {
	spec := &AlfaSpec{
		AutomationIntent: "Implementar dashboard",
		OpenQuestions: []OpenQuestion{
			{
				ID:              "oq_001",
				Reason:          "Falta regla",
				QuestionForEcho: "¿Cómo se calcula riesgo?",
			},
		},
	}

	flow := ExportBravo(spec, time.Time{})
	if len(flow.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(flow.Rules))
	}
	if flow.Rules[0].Name != "Pregunta abierta: oq_001" {
		t.Fatalf("unexpected rule name: %s", flow.Rules[0].Name)
	}
}

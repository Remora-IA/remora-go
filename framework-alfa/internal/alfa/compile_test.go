package alfa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestCompileUsesSelectedOpportunitiesByDefault(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID:              "test",
		SelectedOpportunityIDs: []string{"op_002"},
		Nodes: map[string]*Node{
			"ax_001": {
				ID:     "ax_001",
				Layer:  0,
				Type:   TypeAxiom,
				Title:  "Proceso confirmado",
				Status: StatusValidated,
			},
			"th_001": {
				ID:       "th_001",
				Layer:    1,
				Type:     TypeTheory,
				Title:    "Hay una tarea repetitiva",
				Status:   StatusValidated,
				ParentID: "ax_001",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Preparar reporte",
				Status:   StatusValidated,
				ParentID: "th_001",
			},
			"pn_001": {
				ID:       "pn_001",
				Layer:    3,
				Type:     TypePain,
				Title:    "Reporte toma tiempo",
				Status:   StatusValidated,
				ParentID: "tk_001",
			},
			"op_001": {
				ID:       "op_001",
				Layer:    4,
				Type:     TypeOpportunity,
				Title:    "Dashboard operativo",
				Status:   StatusValidated,
				ParentID: "pn_001",
			},
			"op_002": {
				ID:       "op_002",
				Layer:    4,
				Type:     TypeOpportunity,
				Title:    "Resumen semanal",
				Status:   StatusValidated,
				ParentID: "pn_001",
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
	if len(spec.SelectedOpportunities) != 1 {
		t.Fatalf("expected 1 selected opportunity, got %d", len(spec.SelectedOpportunities))
	}
	if spec.SelectedOpportunities[0].ID != "op_002" {
		t.Fatalf("expected op_002, got %s", spec.SelectedOpportunities[0].ID)
	}
}

func TestCompileBlocksWhenDataTransportIsMissing(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID:              "test",
		SelectedOpportunityIDs: []string{"op_001"},
		Nodes: map[string]*Node{
			"ax_001": {
				ID:       "ax_001",
				Layer:    0,
				Type:     TypeAxiom,
				Title:    "Los prospectos viven en WhatsApp y en la cabeza del usuario",
				Evidence: []string{"No existe registro estructurado"},
				Status:   StatusValidated,
			},
			"th_001": {
				ID:       "th_001",
				Layer:    1,
				Type:     TypeTheory,
				Title:    "El seguimiento informal genera pérdida de contexto",
				Status:   StatusValidated,
				ParentID: "ax_001",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Contactar prospectos por WhatsApp",
				Status:   StatusValidated,
				ParentID: "th_001",
			},
			"pn_001": {
				ID:       "pn_001",
				Layer:    3,
				Type:     TypePain,
				Title:    "Abrir WhatsApp muchas veces al día",
				Status:   StatusValidated,
				ParentID: "tk_001",
			},
			"op_001": {
				ID:       "op_001",
				Layer:    4,
				Type:     TypeOpportunity,
				Title:    "Vista unificada de prospectos",
				Status:   StatusValidated,
				ParentID: "pn_001",
			},
		},
	}
	writeEchoTree(t, treePath, tree)

	spec, err := Compile(CompileOptions{EchoTreePath: treePath})
	if err != nil {
		t.Fatal(err)
	}
	if spec.ExportReady {
		t.Fatal("expected export_ready=false when data transport is missing")
	}
	if !hasOpenQuestionReason(spec, "datos actuales") {
		t.Fatalf("expected data transport open question, got %#v", spec.OpenQuestions)
	}
}

func TestCompileAllowsConfirmedDataTransport(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID:              "test",
		SelectedOpportunityIDs: []string{"op_001"},
		Nodes: map[string]*Node{
			"ax_001": {
				ID:       "ax_001",
				Layer:    0,
				Type:     TypeAxiom,
				Title:    "La lista de prospectos se puede exportar como CSV completo",
				Evidence: []string{"El usuario confirma que ya puede entregar un archivo CSV completo con los prospectos"},
				Status:   StatusValidated,
			},
			"th_001": {
				ID:       "th_001",
				Layer:    1,
				Type:     TypeTheory,
				Title:    "El seguimiento informal genera pérdida de contexto",
				Status:   StatusValidated,
				ParentID: "ax_001",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Contactar prospectos",
				Status:   StatusValidated,
				ParentID: "th_001",
			},
			"pn_001": {
				ID:       "pn_001",
				Layer:    3,
				Type:     TypePain,
				Title:    "Seguimiento se acumula",
				Status:   StatusValidated,
				ParentID: "tk_001",
			},
			"op_001": {
				ID:       "op_001",
				Layer:    4,
				Type:     TypeOpportunity,
				Title:    "Vista unificada de prospectos",
				Status:   StatusValidated,
				ParentID: "pn_001",
			},
		},
	}
	writeEchoTree(t, treePath, tree)

	spec, err := Compile(CompileOptions{EchoTreePath: treePath})
	if err != nil {
		t.Fatal(err)
	}
	if !spec.ExportReady {
		t.Fatalf("expected export_ready=true, got open questions: %#v", spec.OpenQuestions)
	}
}

func TestCompileBlocksManualCaptureWithoutOperationalViability(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID:              "test",
		SelectedOpportunityIDs: []string{"op_001"},
		Nodes: map[string]*Node{
			"ax_001": {
				ID:       "ax_001",
				Layer:    0,
				Type:     TypeAxiom,
				Title:    "Los leads llegan por correo diario y se contactan por WhatsApp",
				Evidence: []string{"El correo contiene nombre y contacto; WhatsApp es el canal de conversación"},
				Status:   StatusValidated,
			},
			"th_001": {
				ID:       "th_001",
				Layer:    1,
				Type:     TypeTheory,
				Title:    "El seguimiento depende de memoria y relectura de chats",
				Status:   StatusValidated,
				ParentID: "ax_001",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Contactar leads y hacer seguimiento por WhatsApp",
				Status:   StatusValidated,
				ParentID: "th_001",
			},
			"pn_001": {
				ID:       "pn_001",
				Layer:    3,
				Type:     TypePain,
				Title:    "Comete errores al retomar y deja leads de lado",
				Status:   StatusValidated,
				ParentID: "tk_001",
			},
			"op_001": {
				ID:               "op_001",
				Layer:            4,
				Type:             TypeOpportunity,
				Title:            "Captura manual de interés, productos y compromisos para retomar conversaciones",
				Evidence:         []string{"El usuario necesita guardar interés, productos de interés y compromisos pendientes"},
				Status:           StatusValidated,
				ParentID:         "pn_001",
				ValidationAnswer: "sí, eso serviría",
			},
		},
	}
	writeEchoTree(t, treePath, tree)

	spec, err := Compile(CompileOptions{EchoTreePath: treePath})
	if err != nil {
		t.Fatal(err)
	}
	if spec.ExportReady {
		t.Fatal("expected export_ready=false when manual capture has no operational viability")
	}
	if !hasOpenQuestionReason(spec, "hábito operativo") {
		t.Fatalf("expected operational viability open question, got %#v", spec.OpenQuestions)
	}
}

func TestCompileTreatsMissingManualViabilityAsRiskAfterFatigue(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID:              "test",
		SelectedOpportunityIDs: []string{"op_001"},
		Signals: []SignalEntry{
			{Type: "fatigue", Note: "El usuario dijo: estas preguntando muchas cosas"},
		},
		Nodes: map[string]*Node{
			"ax_001": {
				ID:       "ax_001",
				Layer:    0,
				Type:     TypeAxiom,
				Title:    "Los proveedores responden por WhatsApp",
				Evidence: []string{"WhatsApp es el canal actual"},
				Status:   StatusValidated,
			},
			"th_001": {
				ID:       "th_001",
				Layer:    1,
				Type:     TypeTheory,
				Title:    "La falta de visibilidad retrasa cotizaciones",
				Status:   StatusValidated,
				ParentID: "ax_001",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Coordinar cotizaciones por WhatsApp",
				Status:   StatusValidated,
				ParentID: "th_001",
			},
			"pn_001": {
				ID:       "pn_001",
				Layer:    3,
				Type:     TypePain,
				Title:    "Retrasos hacen ver poco profesional el servicio",
				Status:   StatusValidated,
				ParentID: "tk_001",
			},
			"op_001": {
				ID:               "op_001",
				Layer:            4,
				Type:             TypeOpportunity,
				Title:            "Dashboard con captura manual de estado de cotizaciones",
				Evidence:         []string{"El equipo marcaría estado cuando revisa WhatsApp"},
				Status:           StatusValidated,
				ParentID:         "pn_001",
				ValidationAnswer: "sí, ayudaría mucho",
			},
		},
	}
	writeEchoTree(t, treePath, tree)

	spec, err := Compile(CompileOptions{EchoTreePath: treePath})
	if err != nil {
		t.Fatal(err)
	}
	if spec.ExportReady {
		t.Fatal("expected export_ready=false while manual capture viability remains unconfirmed")
	}
	if !hasOpenQuestionReason(spec, "Riesgo no resuelto") {
		t.Fatalf("expected risk-oriented open question, got %#v", spec.OpenQuestions)
	}
	if len(spec.ConversationSignals) != 1 {
		t.Fatalf("expected conversation signal to be preserved, got %#v", spec.ConversationSignals)
	}
}

func TestCompileAllowsManualCaptureWithOperationalViability(t *testing.T) {
	dir := t.TempDir()
	treePath := filepath.Join(dir, "frameworkecho.json")
	tree := EchoTree{
		ProjectID:              "test",
		SelectedOpportunityIDs: []string{"op_001"},
		Nodes: map[string]*Node{
			"ax_001": {
				ID:       "ax_001",
				Layer:    0,
				Type:     TypeAxiom,
				Title:    "Los leads llegan por correo diario y se contactan por WhatsApp",
				Evidence: []string{"El correo contiene nombre y contacto; WhatsApp es el canal de conversación"},
				Status:   StatusValidated,
			},
			"th_001": {
				ID:       "th_001",
				Layer:    1,
				Type:     TypeTheory,
				Title:    "El seguimiento depende de memoria y relectura de chats",
				Status:   StatusValidated,
				ParentID: "ax_001",
			},
			"tk_001": {
				ID:       "tk_001",
				Layer:    2,
				Type:     TypeTask,
				Title:    "Contactar leads y hacer seguimiento por WhatsApp",
				Status:   StatusValidated,
				ParentID: "th_001",
			},
			"pn_001": {
				ID:       "pn_001",
				Layer:    3,
				Type:     TypePain,
				Title:    "Comete errores al retomar y deja leads de lado",
				Status:   StatusValidated,
				ParentID: "tk_001",
			},
			"op_001": {
				ID:    "op_001",
				Layer: 4,
				Type:  TypeOpportunity,
				Title: "Captura rápida de interés, productos y compromisos para retomar conversaciones",
				Evidence: []string{
					"El usuario puede registrar apenas corta la llamada",
					"El esfuerzo aceptado es rápido, en segundos, con pocos campos",
				},
				Status:           StatusValidated,
				ParentID:         "pn_001",
				ValidationAnswer: "sí, eso serviría si es rápido",
			},
		},
	}
	writeEchoTree(t, treePath, tree)

	spec, err := Compile(CompileOptions{EchoTreePath: treePath})
	if err != nil {
		t.Fatal(err)
	}
	if !spec.ExportReady {
		t.Fatalf("expected export_ready=true, got open questions: %#v", spec.OpenQuestions)
	}
}

func writeEchoTree(t *testing.T, path string, tree EchoTree) {
	t.Helper()
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func hasOpenQuestionReason(spec *AlfaSpec, needle string) bool {
	for _, q := range spec.OpenQuestions {
		if strings.Contains(q.Reason, needle) {
			return true
		}
	}
	return false
}

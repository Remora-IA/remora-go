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

func TestLayerProgressionNeedsOnlyOneValidatedParentLayer(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}

	ax, err := tm.AddNode(TypeAxiom, "Proceso confirmado", []string{"evidencia"}, "")
	if err != nil {
		t.Fatal(err)
	}
	th, err := tm.AddNode(TypeTheory, "Hipótesis validable", []string{"evidencia"}, ax.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.ValidateNode(th.ID, "sí"); err != nil {
		t.Fatal(err)
	}
	tk, err := tm.AddNode(TypeTask, "Tarea repetitiva", []string{"evidencia"}, th.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.ValidateNode(tk.ID, "sí"); err != nil {
		t.Fatal(err)
	}
	pn, err := tm.AddNode(TypePain, "Dolor confirmado", []string{"evidencia"}, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.ValidateNode(pn.ID, "sí"); err != nil {
		t.Fatal(err)
	}
	if _, err := tm.AddNode(TypeOpportunity, "Oportunidad candidata", []string{"evidencia"}, pn.ID); err != nil {
		t.Fatal(err)
	}
}

func TestAssessReadinessBlocksManualCaptureWithoutViability(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Captura manual de interés, productos y compromisos", []string{
		"El usuario necesita guardar interés, productos y compromisos",
	})
	tm.SelectedOpportunityIDs = []string{"op_001"}

	report := tm.AssessAlfaReadiness()
	if report.ReadyForAlfa {
		t.Fatal("expected readiness to block manual capture without viability")
	}
	if !hasReadinessCheck(report, "manual_capture_viability", false) {
		t.Fatalf("expected failed manual_capture_viability check: %#v", report.Checks)
	}
	if report.RecommendedAction != RecommendedAskNext {
		t.Fatalf("unexpected action: %s", report.RecommendedAction)
	}
}

func TestAssessReadinessRecommendsEarlyAlfaAfterTaskAndPain(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = map[string]*Node{
		"ax_001": {
			ID:       "ax_001",
			Type:     TypeAxiom,
			Layer:    0,
			Title:    "Facturas y transferencias llegan por WhatsApp",
			Evidence: []string{"El usuario recibe capturas en grupos"},
			Status:   StatusValidated,
		},
		"th_001": {
			ID:       "th_001",
			Type:     TypeTheory,
			Layer:    1,
			Title:    "El cruce manual genera errores",
			Status:   StatusValidated,
			ParentID: "ax_001",
		},
		"tk_001": {
			ID:       "tk_001",
			Type:     TypeTask,
			Layer:    2,
			Title:    "Cruzar transferencias con facturas",
			Status:   StatusValidated,
			ParentID: "th_001",
		},
		"pn_001": {
			ID:       "pn_001",
			Type:     TypePain,
			Layer:    3,
			Title:    "Pierde tiempo y no sabe qué pago corresponde a qué factura",
			Status:   StatusValidated,
			ParentID: "tk_001",
		},
	}

	report := tm.AssessAlfaReadiness()
	if report.RecommendedAction != RecommendedConsultAlfaEarly {
		t.Fatalf("expected early alfa consultation, got %s", report.RecommendedAction)
	}
	if !containsReadinessAny(report.NextQuestion, "compila un draft", "primera automatización") {
		t.Fatalf("expected early alfa instruction, got %q", report.NextQuestion)
	}
}

func TestAssessReadinessAsksForResourceExampleWhenEvidenceCanCloseGap(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Ordenar facturas recibidas por foto", []string{
		"Las facturas llegan como foto o pantallazo, pero falta confirmar cómo se ve el recurso real; copiarlas uno por uno no sirve",
	})
	tm.Nodes["ax_001"].Title = "Las facturas llegan como documentos sueltos"
	tm.Nodes["ax_001"].Evidence = []string{"El canal y el camino de entrada todavía no están confirmados"}
	tm.SelectedOpportunityIDs = []string{"op_001"}

	report := tm.AssessAlfaReadiness()
	if report.ReadyForAlfa {
		t.Fatal("expected not ready before data transport is confirmed")
	}
	if report.RecommendedAction != RecommendedAskNext {
		t.Fatalf("unexpected action: %s", report.RecommendedAction)
	}
	if !containsReadinessAny(report.NextQuestion, "ejemplo anonimizado", "mensajes o contexto") {
		t.Fatalf("expected resource example question, got %q", report.NextQuestion)
	}
}

func TestAssessReadinessAsksForContextCommitmentWhenWhatsappTransferNeedsManualContext(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Registrar transferencias de WhatsApp", []string{
		"Las transferencias llegan por WhatsApp como captura y hay que relacionarlas con facturas",
	})
	tm.SelectedOpportunityIDs = []string{"op_001"}

	report := tm.AssessAlfaReadiness()
	if report.ReadyForAlfa {
		t.Fatal("expected not ready before context commitment is confirmed")
	}
	if report.RecommendedAction != RecommendedAskNext {
		t.Fatalf("unexpected action: %s", report.RecommendedAction)
	}
	if !containsReadinessAny(report.NextQuestion, "mensaje corto", "comprometerse") {
		t.Fatalf("expected context commitment question, got %q", report.NextQuestion)
	}
}

func TestAssessReadinessAllowsContextCommitmentAfterScreenshot(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Registrar transferencias de WhatsApp", []string{
		"Las transferencias llegan por WhatsApp como captura",
		"El usuario puede mandar después del pantallazo un mensaje corto con Cliente X, factura Y y pago total o parcial",
	})
	tm.SelectedOpportunityIDs = []string{"op_001"}

	report := tm.AssessAlfaReadiness()
	if !report.ReadyForAlfa {
		t.Fatalf("expected ready after context commitment, got %#v", report)
	}
}

func TestAssessReadinessAllowsValidatedManualCapture(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Captura rápida de interés, productos y compromisos", []string{
		"El usuario puede registrar apenas corta la llamada",
		"El esfuerzo aceptado es rápido, en segundos, con pocos campos",
	})
	tm.SelectedOpportunityIDs = []string{"op_001"}

	report := tm.AssessAlfaReadiness()
	if !report.ReadyForAlfa {
		t.Fatalf("expected ready for alfa, got %#v", report)
	}
	if report.RecommendedAction != RecommendedPassToAlfa {
		t.Fatalf("unexpected action: %s", report.RecommendedAction)
	}
}

func TestAssessReadinessRecommendsMinimumHypothesisAfterUnknown(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Captura manual de interés, productos y compromisos", []string{
		"El usuario necesita guardar interés, productos y compromisos",
	})
	tm.QALog = []QALogEntry{
		{
			Question: "¿Opciones rápidas o nota corta?",
			Answer:   "No tengo idea la verdad",
			Purpose:  "refinar fricción operativa",
		},
	}

	report := tm.AssessAlfaReadiness()
	if report.ReadyForAlfa {
		t.Fatal("expected not ready until opportunity is selected and viability is resolved")
	}
	if report.RecommendedAction != RecommendedValidateMinimumHypothesis {
		t.Fatalf("expected validate minimum hypothesis, got %s", report.RecommendedAction)
	}
	if report.NextQuestion == "" {
		t.Fatal("expected concrete next question")
	}
}

func TestAssessReadinessClosesDiscoveryWithRiskAfterFatigue(t *testing.T) {
	tm, err := LoadOrCreate(filepath.Join(t.TempDir(), "frameworkecho.json"))
	if err != nil {
		t.Fatal(err)
	}
	tm.Nodes = readinessLeadNodes("Captura manual de interés, productos y compromisos", []string{
		"El usuario necesita guardar interés, productos y compromisos",
	})
	tm.SelectedOpportunityIDs = []string{"op_001"}
	tm.Signals = []SignalEntry{
		{
			Type: "fatigue",
			Note: "El usuario dijo: estas preguntando muchas cosas",
		},
	}

	report := tm.AssessAlfaReadiness()
	if report.ReadyForAlfa {
		t.Fatal("expected not ready when manual capture viability is still unconfirmed")
	}
	if report.RecommendedAction != RecommendedCloseDiscoveryWithRisk {
		t.Fatalf("expected close discovery with risk, got %s", report.RecommendedAction)
	}
	if !hasRisk(report, "manual_capture_viability_unconfirmed") {
		t.Fatalf("expected manual capture risk, got %#v", report.Risks)
	}
}

func readinessLeadNodes(opTitle string, opEvidence []string) map[string]*Node {
	return map[string]*Node{
		"ax_001": {
			ID:       "ax_001",
			Type:     TypeAxiom,
			Layer:    0,
			Title:    "Los leads llegan por correo diario y se contactan por WhatsApp",
			Evidence: []string{"El correo contiene nombre y contacto; WhatsApp es el canal de conversación"},
			Status:   StatusValidated,
		},
		"th_001": {
			ID:       "th_001",
			Type:     TypeTheory,
			Layer:    1,
			Title:    "El seguimiento depende de memoria y relectura de chats",
			Status:   StatusValidated,
			ParentID: "ax_001",
		},
		"tk_001": {
			ID:       "tk_001",
			Type:     TypeTask,
			Layer:    2,
			Title:    "Contactar leads y hacer seguimiento por WhatsApp",
			Status:   StatusValidated,
			ParentID: "th_001",
		},
		"pn_001": {
			ID:       "pn_001",
			Type:     TypePain,
			Layer:    3,
			Title:    "Comete errores al retomar y deja leads de lado",
			Status:   StatusValidated,
			ParentID: "tk_001",
		},
		"op_001": {
			ID:               "op_001",
			Type:             TypeOpportunity,
			Layer:            4,
			Title:            opTitle,
			Evidence:         opEvidence,
			Status:           StatusValidated,
			ParentID:         "pn_001",
			ValidationAnswer: "sí, eso serviría",
		},
	}
}

func hasReadinessCheck(report ReadinessReport, id string, passed bool) bool {
	for _, check := range report.Checks {
		if check.ID == id && check.Passed == passed {
			return true
		}
	}
	return false
}

func hasRisk(report ReadinessReport, risk string) bool {
	for _, item := range report.Risks {
		if item == risk {
			return true
		}
	}
	return false
}

package tree

import (
	"fmt"
	"strings"
)

const (
	RecommendedAskNext                   = "ask_next_missing_fact"
	RecommendedConsultAlfaEarly          = "consult_alfa_early"
	RecommendedValidateMinimumHypothesis = "validate_minimum_hypothesis"
	RecommendedCloseDiscoveryWithRisk    = "close_discovery_with_risk"
	RecommendedSelectOpportunity         = "select_opportunity"
	RecommendedPassToAlfa                = "pass_to_alfa"
)

type ReadinessReport struct {
	ReadyForAlfa      bool             `json:"ready_for_alfa"`
	RecommendedAction string           `json:"recommended_action"`
	NextQuestion      string           `json:"next_question,omitempty"`
	Risks             []string         `json:"risks,omitempty"`
	Checks            []ReadinessCheck `json:"checks"`
}

type ReadinessCheck struct {
	ID      string `json:"id"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

func (t *FrameworkEcho) AssessAlfaReadiness() ReadinessReport {
	text := strings.ToLower(t.readinessText())

	hasTask := t.hasValidatedNode(TypeTask)
	hasPain := t.hasValidatedNode(TypePain)
	hasValidatedOpportunity := t.hasValidatedNode(TypeOpportunity)
	hasSelectedOpportunity := len(t.SelectedOpportunities()) > 0
	hasTransport := hasReadinessDataTransport(text)
	requiresManualCapture := needsReadinessManualCapture(text)
	hasManualViability := !requiresManualCapture || hasReadinessManualViability(text)
	hasMinimumInput := hasReadinessMinimumInput(text)
	hasUnknownFatigue := t.hasUnknownAnswer()
	hasConversationFatigue := t.hasConversationFatigue()
	hasCoreDiscovery := hasTask && hasPain && hasValidatedOpportunity
	risks := readinessRisks(hasTransport, hasManualViability)

	checks := []ReadinessCheck{
		{ID: "task_confirmed", Passed: hasTask, Details: "Existe al menos una TASK validada."},
		{ID: "pain_confirmed", Passed: hasPain, Details: "Existe al menos un PAIN validado."},
		{ID: "opportunity_validated", Passed: hasValidatedOpportunity, Details: "Existe al menos una OPPORTUNITY validada."},
		{ID: "opportunity_selected", Passed: hasSelectedOpportunity, Details: "Existe al menos una OPPORTUNITY seleccionada para Alfa."},
		{ID: "data_transport_confirmed", Passed: hasTransport, Details: "Echo registró dónde viven los datos y un camino de entrada usable."},
		{ID: "manual_capture_viability", Passed: hasManualViability, Details: "Si hay captura manual, Echo validó momento real y esfuerzo tolerado."},
	}

	ready := hasTask && hasPain && hasValidatedOpportunity && hasSelectedOpportunity && hasTransport && hasManualViability
	action := RecommendedAskNext
	question := ""

	switch {
	case ready:
		action = RecommendedPassToAlfa
	case hasCoreDiscovery && hasConversationFatigue:
		action = RecommendedCloseDiscoveryWithRisk
		question = "Cierra discovery sin más preguntas abiertas. Pasa a Alfa como draft/prototipo con los riesgos explícitos."
	case hasTask && hasPain && hasValidatedOpportunity && hasMinimumInput && hasUnknownFatigue:
		action = RecommendedValidateMinimumHypothesis
		question = t.minimumHypothesisQuestion()
	case hasTask && hasPain && hasValidatedOpportunity && !hasSelectedOpportunity:
		action = RecommendedSelectOpportunity
		question = "Selecciona la OPPORTUNITY validada que se quiere trabajar con Alfa."
	case !hasTask:
		question = "¿Cuál es la tarea repetitiva concreta que ocurre en este proceso?"
	case !hasPain:
		question = "¿Qué impacto concreto tiene esa tarea cuando sale mal, se atrasa o depende de memoria?"
	case !hasValidatedOpportunity:
		action = RecommendedConsultAlfaEarly
		question = "Consulta Alfa antes de seguir preguntando: `cd ../framework-alfa && ./frameworkalfa compile --echo-tree ../framework-echo/frameworkecho.json --out temp/alfa_spec_draft.json --allow-draft=true`, para idear una primera automatización y devolver solo los gaps que bloquean esa iteración."
	case !hasTransport:
		question = t.dataTransportQuestion(text)
	case !hasManualViability:
		question = t.manualViabilityQuestion(text)
	default:
		question = "Falta aclarar el hueco operativo que impide compilar la oportunidad sin inventar."
	}

	return ReadinessReport{
		ReadyForAlfa:      ready,
		RecommendedAction: action,
		NextQuestion:      question,
		Risks:             risks,
		Checks:            checks,
	}
}

func (t *FrameworkEcho) hasValidatedNode(nodeType string) bool {
	for _, node := range t.Nodes {
		if node.Type == nodeType && node.Status == StatusValidated {
			return true
		}
	}
	return false
}

func (t *FrameworkEcho) readinessText() string {
	var b strings.Builder
	for _, node := range t.Nodes {
		b.WriteString(" ")
		b.WriteString(node.Title)
		b.WriteString(" ")
		b.WriteString(strings.Join(node.Evidence, " "))
		b.WriteString(" ")
		b.WriteString(node.ValidationAnswer)
		b.WriteString(" ")
		b.WriteString(strings.Join(node.Perceptions, " "))
	}
	for _, entry := range t.QALog {
		b.WriteString(" ")
		b.WriteString(entry.Question)
		b.WriteString(" ")
		b.WriteString(entry.Answer)
		b.WriteString(" ")
		b.WriteString(entry.Purpose)
	}
	for _, signal := range t.Signals {
		b.WriteString(" ")
		b.WriteString(signal.Type)
		b.WriteString(" ")
		b.WriteString(signal.Note)
	}
	return b.String()
}

func (t *FrameworkEcho) hasUnknownAnswer() bool {
	count := 0
	for _, entry := range t.QALog {
		answer := strings.ToLower(entry.Answer)
		if containsReadinessAny(answer, "no sé", "no se", "no tengo idea", "ni idea", "no sabría", "no sabria") {
			count++
		}
	}
	if count > 0 {
		return true
	}
	for _, node := range t.Nodes {
		answer := strings.ToLower(node.ValidationAnswer)
		if containsReadinessAny(answer, "no sé", "no se", "no tengo idea", "ni idea", "no sabría", "no sabria") {
			return true
		}
	}
	return false
}

func (t *FrameworkEcho) hasConversationFatigue() bool {
	for _, signal := range t.Signals {
		if signal.Type == "fatigue" || signal.Type == "low_attention" {
			return true
		}
		if containsReadinessAny(strings.ToLower(signal.Note), fatiguePhrases()...) {
			return true
		}
	}
	for _, entry := range t.QALog {
		if containsReadinessAny(strings.ToLower(entry.Answer), fatiguePhrases()...) {
			return true
		}
	}
	for _, node := range t.Nodes {
		if containsReadinessAny(strings.ToLower(node.ValidationAnswer), fatiguePhrases()...) {
			return true
		}
	}
	return false
}

func fatiguePhrases() []string {
	return []string{
		"muchas preguntas",
		"preguntando muchas cosas",
		"no te entiendo",
		"no entiendo",
		"qué se yo",
		"que se yo",
		"me estás preguntando mucho",
		"me estas preguntando mucho",
		"estoy cansado",
		"ya te dije",
	}
}

func readinessRisks(hasTransport, hasManualViability bool) []string {
	var risks []string
	if !hasTransport {
		risks = append(risks, "data_transport_unconfirmed")
	}
	if !hasManualViability {
		risks = append(risks, "manual_capture_viability_unconfirmed")
	}
	return risks
}

func (t *FrameworkEcho) dataTransportQuestion(text string) string {
	if needsReadinessResourceExample(text) {
		return "¿Tienes un ejemplo anonimizado de cómo llega esa información hoy, incluyendo captura/archivo y los mensajes o contexto alrededor?"
	}
	return "¿Dónde vive hoy la información necesaria y cuál es el camino mínimo realista para llevarla a la automatización?"
}

func (t *FrameworkEcho) manualViabilityQuestion(text string) string {
	if needsReadinessContextCommitment(text) {
		return "Para automatizar esto hay que unir el recurso con su contexto. Si hoy ese contexto no viene escrito, ¿qué mensaje corto podría agregar la persona justo después y puede comprometerse a hacerlo?"
	}
	return "Si esta solución requiere registrar información manualmente, ¿en qué momento real lo haría el usuario y qué esfuerzo máximo acepta?"
}

func (t *FrameworkEcho) minimumHypothesisQuestion() string {
	selected := t.SelectedOpportunities()
	if len(selected) > 0 {
		return fmt.Sprintf("Para cerrar: ¿la oportunidad '%s' sí serviría como primera versión mínima, o qué parte no calza?", selected[0].Title)
	}
	for _, node := range t.Nodes {
		if node.Type == TypeOpportunity && node.Status == StatusValidated {
			return fmt.Sprintf("Para cerrar: ¿la oportunidad '%s' sí serviría como primera versión mínima, o qué parte no calza?", node.Title)
		}
	}
	return "Para cerrar: con el dolor y el input mínimo ya claros, ¿esta primera versión mínima sí serviría o qué parte no calza?"
}

func hasReadinessDataTransport(text string) bool {
	if !containsReadinessAny(text,
		"import",
		"export",
		"archivo completo",
		"csv",
		"xlsx",
		"excel completo",
		"subir",
		"cargar archivo",
		"entregar archivo",
		"base de datos",
		"sqlite",
		"api",
		"integración",
		"integracion",
		"formulario",
		"captur",
		"correo diario",
		"whatsapp",
	) {
		return false
	}
	if containsReadinessAny(text, "uno por uno", "manualmente uno por uno", "copiar uno a uno") {
		return false
	}
	return true
}

func needsReadinessResourceExample(text string) bool {
	return containsReadinessAny(text,
		"captura",
		"pantallazo",
		"screenshot",
		"foto",
		"imagen",
		"comprobante",
		"mensaje",
		"chat",
		"whatsapp",
		"correo",
		"pdf",
		"archivo",
		"documento",
		"papel",
	)
}

func needsReadinessContextCommitment(text string) bool {
	hasUnstructuredResource := containsReadinessAny(text,
		"captura",
		"pantallazo",
		"screenshot",
		"foto",
		"imagen",
		"comprobante",
		"mensaje",
		"chat",
		"whatsapp",
		"correo",
		"pdf",
		"archivo",
		"documento",
		"papel",
	)
	hasRelationshipNeed := containsReadinessAny(text,
		"cruzar",
		"relacionar",
		"relacion",
		"relación",
		"asociar",
		"calzar",
		"conciliar",
		"vincular",
		"unir",
		"corresponde",
		"correspondencia",
		"matching",
	)
	hasContext := containsReadinessAny(text,
		"contexto confirmado",
		"mensaje corto",
		"contexto",
		"referencia",
		"identificador",
		"compromet",
	)
	return hasUnstructuredResource && hasRelationshipNeed && !hasContext
}

func needsReadinessManualCapture(text string) bool {
	return containsReadinessAny(text,
		"registr",
		"captur",
		"carga manual",
		"manual",
		"formulario",
		"completar",
		"llenar",
		"anotar",
		"guardar",
		"nota corta",
	)
}

func hasReadinessManualViability(text string) bool {
	hasMoment := containsReadinessAny(text,
		"apenas corto",
		"apenas corta",
		"apenas termina",
		"después de cada llamada",
		"despues de cada llamada",
		"después de hablar",
		"despues de hablar",
		"al terminar",
		"al final de la llamada",
		"después del pantallazo",
		"despues del pantallazo",
		"después de la captura",
		"despues de la captura",
		"momento de captura",
		"en el momento",
		"durante la llamada",
		"puede mandar",
	)
	hasTolerance := containsReadinessAny(text,
		"rápido",
		"rapido",
		"segundos",
		"10-20 segundos",
		"nota corta",
		"mensaje corto",
		"opciones rápidas",
		"opciones rapidas",
		"mínimo",
		"minimo",
		"no rompe",
		"sin fricción",
		"sin friccion",
		"pocos campos",
		"carga mínima",
		"carga minima",
	)
	return hasMoment && hasTolerance
}

func hasReadinessMinimumInput(text string) bool {
	return containsReadinessAny(text,
		"mínimo",
		"minimo",
		"interés",
		"interes",
		"productos",
		"compromisos",
		"último contacto",
		"ultimo contacto",
		"siguiente acción",
		"siguiente accion",
		"estado",
	)
}

func containsReadinessAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

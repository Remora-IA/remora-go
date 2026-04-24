package alfa

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	StatusValidated = "VALIDATED"
	TypeAxiom       = "AXIOM"
	TypeTheory      = "THEORY"
	TypeTask        = "TASK"
	TypePain        = "PAIN"
	TypeOpportunity = "OPPORTUNITY"
)

type CompileOptions struct {
	EchoTreePath string
	Opportunity  string
	OutputPath   string
	AllowDraft   bool
	GeneratedNow time.Time
}

func Compile(opts CompileOptions) (*AlfaSpec, error) {
	if opts.EchoTreePath == "" {
		return nil, fmt.Errorf("echo tree path is required")
	}
	if opts.GeneratedNow.IsZero() {
		opts.GeneratedNow = time.Now()
	}

	tree, err := LoadEchoTree(opts.EchoTreePath)
	if err != nil {
		return nil, err
	}

	opportunities, err := selectOpportunities(tree, opts.Opportunity)
	if err != nil {
		return nil, err
	}

	spec := &AlfaSpec{
		Version:               "0.1",
		Generated:             opts.GeneratedNow.Format(time.RFC3339),
		SourceTree:            opts.EchoTreePath,
		ProjectID:             tree.ProjectID,
		ClientName:            tree.ClientName,
		SelectedOpportunities: make([]OpportunitySpec, 0, len(opportunities)),
	}

	seen := newSeenSet()
	for _, op := range opportunities {
		spec.SelectedOpportunities = append(spec.SelectedOpportunities, OpportunitySpec{
			ID:               op.ID,
			Title:            op.Title,
			ParentPainID:     op.ParentID,
			Evidence:         op.Evidence,
			ValidationAnswer: op.ValidationAnswer,
		})
		collectLineage(tree, op, spec, seen)
	}

	spec.AutomationIntent = buildIntent(spec)
	spec.IdealSteps = buildIdealSteps(spec)
	spec.BusinessRules = buildBusinessRules(spec)
	spec.CriticalVariables = buildCriticalVariables(spec)
	spec.SuccessCriteria = buildSuccessCriteria(spec)
	spec.EdgeCases = buildEdgeCases(spec)
	spec.OpenQuestions = buildOpenQuestions(tree, spec)
	spec.ExportReady = len(spec.OpenQuestions) == 0

	return spec, nil
}

func LoadEchoTree(path string) (*EchoTree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read echo tree: %w", err)
	}
	var tree EchoTree
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("parse echo tree: %w", err)
	}
	if tree.Nodes == nil {
		return nil, fmt.Errorf("echo tree has no nodes")
	}
	return &tree, nil
}

func SaveSpec(spec *AlfaSpec, path string) error {
	if path == "" {
		path = "alfa_spec.json"
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadSpec(path string) (*AlfaSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read alfa spec: %w", err)
	}
	var spec AlfaSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse alfa spec: %w", err)
	}
	return &spec, nil
}

func selectOpportunities(tree *EchoTree, requested string) ([]*Node, error) {
	if requested != "" {
		node, ok := tree.Nodes[requested]
		if !ok {
			return nil, fmt.Errorf("opportunity %q not found", requested)
		}
		if node.Type != TypeOpportunity {
			return nil, fmt.Errorf("node %q is %s, not OPPORTUNITY", requested, node.Type)
		}
		if node.Status != StatusValidated {
			return nil, fmt.Errorf("opportunity %q must be validated before compile", requested)
		}
		return []*Node{node}, nil
	}

	if len(tree.SelectedOpportunityIDs) > 0 {
		var selected []*Node
		for _, id := range tree.SelectedOpportunityIDs {
			node, ok := tree.Nodes[id]
			if !ok {
				return nil, fmt.Errorf("selected opportunity %q not found", id)
			}
			if node.Type != TypeOpportunity {
				return nil, fmt.Errorf("selected node %q is %s, not OPPORTUNITY", id, node.Type)
			}
			if node.Status != StatusValidated {
				return nil, fmt.Errorf("selected opportunity %q must be validated before compile", id)
			}
			selected = append(selected, node)
		}
		sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
		return selected, nil
	}

	var nodes []*Node
	for _, node := range tree.Nodes {
		if node.Type == TypeOpportunity && node.Status == StatusValidated {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no validated opportunities found")
	}
	return nodes, nil
}

func collectLineage(tree *EchoTree, start *Node, spec *AlfaSpec, seen *seenSet) {
	node := start
	for node != nil {
		for _, p := range node.Perceptions {
			if seen.add("perception:" + p) {
				spec.Perceptions = append(spec.Perceptions, p)
			}
		}

		ref := nodeRef(node)
		switch node.Type {
		case TypePain:
			if seen.add("pain:" + node.ID) {
				spec.ConfirmedPains = append(spec.ConfirmedPains, ref)
			}
		case TypeTask:
			if seen.add("task:" + node.ID) {
				spec.SupportingTasks = append(spec.SupportingTasks, ref)
			}
		case TypeTheory:
			if seen.add("theory:" + node.ID) {
				spec.SupportingTheories = append(spec.SupportingTheories, ref)
			}
		case TypeAxiom:
			if seen.add("axiom:" + node.ID) {
				spec.SupportingAxioms = append(spec.SupportingAxioms, ref)
			}
		}

		if node.ParentID == "" {
			break
		}
		node = tree.Nodes[node.ParentID]
	}
}

func nodeRef(node *Node) NodeRef {
	return NodeRef{
		ID:               node.ID,
		Type:             node.Type,
		Title:            node.Title,
		Evidence:         node.Evidence,
		ValidationAnswer: node.ValidationAnswer,
		Status:           node.Status,
	}
}

func buildIntent(spec *AlfaSpec) string {
	var parts []string
	for _, op := range spec.SelectedOpportunities {
		parts = append(parts, op.Title)
	}
	if len(parts) == 0 {
		return "Automatización derivada del árbol Echo"
	}
	return "Implementar " + strings.Join(parts, " + ") + " para resolver dolores confirmados en Echo"
}

func buildIdealSteps(spec *AlfaSpec) []IdealStep {
	steps := []IdealStep{
		{
			ID:          "step_001",
			Name:        "Cargar contexto confirmado",
			Description: "Cargar las fuentes, datos y restricciones confirmadas en Echo antes de tomar decisiones.",
			Outputs:     []string{"contexto_operacional"},
			SourceNodes: idsOf(spec.SupportingAxioms),
		},
		{
			ID:          "step_002",
			Name:        "Evaluar dolores confirmados",
			Description: "Identificar qué PAINS validados debe resolver la automatización y no optimizar problemas no confirmados.",
			Inputs:      []string{"contexto_operacional"},
			Outputs:     []string{"dolores_a_resolver"},
			SourceNodes: idsOf(spec.ConfirmedPains),
		},
	}

	for idx, op := range spec.SelectedOpportunities {
		stepID := fmt.Sprintf("step_%03d", idx+3)
		steps = append(steps, IdealStep{
			ID:          stepID,
			Name:        op.Title,
			Description: describeOpportunityStep(op),
			Inputs:      []string{"contexto_operacional", "dolores_a_resolver"},
			Outputs:     opportunityOutputs(op),
			SourceNodes: []string{op.ID, op.ParentPainID},
		})
	}

	steps = append(steps, IdealStep{
		ID:          fmt.Sprintf("step_%03d", len(steps)+1),
		Name:        "Verificar criterios de éxito",
		Description: "Comprobar que cada salida reduce el dolor confirmado y no obliga al usuario a adaptarse a un flujo no validado.",
		Inputs:      []string{"salidas_de_automatizacion"},
		Outputs:     []string{"resultado_verificado"},
	})

	return steps
}

func describeOpportunityStep(op OpportunitySpec) string {
	text := strings.ToLower(op.Title + " " + strings.Join(op.Evidence, " "))
	switch {
	case strings.Contains(text, "prioriz"):
		return "Generar una priorización operativa basada en criterios validados, no solo en el orden actual o fecha de vencimiento."
	case strings.Contains(text, "resumen") || strings.Contains(text, "reporte"):
		return "Generar automáticamente el reporte o resumen esperado con los datos necesarios para tomar decisiones de gestión."
	default:
		return "Ejecutar la oportunidad validada contra el dolor confirmado, manteniendo el flujo lo más cercano posible a la operación actual."
	}
}

func opportunityOutputs(op OpportunitySpec) []string {
	text := strings.ToLower(op.Title)
	switch {
	case strings.Contains(text, "dashboard"):
		return []string{"dashboard_operativo", "lista_priorizada"}
	case strings.Contains(text, "resumen"):
		return []string{"resumen_generado"}
	case strings.Contains(text, "reporte"):
		return []string{"reporte_generado"}
	default:
		return []string{"salida_de_automatizacion"}
	}
}

func buildBusinessRules(spec *AlfaSpec) []BusinessRule {
	var rules []BusinessRule
	rules = append(rules, BusinessRule{
		ID:          "rule_001",
		Name:        "Resolver solo dolores confirmados",
		Description: "La automatización debe resolver PAINS validados en Echo y no preferencias superficiales.",
		Then:        "Cada salida debe mapear a un PAIN confirmado o quedar marcada como no justificada.",
		Importance:  1,
		SourceNodes: idsOf(spec.ConfirmedPains),
	})
	rules = append(rules, BusinessRule{
		ID:          "rule_002",
		Name:        "No adaptar al usuario a una solución no validada",
		Description: "La automatización debe encajar con el trabajo actual y las percepciones de Echo.",
		Then:        "Si una decisión fuerza un cambio operativo no validado, debe registrarse como gap.",
		Importance:  1,
	})

	next := 3
	for _, th := range spec.SupportingTheories {
		rules = append(rules, BusinessRule{
			ID:          fmt.Sprintf("rule_%03d", next),
			Name:        th.Title,
			Description: firstNonEmpty(th.ValidationAnswer, strings.Join(th.Evidence, "; ")),
			Then:        "El flujo debe respetar esta teoría validada.",
			Importance:  1,
			SourceNodes: []string{th.ID},
		})
		next++
	}

	for _, op := range spec.SelectedOpportunities {
		rules = append(rules, BusinessRule{
			ID:          fmt.Sprintf("rule_%03d", next),
			Name:        "Oportunidad validada: " + op.Title,
			Description: firstNonEmpty(op.ValidationAnswer, strings.Join(op.Evidence, "; ")),
			When:        "cuando se ejecute la automatización candidata",
			Then:        "debe producir una salida que resuelva el PAIN parent " + op.ParentPainID,
			Importance:  1,
			SourceNodes: []string{op.ID, op.ParentPainID},
		})
		next++
	}

	return rules
}

func buildCriticalVariables(spec *AlfaSpec) []string {
	seen := map[string]bool{}
	var vars []string
	add := func(v string) {
		if !seen[v] {
			seen[v] = true
			vars = append(vars, v)
		}
	}
	add("pain_resolved")
	add("source_data_loaded")
	add("output_generated")

	allText := strings.ToLower(joinSpecText(spec))
	if strings.Contains(allText, "excel") {
		add("excel_files_loaded")
	}
	if strings.Contains(allText, "riesgo") {
		add("risk_score")
		add("risk_factors")
	}
	if strings.Contains(allText, "prioriz") {
		add("priority_order")
	}
	if strings.Contains(allText, "reporte") || strings.Contains(allText, "resumen") {
		add("report_period")
		add("summary_metrics")
	}
	if strings.Contains(allText, "whatsapp") || strings.Contains(allText, "llamada") || strings.Contains(allText, "mail") {
		add("contact_channel")
	}
	return vars
}

func buildSuccessCriteria(spec *AlfaSpec) []string {
	criteria := []string{
		"Cada OPPORTUNITY validada genera una salida verificable.",
		"Cada salida puede trazarse a un PAIN validado.",
		"Las decisiones críticas quedan registradas para Bravo con variables y razones.",
	}
	for _, pain := range spec.ConfirmedPains {
		criteria = append(criteria, "Reduce o elimina el dolor confirmado: "+pain.Title)
	}
	return criteria
}

func buildEdgeCases(spec *AlfaSpec) []string {
	var cases []string
	text := strings.ToLower(joinSpecText(spec))
	if strings.Contains(text, "excel") {
		cases = append(cases, "Alguno de los archivos Excel esperados falta, está vacío o tiene columnas incompatibles.")
	}
	if strings.Contains(text, "riesgo") || strings.Contains(text, "histórico") {
		cases = append(cases, "No existe historial suficiente para calcular comportamiento de pago.")
	}
	if strings.Contains(text, "whatsapp") || strings.Contains(text, "llamada") {
		cases = append(cases, "El canal de contacto recomendado no está disponible o no corresponde al cliente priorizado.")
	}
	return cases
}

func buildOpenQuestions(tree *EchoTree, spec *AlfaSpec) []OpenQuestion {
	var questions []OpenQuestion
	next := func(reason, q, needed string, nodes ...string) {
		questions = append(questions, OpenQuestion{
			ID:              fmt.Sprintf("oq_%03d", len(questions)+1),
			Reason:          reason,
			QuestionForEcho: q,
			NeededFor:       needed,
			SourceNodes:     nodes,
		})
	}

	for _, node := range tree.Nodes {
		if node.Status != StatusValidated && hasValidatedDescendant(tree, node.ID) {
			next(
				"Hay nodos validados colgando de un nodo no validado.",
				fmt.Sprintf("Antes de usar '%s' como base, ¿puedes confirmar o reformular esta tarea?", node.Title),
				"corregir consistencia del árbol Echo antes de compilar flujo ideal",
				node.ID,
			)
		}
	}

	text := strings.ToLower(joinSpecText(spec))
	if strings.Contains(text, "riesgo") && !containsAny(text, "peso", "ponder", "formula", "fórmula") {
		next(
			"No está definida la fórmula o ponderación de riesgo.",
			"Cuando dices riesgo de no pago, ¿qué señales pesan más: antigüedad, monto, comportamiento histórico u otra cosa?",
			"regla de priorización verificable en Bravo",
		)
	}
	if strings.Contains(text, "resumen semanal") && !containsAny(text, "kpi", "métrica", "metrica", "columna", "campo") {
		next(
			"No está claro qué debe contener el resumen semanal.",
			"Cuando dices resumen semanal, ¿qué decisión debería poder tomar gerencia al verlo?",
			"estructura del output de resumen semanal",
		)
	}
	if strings.Contains(text, "dashboard") && !containsAny(text, "cobrador", "gerencia", "usuario final", "quien lo usa") {
		next(
			"No está claro quién consume el dashboard y qué acción toma.",
			"¿Quién usa el dashboard diario y qué acción concreta debe tomar después de verlo?",
			"flujo operativo posterior al dashboard",
		)
	}
	if needsDataTransport(spec) && !hasDataTransportConfirmed(text) {
		next(
			"No está confirmado cómo entran los datos actuales a la automatización.",
			"¿Dónde vive hoy la información necesaria y cuál es el camino mínimo realista para llevarla a la automatización sin copiarla uno por uno?",
			"input verificable para Bravo y primera versión local-first",
		)
	}
	if needsOperationalViability(spec) && !hasOperationalViabilityConfirmed(text) {
		next(
			"No está confirmado que el usuario tolere el hábito operativo que requiere la automatización.",
			"Si esta solución requiere registrar información manualmente, ¿en qué momento real lo haría el usuario y qué esfuerzo máximo acepta sin romper su flujo?",
			"evitar que Bravo construya una solución que obligue al usuario a adaptarse a un hábito no validado",
		)
	}

	return questions
}

func needsDataTransport(spec *AlfaSpec) bool {
	return len(spec.SelectedOpportunities) > 0
}

func hasDataTransportConfirmed(text string) bool {
	if !containsAny(text,
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
	) {
		return false
	}

	if containsAny(text, "uno por uno", "manualmente uno por uno", "copiar uno a uno") {
		return false
	}

	return true
}

func needsOperationalViability(spec *AlfaSpec) bool {
	text := strings.ToLower(joinSpecText(spec))
	return containsAny(text,
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

func hasOperationalViabilityConfirmed(text string) bool {
	hasMoment := containsAny(text,
		"apenas corto",
		"apenas corta",
		"apenas termina",
		"después de cada llamada",
		"despues de cada llamada",
		"después de hablar",
		"despues de hablar",
		"al terminar",
		"al final de la llamada",
		"momento de captura",
		"en el momento",
		"durante la llamada",
	)
	hasTolerance := containsAny(text,
		"rápido",
		"rapido",
		"segundos",
		"10-20 segundos",
		"nota corta",
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

func hasValidatedDescendant(tree *EchoTree, nodeID string) bool {
	for _, node := range tree.Nodes {
		if node.ParentID == nodeID && node.Status == StatusValidated {
			return true
		}
	}
	return false
}

func idsOf(refs []NodeRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.ID)
	}
	return out
}

func joinSpecText(spec *AlfaSpec) string {
	var b strings.Builder
	b.WriteString(spec.AutomationIntent)
	for _, op := range spec.SelectedOpportunities {
		b.WriteString(" ")
		b.WriteString(op.Title)
		b.WriteString(" ")
		b.WriteString(strings.Join(op.Evidence, " "))
		b.WriteString(" ")
		b.WriteString(op.ValidationAnswer)
	}
	for _, lists := range [][]NodeRef{spec.ConfirmedPains, spec.SupportingTasks, spec.SupportingTheories, spec.SupportingAxioms} {
		for _, ref := range lists {
			b.WriteString(" ")
			b.WriteString(ref.Title)
			b.WriteString(" ")
			b.WriteString(strings.Join(ref.Evidence, " "))
			b.WriteString(" ")
			b.WriteString(ref.ValidationAnswer)
		}
	}
	for _, p := range spec.Perceptions {
		b.WriteString(" ")
		b.WriteString(p)
	}
	return b.String()
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type seenSet struct {
	values map[string]bool
}

func newSeenSet() *seenSet {
	return &seenSet{values: map[string]bool{}}
}

func (s *seenSet) add(key string) bool {
	if s.values[key] {
		return false
	}
	s.values[key] = true
	return true
}

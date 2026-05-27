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

	opportunities, err := selectOpportunities(tree, opts.Opportunity, opts.AllowDraft)
	if err != nil {
		return nil, err
	}

	spec := &AlfaSpec{
		Version:               "0.1",
		Generated:             opts.GeneratedNow.Format(time.RFC3339),
		SourceTree:            opts.EchoTreePath,
		ProjectID:             tree.ProjectID,
		ClientName:            tree.ClientName,
		ConversationSignals:   tree.Signals,
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
	spec.DataModel = buildDataModelSpec(spec)
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

func selectOpportunities(tree *EchoTree, requested string, allowDraft bool) ([]*Node, error) {
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
		if allowDraft {
			drafts := draftOpportunitiesFromPains(tree)
			if len(drafts) > 0 {
				return drafts, nil
			}
		}
		return nil, fmt.Errorf("no validated opportunities found; %s", summarizeTreeForOperator(tree))
	}
	return nodes, nil
}

func summarizeTreeForOperator(tree *EchoTree) string {
	counts := map[string]map[string]int{}
	for _, node := range tree.Nodes {
		if counts[node.Type] == nil {
			counts[node.Type] = map[string]int{}
		}
		counts[node.Type][node.Status]++
	}

	order := []string{TypeAxiom, TypeTheory, TypeTask, TypePain, TypeOpportunity}
	var parts []string
	for _, t := range order {
		if statuses, ok := counts[t]; ok {
			var entries []string
			for status, n := range statuses {
				entries = append(entries, fmt.Sprintf("%d %s", n, status))
			}
			sort.Strings(entries)
			parts = append(parts, fmt.Sprintf("%s: %s", t, strings.Join(entries, ", ")))
		}
	}

	if len(parts) == 0 {
		return "el árbol Echo está vacío. Echo debe capturar al menos un AXIOM antes de compilar"
	}

	missing := []string{}
	for _, t := range []string{TypeOpportunity, TypePain, TypeTask} {
		if _, ok := counts[t]; !ok {
			missing = append(missing, t)
		}
	}

	hint := "Echo debe avanzar hasta OPPORTUNITY (o PAIN validado si usas --allow-draft) para que Alfa pueda compilar"
	if len(missing) > 0 {
		hint = fmt.Sprintf("falta avanzar hasta %s. %s", strings.Join(missing, "/"), hint)
	}

	return fmt.Sprintf("estado actual del árbol — %s. %s", strings.Join(parts, "; "), hint)
}

func draftOpportunitiesFromPains(tree *EchoTree) []*Node {
	var pains []*Node
	for _, node := range tree.Nodes {
		if node.Type == TypePain && node.Status == StatusValidated {
			pains = append(pains, node)
		}
	}
	sort.Slice(pains, func(i, j int) bool { return pains[i].ID < pains[j].ID })

	drafts := make([]*Node, 0, len(pains))
	for idx, pain := range pains {
		taskTitle := "la tarea repetitiva confirmada"
		if task := tree.Nodes[pain.ParentID]; task != nil && task.Type == TypeTask {
			taskTitle = task.Title
		}
		drafts = append(drafts, &Node{
			ID:    fmt.Sprintf("draft_op_%03d", idx+1),
			Layer: 4,
			Type:  TypeOpportunity,
			Title: "Draft temprano: automatizar " + taskTitle,
			Evidence: []string{
				"Generado por Alfa como primera hipótesis desde PAIN/TASK; debe volver a Echo para validar si calza.",
				"Dolor base: " + pain.Title,
			},
			Status:           "DRAFT",
			ParentID:         pain.ID,
			ValidationAnswer: "No validado por usuario; usar solo para idear primera iteración y preguntas bloqueantes.",
		})
	}
	return drafts
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

func buildDataModelSpec(spec *AlfaSpec) DataModelSpec {
	text := strings.ToLower(joinSpecText(spec))
	model := DataModelSpec{
		CurrentState: DataModelState{
			Description: "Modelo observado desde Echo. Si faltan campos, relaciones o reglas, Alfa debe declararlo como gap y no normalizar inventando.",
			Entities:    observedEntities(text),
		},
		NormalizedTarget: DataModelState{
			Description: "MERE normalizado propuesto para que la automatización preserve la información original sin copiar el desorden operativo.",
		},
	}
	model.NormalizedTarget.Entities = normalizedEntities(text)
	model.NormalizedTarget.Relationships = normalizedRelationships(text)
	model.BusinessRules = dataModelBusinessRules(text)
	model.OpenGaps = dataModelGaps(text)
	return model
}

func observedEntities(text string) []DataEntity {
	var entities []DataEntity
	if containsAny(text, "whatsapp", "captura", "pantallazo", "imagen", "foto", "comprobante", "correo", "pdf", "archivo", "papel", "mensaje") {
		entities = append(entities, DataEntity{
			Name:        "artefacto_actual",
			Description: "Recurso real donde vive información hoy: mensaje, imagen, archivo, documento, correo, papel u otro soporte observado.",
			Fields:      []string{"tipo", "origen", "fecha_recepcion", "contenido_visible", "contexto_si_existe"},
			Source:      "Echo",
		})
	}
	if containsAny(text, "excel", "planilla", "xlsx", "csv", "tabla", "sheet", "base", "sistema", "crm", "erp") {
		entities = append(entities, DataEntity{
			Name:        "registro_actual",
			Description: "Estructura actual donde se registra o consulta información. Requiere muestra real para no inventar campos.",
			Fields:      []string{"campos_actuales", "formato", "frecuencia_actualizacion", "responsable"},
			Source:      "Echo",
		})
	}
	if containsAny(text, "cliente", "proveedor", "usuario", "paciente", "alumno", "empleado", "vendedor", "operador", "responsable", "equipo", "persona") {
		entities = append(entities, DataEntity{
			Name:        "actor_actual",
			Description: "Persona, organización, sistema o rol mencionado en la operación actual.",
			Fields:      []string{"nombre_o_alias", "rol", "canal", "identificador_si_existe"},
			Source:      "Echo",
		})
	}
	if containsAny(text, "solicitud", "pedido", "orden", "caso", "ticket", "venta", "compra", "entrega", "reserva", "cita", "tarea", "proyecto", "servicio", "producto", "documento", "factura", "pago", "transferencia") {
		entities = append(entities, DataEntity{
			Name:        "objeto_operativo_actual",
			Description: "Objeto, evento o caso que el negocio intenta seguir, resolver, controlar o relacionar hoy.",
			Fields:      []string{"nombre", "fecha", "estado_si_existe", "atributos_visibles", "identificador_si_existe"},
			Source:      "Echo",
		})
	}
	return entities
}

func normalizedEntities(text string) []DataEntity {
	entities := []DataEntity{
		{
			Name:        "actor",
			Description: "Persona, organización, área o sistema que participa en el proceso con un rol definido.",
			Fields:      []string{"id", "nombre", "tipo", "rol"},
		},
		{
			Name:        "entidad_negocio",
			Description: "Cosa principal que el negocio necesita registrar, seguir, clasificar o decidir. Alfa no debe fijar su nombre de dominio sin evidencia de Echo.",
			Fields:      []string{"id", "tipo", "nombre_o_referencia", "estado_actual", "fecha_creacion"},
		},
		{
			Name:        "evento_operativo",
			Description: "Hecho que ocurre sobre una entidad de negocio y cambia su estado, historial, monto, prioridad, responsable o decisión.",
			Fields:      []string{"id", "entidad_negocio_id", "tipo", "fecha", "actor_id", "detalle"},
		},
	}
	if relationshipLikely(text) {
		entities = append(entities, DataEntity{
			Name:        "relacion_normalizada",
			Description: "Entidad asociativa genérica para representar cruces entre dos o más elementos cuando la cardinalidad o asignación no está confirmada.",
			Fields:      []string{"id", "origen_id", "destino_id", "tipo_relacion", "valor_o_peso_si_aplica", "criterio_confirmado"},
		})
	}
	if evidenceLikely(text) {
		entities = append(entities, DataEntity{
			Name:        "evidencia",
			Description: "Artefacto original que respalda un dato estructurado y permite auditar de dónde salió.",
			Fields:      []string{"id", "tipo", "origen", "fecha_recepcion", "uri", "texto_extraido", "contexto_confirmado"},
		})
	}
	if containsAny(text, "estado", "etapa", "prioridad", "aprobado", "rechazado", "pendiente", "cerrado", "historial", "seguimiento") {
		entities = append(entities, DataEntity{
			Name:        "estado_historial",
			Description: "Historial de estados o etapas de una entidad de negocio cuando el proceso depende de seguimiento temporal.",
			Fields:      []string{"id", "entidad_negocio_id", "estado", "fecha_inicio", "fecha_fin", "actor_id"},
		})
	}
	return entities
}

func normalizedRelationships(text string) []DataRelationship {
	relationships := []DataRelationship{
		{
			From:        "evento_operativo",
			To:          "entidad_negocio",
			Type:        "many_to_one_unless_echo_confirms_otherwise",
			Description: "Un evento suele ocurrir sobre una entidad, pero Alfa debe marcar gap si el proceso permite varios elementos por evento o varios eventos por elemento.",
		},
		{
			From:        "actor",
			To:          "evento_operativo",
			Type:        "role_based",
			Description: "Un actor participa en un evento con un rol confirmado por Echo.",
		},
	}
	if relationshipLikely(text) {
		relationships = append(relationships, DataRelationship{
			From:        "relacion_normalizada",
			To:          "entidad_negocio/evento_operativo",
			Type:        "cardinality_unconfirmed",
			Description: "Cuando el negocio necesita cruzar, calzar o asociar elementos, la cardinalidad queda abierta hasta que Echo confirme la regla.",
		})
	}
	if evidenceLikely(text) {
		relationships = append(relationships, DataRelationship{
			From:        "evidencia",
			To:          "entidad_negocio/evento_operativo/relacion_normalizada",
			Type:        "audit_link_or_unresolved",
			Description: "La evidencia debe conservarse y vincularse al dato estructurado; si no existe contexto suficiente, la relación queda como gap.",
		})
	}
	if containsAny(text, "estado", "etapa", "prioridad", "aprobado", "rechazado", "pendiente", "cerrado", "historial", "seguimiento") {
		relationships = append(relationships, DataRelationship{
			From:        "estado_historial",
			To:          "entidad_negocio",
			Type:        "many_to_one",
			Description: "Una entidad puede tener muchos estados históricos si el proceso requiere trazabilidad temporal.",
		})
	}
	return relationships
}

func dataModelBusinessRules(text string) []BusinessRule {
	rules := []BusinessRule{
		{
			ID:          "data_rule_001",
			Name:        "No inventar entidades de dominio",
			Description: "Alfa puede proponer estructura MERE genérica, pero no debe convertirla en nombres, campos o reglas del negocio sin evidencia de Echo.",
			Then:        "Si falta la regla, Alfa debe crear un gap para Echo en vez de fijar una cardinalidad o campo específico.",
			Importance:  1,
		},
		{
			ID:          "data_rule_002",
			Name:        "No asumir cardinalidad",
			Description: "Cuando el flujo relaciona elementos, Alfa no puede asumir relación 1 a 1, 1 a muchos o muchos a muchos sin confirmación.",
			Then:        "El MERE debe usar relacion_normalizada o bloquear export_ready con una pregunta de cardinalidad.",
			Importance:  1,
		},
	}
	if evidenceLikely(text) {
		rules = append(rules, BusinessRule{
			ID:          "data_rule_003",
			Name:        "Preservar evidencia original",
			Description: "La automatización debe conservar vínculo entre dato normalizado y recurso original para evitar alucinación o pérdida de auditoría.",
			Then:        "Cada dato extraído desde recursos no estructurados debe referenciar evidencia_id o quedar como dato no verificado.",
			Importance:  1,
		})
	}
	return rules
}

func dataModelGaps(text string) []DataModelGap {
	var gaps []DataModelGap
	next := func(reason, question, needed string) {
		gaps = append(gaps, DataModelGap{
			ID:              fmt.Sprintf("dm_gap_%03d", len(gaps)+1),
			Reason:          reason,
			QuestionForEcho: question,
			NeededFor:       needed,
		})
	}
	if relationshipLikely(text) && !cardinalityConfirmed(text) {
		next(
			"No está clara la cardinalidad entre elementos que deben relacionarse.",
			"Cuando relacionan esos elementos, ¿la relación es siempre 1 a 1, puede ser 1 a muchos, muchos a muchos, parcial o con excepciones?",
			"definir relaciones del MERE sin asumir reglas de negocio",
		)
	}
	if identifierGapLikely(text) && !containsAny(text, "identificador", "codigo", "código", "numero unico", "número único", "rut", "uuid", "folio", "sku", "nombre y contacto", "correo contiene") {
		next(
			"No está claro qué identificador evita duplicados entre entidades relevantes.",
			"¿Qué dato permite reconocer de forma única cada entidad principal: código, número, nombre exacto, correo, cuenta, folio u otro identificador?",
			"evitar duplicados y relacionar registros de forma confiable",
		)
	}
	if evidenceLikely(text) && relationshipLikely(text) && !containsAny(text, "mensaje corto", "contexto confirmado", "contexto", "referencia", "identificador") {
		next(
			"No está claro dónde vive el contexto que relaciona una evidencia con la entidad correcta.",
			"Cuando llega un recurso como captura, archivo o mensaje, ¿qué dato permite saber a qué entidad o evento corresponde: texto alrededor, nombre visible, registro externo, memoria de alguien o un mensaje corto que podrían agregar?",
			"vincular evidencia original con entidades normalizadas sin inventar relaciones",
		)
	}
	return gaps
}

func evidenceLikely(text string) bool {
	return containsAny(text, "whatsapp", "captura", "pantallazo", "imagen", "foto", "comprobante", "correo", "pdf", "archivo", "papel", "mensaje")
}

func relationshipLikely(text string) bool {
	return containsAny(text, "cruzar", "cruce", "relacionar", "relación", "relacion", "asociar", "calzar", "conciliar", "vincular", "unir", "matching", "corresponde", "correspondencia", "comparar", "mapear")
}

func dataStructureLikely(text string) bool {
	return containsAny(text, "dato", "registro", "tabla", "excel", "planilla", "base", "sistema", "crm", "erp", "archivo", "documento", "formulario", "seguimiento") ||
		evidenceLikely(text) || relationshipLikely(text)
}

func identifierGapLikely(text string) bool {
	return relationshipLikely(text) || containsAny(text, "duplicado", "duplicados", "deduplicar", "repetido", "mismo", "misma", "varios sistemas", "varias fuentes", "fuentes distintas")
}

func cardinalityConfirmed(text string) bool {
	return containsAny(text, "1 a 1", "uno a uno", "1:1", "1 a muchos", "uno a muchos", "muchos a muchos", "n:m", "parcial", "varios", "varias", "múltiples", "multiples", "siempre uno", "solo uno")
}

func buildBusinessRules(spec *AlfaSpec) []BusinessRule {
	var rules []BusinessRule
	rules = append(rules, spec.DataModel.BusinessRules...)
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
	for _, gap := range spec.DataModel.OpenGaps {
		next(gap.Reason, gap.QuestionForEcho, gap.NeededFor)
	}
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
			dataTransportQuestionForEcho(text),
			"input verificable para Bravo y primera versión conectable por API, archivo o recurso real",
		)
	}
	if needsOperationalViability(spec) && !hasOperationalViabilityConfirmed(text) {
		if hasConversationFatigue(spec) {
			next(
				"Riesgo no resuelto: la captura manual requerida no fue validada y el usuario ya mostró fatiga conversacional.",
				"No vuelvas a preguntar esto como una pregunta abierta al cliente. Compila solo un draft/prototipo con el riesgo manual_capture_viability_unconfirmed explícito.",
				"prototipo para validar si el registro manual se sostiene sin fricción",
			)
		} else {
			next(
				"No está confirmado que el usuario tolere el hábito operativo que requiere la automatización.",
				operationalViabilityQuestionForEcho(text),
				"evitar que Bravo construya una solución que obligue al usuario a adaptarse a un hábito no validado",
			)
		}
	}

	return questions
}

func dataTransportQuestionForEcho(text string) string {
	if containsAny(text, "whatsapp", "transferencia", "factura", "comprobante", "captura", "pantallazo", "foto") {
		return "Para la primera iteración, necesito ver la estructura real: ¿puedes pedir una captura anonimizada de una transferencia/factura en WhatsApp, incluyendo los mensajes que dan contexto, y confirmar si existe API/permiso para leer ese origen o si partimos con export/archivo?"
	}
	if containsAny(text, "excel", "planilla", "xlsx", "csv") {
		return "Para la primera iteración, ¿puedes pedir una plantilla o foto anonimizada del Excel/planilla actual, con encabezados visibles, y confirmar si se puede acceder por archivo exportado o API?"
	}
	return "¿Dónde vive hoy la información necesaria, puedes pedir un recurso real de ejemplo anonimizado, y cuál es el camino inicial para obtenerla: API confirmada, archivo exportado o carga mínima?"
}

func operationalViabilityQuestionForEcho(text string) string {
	if containsAny(text, "whatsapp", "transferencia", "factura", "comprobante", "captura", "pantallazo") {
		return "Alfa puede idear el cruce, pero falta el contexto que une cada dato: ¿ese contexto vive en mensajes, Excel, factura o memoria? Si no existe, ¿la persona puede comprometerse a agregar un mensaje corto después del pantallazo?"
	}
	return "Si esta solución requiere registrar información manualmente, ¿en qué momento real lo haría el usuario, qué dato mínimo agregaría y qué esfuerzo máximo acepta sin romper su flujo?"
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
	for _, signal := range spec.ConversationSignals {
		b.WriteString(" ")
		b.WriteString(signal.Type)
		b.WriteString(" ")
		b.WriteString(signal.Note)
	}
	return b.String()
}

func hasConversationFatigue(spec *AlfaSpec) bool {
	for _, signal := range spec.ConversationSignals {
		signalType := strings.ToLower(signal.Type)
		note := strings.ToLower(signal.Note)
		if signalType == "fatigue" || signalType == "low_attention" {
			return true
		}
		if containsAny(note,
			"muchas preguntas",
			"preguntando muchas cosas",
			"no te entiendo",
			"no entiendo",
			"qué se yo",
			"que se yo",
		) {
			return true
		}
	}
	return false
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

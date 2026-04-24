package tree

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// FrameworkEcho es la estructura raíz del árbol de conocimiento
type FrameworkEcho struct {
	ProjectID              string           `json:"project_id"`
	ClientName             string           `json:"client_name"`
	DateStarted            string           `json:"date_started"`
	CurrentMaxLayer        int              `json:"current_max_layer"`
	FocusNodes             []string         `json:"focus_nodes"`
	SelectedOpportunityIDs []string         `json:"selected_opportunity_ids,omitempty"`
	Config                 Config           `json:"config"`
	QALog                  []QALogEntry     `json:"qa_log,omitempty"`
	Nodes                  map[string]*Node `json:"nodes"`
	FilePath               string           `json:"-"`
}

type Config struct {
	QALogEnabled bool `json:"qa_log_enabled"`
}

type QALogEntry struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Purpose   string `json:"purpose,omitempty"`
	CreatedAt string `json:"created_at"`
}

// LoadOrCreate carga el árbol desde un archivo o crea uno nuevo
func LoadOrCreate(filePath string) (*FrameworkEcho, error) {
	tree := &FrameworkEcho{
		FilePath: filePath,
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Crear nuevo
			tree.ProjectID = "project-001"
			tree.ClientName = ""
			tree.DateStarted = ""
			tree.CurrentMaxLayer = 0
			tree.FocusNodes = []string{}
			tree.SelectedOpportunityIDs = []string{}
			tree.Config = Config{}
			tree.QALog = []QALogEntry{}
			tree.Nodes = make(map[string]*Node)
			return tree, nil
		}
		return nil, fmt.Errorf("error leyendo %s: %w", filePath, err)
	}

	if err := json.Unmarshal(data, tree); err != nil {
		return nil, fmt.Errorf("error parseando JSON: %w", err)
	}

	if tree.Nodes == nil {
		tree.Nodes = make(map[string]*Node)
	}
	if tree.FocusNodes == nil {
		tree.FocusNodes = []string{}
	}
	if tree.SelectedOpportunityIDs == nil {
		tree.SelectedOpportunityIDs = []string{}
	}
	if tree.QALog == nil {
		tree.QALog = []QALogEntry{}
	}

	tree.FilePath = filePath
	return tree, nil
}

// Save guarda el árbol al archivo JSON
func (t *FrameworkEcho) Save() error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando JSON: %w", err)
	}

	if err := os.WriteFile(t.FilePath, data, 0644); err != nil {
		return fmt.Errorf("error escribiendo archivo: %w", err)
	}

	return nil
}

// Init inicializa un proyecto nuevo
func (t *FrameworkEcho) Init(projectID, clientName, date string) error {
	t.ProjectID = projectID
	t.ClientName = clientName
	t.DateStarted = date
	t.CurrentMaxLayer = 0
	t.FocusNodes = []string{}
	t.SelectedOpportunityIDs = []string{}
	t.Config = Config{}
	t.QALog = []QALogEntry{}
	t.Nodes = make(map[string]*Node)
	return t.Save()
}

// nextSeqNum calcula el siguiente número de secuencia para un tipo de nodo
func (t *FrameworkEcho) nextSeqNum(nodeType string) int {
	prefix := prefixMap[nodeType]
	max := 0
	for id := range t.Nodes {
		if strings.HasPrefix(id, prefix+"_") {
			numStr := strings.TrimPrefix(id, prefix+"_")
			num := 0
			fmt.Sscanf(numStr, "%d", &num)
			if num > max {
				max = num
			}
		}
	}
	return max + 1
}

// CountValidatedInLayer cuenta cuántos nodos validados hay en una capa
func (t *FrameworkEcho) CountValidatedInLayer(layer int) int {
	count := 0
	for _, node := range t.Nodes {
		if node.Layer == layer && node.Status == StatusValidated {
			count++
		}
	}
	return count
}

// countInLayer cuenta cuántos nodos hay en total en una capa
func (t *FrameworkEcho) countInLayer(layer int) int {
	count := 0
	for _, node := range t.Nodes {
		if node.Layer == layer {
			count++
		}
	}
	return count
}

// AddNode agrega un nodo al árbol con todas las validaciones
func (t *FrameworkEcho) AddNode(nodeType string, title string, evidence []string, parentID string) (*Node, error) {
	targetLayer, ok := layerMap[nodeType]
	if !ok {
		return nil, fmt.Errorf("tipo de nodo inválido: %s", nodeType)
	}

	// Validar que no se salten capas
	if targetLayer > 0 {
		prevLayer := targetLayer - 1
		minRequired := minValidatedPrevLayer[targetLayer]
		validated := t.CountValidatedInLayer(prevLayer)

		if validated < minRequired {
			return nil, fmt.Errorf(
				"no puedes crear %s (layer %d): necesitas mínimo %d nodos validados en layer %d, tienes %d",
				nodeType, targetLayer, minRequired, prevLayer, validated,
			)
		}
	}

	// Validar que el parent exista (si se especificó)
	if parentID != "" {
		parent, exists := t.Nodes[parentID]
		if !exists {
			return nil, fmt.Errorf("parent '%s' no existe", parentID)
		}
		if nodeType != TypeAxiom && parent.Status != StatusValidated {
			return nil, fmt.Errorf("parent '%s' debe estar validado antes de crear %s", parentID, nodeType)
		}
		if nodeType == TypeOpportunity && parent.Type != TypePain {
			return nil, fmt.Errorf("las opportunities deben colgar de un PAIN validado")
		}
		// El parent debe ser de una capa inferior
		if parent.Layer >= targetLayer {
			return nil, fmt.Errorf(
				"parent '%s' está en layer %d, pero el nuevo nodo %s es layer %d (el parent debe ser de capa inferior)",
				parentID, parent.Layer, nodeType, targetLayer,
			)
		}
	}

	// Validar que parent sea obligatorio para nodos que no son axiomas
	if nodeType != TypeAxiom && parentID == "" {
		return nil, fmt.Errorf("los nodos de tipo %s requieren un parent_id", nodeType)
	}

	seqNum := t.nextSeqNum(nodeType)

	node, err := NewNode(nodeType, title, evidence, parentID, seqNum)
	if err != nil {
		return nil, err
	}

	// Generar preguntas automáticas
	node.QuestionsToAsk = GenerateQuestions(node)

	// Agregar al árbol
	t.Nodes[node.ID] = node

	// Actualizar parent
	if parentID != "" {
		if parent, exists := t.Nodes[parentID]; exists {
			parent.AddChild(node.ID)
		}
	}

	// Actualizar max layer
	if targetLayer > t.CurrentMaxLayer {
		t.CurrentMaxLayer = targetLayer
	}

	// Guardar
	if err := t.Save(); err != nil {
		return nil, fmt.Errorf("error guardando: %w", err)
	}

	return node, nil
}

// ValidateNode valida un nodo con la respuesta del cliente
func (t *FrameworkEcho) ValidateNode(nodeID string, answer string) error {
	node, exists := t.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("nodo '%s' no existe", nodeID)
	}

	if node.Status == StatusValidated {
		return fmt.Errorf("nodo '%s' ya está validado", nodeID)
	}

	node.Validate(answer)

	return t.Save()
}

// RejectNode rechaza un nodo
func (t *FrameworkEcho) RejectNode(nodeID string, reason string) error {
	node, exists := t.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("nodo '%s' no existe", nodeID)
	}

	node.Reject(reason)

	return t.Save()
}

// UpdateConfidence actualiza la confianza de un nodo manualmente
func (t *FrameworkEcho) UpdateConfidence(nodeID string, confidence int) error {
	node, exists := t.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("nodo '%s' no existe", nodeID)
	}

	if confidence < 0 || confidence > 100 {
		return fmt.Errorf("confidence debe estar entre 0 y 100, recibí %d", confidence)
	}

	node.Confidence = confidence

	return t.Save()
}

// AddPerception agrega una nota interna de percepción a un nodo.
func (t *FrameworkEcho) AddPerception(nodeID string, perception string) error {
	node, exists := t.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("nodo '%s' no existe", nodeID)
	}

	if strings.TrimSpace(perception) == "" {
		return fmt.Errorf("perception no puede estar vacía")
	}

	node.AddPerception(perception)

	return t.Save()
}

func (t *FrameworkEcho) SetQALogEnabled(enabled bool) error {
	t.Config.QALogEnabled = enabled
	return t.Save()
}

func (t *FrameworkEcho) AddQALog(question, answer, purpose string) error {
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	purpose = strings.TrimSpace(purpose)

	if !t.Config.QALogEnabled {
		return fmt.Errorf("qa log está desactivado; actívalo con: frameworkecho config --qa-log on")
	}
	if question == "" {
		return fmt.Errorf("question no puede estar vacía")
	}
	if answer == "" {
		return fmt.Errorf("answer no puede estar vacía")
	}

	t.QALog = append(t.QALog, QALogEntry{
		Question:  question,
		Answer:    answer,
		Purpose:   purpose,
		CreatedAt: timeNowRFC3339(),
	})

	return t.Save()
}

func (t *FrameworkEcho) SelectOpportunity(nodeID string) error {
	node, exists := t.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("nodo '%s' no existe", nodeID)
	}
	if node.Type != TypeOpportunity {
		return fmt.Errorf("nodo '%s' es %s, no OPPORTUNITY", nodeID, node.Type)
	}
	if node.Status != StatusValidated {
		return fmt.Errorf("opportunity '%s' debe estar validada antes de seleccionarse", nodeID)
	}
	for _, selected := range t.SelectedOpportunityIDs {
		if selected == nodeID {
			return nil
		}
	}
	t.SelectedOpportunityIDs = append(t.SelectedOpportunityIDs, nodeID)
	sort.Strings(t.SelectedOpportunityIDs)
	return t.Save()
}

func (t *FrameworkEcho) SelectedOpportunities() []*Node {
	var nodes []*Node
	for _, id := range t.SelectedOpportunityIDs {
		node, exists := t.Nodes[id]
		if exists && node.Type == TypeOpportunity {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func timeNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// GetPendingQuestions retorna todas las preguntas pendientes organizadas por nodo
func (t *FrameworkEcho) GetPendingQuestions() []PendingQuestion {
	var questions []PendingQuestion

	for _, node := range t.Nodes {
		if node.Status == StatusPending && len(node.QuestionsToAsk) > 0 {
			for _, q := range node.QuestionsToAsk {
				questions = append(questions, PendingQuestion{
					NodeID:   node.ID,
					NodeType: node.Type,
					Layer:    node.Layer,
					Title:    node.Title,
					Question: q,
				})
			}
		}
	}

	// Ordenar por layer (primero las más bajas)
	sort.Slice(questions, func(i, j int) bool {
		if questions[i].Layer != questions[j].Layer {
			return questions[i].Layer < questions[j].Layer
		}
		return questions[i].NodeID < questions[j].NodeID
	})

	return questions
}

// PendingQuestion representa una pregunta pendiente
type PendingQuestion struct {
	NodeID   string `json:"node_id"`
	NodeType string `json:"node_type"`
	Layer    int    `json:"layer"`
	Title    string `json:"title"`
	Question string `json:"question"`
}

// GetStats retorna estadísticas del árbol
func (t *FrameworkEcho) GetStats() TreeStats {
	stats := TreeStats{
		ByLayer: make(map[int]LayerStats),
	}

	for _, node := range t.Nodes {
		stats.TotalNodes++

		ls := stats.ByLayer[node.Layer]
		ls.Total++
		if node.Status == StatusValidated {
			ls.Validated++
		} else if node.Status == StatusPending {
			ls.Pending++
		} else if node.Status == StatusRejected {
			ls.Rejected++
		}
		stats.ByLayer[node.Layer] = ls
	}

	return stats
}

// TreeStats estadísticas del árbol
type TreeStats struct {
	TotalNodes int                `json:"total_nodes"`
	ByLayer    map[int]LayerStats `json:"by_layer"`
}

// LayerStats estadísticas por capa
type LayerStats struct {
	Total     int `json:"total"`
	Validated int `json:"validated"`
	Pending   int `json:"pending"`
	Rejected  int `json:"rejected"`
}

// ShowTree imprime el árbol de forma visual
func (t *FrameworkEcho) ShowTree() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("╔══════════════════════════════════════════════════════════╗\n"))
	sb.WriteString(fmt.Sprintf("║  FrameworkEcho: %s\n", t.ProjectID))
	sb.WriteString(fmt.Sprintf("║  Cliente: %s | Inicio: %s\n", t.ClientName, t.DateStarted))
	sb.WriteString(fmt.Sprintf("╚══════════════════════════════════════════════════════════╝\n\n"))

	layerNames := map[int]string{
		0: "AXIOMS (Lo que se observa)",
		1: "THEORIES (Hipótesis a validar)",
		2: "TASKS (Tareas descubiertas)",
		3: "PAINS (Dolores confirmados)",
		4: "OPPORTUNITIES (Automatizaciones candidatas)",
	}

	for layer := 0; layer <= t.CurrentMaxLayer && layer <= 4; layer++ {
		nodesInLayer := t.getNodesInLayer(layer)
		if len(nodesInLayer) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("── Layer %d: %s ──\n", layer, layerNames[layer]))

		for _, node := range nodesInLayer {
			statusIcon := "⏳"
			switch node.Status {
			case StatusValidated:
				statusIcon = "✅"
			case StatusRejected:
				statusIcon = "❌"
			}

			parentInfo := ""
			if node.ParentID != "" {
				parentInfo = fmt.Sprintf(" (parent: %s)", node.ParentID)
			}

			sb.WriteString(fmt.Sprintf("  %s [%s] %s (conf: %d%%)%s\n",
				statusIcon, node.ID, node.Title, node.Confidence, parentInfo))

			if node.ValidationAnswer != "" {
				sb.WriteString(fmt.Sprintf("     └─ Respuesta: %s\n", node.ValidationAnswer))
			}

			if len(node.Perceptions) > 0 {
				sb.WriteString("     └─ Percepciones:\n")
				for _, perception := range node.Perceptions {
					sb.WriteString(fmt.Sprintf("        - %s\n", perception))
				}
			}

			if len(node.ChildrenIDs) > 0 {
				sb.WriteString(fmt.Sprintf("     └─ Hijos: %s\n", strings.Join(node.ChildrenIDs, ", ")))
			}
		}

		sb.WriteString("\n")
	}

	// Stats
	stats := t.GetStats()
	sb.WriteString(fmt.Sprintf("── Resumen ──\n"))
	sb.WriteString(fmt.Sprintf("  Total nodos: %d\n", stats.TotalNodes))
	for layer := 0; layer <= 4; layer++ {
		if ls, ok := stats.ByLayer[layer]; ok {
			sb.WriteString(fmt.Sprintf("  Layer %d: %d total | %d validados | %d pendientes | %d rechazados\n",
				layer, ls.Total, ls.Validated, ls.Pending, ls.Rejected))
		}
	}

	// Verificar si se puede avanzar de capa
	nextLayer := t.CurrentMaxLayer + 1
	if nextLayer <= 4 {
		needed := minValidatedPrevLayer[nextLayer]
		have := t.CountValidatedInLayer(t.CurrentMaxLayer)
		if have >= needed {
			sb.WriteString(fmt.Sprintf("\n  🔓 Puedes crear nodos de Layer %d (%s)\n", nextLayer, layerNames[nextLayer]))
		} else {
			sb.WriteString(fmt.Sprintf("\n  🔒 Para desbloquear Layer %d: necesitas %d validados en Layer %d, tienes %d\n",
				nextLayer, needed, t.CurrentMaxLayer, have))
		}
	}

	return sb.String()
}

// getNodesInLayer retorna nodos de una capa, ordenados por ID
func (t *FrameworkEcho) getNodesInLayer(layer int) []*Node {
	var nodes []*Node
	for _, node := range t.Nodes {
		if node.Layer == layer {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes
}

package tree

import (
	"fmt"
	"time"
)

// Tipos de nodo
const (
	TypeAxiom       = "AXIOM"
	TypeTheory      = "THEORY"
	TypeTask        = "TASK"
	TypePain        = "PAIN"
	TypeOpportunity = "OPPORTUNITY"
)

// Estados
const (
	StatusValidated = "VALIDATED"
	StatusPending   = "PENDING"
	StatusRejected  = "REJECTED"
)

// Prefijos de ID por tipo
var prefixMap = map[string]string{
	TypeAxiom:       "ax",
	TypeTheory:      "th",
	TypeTask:        "tk",
	TypePain:        "pn",
	TypeOpportunity: "op",
}

// Capa por tipo
var layerMap = map[string]int{
	TypeAxiom:       0,
	TypeTheory:      1,
	TypeTask:        2,
	TypePain:        3,
	TypeOpportunity: 4,
}

// Mínimo de nodos validados en la capa anterior para poder crear en la siguiente
var minValidatedPrevLayer = map[int]int{
	0: 0, // Axioms no necesitan nada previo
	1: 3, // Theories necesitan >= 3 axioms validados
	2: 3, // Tasks necesitan >= 3 theories validadas
	3: 2, // Pains necesitan >= 2 tasks validadas
	4: 1, // Opportunities necesitan >= 1 pain validado
}

// Node representa un nodo en el árbol de conocimiento
type Node struct {
	ID               string   `json:"id"`
	Layer            int      `json:"layer"`
	Type             string   `json:"type"`
	Title            string   `json:"title"`
	Evidence         []string `json:"evidence"`
	Status           string   `json:"status"`
	Confidence       int      `json:"confidence"`
	ParentID         string   `json:"parent_id,omitempty"`
	ChildrenIDs      []string `json:"children_ids,omitempty"`
	QuestionsToAsk   []string `json:"questions_to_ask,omitempty"`
	Perceptions      []string `json:"perceptions,omitempty"`
	ValidationAnswer string   `json:"validation_answer,omitempty"`
	CreatedAt        string   `json:"created_at"`
	ValidatedAt      string   `json:"validated_at,omitempty"`
}

// NewNode crea un nodo con valores por defecto según su tipo
func NewNode(nodeType string, title string, evidence []string, parentID string, seqNum int) (*Node, error) {
	prefix, ok := prefixMap[nodeType]
	if !ok {
		return nil, fmt.Errorf("tipo de nodo inválido: %s (válidos: AXIOM, THEORY, TASK, PAIN, OPPORTUNITY)", nodeType)
	}

	layer, ok := layerMap[nodeType]
	if !ok {
		return nil, fmt.Errorf("tipo de nodo sin capa definida: %s", nodeType)
	}

	// Axioms se validan automáticamente (son hechos observables)
	status := StatusPending
	confidence := 60
	if nodeType == TypeAxiom {
		status = StatusValidated
		confidence = 100
	}

	id := fmt.Sprintf("%s_%03d", prefix, seqNum)

	node := &Node{
		ID:          id,
		Layer:       layer,
		Type:        nodeType,
		Title:       title,
		Evidence:    evidence,
		Status:      status,
		Confidence:  confidence,
		ParentID:    parentID,
		ChildrenIDs: []string{},
		Perceptions: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	return node, nil
}

// Validate marca un nodo como validado con la respuesta del cliente
func (n *Node) Validate(answer string) {
	n.Status = StatusValidated
	n.Confidence = 95
	n.ValidationAnswer = answer
	n.ValidatedAt = time.Now().UTC().Format(time.RFC3339)
}

// Reject marca un nodo como rechazado
func (n *Node) Reject(reason string) {
	n.Status = StatusRejected
	n.Confidence = 0
	n.ValidationAnswer = reason
	n.ValidatedAt = time.Now().UTC().Format(time.RFC3339)
}

// AddChild agrega un hijo al nodo
func (n *Node) AddChild(childID string) {
	for _, id := range n.ChildrenIDs {
		if id == childID {
			return
		}
	}
	n.ChildrenIDs = append(n.ChildrenIDs, childID)
}

// AddPerception agrega una nota interna de percepción al nodo.
func (n *Node) AddPerception(perception string) {
	if perception == "" {
		return
	}
	n.Perceptions = append(n.Perceptions, perception)
}

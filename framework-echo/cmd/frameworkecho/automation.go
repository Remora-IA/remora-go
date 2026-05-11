package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/user/framework-echo/internal/llm"
	"github.com/user/framework-echo/internal/tree"
)

// cmdNextQuestion imprime la próxima pregunta estratégica como JSON.
// Contrato (estandarizado por user_input.next_question_cmd del manifest):
//
//	{ "id": "rd_xxxxxxxx", "text": "...", "ask_via": "" }
//
// Si no hay pregunta, imprime {} y exit 0.
//
// El id es un hash determinista del texto, así el orquestador puede deduplicar
// preguntas repetidas dentro de la misma conversación.
func cmdNextQuestion() {
	t := loadTree()
	report := t.AssessAlfaReadiness()
	out := map[string]string{}
	if report.NextQuestion != "" && report.RecommendedAction != tree.RecommendedPassToAlfa {
		// Usar LLM para generar una pregunta contextual y natural.
		questionText := generateEchoQuestion(t, report)
		out["id"] = "rd_" + shortHash(questionText)
		out["text"] = questionText
		out["ask_via"] = ""
		out["recommended_action"] = report.RecommendedAction
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

const echoSystemPrompt = `Eres Echo, la IA de descubrimiento de procesos de Remora.

Tu trabajo es guiar una conversación de descubrimiento para entender el mundo real del usuario y construir un árbol validado:
AXIOM -> THEORY -> TASK -> PAIN -> OPPORTUNITY

Tu personalidad:
- Eres directo y cálido, sin ser formal.
- Hablas en español natural.
- Hacés una pregunta a la vez.
- Preguntás por comportamiento actual, no por soluciones ideales.
- Nunca preguntás "qué querés automatizar".
- Nunca ofrecés una solución antes de confirmar un dolor real.
- Pedís recursos reales (captura, foto, Excel, ejemplo) cuando reducen incertidumbre.

No expliques el framework. Hacé una sola pregunta natural para avanzar el proceso.`

func generateEchoQuestion(t *tree.FrameworkEcho, report tree.ReadinessReport) string {
	client, err := llm.NewClient()
	if err != nil {
		// LLM no disponible: devolver error en lugar de fallback.
		fmt.Fprintf(os.Stderr, "Error LLM: %v\n", err)
		return report.NextQuestion
	}
	treeContext := buildTreeContext(t, report)
	userPrompt := fmt.Sprintf(
		"Estado actual del árbol de descubrimiento:\n%s\n\nAcción recomendada: %s\nPregunta base: %s\n\nGenera UNA sola pregunta natural y directa para hacerle al usuario. Solo la pregunta, sin explicación.",
		treeContext, report.RecommendedAction, report.NextQuestion)
	reply, err := client.Generate(context.Background(), echoSystemPrompt, userPrompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error LLM: %v\n", err)
		return report.NextQuestion
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return report.NextQuestion
	}
	return reply
}

func buildTreeContext(t *tree.FrameworkEcho, report tree.ReadinessReport) string {
	stats := t.GetStats()
	var sb strings.Builder
	if t.ProjectID != "" {
		fmt.Fprintf(&sb, "Proyecto: %s, Cliente: %s\n", t.ProjectID, t.ClientName)
	}
	fmt.Fprintf(&sb, "Nodos totales: %d\n", stats.TotalNodes)
	layerNames := map[int]string{0: "AXIOMS", 1: "THEORIES", 2: "TASKS", 3: "PAINS", 4: "OPPORTUNITIES"}
	for layer := 0; layer <= 4; layer++ {
		if ls, ok := stats.ByLayer[layer]; ok && ls.Total > 0 {
			fmt.Fprintf(&sb, "  %s: %d total, %d validados\n", layerNames[layer], ls.Total, ls.Validated)
		}
	}
	// Incluir títulos de nodos validados para contexto.
	for _, n := range t.Nodes {
		if n.Status == tree.StatusValidated {
			fmt.Fprintf(&sb, "  [%s] %s: %s\n", n.ID, n.Type, n.Title)
		}
	}
	if len(report.Risks) > 0 {
		fmt.Fprintf(&sb, "Riesgos: %s\n", strings.Join(report.Risks, ", "))
	}
	return sb.String()
}

// cmdIngestAnswer recibe una respuesta del usuario y avanza el árbol UN paso
// en la cadena AXIOM → THEORY → TASK → PAIN → OPPORTUNITY → SELECT.
//
// Estandariza la entrada de Echo: en lugar de obligar al cliente a saber qué
// add-X corresponde, este comando interpreta la respuesta según el estado
// actual del árbol y crea/valida el nodo apropiado.
//
// Flags:
//
//	--answer "<texto>"        Respuesta del usuario (obligatorio).
//	--question-id <id>        Opcional. Si está presente y empieza con un id de
//	                          nodo conocido (ej "th_001"), se valida ese nodo
//	                          en lugar de avanzar al siguiente layer.
//
// Output JSON:
//
//	{ "advanced": true, "created": "tk_001", "type": "TASK", "validated": true }
//	{ "advanced": false, "reason": "ready_for_alfa" }
func cmdIngestAnswer() {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	answer := fs.String("answer", "", "respuesta del usuario")
	questionID := fs.String("question-id", "", "opcional: id de nodo a validar directamente")
	_ = fs.Parse(os.Args[2:])
	if strings.TrimSpace(*answer) == "" {
		fmt.Fprintln(os.Stderr, "Error: --answer es obligatorio")
		os.Exit(1)
	}

	t := loadTree()

	// Caso 1: question-id apunta a un nodo concreto → validar ese nodo.
	if *questionID != "" {
		if _, ok := t.Nodes[*questionID]; ok {
			if err := t.ValidateNode(*questionID, *answer); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			emitIngestResult(map[string]interface{}{
				"advanced":   true,
				"created":    *questionID,
				"validated":  true,
				"type":       t.Nodes[*questionID].Type,
				"layer":      t.Nodes[*questionID].Layer,
			})
			return
		}
		// id no reconocido (ej rd_<hash> de readiness): caer al avance progresivo.
	}

	// Caso 2: avance progresivo. Una respuesta del usuario puede requerir
	// crear varios layers internos (ej: axiom → theory → task) hasta que la
	// pregunta estratégica realmente cambie. Sin esto, el orquestador
	// dedup-aría la misma pregunta y la conversación se trabaría.
	beforeQ := t.AssessAlfaReadiness().NextQuestion
	steps := []map[string]interface{}{}
	for i := 0; i < 5; i++ {
		result, err := advanceTree(t, *answer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		steps = append(steps, result)
		if adv, _ := result["advanced"].(bool); !adv {
			break
		}
		afterQ := t.AssessAlfaReadiness().NextQuestion
		if afterQ != beforeQ {
			break
		}
	}
	last := steps[len(steps)-1]
	last["steps"] = steps
	emitIngestResult(last)
}

// advanceTree implementa la máquina de estado:
//   AXIOM → THEORY → TASK → PAIN → OPPORTUNITY → select-opportunity
// Crea y valida UN nodo por llamada y devuelve metadatos del paso.
func advanceTree(t *tree.FrameworkEcho, answer string) (map[string]interface{}, error) {
	title := truncateUTF8(answer, 120)
	evidence := []string{answer}

	// Layer 0: si no hay axiom, crear axiom (auto-validado).
	if !hasAnyOfType(t, tree.TypeAxiom) {
		node, err := t.AddNode(tree.TypeAxiom, title, evidence, "")
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"advanced": true, "created": node.ID, "type": tree.TypeAxiom, "validated": true, "layer": 0,
		}, nil
	}

	// Layer 1: si no hay theory validada, crear theory bajo último axiom y validar.
	if !hasValidatedOfType(t, tree.TypeTheory) {
		parent := lastValidatedOfType(t, tree.TypeAxiom)
		if parent == "" {
			return nil, fmt.Errorf("no hay axiom validado para colgar la theory")
		}
		node, err := t.AddNode(tree.TypeTheory, title, evidence, parent)
		if err != nil {
			return nil, err
		}
		if err := t.ValidateNode(node.ID, answer); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"advanced": true, "created": node.ID, "type": tree.TypeTheory, "validated": true, "layer": 1,
		}, nil
	}

	// Layer 2: TASK
	if !hasValidatedOfType(t, tree.TypeTask) {
		parent := lastValidatedOfType(t, tree.TypeTheory)
		node, err := t.AddNode(tree.TypeTask, title, evidence, parent)
		if err != nil {
			return nil, err
		}
		if err := t.ValidateNode(node.ID, answer); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"advanced": true, "created": node.ID, "type": tree.TypeTask, "validated": true, "layer": 2,
		}, nil
	}

	// Layer 3: PAIN
	if !hasValidatedOfType(t, tree.TypePain) {
		parent := lastValidatedOfType(t, tree.TypeTask)
		node, err := t.AddNode(tree.TypePain, title, evidence, parent)
		if err != nil {
			return nil, err
		}
		if err := t.ValidateNode(node.ID, answer); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"advanced": true, "created": node.ID, "type": tree.TypePain, "validated": true, "layer": 3,
		}, nil
	}

	// Layer 4: OPPORTUNITY
	if !hasValidatedOfType(t, tree.TypeOpportunity) {
		parent := lastValidatedOfType(t, tree.TypePain)
		node, err := t.AddNode(tree.TypeOpportunity, title, evidence, parent)
		if err != nil {
			return nil, err
		}
		if err := t.ValidateNode(node.ID, answer); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"advanced": true, "created": node.ID, "type": tree.TypeOpportunity, "validated": true, "layer": 4,
		}, nil
	}

	// Selección: si hay opportunity validada pero ninguna seleccionada.
	if len(t.SelectedOpportunityIDs) == 0 {
		opID := lastValidatedOfType(t, tree.TypeOpportunity)
		if opID != "" {
			if err := t.SelectOpportunity(opID); err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"advanced": true, "selected_opportunity": opID,
			}, nil
		}
	}

	return map[string]interface{}{
		"advanced": false, "reason": "tree_complete_or_ready_for_alfa",
	}, nil
}

func hasAnyOfType(t *tree.FrameworkEcho, nodeType string) bool {
	for _, n := range t.Nodes {
		if n.Type == nodeType {
			return true
		}
	}
	return false
}

func hasValidatedOfType(t *tree.FrameworkEcho, nodeType string) bool {
	for _, n := range t.Nodes {
		if n.Type == nodeType && n.Status == tree.StatusValidated {
			return true
		}
	}
	return false
}

func lastValidatedOfType(t *tree.FrameworkEcho, nodeType string) string {
	var found string
	for _, n := range t.Nodes {
		if n.Type == nodeType && n.Status == tree.StatusValidated {
			if n.ID > found {
				found = n.ID
			}
		}
	}
	return found
}

func truncateUTF8(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}

func emitIngestResult(data map[string]interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

// shortHash es un FNV-1a 32-bit en hex de 8 chars. Determinista, suficiente
// para deduplicar preguntas idénticas en la misma conversación.
func shortHash(s string) string {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

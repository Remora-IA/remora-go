package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/user/framework-echo/internal/tree"
)

const defaultFile = "frameworkecho.json"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		cmdInit()
	case "reset":
		cmdReset()
	case "add-axiom":
		cmdAddAxiom()
	case "add-theory":
		cmdAddTheory()
	case "add-task":
		cmdAddTask()
	case "add-pain":
		cmdAddPain()
	case "add-opportunity":
		cmdAddOpportunity()
	case "validate":
		cmdValidate()
	case "reject":
		cmdReject()
	case "confidence":
		cmdConfidence()
	case "show-tree":
		cmdShowTree()
	case "next-questions":
		cmdNextQuestions()
	case "status":
		cmdStatus()
	case "add-question":
		cmdAddQuestion()
	case "add-perception":
		cmdAddPerception()
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: comando desconocido '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`FrameworkEcho CLI - Árbol de Conocimiento Progresivo

USO:
  frameworkecho <comando> [opciones]

COMANDOS:
  init                              Inicializa un nuevo proyecto
    --project-id <id>               ID del proyecto
    --client <nombre>               Nombre del cliente
    --date <YYYY-MM-DD>             Fecha de inicio

  add-axiom                         Agrega un axioma (Layer 0, se auto-valida)
    --title <título>                Título del axioma
    --evidence <evidencia>          Evidencia (repetir para múltiples)

  add-theory                        Agrega una teoría (Layer 1)
    --parent <id>                   ID del nodo padre (obligatorio)
    --title <título>                Título de la teoría
    --evidence <evidencia>          Evidencia (repetir para múltiples)

  add-task                          Agrega una tarea (Layer 2)
    --parent <id>                   ID del nodo padre (obligatorio)
    --title <título>                Título de la tarea
    --evidence <evidencia>          Evidencia (repetir para múltiples)

  add-pain                          Agrega un dolor (Layer 3)
    --parent <id>                   ID del nodo padre (obligatorio)
    --title <título>                Título del dolor
    --evidence <evidencia>          Evidencia (repetir para múltiples)

  add-opportunity                   Agrega una automatización candidata (Layer 4)
    --parent <id>                   ID del PAIN padre (obligatorio)
    --title <título>                Título de la oportunidad
    --evidence <evidencia>          Evidencia (repetir para múltiples)
    Nota: se anota como candidata; no significa que deba ofrecerse todavía.

  validate <node_id>                Valida un nodo con respuesta del cliente
    --answer <respuesta>            Respuesta del cliente

  reject <node_id>                  Rechaza un nodo
    --reason <razón>                Razón del rechazo

  confidence <node_id>              Actualiza confianza manualmente
    --value <0-100>                 Nuevo valor de confianza

  add-question <node_id>            Agrega una pregunta personalizada a un nodo
    --question <pregunta>            Pregunta a agregar

  add-perception <node_id>          Agrega una nota interna de percepción a un nodo
    --note <nota>                   Lectura interna: comportamiento, contradicción o fricción latente

  show-tree                         Muestra el árbol visual completo
  next-questions                    Muestra preguntas pendientes para el cliente
  status                            Muestra estadísticas del árbol
  reset                             Elimina todos los nodos (mantiene proyecto)
  help                              Muestra esta ayuda`)
}

// parseFlags parsea flags simples del estilo --key value
func parseFlags(args []string) map[string][]string {
	flags := make(map[string][]string)
	i := 0
	for i < len(args) {
		if strings.HasPrefix(args[i], "--") {
			key := strings.TrimPrefix(args[i], "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[key] = append(flags[key], args[i+1])
				i += 2
			} else {
				flags[key] = append(flags[key], "true")
				i++
			}
		} else {
			// Positional argument
			flags["_positional"] = append(flags["_positional"], args[i])
			i++
		}
	}
	return flags
}

func getFlag(flags map[string][]string, key string) string {
	if vals, ok := flags[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func getFlags(flags map[string][]string, key string) []string {
	if vals, ok := flags[key]; ok {
		return vals
	}
	return []string{}
}

func requireFlag(flags map[string][]string, key string) string {
	val := getFlag(flags, key)
	if val == "" {
		fmt.Fprintf(os.Stderr, "Error: --%s es obligatorio\n", key)
		os.Exit(1)
	}
	return val
}

func loadTree() *tree.FrameworkEcho {
	t, err := tree.LoadOrCreate(defaultFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error cargando árbol: %v\n", err)
		os.Exit(1)
	}
	return t
}

func cmdInit() {
	flags := parseFlags(os.Args[2:])
	projectID := requireFlag(flags, "project-id")
	client := requireFlag(flags, "client")
	date := requireFlag(flags, "date")

	t := loadTree()
	if err := t.Init(projectID, client, date); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Proyecto inicializado: %s (cliente: %s)\n", projectID, client)
	fmt.Printf("  Archivo: %s\n", defaultFile)
}

func cmdAddAxiom() {
	flags := parseFlags(os.Args[2:])
	title := requireFlag(flags, "title")
	evidence := getFlags(flags, "evidence")

	if len(evidence) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --evidence es obligatorio (al menos una)\n")
		os.Exit(1)
	}

	t := loadTree()
	node, err := t.AddNode(tree.TypeAxiom, title, evidence, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Creado %s (Layer %d, %s) - confidence: %d%%\n", node.ID, node.Layer, node.Type, node.Confidence)
	printQuickStats(t)
}

func cmdAddTheory() {
	flags := parseFlags(os.Args[2:])
	title := requireFlag(flags, "title")
	parentID := requireFlag(flags, "parent")
	evidence := getFlags(flags, "evidence")

	if len(evidence) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --evidence es obligatorio (al menos una)\n")
		os.Exit(1)
	}

	t := loadTree()
	node, err := t.AddNode(tree.TypeTheory, title, evidence, parentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Creado %s (Layer %d, %s, parent: %s) - confidence: %d%%\n",
		node.ID, node.Layer, node.Type, node.ParentID, node.Confidence)

	if len(node.QuestionsToAsk) > 0 {
		fmt.Printf("  Preguntas generadas:\n")
		for _, q := range node.QuestionsToAsk {
			fmt.Printf("    → %s\n", q)
		}
	}

	printQuickStats(t)
}

func cmdAddTask() {
	flags := parseFlags(os.Args[2:])
	title := requireFlag(flags, "title")
	parentID := requireFlag(flags, "parent")
	evidence := getFlags(flags, "evidence")

	if len(evidence) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --evidence es obligatorio (al menos una)\n")
		os.Exit(1)
	}

	t := loadTree()
	node, err := t.AddNode(tree.TypeTask, title, evidence, parentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Creado %s (Layer %d, %s, parent: %s) - confidence: %d%%\n",
		node.ID, node.Layer, node.Type, node.ParentID, node.Confidence)

	if len(node.QuestionsToAsk) > 0 {
		fmt.Printf("  Preguntas generadas:\n")
		for _, q := range node.QuestionsToAsk {
			fmt.Printf("    → %s\n", q)
		}
	}

	printQuickStats(t)
}

func cmdAddPain() {
	flags := parseFlags(os.Args[2:])
	title := requireFlag(flags, "title")
	parentID := requireFlag(flags, "parent")
	evidence := getFlags(flags, "evidence")

	if len(evidence) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --evidence es obligatorio (al menos una)\n")
		os.Exit(1)
	}

	t := loadTree()
	node, err := t.AddNode(tree.TypePain, title, evidence, parentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Creado %s (Layer %d, %s, parent: %s) - confidence: %d%%\n",
		node.ID, node.Layer, node.Type, node.ParentID, node.Confidence)

	if len(node.QuestionsToAsk) > 0 {
		fmt.Printf("  Preguntas generadas:\n")
		for _, q := range node.QuestionsToAsk {
			fmt.Printf("    → %s\n", q)
		}
	}

	printQuickStats(t)
}

func cmdAddOpportunity() {
	flags := parseFlags(os.Args[2:])
	title := requireFlag(flags, "title")
	parentID := requireFlag(flags, "parent")
	evidence := getFlags(flags, "evidence")

	if len(evidence) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --evidence es obligatorio (al menos una)\n")
		os.Exit(1)
	}

	t := loadTree()
	node, err := t.AddNode(tree.TypeOpportunity, title, evidence, parentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Creado %s (Layer %d, %s, parent: %s) - confidence: %d%%\n",
		node.ID, node.Layer, node.Type, node.ParentID, node.Confidence)
	fmt.Printf("  Nota: oportunidad anotada como candidata; no se ofrece hasta confirmar encaje con el dolor real.\n")

	if len(node.QuestionsToAsk) > 0 {
		fmt.Printf("  Preguntas generadas:\n")
		for _, q := range node.QuestionsToAsk {
			fmt.Printf("    → %s\n", q)
		}
	}

	printQuickStats(t)
}

func cmdValidate() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: debes especificar el node_id\n")
		fmt.Fprintf(os.Stderr, "Uso: frameworkecho validate <node_id> --answer <respuesta>\n")
		os.Exit(1)
	}

	nodeID := os.Args[2]
	flags := parseFlags(os.Args[3:])
	answer := requireFlag(flags, "answer")

	t := loadTree()
	if err := t.ValidateNode(nodeID, answer); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Nodo %s validado con respuesta del cliente\n", nodeID)
	printQuickStats(t)
}

func cmdReject() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: debes especificar el node_id\n")
		fmt.Fprintf(os.Stderr, "Uso: frameworkecho reject <node_id> --reason <razón>\n")
		os.Exit(1)
	}

	nodeID := os.Args[2]
	flags := parseFlags(os.Args[3:])
	reason := requireFlag(flags, "reason")

	t := loadTree()
	if err := t.RejectNode(nodeID, reason); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Nodo %s rechazado\n", nodeID)
	printQuickStats(t)
}

func cmdConfidence() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: debes especificar el node_id\n")
		fmt.Fprintf(os.Stderr, "Uso: frameworkecho confidence <node_id> --value <0-100>\n")
		os.Exit(1)
	}

	nodeID := os.Args[2]
	flags := parseFlags(os.Args[3:])
	valueStr := requireFlag(flags, "value")

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: --value debe ser un número, recibí '%s'\n", valueStr)
		os.Exit(1)
	}

	t := loadTree()
	if err := t.UpdateConfidence(nodeID, value); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Nodo %s confidence actualizado a %d%%\n", nodeID, value)
}

func cmdShowTree() {
	t := loadTree()
	fmt.Println(t.ShowTree())
}

func cmdNextQuestions() {
	t := loadTree()
	questions := t.GetPendingQuestions()

	if len(questions) == 0 {
		fmt.Println("No hay preguntas pendientes.")
		fmt.Println("Si estás en Layer 0, crea theories con: frameworkecho add-theory ...")
		return
	}

	fmt.Println("═══ Preguntas para hacerle al cliente ═══")
	fmt.Println()

	currentNode := ""
	for _, q := range questions {
		if q.NodeID != currentNode {
			currentNode = q.NodeID
			fmt.Printf("[%s] %s - \"%s\"\n", q.NodeID, q.NodeType, q.Title)
		}
		fmt.Printf("  → %s\n", q.Question)
	}

	fmt.Printf("\nPara validar con respuesta: frameworkecho validate <node_id> --answer \"respuesta del cliente\"\n")
	fmt.Printf("Para rechazar: frameworkecho reject <node_id> --reason \"razón\"\n")
}

func cmdReset() {
	flags := parseFlags(os.Args[2:])
	_ = flags // --force sería para confirmar sin preguntar

	t := loadTree()

	// Contar nodos existentes
	count := len(t.Nodes)

	// Limpiar nodos
	t.Nodes = make(map[string]*tree.Node)
	t.CurrentMaxLayer = 0
	t.FocusNodes = []string{}

	if err := t.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error guardando: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Reset completo: %d nodos eliminados\n", count)
	fmt.Printf("  Proyecto '%s' intacto, listo para nueva sesión\n", t.ProjectID)
}

func cmdStatus() {
	t := loadTree()
	stats := t.GetStats()

	fmt.Printf("Proyecto: %s | Cliente: %s\n", t.ProjectID, t.ClientName)
	fmt.Printf("Total nodos: %d\n\n", stats.TotalNodes)

	layerNames := map[int]string{
		0: "AXIOMS",
		1: "THEORIES",
		2: "TASKS",
		3: "PAINS",
		4: "OPPORTUNITIES",
	}

	for layer := 0; layer <= 4; layer++ {
		if ls, ok := stats.ByLayer[layer]; ok {
			fmt.Printf("  Layer %d (%s): %d total | %d ✅ | %d ⏳ | %d ❌\n",
				layer, layerNames[layer], ls.Total, ls.Validated, ls.Pending, ls.Rejected)
		}
	}

	// Mostrar qué se puede hacer
	nextLayer := t.CurrentMaxLayer + 1
	if nextLayer <= 4 {
		needed := 3
		if nextLayer == 3 {
			needed = 2
		} else if nextLayer == 4 {
			needed = 1
		}
		have := t.CountValidatedInLayer(t.CurrentMaxLayer)
		if have >= needed {
			fmt.Printf("\n🔓 Layer %d desbloqueado - puedes crear nodos de tipo %s\n", nextLayer, layerNames[nextLayer])
		} else {
			fmt.Printf("\n🔒 Para desbloquear Layer %d (%s): valida %d más en Layer %d\n",
				nextLayer, layerNames[nextLayer], needed-have, t.CurrentMaxLayer)
		}
	}
}

func cmdAddQuestion() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: debes especificar el node_id\n")
		fmt.Fprintf(os.Stderr, "Uso: frameworkecho add-question <node_id> --question \"pregunta\"\n")
		os.Exit(1)
	}

	nodeID := os.Args[2]
	flags := parseFlags(os.Args[3:])
	question := requireFlag(flags, "question")

	t := loadTree()

	node, exists := t.Nodes[nodeID]
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: nodo '%s' no existe\n", nodeID)
		os.Exit(1)
	}

	node.QuestionsToAsk = append(node.QuestionsToAsk, question)

	if err := t.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error guardando: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Pregunta agregada a %s\n", nodeID)
	fmt.Printf("  → %s\n", question)
}

func cmdAddPerception() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: debes especificar el node_id\n")
		fmt.Fprintf(os.Stderr, "Uso: frameworkecho add-perception <node_id> --note \"nota interna\"\n")
		os.Exit(1)
	}

	nodeID := os.Args[2]
	flags := parseFlags(os.Args[3:])
	note := requireFlag(flags, "note")

	t := loadTree()
	if err := t.AddPerception(nodeID, note); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Percepción agregada a %s\n", nodeID)
	fmt.Printf("  → %s\n", note)
}

func printQuickStats(t *tree.FrameworkEcho) {
	stats := t.GetStats()
	parts := []string{}
	layerNames := map[int]string{0: "axioms", 1: "theories", 2: "tasks", 3: "pains", 4: "opportunities"}
	for layer := 0; layer <= 4; layer++ {
		if ls, ok := stats.ByLayer[layer]; ok && ls.Total > 0 {
			parts = append(parts, fmt.Sprintf("%d %s(%d✅)", ls.Total, layerNames[layer], ls.Validated))
		}
	}
	fmt.Printf("  Estado: %s\n", strings.Join(parts, " | "))
}

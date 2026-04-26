package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	statePath = "temp/foco/today.json"
	mdPath    = "temp/foco/today.md"
)

type DayPlan struct {
	Date      string `json:"date"`
	Version   string `json:"version"`
	Objective string `json:"objective"`
	Notes     []Note `json:"notes"`
	Nodes     []Node `json:"nodes"`
	Tasks     []Task `json:"tasks"`
}

type Note struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Time string `json:"time"`
}

type Node struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Evidence []string `json:"evidence"`
	Status   string   `json:"status"`
}

type Task struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Why           string `json:"why"`
	Expected      string `json:"expected"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	CompletedAt   string `json:"completed_at,omitempty"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}

func main() {
	command := "show"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	var err error
	switch command {
	case "init":
		err = runInit(os.Args[2:])
	case "note":
		err = runNote(os.Args[2:])
	case "ask":
		err = runAsk()
	case "answer":
		err = runAnswer(os.Args[2:])
	case "axiom":
		err = runAxiom(os.Args[2:])
	case "tree":
		err = runTree()
	case "readiness":
		err = runReadiness()
	case "plan":
		err = runPlan()
	case "priority":
		err = runPriority()
	case "next":
		err = runNext()
	case "done":
		err = runDone(os.Args[2:])
	case "block":
		err = runBlock(os.Args[2:])
	case "show":
		err = runShow()
	case "help", "-h", "--help":
		usage()
		return
	default:
		err = fmt.Errorf("comando desconocido: %s", command)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "foco_error: %v\n", err)
		os.Exit(1)
	}
}

func runInit(args []string) error {
	params := parseArgs(args)
	version := strings.TrimSpace(params["version"])
	objective := strings.TrimSpace(params["objective"])
	if version == "" || objective == "" {
		return errors.New("uso: foco init --version v0.1.5 --objective \"objetivo del dia\"")
	}

	plan := DayPlan{
		Date:      time.Now().Format("2006-01-02"),
		Version:   version,
		Objective: objective,
		Notes: []Note{
			{
				Kind: "why",
				Text: "El dia se controla por resultado demostrable, no por cantidad de piezas nuevas.",
				Time: time.Now().Format(time.RFC3339),
			},
		},
		Nodes: []Node{
			{
				ID:       "ctx_001",
				Type:     "CONTEXT",
				Title:    objective,
				Evidence: []string{"Objetivo declarado al iniciar Foco."},
				Status:   "confirmed",
			},
		},
		Tasks: defaultTasks(time.Now().Format(time.RFC3339)),
	}
	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_ready: %s %s\n", plan.Version, plan.Objective)
	return nil
}

func runNote(args []string) error {
	params := parseArgs(args)
	kind := strings.TrimSpace(params["kind"])
	text := strings.TrimSpace(params["text"])
	if kind == "" || text == "" {
		return errors.New("uso: foco note --kind human|flow|blocker|done|decision --text \"nota\"")
	}
	if !validKind(kind) {
		return fmt.Errorf("kind invalido: %s", kind)
	}

	plan, err := load()
	if err != nil {
		return err
	}
	plan.Notes = append(plan.Notes, Note{
		Kind: kind,
		Text: text,
		Time: time.Now().Format(time.RFC3339),
	})
	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_note: %s\n", text)
	return nil
}

func runShow() error {
	plan, err := load()
	if err != nil {
		return err
	}
	fmt.Print(formatMarkdown(plan))
	return nil
}

func runAsk() error {
	plan, err := load()
	if err != nil {
		return err
	}
	fmt.Println(nextQuestion(plan))
	return nil
}

func runAnswer(args []string) error {
	params := parseArgs(args)
	text := strings.TrimSpace(params["text"])
	if text == "" {
		return errors.New("uso: foco answer --text \"respuesta libre\"")
	}
	plan, err := load()
	if err != nil {
		return err
	}
	created := materializeAnswer(&plan, text)
	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_answer_recorded: %d nodos\n", created)
	fmt.Println(nextQuestion(plan))
	return nil
}

func runAxiom(args []string) error {
	params := parseArgs(args)
	text := strings.TrimSpace(params["text"])
	evidence := strings.TrimSpace(params["evidence"])
	if text == "" {
		return errors.New("uso: foco axiom --text \"regla no negociable\" --evidence \"de donde sale\"")
	}
	if evidence == "" {
		evidence = text
	}
	plan, err := load()
	if err != nil {
		return err
	}
	plan.Nodes = append(plan.Nodes, Node{
		ID:       nextNodeID(plan.Nodes, "AXIOM"),
		Type:     "AXIOM",
		Title:    summarize(text),
		Evidence: []string{evidence},
		Status:   "confirmed",
	})
	plan.Notes = append(plan.Notes, Note{
		Kind: "axiom",
		Text: summarize(text),
		Time: time.Now().Format(time.RFC3339),
	})
	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_axiom: %s\n", summarize(text))
	return nil
}

func runTree() error {
	plan, err := load()
	if err != nil {
		return err
	}
	fmt.Print(formatTree(plan))
	return nil
}

func runReadiness() error {
	plan, err := load()
	if err != nil {
		return err
	}
	ensureTasks(&plan)
	report := assessReadiness(plan)
	fmt.Printf("version: %s\n", plan.Version)
	fmt.Printf("ready: %v\n", report.Ready)
	fmt.Printf("next_action: %s\n", report.NextAction)
	if len(report.Missing) > 0 {
		fmt.Println("missing:")
		for _, item := range report.Missing {
			fmt.Printf("- %s\n", item)
		}
	}
	return nil
}

func runPlan() error {
	plan, err := load()
	if err != nil {
		return err
	}
	if ensureTasks(&plan) {
		if err := save(plan); err != nil {
			return err
		}
	}
	fmt.Print(formatTasks(plan))
	return nil
}

func runNext() error {
	plan, err := load()
	if err != nil {
		return err
	}
	if ensureTasks(&plan) {
		if err := save(plan); err != nil {
			return err
		}
	}
	task, ok := nextTask(plan)
	if !ok {
		fmt.Println("No hay tareas pendientes. Pregunta al humano si queda alcance nuevo; si no, cierra la sesion de trabajo.")
		return nil
	}
	fmt.Printf("AHORA TOCA: %s\n", task.Title)
	fmt.Printf("id: %s\n", task.ID)
	fmt.Printf("why: %s\n", task.Why)
	fmt.Printf("resultado esperado: %s\n", task.Expected)
	fmt.Printf("cuando termines: go run ./cmd/foco done --id %s --evidence \"que quedo demostrado\"\n", task.ID)
	return nil
}

func runPriority() error {
	plan, err := load()
	if err != nil {
		return err
	}
	if ensureTasks(&plan) {
		if err := save(plan); err != nil {
			return err
		}
	}
	decision := priorityDecision(plan)
	fmt.Printf("PRIORIDAD: %s\n", decision.Title)
	fmt.Printf("id: %s\n", decision.TaskID)
	fmt.Printf("decision: %s\n", decision.Decision)
	fmt.Printf("por que: %s\n", decision.Why)
	fmt.Printf("no hacer ahora: %s\n", decision.NotNow)
	return nil
}

func runDone(args []string) error {
	params := parseArgs(args)
	id := strings.TrimSpace(params["id"])
	evidence := strings.TrimSpace(params["evidence"])
	if id == "" || evidence == "" {
		return errors.New("uso: foco done --id task_001 --evidence \"resultado demostrado\"")
	}
	plan, err := load()
	if err != nil {
		return err
	}
	if ensureTasks(&plan) {
		if err := save(plan); err != nil {
			return err
		}
	}
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == id {
			plan.Tasks[i].Status = "done"
			plan.Tasks[i].CompletedAt = time.Now().Format(time.RFC3339)
			plan.Notes = append(plan.Notes, Note{Kind: "done", Text: fmt.Sprintf("%s: %s", plan.Tasks[i].Title, evidence), Time: time.Now().Format(time.RFC3339)})
			if err := save(plan); err != nil {
				return err
			}
			fmt.Printf("foco_done: %s\n", id)
			return runNext()
		}
	}
	return fmt.Errorf("tarea no encontrada: %s", id)
}

func runBlock(args []string) error {
	params := parseArgs(args)
	id := strings.TrimSpace(params["id"])
	reason := strings.TrimSpace(params["reason"])
	if id == "" || reason == "" {
		return errors.New("uso: foco block --id task_001 --reason \"bloqueo concreto\"")
	}
	plan, err := load()
	if err != nil {
		return err
	}
	if ensureTasks(&plan) {
		if err := save(plan); err != nil {
			return err
		}
	}
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == id {
			plan.Tasks[i].Status = "blocked"
			plan.Tasks[i].BlockedReason = reason
			plan.Notes = append(plan.Notes, Note{Kind: "blocker", Text: fmt.Sprintf("%s: %s", plan.Tasks[i].Title, reason), Time: time.Now().Format(time.RFC3339)})
			if err := save(plan); err != nil {
				return err
			}
			fmt.Printf("foco_blocked: %s\n", id)
			return runNext()
		}
	}
	return fmt.Errorf("tarea no encontrada: %s", id)
}

func validKind(kind string) bool {
	switch kind {
	case "why", "axiom", "human", "flow", "blocker", "done", "decision":
		return true
	default:
		return false
	}
}

func materializeAnswer(plan *DayPlan, text string) int {
	now := time.Now().Format(time.RFC3339)
	created := 0
	lower := strings.ToLower(text)
	add := func(kind, title string) {
		plan.Nodes = append(plan.Nodes, Node{
			ID:       nextNodeID(plan.Nodes, kind),
			Type:     kind,
			Title:    title,
			Evidence: []string{text},
			Status:   "observed",
		})
		created++
	}

	if containsAny(lower, "si o si", "sí o sí", "no deberia por ningun motivo", "no debería por ningún motivo", "regla clara", "debe funcionar", "tiene que funcionar") {
		add("AXIOM", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "axiom", Text: summarize(text), Time: now})
	}
	if containsAny(lower, "improvis", "no se", "me cuesta", "me aqueja", "pensar mucho", "reunion", "mostrar") {
		add("HUMAN_PAIN", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "human", Text: summarize(text), Time: now})
	}
	if containsAny(lower, "flujo", "alfa", "echo", "bravo", "handoff", "demo", "demostr") {
		add("FLOW_EXPECTATION", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "flow", Text: summarize(text), Time: now})
	}
	if containsAny(lower, "debe", "tiene que", "necesito que", "resultado", "funcionar", "excel", "gmail", "whatsapp") {
		add("EXPECTED_RESULT", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "done", Text: summarize(text), Time: now})
	}
	if containsAny(lower, "no funciona", "bloque", "falla", "falt", "error", "riesgo") {
		add("BLOCKER", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "blocker", Text: summarize(text), Time: now})
	}
	if created == 0 {
		add("OBSERVATION", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "decision", Text: summarize(text), Time: now})
	}
	return created
}

func nextQuestion(plan DayPlan) string {
	if !hasNode(plan, "HUMAN_PAIN") {
		return "Que es lo que mas te esta impidiendo avanzar hoy sin improvisar?"
	}
	if !hasNode(plan, "FLOW_EXPECTATION") {
		return "Para la demo de hoy, que debe ocurrir dentro del flujo Alfa-Echo-Bravo para que sientas que funciona de verdad?"
	}
	if !hasNode(plan, "EXPECTED_RESULT") {
		return "Cual es el resultado observable que necesitas mostrar al final del dia?"
	}
	if !hasNode(plan, "BLOCKER") {
		return "Que parte del flujo actual sospechas que no va a sostener esa demo?"
	}
	return "Ya hay suficiente foco para ejecutar. Anota solo bloqueos nuevos o decisiones que cambien el alcance."
}

func hasNode(plan DayPlan, nodeType string) bool {
	for _, node := range plan.Nodes {
		if node.Type == nodeType {
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

func nextNodeID(nodes []Node, nodeType string) string {
	prefix := map[string]string{
		"AXIOM":            "ax",
		"CONTEXT":          "ctx",
		"HUMAN_PAIN":       "hp",
		"FLOW_EXPECTATION": "fl",
		"EXPECTED_RESULT":  "er",
		"BLOCKER":          "bl",
		"OBSERVATION":      "ob",
	}[nodeType]
	if prefix == "" {
		prefix = "nd"
	}
	count := 1
	for _, node := range nodes {
		if strings.HasPrefix(node.ID, prefix+"_") {
			count++
		}
	}
	return fmt.Sprintf("%s_%03d", prefix, count)
}

func summarize(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= 180 {
		return text
	}
	return strings.TrimSpace(text[:177]) + "..."
}

type readinessReport struct {
	Ready      bool
	NextAction string
	Missing    []string
}

type priorityReport struct {
	TaskID   string
	Title    string
	Decision string
	Why      string
	NotNow   string
}

func assessReadiness(plan DayPlan) readinessReport {
	var missing []string
	if !hasNode(plan, "HUMAN_PAIN") {
		missing = append(missing, "dolor humano/prioridad del dia")
	}
	if !hasNode(plan, "FLOW_EXPECTATION") {
		missing = append(missing, "expectativa del flujo Alfa-Echo-Bravo")
	}
	if !hasNode(plan, "EXPECTED_RESULT") {
		missing = append(missing, "resultado observable de la version")
	}
	if len(missing) > 0 {
		return readinessReport{Ready: false, NextAction: nextQuestion(plan), Missing: missing}
	}
	if task, ok := nextTask(plan); ok {
		return readinessReport{
			Ready:      true,
			NextAction: fmt.Sprintf("Ahora toca %s (%s)", task.Title, task.ID),
			Missing:    nil,
		}
	}
	return readinessReport{
		Ready:      true,
		NextAction: "No hay tareas pendientes. Confirmar si se cierra la sesion de trabajo o si aparece nuevo alcance.",
		Missing:    nil,
	}
}

func priorityDecision(plan DayPlan) priorityReport {
	// Regla dura para v0.1.5: si el contrato Echo-Alfa no sostiene el axioma,
	// no se avanza a demo parcial ni a documentacion.
	if taskByID(plan, "task_003").Status == "todo" {
		return priorityReport{
			TaskID:   "task_003",
			Title:    "Corregir que Echo formule las preguntas pendientes de Alfa",
			Decision: "Implementar o dirigir la correccion del contrato Echo-Alfa para que el humano siga hablando con Echo.",
			Why:      "Bloquea el axioma: Echo debe entregar contexto en maximo 2 preguntas y luego formular las preguntas que Alfa necesite para obtener recursos tangibles.",
			NotNow:   "No hacer demo parcial ni documentacion de cierre mientras Echo no pueda canalizar preguntas de Alfa.",
		}
	}
	if taskByID(plan, "task_002").Status == "todo" {
		return priorityReport{
			TaskID:   "task_002",
			Title:    "Corregir que Alfa lea el resultado de Echo antes de preguntar",
			Decision: "Hacer que Alfa use el resultado de Echo/readiness para formular preguntas nuevas orientadas a recursos tangibles.",
			Why:      "Si Alfa no usa el arbol de Echo, repite preguntas y rompe el flujo de la version.",
			NotNow:   "No avanzar a Bravo ni a Excel hasta que Alfa deje de repetir preguntas ya cubiertas por Echo.",
		}
	}
	if taskByID(plan, "task_004").Status == "todo" {
		return priorityReport{
			TaskID:   "task_004",
			Title:    "Ejecutar una demo minima Gmail o WhatsApp hacia Excel",
			Decision: "Ejecutar la demo tangible solo despues de que el contrato Echo-Alfa no contradiga el axioma.",
			Why:      "La version necesita resultado visible, pero no debe saltarse el flujo Alfa-Echo-Bravo.",
			NotNow:   "No introducir arquitectura nueva.",
		}
	}
	if task, ok := nextTask(plan); ok {
		return priorityReport{
			TaskID:   task.ID,
			Title:    task.Title,
			Decision: "Ejecutar la siguiente tarea pendiente del checklist.",
			Why:      task.Why,
			NotNow:   "No abrir alcance nuevo.",
		}
	}
	return priorityReport{
		TaskID:   "none",
		Title:    "Sin tareas pendientes",
		Decision: "Preguntar si queda alcance nuevo; si no, cerrar la sesion de trabajo.",
		Why:      "El checklist no tiene tareas pendientes.",
		NotNow:   "No inventar trabajo.",
	}
}

func taskByID(plan DayPlan, id string) Task {
	for _, task := range plan.Tasks {
		if task.ID == id {
			return task
		}
	}
	return Task{}
}

func defaultTasks(now string) []Task {
	return []Task{
		{
			ID:        "task_001",
			Title:     "Verificar el contrato Echo-Alfa con evidencia real",
			Why:       "El bloqueo principal es que Alfa repite preguntas y no parece leer correctamente el arbol de Echo.",
			Expected:  "Queda escrito que archivo/comando produce Echo, que debe leer Alfa y donde se observa la repeticion o ceguera.",
			Status:    "todo",
			CreatedAt: now,
		},
		{
			ID:        "task_002",
			Title:     "Corregir que Alfa lea el resultado de Echo antes de preguntar",
			Why:       "Alfa debe preguntar por gaps del MERE usando el arbol, no repetir preguntas iniciales de Echo.",
			Expected:  "Alfa usa el resultado de Echo y genera una pregunta nueva orientada a recurso tangible.",
			Status:    "todo",
			CreatedAt: now,
		},
		{
			ID:        "task_003",
			Title:     "Corregir que Echo formule las preguntas pendientes de Alfa",
			Why:       "El humano debe hablar con Echo; Alfa deja resultado o pregunta pendiente como artefacto.",
			Expected:  "Cuando Alfa necesita preguntar, Echo detecta la pregunta pendiente y se la hace al humano.",
			Status:    "todo",
			CreatedAt: now,
		},
		{
			ID:        "task_004",
			Title:     "Ejecutar una demo minima Gmail o WhatsApp hacia Excel",
			Why:       "La version v0.1.5 necesita resultado tangible, no solo correcciones internas.",
			Expected:  "Un comando o flujo toma una entrada de Gmail/WhatsApp y escribe un resultado en Excel.",
			Status:    "todo",
			CreatedAt: now,
		},
		{
			ID:        "task_005",
			Title:     "Cerrar con evidencia para Charlie",
			Why:       "La version diaria necesita changelog y commit propuesto sin reconstruir mentalmente lo ocurrido.",
			Expected:  "Foco muestra tareas realizadas/bloqueadas y Charlie puede generar propuesta de version.",
			Status:    "todo",
			CreatedAt: now,
		},
	}
}

func ensureTasks(plan *DayPlan) bool {
	if len(plan.Tasks) > 0 {
		return false
	}
	plan.Tasks = defaultTasks(time.Now().Format(time.RFC3339))
	return true
}

func nextTask(plan DayPlan) (Task, bool) {
	for _, task := range plan.Tasks {
		if task.Status == "" || task.Status == "todo" {
			return task, true
		}
	}
	return Task{}, false
}

func parseArgs(args []string) map[string]string {
	params := map[string]string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			params[key] = args[i+1]
			i++
		} else {
			params[key] = "true"
		}
	}
	return params
}

func load() (DayPlan, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return DayPlan{}, fmt.Errorf("no hay catastro diario; ejecuta foco init primero")
	}
	var plan DayPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return DayPlan{}, err
	}
	return plan, nil
}

func save(plan DayPlan) error {
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return err
	}
	return os.WriteFile(mdPath, []byte(formatMarkdown(plan)), 0644)
}

func formatMarkdown(plan DayPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Foco Diario - %s\n\n", plan.Date)
	fmt.Fprintf(&b, "- Version objetivo: `%s`\n", plan.Version)
	fmt.Fprintf(&b, "- Objetivo: %s\n\n", plan.Objective)
	groups := []string{"why", "human", "flow", "blocker", "decision", "done"}
	titles := map[string]string{
		"why":      "Why De La Version",
		"axiom":    "Axiomas",
		"human":    "Dolores Humanos",
		"flow":     "Fallas O Reglas De Flujo",
		"blocker":  "Bloqueos",
		"decision": "Decisiones",
		"done":     "Criterios De Termino",
	}
	groups = []string{"why", "axiom", "human", "flow", "blocker", "decision", "done"}
	for _, group := range groups {
		items := filterNotes(plan.Notes, group)
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", titles[group])
		for _, note := range items {
			fmt.Fprintf(&b, "- %s\n", note.Text)
		}
		b.WriteString("\n")
	}
	if len(plan.Nodes) > 0 {
		b.WriteString("## Arbol De Foco\n\n")
		for _, node := range plan.Nodes {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", node.ID, node.Type, node.Title)
		}
		b.WriteString("\n")
	}
	if len(plan.Tasks) > 0 {
		b.WriteString("## Checklist De Ejecucion\n\n")
		for _, task := range plan.Tasks {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", task.Status, task.ID, task.Title)
			if task.BlockedReason != "" {
				fmt.Fprintf(&b, "  - bloqueo: %s\n", task.BlockedReason)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatTasks(plan DayPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Checklist %s %s\n\n", plan.Version, plan.Date)
	for _, task := range plan.Tasks {
		fmt.Fprintf(&b, "[%s] %s - %s\n", task.Status, task.ID, task.Title)
		fmt.Fprintf(&b, "  why: %s\n", task.Why)
		fmt.Fprintf(&b, "  esperado: %s\n", task.Expected)
		if task.BlockedReason != "" {
			fmt.Fprintf(&b, "  bloqueo: %s\n", task.BlockedReason)
		}
	}
	return b.String()
}

func formatTree(plan DayPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Foco %s %s\n\n", plan.Version, plan.Date)
	order := []string{"CONTEXT", "AXIOM", "HUMAN_PAIN", "FLOW_EXPECTATION", "EXPECTED_RESULT", "BLOCKER", "OBSERVATION"}
	for _, nodeType := range order {
		nodes := filterNodes(plan.Nodes, nodeType)
		if len(nodes) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\n", nodeType)
		for _, node := range nodes {
			fmt.Fprintf(&b, "  [%s] %s\n", node.ID, node.Title)
			for _, evidence := range node.Evidence {
				fmt.Fprintf(&b, "    evidence: %s\n", evidence)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func filterNodes(nodes []Node, nodeType string) []Node {
	var result []Node
	for _, node := range nodes {
		if node.Type == nodeType {
			result = append(result, node)
		}
	}
	return result
}

func filterNotes(notes []Note, kind string) []Note {
	var result []Note
	for _, note := range notes {
		if note.Kind == kind {
			result = append(result, note)
		}
	}
	return result
}

func usage() {
	fmt.Println(`Foco CLI

USO:
  foco init --version v0.1.5 --objective "objetivo del dia"
  foco ask
  foco answer --text "respuesta libre"
  foco axiom --text "regla no negociable" --evidence "de donde sale"
  foco note --kind human --text "dolor humano"
  foco note --kind flow --text "falla o regla de flujo"
  foco note --kind blocker --text "bloqueo"
  foco note --kind done --text "criterio de termino"
  foco tree
  foco readiness
  foco plan
  foco priority
  foco next
  foco done --id task_001 --evidence "resultado demostrado"
  foco block --id task_001 --reason "bloqueo concreto"
  foco show`)
}

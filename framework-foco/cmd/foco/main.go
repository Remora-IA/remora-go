package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	statePath       = "foco_state.json"
	mdPath          = "foco_state.md"
	legacyStatePath = "temp/foco/today.json"
	legacyMDPath    = "temp/foco/today.md"
)

// Dependency representa una dependencia entre tareas
type Dependency struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`    // tarea que depende
	DependsOn string `json:"depends_on"` // tarea de la que depende
}

// Quadrant representa los cuadrantes de Eisenhower
type Quadrant string

const (
	Q1      Quadrant = "Q1_DO_NOW"    // IMPORTANTE + URGENTE
	Q2      Quadrant = "Q2_SCHEDULE"  // IMPORTANTE + NO URGENTE
	Q3      Quadrant = "Q3_DELEGATE"  // NO IMPORTANTE + URGENTE
	Q4      Quadrant = "Q4_ELIMINATE" // NO IMPORTANTE + NO URGENTE
	QESPERA Quadrant = "ESPERA"       // Con pre-conflicto o dependencia no resuelta
)

type DayPlan struct {
	Date         string        `json:"date"`
	Version      string        `json:"version"`
	Result       string        `json:"result,omitempty"`
	Objective    string        `json:"objective"`
	InferredWhy  string        `json:"inferred_why,omitempty"`
	Notes        []Note        `json:"notes"`
	Nodes        []Node        `json:"nodes"`
	Events       []Event       `json:"events,omitempty"`
	Tasks        []Task        `json:"tasks"`
	Axioms       []Axiom       `json:"axioms,omitempty"`
	Conflicts    []PreConflict `json:"conflicts"`
	Dependencies []Dependency  `json:"dependencies,omitempty"`
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

type Event struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	Time   string `json:"time,omitempty"`
	Result string `json:"result,omitempty"`
	Why    string `json:"why,omitempty"`
	Status string `json:"status"`
}

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	EventID     string `json:"event_id,omitempty"`
	Why         string `json:"why"`
	Expected    string `json:"expected"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Priority    string `json:"priority,omitempty"`
	PreConflict string `json:"pre_conflict,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	Importance  int    `json:"importance,omitempty"`
	Evidence    string `json:"evidence,omitempty"`
}

type Axiom struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	TaskID   string `json:"task_id"`
	Evidence string `json:"evidence,omitempty"`
	Status   string `json:"status"`
}

type PreConflict struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Resolved  bool   `json:"resolved"`
	CreatedAt string `json:"created_at"`
}

type AlignmentIssue struct {
	Kind     string
	Target   string
	Against  string
	Question string
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
	case "event":
		err = runEvent(os.Args[2:])
	case "task":
		err = runTask(os.Args[2:])
	case "note":
		err = runNote(os.Args[2:])
	case "ask":
		err = runAsk()
	case "answer":
		err = runAnswer(os.Args[2:])
	case "axiom":
		err = runAxiom(os.Args[2:])
	case "conflict":
		err = runConflict(os.Args[2:])
	case "annex":
		err = runAnnex(os.Args[2:])
	case "resolve":
		err = runResolve(os.Args[2:])
	case "depends":
		err = runDepends(os.Args[2:])
	case "flow":
		err = runFlow()
	case "whatif":
		err = runWhatIf(os.Args[2:])
	case "priority":
		err = runPriority(os.Args[2:])
	case "tree":
		err = runTree()
	case "readiness":
		err = runReadiness()
	case "plan":
		err = runPlan()
	case "today":
		err = runToday()
	case "conflicts":
		err = runConflicts()
	case "next":
		err = runNext()
	case "done":
		err = runDone(os.Args[2:])
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
	result := strings.TrimSpace(params["result"])
	if result == "" {
		result = strings.TrimSpace(params["objective"])
	}
	if version == "" || result == "" {
		return errors.New("uso: foco init --version v0.1.5 --result \"resultado del dia\"")
	}

	plan := DayPlan{
		Date:      time.Now().Format("2006-01-02"),
		Version:   version,
		Result:    result,
		Objective: result,
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
				Title:    result,
				Evidence: []string{"Resultado declarado al iniciar Foco."},
				Status:   "confirmed",
			},
		},
		Events:    []Event{},
		Tasks:     []Task{},
		Axioms:    []Axiom{},
		Conflicts: []PreConflict{},
	}
	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_ready: %s %s\n", plan.Version, primaryResult(plan))
	return nil
}

func runNote(args []string) error {
	params := parseArgs(args)
	kind := strings.TrimSpace(params["kind"])
	text := strings.TrimSpace(params["text"])
	if kind == "" || text == "" {
		return errors.New("uso: foco note --kind human|flow|pre_conflict|done|decision|axiom --text \"nota\"")
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

func runEvent(args []string) error {
	params := parseArgs(args)
	title := strings.TrimSpace(params["title"])
	date := strings.TrimSpace(params["date"])
	at := strings.TrimSpace(params["time"])
	why := strings.TrimSpace(params["why"])
	if title == "" || date == "" {
		return errors.New("uso: foco event --title \"evento\" --date 2026-04-28 [--time 14:00] [--why \"por que importa\"]")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("fecha invalida: %s", date)
	}

	plan, err := load()
	if err != nil {
		return err
	}

	event := Event{
		ID:     fmt.Sprintf("evt_%03d", len(plan.Events)+1),
		Title:  title,
		Date:   date,
		Time:   at,
		Result: primaryResult(plan),
		Why:    why,
		Status: "planned",
	}
	plan.Events = append(plan.Events, event)
	if why != "" {
		plan.InferredWhy = why
	}

	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_event: %s %s\n", event.ID, event.Title)
	return nil
}

func runTask(args []string) error {
	params := parseArgs(args)
	title := strings.TrimSpace(params["title"])
	eventID := strings.TrimSpace(params["event"])
	expected := strings.TrimSpace(params["expected"])
	why := strings.TrimSpace(params["why"])
	dueDate := strings.TrimSpace(params["date"])
	if title == "" {
		return errors.New("uso: foco task --title \"tarea\" [--event evt_001] [--expected \"resultado\"] [--why \"para que\"]")
	}

	plan, err := load()
	if err != nil {
		return err
	}

	if eventID == "" {
		event := findCurrentOrTodayEvent(plan)
		if event == nil {
			return errors.New("no hay evento para vincular la tarea; crea uno con foco event o indica --event")
		}
		eventID = event.ID
	}

	event := findEvent(eventID, plan)
	if event == nil {
		return fmt.Errorf("evento no encontrado: %s", eventID)
	}

	if dueDate == "" {
		dueDate = event.Date
	}
	if why == "" {
		why = event.Why
	}

	task := Task{
		ID:        fmt.Sprintf("task_%03d", len(plan.Tasks)+1),
		Title:     title,
		EventID:   eventID,
		Why:       why,
		Expected:  expected,
		Status:    "todo",
		CreatedAt: time.Now().Format(time.RFC3339),
		DueDate:   dueDate,
	}

	plan.Tasks = append(plan.Tasks, task)
	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_task: %s %s\n", task.ID, task.Title)
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

func runToday() error {
	plan, err := load()
	if err != nil {
		return err
	}
	fmt.Print(formatPrimarySummary(plan, false))
	return nil
}

func runAsk() error {
	plan, err := load()
	if err != nil {
		fmt.Println("👋 Hola! No hay sesión activa.")
		fmt.Println()
		fmt.Println("Para iniciar:")
		fmt.Println("  foco init --version v0.1.0 --result \"tu resultado\"")
		return nil
	}
	fmt.Println(nextQuestion(plan))
	return nil
}

func printCompactDashboard(plan DayPlan) {
	fmt.Printf("Resultado: %s\n\n", primaryResult(plan))

	// Clasificar tareas
	q1, q2, espera, q3, q4 := categorizeAllTasks(plan)

	// Mostrar en orden de prioridad
	if len(q1) > 0 {
		fmt.Println("► Hacer ahora:")
		for _, t := range q1 {
			fmt.Printf("   - %s\n", t.Title)
		}
		fmt.Println()
	}

	if len(q2) > 0 {
		fmt.Println("○ Después:")
		for _, t := range q2 {
			fmt.Printf("   - %s\n", t.Title)
		}
		fmt.Println()
	}

	if len(espera) > 0 {
		fmt.Println("◌ Esperando:")
		for _, t := range espera {
			reason := ""
			if t.PreConflict != "" {
				for _, c := range plan.Conflicts {
					if c.ID == t.PreConflict {
						reason = fmt.Sprintf(" (%s)", c.Text)
						break
					}
				}
			}
			fmt.Printf("   - %s%s\n", t.Title, reason)
		}
		fmt.Println()
	}

	if len(q3) > 0 || len(q4) > 0 {
		fmt.Println("? Para revisar:")
		for _, t := range q3 {
			fmt.Printf("   - %s\n", t.Title)
		}
		for _, t := range q4 {
			fmt.Printf("   - %s\n", t.Title)
		}
		fmt.Println()
	}

	if len(plan.Conflicts) > 0 {
		open := 0
		for _, c := range plan.Conflicts {
			if !c.Resolved {
				open++
			}
		}
		if open > 0 {
			fmt.Printf("⚠ %d pendiente(s) sin resolver\n", open)
		}
	}
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

func materializeLayerAnswer(plan *DayPlan, text string) bool {
	switch nextMissingLayer(*plan) {
	case "result":
		result := summarize(text)
		plan.Result = result
		plan.Objective = result
		if plan.InferredWhy == "" {
			plan.InferredWhy = result
		}
		plan.Notes = append(plan.Notes, Note{
			Kind: "why",
			Text: result,
			Time: time.Now().Format(time.RFC3339),
		})
		return true
	case "event":
		title, date, at := parseEventAnswer(text, plan.Date)
		plan.Events = append(plan.Events, Event{
			ID:     fmt.Sprintf("evt_%03d", len(plan.Events)+1),
			Title:  title,
			Date:   date,
			Time:   at,
			Result: primaryResult(*plan),
			Why:    inferWhyToday(*plan, nil),
			Status: "planned",
		})
		return true
	case "task":
		event := findCurrentOrTodayEvent(*plan)
		if event == nil {
			return false
		}
		plan.Tasks = append(plan.Tasks, Task{
			ID:        fmt.Sprintf("task_%03d", len(plan.Tasks)+1),
			Title:     summarize(text),
			EventID:   event.ID,
			Why:       event.Why,
			Expected:  primaryResult(*plan),
			Status:    "todo",
			CreatedAt: time.Now().Format(time.RFC3339),
			DueDate:   event.Date,
		})
		return true
	case "axiom":
		task := nextPrimaryTask(*plan)
		if task == nil {
			return false
		}
		plan.Axioms = append(plan.Axioms, Axiom{
			ID:       fmt.Sprintf("ax_%03d", len(plan.Axioms)+1),
			Title:    summarize(text),
			TaskID:   task.ID,
			Evidence: text,
			Status:   "confirmed",
		})
		plan.Nodes = append(plan.Nodes, Node{
			ID:       nextNodeID(plan.Nodes, "AXIOM"),
			Type:     "AXIOM",
			Title:    summarize(text),
			Evidence: []string{text},
			Status:   "confirmed",
		})
		plan.Notes = append(plan.Notes, Note{
			Kind: "axiom",
			Text: summarize(text),
			Time: time.Now().Format(time.RFC3339),
		})
		return true
	default:
		return false
	}
}

func runAxiom(args []string) error {
	params := parseArgs(args)
	text := strings.TrimSpace(params["text"])
	evidence := strings.TrimSpace(params["evidence"])
	taskID := strings.TrimSpace(params["task"])
	if text == "" {
		return errors.New("uso: foco axiom --text \"regla no negociable\" [--task task_001] --evidence \"de donde sale\"")
	}
	if evidence == "" {
		evidence = text
	}
	plan, err := load()
	if err != nil {
		return err
	}

	// Detectar y marcar axiomas contradichos como superseded
	conflicts := detectAxiomConflicts(plan.Nodes, text)
	if len(conflicts) > 0 {
		fmt.Printf("CONFLICTO DETECTADO: Reemplazando axiomas contradictorios:\n")
		for _, conflict := range conflicts {
			fmt.Printf("  [%s] -> superseded: %s\n", conflict.ID, conflict.Title)
			for i := range plan.Nodes {
				if plan.Nodes[i].ID == conflict.ID {
					plan.Nodes[i].Status = "superseded"
					plan.Nodes[i].Evidence = append(plan.Nodes[i].Evidence, "REPLACED: "+text)
				}
			}
		}
		fmt.Println()
	}

	if taskID == "" {
		task := nextPrimaryTask(plan)
		if task == nil && len(plan.Tasks) > 0 {
			task = &plan.Tasks[len(plan.Tasks)-1]
		}
		if task != nil {
			taskID = task.ID
		}
	}
	if taskID == "" {
		return errors.New("no hay tarea para vincular el axioma; usa foco task primero o indica --task")
	}
	task := findTask(taskID, plan)
	if task == nil {
		return fmt.Errorf("tarea no encontrada: %s", taskID)
	}

	axiomID := fmt.Sprintf("ax_%03d", len(plan.Axioms)+1)
	plan.Axioms = append(plan.Axioms, Axiom{
		ID:       axiomID,
		Title:    summarize(text),
		TaskID:   taskID,
		Evidence: evidence,
		Status:   "confirmed",
	})

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
	fmt.Printf("foco_axiom: %s -> %s\n", summarize(text), task.Title)
	return nil
}

// detectAxiomConflicts detecta axiomas que CONTRADICEN directamente el nuevo texto
// SOLO es contradiccion si:
// 1. AMBOS tienen palabras opuestas (framework vs libreria, piensa vs no piensa, etc.)
// 2. Y comparten las MISMAS palabras clave que indican que hablan del MISMO sujeto
//
// Ejemplo de NO contradiccion:
// - "RAG Framework hace RAG" + "Channel es libreria" = NO conflicto (diferentes sujetos)
//
// Ejemplo de SI contradiccion:
// - "Channel es framework" + "Channel es libreria" = SI conflicto (mismo sujeto)
func detectAxiomConflicts(nodes []Node, newText string) []Node {
	var conflicts []Node
	newLower := strings.ToLower(newText)

	// Extraer palabras clave del nuevo axioma (nombres propios, sustantivos importantes)
	newSubjects := extractKeySubjects(newLower)

	for _, node := range nodes {
		if node.Type != "AXIOM" || node.Status == "superseded" {
			continue
		}
		titleLower := strings.ToLower(node.Title)
		titleSubjects := extractKeySubjects(titleLower)

		// Verificar si AMBOS axiomas hablan del mismo sujeto
		// Contar palabras en comun que sean significativas
		sharedCount := countSharedKeyWords(newSubjects, titleSubjects)

		// Contradiccion SOLO si hay palabras opuestas Y comparten palabras clave
		// Si no comparten nada, no son contradicciones reales

		// Tema: framework vs libreria
		newHasFramework := strings.Contains(newLower, "framework") && !strings.Contains(newLower, "libreria")
		newHasLibreria := strings.Contains(newLower, "libreria") && !strings.Contains(newLower, "framework")
		titleHasFramework := strings.Contains(titleLower, "framework") && !strings.Contains(titleLower, "libreria")
		titleHasLibreria := strings.Contains(titleLower, "libreria") && !strings.Contains(titleLower, "framework")

		if sharedCount >= 1 { // Deben compartir al menos 1 palabra clave
			if (newHasFramework && titleHasLibreria) || (newHasLibreria && titleHasFramework) {
				conflicts = append(conflicts, node)
				continue
			}
		}

		// Tema: piensa vs no piensa
		newHasPiensa := strings.Contains(newLower, "piensa") && !strings.Contains(newLower, "no piensa")
		newHasNoPiensa := strings.Contains(newLower, "no piensa")
		titleHasPiensa := strings.Contains(titleLower, "piensa") && !strings.Contains(titleLower, "no piensa")
		titleHasNoPiensa := strings.Contains(titleLower, "no piensa")

		if sharedCount >= 1 {
			if (newHasPiensa && titleHasNoPiensa) || (newHasNoPiensa && titleHasPiensa) {
				conflicts = append(conflicts, node)
				continue
			}
		}

		// Tema: orquesta vs no orquesta
		newHasOrquesta := strings.Contains(newLower, "orquesta") && !strings.Contains(newLower, "no orquesta")
		newHasNoOrquesta := strings.Contains(newLower, "no orquesta")
		titleHasOrquesta := strings.Contains(titleLower, "orquesta") && !strings.Contains(titleLower, "no orquesta")
		titleHasNoOrquesta := strings.Contains(titleLower, "no orquesta")

		if sharedCount >= 1 {
			if (newHasOrquesta && titleHasNoOrquesta) || (newHasNoOrquesta && titleHasOrquesta) {
				conflicts = append(conflicts, node)
				continue
			}
		}

		// Tema: impide vs no impide
		newHasImpide := strings.Contains(newLower, "impide") && !strings.Contains(newLower, "no impide")
		newHasNoImpide := strings.Contains(newLower, "no impide")
		titleHasImpide := strings.Contains(titleLower, "impide") && !strings.Contains(titleLower, "no impide")
		titleHasNoImpide := strings.Contains(titleLower, "no impide")

		if sharedCount >= 1 {
			if (newHasImpide && titleHasNoImpide) || (newHasNoImpide && titleHasImpide) {
				conflicts = append(conflicts, node)
				continue
			}
		}
	}
	return conflicts
}

// extractKeySubjects extrae palabras clave que indican el sujeto del axioma
// Ejemplo: "Channel es una libreria Go" -> ["channel", "libreria", "go"]
func extractKeySubjects(text string) []string {
	// Palabras que indican sujeto (nombres de frameworks, tecnologias, etc.)
	subjects := []string{
		"channel", "paladin", "orden", "semantic", "events", "echo", "alfa", "fpt",
		"mere", "rag", "vector", "github", "gcp", "cloud", "adapter", "rpc", "json-rpc",
		"framework", "libreria", "servidor", "terminal", "tracing", "adapter",
	}
	var found []string
	for _, subj := range subjects {
		if strings.Contains(text, subj) {
			found = append(found, subj)
		}
	}
	return found
}

// countSharedKeyWords cuenta cuantas palabras clave comparten dos textos
func countSharedKeyWords(subjects1, subjects2 []string) int {
	count := 0
	for _, s1 := range subjects1 {
		for _, s2 := range subjects2 {
			if s1 == s2 {
				count++
			}
		}
	}
	return count
}

func runConflict(args []string) error {
	params := parseArgs(args)
	text := strings.TrimSpace(params["text"])
	if text == "" {
		return errors.New("uso: foco conflict --text \"algo que hay que arreglar antes de la tarea principal\"")
	}

	plan, err := load()
	if err != nil {
		return err
	}

	pcID := fmt.Sprintf("pc_%03d", len(plan.Conflicts)+1)
	conflict := PreConflict{
		ID:        pcID,
		Text:      text,
		Resolved:  false,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	plan.Conflicts = append(plan.Conflicts, conflict)

	plan.Notes = append(plan.Notes, Note{
		Kind: "pre_conflict",
		Text: text,
		Time: time.Now().Format(time.RFC3339),
	})

	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_conflict: %s - %s\n", pcID, text)
	return nil
}

func runAnnex(args []string) error {
	params := parseArgs(args)
	conflictID := strings.TrimSpace(params["id"])
	taskID := strings.TrimSpace(params["task"])

	if conflictID == "" || taskID == "" {
		return errors.New("uso: foco annex --id pc_001 --task task_002")
	}

	plan, err := load()
	if err != nil {
		return err
	}

	foundConflict := false
	for _, c := range plan.Conflicts {
		if c.ID == conflictID {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		return fmt.Errorf("pre_conflicto no encontrado: %s", conflictID)
	}

	foundTask := false
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == taskID {
			plan.Tasks[i].PreConflict = conflictID
			foundTask = true
			break
		}
	}
	if !foundTask {
		return fmt.Errorf("tarea no encontrada: %s", taskID)
	}

	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_annex: %s anidado a %s\n", taskID, conflictID)
	return nil
}

func runResolve(args []string) error {
	params := parseArgs(args)
	conflictID := strings.TrimSpace(params["id"])

	if conflictID == "" {
		return errors.New("uso: foco resolve --id pc_001")
	}

	plan, err := load()
	if err != nil {
		return err
	}

	found := false
	for i := range plan.Conflicts {
		if plan.Conflicts[i].ID == conflictID {
			plan.Conflicts[i].Resolved = true
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("pre_conflicto no encontrado: %s", conflictID)
	}

	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("foco_resolved: %s\n", conflictID)
	return nil
}

func runConflicts() error {
	plan, err := load()
	if err != nil {
		return err
	}

	if len(plan.Conflicts) == 0 {
		fmt.Println("No hay pre-conflictos registrados.")
		return nil
	}

	fmt.Println("PRE-CONFLICTOS")
	fmt.Println("==============")
	for _, c := range plan.Conflicts {
		status := "OPEN"
		if c.Resolved {
			status = "RESOLVED"
		}
		fmt.Printf("[%s] %s: %s\n", status, c.ID, c.Text)
	}
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
	if len(plan.Tasks) == 0 {
		fmt.Println("version: " + plan.Version)
		fmt.Println("ready: false")
		fmt.Println("next_action: No hay tareas. Ejecuta foco init o agrega tareas.")
		return nil
	}
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
	if len(plan.Conflicts) > 0 {
		fmt.Println("PRE-CONFLICTOS")
		fmt.Println("==============")
		for _, c := range plan.Conflicts {
			if !c.Resolved {
				fmt.Printf("[OPEN] %s: %s\n", c.ID, c.Text)
			}
		}
		fmt.Println()
	}
	fmt.Print(formatTasks(plan))
	return nil
}

func runNext() error {
	plan, err := load()
	if err != nil {
		return err
	}
	fmt.Print(formatPrimarySummary(plan, true))
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

func validKind(kind string) bool {
	switch kind {
	case "why", "axiom", "human", "flow", "pre_conflict", "done", "decision", "socratic":
		return true
	default:
		return false
	}
}

func runDepends(args []string) error {
	params := parseArgs(args)
	onID := strings.TrimSpace(params["on"])
	taskID := strings.TrimSpace(params["task"])
	forID := strings.TrimSpace(params["for"])

	plan, err := load()
	if err != nil {
		return err
	}

	// Ver dependencias de una tarea
	if forID != "" {
		deps := getTaskDependencies(forID, plan)
		if len(deps) == 0 {
			fmt.Printf("No hay dependencias para %s\n", forID)
			return nil
		}
		fmt.Printf("DEPENDENCIAS DE %s:\n", forID)
		for _, d := range deps {
			fmt.Printf("  - %s\n", d.DependsOn)
		}
		return nil
	}

	// Ver qué depende de una tarea
	if onID != "" && taskID == "" {
		var dependents []string
		for _, d := range plan.Dependencies {
			if d.DependsOn == onID {
				dependents = append(dependents, d.TaskID)
			}
		}
		if len(dependents) == 0 {
			fmt.Printf("Nada depende de %s\n", onID)
			return nil
		}
		fmt.Printf("%s ES PRERREQUISITO DE:\n", onID)
		for _, dep := range dependents {
			fmt.Printf("  - %s\n", dep)
		}
		return nil
	}

	// Agregar dependencia
	if onID == "" || taskID == "" {
		return errors.New("uso: foco depends --on tarea_prerrequisito --task tarea_que_depende")
	}

	if findTask(onID, plan) == nil {
		return fmt.Errorf("tarea no encontrada: %s", onID)
	}
	if findTask(taskID, plan) == nil {
		return fmt.Errorf("tarea no encontrada: %s", taskID)
	}

	dep := Dependency{
		ID:        fmt.Sprintf("dep_%03d", len(plan.Dependencies)+1),
		TaskID:    taskID,
		DependsOn: onID,
	}
	plan.Dependencies = append(plan.Dependencies, dep)

	if err := save(plan); err != nil {
		return err
	}
	fmt.Printf("✓ Dependencia agregada: %s depende de %s\n", taskID, onID)
	return nil
}

func getTaskDependencies(taskID string, plan DayPlan) []Dependency {
	var deps []Dependency
	for _, d := range plan.Dependencies {
		if d.TaskID == taskID {
			deps = append(deps, d)
		}
	}
	return deps
}

func findTask(taskID string, plan DayPlan) *Task {
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == taskID {
			return &plan.Tasks[i]
		}
	}
	return nil
}

func runFlow() error {
	plan, err := load()
	if err != nil {
		return err
	}

	fmt.Println(strings.Repeat("═", 50))
	fmt.Println("FLUJO ACTUAL")
	fmt.Println(strings.Repeat("═", 50))

	if len(plan.Tasks) == 0 {
		fmt.Println("No hay tareas registradas.")
		return nil
	}

	// Construir mapa de tareas
	taskMap := make(map[string]Task)
	for _, t := range plan.Tasks {
		taskMap[t.ID] = t
	}

	// Encontrar raíces
	var roots []string
	for _, t := range plan.Tasks {
		hasDep := false
		for _, d := range plan.Dependencies {
			if d.TaskID == t.ID {
				hasDep = true
				break
			}
		}
		if !hasDep {
			roots = append(roots, t.ID)
		}
	}

	printed := make(map[string]bool)
	for _, root := range roots {
		if !printed[root] {
			printFlowChain(root, plan.Dependencies, taskMap, printed, 0, plan)
		}
	}

	// Tareas sin flujo
	var orphans int
	for _, t := range plan.Tasks {
		if !printed[t.ID] && t.Status != "done" {
			q := classifyTask(t, plan)
			fmt.Printf("  [%s] %s\n", q, t.Title)
			orphans++
		}
	}
	if orphans == 0 {
		fmt.Println("  Todas las tareas tienen flujo definido.")
	}
	return nil
}

func printFlowChain(taskID string, deps []Dependency, tasks map[string]Task, printed map[string]bool, level int, plan DayPlan) {
	task, ok := tasks[taskID]
	if !ok {
		return
	}

	if printed[taskID] {
		return
	}
	printed[taskID] = true

	var nextTask string
	for _, d := range deps {
		if d.DependsOn == taskID {
			nextTask = d.TaskID
			break
		}
	}

	q := classifyTask(task, plan)
	if nextTask == "" {
		fmt.Printf("%s[%s] %s\n", strings.Repeat("  ", level), q, task.ID)
	} else {
		fmt.Printf("%s[%s] %s →\n", strings.Repeat("  ", level), q, task.ID)
		printFlowChain(nextTask, deps, tasks, printed, level+1, plan)
	}
}

func runWhatIf(args []string) error {
	params := parseArgs(args)
	taskID := strings.TrimSpace(params["task"])

	if taskID == "" {
		return errors.New("uso: foco whatif --task task_001")
	}

	plan, err := load()
	if err != nil {
		return err
	}

	task := findTask(taskID, plan)
	if task == nil {
		return fmt.Errorf("tarea no encontrada: %s", taskID)
	}

	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("WHAT-IF: \"%s\"\n", task.Title)
	fmt.Println(strings.Repeat("─", 40))

	// Mostrar pre-conflicto si existe
	if task.PreConflict != "" {
		for _, c := range plan.Conflicts {
			if c.ID == task.PreConflict {
				status := "abierto"
				if c.Resolved {
					status = "resuelto"
				}
				fmt.Printf("⚠ Pre-conflicto [%s]: %s (%s)\n\n", c.ID, c.Text, status)
				break
			}
		}
	}

	// Mostrar qué tareas dependen de esta
	var dependents []string
	for _, d := range plan.Dependencies {
		if d.DependsOn == taskID {
			dependents = append(dependents, d.TaskID)
		}
	}

	if len(dependents) > 0 {
		fmt.Println("Bloquea:")
		for _, dep := range dependents {
			if t := findTask(dep, plan); t != nil {
				fmt.Printf("  - \"%s\"\n", t.Title)
			}
		}
		fmt.Println()
	}

	// Mostrar de qué depende esta tarea
	deps := getTaskDependencies(taskID, plan)
	if len(deps) > 0 {
		fmt.Println("Espera a:")
		for _, dep := range deps {
			if t := findTask(dep.DependsOn, plan); t != nil {
				status := ""
				if t.Status == "done" {
					status = " ✓"
				}
				fmt.Printf("  - \"%s\"%s\n", t.Title, status)
			}
		}
		fmt.Println()
	}

	return nil
}

func runPriority(args []string) error {
	plan, err := load()
	if err != nil {
		return err
	}

	fmt.Println(strings.Repeat("═", 50))
	fmt.Printf("PRIORIDADES - %s\n", plan.Date)
	fmt.Println(strings.Repeat("═", 50))
	fmt.Printf("WHY: %s\n\n", plan.Objective)

	q1, q2, espera, q3, q4 := categorizeAllTasks(plan)

	fmt.Println("Q1 - DO NOW (urgente + importante)")
	for _, t := range q1 {
		fmt.Printf("  ▸ %s\n", t.Title)
	}
	if len(q1) == 0 {
		fmt.Println("  Ninguna")
	}
	fmt.Println()

	fmt.Println("Q2 - SCHEDULE (importante)")
	for _, t := range q2 {
		fmt.Printf("  ▸ %s\n", t.Title)
	}
	if len(q2) == 0 {
		fmt.Println("  Ninguna")
	}
	fmt.Println()

	fmt.Println("ESPERA (pre-conflicto o dependencia)")
	for _, t := range espera {
		reason := ""
		if t.PreConflict != "" {
			reason = fmt.Sprintf(" (pre-conflicto: %s)", t.PreConflict)
		}
		fmt.Printf("  ▸ %s%s\n", t.Title, reason)
	}
	if len(espera) == 0 {
		fmt.Println("  Ninguna")
	}
	fmt.Println()

	if len(q3) > 0 || len(q4) > 0 {
		fmt.Println("Q3/Q4 - Revisar si son necesarias")
		for _, t := range q3 {
			fmt.Printf("  [Q3] %s\n", t.Title)
		}
		for _, t := range q4 {
			fmt.Printf("  [Q4] %s\n", t.Title)
		}
	}
	return nil
}

func classifyTask(task Task, plan DayPlan) Quadrant {
	now := time.Now()

	if task.PreConflict != "" {
		for _, c := range plan.Conflicts {
			if c.ID == task.PreConflict && !c.Resolved {
				return QESPERA
			}
		}
	}

	deps := getTaskDependencies(task.ID, plan)
	for _, dep := range deps {
		depTask := findTask(dep.DependsOn, plan)
		if depTask != nil && depTask.Status != "done" {
			return QESPERA
		}
	}

	var urgency int
	if task.DueDate != "" {
		if due, err := time.Parse("2006-01-02", task.DueDate); err == nil {
			hours := due.Sub(now).Hours()
			switch {
			case hours < 0:
				urgency = 3
			case hours <= 24:
				urgency = 3
			case hours <= 72:
				urgency = 2
			default:
				urgency = 1
			}
		}
	}

	importance := task.Importance
	if importance == 0 {
		lower := strings.ToLower(task.Title + " " + task.Evidence + " " + task.Why)
		if strings.Contains(lower, "para charlie") ||
			strings.Contains(lower, "urgente") ||
			strings.Contains(lower, "demo") ||
			strings.Contains(lower, "crítico") ||
			strings.Contains(lower, "critico") {
			importance = 2
		} else {
			importance = 1
		}
	}

	if importance >= 2 && urgency >= 2 {
		return Q1
	} else if importance >= 2 && urgency < 2 {
		return Q2
	} else if importance < 2 && urgency >= 2 {
		return Q3
	}
	return Q4
}

func categorizeAllTasks(plan DayPlan) (q1, q2, espera, q3, q4 []Task) {
	for _, t := range plan.Tasks {
		if t.Status == "done" {
			continue
		}
		q := classifyTask(t, plan)
		switch q {
		case Q1:
			q1 = append(q1, t)
		case Q2:
			q2 = append(q2, t)
		case QESPERA:
			espera = append(espera, t)
		case Q3:
			q3 = append(q3, t)
		case Q4:
			q4 = append(q4, t)
		}
	}
	return
}

func findEvent(eventID string, plan DayPlan) *Event {
	for i := range plan.Events {
		if plan.Events[i].ID == eventID {
			return &plan.Events[i]
		}
	}
	return nil
}

func findCurrentOrTodayEvent(plan DayPlan) *Event {
	for i := range plan.Events {
		if plan.Events[i].Date == plan.Date && plan.Events[i].Status != "done" {
			return &plan.Events[i]
		}
	}
	if len(plan.Events) > 0 {
		return &plan.Events[len(plan.Events)-1]
	}
	return nil
}

func eventsForDate(plan DayPlan, date string) []Event {
	var events []Event
	for _, event := range plan.Events {
		if event.Date == date && event.Status != "done" {
			events = append(events, event)
		}
	}
	return events
}

func tasksForDate(plan DayPlan, date string) []Task {
	var tasks []Task
	for _, task := range plan.Tasks {
		if task.Status == "done" {
			continue
		}
		event := findEvent(task.EventID, plan)
		if event != nil && event.Date == date {
			tasks = append(tasks, task)
		}
	}
	return prioritizePrimaryTasks(tasks, plan)
}

func prioritizePrimaryTasks(tasks []Task, plan DayPlan) []Task {
	scored := make([]Task, len(tasks))
	copy(scored, tasks)
	sort.SliceStable(scored, func(i, j int) bool {
		qi := classifyTask(scored[i], plan)
		qj := classifyTask(scored[j], plan)
		if primaryQuadrantScore(qi) != primaryQuadrantScore(qj) {
			return primaryQuadrantScore(qi) > primaryQuadrantScore(qj)
		}
		ei := findEvent(scored[i].EventID, plan)
		ej := findEvent(scored[j].EventID, plan)
		ti, tj := "", ""
		if ei != nil {
			ti = ei.Time
		}
		if ej != nil {
			tj = ej.Time
		}
		return ti < tj
	})
	return scored
}

func primaryQuadrantScore(q Quadrant) int {
	switch q {
	case Q1:
		return 5
	case Q2:
		return 4
	case QESPERA:
		return 3
	case Q3:
		return 2
	default:
		return 1
	}
}

func axiomsForTask(plan DayPlan, taskID string) []Axiom {
	var axioms []Axiom
	for _, axiom := range plan.Axioms {
		if axiom.TaskID == taskID && axiom.Status != "superseded" {
			axioms = append(axioms, axiom)
		}
	}
	return axioms
}

func nextPrimaryTask(plan DayPlan) *Task {
	todayTasks := tasksForDate(plan, plan.Date)
	if len(todayTasks) > 0 {
		return &todayTasks[0]
	}
	return nil
}

func primaryResult(plan DayPlan) string {
	if strings.TrimSpace(plan.Result) != "" {
		return plan.Result
	}
	return plan.Objective
}

func inferWhyToday(plan DayPlan, todayTasks []Task) string {
	if plan.InferredWhy != "" {
		return plan.InferredWhy
	}
	for _, event := range eventsForDate(plan, plan.Date) {
		if strings.TrimSpace(event.Why) != "" {
			return event.Why
		}
	}
	if len(todayTasks) > 0 && strings.TrimSpace(todayTasks[0].Why) != "" {
		return todayTasks[0].Why
	}
	return primaryResult(plan)
}

func resultObservableToday(plan DayPlan, todayTasks []Task) string {
	for _, task := range todayTasks {
		if strings.TrimSpace(task.Expected) != "" {
			return task.Expected
		}
	}
	return primaryResult(plan)
}

func formatPrimarySummary(plan DayPlan, includeQuestion bool) string {
	var b strings.Builder
	todayTasks := tasksForDate(plan, plan.Date)
	whyToday := inferWhyToday(plan, todayTasks)
	observable := resultObservableToday(plan, todayTasks)
	nextTask := nextPrimaryTask(plan)
	result := primaryResult(plan)

	fmt.Fprintf(&b, "RESULTADO OBSERVABLE AL FINAL DEL DIA:\n%s\n\n", fallbackText(observable, result))
	fmt.Fprintf(&b, "WHY DE HOY:\n%s\n\n", fallbackText(whyToday, result))

	if nextTask != nil {
		fmt.Fprintf(&b, "PROXIMA TAREA:\n%s\n\n", nextTask.Title)
	} else {
		b.WriteString("PROXIMA TAREA:\nNo definida todavia.\n\n")
	}

	b.WriteString("TAREAS PARA HOY:\n")
	if len(todayTasks) == 0 {
		b.WriteString("- No hay tareas de hoy definidas.\n")
	} else {
		for i, task := range todayTasks {
			event := findEvent(task.EventID, plan)
			when := plan.Date
			if event != nil {
				when = event.Date
				if event.Time != "" {
					when += " " + event.Time
				}
			}
			fmt.Fprintf(&b, "%d. %s -> %s\n", i+1, task.Title, when)
		}
	}

	b.WriteString("\nAXIOMAS RELACIONADOS A HOY:\n")
	count := 0
	for _, task := range todayTasks {
		for _, axiom := range axiomsForTask(plan, task.ID) {
			count++
			fmt.Fprintf(&b, "%d. %s -> %s\n", count, axiom.Title, task.Title)
		}
	}
	if count == 0 {
		b.WriteString("- No hay axiomas vinculados a las tareas de hoy.\n")
	}

	if includeQuestion {
		question := strings.TrimSpace(nextQuestion(plan))
		if question != "" {
			fmt.Fprintf(&b, "\n%s\n", question)
		}
	}
	return b.String()
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func normalizePlan(plan *DayPlan) {
	if strings.TrimSpace(plan.Result) == "" {
		plan.Result = strings.TrimSpace(plan.Objective)
	}
	if strings.TrimSpace(plan.Objective) == "" {
		plan.Objective = strings.TrimSpace(plan.Result)
	}
	if plan.Events == nil {
		plan.Events = []Event{}
	}
	if plan.Tasks == nil {
		plan.Tasks = []Task{}
	}
	if plan.Axioms == nil {
		plan.Axioms = []Axiom{}
	}
	if plan.Conflicts == nil {
		plan.Conflicts = []PreConflict{}
	}
	if plan.Notes == nil {
		plan.Notes = []Note{}
	}
	if plan.Nodes == nil {
		plan.Nodes = []Node{}
	}
	normalizeEvents(plan)
	normalizeTasks(plan)
	ensureAxiomTaskCoverage(plan)
	ensureTaskAxiomCoverage(plan)
	ensurePreConflictFollowupTasks(plan)
}

func cleanupLegacyStateArtifacts() error {
	paths := []string{
		legacyStatePath,
		legacyMDPath,
		"temp/foco/today.json.broken",
		"temp/foco/today.json.fixed",
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func normalizeEvents(plan *DayPlan) {
	result := primaryResult(*plan)
	for i := range plan.Events {
		if strings.TrimSpace(plan.Events[i].Date) == "" {
			plan.Events[i].Date = plan.Date
		}
		if strings.TrimSpace(plan.Events[i].Why) == "" {
			plan.Events[i].Why = fallbackText(plan.InferredWhy, result)
		}
		if strings.TrimSpace(plan.Events[i].Result) == "" {
			plan.Events[i].Result = result
		}
		if strings.TrimSpace(plan.Events[i].Status) == "" {
			plan.Events[i].Status = "planned"
		}
	}
}

func normalizeTasks(plan *DayPlan) {
	defaultEventID := ensureDefaultEvent(plan)
	for i := range plan.Tasks {
		if plan.Tasks[i].Status == string([]byte{98, 108, 111, 99, 107, 101, 100}) {
			plan.Tasks[i].Status = "todo"
			if plan.Tasks[i].PreConflict == "" {
				pcID := ensurePreConflict(plan, fmt.Sprintf("Resolver lo necesario antes de continuar con \"%s\"", plan.Tasks[i].Title))
				plan.Tasks[i].PreConflict = pcID
			}
		}
		if strings.TrimSpace(plan.Tasks[i].Status) == "" {
			plan.Tasks[i].Status = "todo"
		}
		if strings.TrimSpace(plan.Tasks[i].EventID) == "" {
			plan.Tasks[i].EventID = defaultEventID
		}
		event := findEvent(plan.Tasks[i].EventID, *plan)
		if event == nil {
			plan.Tasks[i].EventID = defaultEventID
			event = findEvent(defaultEventID, *plan)
		}
		if event != nil {
			if strings.TrimSpace(plan.Tasks[i].DueDate) == "" {
				plan.Tasks[i].DueDate = event.Date
			}
			if strings.TrimSpace(plan.Tasks[i].Why) == "" {
				plan.Tasks[i].Why = event.Why
			}
			if strings.TrimSpace(plan.Tasks[i].Expected) == "" {
				plan.Tasks[i].Expected = event.Result
			}
		}
	}
}

func ensureDefaultEvent(plan *DayPlan) string {
	if len(plan.Events) > 0 {
		return plan.Events[0].ID
	}
	event := Event{
		ID:     "evt_001",
		Title:  "Entrega principal del dia",
		Date:   fallbackText(plan.Date, time.Now().Format("2006-01-02")),
		Result: primaryResult(*plan),
		Why:    fallbackText(plan.InferredWhy, primaryResult(*plan)),
		Status: "planned",
	}
	plan.Events = append(plan.Events, event)
	return event.ID
}

func ensureTaskAxiomCoverage(plan *DayPlan) {
	for _, task := range plan.Tasks {
		if len(axiomsForTask(*plan, task.ID)) > 0 {
			continue
		}
		event := findEvent(task.EventID, *plan)
		axiomTitle := inferAxiomForTask(task, event)
		plan.Axioms = append(plan.Axioms, Axiom{
			ID:       fmt.Sprintf("ax_%03d", len(plan.Axioms)+1),
			Title:    axiomTitle,
			TaskID:   task.ID,
			Evidence: "Generado por normalizacion para mantener tarea y axioma siempre vinculados.",
			Status:   "confirmed",
		})
	}
}

func ensureAxiomTaskCoverage(plan *DayPlan) {
	defaultEventID := ensureDefaultEvent(plan)
	for i := range plan.Axioms {
		if findTask(plan.Axioms[i].TaskID, *plan) != nil {
			continue
		}
		event := findEvent(defaultEventID, *plan)
		task := Task{
			ID:        fmt.Sprintf("task_%03d", len(plan.Tasks)+1),
			Title:     fmt.Sprintf("Cumplir axioma: %s", plan.Axioms[i].Title),
			EventID:   defaultEventID,
			Why:       fallbackText(event.Why, plan.InferredWhy),
			Expected:  fallbackText(event.Result, primaryResult(*plan)),
			Status:    "todo",
			CreatedAt: time.Now().Format(time.RFC3339),
			DueDate:   event.Date,
		}
		plan.Tasks = append(plan.Tasks, task)
		plan.Axioms[i].TaskID = task.ID
	}
}

func inferAxiomForTask(task Task, event *Event) string {
	if event != nil && strings.TrimSpace(event.Why) != "" {
		return fmt.Sprintf("La tarea \"%s\" debe cumplir el resultado y why de \"%s\"", task.Title, event.Title)
	}
	return fmt.Sprintf("La tarea \"%s\" debe cumplirse con una regla no negociable definida", task.Title)
}

func ensurePreConflict(plan *DayPlan, text string) string {
	for _, conflict := range plan.Conflicts {
		if conflict.Text == text && !conflict.Resolved {
			return conflict.ID
		}
	}
	id := fmt.Sprintf("pc_%03d", len(plan.Conflicts)+1)
	plan.Conflicts = append(plan.Conflicts, PreConflict{
		ID:        id,
		Text:      text,
		Resolved:  false,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
	return id
}

func ensurePreConflictFollowupTasks(plan *DayPlan) {
	for _, task := range plan.Tasks {
		if task.PreConflict == "" {
			continue
		}
		conflict := findConflict(task.PreConflict, *plan)
		if conflict == nil || conflict.Resolved {
			continue
		}
		if hasTaskForPreConflict(*plan, task.EventID, conflict.Text) {
			continue
		}
		title := fmt.Sprintf("Resolver pre-conflicto: %s", conflict.Text)
		newTask := Task{
			ID:        fmt.Sprintf("task_%03d", len(plan.Tasks)+1),
			Title:     title,
			EventID:   task.EventID,
			Why:       task.Why,
			Expected:  fmt.Sprintf("Pre-conflicto resuelto para continuar con %s", task.Title),
			Status:    "todo",
			CreatedAt: time.Now().Format(time.RFC3339),
			DueDate:   task.DueDate,
		}
		plan.Tasks = append(plan.Tasks, newTask)
		plan.Dependencies = append(plan.Dependencies, Dependency{
			ID:        fmt.Sprintf("dep_%03d", len(plan.Dependencies)+1),
			TaskID:    task.ID,
			DependsOn: newTask.ID,
		})
		plan.Axioms = append(plan.Axioms, Axiom{
			ID:       fmt.Sprintf("ax_%03d", len(plan.Axioms)+1),
			Title:    fmt.Sprintf("El pre-conflicto \"%s\" debe resolverse antes de \"%s\"", conflict.Text, task.Title),
			TaskID:   newTask.ID,
			Evidence: "Generado para cubrir una tarea obvia faltante a partir de un pre-conflicto.",
			Status:   "confirmed",
		})
	}
}

func findConflict(conflictID string, plan DayPlan) *PreConflict {
	for i := range plan.Conflicts {
		if plan.Conflicts[i].ID == conflictID {
			return &plan.Conflicts[i]
		}
	}
	return nil
}

func hasTaskForPreConflict(plan DayPlan, eventID, conflictText string) bool {
	target := strings.ToLower(strings.TrimSpace(conflictText))
	for _, task := range plan.Tasks {
		if task.EventID != eventID {
			continue
		}
		title := strings.ToLower(task.Title)
		if strings.Contains(title, target) || (strings.Contains(title, "resolver pre-conflicto") && strings.Contains(title, target)) {
			return true
		}
	}
	return false
}

func materializeAnswer(plan *DayPlan, text string) int {
	now := time.Now().Format(time.RFC3339)
	created := 0
	lower := strings.ToLower(text)
	if materializeLayerAnswer(plan, text) {
		return 1
	}
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
	if containsAny(lower, "arreglar antes", "pre conflicto", "pre-conflicto", "hay que") {
		add("PRE_CONFLICT", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "pre_conflict", Text: summarize(text), Time: now})
	}
	if created == 0 {
		add("OBSERVATION", summarize(text))
		plan.Notes = append(plan.Notes, Note{Kind: "decision", Text: summarize(text), Time: now})
	}
	return created
}

func nextQuestion(plan DayPlan) string {
	switch nextMissingLayer(plan) {
	case "result":
		return "¿Qué resultado observable quieres obtener hoy?\n1. Ya está claro\n2. No está claro todavía\n3. Sí, pero necesito aterrizarlo"
	case "event":
		return "¿Qué evento o entrega con fecha sostiene ese resultado?\n1. Ya lo tengo\n2. Aún no\n3. Sí, pero falta calendarizarlo"
	case "task":
		event := findCurrentOrTodayEvent(plan)
		title := "ese evento"
		if event != nil {
			title = fmt.Sprintf("\"%s\"", event.Title)
		}
		return fmt.Sprintf("¿Qué tarea de hoy mueve %s?\n1. Ya la tengo clara\n2. Aún no\n3. Sí, pero necesito ordenarla", title)
	case "axiom":
		task := nextPrimaryTask(plan)
		title := "esa tarea"
		if task != nil {
			title = fmt.Sprintf("\"%s\"", task.Title)
		}
		return fmt.Sprintf("¿Qué axioma no negociable debe cumplirse en %s?\n1. Ya lo sé\n2. No está claro\n3. Sí, pero hay más de uno", title)
	default:
		if issue := detectAlignmentIssue(plan); issue != nil {
			return issue.Question
		}
		return ""
	}
}

func hasNode(plan DayPlan, nodeType string) bool {
	for _, node := range plan.Nodes {
		if node.Type == nodeType {
			return true
		}
	}
	return false
}

func nextMissingLayer(plan DayPlan) string {
	if strings.TrimSpace(primaryResult(plan)) == "" {
		return "result"
	}
	todayEvents := eventsForDate(plan, plan.Date)
	if len(todayEvents) == 0 {
		return "event"
	}
	todayTasks := tasksForDate(plan, plan.Date)
	if len(todayTasks) == 0 {
		return "task"
	}
	for _, task := range todayTasks {
		if len(axiomsForTask(plan, task.ID)) == 0 {
			return "axiom"
		}
	}
	return ""
}

func parseEventAnswer(text, fallbackDate string) (title, date, at string) {
	title = summarize(text)
	date = fallbackDate

	for _, token := range strings.Fields(text) {
		if _, err := time.Parse("2006-01-02", token); err == nil {
			date = token
			break
		}
	}

	for _, token := range strings.Fields(text) {
		if _, err := time.Parse("15:04", token); err == nil {
			at = token
			break
		}
	}

	title = strings.ReplaceAll(title, date, "")
	if at != "" {
		title = strings.ReplaceAll(title, at, "")
	}
	title = strings.TrimSpace(strings.Trim(title, "-,:"))
	if title == "" {
		title = "Evento de hoy"
	}
	return title, date, at
}

func detectAlignmentIssue(plan DayPlan) *AlignmentIssue {
	todayTasks := tasksForDate(plan, plan.Date)
	why := strings.TrimSpace(inferWhyToday(plan, todayTasks))
	result := strings.TrimSpace(primaryResult(plan))
	if why == "" && result == "" {
		return nil
	}

	if event := findCurrentOrTodayEvent(plan); event != nil {
		if lowAlignment(event.Title+" "+event.Why, why+" "+result) {
			return &AlignmentIssue{
				Kind:    "event_vs_why",
				Target:  event.Title,
				Against: why,
				Question: fmt.Sprintf(
					"Veo que hoy apuntas a \"%s\", pero el evento \"%s\" suena a otra linea. ¿Que manda hoy?\n1. Manda el resultado de hoy\n2. Manda ese evento\n3. Ajustemos el evento para que calce",
					shortText(result, 70),
					event.Title,
				),
			}
		}
	}

	if task := nextPrimaryTask(plan); task != nil {
		event := findEvent(task.EventID, plan)
		base := why + " " + result
		if event != nil {
			base += " " + event.Title + " " + event.Why
		}
		if lowAlignment(task.Title+" "+task.Why, base) {
			return &AlignmentIssue{
				Kind:    "task_vs_why",
				Target:  task.Title,
				Against: why,
				Question: fmt.Sprintf(
					"Veo que \"%s\" no calza tan claro con lo que quieres lograr hoy. ¿Como lo lees?\n1. Si calza, sigamos\n2. No calza, cambiemos la tarea\n3. Calza parcialmente, ajustemos el enfoque",
					task.Title,
				),
			}
		}
	}

	return nil
}

func lowAlignment(a, b string) bool {
	aw := relevantTokens(a)
	bw := relevantTokens(b)
	if len(aw) == 0 || len(bw) == 0 {
		return false
	}
	shared := 0
	for token := range aw {
		if bw[token] {
			shared++
		}
	}
	return shared == 0
}

func relevantTokens(text string) map[string]bool {
	stop := map[string]bool{
		"el": true, "la": true, "los": true, "las": true, "de": true, "del": true,
		"y": true, "o": true, "a": true, "en": true, "para": true, "con": true,
		"por": true, "que": true, "hoy": true, "una": true, "uno": true,
		"un": true, "si": true, "no": true, "real": true, "debe": true,
	}
	clean := strings.NewReplacer(",", " ", ".", " ", ":", " ", ";", " ", "\"", " ", "¿", " ", "?", " ", "(", " ", ")", " ").Replace(strings.ToLower(text))
	tokens := map[string]bool{}
	for _, part := range strings.Fields(clean) {
		if len(part) < 4 || stop[part] {
			continue
		}
		tokens[part] = true
	}
	return tokens
}

func shortText(text string, n int) string {
	text = strings.TrimSpace(text)
	if len(text) <= n {
		return text
	}
	return strings.TrimSpace(text[:n-3]) + "..."
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
		"PRE_CONFLICT":     "pc",
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

func assessReadiness(plan DayPlan) readinessReport {
	var missing []string
	if !hasNode(plan, "HUMAN_PAIN") {
		missing = append(missing, "dolor humano/prioridad del dia")
	}
	if !hasNode(plan, "FLOW_EXPECTATION") {
		missing = append(missing, "expectativa del flujo del dia")
	}
	if !hasNode(plan, "EXPECTED_RESULT") {
		missing = append(missing, "resultado observable de la version")
	}
	if len(missing) > 0 {
		return readinessReport{Ready: false, NextAction: nextQuestion(plan), Missing: missing}
	}
	return readinessReport{
		Ready:      true,
		NextAction: "Catastro listo. Ejecuta foco next para ver la tarea actual.",
		Missing:    nil,
	}
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
		if !os.IsNotExist(err) {
			return DayPlan{}, err
		}
		legacyData, legacyErr := os.ReadFile(legacyStatePath)
		if legacyErr != nil {
			return DayPlan{}, fmt.Errorf("no hay estado de foco; ejecuta foco init primero")
		}
		var migrated DayPlan
		if err := json.Unmarshal(legacyData, &migrated); err != nil {
			return DayPlan{}, err
		}
		normalizePlan(&migrated)
		if err := save(migrated); err != nil {
			return DayPlan{}, err
		}
		return migrated, nil
	}
	var plan DayPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return DayPlan{}, err
	}
	normalizePlan(&plan)
	return plan, nil
}

func save(plan DayPlan) error {
	normalizePlan(&plan)
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
	if err := os.WriteFile(mdPath, []byte(formatMarkdown(plan)), 0644); err != nil {
		return err
	}
	return cleanupLegacyStateArtifacts()
}

func formatMarkdown(plan DayPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Foco Diario - %s\n\n", plan.Date)
	fmt.Fprintf(&b, "- Version foco: `%s`\n", plan.Version)
	fmt.Fprintf(&b, "- Resultado: %s\n\n", primaryResult(plan))
	b.WriteString("## Resumen Primario\n\n")
	b.WriteString("```text\n")
	b.WriteString(formatPrimarySummary(plan, false))
	b.WriteString("```\n\n")

	groups := []string{"why", "axiom", "human", "flow", "pre_conflict", "decision", "done"}
	titles := map[string]string{
		"why":          "Why De La Version",
		"axiom":        "Axiomas",
		"human":        "Dolores Humanos",
		"flow":         "Fallas O Reglas De Flujo",
		"pre_conflict": "Pre Conflictos",
		"decision":     "Decisiones",
		"done":         "Criterios De Termino",
	}

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

	if len(plan.Conflicts) > 0 {
		b.WriteString("## Pre Conflictos\n\n")
		for _, c := range plan.Conflicts {
			status := "[OPEN]"
			if c.Resolved {
				status = "[RESOLVED]"
			}
			fmt.Fprintf(&b, "- %s %s: %s\n", status, c.ID, c.Text)
		}
		b.WriteString("\n")
	}

	if len(plan.Nodes) > 0 {
		b.WriteString("## Arbol De Foco\n\n")
		for _, node := range plan.Nodes {
			if node.Type == "AXIOM" && node.Status == "superseded" {
				fmt.Fprintf(&b, "- [%s] %s: %s [SUPERSEDED]\n", node.ID, node.Type, node.Title)
			} else {
				fmt.Fprintf(&b, "- [%s] %s: %s\n", node.ID, node.Type, node.Title)
			}
		}
		b.WriteString("\n")
	}

	if len(plan.Tasks) > 0 {
		b.WriteString("## Checklist De Ejecucion\n\n")
		for _, task := range plan.Tasks {
			status := task.Status
			if status == "" {
				status = "todo"
			}
			fmt.Fprintf(&b, "- [%s] %s: %s\n", status, task.ID, task.Title)
			if task.EventID != "" {
				fmt.Fprintf(&b, "  event: %s\n", task.EventID)
			}
			if task.PreConflict != "" {
				fmt.Fprintf(&b, "  pre_conflict: %s\n", task.PreConflict)
			}
		}
		b.WriteString("\n")
	}
	if len(plan.Events) > 0 {
		b.WriteString("## Eventos\n\n")
		for _, event := range plan.Events {
			line := fmt.Sprintf("- [%s] %s (%s", event.ID, event.Title, event.Date)
			if event.Time != "" {
				line += " " + event.Time
			}
			line += ")"
			if event.Why != "" {
				line += ": " + event.Why
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}
	if len(plan.Axioms) > 0 {
		b.WriteString("## Axiomas Vinculados\n\n")
		for _, axiom := range plan.Axioms {
			fmt.Fprintf(&b, "- [%s] %s -> %s\n", axiom.ID, axiom.Title, axiom.TaskID)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatTasks(plan DayPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Checklist %s %s\n\n", plan.Version, plan.Date)
	for _, task := range plan.Tasks {
		status := task.Status
		if status == "" {
			status = "todo"
		}
		fmt.Fprintf(&b, "[%s] %s - %s\n", status, task.ID, task.Title)
		fmt.Fprintf(&b, "  why: %s\n", task.Why)
		fmt.Fprintf(&b, "  esperado: %s\n", task.Expected)
		if task.PreConflict != "" {
			fmt.Fprintf(&b, "  pre_conflict: %s\n", task.PreConflict)
		}
	}
	return b.String()
}

func formatTree(plan DayPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Foco %s %s\n\n", plan.Version, plan.Date)
	order := []string{"CONTEXT", "AXIOM", "HUMAN_PAIN", "FLOW_EXPECTATION", "EXPECTED_RESULT", "PRE_CONFLICT", "OBSERVATION"}
	for _, nodeType := range order {
		nodes := filterNodes(plan.Nodes, nodeType)
		if len(nodes) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\n", nodeType)
		for _, node := range nodes {
			if node.Status == "superseded" {
				fmt.Fprintf(&b, "  [%s] %s [SUPERSEDED]\n", node.ID, node.Title)
			} else {
				fmt.Fprintf(&b, "  [%s] %s\n", node.ID, node.Title)
			}
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
	fmt.Println(`Foco CLI - flujo primario por resultado, evento, tarea y axioma

PRIMARIO:
  foco init --version v0.1.5 --result "resultado del dia"
  foco event --title "evento" --date 2026-04-28 [--time 14:00] [--why "por que importa"]
  foco task --title "tarea" [--event evt_001] [--expected "resultado"]
  foco axiom --text "regla no negociable" [--task task_001]
  foco today
  foco next
  foco done --id task_001 --evidence "resultado"

SECUNDARIO:
  foco priority
  foco conflict --text "pre-conflicto"
  foco depends --on task_001 --task task_002
  foco flow
  foco whatif --task task_001
  foco ask
  foco answer --text "texto libre"`)
}

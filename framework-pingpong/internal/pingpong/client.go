package pingpong

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"framework-pingpong/internal/paladin"
)

// ProgressFile es la ruta del archivo de progreso
const ProgressFile = "pingpong_progress.json"

// Step representa un paso en el aprendizaje
type Step struct {
	ID          int    `json:"id"`
	Instruction string `json:"instruction"`
	Done        bool   `json:"done"`
	FailCount   int    `json:"fail_count,omitempty"`
}

// QA representa un registro de pregunta-respuesta
type QA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Purpose  string `json:"purpose,omitempty"`
	At       string `json:"at"`
}

// Signal representa una señal de fatiga o confusión
type Signal struct {
	Type    string `json:"type"`
	Note    string `json:"note,omitempty"`
	At      string `json:"at"`
}

// Progress representa el estado del aprendizaje
type Progress struct {
	Goal         string    `json:"goal"`
	CurrentStep  int       `json:"currentStep"`
	Steps        []Step    `json:"steps"`
	QALog        []QA      `json:"qaLog"`
	Signals      []Signal  `json:"signals"`
	StartedAt    time.Time `json:"startedAt"`
	Active       bool      `json:"active"`
	BatchSize    int       `json:"batchSize"`    // Pasos por mini-test (default 3)
	BatchIndex   int       `json:"batchIndex"`   // Cuál mini-test estamos
	PendingTest  []string  `json:"pendingTest"`   // Pasos pendientes de mini-test
	MiniTest     []string  `json:"miniTest"`     // Ejercicios del mini-test actual
	InTest       bool      `json:"inTest"`        // Si estamos esperando respuesta de mini-test
	InMinitest   bool      `json:"inMinitest"`   // Si estamos en modo mini-test (archivo borrado, reescribir todo)
	Checkpoint   string    `json:"checkpoint,omitempty"`     // Snapshot del archivo al pasar el último mini-test
	CheckpointFile string  `json:"checkpointFile,omitempty"` // Path original del checkpoint
	CompletedAll bool      `json:"completedAll"` // Si completó todos los pasos
	LastVerifyDecls []string `json:"lastVerifyDecls,omitempty"` // Decl names at last scan/verify for rewrite detection
}

// Result representa el resultado de una operacion.
type Result struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ScanResult wraps Progress with additional scan analysis fields.
type ScanResult struct {
	*Progress
	CompileOK  bool     `json:"compile_ok"`
	CompileLog string   `json:"compile_log,omitempty"`
	Noise      []string `json:"noise,omitempty"`
}

// Client es el cliente principal del framework.
type Client struct {
	trace   *paladin.Trace
	ctx     *paladin.Context
	baseDir string
}

// New crea un nuevo cliente.
func New() *Client {
	return NewWithTrace("framework-pingpong")
}

// NewWithTrace crea un cliente con tracing activo.
func NewWithTrace(name string) *Client {
	trace := paladin.NewTrace(name)
	ctx := trace.Start()
	return &Client{trace: trace, ctx: ctx, baseDir: getBaseDir()}
}

// Flush guarda el trace actual.
func (c *Client) Flush() {
	if c.trace != nil {
		c.trace.Flush()
	}
}

// getBaseDir obtiene el directorio base del framework
func getBaseDir() string {
	exe, _ := os.Executable()
	return filepath.Dir(exe)
}

func (c *Client) loadOrCreate() (*Progress, error) {
	data, err := os.ReadFile(ProgressFile)
	if err != nil {
		return &Progress{
			Active:    false,
			Steps:     []Step{},
			QALog:     []QA{},
			Signals:   []Signal{},
			StartedAt: time.Now(),
		}, nil
	}

	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

func (c *Client) saveProgress(p *Progress) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ProgressFile, data, 0644)
}

// Init inicializa un nuevo proyecto
func (c *Client) Init() (*Result, error) {
	childCtx := c.ctx.Child("Init")
	defer childCtx.End()

	p := &Progress{
		Active:    false,
		Steps:     []Step{},
		QALog:     []QA{},
		Signals:   []Signal{},
		StartedAt: time.Now(),
	}

	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Decision("proyecto-inicializado", "progress.json creado")

	return &Result{
		Success: true,
		Message: "Proyecto inicializado. Usa 'start --goal' para comenzar.",
	}, nil
}

// Start inicia un nuevo aprendizaje
func (c *Client) Start(goal string, stepsArg string) (*Result, error) {
	childCtx := c.ctx.Child("Start")
	defer childCtx.End()

	var steps []Step
	
	if stepsArg != "" {
		stepList := strings.Split(stepsArg, ";")
		for i, inst := range stepList {
			inst = strings.TrimSpace(inst)
			if inst != "" {
				steps = append(steps, Step{ID: i + 1, Instruction: inst, Done: false})
			}
		}
	} else {
		steps = generateStepsFromGoal(goal)
	}

	if len(steps) == 0 {
		steps = []Step{{ID: 1, Instruction: "Definir objetivo", Done: false}}
	}

	// Generar pendingTest con los primeros batchSize pasos
	batchSize := 3
	var pendingTest []string
	for i := 0; i < batchSize && i < len(steps); i++ {
		pendingTest = append(pendingTest, steps[i].Instruction)
	}

	progress := &Progress{
		Goal:        goal,
		CurrentStep: 1,
		Steps:       steps,
		QALog:       []QA{},
		Signals:     []Signal{},
		StartedAt:   time.Now(),
		Active:      true,
		BatchSize:   batchSize,
		BatchIndex:  1,
		PendingTest: pendingTest,
		MiniTest:    generateMiniTest(goal),
		InTest:      true,
		CompletedAll: false,
	}

	if err := c.saveProgress(progress); err != nil {
		return nil, err
	}

	childCtx.Decision("proyecto-iniciado", goal)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Iniciado: %s. Mini-test %d: %v", goal, progress.BatchIndex, pendingTest),
		Data:    progress,
	}, nil
}

// Ask hace una pregunta al usuario y la registra
func (c *Client) Ask(question string) (*Result, error) {
	childCtx := c.ctx.Child("Ask")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}

	qa := QA{
		Question: question,
		At:       time.Now().Format(time.RFC3339),
	}

	p.QALog = append(p.QALog, qa)
	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Var("question", question)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Pregunta registrada: %s", question),
	}, nil
}

// LogQA registra una pregunta-respuesta
func (c *Client) LogQA(question, answer, purpose string) (*Result, error) {
	childCtx := c.ctx.Child("LogQA")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}

	qa := QA{
		Question: question,
		Answer:   answer,
		Purpose:  purpose,
		At:       time.Now().Format(time.RFC3339),
	}

	p.QALog = append(p.QALog, qa)
	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Decision("qa-registrado", question)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Q&A registrado: %s → %s", question, answer),
	}, nil
}

// Done marca un paso como completado
func (c *Client) Done(stepID string) (*Result, error) {
	childCtx := c.ctx.Child("Done")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}

	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}

	// Parsear ID del paso
	var id int
	if _, err := fmt.Sscanf(stepID, "%d", &id); err != nil {
		return nil, fmt.Errorf("ID inválido: %s", stepID)
	}

	// Buscar y marcar paso
	found := false
	for i, s := range p.Steps {
		if s.ID == id {
			p.Steps[i].Done = true
			found = true

			// Si es el paso actual, avanzar
			if p.CurrentStep == id && id < len(p.Steps) {
				p.CurrentStep = id + 1
			}
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("paso %d no existe", id)
	}

	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	// Verificar si completó todos
	allDone := true
	for _, s := range p.Steps {
		if !s.Done {
			allDone = false
			break
		}
	}

	if allDone {
		p.Active = false
	}

	nextInstruction := ""
	if !allDone {
		for _, s := range p.Steps {
			if !s.Done {
				nextInstruction = s.Instruction
				break
			}
		}
	}

	childCtx.Var("step_id", id)
	childCtx.Decision("paso-completado", fmt.Sprintf("paso %d", id))

	return &Result{
		Success: allDone,
		Message: func() string {
			if allDone {
				return fmt.Sprintf("¡Completado! %s", p.Goal)
			}
			return fmt.Sprintf("✓ Paso %d. Siguiente: %s", id, nextInstruction)
		}(),
		Data: p,
	}, nil
}

// Status muestra el progreso actual
func (c *Client) Status() (interface{}, error) {
	childCtx := c.ctx.Child("Status")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}

	if !p.Active && len(p.Steps) == 0 {
		return map[string]interface{}{
			"active":  false,
			"message": "No hay proyecto activo. Usa 'init' y luego 'start --goal'.",
		}, nil
	}

	doneCount := 0
	for _, s := range p.Steps {
		if s.Done {
			doneCount++
		}
	}

	childCtx.Decision("status-mostrado", fmt.Sprintf("[%d/%d]", doneCount, len(p.Steps)))

	return map[string]interface{}{
		"active":    p.Active,
		"goal":      p.Goal,
		"progress":  fmt.Sprintf("[%d/%d]", doneCount, len(p.Steps)),
		"nextStep":  p.CurrentStep,
		"steps":     p.Steps,
		"qaLog":     p.QALog,
		"signals":   p.Signals,
	}, nil
}

// Signal registra fatiga o confusión
func (c *Client) Signal(signalType, note string) (*Result, error) {
	childCtx := c.ctx.Child("Signal")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}

	signal := Signal{
		Type: signalType,
		Note: note,
		At:   time.Now().Format(time.RFC3339),
	}

	p.Signals = append(p.Signals, signal)
	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Var("signal_type", signalType)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Señal registrada: %s", signalType),
	}, nil
}

// Run compila y ejecuta el archivo del usuario en un sandbox aislado.
// stdin se pasa como entrada; expected (si no vacío) se compara con stdout.
func (c *Client) Run(filePath string, stdin string, expected string) (*Result, error) {
	childCtx := c.ctx.Child("Run")
	defer childCtx.End()

	if filePath == "" {
		return nil, fmt.Errorf("necesitas --file <archivo>")
	}

	rep, err := RunFile(filePath, stdin, expected)
	if err != nil {
		return nil, err
	}

	childCtx.Var("file", filePath)
	childCtx.Var("compile_ok", fmt.Sprintf("%v", rep.CompileOK))
	childCtx.Var("run_ok", fmt.Sprintf("%v", rep.RunOK))
	childCtx.Var("match", fmt.Sprintf("%v", rep.Match))

	if !rep.CompileOK {
		childCtx.Decision("run-compile-error", rep.CompileLog)
		return &Result{
			Success: false,
			Message: fmt.Sprintf("❌ No compila: %s", rep.CompileLog),
			Data:    rep,
		}, nil
	}

	if !rep.RunOK {
		msg := "❌ Error en ejecución"
		if rep.TimedOut {
			msg = "❌ Timeout (>10s). Posible loop infinito."
		} else if rep.Stderr != "" {
			msg = fmt.Sprintf("❌ Error en ejecución: %s", rep.Stderr)
		}
		childCtx.Decision("run-error", msg)
		return &Result{
			Success: false,
			Message: msg,
			Data:    rep,
		}, nil
	}

	if expected != "" && !rep.Match {
		childCtx.Decision("run-mismatch", fmt.Sprintf("expected=%q got=%q", expected, rep.Stdout))
		return &Result{
			Success: false,
			Message: fmt.Sprintf("❌ Output incorrecto. Esperado: %q — Obtenido: %q", expected, rep.Stdout),
			Data:    rep,
		}, nil
	}

	childCtx.Decision("run-ok", fmt.Sprintf("output=%q", rep.Stdout))
	msg := fmt.Sprintf("✓ Ejecución correcta. Output: %q", rep.Stdout)
	if expected != "" {
		msg = fmt.Sprintf("✓ Output correcto: %q", rep.Stdout)
	}
	return &Result{
		Success: true,
		Message: msg,
		Data:    rep,
	}, nil
}

// Clean removes noisy declarations (code that doesn't map to any step) from the user's file.
// If explicitNames is non-empty, removes those specific declarations (fallback mode).
// Otherwise auto-detects noise by name-matching against step instructions.
// It ONLY deletes lines — never adds or modifies code. Safe for the AI to invoke.
func (c *Client) Clean(filePath string, explicitNames []string) (*Result, error) {
	childCtx := c.ctx.Child("Clean")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo")
	}
	if filePath == "" {
		return nil, fmt.Errorf("clean requiere --file <archivo>")
	}

	var targets []string
	if len(explicitNames) > 0 {
		targets = explicitNames
	} else {
		targets = DetectNoiseNames(filePath, p.Steps)
	}

	if len(targets) == 0 {
		return &Result{
			Success: true,
			Message: "No se detectó ruido para limpiar.",
		}, nil
	}

	removed, rerr := RemoveDeclarations(filePath, targets)
	if rerr != nil {
		return nil, rerr
	}

	// Update declaration baseline so rewrite guard doesn't trigger on this clean.
	p.LastVerifyDecls = ExtractDeclNames(filePath)
	_ = c.saveProgress(p)

	childCtx.Decision("clean", fmt.Sprintf("removed %v", removed))
	return &Result{
		Success: true,
		Message: fmt.Sprintf("Limpieza: se eliminaron %d declaraciones: %s. Solo se borraron líneas, no se agregó código.",
			len(removed), strings.Join(removed, ", ")),
		Data: map[string]interface{}{
			"removed": removed,
		},
	}, nil
}

// Peek returns a small window of source code around a specific line.
// The AI uses this to get targeted context without reading the full file.
func (c *Client) Peek(filePath string, line int, radius int) (*Result, error) {
	if filePath == "" {
		return nil, fmt.Errorf("peek requiere --file <archivo>")
	}
	if line < 1 {
		return nil, fmt.Errorf("peek requiere --line <número> (positivo)")
	}
	if radius < 1 {
		radius = 3
	}
	if radius > 10 {
		radius = 10
	}

	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", abs, err)
	}

	lines := strings.Split(string(src), "\n")
	total := len(lines)
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	if end > total {
		end = total
	}

	width := len(fmt.Sprintf("%d", end))
	var snippet []string
	for i := start; i <= end; i++ {
		prefix := "  "
		if i == line {
			prefix = "→ "
		}
		snippet = append(snippet, fmt.Sprintf("%s%*d | %s", prefix, width, i, lines[i-1]))
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Líneas %d–%d de %d (total %d líneas):", start, end, total, total),
		Data: map[string]interface{}{
			"snippet":    snippet,
			"line":       line,
			"total":      total,
			"range_from": start,
			"range_to":   end,
		},
	}, nil
}

// Reset reinicia el proyecto incluyendo traces
func (c *Client) Reset() (*Result, error) {
	childCtx := c.ctx.Child("Reset")
	defer childCtx.End()

	// Borrar progress.json
	if err := os.Remove(ProgressFile); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Borrar todos los traces de Paladin
	traceDir := filepath.Join(c.baseDir, "temp", "paladin")
	if err := os.RemoveAll(traceDir); err != nil {
		// No es crítico, solo warning
		fmt.Printf("Warning: no se pudieron borrar traces: %v\n", err)
	}

	// Recrear directorio de traces vacío
	os.MkdirAll(traceDir, 0755)

	childCtx.Decision("proyecto-reiniciado", "progress.json y traces eliminados")

	return &Result{
		Success: true,
		Message: "Proyecto y traces reiniciados. Sesión limpia.",
	}, nil
}

// SetSteps permite a la IA registrar pasos específicos para el objetivo actual
func (c *Client) SetSteps(stepsArg string) (*Result, error) {
	childCtx := c.ctx.Child("SetSteps")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}

	stepList := strings.Split(stepsArg, ";")
	var steps []Step
	for i, inst := range stepList {
		inst = strings.TrimSpace(inst)
		if inst != "" {
			steps = append(steps, Step{ID: i + 1, Instruction: inst, Done: false})
		}
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no se proporcionaron pasos")
	}

	// Generar pendingTest con los primeros batchSize pasos
	batchSize := p.BatchSize
	if batchSize == 0 {
		batchSize = 3
	}
	var pendingTest []string
	for i := 0; i < batchSize && i < len(steps); i++ {
		pendingTest = append(pendingTest, steps[i].Instruction)
	}

	p.Steps = steps
	p.CurrentStep = 1
	p.Active = true
	p.PendingTest = pendingTest
	p.InTest = true
	p.BatchIndex = 1

	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Decision("pasos-registrados", fmt.Sprintf("%d pasos", len(steps)))

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Pasos registrados. Mini-test 1: %v", pendingTest),
		Data:    steps,
	}, nil
}

// Subdivide reemplaza un paso por varios sub-pasos más granulares.
// Renumera todos los pasos secuencialmente y recalcula el batch actual.
// Uso: cuando el usuario se traba en un paso, la IA lo parte en sub-pasos
// más concretos sin tocar el resto del plan.
func (c *Client) Subdivide(stepID int, substepsArg string) (*Result, error) {
	childCtx := c.ctx.Child("Subdivide")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}
	if p.InMinitest {
		return nil, fmt.Errorf("no se puede subdividir durante un mini-test")
	}

	// Parsear sub-pasos
	parts := strings.Split(substepsArg, ";")
	var substeps []string
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" {
			substeps = append(substeps, s)
		}
	}
	if len(substeps) < 2 {
		return nil, fmt.Errorf("subdivide requiere al menos 2 sub-pasos")
	}

	// Buscar el paso a subdividir
	idx := -1
	for i, s := range p.Steps {
		if s.ID == stepID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("paso %d no encontrado", stepID)
	}
	if p.Steps[idx].Done {
		return nil, fmt.Errorf("paso %d ya completado, no se puede subdividir", stepID)
	}

	oldInstruction := p.Steps[idx].Instruction

	// Construir nueva lista: anteriores + sub-pasos + posteriores
	var newSteps []Step
	newSteps = append(newSteps, p.Steps[:idx]...)
	for _, sub := range substeps {
		newSteps = append(newSteps, Step{Instruction: sub, Done: false})
	}
	newSteps = append(newSteps, p.Steps[idx+1:]...)

	// Renumerar secuencialmente
	for i := range newSteps {
		newSteps[i].ID = i + 1
	}

	p.Steps = newSteps
	p.CurrentStep = idx + 1 // ID del primer sub-paso (1-based)

	// Recalcular batch
	batchSize := p.BatchSize
	if batchSize == 0 {
		batchSize = 3
	}
	p.BatchIndex = (idx / batchSize) + 1
	batchStart, batchEnd := batchRange(p.BatchIndex, batchSize, len(p.Steps))
	var pending []string
	for i := batchStart; i < batchEnd; i++ {
		pending = append(pending, p.Steps[i].Instruction)
	}
	p.PendingTest = pending
	p.InTest = true

	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Decision("paso-subdividido", fmt.Sprintf("'%s' → %d sub-pasos", oldInstruction, len(substeps)))

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Paso %d subdividido en %d sub-pasos. Paso actual: %d (%s)",
			stepID, len(substeps), p.CurrentStep, newSteps[idx].Instruction),
		Data: p,
	}, nil
}

// Scan verifica todos los pasos contra un archivo existente y auto-avanza
// los que ya están cumplidos. Útil cuando el usuario ya tiene código previo
// y no debe repetir pasos que ya hizo.
// Salta los mini-tests de batches anteriores (son trabajo previo, no actual).
func (c *Client) Scan(filePath string) (*Result, error) {
	childCtx := c.ctx.Child("Scan")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}
	if filePath == "" {
		return nil, fmt.Errorf("scan requiere --file <archivo>")
	}
	if len(p.Steps) <= 1 {
		return nil, fmt.Errorf("primero registrá los pasos con set-steps. Orden: start → set-steps → scan")
	}

	batchSize := p.BatchSize
	if batchSize == 0 {
		batchSize = 3
	}

	var passed []int
	for i := range p.Steps {
		if p.Steps[i].Done {
			passed = append(passed, p.Steps[i].ID)
			continue
		}
		report, verr := VerifyFileLenient(filePath, p.Steps[i])
		if verr != nil {
			continue
		}
		if !report.Passed {
			continue
		}
		p.Steps[i].Done = true
		p.Steps[i].FailCount = 0
		passed = append(passed, p.Steps[i].ID)
	}

	// Buscar el primer paso no cumplido
	firstUndone := -1
	for i := range p.Steps {
		if !p.Steps[i].Done {
			firstUndone = i
			break
		}
	}

	// Save declaration names for rewrite detection in verify
	p.LastVerifyDecls = ExtractDeclNames(filePath)

	// Post-scan analysis: compile check + noise detection
	compileOK, compileLog, noise := ScanAnalysis(filePath, p.Steps)

	if firstUndone == -1 {
		p.CompletedAll = true
		p.Active = false
		_ = c.saveProgress(p)
		childCtx.Decision("scan-completo", fmt.Sprintf("%d pasos, todos cumplidos", len(p.Steps)))
		msg := "Scan: todos los pasos ya están cumplidos. Pasá a la fase final (run)."
		if !compileOK {
			msg += fmt.Sprintf(" ⚠️ ADVERTENCIA: el código no compila: %s", compileLog)
		}
		if len(noise) > 0 {
			msg += fmt.Sprintf(" ⚠️ Código no relacionado: %s", strings.Join(noise, "; "))
		}
		return &Result{
			Success: true,
			Message: msg,
			Data:    &ScanResult{Progress: p, CompileOK: compileOK, CompileLog: compileLog, Noise: noise},
		}, nil
	}

	// Avanzar batch al que corresponde el primer paso pendiente
	p.CurrentStep = p.Steps[firstUndone].ID
	newBatchIndex := (firstUndone / batchSize) + 1
	p.BatchIndex = newBatchIndex

	// Checkpoint del archivo actual si avanzamos batches
	if newBatchIndex > 1 {
		if content, rerr := os.ReadFile(filePath); rerr == nil {
			p.Checkpoint = string(content)
			p.CheckpointFile = filePath
		}
	}

	// Actualizar pending test del batch actual
	batchStart, batchEnd := batchRange(p.BatchIndex, batchSize, len(p.Steps))
	var pending []string
	for i := batchStart; i < batchEnd; i++ {
		pending = append(pending, p.Steps[i].Instruction)
	}
	p.PendingTest = pending
	p.InTest = true
	p.InMinitest = false

	_ = c.saveProgress(p)

	step := p.Steps[firstUndone]
	childCtx.Decision("scan-avanzado", fmt.Sprintf("%d cumplidos, continuar paso %d", len(passed), step.ID))

	msg := fmt.Sprintf("Scan: %d pasos ya cumplidos. Continuá con el paso %d: %s",
		len(passed), step.ID, step.Instruction)
	if !compileOK {
		msg += fmt.Sprintf(" ⚠️ ADVERTENCIA: el código no compila: %s", compileLog)
	}
	if len(noise) > 0 {
		msg += fmt.Sprintf(" ⚠️ Código no relacionado: %s", strings.Join(noise, "; "))
	}

	return &Result{
		Success: true,
		Message: msg,
		Data:    &ScanResult{Progress: p, CompileOK: compileOK, CompileLog: compileLog, Noise: noise},
	}, nil
}

// Verify avanza el proyecto. Tiene dos modos:
//
//  1. Normal (InMinitest=false): valida UN paso (el actual) contra el archivo.
//     Si pasa, lo marca done y avanza. Cuando el batch se completa, dispara
//     mini-test: trunca el archivo, resetea los pasos del batch a done=false,
//     y devuelve un mensaje pidiendo reescribir todo el batch desde cero.
//
//  2. Mini-test (InMinitest=true): valida TODOS los pasos del batch a la vez.
//     Si todos pasan, sale del mini-test y avanza al siguiente batch.
//     Si alguno falla, marca pasados/fallados, sale del mini-test, y deja
//     currentStep en el primer fallado para que el usuario los repita.
//     Al completar de nuevo el batch, se vuelve a disparar otro mini-test.
func (c *Client) Verify(filePath string) (*Result, error) {
	childCtx := c.ctx.Child("Verify")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}
	if filePath == "" {
		return nil, fmt.Errorf("verify requiere --file <archivo>")
	}

	batchSize := p.BatchSize
	if batchSize == 0 {
		batchSize = 3
	}
	batchStart, batchEnd := batchRange(p.BatchIndex, batchSize, len(p.Steps))
	batchSteps := p.Steps[batchStart:batchEnd]

	// ───────── Modo MINI-TEST: verificar todos los pasos del batch a la vez.
	if p.InMinitest {
		var failed []string
		for i := batchStart; i < batchEnd; i++ {
			report, verr := VerifyFile(filePath, p.Steps[i])
			if verr != nil {
				return nil, verr
			}
			if report.Passed {
				p.Steps[i].Done = true
			} else {
				p.Steps[i].Done = false
				failed = append(failed, fmt.Sprintf("paso %d (%s): %s",
					p.Steps[i].ID, p.Steps[i].Instruction, report.Missing))
			}
		}
		p.InMinitest = false

		if len(failed) == 0 {
			// Mini-test pasado: snapshot del archivo y avanzar al siguiente batch.
			if content, rerr := os.ReadFile(filePath); rerr == nil {
				p.Checkpoint = string(content)
				p.CheckpointFile = filePath
			}
			p.BatchIndex++
			newStart, newEnd := batchRange(p.BatchIndex, batchSize, len(p.Steps))
			if newStart >= len(p.Steps) {
				p.Active = false
				p.CompletedAll = true
				p.InTest = false
				_ = c.saveProgress(p)
				childCtx.Decision("proyecto-completado", p.Goal)
				return &Result{
					Success: true,
					Message: fmt.Sprintf("✓ Mini-test pasado. ¡Proyecto '%s' completado!", p.Goal),
					Data:    p,
				}, nil
			}
			var pending []string
			for i := newStart; i < newEnd; i++ {
				pending = append(pending, p.Steps[i].Instruction)
			}
			p.PendingTest = pending
			p.CurrentStep = p.Steps[newStart].ID
			p.InTest = true
			_ = c.saveProgress(p)
			childCtx.Decision("minitest-pasado", fmt.Sprintf("batch %d", p.BatchIndex-1))
			firstStep := p.Steps[newStart]
			return &Result{
				Success: true,
				Message: fmt.Sprintf("✓ Mini-test %d pasado. Continuá con el paso %d: %s",
					p.BatchIndex-1, firstStep.ID, firstStep.Instruction),
				Data: p,
			}, nil
		}

		// Mini-test fallido: dejar currentStep en el primer fallado y volver al modo normal.
		for _, s := range p.Steps {
			if s.ID >= batchSteps[0].ID && s.ID <= batchSteps[len(batchSteps)-1].ID && !s.Done {
				p.CurrentStep = s.ID
				break
			}
		}
		_ = c.saveProgress(p)
		childCtx.Decision("minitest-fallido", fmt.Sprintf("%d pasos", len(failed)))
		return &Result{
			Success: false,
			Message: fmt.Sprintf("✗ Mini-test %d falló. Repetí los pasos fallados (uno por uno) y al completar el batch se intentará otro mini-test.", p.BatchIndex),
			Data: map[string]interface{}{
				"failed":     failed,
				"batchIndex": p.BatchIndex,
				"progress":   p,
			},
		}, nil
	}

	// ───────── Modo NORMAL: verificar el paso actual.
	var current Step
	for _, s := range p.Steps {
		if s.ID == p.CurrentStep {
			current = s
			break
		}
	}
	if current.ID == 0 {
		return nil, fmt.Errorf("no hay paso actual (currentStep=%d)", p.CurrentStep)
	}

	// Rewrite guard: detect if the file was wholesale replaced since last scan/verify.
	// A legitimate edit adds 1-2 declarations per step. A rewrite removes many and adds new ones.
	if len(p.LastVerifyDecls) > 0 {
		currentDecls := ExtractDeclNames(filePath)
		lastSet := make(map[string]bool)
		for _, n := range p.LastVerifyDecls {
			lastSet[n] = true
		}
		currentSet := make(map[string]bool)
		for _, n := range currentDecls {
			currentSet[n] = true
		}

		var removed int
		var newNames []string
		for _, n := range p.LastVerifyDecls {
			if n == "main" || n == "init" {
				continue
			}
			if !currentSet[n] {
				removed++
			}
		}
		for _, n := range currentDecls {
			if n == "main" || n == "init" {
				continue
			}
			if !lastSet[n] {
				newNames = append(newNames, n)
			}
		}

		if removed >= 2 && len(newNames) >= 1 {
			// Likely a wholesale rewrite — don't update LastVerifyDecls so it triggers again.
			childCtx.Decision("rewrite-detectado", fmt.Sprintf("removed=%d new=%v", removed, newNames))
			return &Result{
				Success: false,
				Message: fmt.Sprintf("⚠️ Reescritura detectada: desaparecieron %d declaraciones y aparecieron nuevas (%s). "+
					"Si la IA editó el archivo, revertí los cambios — solo el usuario escribe código. "+
					"Si el usuario lo hizo, ejecutá './pingpong scan --file %s' para re-evaluar el progreso.",
					removed, strings.Join(newNames, ", "), filePath),
				Data: map[string]interface{}{
					"rewrite_detected": true,
					"new_declarations": newNames,
					"removed_count":    removed,
				},
			}, nil
		}

		// No rewrite: update baseline for next verify.
		p.LastVerifyDecls = currentDecls
	}

	report, verr := VerifyFile(filePath, current)
	if verr != nil {
		return nil, verr
	}

	if !report.Passed {
		// Incrementar fail_count del paso actual
		for i := range p.Steps {
			if p.Steps[i].ID == current.ID {
				p.Steps[i].FailCount++
				current = p.Steps[i]
				break
			}
		}
		_ = c.saveProgress(p)

		msg := fmt.Sprintf("✗ Paso %d incompleto: %s", current.ID, report.Missing)
		if current.FailCount >= 3 {
			msg += fmt.Sprintf(" [fail_count=%d → considerá subdividir este paso con ./pingpong subdivide]", current.FailCount)
		}
		childCtx.Decision("paso-no-cumplido", fmt.Sprintf("step %d (fail %d): %s", current.ID, current.FailCount, report.Missing))
		return &Result{
			Success: false,
			Message: msg,
			Data:    report,
		}, nil
	}

	// Marcar paso done, resetear fail_count, y avanzar currentStep.
	for i := range p.Steps {
		if p.Steps[i].ID == current.ID {
			p.Steps[i].Done = true
			p.Steps[i].FailCount = 0
			break
		}
	}
	if current.ID < len(p.Steps) {
		p.CurrentStep = current.ID + 1
	}

	// ¿Quedó el batch entero done? Si sí, disparar mini-test.
	batchAllDone := true
	for i := batchStart; i < batchEnd; i++ {
		if !p.Steps[i].Done {
			batchAllDone = false
			break
		}
	}

	if batchAllDone {
		// Restaurar archivo al checkpoint del último mini-test pasado.
		// Si no hay checkpoint (primer batch), truncar a 0.
		// Esto preserva el código aprobado de batches anteriores.
		var resetMode string
		if p.Checkpoint != "" && p.CheckpointFile == filePath {
			if err := os.WriteFile(filePath, []byte(p.Checkpoint), 0644); err != nil {
				return nil, fmt.Errorf("no se pudo restaurar checkpoint: %w", err)
			}
			resetMode = fmt.Sprintf("restaurado al checkpoint del Mini-test %d", p.BatchIndex-1)
		} else {
			if err := truncateFile(filePath); err != nil {
				return nil, fmt.Errorf("no se pudo truncar archivo para mini-test: %w", err)
			}
			resetMode = "vaciado (no hay batches previos aprobados)"
		}
		var batchInstructions []string
		for i := batchStart; i < batchEnd; i++ {
			p.Steps[i].Done = false
			batchInstructions = append(batchInstructions, p.Steps[i].Instruction)
		}
		p.InMinitest = true
		p.CurrentStep = p.Steps[batchStart].ID
		_ = c.saveProgress(p)
		childCtx.Decision("minitest-iniciado", fmt.Sprintf("batch %d, %d pasos, %s", p.BatchIndex, len(batchInstructions), resetMode))
		return &Result{
			Success: true,
			Message: fmt.Sprintf("🎯 Batch %d completado. INICIO MINI-TEST: archivo %s %s. Reescribí estos %d pasos: %v.",
				p.BatchIndex, filepath.Base(filePath), resetMode, len(batchInstructions), batchInstructions),
			Data: map[string]interface{}{
				"inMinitest": true,
				"batchIndex": p.BatchIndex,
				"steps":      batchInstructions,
				"file":       filePath,
				"resetMode":  resetMode,
				"progress":   p,
			},
		}, nil
	}

	_ = c.saveProgress(p)
	childCtx.Decision("paso-cumplido", fmt.Sprintf("step %d", current.ID))
	return &Result{
		Success: true,
		Message: fmt.Sprintf("✓ Paso %d cumplido (%s).", current.ID, current.Instruction),
		Data:    report,
	}, nil
}

// batchRange devuelve [start, end) de los índices de p.Steps que pertenecen al batchIndex 1-based.
func batchRange(batchIndex, batchSize, total int) (int, int) {
	start := (batchIndex - 1) * batchSize
	end := start + batchSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return start, end
}

// truncateFile vacía el archivo (deja 0 bytes) preservando permisos.
// Si no existe, lo crea vacío.
func truncateFile(path string) error {
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// generateMiniTest ya no hardcodea ejercicios; devuelve vacío.
// Los mini-tests se derivan de los pasos que la IA definió con set-steps.
func generateMiniTest(goal string) []string {
	_ = goal
	return nil
}

// generateStepsFromGoal genera un placeholder genérico cuando la IA no llamó a set-steps.
// El framework es agnóstico al problema: los pasos reales los genera la IA según el goal,
// pasando --steps en start o llamando a set-steps después.
func generateStepsFromGoal(goal string) []Step {
	_ = goal
	return []Step{
		{ID: 1, Instruction: "Definir pasos del problema con set-steps", Done: false},
	}
}


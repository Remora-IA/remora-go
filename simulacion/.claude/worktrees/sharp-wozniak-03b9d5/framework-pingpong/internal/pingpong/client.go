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
	File        string `json:"file,omitempty"` // archivo que este paso modifica
	Instruction string `json:"instruction"`
	Done        bool   `json:"done"`
	FailCount   int    `json:"fail_count,omitempty"`
}

// Detour representa un desvío de sub-pasos cuando el usuario se traba.
// Los sub-pasos se resuelven sin modificar la lista principal de steps.
// Al completar todos, el step padre se marca done automáticamente.
type Detour struct {
	ParentStepID int    `json:"parentStepID"`
	Steps        []Step `json:"steps"`
	CurrentStep  int    `json:"currentStep"` // 1-based dentro del detour
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
	Type string `json:"type"`
	Note string `json:"note,omitempty"`
	At   string `json:"at"`
}

// Progress representa el estado del aprendizaje
type Progress struct {
	Goal             string            `json:"goal"`
	Root             string            `json:"root,omitempty"`
	Files            []string          `json:"files,omitempty"`
	CurrentStep      int               `json:"currentStep"`
	Steps            []Step            `json:"steps"`
	QALog            []QA              `json:"qaLog"`
	Signals          []Signal          `json:"signals"`
	StartedAt        time.Time         `json:"startedAt"`
	Active           bool              `json:"active"`
	BatchSize        int               `json:"batchSize"`             // Pasos por mini-test (default 3)
	BatchIndex       int               `json:"batchIndex"`            // Batch actual (1-based)
	PassedBatches    int               `json:"passedBatches"`         // Batches con mini-test aprobado (nunca regresa)
	MiniTestAttempts int               `json:"miniTestAttempts"`      // Intentos del mini-test actual (auto-pass a 2)
	InMinitest       bool              `json:"inMinitest"`            // Si estamos en modo mini-test
	Checkpoints      map[string]string `json:"checkpoints,omitempty"` // file → contenido al pasar el último mini-test
	LastVerifyHash   string            `json:"lastVerifyHash,omitempty"`
	Detour           *Detour           `json:"detour,omitempty"` // Desvío de sub-pasos activo
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
	CompileOK  bool   `json:"compile_ok"`
	CompileLog string `json:"compile_log,omitempty"`
}

// BatchInfo expone solo el batch actual al tutor IA, evitando confusión.
type BatchInfo struct {
	Index            int         `json:"index"`
	Steps            []BatchStep `json:"steps"`
	CurrentBatchStep int         `json:"currentBatchStep"`
	TotalBatches     int         `json:"totalBatches"`
}

// BatchStep es un paso dentro de un batch, con numeración relativa (1-3).
type BatchStep struct {
	BatchNum    int    `json:"batchStep"`
	File        string `json:"file,omitempty"`
	Instruction string `json:"instruction"`
	Done        bool   `json:"done"`
	GlobalID    int    `json:"globalID"`
}

// Client es el cliente principal del framework.
type Client struct {
	trace   *paladin.Trace
	ctx     *paladin.Context
	baseDir string
	Lang    LangConfig
}

// New crea un nuevo cliente.
func New() *Client {
	return NewWithTrace("framework-pingpong", DefaultLangConfigs["go"])
}

// NewWithTrace crea un cliente con tracing activo.
func NewWithTrace(name string, lang ...LangConfig) *Client {
	trace := paladin.NewTrace(name)
	ctx := trace.Start()
	lc := DefaultLangConfigs["go"]
	if len(lang) > 0 {
		lc = lang[0]
	}
	return &Client{trace: trace, ctx: ctx, baseDir: getBaseDir(), Lang: lc}
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

func (c *Client) Configure(root string, filesArg string) (*Result, error) {
	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	root = filepath.Clean(root)
	var files []string
	for _, part := range strings.Split(filesArg, ",") {
		f := filepath.Clean(strings.TrimSpace(part))
		if f != "" && f != "." {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("configure requiere --files archivo1,archivo2")
	}
	p.Root = root
	p.Files = files
	if len(p.Steps) > 0 {
		normalized, err := applyScopeToSteps(p, p.Steps)
		if err != nil {
			return nil, err
		}
		p.Steps = normalized
	}
	if err := c.saveProgress(p); err != nil {
		return nil, err
	}
	return &Result{
		Success: true,
		Message: fmt.Sprintf("Scope configurado: root=%s, files=%s", root, strings.Join(files, ",")),
		Data: map[string]interface{}{
			"root":               root,
			"files":              files,
			"allowed_step_files": files,
		},
	}, nil
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
		steps = parseSteps(stepsArg)
	} else {
		steps = generateStepsFromGoal(goal)
	}

	if len(steps) == 0 {
		steps = []Step{{ID: 1, Instruction: "Definir objetivo", Done: false}}
	}

	batchSize := 3

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
	}

	if err := c.saveProgress(progress); err != nil {
		return nil, err
	}

	childCtx.Decision("proyecto-iniciado", goal)
	batchInfo := buildBatchInfo(progress)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Iniciado: %s. Batch 1 de %d.", goal, batchInfo.TotalBatches),
		Data: map[string]interface{}{
			"progress":     progress,
			"currentBatch": batchInfo,
		},
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

// Done marca un paso como completado y maneja avance de batch/detour/minitest.
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

	var id int
	if _, err := fmt.Sscanf(stepID, "%d", &id); err != nil {
		return nil, fmt.Errorf("ID inválido: %s", stepID)
	}

	bs := effectiveBatchSize(p)
	batchStart, batchEnd := batchRange(p.BatchIndex, bs, len(p.Steps))

	// ───────── Modo DETOUR
	if p.Detour != nil {
		d := p.Detour
		// Mark the detour sub-step done
		if id >= 1 && id <= len(d.Steps) {
			d.Steps[id-1].Done = true
			d.Steps[id-1].FailCount = 0
		}

		// Find next undone sub-step
		nextSub := -1
		for i := range d.Steps {
			if !d.Steps[i].Done {
				nextSub = i + 1
				break
			}
		}

		if nextSub == -1 {
			// All sub-steps done → mark parent done, clear detour
			parentInstruction := ""
			for i := range p.Steps {
				if p.Steps[i].ID == d.ParentStepID {
					p.Steps[i].Done = true
					p.Steps[i].FailCount = 0
					parentInstruction = p.Steps[i].Instruction
					break
				}
			}
			p.Detour = nil

			nextID := nextUndoneInBatch(p.Steps, batchStart, batchEnd)
			if nextID == -1 {
				return c.handleBatchComplete(p, bs, batchStart, batchEnd, childCtx)
			}

			p.CurrentStep = nextID
			_ = c.saveProgress(p)
			batchInfo := buildBatchInfo(p)
			childCtx.Decision("detour-completado", fmt.Sprintf("parent step %d done", d.ParentStepID))
			return &Result{
				Success: true,
				Message: fmt.Sprintf("✓ Desvío completado, paso %d cumplido (%s). Siguiente: batch %d, paso %d/%d.",
					d.ParentStepID, parentInstruction, batchInfo.Index, batchInfo.CurrentBatchStep, len(batchInfo.Steps)),
				Data: map[string]interface{}{
					"currentBatch":    batchInfo,
					"overallProgress": overallProgress(p),
				},
			}, nil
		}

		d.CurrentStep = nextSub
		_ = c.saveProgress(p)
		childCtx.Decision("detour-avance", fmt.Sprintf("substep → %d", nextSub))
		return &Result{
			Success: true,
			Message: fmt.Sprintf("✓ Sub-paso %d/%d cumplido. Siguiente sub-paso %d/%d: %s",
				id, len(d.Steps), nextSub, len(d.Steps), d.Steps[nextSub-1].Instruction),
			Data: map[string]interface{}{
				"detour":       d,
				"currentBatch": buildBatchInfo(p),
			},
		}, nil
	}

	// ───────── Modo NORMAL / MINITEST: mark the main step done
	found := false
	for i, s := range p.Steps {
		if s.ID == id {
			p.Steps[i].Done = true
			p.Steps[i].FailCount = 0
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("paso %d no existe", id)
	}

	childCtx.Var("step_id", id)
	childCtx.Decision("paso-completado", fmt.Sprintf("paso %d", id))

	// ───────── Check if batch is complete
	nextID := nextUndoneInBatch(p.Steps, batchStart, batchEnd)
	if nextID == -1 {
		// All steps in batch done
		if p.InMinitest {
			// Minitest passed → advance to next batch
			return c.advanceToNextBatch(p, bs, batchStart, batchEnd, childCtx)
		}
		return c.handleBatchComplete(p, bs, batchStart, batchEnd, childCtx)
	}

	// Advance to next undone step in batch
	p.CurrentStep = nextID
	_ = c.saveProgress(p)
	batchInfo := buildBatchInfo(p)
	nextStep := p.Steps[nextID-1]
	fileLabel := ""
	if nextStep.File != "" {
		fileLabel = fmt.Sprintf(" [%s]", filepath.Base(nextStep.File))
	}
	return &Result{
		Success: true,
		Message: fmt.Sprintf("✓ Paso %d cumplido. Siguiente: batch %d, paso %d/%d%s — %s",
			id, batchInfo.Index, batchInfo.CurrentBatchStep, len(batchInfo.Steps), fileLabel, nextStep.Instruction),
		Data: map[string]interface{}{
			"currentBatch":    batchInfo,
			"overallProgress": overallProgress(p),
		},
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
		"active":   p.Active,
		"goal":     p.Goal,
		"progress": fmt.Sprintf("[%d/%d]", doneCount, len(p.Steps)),
		"nextStep": p.CurrentStep,
		"steps":    p.Steps,
		"qaLog":    p.QALog,
		"signals":  p.Signals,
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

	rep, err := RunFile(filePath, stdin, expected, c.Lang)
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

// Clean is a placeholder — noise detection was AST-specific and has been removed.
// The AI tutor now handles code quality judgment directly.
func (c *Client) Clean(filePath string, fromLine int, toLine int) (*Result, error) {
	if filePath == "" {
		return nil, fmt.Errorf("clean requiere --file <archivo>")
	}
	if fromLine < 1 || toLine < fromLine {
		return nil, fmt.Errorf("clean requiere --from N --to M con 1 <= N <= M")
	}

	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", abs, err)
	}

	text := string(src)
	hadFinalNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if hadFinalNewline {
		lines = lines[:len(lines)-1]
	}
	if fromLine > len(lines) {
		return nil, fmt.Errorf("rango fuera del archivo: --from %d, total líneas %d", fromLine, len(lines))
	}
	if toLine > len(lines) {
		toLine = len(lines)
	}

	deleted := append([]string(nil), lines[fromLine-1:toLine]...)
	remaining := append([]string{}, lines[:fromLine-1]...)
	remaining = append(remaining, lines[toLine:]...)
	out := strings.Join(remaining, "\n")
	if hadFinalNewline && len(remaining) > 0 {
		out += "\n"
	}
	if err := os.WriteFile(abs, []byte(out), 0644); err != nil {
		return nil, fmt.Errorf("no se pudo escribir %s: %w", abs, err)
	}

	report := CompileCheck(abs, c.Lang)
	return &Result{
		Success: true,
		Message: fmt.Sprintf("Clean borró líneas %d-%d de %s. Solo se eliminó contenido; no se agregó código.", fromLine, toLine, filepath.Base(abs)),
		Data: map[string]interface{}{
			"file":        abs,
			"from":        fromLine,
			"to":          toLine,
			"deleted":     deleted,
			"compile_ok":  report.CompileOK,
			"compile_log": report.CompileLog,
			"snippet":     report.Snippet,
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

	steps := parseSteps(stepsArg)

	if len(steps) == 0 {
		return nil, fmt.Errorf("no se proporcionaron pasos")
	}
	if p.Root != "" || len(p.Files) > 0 {
		steps, err = applyScopeToSteps(p, steps)
		if err != nil {
			return nil, err
		}
	}

	p.Steps = steps
	p.CurrentStep = 1
	p.Active = true
	p.BatchIndex = 1
	p.PassedBatches = 0
	p.MiniTestAttempts = 0
	p.Detour = nil

	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Decision("pasos-registrados", fmt.Sprintf("%d pasos", len(steps)))
	batchInfo := buildBatchInfo(p)

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Pasos registrados (%d). Batch 1 de %d.", len(steps), batchInfo.TotalBatches),
		Data: map[string]interface{}{
			"steps":        steps,
			"currentBatch": batchInfo,
		},
	}, nil
}

// Subdivide crea un desvío (detour) de sub-pasos para un paso que el usuario no logra.
// NO modifica la lista principal de steps. Al completar todos los sub-pasos,
// el step padre se marca done automáticamente y el flujo principal continúa.
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
	if len(substeps) > 3 {
		return nil, fmt.Errorf("máximo 3 sub-pasos por subdivisión")
	}

	// Buscar el paso a subdividir
	var parentStep *Step
	for i := range p.Steps {
		if p.Steps[i].ID == stepID {
			parentStep = &p.Steps[i]
			break
		}
	}
	if parentStep == nil {
		return nil, fmt.Errorf("paso %d no encontrado", stepID)
	}
	if parentStep.Done {
		return nil, fmt.Errorf("paso %d ya completado, no se puede subdividir", stepID)
	}

	// Crear detour con sub-pasos numerados 1..N
	var detourSteps []Step
	for i, sub := range substeps {
		detourSteps = append(detourSteps, Step{ID: i + 1, Instruction: sub, Done: false})
	}
	p.Detour = &Detour{
		ParentStepID: stepID,
		Steps:        detourSteps,
		CurrentStep:  1,
	}

	if err := c.saveProgress(p); err != nil {
		return nil, err
	}

	childCtx.Decision("detour-iniciado", fmt.Sprintf("step %d → %d sub-pasos", stepID, len(substeps)))

	return &Result{
		Success: true,
		Message: fmt.Sprintf("Desvío para paso %d (%s). Sub-paso 1/%d: %s",
			stepID, parentStep.Instruction, len(substeps), substeps[0]),
		Data: map[string]interface{}{
			"detour":       p.Detour,
			"parentStep":   parentStep.Instruction,
			"currentBatch": buildBatchInfo(p),
		},
	}, nil
}

// Scan runs a compile check on the relevant files and reports progress.
// fileOverride (de --file) se usa como fallback para pasos sin File asignado.
// No auto-marca pasos como done — eso lo hace la IA via done.
func (c *Client) Scan(fileOverride string, auto bool) (*Result, error) {
	childCtx := c.ctx.Child("Scan")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}
	if len(p.Steps) <= 1 {
		return nil, fmt.Errorf("primero registrá los pasos con set-steps. Orden: start → set-steps → scan")
	}

	// Verificar que todos los pasos tengan archivo (propio o via override)
	for _, s := range p.Steps {
		if s.File == "" && fileOverride == "" {
			return nil, fmt.Errorf("paso %d (%s) no tiene archivo asignado. Usá [archivo]instrucción en set-steps o --file como fallback", s.ID, s.Instruction)
		}
	}

	var autoAccepted []string
	if auto {
		for i := range p.Steps {
			if p.Steps[i].Done {
				continue
			}
			filePath, err := resolveFile(p.Steps[i], fileOverride)
			if err != nil {
				break
			}
			ok, reason := autoAcceptStep(filePath, p.Steps[i], c.Lang)
			if !ok {
				break
			}
			p.Steps[i].Done = true
			p.Steps[i].FailCount = 0
			autoAccepted = append(autoAccepted, fmt.Sprintf("%s: %s", displayStepLabel(p, p.Steps[i]), reason))
		}
	}

	batchSize := effectiveBatchSize(p)

	// Count already-done steps
	var passed []int
	for i := range p.Steps {
		if p.Steps[i].Done {
			passed = append(passed, p.Steps[i].ID)
		}
	}

	// Buscar el primer paso no cumplido
	firstUndone := -1
	for i := range p.Steps {
		if !p.Steps[i].Done {
			firstUndone = i
			break
		}
	}

	// Compile check on first relevant file
	scanFile := fileOverride
	if scanFile == "" && firstUndone >= 0 && p.Steps[firstUndone].File != "" {
		scanFile = p.Steps[firstUndone].File
	}
	compileOK, compileLog := true, ""
	var fileContent string
	if scanFile != "" {
		report := CompileCheck(scanFile, c.Lang)
		compileOK = report.CompileOK
		compileLog = report.CompileLog
		fileContent = report.FileContent
		p.LastVerifyHash = report.FileHash
	}

	if firstUndone == -1 {
		p.Active = false
		_ = c.saveProgress(p)
		childCtx.Decision("scan-completo", fmt.Sprintf("%d pasos, todos cumplidos", len(p.Steps)))
		msg := "Scan: todos los pasos ya están cumplidos. Pasá a la fase final (run)."
		if !compileOK {
			msg += fmt.Sprintf(" ⚠️ ADVERTENCIA: el código no compila: %s", compileLog)
		}
		return &Result{
			Success: true,
			Message: msg,
			Data:    &ScanResult{Progress: p, CompileOK: compileOK, CompileLog: compileLog},
		}, nil
	}

	// Avanzar batch al que corresponde el primer paso pendiente
	p.CurrentStep = p.Steps[firstUndone].ID
	newBatchIndex := (firstUndone / batchSize) + 1
	p.BatchIndex = newBatchIndex

	if newBatchIndex > 1 {
		p.PassedBatches = newBatchIndex - 1
		batchStart, batchEnd := batchRange(newBatchIndex, batchSize, len(p.Steps))
		saveCheckpoints(p, batchStart, batchEnd)
	}

	p.InMinitest = false
	p.MiniTestAttempts = 0
	p.Detour = nil

	_ = c.saveProgress(p)

	step := p.Steps[firstUndone]
	batchInfo := buildBatchInfo(p)
	childCtx.Decision("scan-avanzado", fmt.Sprintf("%d cumplidos, continuar paso %d", len(passed), step.ID))

	fileLabel := ""
	if step.File != "" {
		fileLabel = fmt.Sprintf(" [%s]", filepath.Base(step.File))
	}
	msg := fmt.Sprintf("Scan: %d pasos ya cumplidos. Batch %d de %d, paso %d/%d%s: %s",
		len(passed), batchInfo.Index, batchInfo.TotalBatches, batchInfo.CurrentBatchStep, len(batchInfo.Steps), fileLabel, step.Instruction)
	if !compileOK {
		msg += fmt.Sprintf(" ⚠️ ADVERTENCIA: el código no compila: %s", compileLog)
	}

	return &Result{
		Success: true,
		Message: msg,
		Data: map[string]interface{}{
			"scan":            &ScanResult{Progress: p, CompileOK: compileOK, CompileLog: compileLog},
			"auto_accepted":   autoAccepted,
			"currentBatch":    batchInfo,
			"overallProgress": overallProgress(p),
			"file_content":    fileContent,
		},
	}, nil
}

// Verify runs a compile check on the current step's file and returns context for AI judgment.
// The AI reads the result and calls `done --step N` when it judges the step is complete.
func (c *Client) Verify(fileOverride string) (*Result, error) {
	childCtx := c.ctx.Child("Verify")
	defer childCtx.End()

	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}

	current, err := currentStepForProgress(p)
	if err != nil {
		return nil, err
	}

	filePath, ferr := resolveFile(current, fileOverride)
	if ferr != nil {
		return nil, ferr
	}

	// Compile check
	report := CompileCheck(filePath, c.Lang)
	inspection := inspectStep(filePath, current, c.Lang)

	// Rewrite guard via hash
	if p.LastVerifyHash != "" && report.FileHash != "" && p.LastVerifyHash != report.FileHash {
		// Hash changed — normal, just track it. Major rewrites are for AI to judge.
	}
	p.LastVerifyHash = report.FileHash
	_ = c.saveProgress(p)

	batchInfo := buildBatchInfo(p)

	if !report.CompileOK {
		childCtx.Decision("verify-compile-error", report.CompileLog)
		return &Result{
			Success: false,
			Message: fmt.Sprintf("❌ No compila: %s. Igual revisá data.inspection: puede haber evidencia de que el paso actual sí está cumplido.", report.CompileLog),
			Data: map[string]interface{}{
				"report":          report,
				"inspection":      inspection,
				"step":            current,
				"currentBatch":    batchInfo,
				"overallProgress": overallProgress(p),
				"action_required": "judge_step_from_evidence_and_compile_diagnostics",
			},
		}, nil
	}

	// Compile OK — return context for AI to judge
	childCtx.Decision("verify-compile-ok", fmt.Sprintf("step %d", current.ID))
	modeLabel := "normal"
	if p.Detour != nil {
		modeLabel = fmt.Sprintf("detour sub-paso %d/%d", p.Detour.CurrentStep, len(p.Detour.Steps))
	} else if p.InMinitest {
		modeLabel = "mini-test"
	}

	return &Result{
		Success: true,
		Message: fmt.Sprintf("✓ Compilación OK (%s). Juzgá si el paso está cumplido: \"%s\"", modeLabel, current.Instruction),
		Data: map[string]interface{}{
			"report":          report,
			"inspection":      inspection,
			"step":            current,
			"mode":            modeLabel,
			"action_required": "judge",
			"currentBatch":    batchInfo,
			"overallProgress": overallProgress(p),
		},
	}, nil
}

// handleBatchComplete maneja la lógica cuando todos los pasos del batch están done.
// Decide si disparar mini-test, auto-aprobar, o avanzar.
func (c *Client) handleBatchComplete(p *Progress, bs, batchStart, batchEnd int, childCtx *paladin.Context) (*Result, error) {
	// Si ya se intentó el mini-test 2+ veces, auto-aprobar
	if p.MiniTestAttempts >= 2 {
		childCtx.Decision("batch-auto-aprobado", fmt.Sprintf("batch %d, %d intentos", p.BatchIndex, p.MiniTestAttempts))
		return c.advanceToNextBatch(p, bs, batchStart, batchEnd, childCtx)
	}

	// Disparar mini-test: restaurar archivos del batch a su checkpoint
	resetMode, err := restoreCheckpoints(p, batchStart, batchEnd)
	if err != nil {
		return nil, err
	}
	if resetMode == "" {
		resetMode = "sin archivos modificados"
	}

	var batchInstructions []string
	files := batchFiles(p.Steps, batchStart, batchEnd)
	for i := batchStart; i < batchEnd; i++ {
		p.Steps[i].Done = false
		label := ""
		if len(files) > 1 && p.Steps[i].File != "" {
			label = fmt.Sprintf("[%s] ", filepath.Base(p.Steps[i].File))
		}
		batchInstructions = append(batchInstructions, fmt.Sprintf("%d/%d: %s%s",
			i-batchStart+1, batchEnd-batchStart, label, p.Steps[i].Instruction))
	}
	p.InMinitest = true
	p.CurrentStep = p.Steps[batchStart].ID
	_ = c.saveProgress(p)

	childCtx.Decision("minitest-iniciado", fmt.Sprintf("batch %d, %s", p.BatchIndex, resetMode))
	return &Result{
		Success: true,
		Message: fmt.Sprintf("🎯 Batch %d completado. MINI-TEST: archivos %s. Reescribí estos %d pasos desde cero.",
			p.BatchIndex, resetMode, len(batchInstructions)),
		Data: map[string]interface{}{
			"inMinitest":      true,
			"batchIndex":      p.BatchIndex,
			"steps":           batchInstructions,
			"files":           files,
			"fileModified":    true,
			"resetMode":       resetMode,
			"overallProgress": overallProgress(p),
		},
	}, nil
}

// advanceToNextBatch avanza al siguiente batch (después de mini-test pasado o auto-aprobación).
func (c *Client) advanceToNextBatch(p *Progress, bs, batchStart, batchEnd int, childCtx *paladin.Context) (*Result, error) {
	// Snapshot de todos los archivos del batch como checkpoint
	saveCheckpoints(p, batchStart, batchEnd)

	p.PassedBatches = p.BatchIndex
	p.BatchIndex++
	p.InMinitest = false
	p.MiniTestAttempts = 0

	newStart, newEnd := batchRange(p.BatchIndex, bs, len(p.Steps))
	if newStart >= len(p.Steps) {
		// Todos los batches completados
		p.Active = false
		_ = c.saveProgress(p)
		childCtx.Decision("proyecto-completado", p.Goal)
		return &Result{
			Success: true,
			Message: fmt.Sprintf("✓ Batch %d aprobado. ¡Todos los pasos completados! Pasá a la fase final (run).", p.BatchIndex-1),
			Data: map[string]interface{}{
				"completedAll":    true,
				"overallProgress": overallProgress(p),
			},
		}, nil
	}

	p.CurrentStep = p.Steps[newStart].ID
	_ = c.saveProgress(p)

	batchInfo := buildBatchInfo(p)
	childCtx.Decision("batch-avanzado", fmt.Sprintf("→ batch %d", p.BatchIndex))

	firstStep := p.Steps[newStart]
	return &Result{
		Success: true,
		Message: fmt.Sprintf("✓ Batch %d aprobado. Avanzando al batch %d de %d. Paso 1/%d: %s",
			p.BatchIndex-1, batchInfo.Index, batchInfo.TotalBatches, newEnd-newStart, firstStep.Instruction),
		Data: map[string]interface{}{
			"currentBatch":    batchInfo,
			"overallProgress": overallProgress(p),
		},
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

// generateStepsFromGoal genera un placeholder genérico cuando la IA no llamó a set-steps.
func generateStepsFromGoal(goal string) []Step {
	_ = goal
	return []Step{
		{ID: 1, Instruction: "Definir pasos del problema con set-steps", Done: false},
	}
}

// effectiveBatchSize retorna el batchSize efectivo (default 3).
func effectiveBatchSize(p *Progress) int {
	if p.BatchSize > 0 {
		return p.BatchSize
	}
	return 3
}

// buildBatchInfo construye la vista de batch actual para el tutor IA.
func buildBatchInfo(p *Progress) *BatchInfo {
	bs := effectiveBatchSize(p)
	start, end := batchRange(p.BatchIndex, bs, len(p.Steps))
	totalBatches := (len(p.Steps) + bs - 1) / bs

	var steps []BatchStep
	currentBatchStep := 0
	for i := start; i < end; i++ {
		bsNum := i - start + 1
		steps = append(steps, BatchStep{
			BatchNum:    bsNum,
			File:        p.Steps[i].File,
			Instruction: p.Steps[i].Instruction,
			Done:        p.Steps[i].Done,
			GlobalID:    p.Steps[i].ID,
		})
		if p.Steps[i].ID == p.CurrentStep {
			currentBatchStep = bsNum
		}
	}

	return &BatchInfo{
		Index:            p.BatchIndex,
		Steps:            steps,
		CurrentBatchStep: currentBatchStep,
		TotalBatches:     totalBatches,
	}
}

// overallProgress retorna "done/total" para el progreso general.
func overallProgress(p *Progress) string {
	done := 0
	for _, s := range p.Steps {
		if s.Done {
			done++
		}
	}
	return fmt.Sprintf("%d/%d", done, len(p.Steps))
}

// nextUndoneInBatch retorna el ID del siguiente step no-done dentro del rango [batchStart, batchEnd).
// Retorna -1 si todos están done.
func nextUndoneInBatch(steps []Step, batchStart, batchEnd int) int {
	for i := batchStart; i < batchEnd; i++ {
		if !steps[i].Done {
			return steps[i].ID
		}
	}
	return -1
}

// parseSteps parsea la cadena de pasos separados por ; con formato opcional [archivo]instrucción.
func parseSteps(stepsArg string) []Step {
	stepList := strings.Split(stepsArg, ";")
	var steps []Step
	for _, inst := range stepList {
		inst = strings.TrimSpace(inst)
		if inst == "" {
			continue
		}
		s := Step{Done: false}
		if strings.HasPrefix(inst, "[") {
			if idx := strings.Index(inst, "]"); idx > 1 {
				s.File = inst[1:idx]
				s.Instruction = strings.TrimSpace(inst[idx+1:])
			} else {
				s.Instruction = inst
			}
		} else {
			s.Instruction = inst
		}
		if s.Instruction != "" {
			steps = append(steps, s)
		}
	}
	for i := range steps {
		steps[i].ID = i + 1
	}
	return steps
}

func applyScopeToSteps(p *Progress, steps []Step) ([]Step, error) {
	if p.Root == "" && len(p.Files) == 0 {
		return steps, nil
	}
	allowed := map[string]bool{}
	for _, f := range p.Files {
		allowed[filepath.Clean(f)] = true
	}
	root := filepath.Clean(p.Root)
	var out []Step
	for _, s := range steps {
		if s.File == "" {
			return nil, fmt.Errorf("paso %d (%s) no tiene archivo. Con scope configurado usá [archivo]instrucción", s.ID, s.Instruction)
		}
		rel := filepath.Clean(s.File)
		if root != "." {
			prefix := root + string(os.PathSeparator)
			if strings.HasPrefix(rel, prefix) {
				rel = strings.TrimPrefix(rel, prefix)
			}
		}
		if len(allowed) > 0 && !allowed[rel] {
			return nil, fmt.Errorf("archivo no permitido: %s. Archivos activos: %s", s.File, strings.Join(p.Files, ", "))
		}
		if root != "." {
			s.File = filepath.Join(root, rel)
		} else {
			s.File = rel
		}
		out = append(out, s)
	}
	return out, nil
}

// resolveFile retorna el archivo a verificar: usa el override del CLI si se proporcionó,
// sino el File del step. Si ninguno, retorna error.
func resolveFile(step Step, cliOverride string) (string, error) {
	if cliOverride != "" {
		return cliOverride, nil
	}
	if step.File != "" {
		return step.File, nil
	}
	return "", fmt.Errorf("paso %d no tiene archivo asignado. Usá --file o el formato [archivo]instrucción en set-steps", step.ID)
}

func displayStepLabel(p *Progress, step Step) string {
	bs := effectiveBatchSize(p)
	idx := step.ID - 1
	batchStep := (idx % bs) + 1
	batchStart, batchEnd := batchRange((idx/bs)+1, bs, len(p.Steps))
	fileLabel := ""
	if step.File != "" {
		fileLabel = fmt.Sprintf(" [%s]", filepath.Base(step.File))
	}
	return fmt.Sprintf("Paso %d/%d%s", batchStep, batchEnd-batchStart, fileLabel)
}

// saveCheckpoints guarda el contenido de cada archivo único del batch como checkpoint.
func saveCheckpoints(p *Progress, batchStart, batchEnd int) {
	if p.Checkpoints == nil {
		p.Checkpoints = make(map[string]string)
	}
	seen := make(map[string]bool)
	for i := batchStart; i < batchEnd; i++ {
		f := p.Steps[i].File
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		if content, err := os.ReadFile(f); err == nil {
			p.Checkpoints[f] = string(content)
		}
	}
}

// restoreCheckpoints restaura cada archivo del batch a su checkpoint (o trunca si no hay).
func restoreCheckpoints(p *Progress, batchStart, batchEnd int) (string, error) {
	files := batchFiles(p.Steps, batchStart, batchEnd)
	var modes []string
	for _, f := range files {
		if cp, ok := p.Checkpoints[f]; ok {
			if err := os.WriteFile(f, []byte(cp), 0644); err != nil {
				return "", fmt.Errorf("no se pudo restaurar checkpoint %s: %w", f, err)
			}
			modes = append(modes, fmt.Sprintf("%s: restaurado", filepath.Base(f)))
		} else {
			if err := truncateFile(f); err != nil {
				return "", fmt.Errorf("no se pudo truncar %s: %w", f, err)
			}
			modes = append(modes, fmt.Sprintf("%s: vaciado", filepath.Base(f)))
		}
	}
	return strings.Join(modes, ", "), nil
}

// batchFiles retorna archivos únicos del batch (preserva orden de aparición).
func batchFiles(steps []Step, batchStart, batchEnd int) []string {
	seen := make(map[string]bool)
	var files []string
	for i := batchStart; i < batchEnd; i++ {
		f := steps[i].File
		if f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	return files
}

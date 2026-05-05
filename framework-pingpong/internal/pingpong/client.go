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
}

// Result representa el resultado de una operacion.
type Result struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
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
			return &Result{
				Success: true,
				Message: fmt.Sprintf("✓ Mini-test %d pasado. Siguiente batch (Mini-test %d): %v",
					p.BatchIndex-1, p.BatchIndex, pending),
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

	report, verr := VerifyFile(filePath, current)
	if verr != nil {
		return nil, verr
	}

	if !report.Passed {
		childCtx.Decision("paso-no-cumplido", fmt.Sprintf("step %d: %s", current.ID, report.Missing))
		return &Result{
			Success: false,
			Message: fmt.Sprintf("✗ Paso %d incompleto: %s", current.ID, report.Missing),
			Data:    report,
		}, nil
	}

	// Marcar paso done y avanzar currentStep.
	for i := range p.Steps {
		if p.Steps[i].ID == current.ID {
			p.Steps[i].Done = true
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


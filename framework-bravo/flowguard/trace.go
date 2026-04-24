package flowguard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TraceResult es el JSON final que se genera.
type TraceResult struct {
	TraceID        string     `json:"trace_id"`
	Version        string     `json:"version"`
	Generated      string     `json:"generated"`
	Status         string     `json:"status,omitempty"`
	SnapshotReason string     `json:"snapshot_reason,omitempty"`
	TotalDuration  int64      `json:"total_duration_ms"`
	TotalSpans     int        `json:"total_spans"`
	TotalErrors    int        `json:"total_errors"`
	Bottlenecks    []string   `json:"bottlenecks,omitempty"`
	IdealFlow      *IdealFlow `json:"ideal_flow,omitempty"`
	Root           *Span      `json:"root"`
}

// Trace es el contenedor principal.
type Trace struct {
	id        string
	startTime time.Time
	root      *Span
	ctx       *Context
	threshold int64
	ideal     *IdealFlow
	mu        sync.Mutex
	filePath  string
	signalCh  chan os.Signal
	closed    bool
}

// NewTrace crea un nuevo trace y carga automáticamente el IdealFlow.
func NewTrace(appName string) *Trace {
	now := time.Now()
	id := fmt.Sprintf("gf_%d", now.UnixNano())

	rootSpan := &Span{
		Name:    appName,
		StartNs: now.UnixNano(),
		Vars:    make(map[string]any),
	}

	ideal, _ := LoadIdealFlow()

	fmt.Printf("\n========================================\n")
	fmt.Printf(" FlowGuard v5.1 — Golden Flow\n")
	fmt.Printf(" Trace: %s\n", id)
	fmt.Printf(" App:   %s\n", appName)
	if ideal != nil {
		fmt.Printf(" Ideal: %s (%d reglas)\n", ideal.Description, len(ideal.Rules))
	} else {
		fmt.Printf(" Ideal: NO encontrado (ideal_flow.json)\n")
	}
	fmt.Printf("========================================\n\n")

	return &Trace{
		id:        id,
		startTime: now,
		root:      rootSpan,
		ctx: &Context{
			name:  appName,
			span:  rootSpan,
			start: now,
			trace: nil, // se asigna después
		},
		threshold: 50,
		ideal:     ideal,
	}
}

func (t *Trace) GetIdealFlow() *IdealFlow {
	return t.ideal
}

// ReloadIdealFlow vuelve a cargar el IdealFlow desde temp/
// Útil si el IdealFlow fue creado después de NewTrace
func (t *Trace) ReloadIdealFlow() {
	t.ideal, _ = LoadIdealFlow()
	if t.ideal != nil {
		fmt.Printf("[FLOWGUARD] IdealFlow recargado (%d reglas, %d vars críticas)\n",
			len(t.ideal.Rules), len(t.ideal.CriticalVars))
	}
}

func (t *Trace) SetBottleneckThreshold(ms int64) {
	t.threshold = ms
}

func (t *Trace) Start() *Context {
	file, line := getCallerInfo(2)
	t.mu.Lock()
	t.root.File = file
	t.root.Line = line
	t.ctx.trace = t
	t.ensureRuntimeLocked()
	_ = t.persistLocked("running", "trace started", false)
	t.mu.Unlock()

	fmt.Printf("[FLOW] START  %s (%s:%d)\n", t.ctx.name, file, line)
	return t.ctx
}

// Flush guarda el trace como JSON y llama al análisis.
func (t *Trace) Flush() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	err := t.persistLocked("completed", "trace completed", true)
	t.closed = true
	signalCh := t.signalCh
	t.signalCh = nil
	t.mu.Unlock()

	if signalCh != nil {
		signal.Stop(signalCh)
	}
	if err != nil {
		return
	}

	// Análisis final
	t.Analyze()
}

func (t *Trace) recordMutation(reason string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	_ = t.persistLocked("running", reason, false)
}

func (t *Trace) ensureRuntimeLocked() {
	if t.filePath == "" {
		baseDir, _ := os.Getwd()
		if t.ideal != nil {
			if dir := findIdealFlowDir(); dir != "" {
				baseDir = dir
			}
		}

		tempDir := filepath.Join(baseDir, "temp")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			fmt.Printf("[FLOWGUARD] Error al crear carpeta temp: %v\n", err)
		} else {
			t.filePath = filepath.Join(tempDir, fmt.Sprintf("trace_%s.json", t.id))
		}
	}

	if t.signalCh == nil {
		t.signalCh = make(chan os.Signal, 1)
		signal.Notify(t.signalCh, os.Interrupt, syscall.SIGTERM)
		go t.handleSignals(t.signalCh)
	}
}

func (t *Trace) persistLocked(status, reason string, announce bool) error {
	t.ensureRuntimeLocked()
	if t.filePath == "" {
		return fmt.Errorf("trace file path not initialized")
	}

	duration := time.Since(t.startTime).Milliseconds()
	t.root.DurationMs = duration

	totalSpans, totalErrors := countSpansAndErrors(t.root)
	result := TraceResult{
		TraceID:        t.id,
		Version:        "5.2",
		Generated:      time.Now().Format(time.RFC3339),
		Status:         status,
		SnapshotReason: reason,
		TotalDuration:  duration,
		TotalSpans:     totalSpans,
		TotalErrors:    totalErrors,
		Bottlenecks:    findBottlenecks(t.root, t.threshold),
		IdealFlow:      t.ideal,
		Root:           t.root,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("[FLOWGUARD] Error al generar JSON: %v\n", err)
		return err
	}

	if err := os.WriteFile(t.filePath, data, 0644); err != nil {
		fmt.Printf("[FLOWGUARD] Error al guardar trace: %v\n", err)
		return err
	}

	if announce {
		fmt.Printf("\n[FLOWGUARD] Trace %s → %s\n", status, t.filePath)
	}

	return nil
}

func (t *Trace) handleSignals(signalCh chan os.Signal) {
	sig, ok := <-signalCh
	if !ok {
		return
	}

	reason := fmt.Sprintf("signal received: %s", sig.String())

	t.mu.Lock()
	if !t.closed {
		_ = t.persistLocked("interrupted", reason, true)
		t.closed = true
	}
	t.mu.Unlock()

	signal.Stop(signalCh)

	if sigValue, ok := sig.(syscall.Signal); ok {
		signal.Reset(sig)
		_ = syscall.Kill(os.Getpid(), sigValue)
		time.Sleep(200 * time.Millisecond)
	}

	os.Exit(1)
}

// findIdealFlowDir busca el directorio que contiene temp/ideal_flow.json
func findIdealFlowDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Verificar cwd
	if _, err := os.Stat(filepath.Join(cwd, "temp", "ideal_flow.json")); err == nil {
		return cwd
	}

	// Buscar en padres hasta 3 niveles
	for i := 0; i < 3; i++ {
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		if _, err := os.Stat(filepath.Join(parent, "temp", "ideal_flow.json")); err == nil {
			return parent
		}
		cwd = parent
	}

	return ""
}

func countSpansAndErrors(span *Span) (spans, errors int) {
	if span == nil {
		return
	}
	spans = 1
	errors = len(span.Errors)
	for _, child := range span.Children {
		s, e := countSpansAndErrors(child)
		spans += s
		errors += e
	}
	return
}

func findBottlenecks(span *Span, threshold int64) []string {
	var bottlenecks []string
	if span.DurationMs > threshold {
		bottlenecks = append(bottlenecks, fmt.Sprintf("%s (%dms)", span.Name, span.DurationMs))
	}
	for _, child := range span.Children {
		bottlenecks = append(bottlenecks, findBottlenecks(child, threshold)...)
	}
	return bottlenecks
}

// Analyze compara el trace real contra el IdealFlow
func (t *Trace) Analyze() {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf(" FLOWGUARD CONTEXT READY — IDEAL + TRACE\n")
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	if t.ideal == nil {
		fmt.Println("⚠️  No se encontró ideal_flow.json")
		fmt.Println("   El trace se guardó, pero falta el contrato ideal para que una IA compare ideal vs real.")
		fmt.Println("   Sugerencia: crea el ideal_flow.json antes de ejecutar el ejemplo.")
		return
	}

	fmt.Printf("Descripción: %s\n", t.ideal.Description)
	fmt.Printf("Verbalización cargada: %v\n", t.ideal.Verbalization != "")
	fmt.Printf("Reglas definidas: %d\n", len(t.ideal.Rules))
	fmt.Printf("Variables críticas: %d\n\n", len(t.ideal.CriticalVars))

	// Resumen de artefactos disponibles para la IA
	if len(t.ideal.CriticalVars) > 0 {
		fmt.Println("Variables críticas definidas:")
		for _, v := range t.ideal.CriticalVars {
			fmt.Printf("  • %s\n", v)
		}
	}

	fmt.Println("\nFlowGuard no compara estas reglas por sí solo.")
	fmt.Println("Su trabajo termina al empaquetar el ideal y el trace real con suficiente contexto.")
	fmt.Println("Usa prompts/VERIFICATION_PROMPT.md con una IA agentica para hacer la comparación.")
}

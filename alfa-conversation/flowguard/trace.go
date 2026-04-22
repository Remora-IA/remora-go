package flowguard

import (
	"fmt"
	"time"
)

// TraceResult es el JSON final que se genera.
// Este es el archivo que la IA lee para diagnosticar.
type TraceResult struct {
	TraceID       string   `json:"trace_id"`
	Version       string   `json:"version"`
	Generated     string   `json:"generated"`
	TotalDuration int64    `json:"total_duration_ms"`
	TotalSpans    int      `json:"total_spans"`
	TotalErrors   int      `json:"total_errors"`
	Bottlenecks   []string `json:"bottlenecks,omitempty"`
	Root          *Span    `json:"root"`
}

// Trace es el contenedor principal. Se crea una vez en main().
type Trace struct {
	id        string
	startTime time.Time
	root      *Span
	ctx       *Context
	threshold int64 // ms para considerar bottleneck
}

// NewTrace crea un nuevo trace. Llamar UNA sola vez en main().
//
// Uso:
//
//	trace := flowguard.NewTrace("MiAplicacion")
//	defer trace.Flush()
func NewTrace(appName string) *Trace {
	now := time.Now()
	id := fmt.Sprintf("gf_%d", now.UnixNano())

	rootSpan := &Span{
		Name:    appName,
		StartNs: now.UnixNano(),
		Vars:    make(map[string]any),
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf(" FlowGuard v5.0 — Golden Flow\n")
	fmt.Printf(" Trace: %s\n", id)
	fmt.Printf(" App:   %s\n", appName)
	fmt.Printf("========================================\n\n")

	return &Trace{
		id:        id,
		startTime: now,
		root:      rootSpan,
		ctx: &Context{
			name:  appName,
			span:  rootSpan,
			start: now,
		},
		threshold: 50, // default: 50ms = bottleneck
	}
}

// SetBottleneckThreshold cambia el umbral en ms para detectar cuellos de botella.
// Por defecto es 50ms.
//
// Uso:
//
//	trace.SetBottleneckThreshold(100) // solo marca si tarda más de 100ms
func (t *Trace) SetBottleneckThreshold(ms int64) {
	t.threshold = ms
}

// Start crea el contexto raíz. Es lo primero después de NewTrace.
//
// Uso:
//
//	ctx := trace.Start()
//	defer ctx.End()
func (t *Trace) Start() *Context {
	file, line := getCallerInfo(2)
	t.root.File = file
	t.root.Line = line

	fmt.Printf("[FLOW] START  %s (%s:%d)\n", t.ctx.name, file, line)
	return t.ctx
}

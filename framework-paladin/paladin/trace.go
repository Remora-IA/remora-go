package paladin

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

type TraceResult struct {
	TraceID        string   `json:"trace_id"`
	Version        string   `json:"version"`
	Generated      string   `json:"generated"`
	Status         string   `json:"status,omitempty"`
	SnapshotReason string   `json:"snapshot_reason,omitempty"`
	TotalDuration  int64    `json:"total_duration_ms"`
	TotalSpans     int      `json:"total_spans"`
	TotalErrors    int      `json:"total_errors"`
	Bottlenecks    []string `json:"bottlenecks,omitempty"`
	Root           *Span    `json:"root"`
}

type Trace struct {
	id        string
	startTime time.Time
	root      *Span
	ctx       *Context
	threshold int64
	mu        sync.Mutex
	filePath  string
	signalCh  chan os.Signal
	closed    bool
}

func NewTraceWithServer(appName, serverURL string) *Trace {
	trace := NewTrace(appName)
	if serverURL != "" {
		SetGlobalClient(NewTraceClient(serverURL))
	}
	// También intentar leer PALADIN_SERVER_URL del entorno
	if os.Getenv("PALADIN_SERVER_URL") != "" && globalTraceClient == nil {
		SetGlobalClient(NewTraceClient(os.Getenv("PALADIN_SERVER_URL")))
	}
	return trace
}

func NewTrace(appName string) *Trace {
	now := time.Now()
	id := fmt.Sprintf("pal_%d", now.UnixNano())
	rootSpan := &Span{
		Name:    appName,
		StartNs: now.UnixNano(),
		Vars:    make(map[string]any),
	}

	if !traceLoggingDisabled() {
		fmt.Printf("\n========================================\n")
		fmt.Printf(" Paladin Trace\n")
		fmt.Printf(" Trace: %s\n", id)
		fmt.Printf(" App:   %s\n", appName)
		fmt.Printf("========================================\n\n")
	}

	return &Trace{
		id:        id,
		startTime: now,
		root:      rootSpan,
		ctx: &Context{
			name:  appName,
			span:  rootSpan,
			start: now,
			trace: nil,
		},
		threshold: 50,
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

	if !traceLoggingDisabled() {
		fmt.Printf("[PALADIN] START %s (%s:%d)\n", t.ctx.name, file, line)
	}
	return t.ctx
}

// Flush cierra el trace, guarda el archivo y opcionalmente envía al servidor.
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

	// Enviar al servidor si está configurado (no blocking)
	if globalTraceClient != nil {
		go func() {
			traceJSON, _ := json.Marshal(TraceResult{
				TraceID:   t.id,
				Version:   "1.0",
				Generated: time.Now().Format(time.RFC3339),
				Status:    "completed",
				Root:      t.root,
			})
			_ = globalTraceClient.SendTrace(t.root.Name, traceJSON)
		}()
	}
}

func (t *Trace) ensureRuntimeLocked() {
	if t.filePath == "" {
		baseDir, _ := os.Getwd()
		tempDir := filepath.Join(baseDir, "temp", "paladin")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			if !traceLoggingDisabled() {
				fmt.Printf("[PALADIN] Error creando temp: %v\n", err)
			}
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
	duration := time.Since(t.startTime).Milliseconds()
	t.root.DurationMs = duration
	totalSpans, totalErrors := countSpansAndErrors(t.root)
	result := TraceResult{
		TraceID:        t.id,
		Version:        "1.0",
		Generated:      time.Now().Format(time.RFC3339),
		Status:         status,
		SnapshotReason: reason,
		TotalDuration:  duration,
		TotalSpans:     totalSpans,
		TotalErrors:    totalErrors,
		Bottlenecks:    findBottlenecks(t.root, t.threshold),
		Root:           t.root,
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(t.filePath, data, 0644); err != nil {
		return err
	}
	if announce && !traceLoggingDisabled() {
		fmt.Printf("\n[PALADIN] Trace %s -> %s\n", status, t.filePath)
	}
	return nil
}

func traceLoggingDisabled() bool {
	return strings.TrimSpace(os.Getenv("PALADIN_SILENT")) != ""
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
	if span != nil && span.DurationMs > threshold {
		bottlenecks = append(bottlenecks, fmt.Sprintf("%s (%dms)", span.Name, span.DurationMs))
	}
	if span != nil {
		for _, child := range span.Children {
			bottlenecks = append(bottlenecks, findBottlenecks(child, threshold)...)
		}
	}
	return bottlenecks
}

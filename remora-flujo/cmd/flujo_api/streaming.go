package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SSE streaming para postMessage.
//
// Idea: cuando el cliente CLI manda `Accept: text/event-stream`, en lugar de
// devolver JSON con la respuesta final, emitimos eventos en vivo a medida que
// el framework los va escribiendo al archivo `temp/live_<conv_id>.jsonl`.
//
// Eventos emitidos al CLI:
//   - tool_start  {tool, args}
//   - tool_end    {tool, args, ok, status, duration_ms}
//   - llm_start   {iter}
//   - llm_error   {error}
//   - assistant   {text}     ← texto final del framework, se renderiza como bubble
//   - done        {idle}     ← fin de stream
//
// Flujo:
// 1. limpia archivos live previos
// 2. arranca tailers (uno por framework de la conv)
// 3. arranca goroutine que ejecuta runLoop normal
// 4. consume eventos de tailers + esperar a que runLoop termine
// 5. emite "done" y cierra

func wantsSSE(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/event-stream")
}

// sseWriter envuelve http.ResponseWriter para emisión de eventos SSE.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &sseWriter{w: w, flusher: flusher}, nil
}

// emit envía un evento SSE con tipo + data JSON.
func (s *sseWriter) emit(event string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.flusher.Flush()
}

// liveFilePath devuelve el path donde un framework escribe sus eventos JSONL
// para una conversación. Convención: framework-<name>/temp/live_<conv_id>.jsonl
// dentro del REMORA_ROOT.
func liveFilePath(remoraRoot, framework, convID string) string {
	convClean := sanitizeConvForFile(convID)
	return filepath.Join(remoraRoot, "framework-"+framework, "temp", "live_"+convClean+".jsonl")
}

// sanitizeConvForFile limpia el conv_id como hace arquitecto en su lado.
func sanitizeConvForFile(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// tailLiveFile lee eventos JSONL de path mientras done no se cierre.
// Cada línea válida se manda al canal events. Reabre el archivo si fue
// recreado (los frameworks llaman os.Create al iniciar el turno).
func tailLiveFile(ctx context.Context, path string, events chan<- map[string]any) {
	// Borrar pre-existente: el framework lo creará al ejecutar.
	_ = os.Remove(path)

	deadline := time.Now().Add(2 * time.Second)
	var f *os.File
	for time.Now().Before(deadline) {
		if x, err := os.Open(path); err == nil {
			f = x
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
	if f == nil {
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			drainRemaining(reader, events)
			return
		default:
		}
		line, err := reader.ReadString('\n')
		if len(strings.TrimSpace(line)) > 0 {
			var evt map[string]any
			if jerr := json.Unmarshal([]byte(line), &evt); jerr == nil {
				select {
				case events <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
		if err != nil {
			// EOF: esperar más datos.
			select {
			case <-ctx.Done():
				drainRemaining(reader, events)
				return
			case <-time.After(80 * time.Millisecond):
			}
		}
	}
}

// drainRemaining lee lo que quede en el reader sin bloquear.
func drainRemaining(reader *bufio.Reader, events chan<- map[string]any) {
	for {
		line, err := reader.ReadString('\n')
		if len(strings.TrimSpace(line)) > 0 {
			var evt map[string]any
			if jerr := json.Unmarshal([]byte(line), &evt); jerr == nil {
				select {
				case events <- evt:
				default:
					return
				}
			}
		}
		if err != nil {
			return
		}
	}
}

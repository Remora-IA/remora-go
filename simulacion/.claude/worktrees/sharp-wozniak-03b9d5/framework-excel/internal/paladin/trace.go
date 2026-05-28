package paladin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Trace representa un trace de ejecucion.
type Trace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Root      *Span     `json:"root"`
	Current   *Span     `json:"-"`
}

// NewTrace crea un nuevo trace.
func NewTrace(name string) *Trace {
	return &Trace{
		ID:        fmt.Sprintf("pal_%d", time.Now().UnixNano()),
		Name:      name,
		StartTime: time.Now(),
	}
}

// Start inicia el trace con un contexto raiz.
func (t *Trace) Start() *Context {
	ctx := &Context{
		Trace: t,
		Span:  t.newSpan("root"),
	}
	t.Root = ctx.Span
	t.Current = ctx.Span
	return ctx
}

// newSpan crea un nuevo span.
func (t *Trace) newSpan(name string) *Span {
	return &Span{
		ID:        fmt.Sprintf("span_%d", time.Now().UnixNano()),
		Name:      name,
		StartTime: time.Now(),
	}
}

// Flush guarda el trace en un archivo JSON.
func (t *Trace) Flush() {
	t.EndTime = time.Now()
	
	// Crear directorio temp/paladin
	dir := filepath.Join("temp", "paladin")
	os.MkdirAll(dir, 0755)
	
	// Escribir archivo de trace
	filename := filepath.Join(dir, fmt.Sprintf("trace_%s.json", t.ID))
	data, _ := json.MarshalIndent(t, "", "  ")
	os.WriteFile(filename, data, 0644)
	
	fmt.Printf("[PALADIN] Trace completed -> %s\n", filename)
}

// GetJSON devuelve el trace como JSON.
func (t *Trace) GetJSON() []byte {
	data, _ := json.MarshalIndent(t, "", "  ")
	return data
}
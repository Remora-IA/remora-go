package flowguard

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Context representa el contexto de ejecución de una función.
type Context struct {
	name  string
	span  *Span
	start time.Time
	mu    sync.Mutex
	trace *Trace // referencia al Trace padre
}

// Child crea un nuevo contexto hijo.
func (c *Context) Child(name string) *Context {
	file, line := getCallerInfo(2)

	child := &Span{
		Name:    name,
		File:    file,
		Line:    line,
		StartNs: time.Now().UnixNano(),
		Vars:    make(map[string]any),
	}

	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.Children = append(c.span.Children, child)
		_ = c.trace.persistLocked("running", fmt.Sprintf("span started: %s", name), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.Children = append(c.span.Children, child)
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW] START  %s (%s:%d)\n", name, file, line)

	return &Context{
		name:  name,
		span:  child,
		start: time.Now(),
		trace: c.trace, // heredar referencia al trace
	}
}

// Var registra una variable con su nombre y valor actual.
func (c *Context) Var(key string, value any) {
	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.Vars[key] = value
		_ = c.trace.persistLocked("running", fmt.Sprintf("var updated: %s.%s", c.name, key), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.Vars[key] = value
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW]   VAR  %s → %s = %s\n", c.name, key, formatConsoleValue(value))
}

// Error registra un error ocurrido dentro de la función.
func (c *Context) Error(err error) {
	if err == nil {
		return
	}

	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.Errors = append(c.span.Errors, err.Error())
		_ = c.trace.persistLocked("running", fmt.Sprintf("error recorded: %s", c.name), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.Errors = append(c.span.Errors, err.Error())
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW]   ERR  %s → %s\n", c.name, formatConsoleValue(err.Error()))
}

// ErrorMsg registra un mensaje de error como string directamente.
func (c *Context) ErrorMsg(msg string) {
	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.Errors = append(c.span.Errors, msg)
		_ = c.trace.persistLocked("running", fmt.Sprintf("error recorded: %s", c.name), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.Errors = append(c.span.Errors, msg)
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW]   ERR  %s → %s\n", c.name, formatConsoleValue(msg))
}

// Decision registra una decisión lógica: qué se decidió y por qué.
func (c *Context) Decision(what, why string) {
	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.Decisions = append(c.span.Decisions, Decision{What: what, Why: why})
		_ = c.trace.persistLocked("running", fmt.Sprintf("decision recorded: %s", c.name), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.Decisions = append(c.span.Decisions, Decision{What: what, Why: why})
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW]   DEC  %s → [%s] %s\n", c.name, formatConsoleValue(what), formatConsoleValue(why))
}

// End cierra el contexto y calcula la duración.
func (c *Context) End() {
	duration := time.Since(c.start).Milliseconds()

	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.DurationMs = duration
		_ = c.trace.persistLocked("running", fmt.Sprintf("span ended: %s", c.name), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.DurationMs = duration
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW] END    %s (%d ms)\n", c.name, duration)
}

// GetTrace devuelve el Trace padre de este contexto.
func (c *Context) GetTrace() *Trace {
	return c.trace
}

// HasIdealFlow indica si existe un IdealFlow cargado para este trace
func (c *Context) HasIdealFlow() bool {
	return c.trace != nil && c.trace.GetIdealFlow() != nil
}

// getCallerInfo obtiene archivo y línea de quien llamó
func getCallerInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", 0
	}
	// Acortar el path para que sea legible
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	return short, line
}

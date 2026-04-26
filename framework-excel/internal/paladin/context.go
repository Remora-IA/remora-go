package paladin

import (
	"fmt"
	"time"
)

// Context representa el contexto actual del trace.
type Context struct {
	Trace *Trace
	Span  *Span
}

// Child crea un nuevo contexto hijo.
func (c *Context) Child(name string) *Context {
	child := c.Trace.newSpan(name)
	c.Span.AddChild(child)
	return &Context{
		Trace: c.Trace,
		Span:  child,
	}
}

// End termina el span actual.
func (c *Context) End() {
	c.Span.EndTime = time.Now()
}

// Var registra una variable.
func (c *Context) Var(key string, value interface{}) {
	c.Span.Vars[key] = fmt.Sprintf("%v", value)
}

// Decision registra una decision.
func (c *Context) Decision(id string, description string) {
	c.Span.Decisions = append(c.Span.Decisions, Decision{
		ID:          id,
		Description: description,
		Timestamp:   time.Now(),
	})
}

// Error registra un error.
func (c *Context) Error(message string) {
	c.Span.Error = message
}

// Log registra un mensaje de log.
func (c *Context) Log(message string) {
	c.Span.Logs = append(c.Span.Logs, LogEntry{
		Message:   message,
		Timestamp: time.Now(),
	})
}
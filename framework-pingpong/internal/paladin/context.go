package paladin

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

// Actor declares who is acting in business terms.
func (c *Context) Actor(name, responsibility string) {
	c.semantic("actor", name, responsibility, "", "", nil, nil)
}

// Goal declares what this span is trying to accomplish in business terms.
func (c *Context) Goal(goal string) {
	c.semantic("goal", "", goal, "", "", nil, nil)
}

// Event records a relevant business event. Use this instead of Var when the
// information changes the meaning of the flow.
func (c *Context) Event(subject, summary string, meta map[string]any) {
	c.semantic("event", subject, summary, "", "", nil, meta)
}

// Rule declares a business rule that the code believes it is applying.
func (c *Context) Rule(name, summary string, meta map[string]any) {
	c.semantic("rule", name, summary, "", "", nil, meta)
}

// Check records a rule evaluation with expected and actual business state.
func (c *Context) Check(rule, expected, actual string, passed bool) {
	c.semantic("check", rule, "rule evaluated", expected, actual, &passed, nil)
}

// Expect records the next business state expected by this code path.
func (c *Context) Expect(subject, expected string) {
	c.semantic("expect", subject, "expectation declared", expected, "", nil, nil)
}

// Handoff records a business handoff between actors.
func (c *Context) Handoff(from, to, reason string) {
	c.semantic("handoff", from+"->"+to, reason, "next_actor="+to, "from_actor="+from, nil, nil)
}

// Violation records a known mismatch between intended and observed flow.
func (c *Context) Violation(subject, expected, actual string) {
	passed := false
	c.semantic("violation", subject, "business flow mismatch", expected, actual, &passed, nil)
}

func (c *Context) semantic(kind, subject, summary, expected, actual string, passed *bool, meta map[string]any) {
	event := SemanticEvent{
		Kind:     kind,
		Subject:  subject,
		Summary:  summary,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Meta:     meta,
	}
	if c.trace != nil {
		c.trace.mu.Lock()
		c.span.Semantic = append(c.span.Semantic, event)
		_ = c.trace.persistLocked("running", fmt.Sprintf("semantic recorded: %s.%s", c.name, kind), false)
		c.trace.mu.Unlock()
	} else {
		c.mu.Lock()
		c.span.Semantic = append(c.span.Semantic, event)
		c.mu.Unlock()
	}

	fmt.Printf("[FLOW]   SEM  %s → %s %s %s\n", c.name, kind, formatConsoleValue(subject), formatConsoleValue(summary))
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

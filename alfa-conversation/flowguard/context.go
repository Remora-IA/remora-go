package flowguard

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Context representa el contexto de ejecución de una función.
// Se pasa EXPLÍCITAMENTE de padre a hijo. No hay variable global "current".
type Context struct {
	name  string
	span  *Span
	start time.Time
	mu    sync.Mutex
}

// Child crea un nuevo contexto hijo.
// Esto es lo que genera la jerarquía real del árbol.
//
// Uso:
//
//	func miFuncion(parent *flowguard.Context) {
//	    ctx := parent.Child("miFuncion")
//	    defer ctx.End()
//	}
func (c *Context) Child(name string) *Context {
	file, line := getCallerInfo(2)

	child := &Span{
		Name:    name,
		File:    file,
		Line:    line,
		StartNs: time.Now().UnixNano(),
		Vars:    make(map[string]any),
	}

	c.mu.Lock()
	c.span.Children = append(c.span.Children, child)
	c.mu.Unlock()

	fmt.Printf("[FLOW] START  %s (%s:%d)\n", name, file, line)

	return &Context{
		name:  name,
		span:  child,
		start: time.Now(),
	}
}

// Var registra una variable con su nombre y valor actual.
// La IA usa esto para ver el estado exacto en cada punto de ejecución.
//
// Uso:
//
//	ctx.Var("vidas", 3)
//	ctx.Var("jugador", jugador)
func (c *Context) Var(key string, value any) {
	c.mu.Lock()
	c.span.Vars[key] = value
	c.mu.Unlock()

	fmt.Printf("[FLOW]   VAR  %s → %s = %v\n", c.name, key, value)
}

// Error registra un error ocurrido dentro de la función.
// La IA prioriza funciones con errores para encontrar el bug.
//
// Uso:
//
//	if err != nil {
//	    ctx.Error(err)
//	}
func (c *Context) Error(err error) {
	if err == nil {
		return
	}

	c.mu.Lock()
	c.span.Errors = append(c.span.Errors, err.Error())
	c.mu.Unlock()

	fmt.Printf("[FLOW]   ERR  %s → %s\n", c.name, err.Error())
}

// ErrorMsg registra un mensaje de error como string directamente.
//
// Uso:
//
//	ctx.ErrorMsg("el valor no puede ser negativo")
func (c *Context) ErrorMsg(msg string) {
	c.mu.Lock()
	c.span.Errors = append(c.span.Errors, msg)
	c.mu.Unlock()

	fmt.Printf("[FLOW]   ERR  %s → %s\n", c.name, msg)
}

// Decision registra una decisión lógica: qué se decidió y por qué.
// Esto elimina la ambigüedad para la IA sobre por qué se tomó un camino.
//
// Uso:
//
//	ctx.Decision("usar cache", "datos tienen menos de 5 minutos")
//	ctx.Decision("reintentar", "primer intento falló con timeout")
func (c *Context) Decision(what, why string) {
	c.mu.Lock()
	c.span.Decisions = append(c.span.Decisions, Decision{What: what, Why: why})
	c.mu.Unlock()

	fmt.Printf("[FLOW]   DEC  %s → [%s] %s\n", c.name, what, why)
}

// End cierra el contexto y calcula la duración.
// SIEMPRE usar con defer:
//
//	ctx := parent.Child("miFuncion")
//	defer ctx.End()
func (c *Context) End() {
	duration := time.Since(c.start).Milliseconds()

	c.mu.Lock()
	c.span.DurationMs = duration
	c.mu.Unlock()

	fmt.Printf("[FLOW] END    %s (%d ms)\n", c.name, duration)
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

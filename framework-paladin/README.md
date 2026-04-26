# Framework Paladin

Paladin es el framework de **tracing reusable para Remora**.

## Responsabilidad

- spans jerárquicos
- variables de estado
- decisiones lógicas (qué + por qué)
- errores con contexto
- snapshots JSON en `temp/paladin`
- persistencia incremental mientras el proceso corre

**No compara flujo ideal vs flujo real.** Esa responsabilidad queda en Framework Bravo.

## Estructura

```
paladin/
├── trace.go      # Trace manager, persistencia, shutdown
├── span.go       # Estructura de spans y decisiones
├── context.go    # API para crear spans, vars, decisions, errors
└── console.go    # Formateo para consola
```

## Uso

```go
trace := paladin.NewTrace("app-name")
ctx := trace.Start()
defer trace.Flush()

// En cada función:
func miFuncion(ctx *paladin.Context) {
    child := ctx.Child("miFuncion")
    defer child.End()
    
    child.Var("key", value)
    child.Decision("qué", "por qué")
    child.Error(err)
}
```

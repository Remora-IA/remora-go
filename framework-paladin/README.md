# Framework Paladin

Paladin es el framework de **tracing reusable para Remora**.

## Why

El why completo vive en [`WHY.md`](WHY.md). Resumen: Paladin existe para entender
el flujo real que siguió un programa y contrastarlo con las reglas de negocio
que el código cree estar aplicando. La fuente de verdad son eventos semánticos y
reglas emitidas por Go; la IA solo traduce, resume y compara.

Para sesiones agenticas, el contrato operativo vive en
[`INITIAL_PROMPT.md`](INITIAL_PROMPT.md).

## Responsabilidad

- spans jerárquicos
- variables de estado
- decisiones lógicas (qué + por qué)
- eventos semánticos de negocio
- reglas, checks, expectativas y handoffs
- errores con contexto
- snapshots JSON en `temp/paladin`
- persistencia incremental mientras el proceso corre

Paladin no reemplaza a los frameworks de negocio. Les da un protocolo para
declarar qué creen estar haciendo mientras ejecutan.

## Estructura

```
paladin/
├── trace.go      # Trace manager, persistencia, shutdown
├── span.go       # Estructura de spans y decisiones
├── context.go    # API para crear spans, vars, decisions, errors
├── explain.go    # Traducción determinística de trace semántico
└── console.go    # Formateo para consola
```

## Uso Correcto

Usa `Var` para estado técnico útil. Usa la API semántica para lógica de negocio.
No intentes tracear cada variable: registra los puntos donde el programa cambia
de intención, aplica una regla, decide una ruta o transfiere control.

```go
trace := paladin.NewTrace("app-name")
ctx := trace.Start()
defer trace.Flush()

// En cada función:
func miFuncion(ctx *paladin.Context) {
    child := ctx.Child("miFuncion")
    defer child.End()

    child.Actor("echo", "descubre el proceso real del usuario")
    child.Goal("decidir si hay suficiente contexto para activar Alfa")

    child.Var("key", value)
    child.Rule("echo_to_alfa", "Alfa se activa despues de 2 respuestas reales del usuario", nil)
    child.Check("echo_to_alfa", "user_answers >= 2", "user_answers = 1", false)
    child.Decision("mantener_echo", "todavia falta una respuesta real")
    child.Expect("next_actor", "echo")
    child.Error(err)
}
```

## API Semántica

- `Actor(name, responsibility)`: quién actúa en términos de negocio.
- `Goal(goal)`: qué intenta lograr este span.
- `Event(subject, summary, meta)`: evento de negocio relevante.
- `Rule(name, summary, meta)`: regla que el código cree estar aplicando.
- `Check(rule, expected, actual, passed)`: evaluación concreta de una regla.
- `Decision(what, why)`: decisión tomada.
- `Expect(subject, expected)`: próximo estado esperado.
- `Handoff(from, to, reason)`: transferencia de control entre actores.
- `Violation(subject, expected, actual)`: inconsistencia conocida.

## Explain

Para ver el árbol técnico:

```bash
go run ./cmd/paladin temp/paladin/trace_pal_x.json
```

Para ver el flujo semántico:

```bash
go run ./cmd/paladin explain temp/paladin/trace_pal_x.json
```

La salida separa:

- timeline semántico
- reglas y expectativas
- handoffs
- inconsistencias
- señales técnicas

## Audit

Para evaluar si un repo está implementando Paladin según su why:

```bash
go run ./cmd/paladin audit /path/al/repo
```

El audit inspecciona código Go y reporta cobertura semántica:

- creación de traces y spans;
- uso de `Actor`, `Goal`, `Rule`, `Check`, `Expect`, `Handoff`, `Violation`;
- exceso de `Var` respecto a semántica;
- decisiones sin reglas declaradas;
- hallazgos `fail` y `warn` accionables.

## Regla Práctica

Si una IA o un humano necesita entender una decisión de negocio leyendo diez
variables crudas, falta instrumentación semántica. El código debe declarar:

1. qué regla cree estar aplicando;
2. qué observó;
3. qué decidió;
4. qué espera que pase después.

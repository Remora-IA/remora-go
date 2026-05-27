# framework-paladin

> Sustrato semántico de trazas — el pegamento del enjambre

| Campo | Valor |
|---|---|
| Package | `paladin` |
| Pain weight | 0.95 |
| Agente | `agent-gamma` |
| Archivos | audit.go, client.go, console.go, context.go, explain.go, span.go, trace.go |
| Líneas | 1085 |
| Tags | core, tracing, semantics |

## Funciones exportadas

### `AuditRepo`

### `WriteAudit`

### `NewTraceClient`
NewTraceClient crea un cliente para enviar traces al servidor.

### `(&{%!s(token.Pos=7036) TraceClient}) SendTrace`
SendTrace envía un trace al servidor. No bloquea.

### `(&{%!s(token.Pos=8384) TraceClient}) GetFlow`
GetFlow consulta el flow narrado de un trace.

### `(&{%!s(token.Pos=8939) TraceClient}) Ask`
Ask envía una pregunta sobre un trace específico.

### `SetGlobalClient`
SetGlobalClient configura el cliente global.

### `SendTraceAsync`
SendTraceAsync envía trace usando el cliente global (no blocking).

### `(&{%!s(token.Pos=11050) Context}) Child`
Child crea un nuevo contexto hijo.

### `(&{%!s(token.Pos=11809) Context}) Var`
Var registra una variable con su nombre y valor actual.

### `(&{%!s(token.Pos=12259) Context}) Error`
Error registra un error ocurrido dentro de la función.

### `(&{%!s(token.Pos=12780) Context}) ErrorMsg`
ErrorMsg registra un mensaje de error como string directamente.

### `(&{%!s(token.Pos=13257) Context}) Decision`
Decision registra una decisión lógica: qué se decidió y por qué.

### `(&{%!s(token.Pos=13818) Context}) Actor`
Actor declares who is acting in business terms.

### `(&{%!s(token.Pos=14012) Context}) Goal`
Goal declares what this span is trying to accomplish in business terms.

### `(&{%!s(token.Pos=14226) Context}) Event`
Event records a relevant business event. Use this instead of Var when the

### `(&{%!s(token.Pos=14431) Context}) Rule`
Rule declares a business rule that the code believes it is applying.

### `(&{%!s(token.Pos=14632) Context}) Check`
Check records a rule evaluation with expected and actual business state.

### `(&{%!s(token.Pos=14852) Context}) Expect`
Expect records the next business state expected by this code path.

### `(&{%!s(token.Pos=15041) Context}) Handoff`
Handoff records a business handoff between actors.

### `(&{%!s(token.Pos=15264) Context}) Violation`
Violation records a known mismatch between intended and observed flow.

### `(&{%!s(token.Pos=16203) Context}) End`
End cierra el contexto y calcula la duración.

### `(&{%!s(token.Pos=16649) Context}) GetTrace`
GetTrace devuelve el Trace padre de este contexto.

### `BuildExplanation`

### `WriteExplanation`

### `(ExplanationItem) Sentence`

### `NewTraceWithServer`

### `NewTrace`

### `(&{%!s(token.Pos=24734) Trace}) SetBottleneckThreshold`

### `(&{%!s(token.Pos=24806) Trace}) Start`

### `(&{%!s(token.Pos=25205) Trace}) Flush`
Flush cierra el trace, guarda el archivo y opcionalmente envía al servidor.

## Tipos

- **`AuditResult`** (struct)
- **`AuditFinding`** (struct)
- **`TraceClient`** (struct): TraceClient envía traces al servidor Paladin Server.
- **`FlowResult`** (struct): FlowResult es la respuesta del servidor con el flow narrado.
- **`Context`** (struct): Context representa el contexto de ejecución de una función.
- **`Explanation`** (struct)
- **`ExplanationItem`** (struct)
- **`Span`** (struct): Span representa un segmento de ejecución (función o bloque).
- **`Decision`** (struct)
- **`SemanticEvent`** (struct): SemanticEvent records business meaning, not implementation detail.
- **`TraceResult`** (struct)
- **`Trace`** (struct)

---
_Generado por Remora Doc-Swarm · agente `agent-gamma` · 2026-05-27 06:25:03_

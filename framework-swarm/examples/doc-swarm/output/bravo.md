# framework-bravo

> Verificación de flujo ideal vs. traza real

| Campo | Valor |
|---|---|
| Package | `bravo` |
| Pain weight | 0.80 |
| Agente | `agent-beta` |
| Archivos | idealflow.go, trace.go, verifier.go |
| Líneas | 340 |
| Tags | core, verification |

## Funciones exportadas

### `NewIdealFlow`

### `(&{%!s(token.Pos=1158) IdealFlow}) SetVerbalization`

### `(&{%!s(token.Pos=1258) IdealFlow}) SetIntent`

### `(&{%!s(token.Pos=1348) IdealFlow}) AddRule`

### `(&{%!s(token.Pos=1559) IdealFlow}) AddRuleWithWhen`

### `(&{%!s(token.Pos=1805) IdealFlow}) AddCriticalVar`

### `(&{%!s(token.Pos=1932) IdealFlow}) SetCriticalPath`

### `(&{%!s(token.Pos=2183) IdealFlow}) Save`
Save guarda el JSON y también genera IDEAL_FLOW.md para lectura humana

### `LoadIdealFlow`
LoadIdealFlow carga el ideal_flow.json si existe

### `NewTrace`

### `(&{%!s(token.Pos=6041) Trace}) Start`

### `(&{%!s(token.Pos=6103) Trace}) Flush`

### `(&{%!s(token.Pos=6162) Trace}) SetBottleneckThreshold`

### `(&{%!s(token.Pos=6252) Trace}) GetIdealFlow`

### `(&{%!s(token.Pos=6315) Trace}) ReloadIdealFlow`

### `(&{%!s(token.Pos=6543) Trace}) Analyze`

### `PrintVerificationInstructions`
PrintVerificationInstructions imprime cómo usar el verificador

## Tipos

- **`IdealFlow`** (struct)
- **`Rule`** (struct)
- **`Context`** (type)
- **`Span`** (type)
- **`Decision`** (type)
- **`Trace`** (struct)
- **`VerifierResult`** (struct): VerifierResult es el formato exacto que la IA debe devolver al analizar.
- **`Gap`** (struct)
- **`IdealComparison`** (struct)
- **`Diff`** (struct)

---
_Generado por Remora Doc-Swarm · agente `agent-beta` · 2026-05-27 06:25:03_

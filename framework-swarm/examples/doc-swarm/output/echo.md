# framework-echo (tree)

> Árbol de descubrimiento AXIOM→PAIN→OPPORTUNITY

| Campo | Valor |
|---|---|
| Package | `tree` |
| Pain weight | 0.88 |
| Agente | `agent-alpha` |
| Archivos | node.go, questions.go, readiness.go, tree.go |
| Líneas | 1177 |
| Tags | core, discovery |

## Funciones exportadas

### `MinValidatedPreviousLayer`

### `NewNode`
NewNode crea un nodo con valores por defecto según su tipo

### `(&{%!s(token.Pos=3012) Node}) Validate`
Validate marca un nodo como validado con la respuesta del cliente

### `(&{%!s(token.Pos=3226) Node}) Reject`
Reject marca un nodo como rechazado

### `(&{%!s(token.Pos=3432) Node}) AddChild`
AddChild agrega un hijo al nodo

### `(&{%!s(token.Pos=3664) Node}) AddPerception`
AddPerception agrega una nota interna de percepción al nodo.

### `GenerateQuestions`
GenerateQuestions genera preguntas contextuales basadas en el tipo y título del nodo

### `(&{%!s(token.Pos=7119) FrameworkEcho}) AssessAlfaReadiness`

### `LoadOrCreate`
LoadOrCreate carga el árbol desde un archivo o crea uno nuevo

### `(&{%!s(token.Pos=20146) FrameworkEcho}) Save`
Save guarda el árbol al archivo JSON

### `(&{%!s(token.Pos=20484) FrameworkEcho}) Init`
Init inicializa un proyecto nuevo

### `(&{%!s(token.Pos=21324) FrameworkEcho}) CountValidatedInLayer`
CountValidatedInLayer cuenta cuántos nodos validados hay en una capa

### `(&{%!s(token.Pos=21813) FrameworkEcho}) AddNode`
AddNode agrega un nodo al árbol con todas las validaciones

### `(&{%!s(token.Pos=24144) FrameworkEcho}) ValidateNode`
ValidateNode valida un nodo con la respuesta del cliente

### `(&{%!s(token.Pos=24495) FrameworkEcho}) RejectNode`
RejectNode rechaza un nodo

### `(&{%!s(token.Pos=24778) FrameworkEcho}) UpdateConfidence`
UpdateConfidence actualiza la confianza de un nodo manualmente

### `(&{%!s(token.Pos=25206) FrameworkEcho}) AddPerception`
AddPerception agrega una nota interna de percepción a un nodo.

### `(&{%!s(token.Pos=25543) FrameworkEcho}) SetQALogEnabled`

### `(&{%!s(token.Pos=25658) FrameworkEcho}) AddQALog`

### `(&{%!s(token.Pos=26291) FrameworkEcho}) AddSignal`

### `(&{%!s(token.Pos=26937) FrameworkEcho}) SelectOpportunity`

### `(&{%!s(token.Pos=27575) FrameworkEcho}) SelectedOpportunities`

### `(&{%!s(token.Pos=27993) FrameworkEcho}) GetPendingQuestions`
GetPendingQuestions retorna todas las preguntas pendientes organizadas por nodo

### `(&{%!s(token.Pos=29000) FrameworkEcho}) GetStats`
GetStats retorna estadísticas del árbol

### `(&{%!s(token.Pos=29850) FrameworkEcho}) ShowTree`
ShowTree imprime el árbol de forma visual

## Tipos

- **`Node`** (struct): Node representa un nodo en el árbol de conocimiento
- **`ReadinessReport`** (struct)
- **`ReadinessCheck`** (struct)
- **`FrameworkEcho`** (struct): FrameworkEcho es la estructura raíz del árbol de conocimiento
- **`Config`** (struct)
- **`QALogEntry`** (struct)
- **`SignalEntry`** (struct)
- **`PendingQuestion`** (struct): PendingQuestion representa una pregunta pendiente
- **`TreeStats`** (struct): TreeStats estadísticas del árbol
- **`LayerStats`** (struct): LayerStats estadísticas por capa

---
_Generado por Remora Doc-Swarm · agente `agent-alpha` · 2026-05-27 06:25:03_

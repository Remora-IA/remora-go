# framework-agent

Primitiva de Remora para agentes conversacionales: turn loop, estado, traza Paladin y LLM compartido. El developer escribe la lógica de negocio (`Behavior`); la runtime hace el resto.

## Why

Antes de esta librería, cada agente conversacional repetía a mano:
- Loop de turnos con append a history.
- Llamada al LLM con system prompt + history.
- Spans de Paladin con actor/goal/decision/var.
- Detección de estados terminales (acuerdo, escalación, abandono).
- Estado opaco entre turnos.

Eso es ~100 líneas que no son del agente, son de la infraestructura. Esta librería las absorbe.

## Uso

```go
import (
    "github.com/remora-go/framework-agent/agent"
    "github.com/remora-go/framework-llm/llm"
    "github.com/remora-go/framework-paladin/paladin"
)

// 1. Implementa Behavior (lógica de tu agente)
type MiAgente struct{ /* ... */ }

func (m *MiAgente) Name() string           { return "MiAgente" }
func (m *MiAgente) Responsibility() string { return "Lo que hace mi agente" }
func (m *MiAgente) Goal(s agent.State) string {
    return "El objetivo del turno"
}
func (m *MiAgente) SystemPrompt(s agent.State) string {
    return "Eres un agente que..."
}
func (m *MiAgente) OnInput(s agent.State, input string) agent.Decision {
    // Pre-LLM: detectar condiciones para short-circuit o escalar
    return agent.Decision{}
}
func (m *MiAgente) OnReply(s agent.State, input, reply string) agent.Decision {
    // Post-LLM: actualizar state, detectar terminal
    return agent.Decision{Update: func(s agent.State) { s["turnos"] = /* ... */ }}
}

// 2. Crea el agente y córrelo
trace := paladin.NewTrace("MiApp")
defer trace.Flush()
root := trace.Start()
defer root.End()

a := agent.New(&MiAgente{}, llm.NewClientOrStub(), nil)

reply, _ := a.Turn(root, "hola")
// ... loop hasta a.Done() != nil
```

## API

- `Behavior` — interfaz que implementa la lógica de negocio.
- `Agent` — runtime que maneja history, llm, paladin spans.
- `State` — `map[string]any` mutable entre turnos.
- `Decision` — lo que un hook devuelve: update, short-circuit, outcome, traces.
- `Outcome` — terminal con status + reason + data.

## Primer consumidor

[`examples/kobra-carolina`](../examples/kobra-carolina). Antes del refactor: 177 líneas en `carolina.go` con turn loop, paladin spans, llm wiring y lógica de Carolina mezclados. Después: 150 líneas de Behavior puro, runtime totalmente compartido.

## Lo que NO hace todavía

- Persistencia de estado entre procesos (Carolina vive en memoria; falta `agent.Store` con backend SQLite/Redis para correr N conversaciones reales).
- Concurrencia multi-agente (un agente por instancia hoy).
- Tools / function calling (los agentes solo responden texto).
- Composición de agentes (agente que llama a otro agente).

Esas piezas se agregan cuando un consumidor real las pida.

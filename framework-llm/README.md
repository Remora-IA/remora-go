# framework-llm

Primitiva de Remora para llamar a modelos de lenguaje sin escribir wrappers HTTP.

## Why

Cada vez que un founder arma un agente con Claude, escribe el mismo wrapper HTTP a mano: tipos de Messages API, headers, stub para desarrollo sin gastar tokens. Eso es reinventar la rueda. Esta librería lo absorbe.

## Uso

```go
import "github.com/remora-go/framework-llm/llm"

// Producción si hay ANTHROPIC_API_KEY, stub si no:
client := llm.NewClientOrStub(
    "respuesta determinística 1",
    "respuesta determinística 2",
)

resp, err := client.Complete(ctx, llm.Request{
    System:   "Eres un asistente útil.",
    Messages: []llm.Message{{Role: "user", Content: "Hola"}},
})
// resp.Text contiene la respuesta
```

## API

- `llm.Client` — interfaz que cualquier proveedor implementa.
- `llm.NewAnthropic()` — cliente real contra Claude API.
- `llm.NewStub(responses...)` — respuestas pre-cargadas para test/dev.
- `llm.NewClientOrStub(stubs...)` — el helper de MVP: usa real si hay key, stub si no.

## Primer consumidor

[`examples/cobranza-conversacional`](../examples/cobranza-conversacional) usa esto. Antes Carolina tenía 70 líneas de wrapper HTTP propio; ahora 4 líneas de import.

## Lo que NO hace todavía

- Streaming.
- Tool use / function calling.
- Otros proveedores (OpenAI, Gemini). La interfaz está lista, falta implementarlos.
- Reintentos automáticos con backoff.

Esas piezas se agregan cuando un consumidor real las necesite, no antes.

# AGENTS.md — Cómo usar Remora si sos una IA

> Este archivo es para IAs (Claude, Cursor, Aider, etc.) que llegan a este repo con la tarea de construir un producto sobre Remora. Si sos humano, el [README.md](README.md) te sirve mejor.

## Lo que hay que saber en 60 segundos

Remora tiene **dos capas**. No te confundas entre ellas.

### Runtime (lo que ejecuta el producto que estás construyendo)

```
framework-llm      → llama al LLM (Anthropic ya implementado, stub para dev)
framework-agent    → primitiva Agent con Behavior, turn loop, estado
framework-channels → transporte (console funcional, whatsapp stub)
framework-store    → persistencia multi-tenant (memory + file)
framework-paladin  → traza semántica (se obtiene gratis con framework-agent)
```

### Diseño (herramientas que el founder usa ANTES de pedirte construir)

```
framework-echo  → discovery con el cliente (no se ejecuta en runtime)
framework-alfa  → compila el árbol de Echo a spec ejecutable
framework-bravo → verifica ejecución vs spec
framework-swarm → coordinación multi-agente con estigmergia (solo si hay N agentes)
```

Si te piden construir un agente conversacional, lo que necesitás es **runtime**, no diseño.

## Receta para "construir un agente conversacional con Remora"

1. Mirá [`examples/cobranza-conversacional`](examples/cobranza-conversacional) **primero**. Es un agente completo, end-to-end, funcional. Si lo que pide el founder se parece a eso, partí de ahí y modificá. **NO empieces desde cero.**

2. La pieza específica que el founder pone es un `agent.Behavior`. Definí esta interfaz:
   ```go
   type Behavior interface {
       Name() string
       Responsibility() string
       Goal(state State) string
       SystemPrompt(state State) string
       OnInput(state State, input string) Decision   // pre-LLM
       OnReply(state State, input, reply string) Decision  // post-LLM
   }
   ```
   Todo lo demás (turn loop, paladin spans, llm call, snapshot/restore) lo maneja `framework-agent`.

3. Cableado mínimo en `main.go`:
   ```go
   behavior := MiBehavior{ /* config del producto */ }
   llmClient := llm.NewClientOrStub(stubResponses...)
   ag := agent.New(&behavior, llmClient, behavior.InitialState())
   ch := console.New("Usuario: ", "MiAgente")
   st, _ := filestore.New("./conversations")

   // loop: ch.Receive → ag.Turn → ch.Send → st.Save(ag.Snapshot())
   ```

4. Para persistir entre procesos, **usá `framework-store`**, no escribas tu propio JSON writer. Patrón en [`examples/cobranza-conversacional/main.go`](examples/cobranza-conversacional/main.go).

5. Para producción con WhatsApp, hoy `framework-channels/channels/whatsapp` es stub. Tenés que implementarlo vos con tu proveedor (Twilio o Meta Cloud API) hasta que la librería lo absorba.

## Cosas que NO hagas — son fricciones que costó descubrir

- **No reescribas el wrapper LLM**. Usá `framework-llm/llm`. Si lo necesitás para otro proveedor, contribuyé una implementación, no copies el patrón a tu proyecto.
- **No reescribas el turn loop**. Si te encontrás escribiendo un `for` con `history = append(history, ...)`, parate — `framework-agent` ya lo tiene.
- **No reescribas un file store ad-hoc**. Usá `framework-store/store/file`.
- **No mezcles lógica de negocio con plumbing**. El `Behavior` es solo negocio. El runtime es Remora.

## Decisiones que NO podés tomar por el founder, y por qué

Cuando el founder te diga "construilo", hay decisiones que SÍ son tuyas (técnicas) y otras que NO (producto). El founder espera que vos decidas técnico solo:

| Tipo | Quién decide |
|---|---|
| Qué framework usar, qué interfaz implementar, qué archivos crear | Vos (IA) |
| Estructura de directorios, nombres de funciones, manejo de errores | Vos (IA) |
| Modelo LLM por defecto, timeouts, política de stub | Vos (IA) — son reversibles |
| El **catálogo de planes / reglas de negocio / tono del agente** | **Founder.** Pedíselo si no está en el brief. |
| El **canal de WhatsApp** (Twilio vs Meta vs WaSender vs otro) | **Founder.** Depende de creds y compliance. |
| **Persona y prompts específicos del producto** | **Founder.** Es su IP, no la inventes. |

Si el founder te delega una de las "producto", devolvele la decisión con opciones concretas. No la inventes silenciosamente.

## Si tenés memorias previas sobre el producto del founder

Esto es importante: si estás corriendo en una sesión con memorias persistentes (Cursor memory, Claude Code memory), y aterrizás en este repo con conocimiento previo del producto del founder (ej: "playbook de 5 actos", "22 reglas", etc.), **declarálo explícito al founder antes de usarlo**:

> "Tengo memorias previas sobre tu producto que mencionan X, Y, Z. ¿Querés que las use o construyamos solo con lo que está en este repo y el brief actual?"

Esto evita que el founder confunda lo que aporta Remora vs lo que aportan tus memorias. Cuando el founder esté testando si Remora le sirve a otros founders, el bias de memorias arruina el test.

## Si te trabás

- README raíz: [README.md](README.md) — la narrativa conceptual.
- READMEs por framework: cada `framework-*/README.md` documenta su pieza.
- Ejemplo funcional: [examples/cobranza-conversacional](examples/cobranza-conversacional).
- Tests del ejemplo: [examples/cobranza-conversacional/carolina_test.go](examples/cobranza-conversacional/carolina_test.go) — patrón de cómo testear sin red ni disco.

Si después de mirar todo eso no encontrás qué necesitás, **decile al founder qué te faltó**. Eso es feedback útil para Remora; no te quedes inventando.

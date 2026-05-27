# Ejemplo: Cobranza Conversacional

Agente conversacional **Carolina** para una fintech ficticia ("FinCrowd") que necesita cobrar pagos atrasados a sus inversionistas vía WhatsApp.

Este ejemplo demuestra cómo combinar los cuatro frameworks runtime de Remora — `framework-llm`, `framework-agent`, `framework-channels`, `framework-store` — para construir un agente conversacional persistente, observable, y listo para producir en menos de 400 líneas de código de negocio.

> Disclaimer: FinCrowd, Patricia, Roberto y Marta son ficticios. Los planes de pago son ejemplos. Si te interesa adaptar esto para un caso real, todo el código que es lógica de negocio vive en `carolina.go` + `debtor.go`.

## Cómo correr

```bash
go build .
./cobranza-conversacional                          # modo stub determinístico (sin tokens)
ANTHROPIC_API_KEY=sk-... ./cobranza-conversacional # modo Claude API real

# Perfiles de deudor
DEBTOR_PROFILE=patricia ./cobranza-conversacional  # cooperativa, deuda mediana
DEBTOR_PROFILE=roberto  ./cobranza-conversacional  # combativo, deuda alta, formal
DEBTOR_PROFILE=marta    ./cobranza-conversacional  # negociadora, primera mora
```

Escribís como el deudor en stdin. Carolina responde. Termina cuando hay acuerdo, escalación, o silencio.

### Códigos de salida
- `0` — acuerdo alcanzado
- `1` — escalado a humano (hostilidad o sin avance)
- `2` — deudor no responde (canal cerrado)
- `3` — error técnico

### Persistencia

Cada conversación se guarda en `./conversations/<deudor_id>.json`. Si volvés a correr con el mismo `DEBTOR_PROFILE`, retoma desde donde quedó. Si la conversación ya cerró (acuerdo o escalada), se rehúsa a reabrir.

## Qué muestra el ejemplo

| Comportamiento | Dónde vive en el código |
|---|---|
| Persona Carolina con tono adaptable (formal/cercano) | `SystemPrompt()` en `carolina.go` |
| Catálogo de planes — Carolina no inventa fuera del catálogo | `CatalogoPlanes()` en `debtor.go` |
| Detección de hostilidad → escalación inmediata | `OnInput()` + `detectarHostilidad()` |
| 3 rechazos seguidos → escalación por sin-avance | `OnInput()` + state `rechazos_seguidos` |
| Acuerdo detectado por confirmación del deudor | `OnReply()` + `detectarConfirmacion()` |
| Traza Paladin por turno con actor, goal, decisiones | runtime de `framework-agent` |
| Persistencia entre procesos | `framework-store/store/file` |

## Cómo se ve consumir Remora desde acá

`main.go` (resumido):

```go
behavior := NewCarolinaBehavior(debtor)
carolina := agent.New(behavior, LLMClient, behavior.InitialState())
ch := console.New("Deudor: ", "Carolina")  // cambiar por whatsapp.New(cfg) en prod
st, _ := filestore.New("./conversations")

// loop: ch.Receive → carolina.Turn → ch.Send → st.Save(snapshot)
```

Esas son las cuatro líneas que un founder no-técnico necesita aprender. Toda la lógica específica de cobranza vive en `CarolinaBehavior`, que implementa `agent.Behavior`.

## Tests

```bash
go test .
```

Seis tests herméticos (memory store + stub LLM, sin red, sin disco) que cubren:
- Happy path → `agreed`.
- Hostilidad → `escalated` inmediato.
- Snapshot+restore preserva state.
- ErrNotFound en conversation_id desconocido.
- Outcome terminal sticky (no acepta más turnos).
- Restore respeta outcome terminal.

## Próximo paso si lo querés adaptar a tu propio caso

1. Cambiar `Debtor`, `PlanPago` y `CatalogoPlanes` en `debtor.go` por tu modelo de datos.
2. Reescribir `SystemPrompt()` con la persona de tu agente y tus reglas de negocio.
3. Cambiar `detectarHostilidad`/`detectarConfirmacion` por tus señales.
4. Cuando tengas credenciales de WhatsApp Business (Twilio o Meta), reemplazar `console.New(...)` por `whatsapp.New(cfg)` y nada más cambia.

El resto — turn loop, llm, paladin, store — es Remora, no lo reescribís.

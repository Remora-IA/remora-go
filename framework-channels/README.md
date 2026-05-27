# framework-channels

Primitiva de Remora para canales de comunicación: el transporte por el que un agente envía y recibe mensajes con el mundo real.

## Why

El agente no debería saber si está hablando por WhatsApp, consola, email o SMS. Eso es plumbing, no negocio. Con esta librería el mismo `Behavior` corre en `console.New(...)` durante desarrollo y en `whatsapp.New(cfg)` en producción sin tocar la lógica de negocio.

## Interfaz

```go
type Channel interface {
    Send(ctx context.Context, to, text string) error
    Receive(ctx context.Context) (Message, error)
    Close() error
}
```

Receive bloquea hasta que llega un mensaje (o ctx se cancela). Send envía texto. Close libera recursos.

## Implementaciones

| Paquete | Estado | Notas |
|---|---|---|
| `channels/console` | ✅ funcional | stdin/stdout. ENTER vacío = ErrClosed. |
| `channels/whatsapp` | ⚠️ stub | Interfaz lista; integración real requiere Twilio o Meta Cloud API + webhook + templates pre-aprobados. Ver doc del paquete. |

## Por qué WhatsApp está como stub

Implementar WhatsApp real implica:

1. Decidir proveedor (Twilio vs Meta directo).
2. Manejar webhook entrante con servidor HTTPS público.
3. Cumplir reglas de WhatsApp Business: templates aprobados para iniciar conversación, opt-in del destinatario, ventana de 24h.
4. Credenciales y costos por mensaje.

Cada uno de esos pasos depende de decisiones del producto que no se toman en código (qué proveedor, qué número de empresa, qué templates aprueba Meta). El stub deja el diseño cerrado y documenta qué falta para que sea real.

## Primer consumidor

[`examples/cobranza-conversacional`](../examples/cobranza-conversacional) usa `console.Channel`. El día que tu proyecto tenga credenciales WhatsApp aprobadas, cambia una línea (`console.New(...)` por `whatsapp.New(cfg)`) y el agente opera en producción.

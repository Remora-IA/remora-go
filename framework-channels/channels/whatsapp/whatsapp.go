// Package whatsapp es el stub de integración WhatsApp para Remora.
//
// ESTADO: stub. La interfaz channels.Channel está implementada pero
// Send/Receive devuelven ErrNotImplemented. Para hacerlo real hay que:
//
//  1. Decidir proveedor: Twilio WhatsApp Business API o Meta Cloud API.
//     - Twilio: más simple, plantilla aprobada requerida, ~$0.005/msg.
//     - Meta directo: más barato a escala, setup más complejo, requiere
//       verificación de WhatsApp Business Account.
//
//  2. Implementar Send: POST al endpoint del proveedor con auth + body.
//
//  3. Implementar Receive: webhook entrante (HTTPS público, túnel ngrok
//     en dev). El webhook empuja mensajes a un canal Go; Receive lee de
//     ese canal. Esto implica:
//     - Levantar un http.Server en este paquete.
//     - Verificar firmas del proveedor.
//     - Mapear payloads del proveedor al Message de Remora.
//
//  4. Templates y opt-in: WhatsApp Business exige que el primer mensaje
//     a un número que no inició la conversación use un template
//     pre-aprobado. Para Kobra (donde Carolina inicia el contacto al
//     deudor) esto es load-bearing — el template tiene que estar listo.
//
// Hasta que estos 4 puntos se resuelvan, este paquete sirve solo para:
//   - Documentar lo que falta.
//   - Compilar contra la interfaz como compile-time check de que el
//     diseño aguanta una integración real.
package whatsapp

import (
	"context"
	"errors"

	"github.com/Remora-IA/remora-go/framework-channels/channels"
)

// ErrNotImplemented marca operaciones que requieren credenciales y
// webhook configurados.
var ErrNotImplemented = errors.New("whatsapp: not implemented — ver doc del paquete")

// Config son las credenciales mínimas para una integración futura.
type Config struct {
	Provider    string // "twilio" o "meta"
	AccountSID  string // Twilio
	AuthToken   string // Twilio
	PhoneNumber string // número WhatsApp Business desde el que se envía
	WebhookURL  string // URL pública del webhook entrante
	TemplateSID string // template aprobado para iniciar conversación
}

// Channel implementa channels.Channel sobre WhatsApp.
type Channel struct {
	cfg Config
}

// New crea un channel WhatsApp. Hoy devuelve ErrNotImplemented en
// Send/Receive; sirve para validar el diseño y para que un developer
// futuro complete la integración sin tocar consumidores.
func New(cfg Config) *Channel {
	return &Channel{cfg: cfg}
}

func (c *Channel) Send(_ context.Context, to, text string) error {
	_ = to
	_ = text
	return ErrNotImplemented
}

func (c *Channel) Receive(_ context.Context) (channels.Message, error) {
	return channels.Message{}, ErrNotImplemented
}

func (c *Channel) Close() error { return nil }

// Package channels define la interfaz Channel: el canal por el que un
// agente Remora envía y recibe mensajes con el mundo real (consola,
// WhatsApp, email, SMS, etc).
//
// Un Channel oculta el protocolo. Para el agente todos los canales se
// ven igual: recibe un Message, manda un texto. Esto permite que el
// mismo Behavior corra en consola durante desarrollo y en WhatsApp en
// producción sin tocar la lógica de negocio.
package channels

import (
	"context"
	"errors"
	"time"
)

// Message es un mensaje entrante.
type Message struct {
	From      string    // identificador del remitente (número WhatsApp, ID de usuario, etc)
	Text      string    // contenido
	Timestamp time.Time // cuándo se recibió
	Meta      map[string]string
}

// Channel es la interfaz que cualquier transporte implementa.
//
// Diseño:
//   - Receive bloquea hasta que llega un mensaje (o ctx se cancela).
//   - Send envía texto a un destinatario; el destinatario en single-
//     conversation puede venir vacío y el canal lo resuelve.
//   - Close libera recursos (cierra sockets, clientes HTTP, etc).
type Channel interface {
	Send(ctx context.Context, to, text string) error
	Receive(ctx context.Context) (Message, error)
	Close() error
}

// ErrClosed se devuelve cuando se intenta usar un canal ya cerrado, o
// cuando Receive se desbloquea porque la otra punta cerró la conversación
// (ej: usuario presiona ENTER vacío en consola).
var ErrClosed = errors.New("channels: closed")

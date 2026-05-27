// Package console implementa Channel sobre stdin/stdout.
//
// Útil para desarrollo y simulación. El agente corre exactamente igual
// que en WhatsApp; lo que cambia es el transporte.
package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/remora-go/framework-channels/channels"
)

// Channel implementa channels.Channel sobre stdin/stdout.
type Channel struct {
	in       *bufio.Reader
	out      io.Writer
	prompt   string // ej: "Patricia: "
	speaker  string // ej: "Carolina"
	closedCh chan struct{}
}

// New crea un canal de consola. prompt es lo que se muestra antes del
// input del usuario; speaker es el nombre que precede los mensajes que
// el agente envía.
func New(prompt, speaker string) *Channel {
	return &Channel{
		in:       bufio.NewReader(os.Stdin),
		out:      os.Stdout,
		prompt:   prompt,
		speaker:  speaker,
		closedCh: make(chan struct{}),
	}
}

func (c *Channel) Send(_ context.Context, _, text string) error {
	_, err := fmt.Fprintf(c.out, "\n%s: %s\n", c.speaker, text)
	return err
}

func (c *Channel) Receive(ctx context.Context) (channels.Message, error) {
	select {
	case <-c.closedCh:
		return channels.Message{}, channels.ErrClosed
	default:
	}

	fmt.Fprintf(c.out, "\n%s", c.prompt)
	line, err := c.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if err == io.EOF && line == "" {
		return channels.Message{}, channels.ErrClosed
	}
	if line == "" {
		return channels.Message{}, channels.ErrClosed
	}
	return channels.Message{
		From:      "console",
		Text:      line,
		Timestamp: time.Now(),
	}, nil
}

func (c *Channel) Close() error {
	select {
	case <-c.closedCh:
	default:
		close(c.closedCh)
	}
	return nil
}

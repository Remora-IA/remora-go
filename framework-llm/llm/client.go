// Package llm es la primitiva de Remora para llamar a modelos de lenguaje.
//
// Provee una interfaz agnóstica de proveedor (Client) y una implementación
// para Anthropic (NewAnthropic). Otros proveedores se agregan implementando
// Client.
//
// Diseñado para que founders no-técnicos no tengan que escribir wrappers
// HTTP a mano cada vez que armen un agente.
package llm

import (
	"context"
	"errors"
)

// Message es un turno de conversación. Role es "user" o "assistant".
type Message struct {
	Role    string
	Content string
}

// Request es lo que un consumidor le pide al LLM en una llamada.
type Request struct {
	System    string
	Messages  []Message
	MaxTokens int    // opcional; 0 = default del proveedor
	Model     string // opcional; vacío = default del proveedor
}

// Response es lo que devuelve el LLM.
type Response struct {
	Text       string
	StopReason string
}

// Client es la interfaz que cualquier proveedor de LLM debe cumplir
// para ser usable desde Remora.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}

// ErrNoCredentials se devuelve cuando el cliente no tiene cómo autenticarse.
// Útil para que el consumidor decida si caer a modo stub.
var ErrNoCredentials = errors.New("llm: no credentials configured")

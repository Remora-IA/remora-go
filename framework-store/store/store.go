// Package store define la interfaz de persistencia de agentes Remora.
//
// Un Store guarda y recupera Snapshots por conversation_id. Permite que
// un agente pause, persista, y retome — y que múltiples conversaciones
// concurrentes vivan en paralelo sin pisarse.
package store

import (
	"context"
	"errors"

	"github.com/Remora-IA/remora-go/framework-agent/agent"
)

// ErrNotFound se devuelve cuando un conversation_id no existe.
var ErrNotFound = errors.New("store: conversation not found")

// Store es la interfaz que cualquier backend (memoria, archivo, SQLite,
// Redis) debe cumplir.
type Store interface {
	// Save persiste el snapshot bajo conversationID. Sobrescribe si ya existe.
	Save(ctx context.Context, conversationID string, snap *agent.Snapshot) error
	// Load recupera un snapshot. Devuelve ErrNotFound si no existe.
	Load(ctx context.Context, conversationID string) (*agent.Snapshot, error)
	// List devuelve los conversation_ids conocidos. Útil para correr todas
	// las conversaciones activas, hacer audits, etc.
	List(ctx context.Context) ([]string, error)
	// Delete remueve una conversación. No es error si no existe.
	Delete(ctx context.Context, conversationID string) error
}

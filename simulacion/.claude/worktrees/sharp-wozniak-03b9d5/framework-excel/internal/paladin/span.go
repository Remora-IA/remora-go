package paladin

import (
	"time"
)

// Span representa un span de ejecucion.
type Span struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	Vars      Variables `json:"vars,omitempty"`
	Decisions []Decision `json:"decisions,omitempty"`
	Children  []*Span   `json:"children,omitempty"`
	Error     string    `json:"error,omitempty"`
	Logs      []LogEntry `json:"logs,omitempty"`
}

// Variables mapa de variables.
type Variables map[string]string

// Decision representa una decision en el trace.
type Decision struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// LogEntry representa una entrada de log.
type LogEntry struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// AddChild agrega un hijo al span.
func (s *Span) AddChild(child *Span) {
	s.Children = append(s.Children, child)
}
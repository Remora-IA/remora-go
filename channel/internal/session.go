package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionLogger appendea cada interacción RPC a sessions/<id>.jsonl
// dentro del BaseDir. Stateless por sesión (un archivo por sesión).
type SessionLogger struct {
	dir string
	mu  sync.Mutex
}

// NewSessionLogger crea el logger; el directorio se crea si no existe.
func NewSessionLogger(baseDir string) *SessionLogger {
	dir := filepath.Join(baseDir, "sessions")
	_ = os.MkdirAll(dir, 0755)
	return &SessionLogger{dir: dir}
}

// Entry es lo que se persiste por cada llamada.
type Entry struct {
	Timestamp  string                 `json:"ts"`
	SessionID  string                 `json:"session_id"`
	Method     string                 `json:"method"`
	Params     map[string]interface{} `json:"params,omitempty"`
	APIKeyHash string                 `json:"api_key_hash"`
	Response   Response               `json:"response"`
}

// Append escribe una entry al archivo de la sesión.
// Si sessionID está vacío, no hace nada (sesión opcional).
func (s *SessionLogger) Append(sessionID string, entry Entry) {
	if sessionID == "" || s == nil {
		return
	}
	// Sanear: solo [a-zA-Z0-9_-]
	for _, c := range sessionID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' || c == '-' || c == '_') {
			return
		}
	}
	if len(sessionID) > 64 {
		return
	}

	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	entry.SessionID = sessionID

	path := filepath.Join(s.dir, sessionID+".jsonl")
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
	f.Write([]byte("\n"))
}

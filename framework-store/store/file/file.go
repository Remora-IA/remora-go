// Package file implementa store.Store con un archivo JSON por conversación.
//
// Cada conversación se guarda en <Dir>/<conversationID>.json. Suficiente
// para MVP y para que el founder pueda inspeccionar conversaciones a mano.
// Para escala (miles de conversaciones concurrentes) cambiar a SQLite.
package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Remora-IA/remora-go/framework-agent/agent"
	"github.com/Remora-IA/remora-go/framework-store/store"
)

type Store struct {
	dir string
	mu  sync.Mutex // serializa writes para evitar carreras dentro del proceso
}

// New crea un store que persiste en dir. Crea el directorio si no existe.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("file store: mkdir %s: %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(id string) string {
	safe := strings.ReplaceAll(id, string(filepath.Separator), "_")
	return filepath.Join(s.dir, safe+".json")
}

func (s *Store) Save(_ context.Context, id string, snap *agent.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmp := s.path(id) + ".tmp"
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("file store: marshal: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("file store: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path(id)); err != nil {
		return fmt.Errorf("file store: rename: %w", err)
	}
	return nil
}

func (s *Store) Load(_ context.Context, id string) (*agent.Snapshot, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("file store: read: %w", err)
	}
	var snap agent.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("file store: unmarshal: %w", err)
	}
	return &snap, nil
}

func (s *Store) List(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("file store: readdir: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

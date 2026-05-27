// Package memory implementa store.Store en memoria. Útil para tests
// y para correr el MVP sin tocar disco.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/remora-go/framework-agent/agent"
	"github.com/remora-go/framework-store/store"
)

type Store struct {
	mu   sync.RWMutex
	data map[string]*agent.Snapshot
}

func New() *Store {
	return &Store{data: map[string]*agent.Snapshot{}}
}

func (s *Store) Save(_ context.Context, id string, snap *agent.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = snap
	return nil
}

func (s *Store) Load(_ context.Context, id string) (*agent.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return snap, nil
}

func (s *Store) List(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.data))
	for id := range s.data {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

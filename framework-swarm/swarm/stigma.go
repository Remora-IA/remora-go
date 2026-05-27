package swarm

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// StigmaStore is the stigmergic substrate of the swarm.
//
// Stigmergy is coordination through environment modification: agents
// don't communicate directly; they leave signals in the environment
// (pheromones) and react to signals left by others.
//
// StigmaStore is thread-safe and optionally file-backed, so multiple
// goroutines (or future processes) can share the same pheromone field.
type StigmaStore struct {
	mu         sync.RWMutex
	pheromones []*Pheromone
	path       string // optional JSON file for persistence
}

// NewStigmaStore creates a fresh in-memory pheromone store.
func NewStigmaStore() *StigmaStore {
	return &StigmaStore{}
}

// NewStigmaStoreFromFile creates a file-backed store, loading any existing state.
// If the file does not exist, an empty store is created (the file will be
// written on the first Leave call).
func NewStigmaStoreFromFile(path string) (*StigmaStore, error) {
	s := &StigmaStore{path: path}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// Leave deposits a pheromone signal into the environment.
// LeftAt is set automatically to now.
func (s *StigmaStore) Leave(p *Pheromone) {
	p.LeftAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pheromones = append(s.pheromones, p)
	_ = s.saveLocked()
}

// Sense returns all active (non-evaporated) pheromones for a zone.
func (s *StigmaStore) Sense(zoneID string) []*Pheromone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Pheromone
	for _, p := range s.pheromones {
		if p.Zone == zoneID && p.CurrentStrength() > 0 {
			result = append(result, p)
		}
	}
	return result
}

// SenseAll returns all active pheromones across every zone.
func (s *StigmaStore) SenseAll() []*Pheromone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Pheromone
	for _, p := range s.pheromones {
		if p.CurrentStrength() > 0 {
			result = append(result, p)
		}
	}
	return result
}

// IsSolved returns true if any agent left a permanent "solved" pheromone for this zone.
func (s *StigmaStore) IsSolved(zoneID string) bool {
	for _, p := range s.Sense(zoneID) {
		if p.Signal == PheromoneSolved && p.CurrentStrength() > 0 {
			return true
		}
	}
	return false
}

// ActiveAgents counts agents currently exploring a zone (exploring pheromone, not expired).
func (s *StigmaStore) ActiveAgents(zoneID string) int {
	count := 0
	for _, p := range s.Sense(zoneID) {
		if p.Signal == PheromoneExploring {
			count++
		}
	}
	return count
}

// Claim atomically checks-and-sets an "exploring" pheromone for agentID on zoneID.
//
// Returns true if the claim succeeded (no other agent has an active exploring
// pheromone there). Returns false if another agent already claimed the zone.
//
// This is the stigmergic equivalent of a mutex: the first agent to leave a
// pheromone "owns" the zone; others sense it and navigate elsewhere.
func (s *StigmaStore) Claim(zoneID, agentID string, duration time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reject if another agent already has an active exploring pheromone
	for _, p := range s.pheromones {
		if p.Zone == zoneID &&
			p.Signal == PheromoneExploring &&
			p.AgentID != agentID &&
			p.CurrentStrength() > 0 {
			return false
		}
	}

	// Claim the zone
	now := time.Now()
	p := &Pheromone{
		Zone:      zoneID,
		Signal:    PheromoneExploring,
		Strength:  1.0,
		AgentID:   agentID,
		LeftAt:    now,
		ExpiresAt: now.Add(duration),
	}
	s.pheromones = append(s.pheromones, p)
	_ = s.saveLocked()
	return true
}

// Evaporate removes all pheromones whose strength has dropped to zero.
// Returns the number of pheromones removed. Call periodically for long-lived swarms.
func (s *StigmaStore) Evaporate() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.pheromones[:0]
	removed := 0
	for _, p := range s.pheromones {
		if p.CurrentStrength() > 0 {
			active = append(active, p)
		} else {
			removed++
		}
	}
	s.pheromones = active
	_ = s.saveLocked()
	return removed
}

// Snapshot returns a copy of the full pheromone state for inspection or reporting.
func (s *StigmaStore) Snapshot() []*Pheromone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Pheromone, len(s.pheromones))
	copy(out, s.pheromones)
	return out
}

func (s *StigmaStore) load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.pheromones)
}

func (s *StigmaStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.pheromones, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

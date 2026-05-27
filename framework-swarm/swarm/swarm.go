package swarm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/remora-go/framework-paladin/paladin"
)

// Config configures a Swarm before creation.
type Config struct {
	// ID identifies this swarm run. Auto-generated if empty.
	ID string

	// Zones is the problem space the swarm will work on.
	// PainWeight on each Zone drives the pressure field.
	Zones []Zone

	// AgentIDs are the identifiers for the agents to spawn.
	// One goroutine is created per agent.
	AgentIDs []string

	// WorkFunc is the function each agent calls for its assigned zone.
	WorkFunc WorkFunc

	// StigmaPath is an optional file path for a persistent pheromone store.
	// If empty, an in-memory store is used.
	StigmaPath string
}

// Swarm orchestrates multiple agents working on a shared problem space.
//
// Agents coordinate exclusively via the StigmaStore — there is no
// central scheduler or message passing. This is pure stigmergy.
type Swarm struct {
	ID     string
	zones  []Zone
	agents []*Agent
	stigma *StigmaStore
	trace  *paladin.Trace
	rootCtx *paladin.Context
}

// New creates a Swarm from the given Config.
//
// It initialises the StigmaStore, creates a Paladin trace for the whole
// swarm run, and spawns Agent objects (goroutines are started only when
// Run is called).
func New(cfg Config) (*Swarm, error) {
	if len(cfg.Zones) == 0 {
		return nil, fmt.Errorf("swarm: at least one zone required")
	}
	if len(cfg.AgentIDs) == 0 {
		return nil, fmt.Errorf("swarm: at least one agent required")
	}
	if cfg.WorkFunc == nil {
		return nil, fmt.Errorf("swarm: WorkFunc must not be nil")
	}

	// Initialise stigma store
	var stigma *StigmaStore
	var err error
	if cfg.StigmaPath != "" {
		stigma, err = NewStigmaStoreFromFile(cfg.StigmaPath)
		if err != nil {
			return nil, fmt.Errorf("swarm: stigma store: %w", err)
		}
	} else {
		stigma = NewStigmaStore()
	}

	id := cfg.ID
	if id == "" {
		id = fmt.Sprintf("swarm-%d", time.Now().UnixMilli())
	}

	// Paladin trace — one trace for the entire swarm run
	trace := paladin.NewTrace(id)
	rootCtx := trace.Start()
	rootCtx.Actor("swarm", fmt.Sprintf("coordinator for %d agents", len(cfg.AgentIDs)))
	rootCtx.Goal(fmt.Sprintf("solve %d zones via stigmergy", len(cfg.Zones)))
	rootCtx.Rule(
		"no-central-coordinator",
		"agents navigate independently via pressure fields and pheromones",
		nil,
	)

	// Create one agent per ID, each with its own Paladin child context
	agents := make([]*Agent, len(cfg.AgentIDs))
	for i, aid := range cfg.AgentIDs {
		agentCtx := rootCtx.Child(fmt.Sprintf("agent.%s", aid))
		agentCtx.Actor(aid, "worker agent")
		agentCtx.Goal("navigate pressure field, work zones, leave pheromones")
		agents[i] = newAgent(aid, stigma, cfg.WorkFunc, agentCtx)
	}

	return &Swarm{
		ID:      id,
		zones:   cfg.Zones,
		agents:  agents,
		stigma:  stigma,
		trace:   trace,
		rootCtx: rootCtx,
	}, nil
}

// Run launches all agents concurrently and waits for the swarm to converge.
//
// The swarm terminates when every agent finds no zones with positive pressure
// (i.e., all solvable zones are solved). Returns a SwarmResult with
// metrics and all per-zone results.
func (s *Swarm) Run(ctx context.Context) (*SwarmResult, error) {
	start := time.Now()

	// Buffer enough so no goroutine blocks even if caller is slow
	resultCh := make(chan *Result, len(s.zones)*len(s.agents)+1)

	var wg sync.WaitGroup
	for _, agent := range s.agents {
		wg.Add(1)
		go func(a *Agent) {
			defer wg.Done()
			a.runLoop(ctx, s.zones, resultCh)
		}(agent)
	}

	// Close resultCh once all agents have finished
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []*Result
	for r := range resultCh {
		results = append(results, r)
	}

	// --- Compute metrics ---

	solved, blocked := 0, 0
	for _, z := range s.zones {
		if s.stigma.IsSolved(z.ID) {
			solved++
		} else {
			// check if any blocked pheromone exists for an unsolved zone
			for _, p := range s.stigma.Sense(z.ID) {
				if p.Signal == PheromoneBlocked {
					blocked++
					break
				}
			}
		}
	}

	// Collision rate: fraction of zones worked by more than one agent
	zoneAgents := make(map[string]int)
	for _, r := range results {
		zoneAgents[r.ZoneID]++
	}
	collisions := 0
	for _, count := range zoneAgents {
		if count > 1 {
			collisions++
		}
	}
	collisionRate := 0.0
	if len(s.zones) > 0 {
		collisionRate = float64(collisions) / float64(len(s.zones))
	}

	// Record summary in Paladin
	s.rootCtx.Check(
		"all-zones-solved",
		fmt.Sprintf("%d", len(s.zones)),
		fmt.Sprintf("%d", solved),
		solved == len(s.zones),
	)
	s.rootCtx.Var("collision_rate", fmt.Sprintf("%.2f%%", collisionRate*100))
	s.rootCtx.End()

	// Flush Paladin trace to disk
	s.trace.Flush()

	flatResults := make([]Result, len(results))
	for i, r := range results {
		flatResults[i] = *r
	}

	return &SwarmResult{
		SwarmID:       s.ID,
		TotalZones:    len(s.zones),
		SolvedZones:   solved,
		BlockedZones:  blocked,
		TotalAgents:   len(s.agents),
		Duration:      time.Since(start),
		CollisionRate: collisionRate,
		Results:       flatResults,
	}, nil
}

// StigmaSnapshot returns all current pheromones for inspection or reporting.
func (s *Swarm) StigmaSnapshot() []*Pheromone {
	return s.stigma.Snapshot()
}

// Zones returns the zone list this swarm was configured with.
func (s *Swarm) Zones() []Zone {
	return s.zones
}

// PressureTable returns the current pressure for every zone.
// Useful for visualising the field state at any moment.
func (s *Swarm) PressureTable() []ZonePressure {
	return PressureTable(s.zones, s.stigma)
}

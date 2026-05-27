package swarm

import (
	"context"
	"fmt"
	"time"

	"github.com/remora-go/framework-paladin/paladin"
)

// WorkFunc is the function an agent executes for a zone.
// It receives the zone and the agent itself (for stigma access and metadata),
// and must return a Result on success or an error on failure.
type WorkFunc func(ctx context.Context, zone Zone, agent *Agent) (*Result, error)

// Agent is a single member of the swarm with its own identity,
// Paladin context, and access to the shared StigmaStore.
//
// Agents do not communicate directly with each other.
// All coordination happens through the StigmaStore (stigmergy).
type Agent struct {
	// ID is the unique identifier for this agent within the swarm.
	ID string

	stigma *StigmaStore
	work   WorkFunc
	trace  *paladin.Context // Paladin semantic context for this agent
}

// newAgent creates a new swarm agent. Called by Swarm internally.
func newAgent(id string, stigma *StigmaStore, work WorkFunc, trace *paladin.Context) *Agent {
	return &Agent{
		ID:     id,
		stigma: stigma,
		work:   work,
		trace:  trace,
	}
}

// Navigate selects the best available zone for this agent to work on.
// Returns nil when all zones are solved or have zero pressure.
func (a *Agent) Navigate(zones []Zone) *Zone {
	return Navigate(zones, a.stigma, a.ID)
}

// Stigma returns read access to the shared pheromone store.
// Useful within WorkFunc implementations that need to sense the environment.
func (a *Agent) Stigma() *StigmaStore {
	return a.stigma
}

// Work executes the agent's task on a zone, automatically managing pheromones:
//  1. Leaves a temporary "exploring" pheromone (expires in 10 min)
//  2. Calls the WorkFunc
//  3. Leaves a permanent "solved" pheromone on success,
//     or a "blocked" pheromone on failure
//
// All actions are recorded as semantic events in the agent's Paladin context.
func (a *Agent) Work(ctx context.Context, zone Zone) (*Result, error) {
	start := time.Now()

	// Open Paladin span for this zone
	var agentCtx *paladin.Context
	if a.trace != nil {
		agentCtx = a.trace.Child(fmt.Sprintf("zone.%s", zone.ID))
		agentCtx.Actor(a.ID, "swarm agent")
		agentCtx.Goal(fmt.Sprintf("process zone: %s", zone.Name))
		agentCtx.Event(
			"stigma.exploring",
			fmt.Sprintf("agent %s claiming zone %s (pain=%.2f)", a.ID, zone.ID, zone.PainWeight),
			nil,
		)
	}

	// Note: the "exploring" pheromone was already left atomically by Navigate via Claim.
	// We don't leave a duplicate here.

	// Execute the work
	result, err := a.work(ctx, zone, a)
	duration := time.Since(start)

	if result == nil {
		result = &Result{ZoneID: zone.ID, AgentID: a.ID}
	}
	result.Duration = duration
	result.ZoneID = zone.ID
	result.AgentID = a.ID

	if err != nil {
		// Leave permanent "blocked" pheromone — signals problem to future agents
		a.stigma.Leave(&Pheromone{
			Zone:     zone.ID,
			Signal:   PheromoneBlocked,
			Strength: 0.8,
			AgentID:  a.ID,
		})
		result.Success = false
		result.Error = err.Error()

		if agentCtx != nil {
			agentCtx.Error(err)
			agentCtx.Violation("zone.outcome", "success", "blocked: "+err.Error())
			agentCtx.End()
		}
	} else {
		// Leave permanent "solved" pheromone — zone will have 0 pressure henceforth
		a.stigma.Leave(&Pheromone{
			Zone:     zone.ID,
			Signal:   PheromoneSolved,
			Strength: 1.0,
			AgentID:  a.ID,
			// ExpiresAt is zero → permanent
		})
		result.Success = true

		if agentCtx != nil {
			agentCtx.Event(
				"stigma.solved",
				fmt.Sprintf("zone %s completed in %dms", zone.ID, duration.Milliseconds()),
				nil,
			)
			agentCtx.Handoff(a.ID, "swarm", "zone released, pheromone left for peers")
			agentCtx.End()
		}
	}

	return result, err
}

// runLoop drives an agent through the full work cycle:
// Navigate → Work → Navigate → Work → ... until no zones remain.
// Results are sent to out channel. Returns when there is nothing left to do.
func (a *Agent) runLoop(ctx context.Context, zones []Zone, out chan<- *Result) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		zone := a.Navigate(zones)
		if zone == nil {
			// All zones solved or no pressure remaining
			return
		}

		result, _ := a.Work(ctx, *zone)
		if result != nil {
			out <- result
		}

		// Brief pause to let pheromones propagate before re-navigating.
		// In a distributed setting this would be replaced by an event wait.
		time.Sleep(20 * time.Millisecond)
	}
}

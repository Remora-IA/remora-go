// Package swarm implements a bio-inspired multi-agent orchestration layer for Remora.
//
// Agents coordinate via stigmergy (pheromone fields) without a central coordinator,
// inspired by Ant Colony Optimization. Each agent:
//   - Senses the pressure field (urgency × density)
//   - Navigates toward the highest-pressure unclaimed zone
//   - Works the zone and leaves pheromone signals
//   - Other agents adapt navigation based on accumulated signals
//
// The tripod:
//   - Echo defines the problem space (pain weights → pressure field)
//   - Paladin records semantic traces of each agent's actions
//   - Bravo verifies the collective output against the ideal flow
package swarm

import "time"

// PheromoneType represents the kind of signal an agent leaves in a zone.
type PheromoneType string

const (
	// PheromoneExploring signals an agent is actively working on this zone.
	PheromoneExploring PheromoneType = "exploring"
	// PheromoneSolved signals a zone has been completed successfully.
	PheromoneSolved PheromoneType = "solved"
	// PheromoneBlocked signals a zone encountered an obstacle.
	PheromoneBlocked PheromoneType = "blocked"
	// PheromoneProm signals a zone looks particularly valuable to explore.
	PheromoneProm PheromoneType = "promising"
)

// Pheromone is a signal left by an agent in a zone.
// It may decay over time (evaporation) if ExpiresAt is set.
type Pheromone struct {
	Zone      string        `json:"zone"`
	Signal    PheromoneType `json:"signal"`
	Strength  float64       `json:"strength"`            // 0.0–1.0
	AgentID   string        `json:"agent_id"`
	LeftAt    time.Time     `json:"left_at"`
	ExpiresAt time.Time     `json:"expires_at,omitempty"` // zero = permanent
}

// CurrentStrength returns the pheromone's effective strength,
// accounting for linear evaporation toward the expiry time.
func (p *Pheromone) CurrentStrength() float64 {
	if p.ExpiresAt.IsZero() {
		return p.Strength
	}
	now := time.Now()
	if now.After(p.ExpiresAt) {
		return 0
	}
	total := p.ExpiresAt.Sub(p.LeftAt)
	if total <= 0 {
		return 0
	}
	remaining := p.ExpiresAt.Sub(now)
	return p.Strength * float64(remaining) / float64(total)
}

// Zone is a unit of work in the swarm's problem space.
// PainWeight comes from Echo's pain/opportunity tree — it encodes
// how urgently this zone needs to be addressed.
type Zone struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	PainWeight  float64        `json:"pain_weight"` // 0.0–1.0: urgency from Echo
	Tags        []string       `json:"tags,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// ZonePressure is the computed pressure for a zone at a given moment.
// Higher pressure means more attractive to navigate toward.
//
//	Pressure = PainWeight / (1 + AgentDensity) × (1 − SolvedRatio)
//
// Solved zones have 0 pressure; untouched urgent zones have highest pressure.
type ZonePressure struct {
	Zone         Zone
	AgentDensity int     // agents currently exploring this zone
	SolvedRatio  float64 // 0–1: proportion solved
	Pressure     float64 // net pressure
}

// Result is the output produced by one agent completing one zone.
type Result struct {
	ZoneID    string        `json:"zone_id"`
	AgentID   string        `json:"agent_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Artifacts []Artifact    `json:"artifacts,omitempty"`
}

// Artifact is a file or piece of content produced by an agent.
type Artifact struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Kind    string `json:"kind"` // "markdown", "json", "code", etc.
}

// SwarmResult is the collective output of all agents after a full run.
type SwarmResult struct {
	SwarmID       string        `json:"swarm_id"`
	TotalZones    int           `json:"total_zones"`
	SolvedZones   int           `json:"solved_zones"`
	BlockedZones  int           `json:"blocked_zones"`
	TotalAgents   int           `json:"total_agents"`
	Duration      time.Duration `json:"duration"`
	CollisionRate float64       `json:"collision_rate"` // fraction of zones worked by >1 agent
	Results       []Result      `json:"results"`
}

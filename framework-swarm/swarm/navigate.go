package swarm

import (
	"sort"
	"time"
)

// ComputePressure calculates the attraction pressure for every zone.
//
// The formula mirrors ACO (Ant Colony Optimization):
//
//	Pressure = PainWeight / (1 + AgentDensity) × (1 − SolvedRatio)
//
//   - PainWeight (from Echo's pain/opportunity tree) encodes urgency.
//   - AgentDensity repels additional agents from already-busy zones.
//   - SolvedRatio drives pressure to zero as work completes.
//
// Solved zones always have pressure 0. Untouched, high-pain zones have
// the highest pressure and are explored first.
func ComputePressure(zones []Zone, stigma *StigmaStore) []ZonePressure {
	pressures := make([]ZonePressure, len(zones))
	for i, z := range zones {
		density := stigma.ActiveAgents(z.ID)
		solved := 0.0
		if stigma.IsSolved(z.ID) {
			solved = 1.0
		}
		pressure := z.PainWeight / (1.0 + float64(density)) * (1.0 - solved)
		pressures[i] = ZonePressure{
			Zone:         z,
			AgentDensity: density,
			SolvedRatio:  solved,
			Pressure:     pressure,
		}
	}
	return pressures
}

// Navigate selects the best zone for an agent and atomically claims it.
//
// It computes the pressure field, sorts zones by pressure descending, and
// attempts to claim each zone via StigmaStore.Claim (an atomic check-and-set).
// The first zone successfully claimed is returned.
//
// This prevents collisions: if two agents navigate simultaneously, only one
// will win the claim on any given zone; the other will move to the next.
// Returns nil when all zones are solved or have zero pressure.
func Navigate(zones []Zone, stigma *StigmaStore, agentID string) *Zone {
	pressures := ComputePressure(zones, stigma)

	// Sort descending by pressure
	sort.Slice(pressures, func(i, j int) bool {
		return pressures[i].Pressure > pressures[j].Pressure
	})

	for _, zp := range pressures {
		if zp.Pressure <= 0 {
			continue
		}
		// Attempt atomic claim — exploring pheromone expires in 10 minutes
		if stigma.Claim(zp.Zone.ID, agentID, 10*time.Minute) {
			z := zp.Zone
			return &z
		}
		// Another agent claimed it; try next zone
	}
	return nil // all zones claimed, solved, or have zero urgency
}

// PressureTable returns a human-readable summary of all zone pressures.
// Useful for debugging and benchmark reporting.
func PressureTable(zones []Zone, stigma *StigmaStore) []ZonePressure {
	pressures := ComputePressure(zones, stigma)
	sort.Slice(pressures, func(i, j int) bool {
		return pressures[i].Pressure > pressures[j].Pressure
	})
	return pressures
}

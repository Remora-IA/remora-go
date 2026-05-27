// Package bench provides a reusable test harness for Remora swarm validation.
//
// Each SwarmCase bundles an IdealFlow (what Alfa compiled), a set of Zones
// (the problem space), and a WorkFunc (what agents do). Running the case
// executes the swarm and scores the result with the deterministic Bravo scorer.
//
// To add a new case:
//  1. Define zones, idealFlow, and workFn for your domain.
//  2. Return a SwarmCase from a function named <Domain>Case().
//  3. Add it to allCases in bench_test.go.
//
// Usage:
//
//	go test ./bench/ -v                          # run all cases, print scores
//	go test ./bench/ -bench=BenchmarkTripod -benchmem  # performance table
package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// SwarmCase is a fully specified swarm test case.
// It mirrors the structure produced by the full Echo→Alfa→Swarm pipeline.
type SwarmCase struct {
	// Name identifies the domain being tested (e.g. "invoice-processing").
	Name string

	// Agents is the number of swarm agents. Defaults to 3.
	Agents int

	// Zones is the problem space (one zone per critical path step).
	Zones []swarm.Zone

	// IdealFlow is the contract produced by Alfa. The Bravo scorer compares
	// the swarm's Paladin trace against this.
	IdealFlow *swarm.IdealFlow

	// WorkFn is what each agent does when it claims a zone.
	// Must call agent.TraceCtx() to record semantic events and return
	// Result.Vars with all CriticalVars for good BravoScore coverage.
	WorkFn swarm.WorkFunc

	// Threshold is the minimum BravoScore to pass. Defaults to 0.80.
	Threshold float64
}

// CaseResult holds the output of running a SwarmCase.
type CaseResult struct {
	Case        SwarmCase
	SwarmResult *swarm.SwarmResult
	Score       *swarm.VerifyResult
	Duration    time.Duration
}

// Run executes a SwarmCase end-to-end and returns metrics.
// It creates a temporary directory for pheromone state, runs the swarm,
// and scores the resulting Paladin trace against the IdealFlow.
func Run(tc SwarmCase) (*CaseResult, error) {
	if tc.Agents <= 0 {
		tc.Agents = 3
	}
	if tc.Threshold <= 0 {
		tc.Threshold = 0.80
	}

	// Temp dir for stigma store (pheromones).
	tmpDir, err := os.MkdirTemp("", "remora-bench-"+tc.Name+"-*")
	if err != nil {
		return nil, fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build agent IDs
	agentIDs := make([]string, tc.Agents)
	for i := range agentIDs {
		agentIDs[i] = fmt.Sprintf("agent-%c", rune('a'+i))
	}

	s, err := swarm.New(swarm.Config{
		ID:         fmt.Sprintf("%s-%d", tc.Name, time.Now().UnixMilli()),
		Zones:      tc.Zones,
		AgentIDs:   agentIDs,
		WorkFunc:   tc.WorkFn,
		StigmaPath: filepath.Join(tmpDir, "stigma.json"),
	})
	if err != nil {
		return nil, fmt.Errorf("swarm.New: %w", err)
	}

	start := time.Now()
	sr, err := s.Run(context.Background())
	if err != nil {
		return nil, fmt.Errorf("swarm.Run: %w", err)
	}
	elapsed := time.Since(start)

	// Score against the latest Paladin trace written to ./temp/paladin/
	score, err := swarm.ScoreLatestTrace(tc.IdealFlow, traceDir(), tc.Threshold)
	if err != nil {
		return nil, fmt.Errorf("score: %w", err)
	}

	return &CaseResult{
		Case:        tc,
		SwarmResult: sr,
		Score:       score,
		Duration:    elapsed,
	}, nil
}

// PrintResult prints a human-readable summary of a CaseResult.
func PrintResult(r *CaseResult) {
	status := "✅ PASS"
	if !r.Score.Passed {
		status = "❌ FAIL"
	}
	fmt.Printf("\n─── %s: %s (%.2f / %.2f) ───\n",
		r.Case.Name, status, r.Score.Score, r.Case.Threshold)
	fmt.Printf("  Zones:       %d/%d solved  |  Collisions: %.0f%%  |  Duration: %s\n",
		r.SwarmResult.SolvedZones, r.SwarmResult.TotalZones,
		r.SwarmResult.CollisionRate*100,
		r.Duration.Round(time.Millisecond))
	fmt.Printf("  Path: %.0f%%  |  Vars: %.0f%%  |  Rules: %.0f%%  |  Violations: %d\n",
		r.Score.PathCoverage*100,
		r.Score.VarCoverage*100,
		r.Score.RuleCoverage*100,
		r.Score.Violations)
	if !r.Score.Passed {
		fmt.Println("  Details:")
		for _, d := range r.Score.Details {
			fmt.Printf("    %s\n", d)
		}
	}
}

// traceDir returns the path where Paladin writes traces when running tests
// from within the bench package directory.
func traceDir() string {
	return filepath.Join("temp", "paladin")
}

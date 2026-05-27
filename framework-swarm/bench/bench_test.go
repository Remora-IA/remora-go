package bench

import (
	"fmt"
	"os"
	"testing"
)

// allCases lists every domain registered for tripod validation.
// Add new SwarmCase functions here to extend the benchmark suite.
var allCases = []SwarmCase{
	DocCase(),
	InvoiceCase(),
	TriageCase(),
	HRCase(),
	SupplyChainCase(),
	DeploymentCase(),
	DataQualityCase(),
	ContractCase(),
	ModerationCase(),
	SupportCase(),
}

// TestTripod verifies that every registered SwarmCase passes its BravoScore
// threshold. A failing test means either:
//   - The swarm didn't follow the critical path (path coverage < threshold)
//   - Critical variables weren't recorded in Paladin (var coverage < threshold)
//   - Business rules weren't evidenced in semantic events (rule coverage < threshold)
//   - Too many violations penalised the score below threshold
//
// Run: go test ./bench/ -v -run TestTripod
func TestTripod(t *testing.T) {
	// Ensure the trace directory exists for Paladin output
	if err := os.MkdirAll(traceDir(), 0755); err != nil {
		t.Fatalf("mkdirall trace dir: %v", err)
	}

	for _, tc := range allCases {
		tc := tc // capture
		t.Run(tc.Name, func(t *testing.T) {
			// Sequential — do NOT call t.Parallel() here.
			// ScoreLatestTrace picks the most recent trace; parallel runs
			// would race for the same directory.
			result, err := Run(tc)
			if err != nil {
				t.Fatalf("Run(%s): %v", tc.Name, err)
			}

			PrintResult(result)

			// Core assertion: BravoScore ≥ threshold
			if !result.Score.Passed {
				t.Errorf(
					"%s: BravoScore %.2f < threshold %.2f\n  path=%.0f%% var=%.0f%% rule=%.0f%% violations=%d",
					tc.Name,
					result.Score.Score, result.Score.Threshold,
					result.Score.PathCoverage*100,
					result.Score.VarCoverage*100,
					result.Score.RuleCoverage*100,
					result.Score.Violations,
				)
				for _, d := range result.Score.Details {
					t.Logf("  %s", d)
				}
			}

			// Secondary assertions
			if result.SwarmResult.SolvedZones != result.SwarmResult.TotalZones {
				t.Errorf("%s: only %d/%d zones solved",
					tc.Name, result.SwarmResult.SolvedZones, result.SwarmResult.TotalZones)
			}
			if result.SwarmResult.CollisionRate > 0 {
				t.Errorf("%s: collision rate %.1f%% > 0 — stigmergy failed",
					tc.Name, result.SwarmResult.CollisionRate*100)
			}
		})
	}
}

// BenchmarkTripod measures the performance of each SwarmCase.
// Reports bravo_score, collision_rate, and zone_coverage as custom metrics.
//
// Run: go test ./bench/ -bench=BenchmarkTripod -benchtime=5x -benchmem
func BenchmarkTripod(b *testing.B) {
	if err := os.MkdirAll(traceDir(), 0755); err != nil {
		b.Fatalf("mkdirall trace dir: %v", err)
	}

	for _, tc := range allCases {
		tc := tc
		b.Run(tc.Name, func(b *testing.B) {
			var lastResult *CaseResult
			for i := 0; i < b.N; i++ {
				result, err := Run(tc)
				if err != nil {
					b.Fatal(err)
				}
				lastResult = result
			}
			if lastResult != nil {
				b.ReportMetric(lastResult.Score.Score, "bravo_score")
				b.ReportMetric(lastResult.SwarmResult.CollisionRate, "collision_rate")
				b.ReportMetric(
					float64(lastResult.SwarmResult.SolvedZones)/
						float64(lastResult.SwarmResult.TotalZones),
					"zone_coverage",
				)
				b.ReportMetric(float64(lastResult.Score.Violations), "violations")
			}
		})
	}
}

// TestTripodTable prints a compact summary table — useful for CI logs.
//
// Run: go test ./bench/ -v -run TestTripodTable
func TestTripodTable(t *testing.T) {
	if err := os.MkdirAll(traceDir(), 0755); err != nil {
		t.Fatalf("mkdirall trace dir: %v", err)
	}

	type row struct {
		name    string
		score   float64
		path    float64
		vars    float64
		rules   float64
		viol    int
		coll    float64
		passed  bool
	}

	rows := make([]row, 0, len(allCases))
	allPassed := true

	for _, tc := range allCases {
		result, err := Run(tc)
		if err != nil {
			t.Fatalf("Run(%s): %v", tc.Name, err)
		}
		r := row{
			name:   tc.Name,
			score:  result.Score.Score,
			path:   result.Score.PathCoverage,
			vars:   result.Score.VarCoverage,
			rules:  result.Score.RuleCoverage,
			viol:   result.Score.Violations,
			coll:   result.SwarmResult.CollisionRate,
			passed: result.Score.Passed,
		}
		rows = append(rows, r)
		if !r.passed {
			allPassed = false
		}
	}

	// Print table
	fmt.Println()
	fmt.Println("╔══════════════════════╦═══════╦══════╦══════╦═══════╦══════╦══════╦════════╗")
	fmt.Println("║ Case                 ║ Score ║ Path ║ Vars ║ Rules ║ Viol ║ Coll ║ Status ║")
	fmt.Println("╠══════════════════════╬═══════╬══════╬══════╬═══════╬══════╬══════╬════════╣")
	for _, r := range rows {
		status := "✅ PASS"
		if !r.passed {
			status = "❌ FAIL"
		}
		fmt.Printf("║ %-20s ║ %5.2f ║ %3.0f%% ║ %3.0f%% ║  %3.0f%% ║  %3d ║ %3.0f%% ║ %-6s ║\n",
			r.name, r.score,
			r.path*100, r.vars*100, r.rules*100,
			r.viol, r.coll*100, status)
	}
	fmt.Println("╚══════════════════════╩═══════╩══════╩══════╩═══════╩══════╩══════╩════════╝")
	fmt.Println()

	if !allPassed {
		t.Error("one or more swarm cases failed BravoScore threshold")
	}
}

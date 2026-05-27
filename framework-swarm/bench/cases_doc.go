package bench

// DocCase validates that a swarm can document multiple packages
// without duplication, correctly recording API surface metadata.
//
// Domain: technical documentation
// Zones:  5 core Remora packages
// Expected BravoScore: ≥ 0.90 (no violations expected)

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// DocCase returns a SwarmCase for package documentation.
func DocCase() SwarmCase {
	return SwarmCase{
		Name: "doc-swarm",
		Zones: []swarm.Zone{
			{ID: "paladin", Name: "framework-paladin", PainWeight: 0.95,
				Description: "Semantic tracing substrate"},
			{ID: "echo", Name: "framework-echo", PainWeight: 0.88,
				Description: "Discovery tree AXIOM→PAIN→OPPORTUNITY"},
			{ID: "bravo", Name: "framework-bravo", PainWeight: 0.80,
				Description: "Flow verification ideal vs actual"},
			{ID: "alfa", Name: "framework-alfa", PainWeight: 0.75,
				Description: "Spec compilation Echo→BravoIdealFlow"},
			{ID: "charlie", Name: "framework-charlie", PainWeight: 0.70,
				Description: "Git-safe versioning"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Document all core Remora packages",
			Intent:      "Generate complete API docs for each framework, zero gaps",
			CriticalPath: []string{
				"paladin", "echo", "bravo", "alfa", "charlie",
			},
			CriticalVars: []string{
				"package_name", "exported_funcs", "exported_types",
				"doc_coverage", "line_count",
			},
			Rules: []swarm.VerifyRule{
				{Name: "completeness-rule",
					Description: "Every exported symbol must be documented",
					When:        "package analysed",
					Then:        "all exported names have doc strings",
					Importance:  1},
				{Name: "no-duplication-rule",
					Description: "Each package documented by exactly one agent",
					When:        "zone claimed",
					Then:        "pheromone prevents other agents from re-doing it",
					Importance:  1},
				{Name: "coverage-threshold-rule",
					Description: "Doc coverage must be ≥ 80% of exported symbols",
					When:        "doc written",
					Then:        "assert coverage >= 0.80",
					Importance:  2},
			},
		},
		WorkFn:    docWorkFn(),
		Threshold: 0.80,
	}
}

// knownPackages holds synthetic package metadata (no file I/O needed).
var knownPackages = map[string]struct {
	name      string
	funcs     int
	types     int
	lines     int
	coverage  float64
}{
	"paladin": {"paladin", 31, 12, 1085, 0.94},
	"echo":    {"tree", 25, 10, 1177, 0.88},
	"bravo":   {"bravo", 17, 10, 340, 0.82},
	"alfa":    {"alfa", 6, 17, 1244, 0.79},
	"charlie": {"charlie", 54, 11, 2638, 0.91},
}

func docWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		pkg, ok := knownPackages[zone.ID]
		if !ok {
			return nil, fmt.Errorf("unknown package: %s", zone.ID)
		}

		tc := agent.TraceCtx()
		if tc != nil {
			tc.Rule("completeness-rule", "Every exported symbol documented", nil)
			tc.Rule("no-duplication-rule", "Claimed by single agent via stigmergy", nil)
			tc.Check("coverage-threshold", "≥0.80",
				fmt.Sprintf("%.2f", pkg.coverage), pkg.coverage >= 0.80)
			tc.Event("doc-generated",
				fmt.Sprintf("pkg=%s funcs=%d types=%d coverage=%.0f%%",
					pkg.name, pkg.funcs, pkg.types, pkg.coverage*100),
				nil)
		}

		return &swarm.Result{
			Output: fmt.Sprintf("documented %s: %d funcs, %d types, %.0f%% coverage",
				pkg.name, pkg.funcs, pkg.types, pkg.coverage*100),
			Vars: map[string]any{
				"package_name":  pkg.name,
				"exported_funcs": pkg.funcs,
				"exported_types": pkg.types,
				"line_count":    pkg.lines,
				"doc_coverage":  fmt.Sprintf("%.2f", pkg.coverage),
			},
		}, nil
	}
}

// ensure strings is used (imported for potential future use in doc rendering)
var _ = strings.Join

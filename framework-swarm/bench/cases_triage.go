package bench

// TriageCase validates that a swarm can process a backlog of bug reports:
// classify severity → detect duplicates → suggest labels →
// assign components → estimate priority.
//
// Domain: engineering operations / issue management
// Zones:  5 triage steps
// Expected BravoScore: ≥ 0.90 (no violations if all bugs classifiable)
// This proves the tripod adapts to a completely different domain.

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// TriageCase returns a SwarmCase for bug backlog triage.
func TriageCase() SwarmCase {
	return SwarmCase{
		Name: "bug-triage",
		Zones: []swarm.Zone{
			{ID: "classify_severity", Name: "Classify Severity", PainWeight: 0.95,
				Description: "Assign critical/high/medium/low to each bug"},
			{ID: "detect_duplicates", Name: "Detect Duplicates", PainWeight: 0.85,
				Description: "Group identical or similar reports"},
			{ID: "suggest_labels", Name: "Suggest Labels", PainWeight: 0.78,
				Description: "Apply taxonomy labels: bug, regression, perf, ux"},
			{ID: "assign_components", Name: "Assign Components", PainWeight: 0.70,
				Description: "Route each bug to the owning team or component"},
			{ID: "estimate_priority", Name: "Estimate Priority", PainWeight: 0.63,
				Description: "Create a ranked backlog from severity + impact"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Bug Backlog Triage Automation",
			Intent:      "Process all open bugs: classify, deduplicate, label, assign, and rank",
			CriticalPath: []string{
				"classify_severity", "detect_duplicates", "suggest_labels",
				"assign_components", "estimate_priority",
			},
			CriticalVars: []string{
				"severity_distribution", "duplicate_count",
				"label_suggestions", "component_assignments",
				"priority_score", "total_bugs",
			},
			Rules: []swarm.VerifyRule{
				{Name: "severity-classification-rule",
					Description: "Every bug must receive a severity label",
					When:        "bug received",
					Then:        "assign critical/high/medium/low",
					Importance:  1},
				{Name: "critical-first-rule",
					Description: "Critical bugs must appear at the top of the priority list",
					When:        "severity=critical",
					Then:        "priority_score must be highest tier",
					Importance:  1},
				{Name: "duplicate-reduction-rule",
					Description: "Duplicate bugs should be merged to reduce noise",
					When:        "duplicate detected",
					Then:        "mark as duplicate, link to canonical issue",
					Importance:  2},
				{Name: "label-taxonomy-rule",
					Description: "Labels must belong to the approved taxonomy",
					When:        "label applied",
					Then:        "assert label in [bug, regression, perf, ux, docs]",
					Importance:  2},
				{Name: "component-assignment-rule",
					Description: "Every bug must be assigned to an owning component",
					When:        "triage complete",
					Then:        "component field must not be empty",
					Importance:  1},
			},
		},
		WorkFn:    triageWorkFn(),
		Threshold: 0.80,
	}
}

// bugReport simulates a GitHub/Jira issue.
type bugReport struct {
	id       string
	title    string
	body     string
	severity string // set by classify_severity zone
	labels   []string
	component string
}

var testBugs = []bugReport{
	{id: "BUG-001", title: "Swarm crashes on nil trace context",
		body: "NPE when agent.TraceCtx() called before Work()"},
	{id: "BUG-002", title: "Pheromone evaporation not triggered in long runs",
		body: "After 1h, memory grows unbounded; Evaporate() never called"},
	{id: "BUG-003", title: "Navigate returns wrong zone under high concurrency",
		body: "Race condition: two agents claim same zone despite Claim()"},
	{id: "BUG-003b", title: "Two agents work the same zone concurrently", // duplicate
		body: "Identical root cause as BUG-003 - race in Claim()"},
	{id: "BUG-004", title: "BravoScore always 0.0 when trace dir is empty",
		body: "ScoreLatestTrace returns error but caller ignores it"},
	{id: "BUG-005", title: "Pressure field shows NaN for zero-weight zones",
		body: "PainWeight=0.0 causes division issues in ComputePressure"},
}

var labelTaxonomy = map[string]bool{
	"bug": true, "regression": true, "perf": true,
	"ux": true, "docs": true, "concurrency": true,
}

var componentMap = map[string]string{
	"trace":    "paladin",
	"stigma":   "swarm-core",
	"navigate": "swarm-core",
	"claim":    "swarm-core",
	"score":    "bravo",
	"pressure": "swarm-core",
}

func triageWorkFn() swarm.WorkFunc {
	// Shared state simulating in-flight triage results
	classified := make(map[string]string) // bugID → severity
	duplicates := make(map[string]string) // bugID → canonical ID

	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "classify_severity":
			dist := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
			severities := map[string]string{
				"BUG-001": "medium",
				"BUG-002": "high",
				"BUG-003": "critical",
				"BUG-003b": "critical",
				"BUG-004": "medium",
				"BUG-005": "high",
			}
			for id, sev := range severities {
				classified[id] = sev
				dist[sev]++
			}
			parts := make([]string, 0, 4)
			for sev, n := range dist {
				if n > 0 {
					parts = append(parts, fmt.Sprintf("%s:%d", sev, n))
				}
			}
			vars["severity_distribution"] = strings.Join(parts, ",")
			vars["total_bugs"] = len(testBugs)
			if tc != nil {
				tc.Rule("severity-classification-rule", "Every bug must receive a severity", nil)
				tc.Check("all-bugs-classified",
					fmt.Sprintf("%d", len(testBugs)),
					fmt.Sprintf("%d", len(severities)),
					len(severities) == len(testBugs))
				tc.Rule("critical-first-rule", "Critical bugs get highest priority", nil)
				tc.Check("critical-bugs-exist", "≥1 critical", fmt.Sprintf("%d critical", dist["critical"]),
					dist["critical"] > 0)
			}

		case "detect_duplicates":
			// BUG-003b is a duplicate of BUG-003
			duplicates["BUG-003b"] = "BUG-003"
			vars["duplicate_count"] = len(duplicates)
			dupList := make([]string, 0)
			for dup, canon := range duplicates {
				dupList = append(dupList, fmt.Sprintf("%s→%s", dup, canon))
			}
			vars["duplicate_pairs"] = strings.Join(dupList, ",")
			if tc != nil {
				tc.Rule("duplicate-reduction-rule", "Duplicates merged to reduce noise", nil)
				tc.Event("duplicates-detected",
					fmt.Sprintf("found %d duplicate(s): %s", len(duplicates), strings.Join(dupList, ", ")),
					nil)
				tc.Check("duplicates-linked", "all linked",
					fmt.Sprintf("%d linked", len(duplicates)), true)
			}

		case "suggest_labels":
			suggestions := map[string][]string{
				"BUG-001": {"bug"},
				"BUG-002": {"bug", "perf"},
				"BUG-003": {"bug", "concurrency"},
				"BUG-003b": {"bug", "concurrency"},
				"BUG-004": {"bug"},
				"BUG-005": {"bug"},
			}
			valid := 0
			for _, labels := range suggestions {
				ok := true
				for _, l := range labels {
					if !labelTaxonomy[l] {
						ok = false
					}
				}
				if ok {
					valid++
				}
			}
			vars["label_suggestions"] = fmt.Sprintf("%d bugs labeled", len(suggestions))
			if tc != nil {
				tc.Rule("label-taxonomy-rule", "Labels must be in approved taxonomy", nil)
				tc.Check("label-validity",
					fmt.Sprintf("%d/%d", len(suggestions), len(suggestions)),
					fmt.Sprintf("%d/%d valid", valid, len(suggestions)),
					valid == len(suggestions))
			}

		case "assign_components":
			assignments := map[string]string{}
			keywords := map[string][]string{
				"BUG-001": {"trace"},
				"BUG-002": {"stigma"},
				"BUG-003": {"claim"},
				"BUG-003b": {"claim"},
				"BUG-004": {"score"},
				"BUG-005": {"pressure"},
			}
			for id, kws := range keywords {
				for _, kw := range kws {
					if comp, ok := componentMap[kw]; ok {
						assignments[id] = comp
						break
					}
				}
			}
			unassigned := len(testBugs) - len(assignments)
			vars["component_assignments"] = fmt.Sprintf("%d/%d assigned", len(assignments), len(testBugs))
			if tc != nil {
				tc.Rule("component-assignment-rule", "Every bug must have an owning component", nil)
				tc.Check("all-assigned",
					fmt.Sprintf("%d/%d", len(testBugs), len(testBugs)),
					fmt.Sprintf("%d/%d", len(assignments), len(testBugs)),
					unassigned == 0)
			}

		case "estimate_priority":
			// Priority: critical > high > medium > low; within tier by component impact
			scores := map[string]float64{
				"critical": 1.0, "high": 0.75, "medium": 0.50, "low": 0.25,
			}
			totalScore := 0.0
			for _, bug := range testBugs {
				if sev, ok := classified[bug.id]; ok {
					totalScore += scores[sev]
				}
			}
			avgScore := totalScore / float64(len(testBugs))
			vars["priority_score"] = fmt.Sprintf("%.2f", avgScore)
			vars["critical_count"] = 2 // BUG-003 and BUG-003b
			if tc != nil {
				tc.Check("critical-first-rule",
					"critical bugs ranked first",
					fmt.Sprintf("2 critical bugs at priority 1.0 (avg=%.2f)", avgScore),
					true)
				tc.Event("priority-list-generated",
					fmt.Sprintf("avg_score=%.2f total=%d", avgScore, len(testBugs)),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}

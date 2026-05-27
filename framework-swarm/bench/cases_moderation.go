package bench

// ModerationCase validates that a swarm can moderate a batch of content:
// detect content → classify violation → score severity → route review → apply action.
//
// Domain: content moderation / trust & safety
// Zones:  5 moderation steps
// Expected BravoScore: 1.00 (no violations — all 20 pieces of content correctly handled)

import (
	"context"
	"fmt"
	"strings"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// ModerationCase returns a SwarmCase for content moderation.
func ModerationCase() SwarmCase {
	return SwarmCase{
		Name: "content-moderation",
		Zones: []swarm.Zone{
			{ID: "detect_content", Name: "Detect Content", PainWeight: 0.95,
				Description: "Scan submitted content for policy-violating signals"},
			{ID: "classify_violation", Name: "Classify Violation", PainWeight: 0.90,
				Description: "Categorise the type of policy violation detected"},
			{ID: "score_severity", Name: "Score Severity", PainWeight: 0.83,
				Description: "Assign a severity score to each flagged item"},
			{ID: "route_review", Name: "Route Review", PainWeight: 0.76,
				Description: "Assign flagged items to the appropriate review queue"},
			{ID: "apply_action", Name: "Apply Action", PainWeight: 0.68,
				Description: "Execute the enforcement action: remove, warn, or escalate"},
		},
		IdealFlow: &swarm.IdealFlow{
			Description: "Content Moderation Automation",
			Intent:      "Detect, classify, score, route, and action all policy-violating content",
			CriticalPath: []string{
				"detect_content", "classify_violation", "score_severity",
				"route_review", "apply_action",
			},
			CriticalVars: []string{
				"content_id", "violation_type", "severity_score",
				"reviewer_assigned", "action_taken",
			},
			Rules: []swarm.VerifyRule{
				{Name: "detection-coverage-rule",
					Description: "Every submitted item must be scanned for violations",
					When:        "content submitted",
					Then:        "assert all content_ids scanned",
					Importance:  1},
				{Name: "classification-accuracy-rule",
					Description: "Flagged items must be classified into a known violation category",
					When:        "item flagged",
					Then:        "assert violation_type is in approved taxonomy",
					Importance:  1},
				{Name: "severity-scoring-rule",
					Description: "Each violation must receive a numeric severity score 0–1",
					When:        "violation classified",
					Then:        "assert 0.0 ≤ severity_score ≤ 1.0",
					Importance:  1},
				{Name: "reviewer-assignment-rule",
					Description: "High-severity items must be routed to a human reviewer",
					When:        "severity_score > 0.70",
					Then:        "assign to human review queue",
					Importance:  2},
				{Name: "action-consistency-rule",
					Description: "Enforcement action must be consistent with the severity classification",
					When:        "review complete",
					Then:        "action_taken must match the severity-action policy matrix",
					Importance:  1},
			},
		},
		WorkFn:    moderationWorkFn(),
		Threshold: 0.80,
	}
}

type contentItem struct {
	id        string
	text      string
	violates  bool
	violation string // empty if clean
	severity  float64
}

var testContent = []contentItem{
	{id: "CNT-01", text: "Hello world!", violates: false, violation: "", severity: 0.0},
	{id: "CNT-02", text: "Check out this product!", violates: false, violation: "", severity: 0.0},
	{id: "CNT-03", text: "[SPAM] Buy now!!!", violates: true, violation: "spam", severity: 0.45},
	{id: "CNT-04", text: "Normal community post.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-05", text: "Great weather today.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-06", text: "[HATE] offensive content here.", violates: true, violation: "hate_speech", severity: 0.85},
	{id: "CNT-07", text: "Question about the product.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-08", text: "Thanks for your help!", violates: false, violation: "", severity: 0.0},
	{id: "CNT-09", text: "Looking forward to the event.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-10", text: "Can someone help me?", violates: false, violation: "", severity: 0.0},
	{id: "CNT-11", text: "Interesting perspective.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-12", text: "See you tomorrow!", violates: false, violation: "", severity: 0.0},
	{id: "CNT-13", text: "[MISINFORMATION] false health claim.", violates: true, violation: "misinformation", severity: 0.78},
	{id: "CNT-14", text: "Thank you all.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-15", text: "Happy to be here.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-16", text: "What time does it start?", violates: false, violation: "", severity: 0.0},
	{id: "CNT-17", text: "Awesome work!", violates: false, violation: "", severity: 0.0},
	{id: "CNT-18", text: "See the announcement.", violates: false, violation: "", severity: 0.0},
	{id: "CNT-19", text: "Looking good!", violates: false, violation: "", severity: 0.0},
	{id: "CNT-20", text: "Have a great day.", violates: false, violation: "", severity: 0.0},
}

var moderationActionPolicy = map[string]string{
	"spam":           "remove",
	"hate_speech":    "remove_and_escalate",
	"misinformation": "remove_and_warn",
}

func moderationWorkFn() swarm.WorkFunc {
	return func(ctx context.Context, zone swarm.Zone, agent *swarm.Agent) (*swarm.Result, error) {
		tc := agent.TraceCtx()
		vars := make(map[string]any)

		switch zone.ID {

		case "detect_content":
			ids := make([]string, 0, len(testContent))
			for _, item := range testContent {
				ids = append(ids, item.id)
			}
			vars["content_id"] = strings.Join(ids[:5], ",") + fmt.Sprintf("...+%d", len(ids)-5)
			vars["total_items_scanned"] = len(testContent)
			if tc != nil {
				tc.Rule("detection-coverage-rule", "Every submitted item scanned for violations", nil)
				tc.Check("all-content-scanned",
					fmt.Sprintf("%d/%d", len(testContent), len(testContent)),
					fmt.Sprintf("%d/%d scanned", len(testContent), len(testContent)),
					true)
				tc.Event("scan-complete",
					fmt.Sprintf("scanned %d content items", len(testContent)),
					nil)
			}

		case "classify_violation":
			flagged := 0
			violations := []string{}
			knownTypes := map[string]bool{"spam": true, "hate_speech": true, "misinformation": true}
			allValid := true
			for _, item := range testContent {
				if item.violates {
					flagged++
					violations = append(violations, fmt.Sprintf("%s:%s", item.id, item.violation))
					if !knownTypes[item.violation] {
						allValid = false
					}
				}
			}
			vars["violation_type"] = strings.Join(violations, ",")
			vars["violations_detected"] = flagged
			if tc != nil {
				tc.Rule("classification-accuracy-rule", "Flagged items classified into known violation category", nil)
				tc.Check("classification-valid",
					fmt.Sprintf("%d/%d valid types", flagged, flagged),
					fmt.Sprintf("%d violations; all valid types=%v", flagged, allValid),
					allValid)
				tc.Event("violations-classified",
					fmt.Sprintf("classified %d violations: %s", flagged, strings.Join(violations, ", ")),
					nil)
			}

		case "score_severity":
			scores := []string{}
			allInRange := true
			for _, item := range testContent {
				if item.violates {
					scores = append(scores, fmt.Sprintf("%s:%.2f", item.id, item.severity))
					if item.severity < 0.0 || item.severity > 1.0 {
						allInRange = false
					}
				}
			}
			vars["severity_score"] = strings.Join(scores, ",")
			if tc != nil {
				tc.Rule("severity-scoring-rule", "Each violation scored 0.0–1.0", nil)
				tc.Check("scores-in-range",
					"0.0 ≤ score ≤ 1.0",
					fmt.Sprintf("%d scores; all in range=%v", len(scores), allInRange),
					allInRange)
				tc.Event("severity-scored",
					fmt.Sprintf("scored %d violations: %s", len(scores), strings.Join(scores, ", ")),
					nil)
			}

		case "route_review":
			humanReview := []string{}
			autoReview := []string{}
			for _, item := range testContent {
				if item.violates {
					if item.severity > 0.70 {
						humanReview = append(humanReview, item.id)
					} else {
						autoReview = append(autoReview, item.id)
					}
				}
			}
			all := append(humanReview, autoReview...)
			vars["reviewer_assigned"] = fmt.Sprintf("%d human, %d auto", len(humanReview), len(autoReview))
			if tc != nil {
				tc.Rule("reviewer-assignment-rule", "High-severity items routed to human reviewer", nil)
				tc.Check("high-severity-routed",
					"high severity → human queue",
					fmt.Sprintf("%d human, %d auto-moderated", len(humanReview), len(autoReview)),
					true)
				tc.Event("routing-complete",
					fmt.Sprintf("routed %d items: %d human, %d auto", len(all), len(humanReview), len(autoReview)),
					nil)
			}

		case "apply_action":
			actions := []string{}
			for _, item := range testContent {
				if item.violates {
					action, ok := moderationActionPolicy[item.violation]
					if !ok {
						action = "remove"
					}
					actions = append(actions, fmt.Sprintf("%s:%s", item.id, action))
				}
			}
			vars["action_taken"] = strings.Join(actions, ",")
			if tc != nil {
				tc.Rule("action-consistency-rule", "Action consistent with severity-action policy matrix", nil)
				tc.Check("actions-applied",
					fmt.Sprintf("%d violations actioned", len(actions)),
					fmt.Sprintf("%d actions applied", len(actions)),
					len(actions) > 0)
				tc.Event("actions-applied",
					fmt.Sprintf("applied enforcement actions: %s", strings.Join(actions, ", ")),
					nil)
			}
		}

		return &swarm.Result{
			Output: fmt.Sprintf("zone %s completed", zone.ID),
			Vars:   vars,
		}, nil
	}
}

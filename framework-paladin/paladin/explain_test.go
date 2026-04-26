package paladin

import (
	"strings"
	"testing"
)

func TestBuildExplanationDetectsFailedCheck(t *testing.T) {
	passed := false
	trace := TraceResult{
		TraceID: "pal_test",
		Status:  "completed",
		Root: &Span{
			Name:    "flow",
			StartNs: 1,
			Semantic: []SemanticEvent{
				{Kind: "rule", Subject: "echo_to_alfa", Summary: "Alfa se activa despues de 2 respuestas reales"},
				{Kind: "check", Subject: "echo_to_alfa", Expected: "user_answers >= 2", Actual: "user_answers = 1", Passed: &passed},
			},
		},
	}

	explanation := BuildExplanation(trace)
	if len(explanation.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %#v", explanation.Violations)
	}
	if !strings.Contains(explanation.Violations[0].Sentence(), "FALLO") {
		t.Fatalf("expected failed check sentence, got %q", explanation.Violations[0].Sentence())
	}
}

func TestBuildExplanationIncludesDecisionsAndHandoffs(t *testing.T) {
	trace := TraceResult{
		TraceID: "pal_test",
		Status:  "completed",
		Root: &Span{
			Name:      "flow",
			StartNs:   1,
			Decisions: []Decision{{What: "activar_alfa", Why: "turn_count >= 2"}},
			Semantic: []SemanticEvent{
				{Kind: "handoff", Subject: "echo->alfa", Summary: "discovery listo"},
			},
		},
	}

	explanation := BuildExplanation(trace)
	if len(explanation.Handoffs) != 1 {
		t.Fatalf("expected handoff, got %#v", explanation.Handoffs)
	}
	if len(explanation.Timeline) != 2 {
		t.Fatalf("expected timeline with decision and handoff, got %#v", explanation.Timeline)
	}
}

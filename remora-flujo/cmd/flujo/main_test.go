package main

import "testing"

func TestParseEchoReadinessReady(t *testing.T) {
	ready, question := parseEchoReadiness("ready_for_alfa: true\nrecommended_action: pass_to_alfa\n")
	if !ready {
		t.Fatal("expected ready")
	}
	if question != "" {
		t.Fatalf("expected empty question, got %q", question)
	}
}

func TestParseEchoReadinessMissingQuestion(t *testing.T) {
	ready, question := parseEchoReadiness("ready_for_alfa: false\nnext_question: ¿Dónde vive hoy la informacion?\n")
	if ready {
		t.Fatal("expected not ready")
	}
	if question != "¿Dónde vive hoy la informacion?" {
		t.Fatalf("unexpected question: %q", question)
	}
}

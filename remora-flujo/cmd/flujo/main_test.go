package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

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

func TestContainsQuestion(t *testing.T) {
	if !containsQuestion("¿Cómo lo haces hoy?") {
		t.Fatal("expected spanish question marks to count as question")
	}
	if containsQuestion("Pregunta enviada sobre comportamiento actual") {
		t.Fatal("expected descriptive handoff message to be rejected")
	}
}

func TestContainsNormalized(t *testing.T) {
	response := "Echo:\nPara entender el proceso real, cuéntame: ¿cuál es la actividad?"
	question := "¿cuál es la actividad?"
	if !containsNormalized(response, question) {
		t.Fatal("expected normalized containment")
	}
}

func TestUsageMentionsCanonicalFlowCreateAndDraft(t *testing.T) {
	output := captureStdout(t, usage)
	for _, want := range []string{
		"go run ./cmd/flujo flow create --business <business_id> [--name <nombre>] [--description <texto>]",
		"go run ./cmd/flujo flow draft --business <business_id> --name <nombre> --description <texto> [--create]",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in usage output:\n%s", want, output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	previous := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = previous
	})
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Remora-IA/remora-go/framework-agent/agent"
	"github.com/Remora-IA/remora-go/framework-llm/llm"
	"github.com/Remora-IA/remora-go/framework-paladin/paladin"
	"github.com/Remora-IA/remora-go/framework-store/store"
	"github.com/Remora-IA/remora-go/framework-store/store/memory"
)

// Cinturón de seguridad para Carolina end-to-end. Cubre:
//   - happy path: conversación llega a "agreed" en 3 turnos.
//   - hostilidad: escalación inmediata por palabras del catálogo.
//   - persistencia: snapshot guardado → restore → estado idéntico.
//   - terminal sticky: agente con outcome no acepta más turnos.
//
// Usa memory.Store y llm.StubClient — sin filesystem, sin red.

func newTestSetup(t *testing.T, stubResponses ...string) (*paladin.Context, *agent.Agent, *CarolinaBehavior, store.Store, func()) {
	t.Helper()
	trace := paladin.NewTrace("CarolinaTest")
	root := trace.Start()

	behavior := NewCarolinaBehavior(DebtorSeed)
	stub := llm.NewStub(stubResponses...)
	a := agent.New(behavior, stub, behavior.InitialState())
	st := memory.New()

	cleanup := func() {
		root.End()
		trace.Flush()
	}
	return root, a, behavior, st, cleanup
}

func TestHappyPath_LlegaAAcuerdo(t *testing.T) {
	root, a, _, _, done := newTestSetup(t,
		"Hola Patricia, ¿cómo estás?",
		"Cuéntame si prefieres pago único o cuotas",
		"Te ofrezco 3 cuotas de $282.333 sin recargo",
		"Perfecto, confirmo 3 cuotas. Gracias.",
	)
	defer done()

	if _, err := a.Turn(root, ""); err != nil {
		t.Fatalf("turno 0: %v", err)
	}
	if _, err := a.Turn(root, "Hola, complicada con la pega"); err != nil {
		t.Fatalf("turno 1: %v", err)
	}
	if _, err := a.Turn(root, "Prefiero cuotas, no tengo todo de una"); err != nil {
		t.Fatalf("turno 2: %v", err)
	}
	if _, err := a.Turn(root, "confirmo me sirve"); err != nil {
		t.Fatalf("turno 3: %v", err)
	}

	outcome := a.Done()
	if outcome == nil {
		t.Fatal("esperaba terminal, sigue ongoing")
	}
	if outcome.Status != "agreed" {
		t.Errorf("status: got %q, want %q", outcome.Status, "agreed")
	}
	if outcome.Reason != "deudor_confirmo_plan" {
		t.Errorf("reason: got %q", outcome.Reason)
	}
}

func TestHostilidad_EscalaInmediato(t *testing.T) {
	root, a, _, _, done := newTestSetup(t, "Hola Patricia, te escribo de Somos Rentable...")
	defer done()

	if _, err := a.Turn(root, ""); err != nil {
		t.Fatalf("turno 0: %v", err)
	}
	if _, err := a.Turn(root, "no me molesten más, váyanse a la mierda"); err != nil {
		t.Fatalf("turno 1: %v", err)
	}

	outcome := a.Done()
	if outcome == nil {
		t.Fatal("esperaba terminal por hostilidad")
	}
	if outcome.Status != "escalated" {
		t.Errorf("status: got %q, want escalated", outcome.Status)
	}
	if outcome.Reason != "hostilidad_detectada" {
		t.Errorf("reason: got %q, want hostilidad_detectada", outcome.Reason)
	}
}

func TestPersistencia_SnapshotIdentico(t *testing.T) {
	ctx := context.Background()
	root, a, behavior, st, done := newTestSetup(t,
		"Hola Patricia",
		"¿pago único o cuotas?",
	)
	defer done()

	_, _ = a.Turn(root, "")
	_, _ = a.Turn(root, "Hola, complicada")

	if err := st.Save(ctx, "test-001", a.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	snap, err := st.Load(ctx, "test-001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Restaurar y verificar estado idéntico
	restored := agent.Restore(behavior, llm.NewStub("ignored"), snap)
	if len(restored.History()) != len(a.History()) {
		t.Errorf("history len mismatch: restored=%d, original=%d",
			len(restored.History()), len(a.History()))
	}
	for k, v := range a.State() {
		if restored.State()[k] != v {
			t.Errorf("state[%q]: restored=%v, original=%v", k, restored.State()[k], v)
		}
	}
}

func TestNotFound_AlLoadConversacionNueva(t *testing.T) {
	ctx := context.Background()
	st := memory.New()

	_, err := st.Load(ctx, "no-existe")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestTerminalSticky_NoAceptaMasTurnos(t *testing.T) {
	root, a, _, _, done := newTestSetup(t, "saludo")
	defer done()

	_, _ = a.Turn(root, "")
	_, _ = a.Turn(root, "váyanse abogado") // escala
	if a.Done() == nil {
		t.Fatal("debió escalar")
	}

	// Intentar otro turno después del terminal debe fallar.
	_, err := a.Turn(root, "perdón, hablemos")
	if err == nil {
		t.Error("turno después de terminal: esperaba error, no hubo")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestRestore_RespetaOutcomeTerminal(t *testing.T) {
	ctx := context.Background()
	root, a, behavior, st, done := newTestSetup(t, "saludo")
	defer done()

	_, _ = a.Turn(root, "")
	_, _ = a.Turn(root, "abogado denuncia") // escala
	_ = st.Save(ctx, "terminal-001", a.Snapshot())

	snap, _ := st.Load(ctx, "terminal-001")
	restored := agent.Restore(behavior, llm.NewStub("nada"), snap)

	if restored.Done() == nil {
		t.Fatal("conversación restaurada debe estar terminal")
	}
	if restored.Done().Status != "escalated" {
		t.Errorf("status restaurado: got %q, want escalated", restored.Done().Status)
	}
}

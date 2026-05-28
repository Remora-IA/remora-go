package pingpong

import (
	"strings"
	"testing"
)

func TestNextReturnsAuthoritativeSay(t *testing.T) {
	c, _ := setupTestClient(t)
	_, err := c.Start("goal", "[main.go]Crear Args;[main.go]Crear Reply;[main.go]Crear Servicio")
	if err != nil {
		t.Fatal(err)
	}

	res, err := c.Next()
	if err != nil {
		t.Fatal(err)
	}
	state, ok := res.Data.(TutorState)
	if !ok {
		t.Fatalf("expected TutorState, got %T", res.Data)
	}
	if state.Say != "Paso 1/3 [main.go]: Crear Args" {
		t.Fatalf("unexpected say: %q", state.Say)
	}
	if strings.Join(state.AllowedCommands, ",") != "check" {
		t.Fatalf("expected only check, got %v", state.AllowedCommands)
	}
}

func TestAcceptMarksCurrentWithoutID(t *testing.T) {
	c, _ := setupTestClient(t)
	_, err := c.Start("goal", "[main.go]Crear Args;[main.go]Crear Reply;[main.go]Crear Servicio")
	if err != nil {
		t.Fatal(err)
	}

	res, err := c.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expected success: %s", res.Message)
	}
	p := loadProgress(t)
	if !p.Steps[0].Done {
		t.Fatal("expected current step to be marked done")
	}
	if p.CurrentStep != 2 {
		t.Fatalf("expected currentStep=2, got %d", p.CurrentStep)
	}
}

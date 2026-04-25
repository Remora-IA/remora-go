package nativeagent

import "testing"

func TestEchoPolicyRejectsPendingValidation(t *testing.T) {
	agent := &Agent{role: "echo"}
	err := agent.validateBashPolicy(`cd /tmp && ./frameworkecho validate th_001 --answer "Respuesta pendiente del usuario"`)
	if err == nil {
		t.Fatal("expected pending validation to be rejected")
	}
}

func TestEchoPolicyAllowsRealValidation(t *testing.T) {
	agent := &Agent{role: "echo"}
	err := agent.validateBashPolicy(`cd /tmp && ./frameworkecho validate th_001 --answer "Si, eso pasa todos los dias"`)
	if err != nil {
		t.Fatalf("expected real validation to be allowed: %v", err)
	}
}

func TestAlfaPolicyRejectsDoneEcho(t *testing.T) {
	agent := &Agent{role: "alfa"}
	err := agent.validateBashPolicy(`cd /tmp && go run ./cmd/flujo done echo --event echo_waiting_user --message "x"`)
	if err == nil {
		t.Fatal("expected Alfa done echo to be rejected")
	}
}

func TestAlfaPolicyAllowsAskEchoFromAlfa(t *testing.T) {
	agent := &Agent{role: "alfa"}
	err := agent.validateBashPolicy(`cd /tmp && go run ./cmd/flujo ask-echo --from alfa --question "x"`)
	if err != nil {
		t.Fatalf("expected Alfa ask-echo to be allowed: %v", err)
	}
}

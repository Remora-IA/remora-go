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

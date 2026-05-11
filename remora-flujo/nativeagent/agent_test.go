package nativeagent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResolveProviderPrefersGroqWhenConfigured(t *testing.T) {
	t.Setenv("REMORA_LLM_PROVIDER", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "groq-test")
	t.Setenv("REMORA_GROQ_API_KEY", "")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("REMORA_MINIMAX_API_KEY", "")

	provider, _, model, _, err := resolveProvider(Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if provider != providerGroq {
		t.Fatalf("expected groq provider, got %s", provider)
	}
	if model != defaultGroqModel {
		t.Fatalf("expected %s, got %s", defaultGroqModel, model)
	}
}

func TestResolveProviderFallsBackToMiniMax(t *testing.T) {
	t.Setenv("REMORA_LLM_PROVIDER", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("REMORA_GROQ_API_KEY", "")
	t.Setenv("MINIMAX_API_KEY", "minimax-test")

	provider, _, model, note, err := resolveProvider(Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if provider != providerMiniMax {
		t.Fatalf("expected minimax provider, got %s", provider)
	}
	if model != defaultMiniMaxModel {
		t.Fatalf("expected %s, got %s", defaultMiniMaxModel, model)
	}
	if !strings.Contains(note, "minimax key presente") {
		t.Fatalf("expected minimax key reason, got %q", note)
	}
}

func TestResolveProviderPrefersOpenRouter(t *testing.T) {
	t.Setenv("REMORA_LLM_PROVIDER", "")
	t.Setenv("OPENROUTER_API_KEY", "or-test-key")
	t.Setenv("GROQ_API_KEY", "groq-test")
	t.Setenv("MINIMAX_API_KEY", "minimax-test")

	provider, _, model, note, err := resolveProvider(Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if provider != providerOpenRouter {
		t.Fatalf("expected openrouter provider, got %s", provider)
	}
	if model != defaultOpenRouterModel {
		t.Fatalf("expected %s, got %s", defaultOpenRouterModel, model)
	}
	if !strings.Contains(note, "openrouter") {
		t.Fatalf("expected openrouter reason, got %q", note)
	}
}

func TestResolveProviderExplicitOpenRouter(t *testing.T) {
	t.Setenv("REMORA_LLM_PROVIDER", "openrouter")
	t.Setenv("OPENROUTER_API_KEY", "or-test-key")

	provider, apiKey, _, _, err := resolveProvider(Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if provider != providerOpenRouter {
		t.Fatalf("expected openrouter provider, got %s", provider)
	}
	if apiKey != "or-test-key" {
		t.Fatalf("expected or-test-key, got %s", apiKey)
	}
}

func TestGroqMessageConversionPreservesToolCallsAndResults(t *testing.T) {
	input := []Message{
		{
			Role: "user",
			Content: []ContentBlock{{
				Type: "text",
				Text: "hola",
			}},
		},
		{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: "voy"},
				{Type: "tool_use", ID: "call_1", Name: "bash", Input: json.RawMessage(`{"command":"pwd"}`)},
			},
		},
		{
			Role: "user",
			Content: []ContentBlock{{
				Type:      "tool_result",
				ToolUseID: "call_1",
				Content:   "/tmp",
			}},
		},
	}

	out := toGroqMessages(input)
	if len(out) != 3 {
		t.Fatalf("expected 3 groq messages, got %#v", out)
	}
	if out[1].Role != "assistant" || len(out[1].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call, got %#v", out[1])
	}
	if out[2].Role != "tool" || out[2].ToolCallID != "call_1" {
		t.Fatalf("expected tool result message, got %#v", out[2])
	}
}

func TestGroqMessageConversionSupportsImages(t *testing.T) {
	out := toGroqMessages([]Message{{
		Role: "user",
		Content: []ContentBlock{
			{Type: "text", Text: "mira esto"},
			{Type: "image", ImageURL: "data:image/png;base64,abc"},
		},
	}})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %#v", out)
	}
	parts, ok := out[0].Content.([]GroqContentPart)
	if !ok {
		t.Fatalf("expected content parts, got %#v", out[0].Content)
	}
	if len(parts) != 2 || parts[1].Type != "image_url" || parts[1].ImageURL == nil {
		t.Fatalf("expected image_url part, got %#v", parts)
	}
}

func TestMiniMaxRejectsImages(t *testing.T) {
	agent := &Agent{provider: providerMiniMax}
	_, err := agent.requestMiniMax(nil, []Message{{
		Role:    "user",
		Content: []ContentBlock{{Type: "image", ImageURL: "data:image/png;base64,abc"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "no soporta imágenes") {
		t.Fatalf("expected minimax image error, got %v", err)
	}
}

func TestShellCommandFallbackExtractsSingleCommand(t *testing.T) {
	command, display := shellCommandFromTextResponse([]ContentBlock{{
		Type: "text",
		Text: "cd /tmp && ./frameworkecho status",
	}})
	if command != "cd /tmp && ./frameworkecho status" {
		t.Fatalf("unexpected command: %q", command)
	}
	if display != "" {
		t.Fatalf("unexpected display text: %q", display)
	}
}

func TestShellCommandFallbackRejectsNaturalLanguage(t *testing.T) {
	command, _ := shellCommandFromTextResponse([]ContentBlock{{
		Type: "text",
		Text: "Deberias ejecutar cd /tmp && ./frameworkecho status",
	}})
	if command != "" {
		t.Fatalf("expected no fallback command, got %q", command)
	}
}

func TestShellCommandFallbackKeepsQuestionVisible(t *testing.T) {
	command, display := shellCommandFromTextResponse([]ContentBlock{{
		Type: "text",
		Text: "¿Cuál es la actividad que más tiempo te consume?\n\ngo run ./cmd/flujo done echo --event echo_waiting_user --message \"¿Cuál es la actividad que más tiempo te consume?\"",
	}})
	if !strings.HasPrefix(command, "go run ./cmd/flujo done echo") {
		t.Fatalf("unexpected command: %q", command)
	}
	if strings.TrimSpace(display) != "¿Cuál es la actividad que más tiempo te consume?" {
		t.Fatalf("unexpected display text: %q", display)
	}
}

func TestShellCommandFallbackExtractsSerializedToolCall(t *testing.T) {
	command, display := shellCommandFromTextResponse([]ContentBlock{{
		Type: "text",
		Text: `{
  "name": "bash",
  "parameters": {
    "command": "cd /tmp && ./frameworkecho status"
  }
}`,
	}})
	if command != "cd /tmp && ./frameworkecho status" {
		t.Fatalf("unexpected command: %q", command)
	}
	if display != "" {
		t.Fatalf("unexpected display text: %q", display)
	}
}

func TestGroqFailedGenerationResponse(t *testing.T) {
	body, err := json.Marshal(GroqErrorResponse{
		Error: struct {
			Message          string `json:"message"`
			Type             string `json:"type"`
			Code             string `json:"code"`
			FailedGeneration string `json:"failed_generation"`
		}{
			Message:          "Failed to call a function",
			Type:             "invalid_request_error",
			Code:             "tool_use_failed",
			FailedGeneration: "```bash\n./frameworkalfa status\n```",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := groqFailedGenerationResponse(body)
	if !ok {
		t.Fatal("expected failed_generation recovery")
	}
	if len(resp.Content) != 1 || !strings.Contains(resp.Content[0].Text, "./frameworkalfa status") {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestSuccessfulTerminalHandoff(t *testing.T) {
	input := json.RawMessage(`{"command":"go run ./cmd/flujo ask-echo --from alfa --question \"x\""}`)
	if !successfulTerminalHandoff(input, `handoff echo question="x"`+"\n") {
		t.Fatal("expected successful ask-echo to stop the agent turn")
	}
}

func TestRejectedTerminalHandoffDoesNotStop(t *testing.T) {
	input := json.RawMessage(`{"command":"go run ./cmd/flujo done echo --event echo_waiting_user --message \"x\""}`)
	if successfulTerminalHandoff(input, "policy_error: Alfa solo puede ejecutar done alfa") {
		t.Fatal("expected rejected handoff command to keep the agent running")
	}
}

func TestMultiCommandWithHandoffTextDoesNotStop(t *testing.T) {
	input := json.RawMessage(`{"command":"ls -la temp || true\ngo run ./cmd/flujo done alfa --event alfa_asks_question --message \"x\""}`)
	if successfulTerminalHandoff(input, "total 0\n") {
		t.Fatal("expected mixed fallback command block to keep the agent running")
	}
}

func TestCdAndTerminalHandoffStops(t *testing.T) {
	input := json.RawMessage(`{"command":"cd /tmp && go run ./cmd/flujo done alfa --event alfa_asks_question --message \"x\""}`)
	if !successfulTerminalHandoff(input, "[cola] Alfa agregó pregunta: x\noff alfa event=alfa_asks_question\n") {
		t.Fatal("expected cd plus terminal handoff to stop the agent turn")
	}
}

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

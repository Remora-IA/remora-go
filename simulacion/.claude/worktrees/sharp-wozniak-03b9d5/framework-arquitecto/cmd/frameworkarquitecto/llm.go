package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ChatMsg es un turno en la conversación con el LLM.
// Content es texto plano para roles user/assistant/system.
// Para role="tool" Content es el resultado serializado de la tool call.
// ToolCalls son los tool_calls emitidos por el assistant (si los hay).
// ToolCallID asocia un mensaje role=tool con el call específico que lo generó.
type ChatMsg struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall es una invocación de herramienta pedida por el LLM.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // siempre "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolDef es la definición de una tool que el LLM puede llamar.
type ToolDef struct {
	Type     string          `json:"type"` // siempre "function"
	Function ToolDefFunction `json:"function"`
}

type ToolDefFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// LLMResponse encapsula una respuesta del modelo: texto y/o tool_calls.
type LLMResponse struct {
	Content   string
	ToolCalls []ToolCall
	Raw       *ChatMsg
}

// llmCompleteWithTools llama al LLM enviando tools disponibles.
//
// Provider priority:
//   1. ARQUITECTO_LLM_PROVIDER env var si está seteada
//   2. MiniMax (default) si MINIMAX_API_KEY está presente
//   3. Groq fallback si GROQ_API_KEY está presente
//   4. Error si ninguna key está presente
//
// toolChoice:
//   - "auto"     (default): el modelo decide
//   - "required"         : DEBE llamar al menos una tool
//   - "none"             : puro texto
//
// Si onDelta != nil, usa streaming y emite deltas en tiempo real.
func llmCompleteWithTools(ctx context.Context, system string, history []ChatMsg, tools []ToolDef, toolChoice string, onDelta func(StreamDelta)) (LLMResponse, error) {
	loadEnvFiles()
	provider := resolveProvider()
	if toolChoice == "" {
		toolChoice = "auto"
	}
	switch provider {
	case "minimax":
		return minimaxCompleteWithTools(ctx, system, history, tools, toolChoice, onDelta)
	case "openrouter", "groq":
		var resp LLMResponse
		var err error
		if onDelta != nil {
			resp, err = groqStreamWithTools(ctx, system, history, tools, toolChoice, onDelta)
		} else {
			resp, err = groqCompleteWithTools(ctx, system, history, tools, toolChoice)
		}
		if err != nil && isToolUseFailedErr(err) && toolChoice != "none" {
			if onDelta != nil {
				return groqStreamWithTools(ctx, system, history, nil, "none", onDelta)
			}
			return groqCompleteWithTools(ctx, system, history, nil, "none")
		}
		return resp, err
	default:
		return LLMResponse{}, fmt.Errorf("provider %q no soporta tool calling (usá openrouter, groq o minimax)", provider)
	}
}

// resolveProvider elige el proveedor LLM según disponibilidad de keys.
// Groq es el default para frameworks de programación (arquitecto, critico, paladin)
// porque tiene tool calling más estable. MiniMax queda como fallback.
func resolveProvider() string {
	explicit := strings.ToLower(getenvDefault("ARQUITECTO_LLM_PROVIDER", ""))
	if explicit != "" {
		return explicit
	}
	global := strings.ToLower(strings.TrimSpace(os.Getenv("REMORA_LLM_PROVIDER")))
	if global != "" {
		return global
	}
	if firstEnv("OPENROUTER_API_KEY") != "" {
		return "openrouter"
	}
	if firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY") != "" {
		return "groq"
	}
	if firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY") != "" {
		return "minimax"
	}
	return ""
}

// isToolUseFailedErr detecta el error específico de Groq cuando el modelo
// genera texto que el middleware interpreta como tool call malformado.
func isToolUseFailedErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "tool_use_failed") || strings.Contains(s, "Failed to call a function")
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// loadEnvFiles carga variables de .env.local de varios candidatos.
// Solo setea las que no existen en el entorno actual.
func loadEnvFiles() {
	roots := []string{}
	if v := os.Getenv("REMORA_ROOT"); v != "" {
		roots = append(roots, v)
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
		if parent := parentDir(cwd); parent != "" {
			roots = append(roots, parent)
		}
	}
	seen := map[string]bool{}
	for _, r := range roots {
		for _, suffix := range []string{"/.env.local", "/.env", "/remora-flujo/.env.local"} {
			p := r + suffix
			if seen[p] {
				continue
			}
			seen[p] = true
			readEnvFile(p)
		}
	}
}

func parentDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return ""
}

func readEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 1 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// ----- Groq client con tool calling -----

type groqReq struct {
	Model       string    `json:"model"`
	Messages    []groqMsg `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Tools       []ToolDef `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
}

type groqMsg struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type groqResp struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func groqCompleteWithTools(ctx context.Context, system string, history []ChatMsg, tools []ToolDef, toolChoice string) (LLMResponse, error) {
	provider := resolveProvider()
	var apiKey, url, model string
	switch provider {
	case "openrouter":
		apiKey = firstEnv("OPENROUTER_API_KEY")
		url = getenvDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1/chat/completions")
		model = getenvDefault("ARQUITECTO_LLM_MODEL", "meta-llama/llama-4-scout-17b-16e-instruct")
	default:
		apiKey = firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY")
		url = getenvDefault("GROQ_BASE_URL", "https://api.groq.com/openai/v1/chat/completions")
		model = getenvDefault("ARQUITECTO_LLM_MODEL", "llama-3.3-70b-versatile")
	}
	if apiKey == "" {
		return LLMResponse{}, fmt.Errorf("falta API key para provider %s", provider)
	}

	msgs := []groqMsg{}
	if system != "" {
		msgs = append(msgs, groqMsg{Role: "system", Content: system})
	}
	for _, m := range history {
		msgs = append(msgs, groqMsg{
			Role:       m.Role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
		})
	}

	reqBody := groqReq{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   2048,
		Temperature: 0.2,
	}
	if len(tools) > 0 && toolChoice != "none" {
		reqBody.Tools = tools
		reqBody.ToolChoice = toolChoice
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return LLMResponse{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return LLMResponse{}, err
	}
	if resp.StatusCode >= 400 {
		return LLMResponse{}, fmt.Errorf("groq HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed groqResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return LLMResponse{}, fmt.Errorf("groq parse: %w (body=%s)", err, string(respBody))
	}
	if parsed.Error != nil {
		return LLMResponse{}, fmt.Errorf("groq: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return LLMResponse{}, fmt.Errorf("groq: respuesta vacía")
	}
	choice := parsed.Choices[0].Message
	return LLMResponse{
		Content:   strings.TrimSpace(choice.Content),
		ToolCalls: choice.ToolCalls,
		Raw: &ChatMsg{
			Role:      "assistant",
			Content:   choice.Content,
			ToolCalls: choice.ToolCalls,
		},
	}, nil
}

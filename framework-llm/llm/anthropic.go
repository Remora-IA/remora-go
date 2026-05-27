package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultAnthropicModel     = "claude-haiku-4-5-20251001"
	defaultAnthropicMaxTokens = 600
	anthropicEndpoint         = "https://api.anthropic.com/v1/messages"
	anthropicVersion          = "2023-06-01"
)

// AnthropicClient implementa Client contra la Messages API de Anthropic.
type AnthropicClient struct {
	apiKey string
	http   *http.Client
	model  string
}

// NewAnthropic crea un cliente con la API key del env ANTHROPIC_API_KEY.
// Si no está seteada, devuelve ErrNoCredentials para que el consumidor
// pueda decidir caer a un stub.
func NewAnthropic() (*AnthropicClient, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, ErrNoCredentials
	}
	return &AnthropicClient{
		apiKey: key,
		http:   &http.Client{Timeout: 30 * time.Second},
		model:  defaultAnthropicModel,
	}, nil
}

// WithModel devuelve un cliente con el modelo cambiado.
func (c *AnthropicClient) WithModel(model string) *AnthropicClient {
	cp := *c
	cp.model = model
	return &cp
}

func (c *AnthropicClient) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultAnthropicMaxTokens
	}

	type apiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]apiMsg, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = apiMsg{Role: m.Role, Content: m.Content}
	}

	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"system":     req.System,
		"messages":   msgs,
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm/anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm/anthropic: request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Error      *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("llm/anthropic: decode (status=%d body=%s): %w", resp.StatusCode, string(raw), err)
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("llm/anthropic: %s — %s", decoded.Error.Type, decoded.Error.Message)
	}
	if len(decoded.Content) == 0 {
		return nil, fmt.Errorf("llm/anthropic: empty content (status=%d)", resp.StatusCode)
	}
	return &Response{Text: decoded.Content[0].Text, StopReason: decoded.StopReason}, nil
}

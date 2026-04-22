package minimax

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alfa-conversation/flowguard"
)

const (
	APIURL = "https://api.minimax.io/anthropic/v1/messages"
)

// Client for MiniMax API (Anthropic-compatible)
type Client struct {
	apiKey string
	model  string
}

// NewClient creates a new MiniMax client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  "MiniMax-M2.7",
	}
}

// Request for Anthropic-compatible API
type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
	Thinking  *Thinking `json:"thinking,omitempty"`
}

// Thinking config
type Thinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response from Anthropic API
type Response struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat sends a chat request and returns the response
// Instrumentado para tracing completo: inputs, outputs, errores
func (c *Client) Chat(ctx *flowguard.Context, systemPrompt, userMessage string) (string, error) {
	childCtx := ctx.Child("Chat")
	defer childCtx.End()

	// === REGISTRAR TODOS LOS INPUTS ===
	childCtx.Var("model", c.model)
	childCtx.Var("system_prompt_length", len(systemPrompt))
	childCtx.Var("system_prompt", systemPrompt) // TEXTO COMPLETO
	childCtx.Var("user_message_length", len(userMessage))
	childCtx.Var("user_message", userMessage) // TEXTO COMPLETO

	// Build messages
	messages := []Message{}

	if systemPrompt != "" {
		messages = append(messages, Message{
			Role:    "user",
			Content: systemPrompt + "\n\n" + userMessage,
		})
	} else {
		messages = append(messages, Message{
			Role:    "user",
			Content: userMessage,
		})
	}

	childCtx.Decision("request_prepared", fmt.Sprintf("enviando %d mensajes", len(messages)))

	// Build request - Anthropic-style
	req := Request{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 4096,
		Thinking: &Thinking{
			Type:         "enabled",
			BudgetTokens: 8000, // ~16K tokens de reasoning
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		childCtx.Error(err)
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	childCtx.Var("request_size", len(reqBody))
	childCtx.Var("request_body", string(reqBody)) // BODY COMPLETO PARA DEBUG

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", APIURL, bytes.NewBuffer(reqBody))
	if err != nil {
		childCtx.Error(err)
		return "", fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	// Execute request
	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		childCtx.Error(err)
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	childCtx.Var("response_status", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		childCtx.Error(err)
		return "", fmt.Errorf("reading response: %w", err)
	}

	childCtx.Var("response_size", len(body))
	childCtx.Var("response_body", string(body)) // BODY COMPLETO PARA DEBUG

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		childCtx.ErrorMsg(errMsg)
		return "", fmt.Errorf("API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var chatResp Response
	if err := json.Unmarshal(body, &chatResp); err != nil {
		childCtx.Error(err)
		return "", fmt.Errorf("parsing response: %w", err)
	}

	// Registrar usage tokens
	childCtx.Var("usage_input_tokens", chatResp.Usage.InputTokens)
	childCtx.Var("usage_output_tokens", chatResp.Usage.OutputTokens)
	childCtx.Var("response_id", chatResp.ID)

	// Extract text from response
	var responseText strings.Builder
	for _, content := range chatResp.Content {
		if content.Type == "text" {
			responseText.WriteString(content.Text)
		}
	}

	result := responseText.String()
	childCtx.Var("content_length", len(result))
	childCtx.Var("full_response", result) // RESPUESTA COMPLETA

	childCtx.Decision("response_received", fmt.Sprintf("%d tokens generados", chatResp.Usage.OutputTokens))

	return result, nil
}

// ChatWithFullResponse igual que Chat pero retorna la respuesta completa
// para que el caller pueda registrarla en su propio span sin duplicar el trace
func (c *Client) ChatWithFullResponse(ctx *flowguard.Context, systemPrompt, userMessage string) (string, error) {
	// Esta función no crea child context porque delega a Chat
	// que ya lo hace. Solo retorna la respuesta para quien llamó.
	return c.Chat(ctx, systemPrompt, userMessage)
}

// ChatWithHistory sends a chat request with conversation history
func (c *Client) ChatWithHistory(ctx *flowguard.Context, systemPrompt string, history []Message) (string, error) {
	childCtx := ctx.Child("ChatWithHistory")
	defer childCtx.End()

	childCtx.Var("history_messages", len(history))

	messages := []Message{}

	if systemPrompt != "" {
		messages = append(messages, Message{
			Role:    "user",
			Content: systemPrompt,
		})
	}

	messages = append(messages, history...)

	req := Request{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 4096,
		Thinking: &Thinking{
			Type:         "enabled",
			BudgetTokens: 8000,
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		childCtx.Error(err)
		return "", err
	}

	httpReq, err := http.NewRequest("POST", APIURL, bytes.NewBuffer(reqBody))
	if err != nil {
		childCtx.Error(err)
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	client := &http.Client{Timeout: 120 * time.Second}

	resp, err := client.Do(httpReq)
	if err != nil {
		childCtx.Error(err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		childCtx.Error(err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var chatResp Response
	if err := json.Unmarshal(body, &chatResp); err != nil {
		childCtx.Error(err)
		return "", err
	}

	var responseText strings.Builder
	for _, content := range chatResp.Content {
		if content.Type == "text" {
			responseText.WriteString(content.Text)
		}
	}

	return responseText.String(), nil
}

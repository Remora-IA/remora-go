// Package llm es el cliente LLM unificado del orquestador. Cada framework
// declara en su manifest qué modelo usa (provider + name + env_key); aquí
// están los clientes concretos. El framework EN SÍ MISMO no llama al LLM:
// la API REST orquesta el pre-procesamiento (ej: convertir una imagen en
// texto descriptivo antes de pasarlo a un framework de texto) y luego le
// entrega solo texto al framework.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Resource es un input no-textual del usuario (imagen, audio, archivo).
type Resource struct {
	Type     string `json:"type"`           // "image" | "audio" | "file"
	Path     string `json:"path"`           // path absoluto en disco
	MimeType string `json:"mime,omitempty"` // ej "image/png"
	Name     string `json:"name,omitempty"` // nombre original
}

// Client es la interfaz unificada. Cada provider implementa esto.
type Client interface {
	Provider() string
	Model() string
	Capabilities() []string
	Complete(ctx context.Context, req CompletionRequest) (string, error)
	Stream(ctx context.Context, req CompletionRequest, emit func(StreamEvent)) (string, error)
}

// CompletionRequest es la petición unificada. Si Resources tiene imágenes y
// el modelo es multimodal, se envían como inline base64.
type CompletionRequest struct {
	System    string
	User      string
	Resources []Resource
	MaxTokens int
}

type StreamEvent struct {
	Type  string
	Delta string
}

// Spec describe un modelo a usar. Coincide con manifest.ModelSpec.
type Spec struct {
	Provider     string   `json:"provider"`
	Name         string   `json:"name"`
	EnvKey       string   `json:"env_key"`
	Capabilities []string `json:"capabilities"`
	BaseURL      string   `json:"base_url,omitempty"`
}

// New crea un cliente desde un Spec. Lee la API key del env declarado.
func New(spec Spec) (Client, error) {
	apiKey := os.Getenv(spec.EnvKey)
	if apiKey == "" {
		return nil, fmt.Errorf("llm: falta env %s para provider %s", spec.EnvKey, spec.Provider)
	}
	switch strings.ToLower(spec.Provider) {
	case "openrouter":
		s := spec
		if s.BaseURL == "" {
			s.BaseURL = "https://openrouter.ai/api/v1/chat/completions"
		}
		return &groqClient{spec: s, apiKey: apiKey}, nil
	case "groq":
		return &groqClient{spec: spec, apiKey: apiKey}, nil
	case "minimax":
		return &minimaxClient{spec: spec, apiKey: apiKey}, nil
	default:
		return nil, fmt.Errorf("llm: provider desconocido %q", spec.Provider)
	}
}

// HasCapability indica si el modelo soporta una capability dada.
func (s Spec) HasCapability(cap string) bool {
	for _, c := range s.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Groq client (OpenAI-compatible API). Soporta multimodal (vision) cuando el
// modelo lo es (ej: llama-4-scout).
// ---------------------------------------------------------------------------

type groqClient struct {
	spec   Spec
	apiKey string
}

func (g *groqClient) Provider() string       { return "groq" }
func (g *groqClient) Model() string          { return g.spec.Name }
func (g *groqClient) Capabilities() []string { return g.spec.Capabilities }

type groqChatMsg struct {
	Role    string        `json:"role"`
	Content []groqContent `json:"content"`
}

type groqContent struct {
	Type     string        `json:"type"` // "text" | "image_url"
	Text     string        `json:"text,omitempty"`
	ImageURL *groqImageURL `json:"image_url,omitempty"`
}

type groqImageURL struct {
	URL string `json:"url"`
}

type groqRequest struct {
	Model     string        `json:"model"`
	Messages  []groqChatMsg `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Stream    bool          `json:"stream,omitempty"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (g *groqClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	url := g.spec.BaseURL
	if url == "" {
		url = "https://api.groq.com/openai/v1/chat/completions"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	var msgs []groqChatMsg
	if req.System != "" {
		msgs = append(msgs, groqChatMsg{
			Role:    "system",
			Content: []groqContent{{Type: "text", Text: req.System}},
		})
	}

	userContent := []groqContent{{Type: "text", Text: req.User}}
	for _, r := range req.Resources {
		if r.Type != "image" {
			continue
		}
		dataURL, err := imageToDataURL(r)
		if err != nil {
			return "", fmt.Errorf("llm/groq: %w", err)
		}
		userContent = append(userContent, groqContent{
			Type:     "image_url",
			ImageURL: &groqImageURL{URL: dataURL},
		})
	}
	msgs = append(msgs, groqChatMsg{Role: "user", Content: userContent})

	body, err := json.Marshal(groqRequest{Model: g.spec.Name, Messages: msgs, MaxTokens: maxTokens})
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm/groq: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed groqResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llm/groq: parse: %w (body=%s)", err, string(respBody))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm/groq: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm/groq: respuesta vacía")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (g *groqClient) Stream(ctx context.Context, req CompletionRequest, emit func(StreamEvent)) (string, error) {
	url := g.spec.BaseURL
	if url == "" {
		url = "https://api.groq.com/openai/v1/chat/completions"
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}
	var msgs []groqChatMsg
	if req.System != "" {
		msgs = append(msgs, groqChatMsg{Role: "system", Content: []groqContent{{Type: "text", Text: req.System}}})
	}
	msgs = append(msgs, groqChatMsg{Role: "user", Content: []groqContent{{Type: "text", Text: req.User}}})
	body, err := json.Marshal(groqRequest{Model: g.spec.Name, Messages: msgs, MaxTokens: maxTokens, Stream: true})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm/groq: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	emit(StreamEvent{Type: "text_start"})
	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta map[string]string `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return "", fmt.Errorf("llm/groq: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta["content"]
		if delta == "" {
			delta = chunk.Choices[0].Delta["reasoning_content"]
			if delta == "" {
				delta = chunk.Choices[0].Delta["reasoning"]
			}
			if delta != "" {
				emit(StreamEvent{Type: "thinking_delta", Delta: delta})
				continue
			}
		}
		if delta != "" {
			sb.WriteString(delta)
			emit(StreamEvent{Type: "text_delta", Delta: delta})
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	emit(StreamEvent{Type: "text_end"})
	return sb.String(), nil
}

// imageToDataURL lee un archivo de imagen del disco y lo codifica como
// data URL inline (data:image/<mime>;base64,...) para enviar a Groq.
func imageToDataURL(r Resource) (string, error) {
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return "", fmt.Errorf("read image %s: %w", r.Path, err)
	}
	mime := r.MimeType
	if mime == "" {
		switch strings.ToLower(filepath.Ext(r.Path)) {
		case ".png":
			mime = "image/png"
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".webp":
			mime = "image/webp"
		default:
			mime = "image/png"
		}
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// ---------------------------------------------------------------------------
// Minimax client (Anthropic-compatible API).
// ---------------------------------------------------------------------------

type minimaxClient struct {
	spec   Spec
	apiKey string
}

func (m *minimaxClient) Provider() string       { return "minimax" }
func (m *minimaxClient) Model() string          { return m.spec.Name }
func (m *minimaxClient) Capabilities() []string { return m.spec.Capabilities }

type miniRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []miniMsg `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
}

type miniMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type miniResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (m *minimaxClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	url := m.spec.BaseURL
	if url == "" {
		url = "https://api.minimax.io/anthropic/v1/messages"
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	body, err := json.Marshal(miniRequest{
		Model:     m.spec.Name,
		MaxTokens: maxTokens,
		System:    req.System,
		Messages:  []miniMsg{{Role: "user", Content: req.User}},
	})
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-api-key", m.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	httpReq.Header.Set("anthropic-beta", "fine-grained-tool-streaming-2025-05-14")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm/minimax: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed miniResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llm/minimax: parse: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm/minimax: %s", parsed.Error.Message)
	}
	var sb strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}

func (m *minimaxClient) Stream(ctx context.Context, req CompletionRequest, emit func(StreamEvent)) (string, error) {
	url := m.spec.BaseURL
	if url == "" {
		url = "https://api.minimax.io/anthropic/v1/messages"
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}
	body, err := json.Marshal(miniRequest{
		Model:     m.spec.Name,
		MaxTokens: maxTokens,
		System:    req.System,
		Messages:  []miniMsg{{Role: "user", Content: req.User}},
		Stream:    true,
	})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-api-key", m.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	httpReq.Header.Set("anthropic-beta", "fine-grained-tool-streaming-2025-05-14")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm/minimax: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var sb strings.Builder
	var currentEvent string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		handleMiniStreamData(currentEvent, data, &sb, emit)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func handleMiniStreamData(eventName, data string, sb *strings.Builder, emit func(StreamEvent)) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return
	}
	switch eventName {
	case "content_block_start":
		block, _ := payload["content_block"].(map[string]any)
		switch block["type"] {
		case "thinking":
			emit(StreamEvent{Type: "thinking_start"})
		case "text":
			emit(StreamEvent{Type: "text_start"})
		}
	case "content_block_delta":
		delta, _ := payload["delta"].(map[string]any)
		switch delta["type"] {
		case "thinking_delta":
			if s, _ := delta["thinking"].(string); s != "" {
				emit(StreamEvent{Type: "thinking_delta", Delta: s})
			}
		case "text_delta":
			if s, _ := delta["text"].(string); s != "" {
				sb.WriteString(s)
				emit(StreamEvent{Type: "text_delta", Delta: s})
			}
		}
	case "content_block_stop":
		emit(StreamEvent{Type: "text_end"})
	}
}

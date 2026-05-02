// Package llm es un cliente chat mínimo que reproduce las dos rutas
// soportadas por remora-flujo/nativeagent:
//
//   - Groq    → POST https://api.groq.com/openai/v1/chat/completions  (OpenAI-compat)
//   - MiniMax → POST https://api.minimax.io/anthropic/v1/messages     (Anthropic-compat)
//
// La selección de proveedor sigue las MISMAS reglas que nativeagent:
//   1. REMORA_LLM_PROVIDER (groq|minimax)
//   2. Si vacío: groq cuando hay GROQ_API_KEY/REMORA_GROQ_API_KEY, sino minimax
//
// Variables de entorno:
//   GROQ_API_KEY o REMORA_GROQ_API_KEY
//   REMORA_GROQ_MODEL (default meta-llama/llama-4-scout-17b-16e-instruct)
//   MINIMAX_API_KEY o REMORA_MINIMAX_API_KEY
//   REMORA_MINIMAX_MODEL (default MiniMax-M2.7)
//
// Sabio NO necesita tracing/paladin, function-calling ni soporte de
// imágenes; por eso este cliente es una versión recortada en vez de
// importar nativeagent (que arrastra remora-flujo entero).
package llm

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

const (
	providerGroq        = "groq"
	providerMiniMax     = "minimax"
	groqAPIURL          = "https://api.groq.com/openai/v1/chat/completions"
	minimaxAPIURL       = "https://api.minimax.io/anthropic/v1/messages"
	defaultGroqModel    = "meta-llama/llama-4-scout-17b-16e-instruct"
	defaultMiniMaxModel = "MiniMax-M2.7"
	requestTimeout      = 90 * time.Second
)

type Client struct {
	provider string
	apiKey   string
	model    string
	http     *http.Client
}

// NewClient resuelve provider/key/model siguiendo las reglas de nativeagent.
func NewClient() (*Client, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("REMORA_LLM_PROVIDER")))
	if provider == "" {
		if firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY") != "" {
			provider = providerGroq
		} else {
			provider = providerMiniMax
		}
	}

	c := &Client{
		provider: provider,
		http:     &http.Client{Timeout: requestTimeout},
	}

	switch provider {
	case providerGroq:
		c.apiKey = firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY")
		if c.apiKey == "" {
			return nil, fmt.Errorf("falta GROQ_API_KEY/REMORA_GROQ_API_KEY")
		}
		c.model = firstNonEmpty(os.Getenv("REMORA_GROQ_MODEL"), defaultGroqModel)
	case providerMiniMax:
		c.apiKey = firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY")
		if c.apiKey == "" {
			return nil, fmt.Errorf("falta MINIMAX_API_KEY/REMORA_MINIMAX_API_KEY")
		}
		c.model = firstNonEmpty(os.Getenv("REMORA_MINIMAX_MODEL"), defaultMiniMaxModel)
	default:
		return nil, fmt.Errorf("proveedor LLM no soportado: %s", provider)
	}
	return c, nil
}

// Provider devuelve el nombre del backend activo.
func (c *Client) Provider() string { return c.provider }

// Model devuelve el modelo activo.
func (c *Client) Model() string { return c.model }

// Generate manda system + user y retorna el texto del primer candidato.
func (c *Client) Generate(ctx context.Context, system, user string) (string, error) {
	switch c.provider {
	case providerGroq:
		return c.generateGroq(ctx, system, user)
	case providerMiniMax:
		return c.generateMiniMax(ctx, system, user)
	default:
		return "", fmt.Errorf("proveedor no soportado: %s", c.provider)
	}
}

// ---------- Groq (OpenAI-compatible) ----------

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiReq struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens"`
}

type oaiResp struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *Client) generateGroq(ctx context.Context, system, user string) (string, error) {
	msgs := []oaiMessage{}
	if system != "" {
		msgs = append(msgs, oaiMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, oaiMessage{Role: "user", Content: user})

	body, _ := json.Marshal(oaiReq{
		Model:       c.model,
		Messages:    msgs,
		Temperature: 0.2,
		MaxTokens:   1024,
	})
	raw, status, err := c.do(ctx, groqAPIURL, body)
	if err != nil {
		return "", err
	}
	var parsed oaiResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("groq parse (status %d): %w; body=%s", status, err, string(raw))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("groq api error: %s (%s)", parsed.Error.Message, parsed.Error.Type)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("groq: respuesta sin choices; body=%s", string(raw))
	}
	return parsed.Choices[0].Message.Content, nil
}

// ---------- MiniMax (Anthropic-compatible) ----------

type anthBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthMessage struct {
	Role    string      `json:"role"`
	Content []anthBlock `json:"content"`
}

type anthReq struct {
	Model     string        `json:"model"`
	System    string        `json:"system,omitempty"`
	Messages  []anthMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type anthResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) generateMiniMax(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(anthReq{
		Model:  c.model,
		System: system,
		Messages: []anthMessage{
			{Role: "user", Content: []anthBlock{{Type: "text", Text: user}}},
		},
		MaxTokens: 1024,
	})
	raw, status, err := c.do(ctx, minimaxAPIURL, body)
	if err != nil {
		return "", err
	}
	var parsed anthResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("minimax parse (status %d): %w; body=%s", status, err, string(raw))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("minimax api error: %s (%s)", parsed.Error.Message, parsed.Error.Type)
	}
	// Tomamos el primer bloque de tipo "text"; ignoramos "thinking".
	for _, b := range parsed.Content {
		if b.Type == "text" && b.Text != "" {
			return b.Text, nil
		}
	}
	return "", fmt.Errorf("minimax: respuesta sin bloque text; body=%s", string(raw))
}

// ---------- helpers ----------

func (c *Client) do(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode, nil
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

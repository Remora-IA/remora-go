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
	providerGroq           = "groq"
	providerMiniMax        = "minimax"
	providerOpenRouter     = "openrouter"
	groqAPIURL             = "https://api.groq.com/openai/v1/chat/completions"
	minimaxAPIURL          = "https://api.minimax.io/anthropic/v1/messages"
	openRouterAPIURL       = "https://openrouter.ai/api/v1/chat/completions"
	defaultGroqModel       = "meta-llama/llama-4-scout-17b-16e-instruct"
	defaultMiniMaxModel    = "MiniMax-M2.7"
	defaultOpenRouterModel = "meta-llama/llama-4-scout-17b-16e-instruct"
)

type Client struct {
	provider string
	apiKey   string
	model    string
	http     *http.Client
}

func NewClient() (*Client, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("REMORA_LLM_PROVIDER")))
	if provider == "" {
		if firstEnv("OPENROUTER_API_KEY") != "" {
			provider = providerOpenRouter
		} else if firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY") != "" {
			provider = providerGroq
		} else {
			provider = providerMiniMax
		}
	}

	c := &Client{provider: provider, http: &http.Client{Timeout: 90 * time.Second}}
	switch provider {
	case providerOpenRouter:
		c.apiKey = firstEnv("OPENROUTER_API_KEY")
		if c.apiKey == "" {
			return nil, fmt.Errorf("falta OPENROUTER_API_KEY")
		}
		c.model = firstNonEmpty(os.Getenv("REMORA_OPENROUTER_MODEL"), defaultOpenRouterModel)
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

func (c *Client) Generate(ctx context.Context, system, user string) (string, error) {
	switch c.provider {
	case providerOpenRouter:
		return c.generateOAICompat(ctx, openRouterAPIURL, system, user)
	case providerGroq:
		return c.generateOAICompat(ctx, groqAPIURL, system, user)
	case providerMiniMax:
		return c.generateMiniMax(ctx, system, user)
	default:
		return "", fmt.Errorf("proveedor no soportado: %s", c.provider)
	}
}

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

func (c *Client) generateOAICompat(ctx context.Context, apiURL, system, user string) (string, error) {
	msgs := []oaiMessage{}
	if system != "" {
		msgs = append(msgs, oaiMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, oaiMessage{Role: "user", Content: user})
	body, _ := json.Marshal(oaiReq{Model: c.model, Messages: msgs, Temperature: 0.1, MaxTokens: 900})
	raw, status, err := c.do(ctx, apiURL, body)
	if err != nil {
		return "", err
	}
	var parsed oaiResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("%s parse (status %d): %w; body=%s", c.provider, status, err, string(raw))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s api error: %s (%s)", c.provider, parsed.Error.Message, parsed.Error.Type)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("%s: respuesta sin choices; body=%s", c.provider, string(raw))
	}
	return parsed.Choices[0].Message.Content, nil
}

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
	body, _ := json.Marshal(anthReq{Model: c.model, System: system, Messages: []anthMessage{{Role: "user", Content: []anthBlock{{Type: "text", Text: user}}}}, MaxTokens: 900})
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
	for _, b := range parsed.Content {
		if b.Type == "text" && b.Text != "" {
			return b.Text, nil
		}
	}
	return "", fmt.Errorf("minimax: respuesta sin bloque text; body=%s", string(raw))
}

func (c *Client) do(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.provider == providerMiniMax {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return raw, resp.StatusCode, fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, string(raw))
	}
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

package main

// Streaming LLM client para Groq. Emite deltas de texto y tool_calls en
// tiempo real, inspirado en agent-loop.ts de tau.
//
// Groq es OpenAI-compatible: cada chunk SSE tiene choices[0].delta con
// content (texto parcial) y/o tool_calls (args parciales por index).

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StreamDelta es el callback que recibe cada delta durante el streaming.
// - Si Content != "": hay texto nuevo para agregar a la respuesta
// - Si ToolCall != nil: hay progreso en una tool call (por index)
type StreamDelta struct {
	Content  string
	ToolCall *ToolCallDelta
}

type ToolCallDelta struct {
	Index        int
	ID           string // presente en el primer chunk de ese tool call
	Name         string // presente en el primer chunk
	ArgsChunk    string // fragmento JSON de los args
	ArgsSoFar    string // acumulado hasta ahora (para display)
}

// groqStreamWithTools llama a Groq con stream=true, parsea los SSE chunks,
// invoca onDelta para cada fragmento, y devuelve LLMResponse final acumulada.
func groqStreamWithTools(
	ctx context.Context,
	system string,
	history []ChatMsg,
	tools []ToolDef,
	toolChoice string,
	onDelta func(StreamDelta),
) (LLMResponse, error) {
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
			Role: m.Role, Content: m.Content, Name: m.Name,
			ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID,
		})
	}

	type streamReq struct {
		Model       string    `json:"model"`
		Messages    []groqMsg `json:"messages"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
		Stream      bool      `json:"stream"`
		Tools       []ToolDef `json:"tools,omitempty"`
		ToolChoice  string    `json:"tool_choice,omitempty"`
	}
	reqBody := streamReq{
		Model: model, Messages: msgs, MaxTokens: 2048,
		Temperature: 0.2, Stream: true,
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
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return LLMResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return LLMResponse{}, fmt.Errorf("groq HTTP %d: %s", resp.StatusCode, string(raw))
	}

	// Acumuladores: texto completo + tool_calls indexados por index.
	var contentSB strings.Builder
	toolByIndex := map[int]*ToolCall{}
	argsByIndex := map[int]*strings.Builder{}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return LLMResponse{}, ctx.Err()
		default:
		}
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\r\n")
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				if data == "[DONE]" {
					break
				}
				continue
			}
			var chunk groqStreamChunk
			if jerr := json.Unmarshal([]byte(data), &chunk); jerr != nil {
				continue
			}
			if chunk.Error != nil {
				return LLMResponse{}, fmt.Errorf("groq: %s", chunk.Error.Message)
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				contentSB.WriteString(delta.Content)
				if onDelta != nil {
					onDelta(StreamDelta{Content: delta.Content})
				}
			}
			for _, tcd := range delta.ToolCalls {
				tc, ok := toolByIndex[tcd.Index]
				if !ok {
					tc = &ToolCall{Type: "function"}
					toolByIndex[tcd.Index] = tc
					argsByIndex[tcd.Index] = &strings.Builder{}
				}
				if tcd.ID != "" {
					tc.ID = tcd.ID
				}
				if tcd.Function.Name != "" {
					tc.Function.Name = tcd.Function.Name
				}
				if tcd.Function.Arguments != "" {
					argsByIndex[tcd.Index].WriteString(tcd.Function.Arguments)
					tc.Function.Arguments = argsByIndex[tcd.Index].String()
				}
				if onDelta != nil {
					onDelta(StreamDelta{ToolCall: &ToolCallDelta{
						Index:     tcd.Index,
						ID:        tc.ID,
						Name:      tc.Function.Name,
						ArgsChunk: tcd.Function.Arguments,
						ArgsSoFar: tc.Function.Arguments,
					}})
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return LLMResponse{}, err
		}
	}

	// Armar LLMResponse final.
	var toolCalls []ToolCall
	for i := 0; i < len(toolByIndex); i++ {
		if tc, ok := toolByIndex[i]; ok {
			if tc.ID == "" {
				tc.ID = fmt.Sprintf("call_stream_%d_%d", i, time.Now().UnixNano())
			}
			toolCalls = append(toolCalls, *tc)
		}
	}
	content := strings.TrimSpace(contentSB.String())
	return LLMResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Raw:       &ChatMsg{Role: "assistant", Content: content, ToolCalls: toolCalls},
	}, nil
}

// groqStreamChunk es el formato de cada chunk SSE de Groq/OpenAI.
type groqStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

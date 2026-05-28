package main

// Cliente MiniMax Anthropic-compatible para arquitecto.
// MiniMax expone /anthropic/v1/messages con formato Anthropic Messages API.
// Soporta tool_use, tool_result, streaming SSE, y tool_choice.
//
// Endpoint: https://api.minimax.io/anthropic/v1/messages
// Modelo default: MiniMax-M2.7 (configurable via ARQUITECTO_LLM_MODEL)
// Auth: header x-api-key + anthropic-version: 2023-06-01

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

const (
	minimaxAPIURL       = "https://api.minimax.io/anthropic/v1/messages"
	defaultMiniMaxModel = "MiniMax-M2.7"
)

// ---------------------------------------------------------------------------
// Público: entrypoint usado desde llm.go
// ---------------------------------------------------------------------------

// minimaxCompleteWithTools llama a MiniMax. Si onDelta != nil usa streaming.
func minimaxCompleteWithTools(
	ctx context.Context,
	system string,
	history []ChatMsg,
	tools []ToolDef,
	toolChoice string,
	onDelta func(StreamDelta),
) (LLMResponse, error) {
	apiKey := firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY")
	if apiKey == "" {
		return LLMResponse{}, fmt.Errorf("falta MINIMAX_API_KEY o REMORA_MINIMAX_API_KEY")
	}
	model := getenvDefault("ARQUITECTO_LLM_MODEL", defaultMiniMaxModel)

	req := anthropicReq{
		Model:      model,
		MaxTokens:  4096,
		System:     system,
		Messages:   toAnthropicMessages(history),
		Tools:      toAnthropicTools(tools),
		ToolChoice: toAnthropicToolChoice(toolChoice),
	}
	if onDelta != nil {
		req.Stream = true
		return minimaxStream(ctx, req, apiKey, onDelta)
	}
	return minimaxNonStream(ctx, req, apiKey)
}

// ---------------------------------------------------------------------------
// Non-streaming
// ---------------------------------------------------------------------------

func minimaxNonStream(ctx context.Context, req anthropicReq, apiKey string) (LLMResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return LLMResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", minimaxAPIURL, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, err
	}
	setAnthropicHeaders(httpReq, apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return LLMResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return LLMResponse{}, err
	}
	if resp.StatusCode >= 400 {
		// Debug: incluir IDs en el error para diagnóstico visible en CLI
		idDebug := debugToolIDSummary(req.Messages)
		return LLMResponse{}, fmt.Errorf("minimax HTTP %d: %s\n[DEBUG TOOL IDs]:\n%s", resp.StatusCode, string(respBody), idDebug)
	}

	var ar anthropicResp
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return LLMResponse{}, fmt.Errorf("minimax parse: %w", err)
	}
	return fromAnthropicResp(ar)
}

func debugToolIDSummary(msgs []anthropicMsg) string {
	var sb strings.Builder
	var toolUseIDs []string
	var toolResultIDs []string
	for i, m := range msgs {
		blocks, ok := m.Content.([]anthropicContentBlock)
		if !ok {
			continue
		}
		for _, b := range blocks {
			switch b.Type {
			case "tool_use":
				toolUseIDs = append(toolUseIDs, fmt.Sprintf("msg[%d] tool_use id=%s", i, b.ID))
			case "tool_result":
				toolResultIDs = append(toolResultIDs, fmt.Sprintf("msg[%d] tool_result tool_use_id=%s", i, b.ToolUseID))
			}
		}
	}
	sb.WriteString("tool_use IDs in history:\n")
	for _, id := range toolUseIDs {
		sb.WriteString("  " + id + "\n")
	}
	sb.WriteString("tool_result IDs in history:\n")
	for _, id := range toolResultIDs {
		sb.WriteString("  " + id + "\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Streaming SSE (Anthropic format)
// ---------------------------------------------------------------------------

func minimaxStream(ctx context.Context, req anthropicReq, apiKey string, onDelta func(StreamDelta)) (LLMResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return LLMResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", minimaxAPIURL, bytes.NewReader(body))
	if err != nil {
		return LLMResponse{}, err
	}
	setAnthropicHeaders(httpReq, apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return LLMResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return LLMResponse{}, fmt.Errorf("minimax HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var contentSB strings.Builder
	// index (0,1,...) -> acumulador de tool_call
	toolByIndex := map[int]*ToolCall{}
	argsByIndex := map[int]*strings.Builder{}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return LLMResponse{}, ctx.Err()
		default:
		}

		// Leer un evento SSE completo (termina en línea vacía)
		eventType := ""
		var eventData strings.Builder
		for {
			line, err := reader.ReadString('\n')
			line = strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if eventData.Len() > 0 {
					eventData.WriteByte('\n')
				}
				eventData.WriteString(data)
			} else if line == "" {
				// Fin de evento
				break
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return LLMResponse{}, err
			}
		}
		if eventType == "" && eventData.Len() == 0 {
			// No más eventos
			break
		}
		if eventType == "ping" {
			continue
		}

		var evt anthropicStreamEvent
		if jerr := json.Unmarshal([]byte(eventData.String()), &evt); jerr != nil {
			continue // ignora eventos malformados
		}

		switch evt.Type {
		case "message_start":
			// nada que acumular todavía
		case "content_block_start":
			if evt.ContentBlock == nil {
				continue
			}
			switch evt.ContentBlock.Type {
			case "text":
				// texto empieza vacío, los deltas vienen después
			case "tool_use":
				// Inicializar tool call nueva
				idx := evt.Index
				toolByIndex[idx] = &ToolCall{
					Type: "function",
					ID:   evt.ContentBlock.ID,
					Function: ToolCallFunction{
						Name:      evt.ContentBlock.Name,
						Arguments: "",
					},
				}
				argsByIndex[idx] = &strings.Builder{}
				if onDelta != nil {
					onDelta(StreamDelta{ToolCall: &ToolCallDelta{
						Index:  idx,
						ID:     evt.ContentBlock.ID,
						Name:   evt.ContentBlock.Name,
						ArgsSoFar: "",
					}})
				}
			}
		case "content_block_delta":
			if evt.Delta == nil {
				continue
			}
			switch evt.Delta.Type {
			case "text_delta":
				contentSB.WriteString(evt.Delta.Text)
				if onDelta != nil {
					onDelta(StreamDelta{Content: evt.Delta.Text})
				}
			case "input_json_delta":
				idx := evt.Index
				if sb, ok := argsByIndex[idx]; ok {
					sb.WriteString(evt.Delta.PartialJSON)
				}
				if tc, ok := toolByIndex[idx]; ok {
					tc.Function.Arguments = argsByIndex[idx].String()
					if onDelta != nil {
						onDelta(StreamDelta{ToolCall: &ToolCallDelta{
							Index:     idx,
							ID:        tc.ID,
							Name:      tc.Function.Name,
							ArgsChunk: evt.Delta.PartialJSON,
							ArgsSoFar: tc.Function.Arguments,
						}})
					}
				}
			}
		case "content_block_stop":
			// tool call terminó de streamear args
		case "message_delta":
			// stop_reason, usage — nada que acumular
		case "message_stop":
			// fin del mensaje
		}
	}

	// Armar LLMResponse final
	var toolCalls []ToolCall
	for i := 0; i < len(toolByIndex); i++ {
		if tc, ok := toolByIndex[i]; ok {
			if tc.ID == "" {
				tc.ID = fmt.Sprintf("call_mm_%d_%d", i, time.Now().UnixNano())
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

func setAnthropicHeaders(r *http.Request, apiKey string) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("x-api-key", apiKey)
	r.Header.Set("anthropic-version", "2023-06-01")
	r.Header.Set("anthropic-dangerous-direct-browser-access", "true")
}

// ---------------------------------------------------------------------------
// Conversión de formatos
// ---------------------------------------------------------------------------

func toAnthropicMessages(history []ChatMsg) []anthropicMsg {
	msgs := []anthropicMsg{}
	// Set de tool_use IDs que hemos visto en el historial.
	// Si un tool_result referencia un ID que no está aquí, es huérfano
	// (de una sesión previa con otro proveedor) y lo saltamos.
	seenToolUseIDs := map[string]bool{}
	for _, m := range history {
		switch m.Role {
		case "system":
			// Skip: Anthropic usa campo top-level "system"
			continue
		case "user":
			msgs = append(msgs, anthropicMsg{Role: "user", Content: m.Content})
		case "assistant":
			if len(m.ToolCalls) > 0 {
				blocks := []anthropicContentBlock{}
				if m.Content != "" {
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					seenToolUseIDs[tc.ID] = true
					blocks = append(blocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: json.RawMessage(tc.Function.Arguments),
					})
				}
				msgs = append(msgs, anthropicMsg{Role: "assistant", Content: blocks})
			} else {
				msgs = append(msgs, anthropicMsg{Role: "assistant", Content: m.Content})
			}
		case "tool":
			// Saltar tool_results cuyo tool_use_id no tenga un tool_use previo.
			// Esto pasa cuando se cambia de proveedor (ej: Groq → MiniMax) y el
			// historial persistente tiene tool results de sesiones anteriores.
			if !seenToolUseIDs[m.ToolCallID] {
				continue
			}
			blocks := []anthropicContentBlock{{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}}
			msgs = append(msgs, anthropicMsg{Role: "user", Content: blocks})
		}
	}
	return msgs
}

func toAnthropicTools(tools []ToolDef) []anthropicTool {
	result := []anthropicTool{}
	for _, t := range tools {
		result = append(result, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return result
}

func toAnthropicToolChoice(tc string) *anthropicToolChoice {
	switch tc {
	case "required", "any":
		return &anthropicToolChoice{Type: "any"}
	case "none":
		return &anthropicToolChoice{Type: "none"}
	default:
		return &anthropicToolChoice{Type: "auto"}
	}
}

func fromAnthropicResp(ar anthropicResp) (LLMResponse, error) {
	var content string
	var toolCalls []ToolCall
	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}
	return LLMResponse{
		Content:   strings.TrimSpace(content),
		ToolCalls: toolCalls,
		Raw:       &ChatMsg{Role: "assistant", Content: content, ToolCalls: toolCalls},
	}, nil
}

// ---------------------------------------------------------------------------
// Estructuras Anthropic
// ---------------------------------------------------------------------------

type anthropicReq struct {
	Model      string               `json:"model"`
	MaxTokens  int                  `json:"max_tokens"`
	System     string               `json:"system,omitempty"`
	Messages   []anthropicMsg       `json:"messages"`
	Tools      []anthropicTool      `json:"tools,omitempty"`
	ToolChoice *anthropicToolChoice `json:"tool_choice,omitempty"`
	Stream     bool                 `json:"stream,omitempty"`
}

type anthropicMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string | []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string          `json:"type"` // text | tool_use | tool_result
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"` // auto | any | none | tool
}

type anthropicResp struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type         string                 `json:"type"`
	Message      *anthropicResp         `json:"message,omitempty"`
	Index        int                    `json:"index,omitempty"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Delta        *anthropicStreamDelta  `json:"delta,omitempty"`
}

type anthropicStreamDelta struct {
	Type        string `json:"type"` // text_delta | input_json_delta
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/adapter"
)

type frameworkSessionStartResponse struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	Reasoning string         `json:"reasoning,omitempty"`
	AskVia    string         `json:"ask_via"`
	Chips     []string       `json:"chips,omitempty"`
	Events    []MessageEvent `json:"events,omitempty"`
}

func (s *server) startFrameworkSession(ctx context.Context, ch *adapter.Client, conv *Conversation, frameworkName string) (*Message, error) {
	cwd := filepath.Join(s.rootDir, "remora-flujo")
	args := []string{"run", "./cmd/framework_session", "start", "--framework", frameworkName, "--conv-id", conv.ID}
	if ctxB64 := encodeFrameworkSessionContext(conv.RuntimeContext); ctxB64 != "" {
		args = append(args, "--context-b64", ctxB64)
	}
	resp, err := ch.ExecuteCommand(ctx, "go", args, cwd)
	if err != nil {
		return nil, fmt.Errorf("%s session-start rpc: %w", frameworkName, err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s session-start channel error: %s", frameworkName, resp.Error)
	}
	if resp.ExitCode != 0 {
		detail := strings.TrimSpace(resp.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(resp.Stdout)
		}
		return nil, fmt.Errorf("%s session-start exit %d: %s", frameworkName, resp.ExitCode, detail)
	}
	out, err := parseFrameworkSessionOutput(resp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("%s session-start JSON inválido: %w; stdout=%s", frameworkName, err, resp.Stdout)
	}
	if strings.TrimSpace(out.Text) == "" {
		return nil, fmt.Errorf("%s session-start devolvió text vacío", frameworkName)
	}
	if out.ID == "" {
		out.ID = generateMessageID()
	}
	return &Message{
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      frameworkName,
		Content:        out.Text,
		Reasoning:      out.Reasoning,
		Status:         "needs_input",
		Events:         out.Events,
		QuestionID:     out.ID,
		AskVia:         out.AskVia,
		SuggestedChips: out.Chips,
		Timestamp:      time.Now(),
	}, nil
}

func (s *server) sendFrameworkSessionMessage(ctx context.Context, ch *adapter.Client, conv *Conversation, frameworkName, text string) (*Message, error) {
	cwd := filepath.Join(s.rootDir, "remora-flujo")
	msgB64 := base64.RawURLEncoding.EncodeToString([]byte(text))
	args := []string{"run", "./cmd/framework_session", "message", "--framework", frameworkName, "--conv-id", conv.ID, "--message-b64", msgB64, "--history", encodeRecentHistory(conv.ID, "")}
	if ctxB64 := encodeFrameworkSessionContext(conv.RuntimeContext); ctxB64 != "" {
		args = append(args, "--context-b64", ctxB64)
	}
	resp, err := ch.ExecuteCommand(ctx, "go", args, cwd)
	if err != nil {
		return nil, fmt.Errorf("%s session-message rpc: %w", frameworkName, err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s session-message channel error: %s", frameworkName, resp.Error)
	}
	if resp.ExitCode != 0 {
		detail := strings.TrimSpace(resp.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(resp.Stdout)
		}
		return nil, fmt.Errorf("%s session-message exit %d: %s", frameworkName, resp.ExitCode, detail)
	}
	out, err := parseFrameworkSessionOutput(resp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("%s session-message JSON inválido: %w; stdout=%s", frameworkName, err, resp.Stdout)
	}
	if strings.TrimSpace(out.Text) == "" {
		return nil, fmt.Errorf("%s session-message devolvió text vacío", frameworkName)
	}
	if out.ID == "" {
		out.ID = generateMessageID()
	}
	return &Message{
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      frameworkName,
		Content:        out.Text,
		Reasoning:      out.Reasoning,
		Status:         "needs_input",
		Events:         out.Events,
		QuestionID:     out.ID,
		AskVia:         out.AskVia,
		SuggestedChips: out.Chips,
		Timestamp:      time.Now(),
	}, nil
}

func encodeFrameworkSessionContext(ctx map[string]any) string {
	if len(ctx) == 0 {
		return ""
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseFrameworkSessionOutput(stdout string) (frameworkSessionStartResponse, error) {
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return frameworkSessionStartResponse{}, fmt.Errorf("stdout vacío")
	}
	var direct frameworkSessionStartResponse
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil && strings.TrimSpace(direct.Text) != "" {
		return direct, nil
	}

	var events []MessageEvent
	var final frameworkSessionStartResponse
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return frameworkSessionStartResponse{}, err
		}
		if _, ok := raw["text"]; ok {
			if err := json.Unmarshal([]byte(line), &final); err != nil {
				return frameworkSessionStartResponse{}, err
			}
			continue
		}
		var ev MessageEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return frameworkSessionStartResponse{}, err
		}
		if ev.Type != "" {
			events = append(events, ev)
		}
	}
	if strings.TrimSpace(final.Text) == "" {
		return frameworkSessionStartResponse{}, fmt.Errorf("JSONL sin respuesta final text")
	}
	if len(events) > 0 {
		final.Events = append(events, final.Events...)
	}
	return final, nil
}

func (s *server) getFrameworkSessionLiveEvents(w http.ResponseWriter, r *http.Request) {
	id := muxVar(r, "id")
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	if !s.requireConversationAccess(w, r, conv) {
		return
	}
	framework := strings.TrimSpace(r.URL.Query().Get("framework"))
	if framework == "" && len(conv.Frameworks) > 0 {
		framework = conv.Frameworks[0]
	}
	if framework == "" || strings.Contains(framework, "/") || strings.Contains(framework, "..") {
		writeErr(w, http.StatusBadRequest, "framework inválido")
		return
	}
	path := filepath.Join(s.rootDir, "framework-"+framework, "temp", "live_"+sanitizeLiveConvFile(conv.ID)+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		writeOK(w, map[string]interface{}{"events": []MessageEvent{}})
		return
	}
	defer f.Close()
	events := []MessageEvent{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev MessageEvent
		if json.Unmarshal(scanner.Bytes(), &ev) == nil && ev.Type != "" {
			ev.Message = redactLiveEventText(ev.Message)
			ev.Delta = redactLiveEventText(ev.Delta)
			events = append(events, ev)
		}
	}
	writeOK(w, map[string]interface{}{"events": events})
}

func sanitizeLiveConvFile(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func redactLiveEventText(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		low := strings.ToLower(line)
		if strings.Contains(low, "contraseña") || strings.Contains(low, "password") || strings.Contains(low, "token") || strings.Contains(low, "clave") {
			lines[i] = "[secreto oculto]"
		}
	}
	return strings.Join(lines, "\n")
}

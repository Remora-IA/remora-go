package nativeagent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"framework-paladin/paladin"
)

const apiURL = "https://api.minimax.io/anthropic/v1/messages"

type Agent struct {
	apiKey       string
	model        string
	role         string
	cwd          string
	sessionPath  string
	maxTurns     int
	maxTokens    int
	allowedTools map[string]bool
	traceCtx     *paladin.Context
	httpClient   *http.Client
}

type Options struct {
	APIKey       string
	Model        string
	Role         string
	CWD          string
	SessionPath  string
	MaxTurns     int
	AllowedTools []string
	Trace        *paladin.Context
}

func New(options Options) (*Agent, error) {
	apiKey := strings.TrimSpace(options.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("MINIMAX_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("REMORA_MINIMAX_API_KEY"))
	}
	if apiKey == "" {
		return nil, errors.New("falta MINIMAX_API_KEY o REMORA_MINIMAX_API_KEY para usar runtime Go nativo")
	}

	model := options.Model
	if model == "" {
		model = "MiniMax-M2.7"
	}
	cwd := options.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	maxTurns := options.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 20
	}

	allowedTools := map[string]bool{}
	for _, tool := range options.AllowedTools {
		allowedTools[tool] = true
	}

	return &Agent{
		apiKey:       apiKey,
		model:        model,
		role:         options.Role,
		cwd:          cwd,
		sessionPath:  options.SessionPath,
		maxTurns:     maxTurns,
		maxTokens:    4096,
		allowedTools: allowedTools,
		traceCtx:     options.Trace,
		httpClient:   &http.Client{Timeout: 180 * time.Second},
	}, nil
}

func (a *Agent) Prompt(prompt string) (string, error) {
	ctx := a.child("nativeagent.Prompt")
	if ctx != nil {
		defer ctx.End()
		ctx.Var("prompt_length", len(prompt))
		ctx.Var("session_path", a.sessionPath)
		ctx.Var("cwd", a.cwd)
	}
	messages, err := a.loadMessages()
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return "", err
	}
	if ctx != nil {
		ctx.Var("history_messages", len(messages))
	}
	messages = append(messages, Message{
		Role: "user",
		Content: []ContentBlock{{
			Type: "text",
			Text: prompt,
		}},
	})

	var visible strings.Builder
	for turn := 0; turn < a.maxTurns; turn++ {
		resp, err := a.request(messages)
		if err != nil {
			if ctx != nil {
				ctx.Error(err)
			}
			return visible.String(), err
		}
		messages = append(messages, Message{Role: "assistant", Content: resp.Content})

		toolResults := make([]ContentBlock, 0)
		textBlocks := 0
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				textBlocks++
				visible.WriteString(block.Text)
			case "tool_use":
				if ctx != nil {
					ctx.Decision("tool_use", block.Name)
					ctx.Var("tool_input_"+block.ID, string(block.Input))
				}
				result := a.runTool(block.Name, block.Input)
				if ctx != nil {
					ctx.Var("tool_result_"+block.ID, truncate(result, 1000))
				}
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   result,
				})
			}
		}
		if ctx != nil {
			ctx.Var("turn", turn)
			ctx.Var("assistant_blocks", len(resp.Content))
			ctx.Var("assistant_text_blocks", textBlocks)
			ctx.Var("tool_calls", len(toolResults))
		}
		if len(toolResults) == 0 {
			if err := a.saveMessages(messages); err != nil {
				if ctx != nil {
					ctx.Error(err)
				}
				return visible.String(), err
			}
			if ctx != nil {
				ctx.Var("visible_length", visible.Len())
				ctx.Decision("agent_done", "no tool calls")
			}
			return visible.String(), nil
		}
		messages = append(messages, Message{Role: "user", Content: toolResults})
	}

	if err := a.saveMessages(messages); err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return visible.String(), err
	}
	err = fmt.Errorf("agente excedio max_turns=%d", a.maxTurns)
	if ctx != nil {
		ctx.Error(err)
	}
	return visible.String(), err
}

func (a *Agent) request(messages []Message) (*Response, error) {
	ctx := a.child("nativeagent.request")
	if ctx != nil {
		defer ctx.End()
		ctx.Var("message_count", len(messages))
		ctx.Var("tool_count", len(a.tools()))
	}
	req := Request{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Messages:  messages,
		Tools:     a.tools(),
	}
	body, err := json.Marshal(req)
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	if ctx != nil {
		ctx.Var("request_bytes", len(body))
	}
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	if ctx != nil {
		ctx.Var("status_code", resp.StatusCode)
		ctx.Var("response_bytes", len(respBody))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("minimax HTTP %d: %s", resp.StatusCode, string(respBody))
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		if ctx != nil {
			ctx.Error(err)
			ctx.Var("response_preview", truncate(string(respBody), 1000))
		}
		return nil, err
	}
	if ctx != nil {
		ctx.Var("response_blocks", len(out.Content))
	}
	return &out, nil
}

func (a *Agent) runTool(name string, input json.RawMessage) string {
	if len(a.allowedTools) > 0 && !a.allowedTools[name] {
		return "error: herramienta no permitida para esta sesion: " + name
	}
	switch name {
	case "bash":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return "error: input invalido: " + err.Error()
		}
		return a.toolBash(args.Command)
	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return "error: input invalido: " + err.Error()
		}
		return a.toolRead(args.Path)
	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return "error: input invalido: " + err.Error()
		}
		return a.toolWrite(args.Path, args.Content)
	case "list_files":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return "error: input invalido: " + err.Error()
		}
		return a.toolList(args.Path)
	default:
		return "error: herramienta desconocida: " + name
	}
}

func (a *Agent) tools() []Tool {
	all := tools()
	if len(a.allowedTools) == 0 {
		return all
	}
	filtered := make([]Tool, 0, len(all))
	for _, tool := range all {
		if a.allowedTools[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func (a *Agent) toolBash(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "error: command vacio"
	}
	if err := a.validateBashPolicy(command); err != nil {
		return "policy_error: " + err.Error()
	}
	ctxCommand := exec.Command("/bin/zsh", "-lc", command)
	ctxCommand.Dir = a.cwd
	output, err := ctxCommand.CombinedOutput()
	text := truncate(string(output), 16000)
	if err != nil {
		return fmt.Sprintf("exit_error: %v\n%s", err, text)
	}
	return text
}

func (a *Agent) validateBashPolicy(command string) error {
	lower := strings.ToLower(command)
	if a.role == "echo" && strings.Contains(lower, "./frameworkecho validate") {
		if strings.Contains(lower, "respuesta pendiente") || strings.Contains(lower, "pending") {
			return errors.New("Echo no puede validar con una respuesta pendiente o inventada; debe usar una respuesta real del usuario")
		}
	}
	return nil
}

func (a *Agent) toolRead(path string) string {
	full, err := a.resolvePath(path)
	if err != nil {
		return "error: " + err.Error()
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "error: " + err.Error()
	}
	return truncate(string(data), 24000)
}

func (a *Agent) toolWrite(path, content string) string {
	full, err := a.resolvePath(path)
	if err != nil {
		return "error: " + err.Error()
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}

func (a *Agent) toolList(path string) string {
	full, err := a.resolvePath(path)
	if err != nil {
		return "error: " + err.Error()
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "error: " + err.Error()
	}
	var out strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			out.WriteString(entry.Name() + "/\n")
		} else {
			out.WriteString(entry.Name() + "\n")
		}
	}
	return out.String()
}

func (a *Agent) resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path vacio")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(a.cwd, path), nil
}

func (a *Agent) loadMessages() ([]Message, error) {
	if a.sessionPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(a.sessionPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (a *Agent) saveMessages(messages []Message) error {
	if a.sessionPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.sessionPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.sessionPath, append(data, '\n'), 0644)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "\n...[truncated]"
}

func (a *Agent) child(name string) *paladin.Context {
	if a == nil || a.traceCtx == nil {
		return nil
	}
	return a.traceCtx.Child(name)
}

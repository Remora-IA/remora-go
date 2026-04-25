package nativeagent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"framework-paladin/paladin"
)

const (
	providerGroq        = "groq"
	providerMiniMax     = "minimax"
	groqAPIURL          = "https://api.groq.com/openai/v1/chat/completions"
	minimaxAPIURL       = "https://api.minimax.io/anthropic/v1/messages"
	defaultGroqModel    = "meta-llama/llama-4-scout-17b-16e-instruct"
	defaultMiniMaxModel = "MiniMax-M2.7"
)

type Agent struct {
	provider     string
	providerNote string
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
	Provider     string
	APIKey       string
	Model        string
	Role         string
	CWD          string
	SessionPath  string
	MaxTurns     int
	AllowedTools []string
	Trace        *paladin.Context
}

type ImageInput struct {
	Path string
}

func New(options Options) (*Agent, error) {
	envFiles := loadDefaultEnvFiles()

	provider, apiKey, model, note, err := resolveProvider(options, envFiles)
	if err != nil {
		return nil, err
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
		provider:     provider,
		providerNote: note,
		apiKey:       apiKey,
		model:        model,
		role:         options.Role,
		cwd:          cwd,
		sessionPath:  options.SessionPath,
		maxTurns:     maxTurns,
		maxTokens:    8192,
		allowedTools: allowedTools,
		traceCtx:     options.Trace,
		httpClient:   &http.Client{Timeout: 180 * time.Second},
	}, nil
}

func resolveProvider(options Options, envFiles []string) (string, string, string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(options.Provider))
	providerSource := "options.Provider"
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(os.Getenv("REMORA_LLM_PROVIDER")))
		providerSource = "REMORA_LLM_PROVIDER"
	}
	if provider == "" {
		if firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY") != "" {
			provider = providerGroq
			providerSource = "groq key presente"
		} else {
			provider = providerMiniMax
			providerSource = "groq key ausente; fallback a minimax"
		}
	}

	switch provider {
	case providerGroq:
		apiKey := strings.TrimSpace(options.APIKey)
		if apiKey == "" {
			apiKey = firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY")
		}
		if apiKey == "" {
			return "", "", "", "", fmt.Errorf("falta GROQ_API_KEY o REMORA_GROQ_API_KEY para usar Groq; env_files=%s", strings.Join(envFiles, ","))
		}
		model := firstNonEmpty(options.Model, os.Getenv("REMORA_GROQ_MODEL"), defaultGroqModel)
		return providerGroq, apiKey, model, fmt.Sprintf("%s; env_files=%s", providerSource, strings.Join(envFiles, ",")), nil
	case providerMiniMax:
		apiKey := strings.TrimSpace(options.APIKey)
		if apiKey == "" {
			apiKey = firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY")
		}
		if apiKey == "" {
			return "", "", "", "", fmt.Errorf("groq no configurado y falta MINIMAX_API_KEY/REMORA_MINIMAX_API_KEY; env_files=%s", strings.Join(envFiles, ","))
		}
		model := firstNonEmpty(options.Model, os.Getenv("REMORA_MINIMAX_MODEL"), defaultMiniMaxModel)
		return providerMiniMax, apiKey, model, fmt.Sprintf("%s; env_files=%s", providerSource, strings.Join(envFiles, ",")), nil
	default:
		return "", "", "", "", fmt.Errorf("proveedor LLM no soportado: %s", provider)
	}
}

func (a *Agent) Provider() string {
	return a.provider
}

func (a *Agent) Model() string {
	return a.model
}

func (a *Agent) ProviderNote() string {
	return a.providerNote
}

func RuntimeInfo() (string, string, string, error) {
	envFiles := loadDefaultEnvFiles()
	provider, _, model, note, err := resolveProvider(Options{}, envFiles)
	return provider, model, note, err
}

func (a *Agent) Prompt(prompt string) (string, error) {
	return a.PromptWithImages(prompt, nil)
}

func (a *Agent) PromptWithImages(prompt string, images []ImageInput) (string, error) {
	ctx := a.child("nativeagent.Prompt")
	if ctx != nil {
		defer ctx.End()
		ctx.Var("prompt_length", len(prompt))
		ctx.Var("image_count", len(images))
		ctx.Var("session_path", a.sessionPath)
		ctx.Var("cwd", a.cwd)
	}
	imageBlocks, err := a.imageBlocks(images)
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return "", err
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
	content := []ContentBlock{{
		Type: "text",
		Text: prompt,
	}}
	content = append(content, imageBlocks...)
	messages = append(messages, Message{
		Role:    "user",
		Content: content,
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
			if command := shellCommandFromTextResponse(resp.Content); command != "" && a.allowedTools["bash"] {
				toolID := fmt.Sprintf("fallback_bash_%d", turn)
				input, _ := json.Marshal(map[string]string{"command": command})
				messages[len(messages)-1].Content = append(messages[len(messages)-1].Content, ContentBlock{
					Type:  "tool_use",
					ID:    toolID,
					Name:  "bash",
					Input: input,
				})
				if ctx != nil {
					ctx.Decision("text_command_fallback", command)
				}
				result := a.runTool("bash", input)
				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: toolID,
					Content:   result,
				})
			}
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
		ctx.Var("provider", a.provider)
		ctx.Var("model", a.model)
		ctx.Var("provider_reason", a.providerNote)
	}
	switch a.provider {
	case providerGroq:
		return a.requestGroq(ctx, messages)
	case providerMiniMax:
		return a.requestMiniMax(ctx, messages)
	default:
		return nil, fmt.Errorf("proveedor LLM no soportado: %s", a.provider)
	}
}

func (a *Agent) requestMiniMax(ctx *paladin.Context, messages []Message) (*Response, error) {
	if messagesContainImages(messages) {
		return nil, errors.New("el proveedor activo minimax no soporta imágenes en esta integración; usa Groq/Llama 4 Scout")
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
	httpReq, err := http.NewRequest("POST", minimaxAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	respBody, statusCode, err := a.doRequest(httpReq)
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	if ctx != nil {
		ctx.Var("status_code", statusCode)
		ctx.Var("response_bytes", len(respBody))
	}
	if statusCode < 200 || statusCode >= 300 {
		err := fmt.Errorf("minimax HTTP %d: %s", statusCode, string(respBody))
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

func (a *Agent) requestGroq(ctx *paladin.Context, messages []Message) (*Response, error) {
	groqMessages := toGroqMessages(messages)
	req := GroqRequest{
		Model:       a.model,
		MaxTokens:   a.maxTokens,
		Messages:    groqMessages,
		Tools:       toGroqTools(a.tools()),
		Temperature: 0,
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
	httpReq, err := http.NewRequest("POST", groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	respBody, statusCode, err := a.doRequest(httpReq)
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	if ctx != nil {
		ctx.Var("status_code", statusCode)
		ctx.Var("response_bytes", len(respBody))
	}
	if statusCode < 200 || statusCode >= 300 {
		err := fmt.Errorf("groq HTTP %d: %s", statusCode, string(respBody))
		if ctx != nil {
			ctx.Error(err)
		}
		return nil, err
	}
	var out GroqResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		if ctx != nil {
			ctx.Error(err)
			ctx.Var("response_preview", truncate(string(respBody), 1000))
		}
		return nil, err
	}
	resp := fromGroqResponse(out)
	if ctx != nil {
		ctx.Var("response_blocks", len(resp.Content))
	}
	return resp, nil
}

func shellCommandFromTextResponse(blocks []ContentBlock) string {
	var text strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	command := strings.TrimSpace(text.String())
	if command == "" {
		return ""
	}
	if strings.HasPrefix(command, "```") {
		lines := strings.Split(command, "\n")
		if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			command = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	lines := strings.Split(command, "\n")
	if len(lines) != 1 {
		return ""
	}
	if !looksLikeShellCommand(command) {
		return ""
	}
	return command
}

func looksLikeShellCommand(command string) bool {
	command = strings.TrimSpace(command)
	prefixes := []string{
		"cd ",
		"./",
		"go run ",
		"bash ",
		"mkdir ",
		"ls ",
		"cat ",
		"sed ",
		"rg ",
		"find ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

func (a *Agent) doRequest(req *http.Request) ([]byte, int, error) {
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func toGroqMessages(messages []Message) []GroqMessage {
	var out []GroqMessage
	for _, msg := range messages {
		var parts []GroqContentPart
		var toolCalls []GroqToolCall
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if strings.TrimSpace(block.Text) != "" {
					parts = append(parts, GroqContentPart{Type: "text", Text: block.Text})
				}
			case "image":
				if strings.TrimSpace(block.ImageURL) != "" {
					parts = append(parts, GroqContentPart{
						Type:     "image_url",
						ImageURL: &GroqImageURL{URL: block.ImageURL},
					})
				}
			case "tool_use":
				toolCalls = append(toolCalls, GroqToolCall{
					ID:   block.ID,
					Type: "function",
					Function: GroqToolCallPayload{
						Name:      block.Name,
						Arguments: string(block.Input),
					},
				})
			case "tool_result":
				if len(parts) > 0 || len(toolCalls) > 0 {
					out = append(out, GroqMessage{
						Role:      msg.Role,
						Content:   groqContent(parts),
						ToolCalls: toolCalls,
					})
					parts = nil
					toolCalls = nil
				}
				out = append(out, GroqMessage{
					Role:       "tool",
					ToolCallID: block.ToolUseID,
					Content:    block.Content,
				})
			}
		}
		if len(parts) > 0 || len(toolCalls) > 0 {
			out = append(out, GroqMessage{
				Role:      msg.Role,
				Content:   groqContent(parts),
				ToolCalls: toolCalls,
			})
		}
	}
	return out
}

func groqContent(parts []GroqContentPart) any {
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	return parts
}

func toGroqTools(tools []Tool) []GroqTool {
	out := make([]GroqTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, GroqTool{
			Type: "function",
			Function: GroqFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return out
}

func fromGroqResponse(out GroqResponse) *Response {
	resp := &Response{ID: out.ID, Role: "assistant"}
	if len(out.Choices) == 0 {
		return resp
	}
	msg := out.Choices[0].Message
	if text, ok := msg.Content.(string); ok && strings.TrimSpace(text) != "" {
		resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: text})
	}
	for _, call := range msg.ToolCalls {
		resp.Content = append(resp.Content, ContentBlock{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: json.RawMessage(firstNonEmpty(call.Function.Arguments, "{}")),
		})
	}
	return resp
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func loadDefaultEnvFiles() []string {
	candidates := []string{
		os.Getenv("REMORA_RUNTIME_ENV"),
		".env.local",
		"temp/runtime.env",
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, ".env.local"),
			filepath.Join(wd, "temp", "runtime.env"),
			filepath.Join(wd, "remora-flujo", ".env.local"),
			filepath.Join(wd, "remora-flujo", "temp", "runtime.env"),
		)
	}
	loaded := make([]string, 0)
	seen := map[string]bool{}
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if loadLocalEnv(path) == nil {
			if _, err := os.Stat(path); err == nil {
				loaded = append(loaded, path)
			}
		}
	}
	return loaded
}

func loadLocalEnv(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}

func (a *Agent) imageBlocks(images []ImageInput) ([]ContentBlock, error) {
	if len(images) == 0 {
		return nil, nil
	}
	if a.provider != providerGroq {
		return nil, errors.New("las imágenes solo están habilitadas con Groq/Llama 4 Scout en esta integración")
	}
	if len(images) > 5 {
		return nil, fmt.Errorf("Groq/Llama 4 Scout admite máximo 5 imágenes por mensaje; recibidas=%d", len(images))
	}
	blocks := make([]ContentBlock, 0, len(images))
	for _, image := range images {
		dataURL, err := a.imageDataURL(image.Path)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, ContentBlock{Type: "image", ImageURL: dataURL})
	}
	return blocks, nil
}

func (a *Agent) imageDataURL(path string) (string, error) {
	full, err := a.resolvePath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	const maxImageBytes = 20 * 1024 * 1024
	if len(data) > maxImageBytes {
		return "", fmt.Errorf("imagen excede 20MB: %s", full)
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(full)))
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("archivo no parece imagen: %s (%s)", full, mimeType)
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func messagesContainImages(messages []Message) bool {
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "image" {
				return true
			}
		}
	}
	return false
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
	if strings.Contains(lower, "go run ./cmd/flujo done ") {
		switch a.role {
		case "echo":
			if !strings.Contains(lower, "go run ./cmd/flujo done echo ") {
				return errors.New("Echo solo puede apagar/pasar mando para echo")
			}
		case "alfa":
			if !strings.Contains(lower, "go run ./cmd/flujo done alfa ") {
				return errors.New("Alfa solo puede ejecutar done alfa; para preguntar debe usar ask-echo --from alfa")
			}
		case "bravo":
			if !strings.Contains(lower, "go run ./cmd/flujo done bravo ") {
				return errors.New("Bravo solo puede ejecutar done bravo; para preguntar debe usar ask-echo --from bravo")
			}
		}
	}
	if strings.Contains(lower, "go run ./cmd/flujo ask-echo") {
		switch a.role {
		case "alfa":
			if !strings.Contains(lower, "--from alfa") {
				return errors.New("Alfa debe usar ask-echo --from alfa")
			}
		case "bravo":
			if !strings.Contains(lower, "--from bravo") {
				return errors.New("Bravo debe usar ask-echo --from bravo")
			}
		default:
			return errors.New("solo Alfa o Bravo pueden usar ask-echo")
		}
	}
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

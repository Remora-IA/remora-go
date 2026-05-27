package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultClaudeCLIModel = "claude-haiku-4-5-20251001"
	defaultClaudeCLITimeout = 90 * time.Second
)

// ClaudeCLIClient implementa Client invocando el binario `claude` como
// subprocess. Usa la autenticación OAuth de Claude Max plan (~/.claude/.credentials.json)
// — no requiere ANTHROPIC_API_KEY ni facturación adicional.
//
// Ventaja: si ya pagás Claude Max para Claude Code, podés usar el mismo
// plan para los agentes que construyas con Remora, sin costo marginal por
// mensaje.
//
// Limitación: depende de tener el binario `claude` instalado y autenticado.
// Está pensado para dev local y deploys con OAuth mounteado (Cloud Run, etc.).
// Para producción sin Max plan, usar NewAnthropic con API key directa.
type ClaudeCLIClient struct {
	cliPath string
	model   string
	timeout time.Duration
}

// NewClaudeCLI crea un cliente que invoca `claude` desde PATH. Si no
// encuentra el binario devuelve ErrNoCredentials para que el consumidor
// pueda caer a otra implementación.
func NewClaudeCLI() (*ClaudeCLIClient, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return nil, ErrNoCredentials
	}
	return &ClaudeCLIClient{
		cliPath: path,
		model:   defaultClaudeCLIModel,
		timeout: defaultClaudeCLITimeout,
	}, nil
}

// WithModel devuelve un cliente apuntando a otro modelo.
func (c *ClaudeCLIClient) WithModel(model string) *ClaudeCLIClient {
	cp := *c
	cp.model = model
	return &cp
}

// WithTimeout devuelve un cliente con timeout distinto para el subprocess.
func (c *ClaudeCLIClient) WithTimeout(d time.Duration) *ClaudeCLIClient {
	cp := *c
	cp.timeout = d
	return &cp
}

func (c *ClaudeCLIClient) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	// El CLI toma un único string como prompt. Concatenamos los Messages
	// en el formato Role: content.
	prompt := buildCLIPrompt(req.Messages)
	if prompt == "" {
		return nil, errors.New("llm/claudecli: empty prompt")
	}

	cmdCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.cliPath,
		"-p", prompt,
		"--system-prompt", req.System,
		"--model", model,
		"--output-format", "text",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("llm/claudecli: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	text := strings.TrimSpace(stdout.String())
	return &Response{
		Text:       text,
		StopReason: "end_turn",
	}, nil
}

func buildCLIPrompt(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	// Caso simple: un solo mensaje user → mandarlo directo.
	if len(messages) == 1 && messages[0].Role == "user" {
		return messages[0].Content
	}
	// Caso multi-turno: formatear como conversación.
	var b strings.Builder
	for _, m := range messages {
		role := m.Role
		if role == "user" {
			role = "Usuario"
		} else if role == "assistant" {
			role = "Asistente"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

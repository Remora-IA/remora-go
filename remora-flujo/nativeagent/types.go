package nativeagent

import "encoding/json"

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
}

type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type Response struct {
	Type    string         `json:"type"`
	ID      string         `json:"id"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

func tools() []Tool {
	return []Tool{
		{
			Name:        "bash",
			Description: "Ejecuta un comando shell en el directorio actual del flujo. Usalo para invocar los CLIs de Framework Echo/Alfa/Bravo.",
			InputSchema: objectSchema(map[string]any{
				"command": map[string]any{"type": "string"},
			}, []string{"command"}),
		},
		{
			Name:        "read_file",
			Description: "Lee un archivo local.",
			InputSchema: objectSchema(map[string]any{
				"path": map[string]any{"type": "string"},
			}, []string{"path"}),
		},
		{
			Name:        "write_file",
			Description: "Escribe/reemplaza un archivo local. Preferir CLIs de framework para estado estructurado cuando existan.",
			InputSchema: objectSchema(map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			}, []string{"path", "content"}),
		},
		{
			Name:        "list_files",
			Description: "Lista archivos en un directorio local.",
			InputSchema: objectSchema(map[string]any{
				"path": map[string]any{"type": "string"},
			}, []string{"path"}),
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

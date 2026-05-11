// Package adapter provee un cliente Go reusable para hablar con Channel
// vía JSON-RPC 2.0. Cualquier framework lo importa y obtiene poderes de
// terminal (execute, read, write, list, http_get) sin reimplementar nada.
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Response refleja exactamente el contrato fijo de Channel (Axioma 3).
type Response struct {
	Success    bool   `json:"success"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Error      string `json:"error"`
	DurationMs int64  `json:"duration_ms"`
}

// Client es un cliente JSON-RPC 2.0 hacia Channel.
type Client struct {
	BaseURL    string
	APIKey     string
	SessionID  string // opcional: si está set, Channel persiste en sessions/<id>.jsonl
	HTTPClient *http.Client
}

// New crea un cliente con timeout default de 300s (soporta operaciones largas como Mecánico apply-all sobre datasets grandes).
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// ExecuteCommand ejecuta un comando de la whitelist de Channel.
// cwd puede ser "" (default del proceso Channel) o un path dentro de BaseDir.
func (c *Client) ExecuteCommand(ctx context.Context, command string, args []string, cwd string) (*Response, error) {
	params := map[string]interface{}{
		"command": command,
		"args":    args,
	}
	if cwd != "" {
		params["cwd"] = cwd
	}
	return c.call(ctx, "execute_command", params)
}

// ReadFile lee un archivo dentro del BaseDir de Channel.
func (c *Client) ReadFile(ctx context.Context, path string) (*Response, error) {
	return c.call(ctx, "read_file", map[string]interface{}{"path": path})
}

// WriteFile escribe un archivo dentro del BaseDir de Channel.
func (c *Client) WriteFile(ctx context.Context, path, content string) (*Response, error) {
	return c.call(ctx, "write_file", map[string]interface{}{
		"path":    path,
		"content": content,
	})
}

// ListDir lista un directorio dentro del BaseDir de Channel.
func (c *Client) ListDir(ctx context.Context, path string) (*Response, error) {
	return c.call(ctx, "list_dir", map[string]interface{}{"path": path})
}

func (c *Client) Grep(ctx context.Context, path, pattern string, maxResults int) (*Response, error) {
	return c.call(ctx, "grep", map[string]interface{}{
		"path":        path,
		"pattern":     pattern,
		"max_results": maxResults,
	})
}

func (c *Client) Find(ctx context.Context, path, query string, maxResults int) (*Response, error) {
	return c.call(ctx, "find", map[string]interface{}{
		"path":        path,
		"query":       query,
		"max_results": maxResults,
	})
}

func (c *Client) EditFile(ctx context.Context, path, oldText, newText string, replaceAll bool) (*Response, error) {
	return c.call(ctx, "edit_file", map[string]interface{}{
		"path":        path,
		"old":         oldText,
		"new":         newText,
		"replace_all": replaceAll,
	})
}

// HTTPGet ejecuta un GET HTTP a través de Channel.
func (c *Client) HTTPGet(ctx context.Context, url string) (*Response, error) {
	return c.call(ctx, "http_get", map[string]interface{}{"url": url})
}

// call hace el POST JSON-RPC 2.0 y deserializa la respuesta.
func (c *Client) call(ctx context.Context, method string, params map[string]interface{}) (*Response, error) {
	if c.BaseURL == "" {
		return nil, errors.New("adapter: BaseURL is empty")
	}
	if c.APIKey == "" {
		return nil, errors.New("adapter: APIKey is empty")
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("adapter: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("adapter: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.APIKey)
	if c.SessionID != "" {
		httpReq.Header.Set("X-Session-ID", c.SessionID)
	}

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("adapter: http call: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("adapter: read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("adapter: decode response: %w (body=%s)", err, string(respBytes))
	}
	return &resp, nil
}

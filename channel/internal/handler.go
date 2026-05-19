package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errStopWalk = errors.New("stop walk")

// Handler procesa requests JSON-RPC 2.0 (Axioma 2 - Dumb Executor)
type Handler struct {
	BaseDir  string
	APIKeys  map[string]bool
	Timeout  time.Duration
	Sessions *SessionLogger
}

// NewHandler crea un handler stateless (Axioma 1)
func NewHandler(baseDir string, apiKeys []string) *Handler {
	keyMap := make(map[string]bool, len(apiKeys))
	for _, k := range apiKeys {
		keyMap[k] = true
	}
	absBase, _ := filepath.Abs(baseDir)
	timeout := 180 * time.Second
	if v := os.Getenv("CHANNEL_EXEC_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	return &Handler{
		BaseDir:  absBase,
		APIKeys:  keyMap,
		Timeout:  timeout,
		Sessions: NewSessionLogger(absBase),
	}
}

// Handle procesa el request HTTP (Axioma 9 - errores en JSON, no HTTP 5xx)
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json") // Axioma 3

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, "failed to read body", start)
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		LogJSONRPCError("parse error: " + err.Error())
		h.writeError(w, "invalid JSON", start)
		return
	}

	if valid, errMsg := ValidateJSONRPC(&req); !valid {
		LogJSONRPCError(errMsg)
		h.writeError(w, errMsg, start)
		return
	}

	// Axioma 4.1: Auth obligatoria
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" || !h.APIKeys[apiKey] {
		LogSecurityReject(req.Method, ObfuscateAPIKey(apiKey), "invalid API key")
		h.writeError(w, "unauthorized", start)
		return
	}

	// Axioma 8: solo métodos permitidos
	var resp Response
	var logCmd string
	var logArgs []string
	sessionID := r.Header.Get("X-Session-ID")
	switch req.Method {
	case "execute_command":
		resp, logCmd, logArgs = h.executeCommand(&req, sessionID, start)
	case "read_file":
		resp = h.readFile(&req, start)
	case "write_file":
		resp = h.writeFile(&req, start)
	case "list_dir":
		resp = h.listDir(&req, start)
	case "grep":
		resp = h.grep(&req, start)
	case "find":
		resp = h.find(&req, start)
	case "edit_file":
		resp = h.editFile(&req, start)
	case "http_get":
		resp = h.httpGet(&req, r, start)
	default:
		resp = NewErrorResponse("method not found", time.Since(start))
	}

	// Axioma 10: log estructurado
	LogRequest(req.Method, ObfuscateAPIKey(apiKey), logCmd, logArgs, h.BaseDir,
		resp.ExitCode, resp.DurationMs, resp.Success)

	// Persistencia de sesión (opcional vía X-Session-ID)
	h.Sessions.Append(r.Header.Get("X-Session-ID"), Entry{
		Method:     req.Method,
		Params:     req.Params,
		APIKeyHash: ObfuscateAPIKey(apiKey),
		Response:   resp,
	})

	w.WriteHeader(http.StatusOK) // Axioma 9: siempre 200
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, errMsg string, start time.Time) {
	resp := NewErrorResponse(errMsg, time.Since(start))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// execute_command: { command: string, args?: []string, cwd?: string } (Axiomas 4, 5, 7, 8)
// sessionID viene del header X-Session-ID y se propaga como REMORA_CONV_ID al
// proceso hijo, así los frameworks pueden escribir eventos live scope-ados
// a la conversación.
func (h *Handler) executeCommand(req *JSONRPCRequest, sessionID string, start time.Time) (Response, string, []string) {
	command, ok := req.Params["command"].(string)
	if !ok || command == "" {
		return NewErrorResponse("params.command must be a non-empty string", time.Since(start)), "", nil
	}

	var commandArgs []string
	if rawArgs, present := req.Params["args"]; present && rawArgs != nil {
		argsSlice, ok := rawArgs.([]interface{})
		if !ok {
			return NewErrorResponse("params.args must be an array of strings", time.Since(start)), command, nil
		}
		commandArgs = make([]string, len(argsSlice))
		for i, a := range argsSlice {
			s, ok := a.(string)
			if !ok {
				return NewErrorResponse("params.args must contain strings only", time.Since(start)), command, nil
			}
			commandArgs[i] = s
		}
	}

	var cwd string
	if rawCwd, present := req.Params["cwd"]; present && rawCwd != nil {
		cwdStr, ok := rawCwd.(string)
		if !ok {
			return NewErrorResponse("params.cwd must be a string", time.Since(start)), command, commandArgs
		}
		if cwdStr != "" {
			resolved, err := resolveWithinBase(cwdStr, h.BaseDir)
			if err != nil {
				LogSecurityReject(req.Method, "****", "cwd: "+err.Error())
				return NewErrorResponse("cwd: "+err.Error(), time.Since(start)), command, commandArgs
			}
			cwd = resolved
		}
	}

	if valid, errMsg := ValidateSecurity(command, commandArgs, h.BaseDir); !valid {
		LogSecurityReject(req.Method, "****", errMsg)
		return NewErrorResponse(errMsg, time.Since(start)), command, commandArgs
	}

	// Propagar conv_id al framework hijo vía env.
	var extraEnv map[string]string
	if sessionID != "" {
		extraEnv = map[string]string{"REMORA_CONV_ID": sessionID}
	}
	exitCode, stdout, stderr, err := ExecuteCommandWithEnv(command, commandArgs, cwd, extraEnv, h.Timeout)
	if err != nil {
		if err == context.DeadlineExceeded {
			return NewErrorResponse("timeout exceeded", time.Since(start)), command, commandArgs
		}
		return NewErrorResponse(err.Error(), time.Since(start)), command, commandArgs
	}
	// exit_code != 0 NO es error de Channel (Axioma 12 - bytes opacos)
	return NewSuccessResponse(stdout, stderr, exitCode, time.Since(start)), command, commandArgs
}

func (h *Handler) readFile(req *JSONRPCRequest, start time.Time) Response {
	path, ok := req.Params["path"].(string)
	if !ok {
		return NewErrorResponse("params.path must be a string", time.Since(start))
	}
	full, err := resolveWithinBase(path, h.BaseDir)
	if err != nil {
		LogSecurityReject(req.Method, "****", err.Error())
		return NewErrorResponse(err.Error(), time.Since(start))
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return NewErrorResponse("read error: "+err.Error(), time.Since(start))
	}
	return NewSuccessResponse(string(content), "", 0, time.Since(start))
}

func (h *Handler) writeFile(req *JSONRPCRequest, start time.Time) Response {
	path, ok := req.Params["path"].(string)
	if !ok {
		return NewErrorResponse("params.path must be a string", time.Since(start))
	}
	content, ok := req.Params["content"].(string)
	if !ok {
		return NewErrorResponse("params.content must be a string", time.Since(start))
	}
	full, err := resolveWithinBase(path, h.BaseDir)
	if err != nil {
		LogSecurityReject(req.Method, "****", err.Error())
		return NewErrorResponse(err.Error(), time.Since(start))
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		return NewErrorResponse("write error: "+err.Error(), time.Since(start))
	}
	return NewSuccessResponse("file written", "", 0, time.Since(start))
}

func (h *Handler) listDir(req *JSONRPCRequest, start time.Time) Response {
	path, ok := req.Params["path"].(string)
	if !ok {
		return NewErrorResponse("params.path must be a string", time.Since(start))
	}
	full, err := resolveWithinBase(path, h.BaseDir)
	if err != nil {
		LogSecurityReject(req.Method, "****", err.Error())
		return NewErrorResponse(err.Error(), time.Since(start))
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return NewErrorResponse("read error: "+err.Error(), time.Since(start))
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return NewSuccessResponse(strings.Join(names, "\n"), "", 0, time.Since(start))
}

func (h *Handler) grep(req *JSONRPCRequest, start time.Time) Response {
	pattern, ok := req.Params["pattern"].(string)
	if !ok || pattern == "" {
		return NewErrorResponse("params.pattern must be a non-empty string", time.Since(start))
	}
	path := "."
	if raw, present := req.Params["path"]; present && raw != nil {
		var ok bool
		path, ok = raw.(string)
		if !ok {
			return NewErrorResponse("params.path must be a string", time.Since(start))
		}
	}
	maxResults := intParam(req.Params, "max_results", 200)
	full, err := resolveWithinBase(path, h.BaseDir)
	if err != nil {
		LogSecurityReject(req.Method, "****", err.Error())
		return NewErrorResponse(err.Error(), time.Since(start))
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return NewErrorResponse("invalid pattern: "+err.Error(), time.Since(start))
	}
	var matches []string
	err = walkReadableFiles(full, func(file string, data []byte) bool {
		rel, _ := filepath.Rel(h.BaseDir, file)
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				matches = append(matches, rel+":"+itoa(i+1)+":"+line)
				if len(matches) >= maxResults {
					return false
				}
			}
		}
		return len(matches) < maxResults
	})
	if err != nil {
		return NewErrorResponse("grep error: "+err.Error(), time.Since(start))
	}
	return NewSuccessResponse(strings.Join(matches, "\n"), "", 0, time.Since(start))
}

func (h *Handler) find(req *JSONRPCRequest, start time.Time) Response {
	path := "."
	if raw, present := req.Params["path"]; present && raw != nil {
		var ok bool
		path, ok = raw.(string)
		if !ok {
			return NewErrorResponse("params.path must be a string", time.Since(start))
		}
	}
	query := ""
	if raw, present := req.Params["query"]; present && raw != nil {
		var ok bool
		query, ok = raw.(string)
		if !ok {
			return NewErrorResponse("params.query must be a string", time.Since(start))
		}
	}
	maxResults := intParam(req.Params, "max_results", 200)
	full, err := resolveWithinBase(path, h.BaseDir)
	if err != nil {
		LogSecurityReject(req.Method, "****", err.Error())
		return NewErrorResponse(err.Error(), time.Since(start))
	}
	var matches []string
	err = filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && shouldSkipDir(d.Name()) && path != full {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(h.BaseDir, path)
		if query == "" || strings.Contains(d.Name(), query) || strings.Contains(rel, query) {
			matches = append(matches, rel)
			if len(matches) >= maxResults {
				return errStopWalk
			}
		}
		return nil
	})
	if err != nil && err != errStopWalk {
		return NewErrorResponse("find error: "+err.Error(), time.Since(start))
	}
	sort.Strings(matches)
	return NewSuccessResponse(strings.Join(matches, "\n"), "", 0, time.Since(start))
}

func (h *Handler) editFile(req *JSONRPCRequest, start time.Time) Response {
	path, ok := req.Params["path"].(string)
	if !ok || path == "" {
		return NewErrorResponse("params.path must be a non-empty string", time.Since(start))
	}
	oldText, ok := req.Params["old"].(string)
	if !ok || oldText == "" {
		return NewErrorResponse("params.old must be a non-empty string", time.Since(start))
	}
	newText, ok := req.Params["new"].(string)
	if !ok {
		return NewErrorResponse("params.new must be a string", time.Since(start))
	}
	full, err := resolveWithinBase(path, h.BaseDir)
	if err != nil {
		LogSecurityReject(req.Method, "****", err.Error())
		return NewErrorResponse(err.Error(), time.Since(start))
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return NewErrorResponse("read error: "+err.Error(), time.Since(start))
	}
	content := string(data)
	if !strings.Contains(content, oldText) {
		return NewErrorResponse("old text not found", time.Since(start))
	}
	count := 1
	if v, _ := req.Params["replace_all"].(bool); v {
		count = -1
	}
	updated := strings.Replace(content, oldText, newText, count)
	if err := os.WriteFile(full, []byte(updated), 0644); err != nil {
		return NewErrorResponse("write error: "+err.Error(), time.Since(start))
	}
	return NewSuccessResponse("file edited", "", 0, time.Since(start))
}

func (h *Handler) httpGet(req *JSONRPCRequest, r *http.Request, start time.Time) Response {
	url, ok := req.Params["url"].(string)
	if !ok {
		return NewErrorResponse("params.url must be a string", time.Since(start))
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return NewErrorResponse("url must start with http:// or https://", time.Since(start))
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.Timeout)
	defer cancel()

	outReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return NewErrorResponse("invalid url: "+err.Error(), time.Since(start))
	}
	client := &http.Client{Timeout: h.Timeout}
	resp, err := client.Do(outReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return NewErrorResponse("timeout exceeded", time.Since(start))
		}
		return NewErrorResponse("http error: "+err.Error(), time.Since(start))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewErrorResponse("read response error: "+err.Error(), time.Since(start))
	}
	return NewSuccessResponse(string(body), "", resp.StatusCode, time.Since(start))
}

// resolveWithinBase resuelve un path y verifica que quede DENTRO de baseDir
// tras resolver symlinks (Axioma 4.3).
func resolveWithinBase(path, baseDir string) (string, error) {
	if path == "" {
		return "", pathError("path is required")
	}
	if strings.Contains(path, "..") {
		return "", pathError("path contains '..'")
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", pathError("invalid base_dir")
	}
	full := path
	if !filepath.IsAbs(path) {
		full = filepath.Join(absBase, path)
	}
	full = filepath.Clean(full)

	resolved := full
	if rs, err := filepath.EvalSymlinks(full); err == nil {
		resolved = rs
	} else if parent, errP := filepath.EvalSymlinks(filepath.Dir(full)); errP == nil {
		resolved = filepath.Join(parent, filepath.Base(full))
	}

	rel, err := filepath.Rel(absBase, resolved)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", pathError("path escapes base_dir")
	}
	return resolved, nil
}

func intParam(params map[string]interface{}, key string, def int) int {
	raw, ok := params[key]
	if !ok || raw == nil {
		return def
	}
	switch v := raw.(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return def
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func walkReadableFiles(root string, visit func(file string, data []byte) bool) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(root)
		if err != nil {
			return err
		}
		visit(root, data)
		return nil
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || strings.ContainsRune(string(data), 0) {
			return nil
		}
		if !visit(path, data) {
			return errStopWalk
		}
		return nil
	})
	if err == errStopWalk {
		return nil
	}
	return err
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", ".cache", "tmp", "temp":
		return true
	default:
		return false
	}
}

type pathError string

func (e pathError) Error() string { return string(e) }

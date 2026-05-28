package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/remora-go/framework-paladin/paladin"
)

const (
	defaultPort         = "8099"
	minimaxAPIURL       = "https://api.minimax.io/anthropic/v1/messages"
	openRouterAPIURL    = "https://openrouter.ai/api/v1/chat/completions"
	groqAPIURL          = "https://api.groq.com/openai/v1/chat/completions"
	defaultMiniMaxModel = "MiniMax-M2.7"
	defaultOAIModel     = "meta-llama/llama-4-scout-17b-16e-instruct"
)

// TraceEntry guarda un trace recibido y su flow traducido.
type TraceEntry struct {
	TraceID    string          `json:"trace_id"`
	Framework  string          `json:"framework"`
	ReceivedAt time.Time       `json:"received_at"`
	TraceJSON  json.RawMessage `json:"trace_json"`
	FlowResult *paladin.FlowResult `json:"flow_result,omitempty"`
	Processing bool            `json:"processing"`
	mu         sync.Mutex
}

// Server maneja las requests HTTP.
type Server struct {
	port    string
	traces  map[string]*TraceEntry
	mu      sync.RWMutex
	httpClient *http.Client
	llmProvider string
	llmAPIKey   string
	llmModel    string
	llmAPIURL   string
}

func main() {
	port := os.Getenv("PALADIN_PORT")
	if port == "" {
		port = defaultPort
	}

	// Cargar .env si existe
	loadEnv(".env")
	loadEnv(".env.local")

	provider, apiKey, model, apiURL := resolvePaladinProvider()

	srv := &Server{
		port:        port,
		traces:      make(map[string]*TraceEntry),
		httpClient:  &http.Client{Timeout: 90 * time.Second},
		llmProvider: provider,
		llmAPIKey:   apiKey,
		llmModel:    model,
		llmAPIURL:   apiURL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.health)
	mux.HandleFunc("POST /trace", srv.postTrace)
	mux.HandleFunc("GET /flow/{trace_id}", srv.getFlow)
	mux.HandleFunc("POST /ask", srv.postAsk)
	mux.HandleFunc("GET /traces", srv.listTraces)

	addr := ":" + port
	fmt.Printf("========================================\n")
	fmt.Printf(" Paladin Server\n")
	fmt.Printf(" Puerto:  %s\n", port)
	fmt.Printf(" LLM:    %s / %s (%s)\n", provider, model, apiKeyPrefix(apiKey))
	fmt.Printf("========================================\n\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (srv *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (srv *Server) postTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		Framework string          `json:"framework"`
		TraceJSON json.RawMessage `json:"trace_json"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	traceID := fmt.Sprintf("pal_%d", time.Now().UnixNano())

	entry := &TraceEntry{
		TraceID:    traceID,
		Framework:  payload.Framework,
		ReceivedAt: time.Now(),
		TraceJSON:  payload.TraceJSON,
		Processing: true,
	}

	srv.mu.Lock()
	srv.traces[traceID] = entry
	srv.mu.Unlock()

	// Traducir async
	go srv.translateTrace(entry)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"trace_id": traceID,
		"status":   "processing",
	})
}

func (srv *Server) getFlow(w http.ResponseWriter, r *http.Request) {
	traceID := strings.TrimPrefix(r.URL.Path, "/flow/")
	traceID = strings.Split(traceID, "?")[0]

	srv.mu.RLock()
	entry, exists := srv.traces[traceID]
	srv.mu.RUnlock()

	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "trace not found"})
		return
	}

	if entry.Processing {
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "processing",
			"message": "translation in progress",
		})
		return
	}

	if entry.FlowResult != nil {
		writeJSON(w, http.StatusOK, entry.FlowResult)
	} else {
		writeJSON(w, http.StatusOK, map[string]string{
			"trace_id": traceID,
			"message":  "no flow result available",
		})
	}
}

func (srv *Server) postAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		TraceID  string `json:"trace_id"`
		Question string `json:"question"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	srv.mu.RLock()
	entry, exists := srv.traces[payload.TraceID]
	srv.mu.RUnlock()

	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "trace not found"})
		return
	}

	answer, err := srv.askQuestion(entry, payload.Question)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"answer": answer})
}

func (srv *Server) listTraces(w http.ResponseWriter, r *http.Request) {
	srv.mu.RLock()
	defer srv.mu.RUnlock()

	traces := make([]map[string]any, 0, len(srv.traces))
	for id, entry := range srv.traces {
		traces = append(traces, map[string]any{
			"trace_id":   id,
			"framework":  entry.Framework,
			"processing": entry.Processing,
			"received":   entry.ReceivedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"traces": traces})
}

func (srv *Server) translateTrace(entry *TraceEntry) {
	if srv.llmAPIKey == "" {
		entry.mu.Lock()
		entry.Processing = false
		entry.FlowResult = &paladin.FlowResult{
			TraceID:     entry.TraceID,
			Framework:   entry.Framework,
			FlowNarrative: "LLM API key no configurada - traducción deshabilitada",
		}
		entry.mu.Unlock()
		return
	}

	prompt := buildTranslationPrompt(entry.TraceJSON)

	result, err := srv.callMiniMax(prompt)
	if err != nil {
		fmt.Printf("[PALADIN] Error traduciendo trace %s: %v\n", entry.TraceID, err)
		entry.mu.Lock()
		entry.Processing = false
		entry.FlowResult = &paladin.FlowResult{
			TraceID:       entry.TraceID,
			Framework:    entry.Framework,
			FlowNarrative: fmt.Sprintf("Error en traducción: %v", err),
		}
		entry.mu.Unlock()
		return
	}

	entry.mu.Lock()
	entry.Processing = false
	entry.FlowResult = result
	entry.mu.Unlock()

	fmt.Printf("[PALADIN] Trace %s traducido\n", entry.TraceID)
}

func buildTranslationPrompt(traceJSON json.RawMessage) string {
	var prettyJSON string
	if len(traceJSON) > 0 {
		var data any
		if json.Unmarshal(traceJSON, &data) == nil {
			if b, err := json.MarshalIndent(data, "", "  "); err == nil {
				prettyJSON = string(b)
			}
		}
	}

	if prettyJSON == "" {
		prettyJSON = string(traceJSON)
	}

	return `Eres un traductor de traces de código a flows de negocio narrados.
Recibirás un trace JSON de un framework.
Tu trabajo es:
1. Leer el trace y entender qué está haciendo el código
2. Traducirlo a pseudocódigo/narrativa que un humano pueda entender
3. Identificar qué reglas de negocio se están siguiendo o violando
4. Detectar patrones sospechosos (ej: ofreciendo solución antes de confirmar dolor)

Formato de salida OBLIGATORIO (JSON exacto, sin texto adicional):
{
  "flow_narrative": "narrativa en español, clara y concisa",
  "business_rules_detected": ["lista de reglas detectadas con ✓ o ✗"],
  "potential_issues": ["patrones que necesitan atención"],
  "summary": "resumen de una línea"
}

Trace JSON:
` + prettyJSON
}

func (srv *Server) callLLMText(prompt string, maxTokens int) (string, error) {
	switch srv.llmProvider {
	case "openrouter", "groq":
		return srv.callOAICompat(prompt, maxTokens)
	case "minimax":
		return srv.callMiniMaxText(prompt, maxTokens)
	default:
		return "", fmt.Errorf("provider %q no soportado", srv.llmProvider)
	}
}

func (srv *Server) callOAICompat(prompt string, maxTokens int) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body, _ := json.Marshal(map[string]any{
		"model":       srv.llmModel,
		"max_tokens":  maxTokens,
		"temperature": 0.2,
		"messages":    []msg{{Role: "user", Content: prompt}},
	})
	req, err := http.NewRequest("POST", srv.llmAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+srv.llmAPIKey)
	resp, err := srv.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llm parse: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm: respuesta sin choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (srv *Server) callMiniMaxText(prompt string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      srv.llmModel,
		"max_tokens": maxTokens,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	})
	req, err := http.NewRequest("POST", srv.llmAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", srv.llmAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	resp, err := srv.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("minimax HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var miniResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &miniResp); err != nil {
		return "", fmt.Errorf("minimax parse: %w", err)
	}
	if len(miniResp.Content) == 0 {
		return "", fmt.Errorf("respuesta vacía de LLM")
	}
	return miniResp.Content[0].Text, nil
}

func (srv *Server) callMiniMax(prompt string) (*paladin.FlowResult, error) {
	text, err := srv.callLLMText(prompt, 4096)
	if err != nil {
		return nil, err
	}

	var flowResult paladin.FlowResult
	if err := json.Unmarshal([]byte(text), &flowResult); err != nil {
		cleaned := extractJSON(text)
		if err := json.Unmarshal([]byte(cleaned), &flowResult); err != nil {
			return nil, fmt.Errorf("no se pudo parsear flow result: %w", err)
		}
	}

	return &flowResult, nil
}

func (srv *Server) askQuestion(entry *TraceEntry, question string) (string, error) {
	if srv.llmAPIKey == "" {
		return "LLM API key no configurada", nil
	}

	var narrative string
	if entry.FlowResult != nil {
		narrative = entry.FlowResult.FlowNarrative
	} else {
		narrative = string(entry.TraceJSON)
	}

	prompt := fmt.Sprintf(`Contexto del trace:
%s

Pregunta: %s

Responde en español, directamente.`, narrative, question)

	text, err := srv.callLLMText(prompt, 2048)
	if err != nil {
		return "", err
	}
	return text, nil
}

func loadEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
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
			os.Setenv(key, value)
		}
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func resolvePaladinProvider() (provider, apiKey, model, apiURL string) {
	p := strings.ToLower(strings.TrimSpace(os.Getenv("PALADIN_LLM_PROVIDER")))
	if p == "" {
		p = strings.ToLower(strings.TrimSpace(os.Getenv("REMORA_LLM_PROVIDER")))
	}
	if p == "" {
		if firstEnv("OPENROUTER_API_KEY") != "" {
			p = "openrouter"
		} else if firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY") != "" {
			p = "groq"
		} else {
			p = "minimax"
		}
	}
	switch p {
	case "openrouter":
		return p, firstEnv("OPENROUTER_API_KEY"), firstEnv("PALADIN_LLM_MODEL", defaultOAIModel), openRouterAPIURL
	case "groq":
		return p, firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY"), firstEnv("PALADIN_LLM_MODEL", defaultOAIModel), groqAPIURL
	default:
		return "minimax", firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY"), firstEnv("PALADIN_LLM_MODEL", defaultMiniMaxModel), minimaxAPIURL
	}
}

func apiKeyPrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "***"
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	// Buscar primer {
	start := strings.Index(text, "{")
	if start == -1 {
		return text
	}
	// Buscar último } después del primer {
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return text
}
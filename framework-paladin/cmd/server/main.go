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

	"github.com/Remora-IA/remora-go/framework-paladin/paladin"
)

const (
	defaultPort      = "8099"
	minimaxAPIURL    = "https://api.minimax.io/anthropic/v1/messages"
	defaultMiniMaxModel = "MiniMax-M2.7"
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
	minimaxAPIKey string
	minimaxModel string
}

func main() {
	port := os.Getenv("PALADIN_PORT")
	if port == "" {
		port = defaultPort
	}

	// Cargar .env si existe
	loadEnv(".env")
	loadEnv(".env.local")

	apiKey := firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY")
	model := firstEnv("MINIMAX_MODEL", "REMORA_MINIMAX_MODEL", defaultMiniMaxModel)

	srv := &Server{
		port:         port,
		traces:       make(map[string]*TraceEntry),
		httpClient:   &http.Client{Timeout: 60 * time.Second},
		minimaxAPIKey: apiKey,
		minimaxModel:  model,
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
	fmt.Printf(" MiniMax: %s (%s)\n", model, apiKeyPrefix(apiKey))
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
	if srv.minimaxAPIKey == "" {
		entry.mu.Lock()
		entry.Processing = false
		entry.FlowResult = &paladin.FlowResult{
			TraceID:     entry.TraceID,
			Framework:   entry.Framework,
			FlowNarrative: "MINIMAX_API_KEY no configurada - traducción deshabilitada",
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

func (srv *Server) callMiniMax(prompt string) (*paladin.FlowResult, error) {
	messages := []map[string]any{
		{"role": "user", "content": prompt},
	}

	reqBody := map[string]any{
		"model":     srv.minimaxModel,
		"max_tokens": 4096,
		"messages":  messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", minimaxAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", srv.minimaxAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := srv.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("minimax HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var miniResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &miniResp); err != nil {
		return nil, fmt.Errorf("error parseando respuesta: %w - body: %s", err, string(respBody))
	}

	if len(miniResp.Content) == 0 {
		return nil, fmt.Errorf("respuesta vacía de MiniMax")
	}

	text := miniResp.Content[0].Text

	// Parsear JSON de la respuesta
	var flowResult paladin.FlowResult
	if err := json.Unmarshal([]byte(text), &flowResult); err != nil {
		// Intentar extraer de markdown code block
		cleaned := extractJSON(text)
		if err := json.Unmarshal([]byte(cleaned), &flowResult); err != nil {
			return nil, fmt.Errorf("no se pudo parsear flow result: %w", err)
		}
	}

	return &flowResult, nil
}

func (srv *Server) askQuestion(entry *TraceEntry, question string) (string, error) {
	if srv.minimaxAPIKey == "" {
		return "MINIMAX_API_KEY no configurada", nil
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

	messages := []map[string]any{
		{"role": "user", "content": prompt},
	}

	reqBody := map[string]any{
		"model":      srv.minimaxModel,
		"max_tokens": 2048,
		"messages":   messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", minimaxAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", srv.minimaxAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := srv.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

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
		return "", fmt.Errorf("error parseando respuesta: %w", err)
	}

	if len(miniResp.Content) == 0 {
		return "", fmt.Errorf("respuesta vacía de MiniMax")
	}

	return miniResp.Content[0].Text, nil
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
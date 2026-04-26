package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"remora-flujo/handoff"
)

const (
	apiPort      = ":8084"
	apiBase      = "/api/v1"
	convDir      = "temp/api_conversations"
	statePath    = "temp/handoff/state.json"
	sessionPath  = "temp/sessions/echo/native.json"
)

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Event     string    `json:"event,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func main() {
	os.MkdirAll(convDir, 0755)
	os.MkdirAll("temp/handoff", 0755)
	os.MkdirAll(filepath.Dir(sessionPath), 0755)

	r := mux.NewRouter()

	// Apply CORS globally
	r.Use(corsMiddleware)

	r.HandleFunc("/health", healthHandler)

	// Conversations
	r.HandleFunc(apiBase+"/conversations", listConversations).Methods("GET")
	r.HandleFunc(apiBase+"/conversations", createConversation).Methods("POST")
	r.HandleFunc(apiBase+"/conversations/{id}", getConversation).Methods("GET")
	r.HandleFunc(apiBase+"/conversations/{id}", deleteConversation).Methods("DELETE")

	// Messages
	r.HandleFunc(apiBase+"/conversations/{id}/messages", getMessages).Methods("GET")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", sendMessage).Methods("POST")

	// Status
	r.HandleFunc(apiBase+"/conversations/{id}/status", getStatus).Methods("GET")
	r.HandleFunc(apiBase+"/echo/readiness", getEchoReadiness).Methods("GET")

	// CORS preflight handler for all API routes
	r.HandleFunc(apiBase+"/conversations", handleCORS).Methods("OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}", handleCORS).Methods("OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", handleCORS).Methods("OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/status", handleCORS).Methods("OPTIONS")
	r.HandleFunc(apiBase+"/echo/readiness", handleCORS).Methods("OPTIONS")

	fmt.Printf("🚀 Flujo API en http://localhost%s%s\n", apiPort, apiBase)
	fmt.Println("   Endpoints:")
	fmt.Println("   GET  /health")
	fmt.Println("   GET  /api/v1/conversations")
	fmt.Println("   POST /api/v1/conversations")
	fmt.Println("   GET  /api/v1/conversations/{id}")
	fmt.Println("   DELETE /api/v1/conversations/{id}")
	fmt.Println("   GET  /api/v1/conversations/{id}/messages")
	fmt.Println("   POST /api/v1/conversations/{id}/messages")
	fmt.Println("   GET  /api/v1/conversations/{id}/status")
	fmt.Println("   GET  /api/v1/echo/readiness")

	if err := http.ListenAndServe(apiPort, r); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers for all responses
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Set content type for all responses
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func handleCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, APIResponse{Success: false, Error: msg})
}

func generateID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// Handlers

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"status": "ok"}})
}

func listConversations(w http.ResponseWriter, r *http.Request) {
	var convs []Conversation
	entries, _ := os.ReadDir(convDir)
	for _, e := range entries {
		if c, err := loadConvMeta(e.Name()); err == nil {
			convs = append(convs, *c)
		}
	}
	if convs == nil {
		convs = []Conversation{}
	}
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: convs})
}

func createConversation(w http.ResponseWriter, r *http.Request) {
	var req struct{ Title string }
	json.NewDecoder(r.Body).Decode(&req)

	convID := fmt.Sprintf("conv_%d", time.Now().UnixNano())
	conv := &Conversation{
		ID:        convID,
		Title:     req.Title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	saveConvMeta(conv)
	saveMessages(convID, []Message{})

	// Reset handoff state
	handoff.Save(statePath, handoff.NewState())

	// Clear Echo session for new conversation
	os.RemoveAll("temp/sessions/echo")
	os.MkdirAll("temp/sessions/echo", 0755)

	// Iniciar Echo con prompt inicial (async)
	go func() {
		runEchoInit(convID)
	}()

	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: conv})
}

func getConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := loadConvMeta(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	msgs, _ := loadMessages(conv.ID)
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{
		"conversation": conv,
		"messages":    msgs,
	}})
}

func deleteConversation(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := loadConvMeta(id); err != nil {
		writeError(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	os.RemoveAll(filepath.Join(convDir, id))
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"deleted": id}})
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	convID := mux.Vars(r)["id"]
	if _, err := loadConvMeta(convID); err != nil {
		writeError(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	msgs, _ := loadMessages(convID)
	var visible []Message
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "echo" {
			visible = append(visible, m)
		}
	}
	if visible == nil {
		visible = []Message{}
	}
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: visible})
}

func sendMessage(w http.ResponseWriter, r *http.Request) {
	convID := mux.Vars(r)["id"]
	if _, err := loadConvMeta(convID); err != nil {
		writeError(w, http.StatusNotFound, "conversación no encontrada")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "mensaje vacío")
		return
	}

	msgs, _ := loadMessages(convID)

	// Agregar mensaje del usuario
	userMsg := Message{
		ID:        generateID(),
		Role:      "user",
		Content:   req.Content,
		Timestamp: time.Now(),
	}
	msgs = append(msgs, userMsg)
	saveMessages(convID, msgs)

	// Procesar con Echo
	echoResp, event := processWithEcho(req.Content, msgs, convID)

	var echoMsg *Message
	if echoResp != "" {
		echoMsg = &Message{
			ID:        generateID(),
			Role:      "echo",
			Content:   echoResp,
			Event:     event,
			Timestamp: time.Now(),
		}
		msgs = append(msgs, *echoMsg)
		saveMessages(convID, msgs)
	}

	if echoMsg != nil {
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: echoMsg})
	} else {
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{
			"message": userMsg, "handoff_event": event,
		}})
	}
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	_ = mux.Vars(r)["id"]
	state, _ := handoff.Load(statePath)

	cmd := exec.Command("/bin/zsh", "-lc", "cd /Users/alcless_a1234_cursor/remora-go/framework-echo && ./frameworkecho readiness")
	out, _ := cmd.CombinedOutput()

	data := map[string]interface{}{
		"echo_status":    string(state.Roles[handoff.RoleEcho].Status),
		"alfa_status":    string(state.Roles[handoff.RoleAlfa].Status),
		"bravo_status":   string(state.Roles[handoff.RoleBravo].Status),
		"ready_for_alfa": strings.Contains(string(out), "ready_for_alfa: true"),
	}

	if last, ok := state.LastEvent(); ok {
		data["last_event"] = string(last.Type)
		data["last_message"] = last.Message
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

func getEchoReadiness(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("/bin/zsh", "-lc", "cd /Users/alcless_a1234_cursor/remora-go/framework-echo && ./frameworkecho readiness && ./frameworkecho show-tree")
	out, _ := cmd.CombinedOutput()
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"readiness": string(out)}})
}

// Echo Processing

func runEchoInit(convID string) {
	// Ejecutar comandos iniciales de Echo para verificar estado
	cmd := exec.Command("/bin/zsh", "-lc",
		"cd /Users/alcless_a1234_cursor/remora-go/framework-echo && ./frameworkecho status && ./frameworkecho show-tree && ./frameworkecho readiness && ./frameworkecho config")
	out, _ := cmd.CombinedOutput()
	fmt.Printf("[FlujoAPI] Echo init status:\n%s\n", string(out))

	// Ejecutar reply con mensaje inicial para obtener respuesta de Echo
	resp, err := callReplyCLI("(Inicio de conversación con nuevo usuario)")
	if err != nil {
		fmt.Printf("Error en Echo init: %v\n", err)
		return
	}

	// Actualizar handoff basándose en el output completo
	updateHandoffState(resp)

	// Guardar respuesta (ya viene parseada de callReplyCLI)
	msgs, _ := loadMessages(convID)
	echoMsg := Message{
		ID:        generateID(),
		Role:      "echo",
		Content:   resp,
		Event:     detectEvent(resp),
		Timestamp: time.Now(),
	}
	msgs = append(msgs, echoMsg)
	saveMessages(convID, msgs)

	fmt.Printf("[FlujoAPI] Nueva conv %s: %s\n", convID, echoMsg.Content)
}

func processWithEcho(message string, messages []Message, convID string) (string, string) {
	// Usar callReplyCLI para mantener consistencia con el CLI de flujo
	resp, err := callReplyCLI(message)
	if err != nil {
		return "Error: " + err.Error(), "error"
	}

	// Actualizar handoff basándose en el output completo
	updateHandoffState(resp)

	return resp, detectEvent(resp)
}

func callReplyCLI(message string) (string, error) {
	// Ejecutar el comando reply de flujo CLI para mantener consistencia
	// Esto usa el mismo session path y system prompt que el CLI
	args := []string{"run", "./cmd/flujo", "reply", message}
	cmd := exec.Command("go", args...)
	cmd.Dir = "/Users/alcless_a1234_cursor/remora-go/remora-flujo"

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error ejecutando reply: %v - %s", err, string(output))
	}

	return parseReplyOutput(string(output)), nil
}

func parseReplyOutput(output string) string {
	// Mejorar el parseo para extraer solo lo visible al usuario
	lines := strings.Split(output, "\n")
	var echoContent []string

	// Primero buscar si hay un bloque Echo: explícito
	echoStartIdx := -1
	echoEndIdx := len(lines)
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Echo:") && echoStartIdx == -1 {
			echoStartIdx = i
		}
		if echoStartIdx != -1 && echoEndIdx == len(lines) {
			if strings.HasPrefix(trimmed, "[FLOW]") || strings.HasPrefix(trimmed, "[PALADIN]") {
				echoEndIdx = i
			}
		}
	}

	// Extraer contenido del bloque Echo si existe
	if echoStartIdx != -1 {
		for i := echoStartIdx + 1; i < echoEndIdx; i++ {
			trimmed := strings.TrimSpace(lines[i])
			
			// Saltar comandos del sistema
			if strings.HasPrefix(trimmed, "go run ./cmd/flujo") ||
				strings.HasPrefix(trimmed, "cd /Users") ||
				strings.HasPrefix(trimmed, "./frameworkecho") ||
				strings.HasPrefix(trimmed, "FUNC ") ||
				strings.HasPrefix(trimmed, "START ") ||
				strings.HasPrefix(trimmed, "END ") ||
				strings.HasPrefix(trimmed, "DEC ") ||
				strings.HasPrefix(trimmed, "VAR ") {
				continue
			}
			
			if trimmed != "" {
				echoContent = append(echoContent, trimmed)
			}
		}
	}

	// Limpiar: remover líneas vacías al inicio y fin
	for len(echoContent) > 0 && strings.TrimSpace(echoContent[0]) == "" {
		echoContent = echoContent[1:]
	}
	for len(echoContent) > 0 && strings.TrimSpace(echoContent[len(echoContent)-1]) == "" {
		echoContent = echoContent[:len(echoContent)-1]
	}

	if len(echoContent) > 0 {
		return strings.TrimSpace(strings.Join(echoContent, "\n"))
	}

	// Fallback: buscar comando handoff con pregunta
	return extractHandoffMessage(output)
}

func extractFallbackText(output string) string {
	// Cuando no hay bloque Echo explícito, buscar respuestas significativas
	lines := strings.Split(output, "\n")
	var result []string

	// Saltar líneas de sistema y logs
	skipPatterns := []string{
		"========================================",
		"[FLOW]",
		"[PALADIN]",
		"Paladin Trace",
		"modelo_activo:",
		"modelo_razon:",
		"go run ./cmd/flujo",
		"./frameworkecho",
		"START ",
		"END ",
		"DEC ",
		"VAR ",
		"FUNC ",
		"Trace:",
		"App:",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		skip := false
		for _, pattern := range skipPatterns {
			if strings.HasPrefix(trimmed, pattern) {
				skip = true
				break
			}
		}
		if !skip && trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return "(sin respuesta visible)"
	}

	// Devolver las últimas líneas significativas (típicamente la respuesta)
	startIdx := 0
	if len(result) > 5 {
		startIdx = len(result) - 5
	}
	return strings.TrimSpace(strings.Join(result[startIdx:], "\n"))
}

func extractHandoffMessage(output string) string {
	// Buscar el comando handoff y extraer la pregunta/mensaje
	// Patrón: go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta"
	re := regexp.MustCompile(`--message "([^"]+)"`)
	matches := re.FindAllStringSubmatch(output, -1)

	if len(matches) > 0 {
		// Devolver el último mensaje (que es la respuesta actual de Echo)
		return matches[len(matches)-1][1]
	}

	// Si no hay comando handoff, usar el fallback
	return extractFallbackText(output)
}

func detectEvent(response string) string {
	if strings.Contains(response, "echo_waiting_user") || strings.Contains(response, "?") {
		return "echo_waiting_user"
	}
	if strings.Contains(response, "echo_ready_for_alfa") || strings.Contains(response, "ready_for_alfa") {
		return "echo_ready_for_alfa"
	}
	if strings.Contains(response, "echo_user_answered") {
		return "echo_user_answered"
	}
	return ""
}

func updateHandoffState(response string) {
	// Parse commands from response
	re := regexp.MustCompile(`go run \./cmd/flujo done echo --event (\w+) --message "([^"]*)"`)
	matches := re.FindAllStringSubmatch(response, -1)

	if len(matches) > 0 {
		last := matches[len(matches)-1]
		event := last[1]
		message := last[2]

		state, _ := handoff.Load(statePath)
		evt, _ := handoff.ParseEvent(event)
		state.Done(handoff.RoleEcho, evt, message)
		handoff.Save(statePath, state)

		fmt.Printf("[FlujoAPI] Handoff update: %s - %s\n", event, message)
	}
}

// File helpers

func loadConvMeta(id string) (*Conversation, error) {
	metaPath := filepath.Join(convDir, id, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var conv Conversation
	json.Unmarshal(data, &conv)
	return &conv, nil
}

func loadMessages(convID string) ([]Message, error) {
	msgsPath := filepath.Join(convDir, convID, "messages.json")
	data, err := os.ReadFile(msgsPath)
	if err != nil {
		return []Message{}, nil
	}
	var msgs []Message
	json.Unmarshal(data, &msgs)
	return msgs, nil
}

func saveMessages(convID string, messages []Message) error {
	convPath := filepath.Join(convDir, convID)
	os.MkdirAll(convPath, 0755)

	msgsData, _ := json.MarshalIndent(messages, "", "  ")
	return os.WriteFile(filepath.Join(convPath, "messages.json"), msgsData, 0644)
}

func saveConvMeta(conv *Conversation) error {
	convPath := filepath.Join(convDir, conv.ID)
	os.MkdirAll(convPath, 0755)

	metaData, _ := json.MarshalIndent(conv, "", "  ")
	return os.WriteFile(filepath.Join(convPath, "meta.json"), metaData, 0644)
}
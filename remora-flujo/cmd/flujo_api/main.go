// flujo_api expone una API REST para conversaciones multi-framework.
//
// Una conversación es una "red comunicacional" entre el usuario y N frameworks
// elegidos al crearla. Cada framework declara en su framework.manifest.json
// (campo user_input) si necesita input del usuario y cómo. El orquestador
// consume preguntas de los frameworks vía Channel (JSON-RPC) y las entrega al
// usuario UNA a la vez por la cola compartida.
//
// Endpoints:
//   GET    /health
//   GET    /api/v1/conversations                    lista conversaciones
//   POST   /api/v1/conversations                    crea conversación con frameworks
//   GET    /api/v1/conversations/{id}               metadata + mensajes
//   DELETE /api/v1/conversations/{id}               elimina
//   GET    /api/v1/conversations/{id}/messages      historial visible
//   POST   /api/v1/conversations/{id}/messages      manda input del usuario
//   GET    /api/v1/conversations/{id}/queue         cola de preguntas
//   GET    /api/v1/frameworks                       drivers disponibles
//
// Variables de entorno:
//   CHANNEL_URL      default http://localhost:8765
//   CHANNEL_API_KEY  default test-key-001
//   FLUJO_API_PORT   default 8084
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"channel/adapter"
	"github.com/gorilla/mux"
	"remora-flujo/handoff"
)

const apiBase = "/api/v1"

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type server struct {
	channel *adapter.Client
	rules   *FlowRules
}

func main() {
	channelURL := envOr("CHANNEL_URL", "http://localhost:8765")
	apiKey := envOr("CHANNEL_API_KEY", "test-key-001")
	port := envOr("FLUJO_API_PORT", "8084")

	if err := os.MkdirAll(convDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "no pude crear %s: %v\n", convDir, err)
		os.Exit(1)
	}

	rulesPath := envOr("FLOW_RULES", "cmd/flujo_api/flow.rules.json")
	rules, rerr := loadFlowRules(rulesPath)
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "warn: no pude cargar %s: %v (continuamos sin reglas)\n", rulesPath, rerr)
		rules = &FlowRules{Version: 1}
	}

	srv := &server{channel: adapter.New(channelURL, apiKey), rules: rules}

	r := mux.NewRouter()
	r.Use(corsMiddleware)
	r.HandleFunc("/health", srv.health).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/frameworks", srv.listFrameworks).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations", srv.listConversations).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations", srv.createConversation).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}", srv.getConversation).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}", srv.deleteConversation).Methods("DELETE", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", srv.getMessages).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", srv.postMessage).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/queue", srv.getQueue).Methods("GET", "OPTIONS")

	addr := ":" + port
	fmt.Printf("Flujo API en http://localhost%s%s\n", addr, apiBase)
	fmt.Printf("  Channel:        %s\n", channelURL)
	fmt.Printf("  Frameworks:     %v\n", knownFrameworks())
	fmt.Printf("  Reglas activas: %d (%s)\n", len(rules.Rules), rulesPath)
	if err := http.ListenAndServe(addr, r); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, body APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, APIResponse{Success: false, Error: msg})
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"status": "ok"})
}

func (s *server) listFrameworks(w http.ResponseWriter, r *http.Request) {
	type fwInfo struct {
		Name         string   `json:"name"`
		Provider     string   `json:"provider"`
		Model        string   `json:"model"`
		Capabilities []string `json:"capabilities"`
		EnvKey       string   `json:"env_key"`
		AskVia       string   `json:"ask_via,omitempty"`
		Modes        []string `json:"modes,omitempty"`
	}
	out := []fwInfo{}
	for name := range driverRegistry {
		info := fwInfo{Name: name}
		if m, err := loadFrameworkManifest(name); err == nil {
			info.Provider = m.Model.Provider
			info.Model = m.Model.Name
			info.Capabilities = m.Model.Capabilities
			info.EnvKey = m.Model.EnvKey
			info.AskVia = m.UserInput.AskVia
			info.Modes = m.UserInput.Modes
		}
		out = append(out, info)
	}
	writeOK(w, out)
}

func (s *server) listConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := listConvs()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, convs)
}

type createConvRequest struct {
	Title      string            `json:"title"`
	Frameworks []string          `json:"frameworks"`
	Models     map[string]string `json:"models,omitempty"`
}

func (s *server) createConversation(w http.ResponseWriter, r *http.Request) {
	var req createConvRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if len(req.Frameworks) == 0 {
		req.Frameworks = []string{"echo", "alfa"}
	}
	for _, fw := range req.Frameworks {
		if _, ok := driverRegistry[fw]; !ok {
			writeErr(w, http.StatusBadRequest, "framework desconocido: "+fw)
			return
		}
	}

	conv := &Conversation{
		ID:         fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		Title:      req.Title,
		Frameworks: req.Frameworks,
		Models:     req.Models,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := saveConv(conv); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Cola por conversación, con frameworks declarados.
	queue := handoff.NewQuestionsQueue(conv.Frameworks...)
	if err := saveQueue(conv.ID, queue); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Channel client con session id de la conv → JSONL automático en sessions/<id>.jsonl
	ch := s.scoped(conv.ID)

	// Init drivers
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	for _, d := range driversFor(conv) {
		if err := d.Init(ctx, ch, conv); err != nil {
			fmt.Fprintf(os.Stderr, "[flujo_api] driver %s.Init error: %v\n", d.Name(), err)
		}
	}

	// Pedir primera pregunta sin respuesta previa.
	q, ok, err := runLoop(ctx, ch, conv, s.rules, "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[flujo_api] runLoop init: %v\n", err)
	}
	first := initialFrameworkMessage(conv, q, ok)
	if first != nil {
		_ = appendMessage(conv.ID, *first)
	}

	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: map[string]interface{}{
		"conversation":    conv,
		"first_question":  first,
	}})
}

func (s *server) getConversation(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	msgs, _ := loadMessages(id)
	writeOK(w, map[string]interface{}{"conversation": conv, "messages": msgs})
}

func (s *server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := loadConv(id); err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	if err := deleteConv(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"deleted": id})
}

func (s *server) getMessages(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := loadConv(id); err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	msgs, _ := loadMessages(id)
	writeOK(w, msgs)
}

type sendMessageRequest struct {
	Content   string            `json:"content"`
	Resources []MessageResource `json:"resources,omitempty"`
}

func (s *server) postMessage(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inv\u00e1lido")
		return
	}
	if req.Content == "" && len(req.Resources) == 0 {
		writeErr(w, http.StatusBadRequest, "content o resources requerido")
		return
	}

	// 1. Copiar recursos al directorio de la conversación para trazabilidad.
	copiedResources, cerr := storeResources(conv.ID, req.Resources)
	if cerr != nil {
		writeErr(w, http.StatusBadRequest, cerr.Error())
		return
	}

	// 2. Persistir mensaje del usuario
	userMsg := Message{
		ID:        generateMessageID(),
		Role:      "user",
		Content:   req.Content,
		Resources: copiedResources,
		Timestamp: time.Now(),
	}
	if err := appendMessage(id, userMsg); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	conv.UserAnswerCount++
	_ = saveConv(conv)

	ch := s.scoped(conv.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	// 3. Marcar la entrada del usuario en el JSONL de Channel.
	_, _ = ch.ExecuteCommand(ctx, "echo", []string{"user_input:", req.Content, "resources:", fmt.Sprintf("%d", len(copiedResources))}, "")

	q, ok, err := runLoop(ctx, ch, conv, s.rules, req.Content, copiedResources)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !ok {
		// Sin más preguntas: la conversación quedó en idle.
		writeOK(w, map[string]interface{}{
			"user_message":     userMsg,
			"framework_message": nil,
			"idle":             true,
		})
		return
	}

	frameworkMsg := Message{
		ID:         generateMessageID(),
		Role:       "framework",
		Framework:  q.Framework,
		Content:    q.Text,
		QuestionID: q.ID,
		AskVia:     q.AskVia,
		Timestamp:  time.Now(),
	}
	if err := appendMessage(id, frameworkMsg); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"user_message":      userMsg,
		"framework_message": frameworkMsg,
		"idle":              false,
	})
}

func (s *server) getQueue(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := loadConv(id); err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	q, err := loadQueue(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, q)
}

// scoped devuelve un cliente Channel cuya SessionID = convID, así Channel
// persiste automáticamente cada llamada en sessions/<convID>.jsonl.
func (s *server) scoped(convID string) *adapter.Client {
	c := adapter.New(s.channel.BaseURL, s.channel.APIKey)
	c.SessionID = convID
	return c
}

// initialFrameworkMessage construye el primer Message del framework para una conv nueva.
func initialFrameworkMessage(conv *Conversation, q handoff.QueuedQuestion, ok bool) *Message {
	if !ok {
		// Saludo genérico si ningún framework tiene pregunta inicial.
		return &Message{
			ID:        generateMessageID(),
			Role:      "framework",
			Framework: conv.Frameworks[0],
			Content:   "Conversación iniciada. Cuéntame por qué proceso quieres empezar.",
			Timestamp: time.Now(),
		}
	}
	return &Message{
		ID:         generateMessageID(),
		Role:       "framework",
		Framework:  q.Framework,
		Content:    q.Text,
		QuestionID: q.ID,
		AskVia:     q.AskVia,
		Timestamp:  time.Now(),
	}
}

func knownFrameworks() []string {
	out := make([]string, 0, len(driverRegistry))
	for name := range driverRegistry {
		out = append(out, name)
	}
	return out
}

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
//   GET    /api/v1/frameworks/{name}                detail de un framework
//   GET    /api/v1/rules                            ver reglas de composición
//   PUT    /api/v1/rules                            modificar reglas de composición
//   POST   /api/v1/conversations-single             crear conversación con 1 solo framework
//   POST   /api/v1/conversations-single/{id}/messages  enviar a conversación single
//
// Variables de entorno:
//   CHANNEL_URL      default http://localhost:8765
//   CHANNEL_API_KEY  default test-key-001
//   FLUJO_API_PORT   default 8084
package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"channel/adapter"
	"channel/manifest"
	"github.com/gorilla/mux"
	"remora-flujo/handoff"
	"remora-flujo/nativeagent"
)

//go:embed static
var staticFS embed.FS

const apiBase = "/api/v1"

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type server struct {
	channel      *adapter.Client
	rules        *FlowRules
	runtimeInfo  runtimeInfo
	allManifests map[string]*manifest.Manifest
}

type runtimeInfo struct {
	Provider  string
	Model     string
	Note      string
}

func getRuntimeInfo() runtimeInfo {
	provider, model, note, err := nativeagent.RuntimeInfo()
	if err != nil {
		return runtimeInfo{Provider: "unknown", Model: "unknown", Note: err.Error()}
	}
	return runtimeInfo{Provider: provider, Model: model, Note: note}
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

	// Auto-discovery de frameworks vía manifests. Los drivers hardcodeados
	// (echo, alfa) se mantienen; cualquier framework adicional con manifest
	// válido se registra automáticamente vía genericDriver.
	rootDir := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "/workspace"))
	bootLog := log.New(os.Stderr, "[boot] ", log.LstdFlags)
	loadedManifests, skippedManifests := initDriverRegistry(rootDir, bootLog)
	if len(skippedManifests) > 0 {
		bootLog.Printf("manifests omitidos: %d", len(skippedManifests))
	}

	srv := &server{
		channel:      adapter.New(channelURL, apiKey),
		rules:        rules,
		runtimeInfo:  getRuntimeInfo(),
		allManifests: loadedManifests,
	}

	fmt.Printf("Runtime LLM: %s | %s | %s\n", srv.runtimeInfo.Provider, srv.runtimeInfo.Model, srv.runtimeInfo.Note)

	r := mux.NewRouter()
	r.Use(corsMiddleware)
	r.HandleFunc("/health", srv.health).Methods("GET", "OPTIONS")
	r.HandleFunc("/healthz", srv.healthz).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/frameworks", srv.listFrameworks).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations", srv.listConversations).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations", srv.createConversation).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}", srv.getConversation).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}", srv.deleteConversation).Methods("DELETE", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", srv.getMessages).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", srv.postMessage).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations/{id}/queue", srv.getQueue).Methods("GET", "OPTIONS")

	// Rules endpoints - para ver y modificar flow.rules.json
	r.HandleFunc(apiBase+"/rules", srv.getRules).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/rules", srv.updateRules).Methods("PUT", "POST", "OPTIONS")

	// Framework detail endpoint
	r.HandleFunc(apiBase+"/frameworks/{name}", srv.getFramework).Methods("GET", "OPTIONS")

	// Single framework conversation (para probar un framework solo)
	r.HandleFunc(apiBase+"/conversations-single", srv.createSingleConversation).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations-single/{id}/messages", srv.postSingleMessage).Methods("POST", "OPTIONS")

	// Runtime info endpoint
	r.HandleFunc(apiBase+"/runtime", srv.getRuntime).Methods("GET", "OPTIONS")

	// Models available for selection
	r.HandleFunc(apiBase+"/models", srv.listModels).Methods("GET", "OPTIONS")

	// Email sending (SMTP via hosting)
	r.HandleFunc(apiBase+"/send-email", srv.handleSendEmail).Methods("POST", "OPTIONS")

	// Configuración pública (dev mode, profile activo, etc). El frontend lo
	// consulta al bootear para saber si pintar el badge de modo dev.
	r.HandleFunc(apiBase+"/config", srv.handleConfig).Methods("GET", "OPTIONS")

	// Task ledger — lista, próxima, crear, eventos.
	r.HandleFunc(apiBase+"/tasks", srv.handleTasksList).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/tasks", srv.handleTasksCreate).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/tasks/next", srv.handleTasksNext).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/tasks/{id}/event", srv.handleTaskEvent).Methods("POST", "OPTIONS")

	// Frontend estático embebido
	// GET / → sirve index.html
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Frontend no encontrado", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}).Methods("GET")
	// GET /app → redirect a /
	r.HandleFunc("/app", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/", http.StatusFound)
	}).Methods("GET")

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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
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

// healthz es la readiness probe profunda usada por Cloud Run para decidir si
// la instancia esta lista para recibir trafico. Devuelve 200 si todo OK, 503
// si algun componente critico no esta configurado o no responde.
//
// Cloud Run config:
//
//	gcloud run deploy ... --use-http2 \
//	  --readiness-probe=httpGet.path=/healthz,initialDelaySeconds=5,periodSeconds=10
func (s *server) healthz(w http.ResponseWriter, r *http.Request) {
	type checkResult struct {
		Name   string `json:"name"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail,omitempty"`
	}
	checks := []checkResult{}
	allOK := true

	// 1. LLM configurado.
	llmOK := s.runtimeInfo.Provider != "" && s.runtimeInfo.Provider != "unknown"
	llmDetail := fmt.Sprintf("provider=%s model=%s", s.runtimeInfo.Provider, s.runtimeInfo.Model)
	if !llmOK {
		llmDetail = "LLM no configurado: " + s.runtimeInfo.Note
		allOK = false
	}
	checks = append(checks, checkResult{Name: "llm", OK: llmOK, Detail: llmDetail})

	// 2. Frameworks cargados.
	fwCount := len(s.allManifests)
	fwOK := fwCount > 0
	fwDetail := fmt.Sprintf("%d manifests cargados", fwCount)
	if !fwOK {
		fwDetail = "no hay frameworks registrados"
		allOK = false
	}
	checks = append(checks, checkResult{Name: "frameworks", OK: fwOK, Detail: fwDetail})

	// 3. Flow rules cargadas (no-bloqueante: solo informativo).
	rulesCount := 0
	if s.rules != nil {
		rulesCount = len(s.rules.Rules)
	}
	checks = append(checks, checkResult{
		Name:   "rules",
		OK:     true,
		Detail: fmt.Sprintf("%d reglas activas", rulesCount),
	})

	// 4. Channel adapter inicializado (no se hace ping para evitar dependency loop).
	chOK := s.channel != nil
	chDetail := "adapter inicializado"
	if !chOK {
		chDetail = "channel adapter nil"
		allOK = false
	}
	checks = append(checks, checkResult{Name: "channel", OK: chOK, Detail: chDetail})

	status := "ok"
	httpStatus := http.StatusOK
	if !allOK {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"checks": checks,
	})
}

func (s *server) getRuntime(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.runtimeInfo)
}

func (s *server) listModels(w http.ResponseWriter, r *http.Request) {
	// Lista de modelos disponibles para selección en el frontend
	models := []map[string]string{
		{"id": "minimax", "name": "MiniMax M2.7", "description": "Modelo rápido para análisis"},
		{"id": "groq", "name": "Llama 4 Scout", "description": "Modelo de alto rendimiento"},
	}
	writeOK(w, models)
}

func (s *server) listFrameworks(w http.ResponseWriter, r *http.Request) {
	type fwInfo struct {
		Name          string   `json:"name"`
		Provider      string   `json:"provider"`
		Model         string   `json:"model"`
		Capabilities  []string `json:"capabilities"`
		EnvKey        string   `json:"env_key"`
		AskVia        string   `json:"ask_via,omitempty"`
		Modes         []string `json:"modes,omitempty"`
		ExecutionMode string   `json:"execution_mode"`
		Description   string   `json:"description,omitempty"`
		Produces      []string `json:"produces,omitempty"`
		Requires      []string `json:"requires,omitempty"`
	}
	seen := map[string]bool{}
	out := []fwInfo{}

	// Frameworks en el chain conversacional (driverRegistry)
	for name := range driverRegistry {
		seen[name] = true
		info := fwInfo{Name: name, ExecutionMode: manifest.ExecutionModeSync}
		if m, ok := s.allManifests[name]; ok {
			info.Provider = m.Model.Provider
			info.Model = m.Model.Name
			info.Capabilities = m.Model.Capabilities
			info.EnvKey = m.Model.EnvKey
			info.AskVia = m.UserInput.AskVia
			info.Modes = m.UserInput.Modes
			info.Description = m.Description
			info.Produces = m.CapabilitiesSemantic.Produces
			info.Requires = m.CapabilitiesSemantic.Requires
		}
		out = append(out, info)
	}

	// Frameworks async_trigger (fuera del chain pero conocidos)
	for name, m := range s.allManifests {
		if seen[name] {
			continue
		}
		info := fwInfo{
			Name:          name,
			ExecutionMode: m.EffectiveExecutionMode(),
			Description:   m.Description,
			Provider:      m.Model.Provider,
			Capabilities:  m.Model.Capabilities,
		}
		info.Produces = m.CapabilitiesSemantic.Produces
		info.Requires = m.CapabilitiesSemantic.Requires
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

	var first *Message

	if os.Getenv("REMORA_PROFILE") == "cobranza-chile" && contains(req.Frameworks, "foco") {
		// En modo cobranza, Foco habla primero con prioridades automáticas.
		// No llamamos runLoop para evitar que Sabio inyecte su saludo en el historial.
		first = s.triggerFocoFirst(ch, conv)
		if first != nil {
			_ = appendMessage(conv.ID, *first)
		}
	} else {
		// Flujo normal: primera pregunta sin respuesta previa.
		q, ok, err := runLoop(ctx, ch, conv, s.rules, s.allManifests, "", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[flujo_api] runLoop init: %v\n", err)
		}
		first = initialFrameworkMessage(conv, q, ok)
		if first != nil {
			_ = appendMessage(conv.ID, *first)
		}
	}

	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: map[string]interface{}{
		"conversation":    conv,
		"first_question":  first,
	}})
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// triggerFocoFirst llama a foco para que dé las prioridades automáticamente
func (s *server) triggerFocoFirst(ch *adapter.Client, conv *Conversation) *Message {
	// Usar el driver de foco directamente
	for _, d := range driversFor(conv) {
		if d.Name() == "foco" {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			// Simular que el usuario preguntó "¿qué hago hoy?"
			qctx := QueuedAnswerCtx{
				Answer:       "iniciar día - mostrar prioridades",
				QuestionText: "(inicio de sesión)",
			}
			if err := d.IngestAnswer(ctx, ch, conv, qctx); err != nil {
				fmt.Printf("[flujo_api] foco ingest error: %v\n", err)
			}
			
			// Pedir la pregunta de foco (que será las prioridades)
			// Usar PollQuestionFull si está disponible para capturar chips.
			type fullPoller interface {
				PollQuestionFull(context.Context, *adapter.Client, *Conversation, map[string]bool) (nextQuestionResponse, bool)
			}
			if fp, hasFull := d.(fullPoller); hasFull {
				r, ok := fp.PollQuestionFull(ctx, ch, conv, nil)
				if ok && r.Text != "" {
					return &Message{
						ID:             generateMessageID(),
						Role:           "framework",
						Framework:      "foco",
						Content:        r.Text,
						QuestionID:     r.ID,
						AskVia:         r.AskVia,
						SuggestedChips: r.Chips,
						Timestamp:      time.Now(),
					}
				}
			} else {
				text, extID, askVia, ok := d.PollQuestion(ctx, ch, conv, nil)
				if ok && text != "" {
					return &Message{
						ID:          generateMessageID(),
						Role:        "framework",
						Framework:   "foco",
						Content:     text,
						QuestionID:  extID,
						AskVia:      askVia,
						Timestamp:   time.Now(),
					}
				}
			}
		}
	}
	return nil
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

	q, ok, err := runLoop(ctx, ch, conv, s.rules, s.allManifests, req.Content, copiedResources)
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
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      q.Framework,
		Content:        q.Text,
		QuestionID:     q.ID,
		AskVia:         q.AskVia,
		SuggestedChips: q.Chips,
		Timestamp:      time.Now(),
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

// ============================================
// MESSAGE SEND ENDPOINT (genérico, agnóstico al negocio)
// ============================================
//
// El API NO conoce SMTP, IMAP, Twilio ni cualquier protocolo. Delega el
// envío al framework `mensajero` (binario CLI) que lee credentials.<channel>
// del vault compartido. Si la capability no existe, devuelve 412 con un
// hint de qué framework la provee, y el frontend puede ofrecer el setup.
//
// Contrato HTTP:
//
//	POST /api/v1/send-email
//	  body: {subject, body, to?, channel?, conv_id?}
//	  channel default: "email"  conv_id default: "default"
//	200 {success:true, message_id, to, channel}
//	412 {success:false, missing_capability, provider_hint}  (vault vacío)
//	500 {success:false, error}                              (fallo del envío)

type sendEmailReq struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	To      string `json:"to,omitempty"`
	// EntityType + EntityRef permiten al backend resolver el destinatario
	// vía framework-contactos cuando `To` no viene en el request.
	EntityType string `json:"entity_type,omitempty"`
	EntityRef  string `json:"entity_ref,omitempty"`
	Channel string `json:"channel,omitempty"`
	ConvID  string `json:"conv_id,omitempty"`
	// TaskID opcional: si viene, al completar el envío se emite un task.event
	// con kind=email_sent para cerrar la tarea automáticamente en el ledger.
	TaskID string `json:"task_id,omitempty"`
}

type sendEmailResp struct {
	Success           bool   `json:"success"`
	Channel           string `json:"channel,omitempty"`
	To                string `json:"to,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	Subject           string `json:"subject,omitempty"`
	MissingCapability string `json:"missing_capability,omitempty"`
	ProviderHint      string `json:"provider_hint,omitempty"`
	Error             string `json:"error,omitempty"`
	// DevRewritten indica que el `to` fue reescrito al destinatario dev.
	DevRewritten bool   `json:"dev_rewritten,omitempty"`
	OriginalTo   string `json:"original_to,omitempty"`
}

// envBootstrapOnce migra credenciales SMTP de env vars al vault una sola vez
// (back-compat con el deploy actual que usa SMTP_USER/SMTP_PASS en Cloud
// Run). Si el vault ya tiene credentials.smtp para la conv, no hace nada.
var envBootstrapOnce sync.Map // key: convID → bool

func bootstrapSMTPFromEnvIfNeeded(convID string) {
	if _, done := envBootstrapOnce.LoadOrStore(convID, true); done {
		return
	}
	if vaultHasFromAPI(convID, "credentials.smtp") {
		return
	}
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	if user == "" || pass == "" {
		return
	}
	bundle := map[string]string{
		"host":       envOr("SMTP_HOST", "mail.patriciastocker.com"),
		"port":       envOr("SMTP_PORT", "587"),
		"user":       user,
		"pass":       pass,
		"from":       envOr("SMTP_FROM", user),
		"default_to": envOr("TEST_EMAIL_RECIPIENT", "tom3bs@gmail.com"),
	}
	data, _ := json.Marshal(bundle)
	cmd := exec.Command(vaultBinPath(), "set", "--conv", convOrDefault(convID), "--key", "credentials.smtp", "--stdin")
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(string(data))
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("bootstrap SMTP→vault: %v (%s)", err, string(out))
	} else {
		log.Printf("bootstrap SMTP→vault: ok (conv=%s)", convOrDefault(convID))
	}
}

// devModeEnabled devuelve true si el deployment actual está en modo dev.
// Criterios (OR):
//   - REMORA_DEV_MODE=true explícito
//   - el servicio se llama *-dev (heurística Cloud Run via K_SERVICE)
func devModeEnabled() bool {
	if v := strings.ToLower(os.Getenv("REMORA_DEV_MODE")); v == "true" || v == "1" || v == "yes" {
		return true
	}
	if strings.HasSuffix(os.Getenv("K_SERVICE"), "-dev") {
		return true
	}
	return false
}

func devRecipient() string {
	return envOr("TEST_EMAIL_RECIPIENT", "tom3bs@gmail.com")
}

// handleConfig expone configuración pública al frontend: profile activo,
// dev mode on/off, destinatario de redirect. Es público (no filtra secretos).
func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]interface{}{
		"profile":       envOr("REMORA_PROFILE", ""),
		"dev_mode":      devModeEnabled(),
		"dev_recipient": devRecipient(),
		"runtime":       s.runtimeInfo,
	})
}

func (s *server) handleSendEmail(w http.ResponseWriter, r *http.Request) {
	var req sendEmailReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.Subject == "" && req.Body == "" {
		writeErr(w, http.StatusBadRequest, "subject o body requerido")
		return
	}
	if req.Channel == "" {
		req.Channel = "email"
	}
	if req.ConvID == "" {
		req.ConvID = "default"
	}

	// Resolver destinatario vía framework-contactos si no vino `to` en el
	// request pero sí vino entity_type+entity_ref. Si tampoco hay contacto,
	// devolvemos 412 con provider_hint=contactos para que el frontend
	// dispare el flujo de captura/import.
	if req.To == "" && req.EntityType != "" && req.EntityRef != "" {
		res, err := contactosLookup(req.EntityType, req.EntityRef, req.Channel)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "contactos lookup: "+err.Error())
			return
		}
		if res.Found {
			req.To = res.Value
		} else {
			writeJSON(w, http.StatusPreconditionFailed, APIResponse{
				Success: false,
				Data: map[string]interface{}{
					"missing_capability": res.MissingCapability,
					"provider_hint":     res.ProviderHint,
					"entity_type":       req.EntityType,
					"entity_ref":        req.EntityRef,
					"channel":           req.Channel,
				},
				Error: "contacto faltante; cargá email vía framework-contactos",
			})
			return
		}
	}

	// Modo dev: reescribir destinatario a tom3bs@gmail.com (o lo que diga
	// TEST_EMAIL_RECIPIENT) y dejar rastro del destinatario original en el
	// subject para que el operador vea a quién se hubiera enviado en prod.
	devRewritten := false
	originalTo := req.To
	if devModeEnabled() {
		dev := devRecipient()
		if dev != "" && req.To != dev {
			req.To = dev
			devRewritten = true
		}
		if originalTo == "" {
			originalTo = "(sin destinatario)"
		}
		prefix := "[DEV → " + originalTo + "] "
		if !strings.HasPrefix(req.Subject, "[DEV") {
			req.Subject = prefix + req.Subject
		}
	}

	bootstrapSMTPFromEnvIfNeeded(req.ConvID)

	binPath, cwd, err := resolveMensajero()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	bodyB64 := base64.StdEncoding.EncodeToString([]byte(req.Body))
	args := []string{"send",
		"--channel", req.Channel,
		"--to", req.To,
		"--subject", req.Subject,
		"--body-b64", bodyB64,
		"--conv-id", req.ConvID,
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	out, runErr := cmd.Output()

	// Mensajero siempre imprime JSON en stdout, incluso en error.
	var msResp sendEmailResp
	if jerr := json.Unmarshal(out, &msResp); jerr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		writeErr(w, http.StatusInternalServerError,
			fmt.Sprintf("mensajero respuesta inválida: %v stderr=%s out=%s", jerr, stderr, string(out)))
		return
	}

	if msResp.MissingCapability != "" {
		// 412 Precondition Failed: el cliente debe ejecutar provisioning.
		writeJSON(w, http.StatusPreconditionFailed, APIResponse{
			Success: false,
			Data:    msResp,
			Error:   "credenciales faltantes; ejecutá provisioning con el framework " + msResp.ProviderHint,
		})
		return
	}
	if !msResp.Success {
		writeErr(w, http.StatusInternalServerError, "error enviando: "+msResp.Error)
		return
	}
	msResp.Subject = req.Subject
	if devRewritten {
		msResp.DevRewritten = true
		msResp.OriginalTo = originalTo
	}
	// Auto-evento al ledger si el cliente pasó task_id. Esto cierra el loop
	// de Foco: ejecuta acción → framework la registra → Foco ve la siguiente.
	if req.TaskID != "" {
		emitTaskEvent(req.TaskID, "mensajero", "email_sent", map[string]interface{}{
			"message_id":     msResp.MessageID,
			"to":             msResp.To,
			"original_to":    originalTo,
			"dev_rewritten":  devRewritten,
			"channel":        msResp.Channel,
			"result_ref":     "message:" + msResp.MessageID,
		})
	}
	writeOK(w, msResp)
}

// resolveMensajero localiza el binario `frameworkmensajero` y su cwd.
// Resolución: env REMORA_MENSAJERO_BIN, o ../framework-mensajero/frameworkmensajero
// relativo a REMORA_ROOT.
func resolveMensajero() (string, string, error) {
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "/workspace"))
	cwd := filepath.Join(root, "framework-mensajero")
	bin := os.Getenv("REMORA_MENSAJERO_BIN")
	if bin == "" {
		bin = filepath.Join(cwd, "frameworkmensajero")
	}
	if _, err := os.Stat(bin); err != nil {
		return "", "", fmt.Errorf("framework-mensajero no encontrado en %s: %w", bin, err)
	}
	return bin, cwd, nil
}

// vaultBinPath localiza el binario vault.
func vaultBinPath() string {
	if v := os.Getenv("REMORA_VAULT_BIN"); v != "" {
		return v
	}
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "/workspace"))
	return filepath.Join(root, "channel", "bin", "vault")
}

// vaultHasFromAPI hace exit-code-check sin desencriptar.
func vaultHasFromAPI(convID, key string) bool {
	cmd := exec.Command(vaultBinPath(), "has", "--conv", convOrDefault(convID), "--key", key)
	cmd.Env = os.Environ()
	return cmd.Run() == nil
}

func convOrDefault(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "default"
	}
	return c
}

func knownFrameworks() []string {
	out := make([]string, 0, len(driverRegistry))
	for name := range driverRegistry {
		out = append(out, name)
	}
	return out
}

// ============================================
// RULES ENDPOINTS
// ============================================

func (s *server) getRules(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.rules)
}

func (s *server) updateRules(w http.ResponseWriter, r *http.Request) {
	var updated FlowRules
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	// Validar versión mínima
	if updated.Version != 1 {
		writeErr(w, http.StatusBadRequest, "versión debe ser 1")
		return
	}

	s.rules = &updated

	// Persistir a archivo
	rulesPath := envOr("FLOW_RULES", "cmd/flujo_api/flow.rules.json")
	data, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error serializando: "+err.Error())
		return
	}
	if err := os.WriteFile(rulesPath, append(data, '\n'), 0644); err != nil {
		writeErr(w, http.StatusInternalServerError, "error guardando archivo: "+err.Error())
		return
	}

	writeOK(w, map[string]string{"saved": rulesPath})
}

// ============================================
// FRAMEWORK DETAIL ENDPOINT
// ============================================

func (s *server) getFramework(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	
	// Verificar que existe
	if _, ok := driverRegistry[name]; !ok {
		writeErr(w, http.StatusNotFound, "framework no encontrado: "+name)
		return
	}

	// Cargar manifest
	m, err := loadFrameworkManifest(name)
	if err != nil {
		writeErr(w, http.StatusNotFound, "manifest no encontrado para: "+name)
		return
	}

	writeOK(w, m)
}

// ============================================
// SINGLE FRAMEWORK CONVERSATION
// ============================================

type createSingleConvRequest struct {
	Title      string `json:"title"`
	Framework  string `json:"framework"`
}

func (s *server) createSingleConversation(w http.ResponseWriter, r *http.Request) {
	var req createSingleConvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}

	// Verificar que el framework existe
	if _, ok := driverRegistry[req.Framework]; !ok {
		writeErr(w, http.StatusBadRequest, "framework desconocido: "+req.Framework)
		return
	}

	conv := &Conversation{
		ID:         fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		Title:      req.Title,
		Frameworks: []string{req.Framework},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := saveConv(conv); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	queue := handoff.NewQuestionsQueue(conv.Frameworks...)
	if err := saveQueue(conv.ID, queue); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	ch := s.scoped(conv.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Init driver
	drivers := driversFor(conv)
	for _, d := range drivers {
		if err := d.Init(ctx, ch, conv); err != nil {
			fmt.Fprintf(os.Stderr, "[flujo_api] driver %s.Init error: %v\n", d.Name(), err)
		}
	}

	// Pedir primera pregunta
	q, ok, err := runLoop(ctx, ch, conv, s.rules, s.allManifests, "", nil)
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

// ============================================
// SEND TO SINGLE FRAMEWORK
// ============================================

func (s *server) postSingleMessage(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.Content == "" && len(req.Resources) == 0 {
		writeErr(w, http.StatusBadRequest, "content o resources requerido")
		return
	}

	copiedResources, cerr := storeResources(conv.ID, req.Resources)
	if cerr != nil {
		writeErr(w, http.StatusBadRequest, cerr.Error())
		return
	}

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

	_, _ = ch.ExecuteCommand(ctx, "echo", []string{"user_input:", req.Content}, "")

	q, ok, err := runLoop(ctx, ch, conv, s.rules, s.allManifests, req.Content, copiedResources)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !ok {
		writeOK(w, map[string]interface{}{
			"user_message":     userMsg,
			"framework_message": nil,
			"idle":             true,
		})
		return
	}

	frameworkMsg := Message{
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      q.Framework,
		Content:        q.Text,
		QuestionID:     q.ID,
		AskVia:         q.AskVia,
		SuggestedChips: q.Chips,
		Timestamp:      time.Now(),
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

// api_rest expone una API REST para conversaciones multi-framework.
//
// Una conversación es una "red comunicacional" entre el usuario y N frameworks
// elegidos al crearla. Cada framework declara en su framework.manifest.json
// (campo user_input) si necesita input del usuario y cómo. El orquestador
// consume preguntas de los frameworks vía Channel (JSON-RPC) y las entrega al
// usuario UNA a la vez por la cola compartida.
//
// Endpoints:
//
//	GET    /health
//	GET    /api/v1/conversations                    lista conversaciones
//	POST   /api/v1/conversations                    crea conversación con frameworks
//	GET    /api/v1/conversations/{id}               metadata + mensajes
//	DELETE /api/v1/conversations/{id}               elimina
//	GET    /api/v1/conversations/{id}/messages      historial visible
//	POST   /api/v1/conversations/{id}/messages      manda input del usuario
//	GET    /api/v1/conversations/{id}/queue         cola de preguntas
//	GET    /api/v1/frameworks                       drivers disponibles
//	GET    /api/v1/frameworks/{name}                detail de un framework
//	GET    /api/v1/rules                            ver reglas de composición
//	PUT    /api/v1/rules                            modificar reglas de composición
//	POST   /api/v1/conversations-single             crear conversación con 1 solo framework
//	POST   /api/v1/conversations-single/{id}/messages  enviar a conversación single
//
// Variables de entorno:
//
//	CHANNEL_URL      default http://localhost:8765
//	CHANNEL_API_KEY  default test-key-001
//	API_REST_PORT    default 8084 (FLUJO_API_PORT kept as legacy fallback)
//	REMORA_DEV_STATIC=1 serves cmd/api_rest/static files from disk instead of go:embed
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
	Testable      bool     `json:"testable"`
	Chainable     bool     `json:"chainable"`
}

type server struct {
	channel      *adapter.Client
	rules        *FlowRules
	runtimeInfo  runtimeInfo
	allManifests map[string]*manifest.Manifest
	rootDir      string
	auth         *authStore
	flows        *flowStore
}

type runtimeInfo struct {
	Provider string
	Model    string
	Note     string
}

func getRuntimeInfo() runtimeInfo {
	provider, model, note, err := nativeagent.RuntimeInfo()
	if err != nil {
		return runtimeInfo{Provider: "unknown", Model: "unknown", Note: err.Error()}
	}
	return runtimeInfo{Provider: provider, Model: model, Note: note}
}

func main() {
	// Cargar .env desde el root del repo si existe
	loadDotEnv()

	channelURL := envOr("CHANNEL_URL", "http://localhost:8765")
	apiKey := envOr("CHANNEL_API_KEY", "test-key-001")
	port := envOr("API_REST_PORT", envOr("FLUJO_API_PORT", "8084"))
	authStore, authErr := openAuthStore()
	if authErr != nil {
		fmt.Fprintf(os.Stderr, "no pude abrir auth store: %v\n", authErr)
		os.Exit(1)
	}

	flowDBPath := envOr("REMORA_AUTH_DB", defaultAuthDBPath())
	flowStore, flowErr := openFlowStore(flowDBPath)
	if flowErr != nil {
		fmt.Fprintf(os.Stderr, "no pude abrir flow store: %v\n", flowErr)
		os.Exit(1)
	}

	if err := os.MkdirAll(convDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "no pude crear %s: %v\n", convDir, err)
		os.Exit(1)
	}

	rulesPath := envOr("FLOW_RULES", "cmd/api_rest/flow.rules.json")
	rules, rerr := loadFlowRules(rulesPath)
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "warn: no pude cargar %s: %v (continuamos sin reglas)\n", rulesPath, rerr)
		rules = &FlowRules{Version: 1}
	}

	// Auto-discovery de frameworks vía manifests. Los drivers hardcodeados
	// (echo, alfa) se mantienen; cualquier framework adicional con manifest
	// válido se registra automáticamente vía genericDriver.
	rootDir := resolveRemoraRoot()
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
		rootDir:      rootDir,
		auth:         authStore,
		flows:        flowStore,
	}

	fmt.Printf("Runtime LLM: %s | %s | %s\n", srv.runtimeInfo.Provider, srv.runtimeInfo.Model, srv.runtimeInfo.Note)

	r := mux.NewRouter()
	r.Use(corsMiddleware)
	r.HandleFunc("/health", srv.health).Methods("GET", "OPTIONS")
	r.HandleFunc("/healthz", srv.healthz).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/auth/register", srv.handleAuthRegister).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/auth/login", srv.handleAuthLogin).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/auth/logout", srv.handleAuthLogout).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/auth/me", srv.handleAuthMe).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses", srv.handleBusinesses).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses", srv.handleBusinessCreate).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/members", srv.handleBusinessMembers).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/invites", srv.handleBusinessInviteCreate).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/invites/lookup", srv.handleInviteLookup).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/invites/accept", srv.handleInviteAccept).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/admin/users", srv.handleAdminUsers).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/admin/team", srv.handleAdminTeam).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/admin/remora-invites", srv.handleAdminRemoraInviteCreate).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/remora-invites/lookup", srv.handleRemoraInviteLookup).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/remora-invites/accept", srv.handleRemoraInviteAccept).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/frameworks", srv.listFrameworks).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/frameworks/testable", srv.listTestableFrameworks).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/frameworks/chainable", srv.listChainableFrameworks).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/capabilities", srv.listCapabilities).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/capabilities/{id}/providers", srv.listCapabilityProviders).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/validate", srv.validateFlow).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/simulate", srv.simulateFlow).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/run", srv.runFlow).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/run/stream", srv.runFlowStream).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/suggest", srv.suggestFlowCapabilities).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/flows", srv.handleListFlows).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/flows", srv.handleCreateFlow).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/hosting/connect", srv.handleHostingConnect).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/smtp/check", srv.handleSMTPCheck).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/smtp/import", srv.handleSMTPImport).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/smtp", srv.handleSMTPDelete).Methods("DELETE", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/{id}/install", srv.handleInstallFlow).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/{id}", srv.handleGetFlow).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/{id}", srv.handleUpdateFlow).Methods("PUT", "OPTIONS")
	r.HandleFunc(apiBase+"/flows/{id}", srv.handleDeleteFlow).Methods("DELETE", "OPTIONS")
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
	r.HandleFunc(apiBase+"/frameworks/{name}/commands/{command}/run", srv.runFrameworkCommand).Methods("POST", "OPTIONS")

	// Single framework conversation (para probar un framework solo)
	r.HandleFunc(apiBase+"/conversations-single", srv.createSingleConversation).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations-single/{id}/messages", srv.postSingleMessage).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/conversations-single/{id}/live", srv.getFrameworkSessionLiveEvents).Methods("GET", "OPTIONS")

	// Runtime info endpoint
	r.HandleFunc(apiBase+"/runtime", srv.getRuntime).Methods("GET", "OPTIONS")

	// Models available for selection
	r.HandleFunc(apiBase+"/models", srv.listModels).Methods("GET", "OPTIONS")

	// Email sending (SMTP via hosting)
	r.HandleFunc(apiBase+"/send-email", srv.handleSendEmail).Methods("POST", "OPTIONS")

	// Configuración pública (dev mode, profile activo, etc). El frontend lo
	// consulta al bootear para saber si pintar el badge de modo dev.
	r.HandleFunc(apiBase+"/config", srv.handleConfig).Methods("GET", "OPTIONS")

	// Paladin traces — para debug del flujo. Devuelve el trace más reciente
	// persistido en temp/paladin/trace_*.json. Sin auth: solo útil en dev.
	r.HandleFunc(apiBase+"/traces/latest", srv.handleTracesLatest).Methods("GET", "OPTIONS")

	// Data browser read-only — visor spreadsheet para la SQLite declarada.
	r.HandleFunc(apiBase+"/data/tables", srv.handleDataTables).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/data/tables/{table}", srv.handleDataTableRows).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/data/upload", srv.handleBusinessDataUpload).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/data/tables", srv.handleBusinessDataTables).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/data/tables/{table}", srv.handleBusinessDataTableRows).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/artifacts", srv.handleBusinessArtifacts).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/api-connections", srv.handleAPIConnectionsList).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/api-connections", srv.handleAPIConnectionCreate).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/api-connections/plan", srv.handleAPIConnectionPlan).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/businesses/{business_id}/api-connections/{connection_id}/sync", srv.handleAPIConnectionSync).Methods("POST", "OPTIONS")

	// Task ledger — lista, próxima, crear, eventos.
	r.HandleFunc(apiBase+"/tasks", srv.handleTasksList).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/tasks", srv.handleTasksCreate).Methods("POST", "OPTIONS")
	r.HandleFunc(apiBase+"/tasks/next", srv.handleTasksNext).Methods("GET", "OPTIONS")
	r.HandleFunc(apiBase+"/tasks/{id}/event", srv.handleTaskEvent).Methods("POST", "OPTIONS")

	staticIndex := func(name string) ([]byte, error) {
		if os.Getenv("REMORA_DEV_STATIC") == "1" {
			return os.ReadFile(filepath.Join("cmd", "api_rest", "static", name))
		}
		return staticFS.ReadFile(filepath.Join("static", name))
	}

	// Frontend estático embebido o servido desde disco en dev.
	// GET / → sirve index.html
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		data, err := staticIndex("index.html")
		if err != nil {
			http.Error(w, "Frontend no encontrado", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}).Methods("GET")
	r.HandleFunc("/data", func(w http.ResponseWriter, req *http.Request) {
		data, err := staticIndex("data.html")
		if err != nil {
			http.Error(w, "Data browser no encontrado", 500)
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
	fmt.Printf("API REST en http://localhost%s%s\n", addr, apiBase)
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

// loadDotEnv carga variables desde el archivo .env del root del repo.
// No sobreescribe variables que ya existan en el entorno.
func loadDotEnv() {
	// Buscar .env en varias ubicaciones posibles
	candidates := []string{
		".env",
		"../.env",
		"../../.env",
		"../../../.env",
		"../../../../.env",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, `"'`)
				if os.Getenv(key) == "" {
					os.Setenv(key, value)
				}
			}
			fmt.Fprintf(os.Stderr, "[boot] .env cargado desde %s\n", path)
			return
		}
	}
}

func resolveRemoraRoot() string {
	if v := os.Getenv("REMORA_ROOT"); v != "" {
		return v
	}
	if v := os.Getenv("CHANNEL_BASE_DIR"); v != "" {
		return v
	}
	cwd, err := os.Getwd()
	if err == nil {
		if root, ok := findRemoraRoot(cwd); ok {
			return root
		}
	}
	return "/workspace"
}

func findRemoraRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if looksLikeRemoraRoot(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func looksLikeRemoraRoot(dir string) bool {
	required := []string{"channel", "remora-flujo", "framework-echo", "framework-alfa"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
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
	// Estos IDs se usan en el campo Models de la conversación
	models := []map[string]string{
		{
			"id":          "meta-llama/llama-4-scout-17b-16e-instruct",
			"name":        "Llama 4 Scout (Groq)",
			"description": "Modelo principal de alto rendimiento",
			"provider":    "groq",
		},
		{
			"id":          "MiniMax-Text-01",
			"name":        "MiniMax 2.7",
			"description": "Modelo de respaldo rápido",
			"provider":    "minimax",
		},
	}
	writeOK(w, models)
}

func (s *server) listFrameworks(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.collectFrameworkInfos())
}

func (s *server) collectFrameworkInfos() []fwInfo {
	seen := map[string]bool{}
	out := []fwInfo{}

	// Frameworks en el chain conversacional (driverRegistry)
	for name := range driverRegistry {
		seen[name] = true
		info := fwInfo{Name: name, ExecutionMode: manifest.ExecutionModeSync, Testable: true, Chainable: true}
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
			Testable:      frameworkManifestTestable(m),
			Chainable:     false,
		}
		info.Produces = m.CapabilitiesSemantic.Produces
		info.Requires = m.CapabilitiesSemantic.Requires
		out = append(out, info)
	}
	return out
}

func (s *server) listTestableFrameworks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	s.listFrameworksFiltered(w, func(f fwInfo) bool {
		return f.Testable
	})
}

func (s *server) listChainableFrameworks(w http.ResponseWriter, r *http.Request) {
	s.listFrameworksFiltered(w, func(f fwInfo) bool {
		return f.Chainable
	})
}

func (s *server) listFrameworksFiltered(w http.ResponseWriter, keep func(fwInfo) bool) {
	all := s.collectFrameworkInfos()
	items := make([]fwInfo, 0, len(all))
	for _, item := range all {
		if keep(item) {
			items = append(items, item)
		}
	}
	writeOK(w, items)
}

func frameworkManifestTestable(m *manifest.Manifest) bool {
	if m == nil {
		return false
	}
	if m.UserInput.Supported && m.EffectiveExecutionMode() == manifest.ExecutionModeSync {
		return true
	}
	return len(m.Commands) > 0
}

func (s *server) listConversations(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	convs, err := listConvs()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	filtered := make([]Conversation, 0, len(convs))
	for _, conv := range convs {
		if conv.UserID == user.ID {
			filtered = append(filtered, conv)
		}
	}
	writeOK(w, filtered)
}

type createConvRequest struct {
	Title      string            `json:"title"`
	Frameworks []string          `json:"frameworks"`
	Models     map[string]string `json:"models,omitempty"`
	BusinessID string            `json:"business_id,omitempty"`
	Context    map[string]any    `json:"context,omitempty"`
}

func (s *server) createConversation(w http.ResponseWriter, r *http.Request) {
	var req createConvRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if len(req.Frameworks) == 0 {
		req.Frameworks = []string{"echo", "alfa"}
	}
	businessID, runtimeContext, ok := s.requireMembershipContext(w, r, req.BusinessID, req.Context)
	if !ok {
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = businessID
	}
	req.Context = runtimeContext
	for _, fw := range req.Frameworks {
		if _, ok := driverRegistry[fw]; !ok {
			writeErr(w, http.StatusBadRequest, "framework desconocido: "+fw)
			return
		}
	}

	userID, _ := req.Context["remora_user_id"].(string)
	conv := &Conversation{
		ID:             fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		UserID:         userID,
		Title:          req.Title,
		Frameworks:     req.Frameworks,
		Models:         req.Models,
		BusinessID:     req.BusinessID,
		RuntimeContext: req.Context,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
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
			fmt.Fprintf(os.Stderr, "[api_rest] driver %s.Init error: %v\n", d.Name(), err)
		}
	}

	// Siempre iniciar la sesión del primer framework vía Channel JSON-RPC.
	// El framework lee su INITIAL_PROMPT.md y genera su primera respuesta.
	firstFw := conv.Frameworks[0]
	first, err := s.startFrameworkSession(ctx, ch, conv, firstFw)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// first nunca debe ser nil: startFrameworkSession devuelve error explícito si falla.
	_ = appendMessage(conv.ID, *first)

	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: map[string]interface{}{
		"conversation":   conv,
		"first_question": first,
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

func (s *server) getConversation(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	if !s.requireConversationAccess(w, r, conv) {
		return
	}
	msgs, _ := loadMessages(id)
	writeOK(w, map[string]interface{}{"conversation": conv, "messages": msgs})
}

func (s *server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	if !s.requireConversationAccess(w, r, conv) {
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
	conv, err := loadConv(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conversación no encontrada")
		return
	}
	if !s.requireConversationAccess(w, r, conv) {
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
	if !s.requireConversationAccess(w, r, conv) {
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

	if wantsSSE(r) {
		s.postMessageSSE(w, r, conv, req)
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
		Content:   redactLiveEventText(req.Content),
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
			"user_message":      userMsg,
			"framework_message": nil,
			"idle":              true,
		})
		return
	}

	frameworkMsg := Message{
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      q.Framework,
		Content:        q.Text,
		Reasoning:      q.Reasoning,
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

// postMessageSSE es el handler de streaming. Emite eventos SSE en vivo
// mientras los frameworks ejecutan tool calls. Termina con un evento "done".
func (s *server) postMessageSSE(w http.ResponseWriter, r *http.Request, conv *Conversation, req sendMessageRequest) {
	sse, err := newSSEWriter(w)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Persistir recursos y mensaje user (igual que el path normal).
	copiedResources, cerr := storeResources(conv.ID, req.Resources)
	if cerr != nil {
		sse.emit("error", map[string]string{"error": cerr.Error()})
		return
	}
	userMsg := Message{
		ID:        generateMessageID(),
		Role:      "user",
		Content:   req.Content,
		Resources: copiedResources,
		Timestamp: time.Now(),
	}
	if err := appendMessage(conv.ID, userMsg); err != nil {
		sse.emit("error", map[string]string{"error": err.Error()})
		return
	}
	conv.UserAnswerCount++
	_ = saveConv(conv)

	sse.emit("user_message", userMsg)

	// Arranca tailers en paralelo, uno por framework de la conv.
	// Solo arquitecto/critico tienen live JSONL emitido por ahora; los demás
	// frameworks devuelven {} sin emitir nada y el tailer simplemente no
	// recibe eventos (no es error).
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	events := make(chan map[string]any, 64)
	var tailerWG sync.WaitGroup
	remoraRoot := resolveRemoraRoot()
	for _, fw := range conv.Frameworks {
		path := liveFilePath(remoraRoot, fw, conv.ID)
		tailerWG.Add(1)
		go func(framework, path string) {
			defer tailerWG.Done()
			tailLiveFile(ctx, path, events)
		}(fw, path)
	}

	// Goroutine que ejecuta el runLoop pesado.
	type loopResult struct {
		q   handoff.QueuedQuestion
		ok  bool
		err error
	}
	resultCh := make(chan loopResult, 1)
	loopCtx, loopCancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer loopCancel()
	go func() {
		ch := s.scoped(conv.ID)
		_, _ = ch.ExecuteCommand(loopCtx, "echo", []string{"user_input:", req.Content, "resources:", fmt.Sprintf("%d", len(copiedResources))}, "")
		q, ok, lerr := runLoop(loopCtx, ch, conv, s.rules, s.allManifests, req.Content, copiedResources)
		resultCh <- loopResult{q: q, ok: ok, err: lerr}
	}()

	// Consumer: bombea eventos al SSE y espera que el runLoop termine.
	var result loopResult
	loopDone := false
	for !loopDone {
		select {
		case evt, ok := <-events:
			if !ok {
				continue
			}
			t, _ := evt["type"].(string)
			sse.emit(t, evt)
		case result = <-resultCh:
			loopDone = true
		case <-r.Context().Done():
			loopCancel()
			cancel()
			return
		}
	}

	// El loop terminó: dar 200ms para que los tailers drenen los últimos
	// eventos pendientes (assistant_final, turn_end).
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	go func() {
		<-drainCtx.Done()
		cancel()
	}()
	for {
		select {
		case evt := <-events:
			if evt == nil {
				goto finalize
			}
			t, _ := evt["type"].(string)
			sse.emit(t, evt)
		case <-drainCtx.Done():
			goto finalize
		}
	}
finalize:
	drainCancel()
	tailerWG.Wait()

	if result.err != nil {
		sse.emit("error", map[string]string{"error": result.err.Error()})
		sse.emit("done", map[string]bool{"idle": true})
		return
	}

	if !result.ok {
		sse.emit("done", map[string]bool{"idle": true})
		return
	}

	frameworkMsg := Message{
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      result.q.Framework,
		Content:        result.q.Text,
		Reasoning:      result.q.Reasoning,
		QuestionID:     result.q.ID,
		AskVia:         result.q.AskVia,
		SuggestedChips: result.q.Chips,
		Timestamp:      time.Now(),
	}
	_ = appendMessage(conv.ID, frameworkMsg)
	sse.emit("framework_message", frameworkMsg)
	sse.emit("done", map[string]bool{"idle": false})
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
	// vía Sabio cuando `To` no viene en el request.
	EntityType string `json:"entity_type,omitempty"`
	EntityRef  string `json:"entity_ref,omitempty"`
	Channel    string `json:"channel,omitempty"`
	ConvID     string `json:"conv_id,omitempty"`
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
	cmd := exec.Command(resolveVaultBin(), "set", "--conv", convOrDefault(convID), "--key", "credentials.smtp", "--stdin")
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
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
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

	// Sanitización defensiva del subject: deduplicar prefijos repetidos
	// ("Cobranza: Cobranza: Foo" → "Cobranza: Foo"). El frontend genera el
	// subject vía LLM y puede repetir el namespace.
	req.Subject = dedupeSubjectPrefix(req.Subject)

	// Validar que el body no contenga placeholders sin resolver. Los drafts
	// del LLM a veces dejan `[Tu nombre]`, `[Fecha]` etc. Si llegan al SMTP
	// se envían tal cual al cliente. Mejor 422 + lista al frontend para que
	// pinte el botón en rojo.
	if missing := unresolvedPlaceholders(req.Body); len(missing) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, APIResponse{
			Success: false,
			Data: map[string]interface{}{
				"unresolved_placeholders": missing,
			},
			Error: "placeholders sin resolver: " + strings.Join(missing, ", "),
		})
		return
	}

	// Resolver destinatario vía Sabio si no vino `to` en el
	// request pero sí vino entity_type+entity_ref. Si tampoco hay contacto,
	// devolvemos 412 con provider_hint=sabio para que el frontend
	// dispare el flujo de captura/import.
	if req.To == "" && req.EntityType != "" && req.EntityRef != "" {
		res, err := contactosLookup(req.EntityType, req.EntityRef, req.Channel)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "sabio contact-lookup: "+err.Error())
			return
		}
		if res.Found {
			req.To = res.Value
		} else {
			writeJSON(w, http.StatusPreconditionFailed, APIResponse{
				Success: false,
				Data: map[string]interface{}{
					"missing_capability": res.MissingCapability,
					"provider_hint":      res.ProviderHint,
					"entity_type":        req.EntityType,
					"entity_ref":         req.EntityRef,
					"channel":            req.Channel,
				},
				Error: "contacto faltante; cargá email vía Sabio",
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
			"message_id":    msResp.MessageID,
			"to":            msResp.To,
			"original_to":   originalTo,
			"dev_rewritten": devRewritten,
			"channel":       msResp.Channel,
			"result_ref":    "message:" + msResp.MessageID,
		})
		// El ledger cambió: la próxima vez que el orchestrator pida active
		// task verá la siguiente en la fila, no la que se acaba de cerrar.
		invalidateActiveTaskCache()
	}
	writeOK(w, msResp)
}

// resolveMensajero localiza el binario `frameworkmensajero` y su cwd.
// Resolución: env REMORA_MENSAJERO_BIN, o ../framework-mensajero/frameworkmensajero
// relativo a REMORA_ROOT.
func resolveMensajero() (string, string, error) {
	root := resolveRemoraRoot()
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

// resolveVaultBin localiza el binario vault.
func resolveVaultBin() string {
	if v := os.Getenv("REMORA_VAULT_BIN"); v != "" {
		return v
	}
	root := resolveRemoraRoot()
	return filepath.Join(root, "channel", "bin", "vault")
}

// vaultHasFromAPI hace exit-code-check sin desencriptar.
func vaultHasFromAPI(convID, key string) bool {
	cmd := exec.Command(resolveVaultBin(), "has", "--conv", convOrDefault(convID), "--key", key)
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
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	writeOK(w, s.rules)
}

func (s *server) updateRules(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
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
	rulesPath := envOr("FLOW_RULES", "cmd/api_rest/flow.rules.json")
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
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	name := mux.Vars(r)["name"]

	m, ok := s.allManifests[name]
	if !ok {
		writeErr(w, http.StatusNotFound, "framework no encontrado: "+name)
		return
	}

	writeOK(w, m)
}

type runFrameworkCommandRequest struct {
	Params         map[string]string `json:"params"`
	ConversationID string            `json:"conversation_id"`
}

func (s *server) runFrameworkCommand(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireRemoraStaff(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	commandName := vars["command"]

	m, ok := s.allManifests[name]
	if !ok {
		writeErr(w, http.StatusNotFound, "framework no encontrado: "+name)
		return
	}
	cmd, ok := m.Commands[commandName]
	if !ok {
		writeErr(w, http.StatusNotFound, "comando no encontrado: "+commandName)
		return
	}

	var req runFrameworkCommandRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "body inválido")
			return
		}
	}
	if req.Params == nil {
		req.Params = map[string]string{}
	}
	businessID := req.Params["business_id"]
	if businessID == "" && commandNeedsBusinessContext(cmd) {
		writeErr(w, http.StatusBadRequest, "business_id requerido para este comando")
		return
	}
	runtimeContext := map[string]any{}
	if businessID != "" {
		membership, err := s.auth.membership(user.ID, businessID)
		if err != nil {
			writeErr(w, http.StatusForbidden, "usuario sin acceso al negocio: "+businessID)
			return
		}
		runtimeContext = contextFromMembership(user, membership, nil)
	}
	if commandHasParam(cmd, "business_id") {
		req.Params["business_id"] = businessID
	}
	if commandHasParam(cmd, "db") {
		req.Params["db"] = s.businessSQLitePath(businessID)
		if req.Params["db"] == "" {
			req.Params["db"] = businessDataDBPath(s.rootDir, businessID)
		}
	}
	if commandHasParam(cmd, "context_b64") {
		raw, _ := json.Marshal(runtimeContext)
		req.Params["context_b64"] = base64.RawURLEncoding.EncodeToString(raw)
	}
	convID := req.ConversationID
	if convID == "" {
		convID = fmt.Sprintf("fwtest_%s_%d", name, time.Now().UnixNano())
	}

	args, err := cmd.ResolveArgs(req.Params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + m.Name
	}
	cwd := filepath.Join(s.rootDir, cwdRel)

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	resp, err := s.scoped(convID).ExecuteCommand(ctx, m.Binary.Command, fullArgs, cwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{
		"conversation_id": convID,
		"framework":       name,
		"command":         commandName,
		"args":            fullArgs,
		"cwd":             cwd,
		"response":        resp,
	})
}

func frameworkIOPaths(root string, ports []manifest.IOPort) map[string]string {
	out := map[string]string{}
	for _, p := range ports {
		if p.Name == "" || p.Path == "" {
			continue
		}
		if filepath.IsAbs(p.Path) {
			out[p.Name] = p.Path
			continue
		}
		out[p.Name] = filepath.Join(root, p.Path)
	}
	return out
}

func commandNeedsBusinessContext(cmd manifest.Command) bool {
	return commandHasParam(cmd, "business_id") || commandHasParam(cmd, "db") || commandHasParam(cmd, "context_b64") || commandHasParam(cmd, "profile")
}

// ============================================
// SINGLE FRAMEWORK CONVERSATION
// ============================================

type createSingleConvRequest struct {
	Title      string            `json:"title"`
	Framework  string            `json:"framework"`
	Models     map[string]string `json:"models,omitempty"`
	BusinessID string            `json:"business_id,omitempty"`
	Context    map[string]any    `json:"context,omitempty"`
}

func (s *server) createSingleConversation(w http.ResponseWriter, r *http.Request) {
	var req createSingleConvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}

	m, manifestOK := s.allManifests[req.Framework]
	_, driverOK := driverRegistry[req.Framework]
	if !driverOK && (!manifestOK || !frameworkManifestTestable(m)) {
		writeErr(w, http.StatusBadRequest, "framework no testeable: "+req.Framework)
		return
	}
	businessID, runtimeContext, ok := s.requireMembershipContext(w, r, req.BusinessID, req.Context)
	if !ok {
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = businessID
	}
	req.Context = runtimeContext

	userID, _ := req.Context["remora_user_id"].(string)
	conv := &Conversation{
		ID:             fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		UserID:         userID,
		Title:          req.Title,
		Frameworks:     []string{req.Framework},
		Models:         req.Models,
		BusinessID:     req.BusinessID,
		RuntimeContext: req.Context,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
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

	drivers := driversFor(conv)
	for _, d := range drivers {
		if err := d.Init(ctx, ch, conv); err != nil {
			fmt.Fprintf(os.Stderr, "[api_rest] driver %s.Init error: %v\n", d.Name(), err)
		}
	}

	// Siempre iniciar la sesión del framework vía Channel JSON-RPC.
	// El framework lee su INITIAL_PROMPT.md y genera su primera respuesta.
	first, err := s.startFrameworkSession(ctx, ch, conv, req.Framework)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// first nunca debe ser nil: startFrameworkSession devuelve error explícito si falla.
	_ = appendMessage(conv.ID, *first)

	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: map[string]interface{}{
		"conversation":   conv,
		"first_question": first,
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
	if !s.requireConversationAccess(w, r, conv) {
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

	if len(conv.Frameworks) == 1 {
		frameworkMsg, err := s.sendFrameworkSessionMessage(ctx, ch, conv, conv.Frameworks[0], req.Content)
		if err != nil {
			fallback := frameworkSessionErrorMessage(conv.Frameworks[0], err)
			_ = appendMessage(id, fallback)
			writeOK(w, map[string]interface{}{
				"user_message":      userMsg,
				"framework_message": fallback,
				"idle":              false,
			})
			return
		}
		if err := appendMessage(id, *frameworkMsg); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, map[string]interface{}{
			"user_message":      userMsg,
			"framework_message": frameworkMsg,
			"idle":              false,
		})
		return
	}

	if len(driversFor(conv)) == 0 {
		m, ok := s.allManifests[conv.Frameworks[0]]
		if !ok || !frameworkManifestTestable(m) {
			writeErr(w, http.StatusBadRequest, "framework no testeable: "+conv.Frameworks[0])
			return
		}
		result, err := s.runUniversalSingle(ctx, ch, conv, m, req.Content, copiedResources)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		frameworkMsg := result.Message
		if err := appendMessage(id, frameworkMsg); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, map[string]interface{}{
			"user_message":      userMsg,
			"framework_message": frameworkMsg,
			"idle":              result.Idle,
		})
		return
	}

	q, ok, err := runLoop(ctx, ch, conv, s.rules, s.allManifests, req.Content, copiedResources)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !ok {
		fallback := singleNoQuestionMessage(conv)
		if err := appendMessage(id, fallback); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeOK(w, map[string]interface{}{
			"user_message":      userMsg,
			"framework_message": fallback,
			"idle":              false,
		})
		return
	}

	frameworkMsg := Message{
		ID:             generateMessageID(),
		Role:           "framework",
		Framework:      q.Framework,
		Content:        q.Text,
		Reasoning:      q.Reasoning,
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

func singleNoQuestionMessage(conv *Conversation) Message {
	framework := ""
	if len(conv.Frameworks) > 0 {
		framework = conv.Frameworks[0]
	}
	content := "No tengo una nueva pregunta estructurada para este turno, pero la sesión aislada sigue activa. Dame más contexto o una instrucción más concreta."
	if framework == "alfa" {
		content = "Alfa compila especificaciones desde un árbol de Echo. Para probarlo en aislamiento, pásame contexto de proceso o ejecuta primero Echo para generar evidencia; si no hay árbol Echo validado, no puedo producir una pregunta útil todavía."
	}
	if framework == "echo" {
		content = "Echo no emitió una nueva pregunta estructurada. Cuéntame el proceso, tarea repetitiva o dolor que quieres descubrir y continuaré desde ahí."
	}
	return Message{
		ID:        generateMessageID(),
		Role:      "framework",
		Framework: framework,
		Content:   content,
		Reasoning: "Single-session fallback: el driver no devolvió next-question válido, así que la API evita idle silencioso y mantiene la conversación estándar.",
		Status:    "needs_input",
		Events: []MessageEvent{{
			Type:      "framework.needs_input",
			Framework: framework,
			Message:   "driver returned no next question",
		}},
		Timestamp: time.Now(),
	}
}

func frameworkSessionErrorMessage(framework string, err error) Message {
	msg := "Tu mensaje llegó, pero el framework no pudo terminar la acción. Lo dejé registrado; puedes intentar de nuevo o reformularlo en una sola respuesta."
	detail := ""
	if err != nil {
		detail = redactLiveEventText(err.Error())
	}
	return Message{
		ID:        generateMessageID(),
		Role:      "framework",
		Framework: framework,
		Content:   msg,
		Reasoning: "La API capturó un error de ejecución del framework_session y evitó dejar la conversación en silencio.",
		Status:    "needs_input",
		Events: []MessageEvent{{
			Type:      "framework.session_error",
			Framework: framework,
			Message:   detail,
		}},
		Timestamp: time.Now(),
	}
}

// ============================================

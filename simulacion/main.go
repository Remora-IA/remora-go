package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed autonomia-controlada.html
var indexHTML []byte

const (
	defaultClientID = "60"
	defaultSource   = "simulacion/panalbit.sqlite"
	groqURL         = "https://api.groq.com/openai/v1/chat/completions"
	groqModel       = "meta-llama/llama-4-scout-17b-16e-instruct"
)

type row map[string]string

type derivedContext struct {
	Source                             string               `json:"source"`
	Root                               rootInfo             `json:"root"`
	Profile                            profileInfo          `json:"profile"`
	Counts                             countsInfo           `json:"counts"`
	Mere                               []mereEdge           `json:"mere"`
	RelationMap                        []string             `json:"relationMap"`
	Distributions                      distributions        `json:"distributions"`
	PaymentStats                       paymentStats         `json:"paymentStats"`
	Coverage                           coverageInfo         `json:"coverage"`
	ScopeMetrics                       scopeMetrics         `json:"scopeMetrics"`
	AtomicFacts                        []atomicFact         `json:"atomicFacts"`
	PendingDetails                     []pendingDetail      `json:"pendingDetails"`
	ActiveProjectChargeRows            []projectChargeRow   `json:"activeProjectChargeRows"`
	ActiveCollectableProjectChargeRows []projectChargeRow   `json:"activeCollectableProjectChargeRows"`
	AgreementChargeRows                []agreementChargeRow `json:"agreementChargeRows"`
	PendingAgreementRow                *agreementChargeRow  `json:"pendingAgreementRow,omitempty"`
	ActiveProjectNames                 []string             `json:"activeProjectNames"`
	TimelineAnomalies                  []pendingDetail      `json:"timelineAnomalies"`
	Highlights                         []string             `json:"highlights"`
	Gaps                               []string             `json:"gaps"`
}

type rootInfo struct {
	Entity string `json:"entity"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
}

type profileInfo struct {
	Name               string `json:"name"`
	Code               string `json:"code"`
	Active             bool   `json:"active"`
	AgreementStartDate string `json:"agreement_start_date"`
	Currency           string `json:"currency"`
	Language           string `json:"language"`
}

type countsInfo struct {
	Projects                  int `json:"projects"`
	Agreements                int `json:"agreements"`
	Charges                   int `json:"charges"`
	Documents                 int `json:"documents"`
	Payments                  int `json:"payments"`
	OpenCharges               int `json:"openCharges"`
	ActiveProjects            int `json:"activeProjects"`
	ActiveCollectableProjects int `json:"activeCollectableProjects"`
}

type mereEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

type countItem struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type distributions struct {
	TypeCounts        []countItem `json:"typeCounts"`
	AreaCounts        []countItem `json:"areaCounts"`
	ResponsibleCounts []countItem `json:"responsibleCounts"`
	ChargeYearCounts  []countItem `json:"chargeYearCounts"`
	PaymentYearCounts []countItem `json:"paymentYearCounts"`
	DocYearCounts     []countItem `json:"docYearCounts"`
}

type paymentStats struct {
	First     string  `json:"first"`
	Last      string  `json:"last"`
	Total     int     `json:"total"`
	TotalPaid float64 `json:"totalPaid"`
	Avg       float64 `json:"avg"`
	Max       float64 `json:"max"`
}

type coverageInfo struct {
	ChargesWithDocuments    int `json:"chargesWithDocuments"`
	ChargesWithoutDocuments int `json:"chargesWithoutDocuments"`
}

type scopeMetrics struct {
	Client                    scopeCount `json:"client"`
	ActiveProjects            scopeCount `json:"activeProjects"`
	ActiveCollectableProjects scopeCount `json:"activeCollectableProjects"`
}

type scopeCount struct {
	Projects   int `json:"projects"`
	Agreements int `json:"agreements,omitempty"`
	Charges    int `json:"charges"`
	Documents  int `json:"documents,omitempty"`
	Payments   int `json:"payments,omitempty"`
	Pending    int `json:"pending"`
	Paid       int `json:"paid,omitempty"`
}

type atomicFact struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
	Scope string `json:"scope"`
}

type pendingDetail struct {
	ChargeID           string  `json:"charge_id"`
	AgreementID        string  `json:"agreement_id"`
	State              string  `json:"state"`
	ChargeDate         string  `json:"charge_date"`
	Description        string  `json:"description"`
	ProjectCode        string  `json:"project_code"`
	ProjectName        string  `json:"project_name"`
	ProjectActive      string  `json:"project_active"`
	ProjectCollectable string  `json:"project_collectable"`
	DocumentNumber     string  `json:"document_number"`
	DocumentDate       string  `json:"document_date"`
	TimelineGapYears   float64 `json:"timeline_gap_years"`
}

type projectChargeRow struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Active      bool   `json:"active"`
	Collectable bool   `json:"collectable"`
	Type        string `json:"type"`
	Area        string `json:"area"`
	Charges     int    `json:"charges"`
	Pending     int    `json:"pending"`
	Paid        int    `json:"paid"`
}

type agreementChargeRow struct {
	AgreementID        string `json:"agreement_id"`
	ProjectCode        string `json:"project_code"`
	ProjectName        string `json:"project_name"`
	ProjectActive      bool   `json:"project_active"`
	ProjectCollectable bool   `json:"project_collectable"`
	Charges            int    `json:"charges"`
	Pending            int    `json:"pending"`
	Paid               int    `json:"paid"`
}

type agendaGroup struct {
	ID    string       `json:"id"`
	Title string       `json:"title"`
	Why   string       `json:"why"`
	Items []agendaItem `json:"items"`
}

type agendaItem struct {
	ID     string `json:"id,omitempty"`
	Text   string `json:"text"`
	Status string `json:"status,omitempty"`
}

type agendaProgress struct {
	ActiveQuestionID string                `json:"active_question_id"`
	ActiveItemIndex  int                   `json:"active_item_index"`
	Completed        []agendaCompletedItem `json:"completed"`
}

type agendaCompletedItem struct {
	QuestionID string `json:"question_id"`
	ItemIndex  int    `json:"item_index"`
}

type action struct {
	Label   string `json:"label"`
	Primary bool   `json:"primary,omitempty"`
	Muted   bool   `json:"muted,omitempty"`
}

type bar struct {
	Label  string `json:"label"`
	Pct    int    `json:"pct"`
	Val    string `json:"val,omitempty"`
	Accent bool   `json:"accent,omitempty"`
}

type metric struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type table struct {
	Header string     `json:"header,omitempty"`
	Rows   [][]string `json:"rows"`
}

type aiResponse struct {
	Phase          string          `json:"phase,omitempty"`
	Text           string          `json:"text,omitempty"`
	Agenda         []agendaGroup   `json:"agenda,omitempty"`
	AgendaProgress *agendaProgress `json:"agenda_progress,omitempty"`
	Bars           []bar           `json:"bars,omitempty"`
	Table          *table          `json:"table,omitempty"`
	Metrics        []metric        `json:"metrics,omitempty"`
	Actions        []action        `json:"actions,omitempty"`
}

type envelope struct {
	SessionID string        `json:"session_id"`
	Context   *uiContext    `json:"context,omitempty"`
	Agenda    []agendaGroup `json:"agenda,omitempty"`
	Phase     string        `json:"phase,omitempty"`
	Response  *aiResponse   `json:"response,omitempty"`
}

type chatRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type actionRequest struct {
	SessionID string `json:"session_id"`
	Label     string `json:"label"`
}

type groqConfigRequest struct {
	SessionID string `json:"session_id"`
	APIKey    string `json:"api_key"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type session struct {
	ID              string
	View            string
	Context         derivedContext
	Portfolio       portfolioSnapshot
	CurrentClientID string
	Agenda          []agendaGroup
	ChatHistory     []chatMessage
	Phase           string
	ScriptStep      int
	GroqKey         string
	LastIntent      string
	LastUserText    string
	LastResponseText string
}

type app struct {
	db          *sql.DB
	baseGroqKey string
	mu          sync.RWMutex
	sessions    map[string]*session
	httpClient  *http.Client
}

type groqRequestPayload struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type groqAPIResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error any `json:"error"`
}

func main() {
	dbPath := os.Getenv("PANALBIT_DB_PATH")
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultSimulationDBPath()
	}
	groqKey, groqKeySource, err := resolveGroqKey()
	if err != nil {
		log.Fatalf("resolve groq api key: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping sqlite: %v", err)
	}

	server := &app{
		db:          db,
		baseGroqKey: groqKey,
		sessions:    map[string]*session{},
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/api/bootstrap", server.handleBootstrap)
	mux.HandleFunc("/api/chat", server.handleChat)
	mux.HandleFunc("/api/action", server.handleAction)
	mux.HandleFunc("/api/config/groq", server.handleGroqConfig)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	addr := ":8091"
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			addr = port
		} else {
			addr = ":" + port
		}
	}
	log.Printf("autonomia-controlada escuchando en http://localhost%s", addr)
	log.Printf("usando sqlite en %s", dbPath)
	log.Printf("groq api key cargada desde %s", groqKeySource)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func defaultSimulationDBPath() string {
	execPath, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(execPath), "panalbit.sqlite")
	}
	return "panalbit.sqlite"
}

func resolveGroqKey() (string, string, error) {
	if key := strings.TrimSpace(os.Getenv("GROQ_API_KEY")); key != "" {
		return key, "variable de entorno GROQ_API_KEY", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	configPath := filepath.Join(home, ".config", "remora-go-lite", "simulacion.env")
	file, err := os.Open(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", errors.New("GROQ_API_KEY no configurada; define la variable de entorno o crea ~/.config/remora-go-lite/simulacion.env")
		}
		return "", "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "GROQ_API_KEY" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if value != "" {
			return value, configPath, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	return "", "", fmt.Errorf("GROQ_API_KEY no encontrada en %s", configPath)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (a *app) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	portfolio, err := loadPortfolioSnapshot(a.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s := &session{
		ID:        id,
		View:      "portfolio",
		Portfolio: portfolio,
		Agenda:    normalizeAgenda(buildPortfolioAgenda(), derivedContext{}),
		Phase:     "exploring",
	}
	if clientID != "" {
		ctx, err := loadDerivedContext(a.db, clientID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.View = "debtor"
		s.Context = ctx
		s.CurrentClientID = clientID
		if item, ok := s.Portfolio.ByClientID[clientID]; ok {
			s.Agenda = normalizeAgenda(buildDebtor360Agenda(ctx, item), ctx)
		}
	}
	a.mu.Lock()
	a.sessions[id] = s
	a.mu.Unlock()
	ui := buildUIContext(s)
	writeJSON(w, http.StatusOK, envelope{SessionID: id, Context: &ui, Agenda: s.Agenda, Phase: s.Phase})
}

func (a *app) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s, err := a.getSession(req.SessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	resp, err := a.respondToChat(s, req.Text)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ui := buildUIContext(s)
	writeJSON(w, http.StatusOK, envelope{SessionID: s.ID, Context: &ui, Agenda: s.Agenda, Phase: s.Phase, Response: resp})
}

func (a *app) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s, err := a.getSession(req.SessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	resp, err := a.respondToAction(s, req.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ui := buildUIContext(s)
	writeJSON(w, http.StatusOK, envelope{SessionID: s.ID, Context: &ui, Agenda: s.Agenda, Phase: s.Phase, Response: resp})
}

func (a *app) handleGroqConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req groqConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s, err := a.getSession(req.SessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	key := strings.TrimSpace(req.APIKey)
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("api_key vacío"))
		return
	}
	s.GroqKey = key
	resp := &aiResponse{
		Phase:   "exploring",
		Text:    "API key guardada en esta sesión. Ya puedo continuar el análisis dinámico desde el backend en Go.",
		Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ver priorización de hoy", Primary: true}, {Label: "Analizar deudor prioritario"}},
	}
	resp = a.finalizeResponse(s, resp, "Ingresar API key")
	ui := buildUIContext(s)
	writeJSON(w, http.StatusOK, envelope{SessionID: s.ID, Context: &ui, Agenda: s.Agenda, Phase: s.Phase, Response: resp})
}

func (a *app) respondToChat(s *session, text string) (*aiResponse, error) {
	prompt := strings.TrimSpace(text)
	if prompt == "" {
		return nil, errors.New("mensaje vacío")
	}
	resp, err := a.deterministicResponseForPrompt(s, prompt)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		return a.finalizeResponse(s, resp, prompt), nil
	}
	resp, err = a.callGroq(s, prompt)
	if err != nil {
		return nil, err
	}
	return a.finalizeResponse(s, resp, prompt), nil
}

func (a *app) respondToAction(s *session, label string) (*aiResponse, error) {
	text := strings.TrimSpace(label)
	if text == "" {
		return nil, errors.New("acción vacía")
	}
	resp, err := a.deterministicResponseForAction(s, text)
	if resp == nil {
		resp, err = a.callGroq(s, text)
		if err != nil {
			return nil, err
		}
	}
	return a.finalizeResponse(s, resp, text), nil
}

func (a *app) finalizeResponse(s *session, resp *aiResponse, userText string) *aiResponse {
	if resp == nil {
		return nil
	}
	applyResponseMeta(s, resp)
	if err := a.maybeSynthesizeResponseText(s, resp, userText); err != nil {
		log.Printf("grounded synthesis fallback: %v", err)
	}
	postProcessResponse(s, resp, userText)
	if resp.Phase == "" {
		resp.Phase = s.Phase
	}
	s.Phase = resp.Phase
	resp.Actions = normalizeActions(resp.Actions)
	s.LastUserText = userText
	s.LastResponseText = resp.Text
	return resp
}

func (a *app) callGroq(s *session, userText string) (*aiResponse, error) {
	apiKey := strings.TrimSpace(s.GroqKey)
	if apiKey == "" {
		apiKey = a.baseGroqKey
	}
	if apiKey == "" {
		return &aiResponse{
			Phase:   "exploring",
			Text:    "No hay API key configurada. Puedo seguir operando la cartera y el diagnóstico 360 con lógica determinística del backend, y si quieres análisis libre sobre el deudor actual puedo usar Groq cuando configures la key.",
			Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Ingresar API key", Primary: true}, {Label: "Volver a cartera"}},
		}, nil
	}
	isFirstTurn := len(s.ChatHistory) == 0
	continueHint := ""
	if normalizeLabel(userText) == "seguir analizando" && len(s.Agenda) > 0 {
		continueHint = "\n\n" + currentAgendaFocusText(s.Agenda)
	}
	effectiveUserText := userText
	systemPrompt := buildSystemPrompt(s.Context, s.Agenda)
	if s.View == "portfolio" {
		systemPrompt = buildPortfolioSystemPrompt(s)
	}
	if isFirstTurn && s.View == "portfolio" {
		effectiveUserText = fmt.Sprintf("Primer turno del usuario: %q.\n\nSi es saludo, small talk o una pregunta meta como quién eres, cómo estás o qué día es hoy, responde natural y breve sin volcarte a la cartera. Si es una pregunta de cartera, usa el contexto gerencial y responde grounded con la evidencia disponible.%s", userText, continueHint)
	} else if isFirstTurn {
		effectiveUserText = fmt.Sprintf("Primer turno del usuario: %q.\n\nAntes de entrar al análisis duro, reconoce el acto conversacional del usuario en 1 frase breve. Si es saludo, saluda. Si es una duda puntual, respóndela brevemente con base en el caso. Después conecta esa respuesta con la agenda de análisis y mantén el foco en cobranzas.%s", userText, continueHint)
	} else {
		effectiveUserText = userText + continueHint
	}
	s.ChatHistory = append(s.ChatHistory, chatMessage{Role: "user", Content: effectiveUserText})
	payload := groqRequestPayload{
		Model:          groqModel,
		Messages:       append([]chatMessage{{Role: "system", Content: systemPrompt}}, s.ChatHistory...),
		Temperature:    0.7,
		MaxTokens:      4096,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, groqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := a.httpClient.Do(req)
	if err != nil {
		return &aiResponse{
			Phase:   "exploring",
			Text:    "Error de conexión. Puedo seguir analizando con el contexto ya cargado o pausar el caso.",
			Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Reintentar análisis", Primary: true}, {Label: "Pausar este caso"}},
		}, nil
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var groqResp groqAPIResponse
	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return nil, err
	}
	if groqResp.Error != nil {
		return &aiResponse{
			Phase:   "exploring",
			Text:    "Groq devolvió un error. Puedo pausar el caso o reintentar cuando la key y el servicio estén listos.",
			Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Reintentar análisis", Primary: true}, {Label: "Pausar este caso"}},
		}, nil
	}
	if len(groqResp.Choices) == 0 {
		return nil, errors.New("respuesta vacía de Groq")
	}
	content := groqResp.Choices[0].Message.Content
	s.ChatHistory = append(s.ChatHistory, chatMessage{Role: "assistant", Content: content})
	var parsed aiResponse
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		return &parsed, nil
	}
	match := regexp.MustCompile(`\{[\s\S]*\}`).FindString(content)
	if strings.TrimSpace(match) != "" {
		if err := json.Unmarshal([]byte(match), &parsed); err == nil {
			return &parsed, nil
		}
	}
	return &aiResponse{
		Phase:   "exploring",
		Text:    content,
		Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Actuar con la información disponible", Primary: true}, {Label: "Pausar este caso"}},
	}, nil
}

func buildPortfolioSystemPrompt(s *session) string {
	now := time.Now()
	today := now.Format("2006-01-02 15:04 MST")
	ctx := map[string]any{
		"today":              today,
		"assistant_name":     "Panalbit AI",
		"view":               s.View,
		"portfolio":          s.Portfolio.Context,
		"selected_client_id": s.CurrentClientID,
		"last_user_text":     s.LastUserText,
		"last_response_text": s.LastResponseText,
		"agenda":             s.Agenda,
	}
	return fmt.Sprintf(`Eres Panalbit AI, un asistente conversacional de cobranza y análisis gerencial. Hablas en español natural.

## CONTEXTO
%s

## COMPORTAMIENTO
- Responde como un LLM útil y natural, no como un menú rígido.
- Si el usuario saluda, pregunta cómo estás, pregunta tu nombre o pregunta qué día es hoy, responde normal y breve.
- Si el usuario hace una pregunta gerencial sobre cartera, usa SOLO la evidencia del contexto.
- Si no existe una entidad en la fuente (por ejemplo promesas o gestiones), dilo con claridad y ofrece el análogo más cercano soportado por la base.
- No inventes fechas de vencimiento, promesas, gestiones, mora exacta ni montos no presentes.
- Si el usuario hace un follow-up ambiguo como "y eso?", "por qué?" o "explícame", aclara el último resultado en vez de resetear la conversación.

## FORMATO
Devuelve JSON válido con esta forma:
{
  "phase": "exploring|synthesizing|deciding|acting|done",
  "text": "HTML breve y claro",
  "agenda": [],
  "agenda_progress": {"active_question_id":"priority|debtor|chat","active_item_index":0,"completed":[]},
  "bars": [],
  "table": {"header":"", "rows":[]},
  "metrics": [],
  "actions": [
    {"label":"Seguir analizando"},
    {"label":"Ver priorización de hoy","primary":true},
    {"label":"Analizar deudor prioritario"}
  ]
}

## REGLAS DE SALIDA
- Si la pregunta no requiere tabla ni métricas, déjalas vacías.
- En small talk o meta-conversación, no metas cartera innecesariamente.
- Actions debe tener exactamente 3 opciones.
- Mantén la continuidad con la conversación previa.`, mustJSON(ctx))
}

func (a *app) hasGroqKey(s *session) bool {
	return strings.TrimSpace(s.GroqKey) != "" || strings.TrimSpace(a.baseGroqKey) != ""
}

func (a *app) groqKeyForSession(s *session) string {
	if strings.TrimSpace(s.GroqKey) != "" {
		return strings.TrimSpace(s.GroqKey)
	}
	return strings.TrimSpace(a.baseGroqKey)
}

func (a *app) maybeSynthesizeResponseText(s *session, resp *aiResponse, userText string) error {
	if resp == nil || strings.TrimSpace(resp.Text) == "" || !a.hasGroqKey(s) || isSmallTalkPrompt(userText) || isMetaQuestionPrompt(userText) {
		return nil
	}
	evidence := groundedEvidenceForResponse(s, resp, userText)
	body, err := json.Marshal(groqRequestPayload{
		Model:       groqModel,
		Temperature: 0.4,
		MaxTokens:   700,
		Messages: []chatMessage{
			{
				Role: "system",
				Content: `Eres Panalbit AI. Tu tarea es redactar únicamente el campo "text" de una respuesta grounded.

Reglas:
- Usa SOLO la evidencia JSON entregada.
- No inventes entidades, montos, promesas, gestiones, mora exacta ni datos ausentes.
- No cambies cifras ni contradigas la tabla/métricas.
- Responde en español natural, 2-4 párrafos, usando <strong> solo cuando aporte.
- Si el usuario escribió algo ambiguo como "y eso?", "por qué?" o "explícame", aclara el último resultado en vez de reiniciar desde cero.
- Si es saludo o cortesía, responde breve pero mantén el contexto actual.
- Devuelve solo HTML breve, sin JSON, sin markdown, sin fences.`,
			},
			{
				Role:    "user",
				Content: mustJSON(evidence),
			},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, groqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.groqKeyForSession(s))
	req.Header.Set("Content-Type", "application/json")
	res, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	var groqResp groqAPIResponse
	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return err
	}
	if groqResp.Error != nil {
		return fmt.Errorf("groq synthesis error")
	}
	if len(groqResp.Choices) == 0 {
		return errors.New("respuesta vacía de Groq en síntesis")
	}
	text := strings.TrimSpace(groqResp.Choices[0].Message.Content)
	if text != "" {
		resp.Text = text
	}
	return nil
}

func groundedEvidenceForResponse(s *session, resp *aiResponse, userText string) map[string]any {
	evidence := map[string]any{
		"user_text":          userText,
		"last_user_text":     s.LastUserText,
		"last_response_text": s.LastResponseText,
		"view":               s.View,
		"phase":              resp.Phase,
		"intent":             s.LastIntent,
		"actions":            resp.Actions,
		"metrics":            resp.Metrics,
		"agenda_focus":       currentAgendaFocusText(s.Agenda),
	}
	if ui := buildUIContext(s); true {
		evidence["ui_context"] = ui
	}
	if resp.Table != nil {
		evidence["table"] = compactTable(*resp.Table, 12)
	}
	if len(resp.Bars) > 0 {
		evidence["bars"] = resp.Bars
	}
	evidence["deterministic_draft"] = resp.Text
	return evidence
}

func compactTable(tbl table, limit int) map[string]any {
	rows := tbl.Rows
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return map[string]any{
		"header":     tbl.Header,
		"row_count":  len(tbl.Rows),
		"sample_rows": rows,
	}
}

func buildSystemPrompt(ctx derivedContext, agenda []agendaGroup) string {
	return fmt.Sprintf(`Eres un asistente experto en análisis de operaciones y cobranza. Tu trabajo es razonar con el mismo idioma que el desarrollador: entidades, relaciones, cobertura, vacíos y nodos problemáticos. Hablas en español, en primera persona, con tono de analista senior.

## REGLA CENTRAL
Solo puedes usar los hechos del contexto semántico y la fuente delimitada. Si algo no está soportado, dilo. No inventes mora, riesgo legal, vencimientos, montos pendientes ni score si no están derivados explícitamente.

## FUENTE
%s

## MERE
%s

## PERFIL
%s

## HECHOS DERIVADOS
%s

## AGENDA ACTUAL
%s

## INSTRUCCIONES DE RAZONAMIENTO
- En el primer turno, primero reconoce la intención del usuario en 1 frase breve y natural. Si es saludo, saluda. Si es una duda puntual, respóndela brevemente.
- Después de reconocer el input, conecta la conversación con el análisis del caso y proponle el siguiente ángulo útil de cobranzas.
- La agenda no es decorativa: es tu plan de trabajo. En cada turno debes responder al usuario y, además, avanzar la agenda.
- Trabaja una pregunta principal por vez y, dentro de ella, intenta cerrar las subpreguntas una por una antes de saltar a la siguiente principal.
- Si el usuario pregunta algo que pertenece a otra parte de la agenda, respóndelo sin perder foco y mapea esa respuesta al punto de agenda más cercano.
- Si el usuario pulsa "Seguir analizando", continúa sobre la subpregunta activa o, si ya quedó respondida, abre la siguiente subpregunta pendiente más útil.
- Evita repetir textualmente hechos ya dichos en el turno inmediatamente anterior, salvo que sean necesarios para sostener una conclusión nueva.
- Cuando avances la agenda, prioriza aportar el delta: qué respuesta nueva cerraste, qué cambió la lectura y qué pregunta sigue.
- Si el turno es social o muy breve (por ejemplo saludo, cortesía o "cómo estás"), responde de forma corta y no llenes la UI con tablas, barras o métricas.
- Usa el MERE para ubicar dónde está concentrado el problema.
- Distingue hechos confirmados, inferencias razonables y gaps reales.
- Prioriza trazabilidad cliente→proyecto→acuerdo→cargo→documento→pago.
- Si el problema está localizado, dilo explícitamente.
- Si detectas anomalías temporales o de gobernanza de datos, destácalas.
- Nunca uses conteos del cliente completo para describir subconjuntos activos o cobrables.
- Si existe un conteo por scope en scopeMetrics o activeProjectChargeRows, ese número manda sobre cualquier total agregado.
- "Cargos del cliente", "cargos en proyectos activos" y "cargos en proyectos activos y cobrables" son universos distintos.
- Si un hallazgo depende de interpretación y no de una relación confirmada, baja la confianza y dilo como inferencia.
- Si el usuario pide actuar, simula el siguiente paso; no ejecutes nada real.

## FORMATO DE RESPUESTA
Devuelve JSON válido con esta forma exacta:
{
  "phase": "exploring|synthesizing|deciding|acting|done",
  "text": "HTML breve y claro. Usa <strong>. 3-6 párrafos.",
  "agenda": [
    {
      "id": "focus",
      "title": "Pregunta principal 1",
      "why": "Por qué esta pregunta importa para cobranzas",
      "items": [{"text": "Subpregunta 1"}, {"text": "Subpregunta 2"}, {"text": "Subpregunta 3"}]
    },
    {
      "id": "urgency",
      "title": "Pregunta principal 2",
      "why": "Por qué esta pregunta importa para cobranzas",
      "items": [{"text": "Subpregunta 1"}, {"text": "Subpregunta 2"}, {"text": "Subpregunta 3"}]
    },
    {
      "id": "action",
      "title": "Pregunta principal 3",
      "why": "Por qué esta pregunta importa para cobranzas",
      "items": [{"text": "Subpregunta 1"}, {"text": "Subpregunta 2"}, {"text": "Subpregunta 3"}]
    }
  ],
  "agenda_progress": {
    "active_question_id": "focus|urgency|action",
    "active_item_index": 0,
    "completed": [{"question_id": "focus|urgency|action", "item_index": 0}]
  },
  "bars": [{"label": "Cobertura documental", "pct": 99, "val": "79/79", "accent": true}],
  "table": {"header": "Hallazgos", "rows": [["Item", "Valor", "Lectura"]]},
  "metrics": [{"value": "79", "label": "Cargos"}],
  "actions": [
    {"label": "Seguir analizando"},
    {"label": "Acción concreta", "primary": true},
    {"label": "Pausar este caso", "muted": true}
  ]
}

## REGLAS DE UI
- actions debe tener EXACTAMENTE 3 opciones.
- Una debe llamarse exactamente "Seguir analizando".
- No uses más de una acción primaria.
- No emitas métricas monetarias pendientes si no existen en la fuente.
- No sugieras montos, due dates, contactos comerciales o intentos previos de contacto como si existieran si la fuente no los trae.
- En el primer análisis genera agenda con exactamente 3 preguntas principales y 3 subpreguntas por cada una.
- En turnos siguientes reutiliza la misma agenda si sigue siendo válida y solo ajústala si mejora claramente el análisis.
- Devuelve agenda_progress en todos los turnos.
- Si completas una subpregunta, márcala en completed y activa la siguiente más útil.
- No declares completada una subpregunta si tu texto no la responde de forma suficiente.
- Cierra los turnos analíticos con una invitación breve a seguir la siguiente subpregunta activa de la agenda.
- Responde en español.`,
		ctx.Source,
		mustJSON(ctx.Mere),
		mustJSON(ctx.Profile),
		mustJSON(map[string]any{
			"counts":                             ctx.Counts,
			"coverage":                           ctx.Coverage,
			"scopeMetrics":                       ctx.ScopeMetrics,
			"atomicFacts":                        ctx.AtomicFacts,
			"paymentStats":                       ctx.PaymentStats,
			"activeProjectChargeRows":            ctx.ActiveProjectChargeRows,
			"activeCollectableProjectChargeRows": ctx.ActiveCollectableProjectChargeRows,
			"agreementChargeRows":                ctx.AgreementChargeRows,
			"pendingAgreementRow":                ctx.PendingAgreementRow,
			"pendingDetails":                     ctx.PendingDetails,
			"distributions":                      ctx.Distributions,
			"relationMap":                        ctx.RelationMap,
			"highlights":                         ctx.Highlights,
			"gaps":                               ctx.Gaps,
			"timelineAnomalies":                  ctx.TimelineAnomalies,
		}),
		agendaStateForPrompt(agenda),
	)
}

func loadDerivedContext(db *sql.DB, clientID string) (derivedContext, error) {
	clients, err := namedQueryRows(db, `select coalesce(id,''), coalesce(code,''), coalesce(name,''), coalesce(active,''), coalesce(agreement_start_date,''), coalesce(created_at,''), coalesce(updated_at,'') from clients where id = ?`, []string{"id", "code", "name", "active", "agreement_start_date", "created_at", "updated_at"}, clientID)
	if err != nil {
		return derivedContext{}, err
	}
	if len(clients) == 0 {
		return derivedContext{}, fmt.Errorf("cliente %s no encontrado", clientID)
	}
	client := clients[0]
	projects, err := namedQueryRows(db, `select coalesce(id,''), coalesce(agreement_id,''), coalesce(client_id,''), coalesce(code,''), coalesce(name,''), coalesce(active,''), coalesce(collectable,''), coalesce(currency_code,''), coalesce(language_name,''), coalesce(project_type_id,''), coalesce(project_area_id,''), coalesce(responsible_user_ids,''), coalesce(created_at,''), coalesce(updated_at,'') from projects where client_id = ? order by cast(id as integer)`, []string{"id", "agreement_id", "client_id", "code", "name", "active", "collectable", "currency_code", "language_name", "project_type_id", "project_area_id", "responsible_user_ids", "created_at", "updated_at"}, clientID)
	if err != nil {
		return derivedContext{}, err
	}
	agreements, err := namedQueryRows(db, `select coalesce(client_id,''), coalesce(code,''), coalesce(id,''), coalesce(name,'') from agreements where client_id = ? order by cast(id as integer)`, []string{"client_id", "code", "id", "name"}, clientID)
	if err != nil {
		return derivedContext{}, err
	}
	charges, err := namedQueryRows(db, `select coalesce(agreement_id,''), coalesce(client_code,''), coalesce(client_id,''), coalesce(created_at,''), coalesce(date_from,''), coalesce(date_to,''), coalesce(description,''), coalesce(id,''), coalesce(state,''), coalesce(updated_at,'') from charges where client_id = ? order by cast(id as integer)`, []string{"agreement_id", "client_code", "client_id", "created_at", "date_from", "date_to", "description", "id", "state", "updated_at"}, clientID)
	if err != nil {
		return derivedContext{}, err
	}
	docs, err := namedQueryRows(db, `select coalesce(charge_id,''), coalesce(client_code,''), coalesce(client_id,''), coalesce(created_at,''), coalesce(date,''), coalesce(id,''), coalesce(number,''), coalesce(series_number,''), coalesce(updated_at,'') from billing_documents where client_id = ? order by cast(id as integer)`, []string{"charge_id", "client_code", "client_id", "created_at", "date", "id", "number", "series_number", "updated_at"}, clientID)
	if err != nil {
		return derivedContext{}, err
	}
	payments, err := namedQueryRows(db, `select coalesce(amount,''), coalesce(client_id,''), coalesce(created_at,''), coalesce(currency_id,''), coalesce(date,''), coalesce(id,''), coalesce(residue,''), coalesce(updated_at,'') from payments where client_id = ? order by date, cast(id as integer)`, []string{"amount", "client_id", "created_at", "currency_id", "date", "id", "residue", "updated_at"}, clientID)
	if err != nil {
		return derivedContext{}, err
	}
	projectTypes, err := namedQueryRows(db, `select coalesce(id,''), coalesce(name,'') from project_types order by cast(id as integer)`, []string{"id", "name"})
	if err != nil {
		return derivedContext{}, err
	}
	areas, err := namedQueryRows(db, `select coalesce(id,''), coalesce(name,'') from areas order by cast(id as integer)`, []string{"id", "name"})
	if err != nil {
		return derivedContext{}, err
	}
	users, err := namedQueryRows(db, `select coalesce(id,''), coalesce(name,''), coalesce(active,'') from users order by cast(id as integer)`, []string{"id", "name", "active"})
	if err != nil {
		return derivedContext{}, err
	}

	projectTypeByID := make(map[string]string, len(projectTypes))
	for _, r := range projectTypes {
		projectTypeByID[r["id"]] = r["name"]
	}
	areaByID := make(map[string]string, len(areas))
	for _, r := range areas {
		areaByID[r["id"]] = r["name"]
	}
	userByID := make(map[string]row, len(users))
	for _, r := range users {
		userByID[r["id"]] = r
	}
	activeProjects := filterRows(projects, func(r row) bool { return r["active"] == "1" })
	collectableProjects := filterRows(projects, func(r row) bool { return r["collectable"] == "1" })
	activeCollectableProjects := filterRows(projects, func(r row) bool { return r["active"] == "1" && r["collectable"] == "1" })
	_ = collectableProjects
	openCharges := filterRows(charges, func(r row) bool { return r["state"] == "FACTURADO" })
	docByChargeID := map[string]row{}
	for _, d := range docs {
		docByChargeID[d["charge_id"]] = d
	}
	chargesWithoutDoc := filterRows(charges, func(r row) bool {
		_, ok := docByChargeID[r["id"]]
		return !ok
	})
	projectsByAgreement := map[string]row{}
	for _, p := range projects {
		projectsByAgreement[p["agreement_id"]] = p
	}
	pendingDetails := make([]pendingDetail, 0, len(openCharges))
	for _, ch := range openCharges {
		project := projectsByAgreement[ch["agreement_id"]]
		doc := docByChargeID[ch["id"]]
		embeddedDates := extractEmbeddedDates(ch["description"])
		chargeDate := parseDate(ch["date_to"])
		largestGap := 0.0
		if !chargeDate.IsZero() && len(embeddedDates) > 0 {
			for _, d := range embeddedDates {
				gap := math.Abs(chargeDate.Sub(d).Hours()) / 24 / 365
				if gap > largestGap {
					largestGap = gap
				}
			}
		}
		pendingDetails = append(pendingDetails, pendingDetail{
			ChargeID:           ch["id"],
			AgreementID:        ch["agreement_id"],
			State:              ch["state"],
			ChargeDate:         ch["date_to"],
			Description:        ch["description"],
			ProjectCode:        project["code"],
			ProjectName:        project["name"],
			ProjectActive:      project["active"],
			ProjectCollectable: project["collectable"],
			DocumentNumber:     doc["number"],
			DocumentDate:       doc["date"],
			TimelineGapYears:   math.Round(largestGap*10) / 10,
		})
	}
	typeCounts := groupCount(projects, func(r row) string { return projectTypeByID[r["project_type_id"]] }, func(raw string, _ row) string {
		if strings.TrimSpace(raw) == "" {
			return "Sin tipo"
		}
		return raw
	})
	areaCounts := groupCount(projects, func(r row) string { return areaByID[r["project_area_id"]] }, func(raw string, _ row) string {
		if strings.TrimSpace(raw) == "" {
			return "Sin área"
		}
		return raw
	})
	type responsibleLink struct {
		Project row
		ID      string
	}
	responsibleLinks := []responsibleLink{}
	for _, p := range projects {
		ids := safeJSONStringSlice(p["responsible_user_ids"])
		if len(ids) == 0 {
			responsibleLinks = append(responsibleLinks, responsibleLink{Project: p, ID: "__none__"})
			continue
		}
		for _, id := range ids {
			responsibleLinks = append(responsibleLinks, responsibleLink{Project: p, ID: id})
		}
	}
	responsibleCounts := groupCount(responsibleLinks, func(link responsibleLink) string { return link.ID }, func(id string, _ responsibleLink) string {
		if id == "__none__" {
			return "Sin responsable"
		}
		if user, ok := userByID[id]; ok && strings.TrimSpace(user["name"]) != "" {
			return user["name"]
		}
		return "Usuario " + id
	})
	chargeYearCounts := groupCount(charges, func(r row) string { return yearPrefix(r["date_to"]) }, func(raw string, _ row) string { return fallbackLabel(raw, "Sin dato") })
	paymentYearCounts := groupCount(payments, func(r row) string { return yearPrefix(r["date"]) }, func(raw string, _ row) string { return fallbackLabel(raw, "Sin dato") })
	docYearCounts := groupCount(docs, func(r row) string { return yearPrefix(r["date"]) }, func(raw string, _ row) string { return fallbackLabel(raw, "Sin dato") })
	agreementNameCoverage := 0
	for _, a := range agreements {
		if strings.TrimSpace(a["name"]) != "" {
			agreementNameCoverage++
		}
	}
	projectsWithoutResponsible := 0
	projectsWithoutArea := 0
	for _, p := range projects {
		if len(safeJSONStringSlice(p["responsible_user_ids"])) == 0 {
			projectsWithoutResponsible++
		}
		if strings.TrimSpace(p["project_area_id"]) == "" {
			projectsWithoutArea++
		}
	}
	timelineAnomalies := []pendingDetail{}
	for _, pd := range pendingDetails {
		if pd.TimelineGapYears >= 3 {
			timelineAnomalies = append(timelineAnomalies, pd)
		}
	}
	activeProjectChargeRows := make([]projectChargeRow, 0, len(activeProjects))
	for _, project := range activeProjects {
		related := chargesForProject(charges, project)
		pending := 0
		paid := 0
		for _, ch := range related {
			switch ch["state"] {
			case "FACTURADO":
				pending++
			case "PAGADO":
				paid++
			}
		}
		activeProjectChargeRows = append(activeProjectChargeRows, projectChargeRow{
			Code:        project["code"],
			Name:        project["name"],
			Active:      project["active"] == "1",
			Collectable: project["collectable"] == "1",
			Type:        fallbackLabel(projectTypeByID[project["project_type_id"]], "Sin tipo"),
			Area:        fallbackLabel(areaByID[project["project_area_id"]], "Sin área"),
			Charges:     len(related),
			Pending:     pending,
			Paid:        paid,
		})
	}
	activeCollectableProjectChargeRows := []projectChargeRow{}
	for _, p := range activeProjectChargeRows {
		if p.Collectable {
			activeCollectableProjectChargeRows = append(activeCollectableProjectChargeRows, p)
		}
	}
	agreementChargeRows := make([]agreementChargeRow, 0, len(agreements))
	for _, agreement := range agreements {
		related := filterRows(charges, func(r row) bool { return r["agreement_id"] == agreement["id"] })
		if len(related) == 0 {
			continue
		}
		project := projectsByAgreement[agreement["id"]]
		pending := 0
		paid := 0
		for _, ch := range related {
			switch ch["state"] {
			case "FACTURADO":
				pending++
			case "PAGADO":
				paid++
			}
		}
		agreementChargeRows = append(agreementChargeRows, agreementChargeRow{
			AgreementID:        agreement["id"],
			ProjectCode:        project["code"],
			ProjectName:        project["name"],
			ProjectActive:      project["active"] == "1",
			ProjectCollectable: project["collectable"] == "1",
			Charges:            len(related),
			Pending:            pending,
			Paid:               paid,
		})
	}
	sort.Slice(agreementChargeRows, func(i, j int) bool {
		if agreementChargeRows[i].Charges != agreementChargeRows[j].Charges {
			return agreementChargeRows[i].Charges > agreementChargeRows[j].Charges
		}
		if agreementChargeRows[i].Pending != agreementChargeRows[j].Pending {
			return agreementChargeRows[i].Pending > agreementChargeRows[j].Pending
		}
		return agreementChargeRows[i].AgreementID < agreementChargeRows[j].AgreementID
	})
	scope := scopeMetrics{
		Client:                    scopeCount{Projects: len(projects), Agreements: len(agreements), Charges: len(charges), Documents: len(docs), Payments: len(payments), Pending: len(openCharges)},
		ActiveProjects:            scopeCount{Projects: len(activeProjects), Charges: sumProjectCharges(activeProjectChargeRows), Pending: sumProjectPending(activeProjectChargeRows), Paid: sumProjectPaid(activeProjectChargeRows)},
		ActiveCollectableProjects: scopeCount{Projects: len(activeCollectableProjects), Charges: sumProjectCharges(activeCollectableProjectChargeRows), Pending: sumProjectPending(activeCollectableProjectChargeRows), Paid: sumProjectPaid(activeCollectableProjectChargeRows)},
	}
	var pendingAgreementRow *agreementChargeRow
	if len(pendingDetails) > 0 {
		for i := range agreementChargeRows {
			if agreementChargeRows[i].AgreementID == pendingDetails[0].AgreementID {
				rowCopy := agreementChargeRows[i]
				pendingAgreementRow = &rowCopy
				break
			}
		}
	}
	activeProjectNames := []string{}
	for _, row := range activeProjectChargeRows {
		if strings.TrimSpace(row.Name) != "" {
			activeProjectNames = append(activeProjectNames, row.Name)
		}
	}
	relationMap := []string{
		fmt.Sprintf("Cliente → Proyectos (%d)", len(projects)),
		fmt.Sprintf("Cliente → Acuerdos (%d)", len(agreements)),
		fmt.Sprintf("Cliente → Cargos (%d)", len(charges)),
		fmt.Sprintf("Cargos → Documentos (%d)", len(docs)),
		fmt.Sprintf("Cliente → Pagos (%d)", len(payments)),
		fmt.Sprintf("Proyectos activos → Cargos (%d)", scope.ActiveProjects.Charges),
		fmt.Sprintf("Proyectos activos y cobrables → Cargos (%d)", scope.ActiveCollectableProjects.Charges),
	}
	if pendingAgreementRow != nil && strings.TrimSpace(pendingAgreementRow.ProjectName) != "" {
		relationMap = append(relationMap, fmt.Sprintf("Cargo pendiente → Acuerdo %s → Proyecto %s", pendingAgreementRow.AgreementID, pendingAgreementRow.ProjectName))
	}
	atomicFacts := []atomicFact{
		{ID: "client_total_charges", Label: "Cargos totales del cliente", Value: scope.Client.Charges, Scope: "cliente"},
		{ID: "client_pending_charges", Label: "Cargos pendientes del cliente", Value: scope.Client.Pending, Scope: "cliente"},
		{ID: "active_projects_count", Label: "Proyectos activos", Value: scope.ActiveProjects.Projects, Scope: "activos"},
		{ID: "active_project_charges", Label: "Cargos en proyectos activos", Value: scope.ActiveProjects.Charges, Scope: "activos"},
		{ID: "active_collectable_projects_count", Label: "Proyectos activos y cobrables", Value: scope.ActiveCollectableProjects.Projects, Scope: "activo_cobrable"},
		{ID: "active_collectable_project_charges", Label: "Cargos en proyectos activos y cobrables", Value: scope.ActiveCollectableProjects.Charges, Scope: "activo_cobrable"},
		{ID: "projects_without_responsible", Label: "Proyectos sin responsable", Value: projectsWithoutResponsible, Scope: "gobernanza"},
		{ID: "timeline_anomalies", Label: "Anomalías temporales detectadas", Value: len(timelineAnomalies), Scope: "calidad_datos"},
	}
	highlights := []string{}
	if len(openCharges) == 1 && len(pendingDetails) > 0 && strings.TrimSpace(pendingDetails[0].ProjectName) != "" {
		highlights = append(highlights, fmt.Sprintf("El pendiente está focalizado en %s y no repartido en todo el cliente.", pendingDetails[0].ProjectName))
	}
	if len(charges) == len(docs) && len(chargesWithoutDoc) == 0 {
		highlights = append(highlights, "La cobertura cargo→documento es completa: todos los cargos tienen documento asociado.")
	}
	if len(activeCollectableProjects) == 1 {
		highlights = append(highlights, fmt.Sprintf("Solo hay un proyecto activo y cobrable: %s.", activeCollectableProjects[0]["name"]))
	}
	if scope.ActiveProjects.Charges > 0 {
		highlights = append(highlights, fmt.Sprintf("Los proyectos activos suman %d cargos; el universo total del cliente suma %d.", scope.ActiveProjects.Charges, scope.Client.Charges))
	}
	if scope.ActiveCollectableProjects.Charges > 0 {
		highlights = append(highlights, fmt.Sprintf("El subconjunto activo y cobrable concentra %d cargos, no %d.", scope.ActiveCollectableProjects.Charges, scope.Client.Charges))
	}
	if projectsWithoutResponsible > 0 {
		highlights = append(highlights, fmt.Sprintf("Hay %d proyectos sin responsable explícito, lo que es un riesgo operativo.", projectsWithoutResponsible))
	}
	if len(timelineAnomalies) > 0 {
		highlights = append(highlights, "Existe al menos una anomalía temporal relevante entre periodo descrito y fechas administrativas del cargo/documento.")
	}
	gaps := []string{}
	if projectsWithoutResponsible > 0 {
		gaps = append(gaps, fmt.Sprintf("%d proyectos sin responsable asignado.", projectsWithoutResponsible))
	}
	if projectsWithoutArea > 0 {
		gaps = append(gaps, fmt.Sprintf("%d proyectos sin área registrada.", projectsWithoutArea))
	}
	if agreementNameCoverage == 0 {
		gaps = append(gaps, "Los acuerdos no traen nombre útil: solo IDs.")
	}
	gaps = append(gaps,
		"No hay due_date explícito por cargo o documento.",
		"No hay monto por cargo o por documento en esta fuente.",
		"No existe vínculo directo pago→documento→cargo.",
		"No hay contacto comercial o dueño actual de cobranza del cliente.",
	)
	ctx := derivedContext{
		Source:                             defaultSource,
		Root:                               rootInfo{Entity: "clients", ID: client["id"], Name: client["name"], Code: client["code"]},
		Profile:                            profileInfo{Name: client["name"], Code: client["code"], Active: client["active"] == "1", AgreementStartDate: client["agreement_start_date"], Currency: firstNonEmptyProjectValue(projects, "currency_code"), Language: firstNonEmptyProjectValue(projects, "language_name")},
		Counts:                             countsInfo{Projects: len(projects), Agreements: len(agreements), Charges: len(charges), Documents: len(docs), Payments: len(payments), OpenCharges: len(openCharges), ActiveProjects: len(activeProjects), ActiveCollectableProjects: len(activeCollectableProjects)},
		Mere:                               []mereEdge{{From: "Cliente", To: "Proyectos", Count: len(projects)}, {From: "Cliente", To: "Acuerdos", Count: len(agreements)}, {From: "Cliente", To: "Cargos", Count: len(charges)}, {From: "Cargos", To: "Documentos", Count: len(docs)}, {From: "Cliente", To: "Pagos", Count: len(payments)}, {From: "Proyectos", To: "Tipos", Count: len(typeCounts)}, {From: "Proyectos", To: "Áreas", Count: len(areaCounts)}, {From: "Proyectos", To: "Responsables", Count: len(responsibleCounts)}},
		RelationMap:                        relationMap,
		Distributions:                      distributions{TypeCounts: typeCounts, AreaCounts: areaCounts, ResponsibleCounts: responsibleCounts, ChargeYearCounts: chargeYearCounts, PaymentYearCounts: paymentYearCounts, DocYearCounts: docYearCounts},
		PaymentStats:                       paymentStats{First: firstPaymentDate(payments), Last: lastPaymentDate(payments), Total: len(payments), TotalPaid: sumFloatRows(payments, "amount"), Avg: avgFloatRows(payments, "amount"), Max: maxFloatRows(payments, "amount")},
		Coverage:                           coverageInfo{ChargesWithDocuments: len(charges) - len(chargesWithoutDoc), ChargesWithoutDocuments: len(chargesWithoutDoc)},
		ScopeMetrics:                       scope,
		AtomicFacts:                        atomicFacts,
		PendingDetails:                     pendingDetails,
		ActiveProjectChargeRows:            activeProjectChargeRows,
		ActiveCollectableProjectChargeRows: activeCollectableProjectChargeRows,
		AgreementChargeRows:                agreementChargeRows,
		PendingAgreementRow:                pendingAgreementRow,
		ActiveProjectNames:                 activeProjectNames,
		TimelineAnomalies:                  timelineAnomalies,
		Highlights:                         highlights,
		Gaps:                               gaps,
	}
	return ctx, nil
}

func deterministicResponseForAction(label string, s *session) *aiResponse {
	actionLabel := normalizeLabel(label)
	ctx := s.Context
	pending := pendingDetail{}
	if len(ctx.PendingDetails) > 0 {
		pending = ctx.PendingDetails[0]
	}
	var pendingAgreement agreementChargeRow
	if ctx.PendingAgreementRow != nil {
		pendingAgreement = *ctx.PendingAgreementRow
	}
	activeCollectable := projectChargeRow{}
	if len(ctx.ActiveCollectableProjectChargeRows) > 0 {
		activeCollectable = ctx.ActiveCollectableProjectChargeRows[0]
	} else if pendingAgreement.ProjectName != "" {
		activeCollectable = projectChargeRow{Name: pendingAgreement.ProjectName, Code: pendingAgreement.ProjectCode, Charges: pendingAgreement.Charges, Pending: pendingAgreement.Pending, Paid: pendingAgreement.Paid, Collectable: pendingAgreement.ProjectCollectable}
	}

	switch actionLabel {
	case "analizar este caso en detalle":
		s.ScriptStep = 1
		return &aiResponse{
			Phase:          "synthesizing",
			AgendaProgress: makeAgendaProgress("focus", 1, []agendaCompletedItem{{QuestionID: "focus", ItemIndex: 0}}),
			Text:           fmt.Sprintf("Ya con esta fuente puedo darte una lectura útil para cobranza. <strong>%s</strong> tiene <strong>%d cargos</strong>, <strong>%d documentos</strong> y <strong>%d pagos</strong>; además, la cobertura cargo→documento es completa, así que no estoy viendo un caso desordenado o sin soporte.<br><br>Lo importante para cobrar no es el volumen histórico total, sino el foco actual: hay <strong>%d cargo facturado sin pago visible</strong> y está concentrado en <strong>%s</strong>. Mi lectura inicial es que conviene trabajar un <strong>caso puntual</strong>, no abrir un frente amplio sobre todo el cliente.", ctx.Profile.Name, ctx.Counts.Charges, ctx.Counts.Documents, ctx.Counts.Payments, ctx.Counts.OpenCharges, fallbackLabel(pending.ProjectName, "un proyecto específico")),
			Bars:           []bar{{Label: "Cobertura documental", Pct: pct(len(ctx.PendingDetails)+ctx.Coverage.ChargesWithDocuments-len(ctx.PendingDetails), maxInt(ctx.Counts.Charges, 1)), Val: fmt.Sprintf("%d/%d", ctx.Coverage.ChargesWithDocuments, ctx.Counts.Charges), Accent: true}},
			Metrics:        []metric{{Value: strconv.Itoa(ctx.Counts.OpenCharges), Label: "Pendientes visibles"}, {Value: strconv.Itoa(ctx.Counts.Charges), Label: "Cargos totales"}, {Value: strconv.Itoa(ctx.Counts.Documents), Label: "Documentos"}, {Value: strconv.Itoa(ctx.Counts.Payments), Label: "Pagos"}},
			Table:          &table{Header: "Lectura para cobranza", Rows: [][]string{{"Pendientes visibles", strconv.Itoa(ctx.Counts.OpenCharges), "Número de focos que hoy merecen seguimiento"}, {"Proyecto foco", fallbackLabel(pending.ProjectName, "Sin dato"), "Donde conviene concentrar la gestión"}, {"Cobertura documental", fmt.Sprintf("%d/%d", ctx.Coverage.ChargesWithDocuments, ctx.Counts.Charges), "Hay soporte documental para todo el histórico"}, {"Lectura ejecutiva", "Caso puntual", "No parece mora distribuida en todo el cliente"}, {"Riesgo principal", "Conciliación o cierre", "Más probable que un incumplimiento masivo"}, {"Qué no sé", "Monto exacto pendiente", "La fuente no lo trae de forma directa"}}},
			Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Revisar proyectos activos y cobrables", Primary: true}, {Label: "Pausar este caso"}},
		}
	case "revisar proyectos activos y cobrables":
		s.ScriptStep = 2
		return &aiResponse{
			Phase:          "synthesizing",
			AgendaProgress: makeAgendaProgress("focus", 2, []agendaCompletedItem{{QuestionID: "focus", ItemIndex: 0}, {QuestionID: "focus", ItemIndex: 1}}),
			Text:           fmt.Sprintf("Si separo el histórico total del frente realmente cobrable, el caso se aclara bastante. Hay <strong>%d proyectos activos</strong>, pero solo <strong>%d</strong> es a la vez <strong>activo y cobrable</strong>: <strong>%s</strong>.<br><br>Ese proyecto foco concentra <strong>%d cargos</strong>, no los <strong>%d</strong> del cliente completo. Ahí veo <strong>%d pagados</strong> y <strong>%d pendiente</strong>. Para cobranza, eso significa que el seguimiento de hoy está casi totalmente aislado en un solo nodo.", ctx.ScopeMetrics.ActiveProjects.Projects, ctx.ScopeMetrics.ActiveCollectableProjects.Projects, fallbackLabel(activeCollectable.Name, "sin nombre"), ctx.ScopeMetrics.ActiveCollectableProjects.Charges, ctx.Counts.Charges, ctx.ScopeMetrics.ActiveCollectableProjects.Paid, ctx.ScopeMetrics.ActiveCollectableProjects.Pending),
			Bars:           []bar{{Label: "Proyectos activos y cobrables", Pct: pct(ctx.ScopeMetrics.ActiveCollectableProjects.Projects, maxInt(ctx.Counts.Projects, 1)), Val: fmt.Sprintf("%d/%d", ctx.ScopeMetrics.ActiveCollectableProjects.Projects, ctx.Counts.Projects), Accent: true}},
			Metrics:        []metric{{Value: strconv.Itoa(ctx.ScopeMetrics.ActiveProjects.Projects), Label: "Proyectos activos"}, {Value: strconv.Itoa(ctx.ScopeMetrics.ActiveProjects.Charges), Label: "Cargos en activos"}, {Value: strconv.Itoa(ctx.ScopeMetrics.ActiveCollectableProjects.Charges), Label: "Cargos foco"}},
			Table:          &table{Header: "Foco de cobranza", Rows: [][]string{{"Proyecto foco", fallbackLabel(activeCollectable.Name, "Sin dato"), "Único proyecto activo y cobrable"}, {"Cargos del proyecto foco", strconv.Itoa(ctx.ScopeMetrics.ActiveCollectableProjects.Charges), "Universo correcto para decidir seguimiento"}, {"Pagados en el proyecto foco", strconv.Itoa(ctx.ScopeMetrics.ActiveCollectableProjects.Paid), "Historial interno del mismo nodo"}, {"Pendientes en el proyecto foco", strconv.Itoa(ctx.ScopeMetrics.ActiveCollectableProjects.Pending), "Caso actual a revisar"}, {"Cargos del cliente completo", strconv.Itoa(ctx.Counts.Charges), "No usar este total para describir el foco actual"}, {"Lectura ejecutiva", "Nodo aislado", "Conviene seguimiento puntual, no masivo"}}},
			Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Investigar brecha temporal en cargo pendiente", Primary: true}, {Label: "Pausar este caso"}},
		}
	case "investigar brecha temporal en cargo pendiente":
		s.ScriptStep = 3
		return &aiResponse{
			Phase:          "deciding",
			AgendaProgress: makeAgendaProgress("urgency", 0, []agendaCompletedItem{{QuestionID: "focus", ItemIndex: 0}, {QuestionID: "focus", ItemIndex: 1}, {QuestionID: "focus", ItemIndex: 2}}),
			Text:           fmt.Sprintf("El punto que más cambia la gestión es temporal. El único cargo pendiente es el <strong>%s</strong>, asociado a <strong>%s</strong>. La fecha administrativa del cargo es <strong>%s</strong>, pero el periodo descrito es <strong>%s</strong>.<br><br>Eso abre una brecha de <strong>%.1f años</strong>. Para cobranza, esta señal me hace ser prudente: antes de presionar como si fuera mora reciente, conviene validar si se trata de <strong>arrastre histórico, reproceso o falta de conciliación</strong>. La anomalía existe; lo que no puedo afirmar todavía es que el cliente haya dejado de pagar un cargo corriente.", fallbackLabel(pending.ChargeID, "sin ID"), fallbackLabel(pending.ProjectName, "sin dato"), fallbackLabel(pending.ChargeDate, "sin dato"), fallbackLabel(pending.Description, "sin dato"), pending.TimelineGapYears),
			Metrics:        []metric{{Value: fallbackLabel(pending.ChargeID, "—"), Label: "Cargo foco"}, {Value: fallbackLabel(pending.DocumentNumber, "—"), Label: "Documento foco"}, {Value: fmt.Sprintf("%.1f años", pending.TimelineGapYears), Label: "Brecha temporal"}},
			Table:          &table{Header: "Qué mirar antes de cobrar fuerte", Rows: [][]string{{"Cargo", fallbackLabel(pending.ChargeID, "Sin dato"), "Único pendiente visible"}, {"Documento", fallbackLabel(pending.DocumentNumber, "Sin dato"), "Referencia concreta para conciliación"}, {"Fecha cargo", fallbackLabel(pending.ChargeDate, "Sin dato"), "Fecha administrativa actual"}, {"Fecha documento", fallbackLabel(pending.DocumentDate, "Sin dato"), "Emisión del soporte"}, {"Período descrito", fallbackLabel(pending.Description, "Sin dato"), "Señal de histórico/backlog"}, {"Lectura prudente", "Validar conciliación primero", "Evita tratar como mora reciente algo que puede ser arrastre"}}},
			Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Actuar con la información disponible", Primary: true}, {Label: "Pausar este caso"}},
		}
	case "actuar con la informacion disponible":
		s.ScriptStep = 4
		return &aiResponse{
			Phase:          "acting",
			AgendaProgress: makeAgendaProgress("action", 1, []agendaCompletedItem{{QuestionID: "focus", ItemIndex: 0}, {QuestionID: "focus", ItemIndex: 1}, {QuestionID: "focus", ItemIndex: 2}, {QuestionID: "urgency", ItemIndex: 0}, {QuestionID: "action", ItemIndex: 0}}),
			Text:           fmt.Sprintf("Si yo tuviera que cobrar hoy con esta evidencia, <strong>no escalaría sobre todo el cliente</strong>. Iría directo a <strong>%s</strong> y usaría el caso como una revisión puntual de conciliación.<br><br>Mi siguiente paso sería validar internamente el <strong>cargo %s</strong> y el <strong>documento %s</strong>. Si ahí confirmo que sigue abierto de verdad, recién después armaría la gestión de cobro. Si no, probablemente estás frente a un pendiente administrativo más que a una deuda nueva para empujar con fuerza.<br><br>Si luego decides escribir, el correo debería incluir solo hechos verificables: <strong>proyecto</strong>, <strong>documento</strong>, <strong>cargo pendiente visible</strong> y una <strong>solicitud de confirmación/conciliación</strong>; no afirmaría todavía mora reciente ni monto exacto pendiente.", fallbackLabel(pending.ProjectName, "el proyecto focalizado"), fallbackLabel(pending.ChargeID, "sin dato"), fallbackLabel(pending.DocumentNumber, "sin dato")),
			Table:          &table{Header: "Plan de acción de cobranza", Rows: [][]string{{"No hacer", "Cobranza masiva al cliente", "La evidencia no soporta un problema distribuido"}, {"Sí hacer", fallbackLabel(pending.ProjectName, "Sin dato"), "Trabajar el único nodo donde hoy aparece el pendiente"}, {"Validar cargo", fallbackLabel(pending.ChargeID, "Sin dato"), "Confirmar si sigue abierto o fue reprocesado"}, {"Validar documento", fallbackLabel(pending.DocumentNumber, "Sin dato"), "Revisar conciliación o aplicación de pago"}, {"Correo si escalas", fmt.Sprintf("Proyecto %s · Documento %s · Cargo %s", fallbackLabel(pending.ProjectName, "sin dato"), fallbackLabel(pending.DocumentNumber, "sin dato"), fallbackLabel(pending.ChargeID, "sin dato")), "Incluir hechos verificables y pedido de confirmación"}, {"Límite real", pickGap(ctx.Gaps, 2), "No existe enlace directo pago→documento→cargo"}}},
			Actions:        []action{{Label: "Seguir analizando", Muted: true}, {Label: "Pausar este caso", Primary: true}, {Label: "Reiniciar este caso"}},
		}
	case "pausar este caso":
		return &aiResponse{Phase: "done", Text: "Caso pausado. El contexto relacional y los hallazgos quedan visibles para retomarlo después sin perder el foco del análisis.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Reiniciar este caso", Primary: true}, {Label: "Analizar este caso en detalle"}}}
	case "reiniciar este caso":
		s.ScriptStep = 0
		s.Agenda = nil
		s.ChatHistory = nil
		return &aiResponse{Phase: "exploring", Text: "Reinicié la secuencia analítica. Puedo volver a recorrer el caso desde el resumen, el foco por proyecto o la anomalía temporal.", Actions: []action{{Label: "Seguir analizando", Muted: true}, {Label: "Analizar este caso en detalle", Primary: true}, {Label: "Pausar este caso"}}}
	default:
		return nil
	}
}

func buildAnalysisAgenda(ctx derivedContext) []agendaGroup {
	pending := pendingDetail{}
	if len(ctx.PendingDetails) > 0 {
		pending = ctx.PendingDetails[0]
	}
	activeCollectable := projectChargeRow{}
	if len(ctx.ActiveCollectableProjectChargeRows) > 0 {
		activeCollectable = ctx.ActiveCollectableProjectChargeRows[0]
	}
	projectName := activeCollectable.Name
	if strings.TrimSpace(projectName) == "" {
		projectName = pending.ProjectName
	}
	if strings.TrimSpace(projectName) == "" {
		projectName = "el caso"
	}
	return []agendaGroup{
		{ID: "focus", Title: "1. ¿Dónde está el foco real de cobranza?", Why: "Primero debo aislar si hoy hay un solo frente de seguimiento o si el problema está distribuido.", Items: []agendaItem{{Text: "¿Qué pendiente requiere seguimiento hoy?"}, {Text: fmt.Sprintf("¿Qué proyecto concentra el foco actual%s?", suffixWithValue(pending.ProjectName))}, {Text: "¿Debo abrir un frente amplio o trabajar un caso puntual?"}}},
		{ID: "urgency", Title: "2. ¿Qué tan cobrable y urgente es el caso?", Why: "Antes de empujar una gestión dura, necesito distinguir mora real de descalce administrativo.", Items: []agendaItem{{Text: "¿El pendiente parece mora reciente o más bien un arrastre administrativo?"}, {Text: "¿Hay anomalías temporales o de conciliación que cambien la lectura?"}, {Text: "¿El historial del cliente sugiere incumplimiento general o excepción puntual?"}}},
		{ID: "action", Title: "3. ¿Cómo debería actuar cobranzas ahora?", Why: "La gestión solo sirve si termina en una acción concreta y proporcionada a la evidencia disponible.", Items: []agendaItem{{Text: fmt.Sprintf("¿Qué debo validar antes de contactar sobre %s?", projectName)}, {Text: "¿Qué información debería ir en el correo si decido escalar?"}, {Text: "¿Qué límites de la fuente debo explicitar para no sobreafirmar?"}}},
	}
}

func normalizeAgenda(raw []agendaGroup, ctx derivedContext) []agendaGroup {
	fallbackAgenda := buildAnalysisAgenda(ctx)
	if strings.TrimSpace(ctx.Profile.Name) == "" && strings.TrimSpace(ctx.Root.ID) == "" {
		fallbackAgenda = buildPortfolioAgenda()
	}
	if len(raw) == 0 {
		return nil
	}
	fallbackIDs := []string{"focus", "urgency", "action"}
	cleaned := []agendaGroup{}
	for idx, group := range raw {
		fallbackID := fmt.Sprintf("llm_%d", idx+1)
		if idx < len(fallbackIDs) {
			fallbackID = fallbackIDs[idx]
		}
		title := strings.TrimSpace(group.Title)
		if title == "" {
			title = fmt.Sprintf("Pregunta principal %d", idx+1)
		}
		items := []agendaItem{}
		for itemIdx, item := range group.Items {
			text := strings.TrimSpace(item.Text)
			if text == "" {
				continue
			}
			itemID := strings.TrimSpace(item.ID)
			if itemID == "" {
				itemID = fmt.Sprintf("%s_%d", fallbackID, itemIdx+1)
			}
			status := strings.TrimSpace(item.Status)
			if status == "" {
				status = "pending"
			}
			items = append(items, agendaItem{ID: itemID, Text: text, Status: status})
			if len(items) == 3 {
				break
			}
		}
		if title != "" && len(items) > 0 {
			id := strings.TrimSpace(group.ID)
			if id == "" {
				id = fallbackID
			}
			cleaned = append(cleaned, agendaGroup{ID: id, Title: title, Why: strings.TrimSpace(group.Why), Items: items})
		}
		if len(cleaned) == 3 {
			break
		}
	}
	if !(len(cleaned) == 3 && allAgendaItems(cleaned, 3)) {
		return normalizeAgenda(fallbackAgenda, ctx)
	}
	hasProgress := false
	for _, g := range cleaned {
		for _, item := range g.Items {
			if item.Status == "active" || item.Status == "done" {
				hasProgress = true
				break
			}
		}
	}
	if !hasProgress {
		for gi := range cleaned {
			for ii := range cleaned[gi].Items {
				cleaned[gi].Items[ii].Status = "pending"
			}
		}
		cleaned[0].Items[0].Status = "active"
	}
	return cleaned
}

func applyResponseMeta(s *session, resp *aiResponse) {
	if resp == nil {
		return
	}
	if len(resp.Agenda) > 0 {
		s.Agenda = normalizeAgenda(resp.Agenda, s.Context)
	}
	if len(s.Agenda) == 0 {
		fallbackAgenda := buildAnalysisAgenda(s.Context)
		if s.View == "portfolio" {
			fallbackAgenda = buildPortfolioAgenda()
		}
		s.Agenda = normalizeAgenda(fallbackAgenda, s.Context)
	}
	applyAgendaProgress(&s.Agenda, resp.AgendaProgress)
	if resp.Phase != "" {
		s.Phase = resp.Phase
	}
}

func applyAgendaProgress(agenda *[]agendaGroup, progress *agendaProgress) {
	if agenda == nil || len(*agenda) == 0 {
		return
	}
	for gi := range *agenda {
		for ii := range (*agenda)[gi].Items {
			if strings.TrimSpace((*agenda)[gi].Items[ii].Status) == "" {
				(*agenda)[gi].Items[ii].Status = "pending"
			}
		}
	}
	if progress != nil {
		for _, entry := range progress.Completed {
			for gi := range *agenda {
				if (*agenda)[gi].ID != entry.QuestionID {
					continue
				}
				if entry.ItemIndex >= 0 && entry.ItemIndex < len((*agenda)[gi].Items) {
					(*agenda)[gi].Items[entry.ItemIndex].Status = "done"
				}
			}
		}
		if strings.TrimSpace(progress.ActiveQuestionID) != "" && progress.ActiveItemIndex >= 0 {
			for gi := range *agenda {
				for ii := range (*agenda)[gi].Items {
					if (*agenda)[gi].Items[ii].Status == "active" {
						(*agenda)[gi].Items[ii].Status = "pending"
					}
				}
			}
			for gi := range *agenda {
				if (*agenda)[gi].ID != progress.ActiveQuestionID {
					continue
				}
				if progress.ActiveItemIndex < len((*agenda)[gi].Items) && (*agenda)[gi].Items[progress.ActiveItemIndex].Status != "done" {
					(*agenda)[gi].Items[progress.ActiveItemIndex].Status = "active"
				}
			}
		}
	}
	if !hasActiveAgendaItem(*agenda) {
		for gi := range *agenda {
			for ii := range (*agenda)[gi].Items {
				if (*agenda)[gi].Items[ii].Status != "done" {
					(*agenda)[gi].Items[ii].Status = "active"
					return
				}
			}
		}
	}
}

func postProcessResponse(s *session, resp *aiResponse, userText string) {
	if resp == nil {
		return
	}
	resp.Bars = resp.Bars
	if isSmallTalkPrompt(userText) || isMetaQuestionPrompt(userText) {
		resp.Metrics = nil
		resp.Table = nil
		resp.Bars = nil
		return
	}
	if resp.Phase != "done" {
		resp.Text = appendAgendaBridge(resp.Text, getActiveAgendaItem(s.Agenda))
	}
}

func agendaStateForPrompt(agenda []agendaGroup) string {
	if len(agenda) == 0 {
		return "No existe agenda todavía. En esta respuesta debes crear una con exactamente 3 preguntas principales y 3 subpreguntas por cada una."
	}
	type promptItem struct {
		Index  int    `json:"index"`
		Text   string `json:"text"`
		Status string `json:"status"`
	}
	type promptGroup struct {
		ID     string       `json:"id"`
		Title  string       `json:"title"`
		Why    string       `json:"why"`
		Status string       `json:"status"`
		Items  []promptItem `json:"items"`
	}
	groups := make([]promptGroup, 0, len(agenda))
	for _, group := range agenda {
		items := make([]promptItem, 0, len(group.Items))
		for idx, item := range group.Items {
			status := item.Status
			if status == "" {
				status = "pending"
			}
			items = append(items, promptItem{Index: idx, Text: item.Text, Status: status})
		}
		groups = append(groups, promptGroup{ID: group.ID, Title: group.Title, Why: group.Why, Status: agendaGroupState(group), Items: items})
	}
	return mustJSON(groups)
}

func currentAgendaFocusText(agenda []agendaGroup) string {
	for _, group := range agenda {
		activeIdx := activeSubIndexForGroup(group)
		if activeIdx >= 0 {
			item := group.Items[activeIdx]
			return fmt.Sprintf("La subpregunta activa de la agenda es %q dentro de %q. Continúa desde ahí y, si la respondes suficientemente, marca progreso hacia la siguiente subpregunta.", item.Text, group.Title)
		}
	}
	return ""
}

func appendAgendaBridge(text string, active *agendaFocus) string {
	if active == nil || strings.TrimSpace(active.Item.Text) == "" {
		return text
	}
	if strings.Contains(text, active.Item.Text) || strings.Contains(text, "agenda-bridge") {
		return text
	}
	return text + fmt.Sprintf(`<br><br><span class="agenda-bridge">Si quieres, sigo con <strong>%s</strong>.</span>`, active.Item.Text)
}

type agendaFocus struct {
	Group agendaGroup
	Item  agendaItem
	Index int
}

func getActiveAgendaItem(agenda []agendaGroup) *agendaFocus {
	for _, group := range agenda {
		idx := activeSubIndexForGroup(group)
		if idx >= 0 {
			return &agendaFocus{Group: group, Item: group.Items[idx], Index: idx}
		}
	}
	return nil
}

func agendaGroupState(group agendaGroup) string {
	if len(group.Items) == 0 {
		return "pending"
	}
	allDone := true
	for _, item := range group.Items {
		if item.Status == "active" {
			return "active"
		}
		if item.Status != "done" {
			allDone = false
		}
	}
	if allDone {
		return "done"
	}
	return "pending"
}

func activeSubIndexForGroup(group agendaGroup) int {
	for idx, item := range group.Items {
		if item.Status == "active" {
			return idx
		}
	}
	return -1
}

func isSmallTalkPrompt(text string) bool {
	normalized := cleanedPrompt(text)
	if normalized == "" {
		return false
	}
	exact := map[string]struct{}{"hola": {}, "buenas": {}, "buenos dias": {}, "buen dia": {}, "buenas tardes": {}, "buenas noches": {}, "como estas": {}, "como andas": {}, "que tal": {}, "y tu": {}, "y tu como estas": {}, "gracias": {}, "ok": {}, "vale": {}, "dale": {}, "perfecto": {}, "excelente": {}, "genial": {}}
	if _, ok := exact[normalized]; ok {
		return true
	}
	tokens := strings.Fields(normalized)
	if len(tokens) <= 4 {
		switch {
		case containsToken(tokens, "hola"), containsToken(tokens, "buenas"), containsToken(tokens, "gracias"), containsToken(tokens, "dale"), containsToken(tokens, "ok"), containsToken(tokens, "vale"):
			return true
		case containsLike(tokens, "como", 1) && containsLike(tokens, "estas", 2):
			return true
		case containsLike(tokens, "que", 1) && containsLike(tokens, "tal", 1):
			return true
		case containsLike(tokens, "y", 0) && containsLike(tokens, "tu", 1):
			return true
		}
	}
	return regexp.MustCompile(`^(hola|buenas|gracias|como estas|que tal|y tu)\b`).MatchString(normalized)
}

func cleanedPrompt(text string) string {
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(regexp.MustCompile(`[!?.,;:]`).ReplaceAllString(normalizeLabel(text), " "), " "))
}

func containsToken(tokens []string, target string) bool {
	for _, token := range tokens {
		if token == target {
			return true
		}
	}
	return false
}

func containsLike(tokens []string, target string, maxDistance int) bool {
	for _, token := range tokens {
		if levenshtein(token, target) <= maxDistance {
			return true
		}
	}
	return false
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			insertCost := current[j-1] + 1
			deleteCost := prev[j] + 1
			replaceCost := prev[j-1] + cost
			current[j] = minInt(insertCost, minInt(deleteCost, replaceCost))
		}
		prev = current
	}
	return prev[len(b)]
}

func normalizeActions(actions []action) []action {
	items := []action{}
	for _, a := range actions {
		if strings.TrimSpace(a.Label) != "" {
			items = append(items, a)
		}
	}
	hasSeguir := false
	for _, a := range items {
		if a.Label == "Seguir analizando" {
			hasSeguir = true
			break
		}
	}
	if !hasSeguir {
		items = append([]action{{Label: "Seguir analizando"}}, items...)
	}
	if len(items) > 3 {
		items = items[:3]
	}
	for len(items) < 3 {
		fallbacks := []action{{Label: "Actuar con la información disponible"}, {Label: "Pausar este caso", Muted: true}}
		added := false
		for _, fallback := range fallbacks {
			if !containsAction(items, fallback.Label) {
				items = append(items, fallback)
				added = true
				break
			}
		}
		if !added {
			break
		}
	}
	primaryIndex := -1
	for i, item := range items {
		if item.Primary {
			primaryIndex = i
			break
		}
	}
	if primaryIndex == -1 {
		for i, item := range items {
			if item.Label != "Seguir analizando" {
				primaryIndex = i
				break
			}
		}
		if primaryIndex == -1 {
			primaryIndex = 0
		}
	}
	for i := range items {
		items[i].Primary = i == primaryIndex
		if i != primaryIndex {
			items[i].Muted = items[i].Muted
		}
	}
	if !containsAction(items, "Seguir analizando") && len(items) > 0 {
		items[0] = action{Label: "Seguir analizando", Muted: true}
	}
	return items
}

func makeAgendaProgress(activeQuestionID string, activeItemIndex int, completed []agendaCompletedItem) *agendaProgress {
	return &agendaProgress{ActiveQuestionID: activeQuestionID, ActiveItemIndex: activeItemIndex, Completed: completed}
}

func (a *app) getSession(id string) (*session, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.sessions[strings.TrimSpace(id)]
	if s == nil {
		return nil, errors.New("sesión no encontrada")
	}
	return s, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func queryRows(db *sql.DB, query string, args ...any) ([]row, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	results := []row{}
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := row{}
		for i, col := range cols {
			item[col] = dbValueToString(values[i])
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func namedQueryRows(db *sql.DB, query string, columns []string, args ...any) ([]row, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []row{}
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := row{}
		for i, col := range columns {
			item[col] = dbValueToString(values[i])
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func remapColumns(item row, columns []string) row {
	mapped := row{}
	for _, col := range columns {
		mapped[col] = item[col]
	}
	return mapped
}

func dbValueToString(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(value)
	case string:
		return value
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return fmt.Sprint(value)
	}
}

func filterRows(items []row, keep func(row) bool) []row {
	out := []row{}
	for _, item := range items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

func chargesForProject(charges []row, project row) []row {
	return filterRows(charges, func(r row) bool { return r["agreement_id"] == project["agreement_id"] })
}

func safeJSONStringSlice(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err == nil {
		return out
	}
	return nil
}

func parseDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" || value == "0000-00-00" {
		return time.Time{}
	}
	layouts := []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func extractEmbeddedDates(text string) []time.Time {
	matches := regexp.MustCompile(`(\d{2})-(\d{2})-(\d{4})`).FindAllStringSubmatch(text, -1)
	out := []time.Time{}
	for _, m := range matches {
		if len(m) != 4 {
			continue
		}
		if t, err := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", m[3], m[2], m[1])); err == nil {
			out = append(out, t)
		}
	}
	return out
}

func num(value string) float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return n
}

func sumFloatRows(items []row, key string) float64 {
	total := 0.0
	for _, item := range items {
		total += num(item[key])
	}
	return total
}

func avgFloatRows(items []row, key string) float64 {
	if len(items) == 0 {
		return 0
	}
	return sumFloatRows(items, key) / float64(len(items))
}

func maxFloatRows(items []row, key string) float64 {
	maxValue := 0.0
	for _, item := range items {
		maxValue = math.Max(maxValue, num(item[key]))
	}
	return maxValue
}

func firstPaymentDate(payments []row) string {
	if len(payments) == 0 {
		return ""
	}
	return payments[0]["date"]
}

func lastPaymentDate(payments []row) string {
	if len(payments) == 0 {
		return ""
	}
	return payments[len(payments)-1]["date"]
}

func firstNonEmptyProjectValue(projects []row, key string) string {
	for _, p := range projects {
		if strings.TrimSpace(p[key]) != "" {
			return p[key]
		}
	}
	return ""
}

func yearPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 4 {
		return value[:4]
	}
	return ""
}

func fallbackLabel(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func suffixWithValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf(" (%s)", value)
}

func pickGap(gaps []string, idx int) string {
	if idx >= 0 && idx < len(gaps) {
		return gaps[idx]
	}
	if len(gaps) > 0 {
		return gaps[len(gaps)-1]
	}
	return "Sin dato"
}

func containsAction(actions []action, label string) bool {
	for _, a := range actions {
		if a.Label == label {
			return true
		}
	}
	return false
}

func pct(part, total int) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(part) / float64(total) * 100))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sumProjectCharges(items []projectChargeRow) int {
	total := 0
	for _, item := range items {
		total += item.Charges
	}
	return total
}

func sumProjectPending(items []projectChargeRow) int {
	total := 0
	for _, item := range items {
		total += item.Pending
	}
	return total
}

func sumProjectPaid(items []projectChargeRow) int {
	total := 0
	for _, item := range items {
		total += item.Paid
	}
	return total
}

func allAgendaItems(groups []agendaGroup, expected int) bool {
	for _, group := range groups {
		if len(group.Items) != expected {
			return false
		}
	}
	return true
}

func hasActiveAgendaItem(groups []agendaGroup) bool {
	for _, group := range groups {
		for _, item := range group.Items {
			if item.Status == "active" {
				return true
			}
		}
	}
	return false
}

func mustJSON(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func normalizeLabel(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ä", "a", "â", "a",
		"é", "e", "è", "e", "ë", "e", "ê", "e",
		"í", "i", "ì", "i", "ï", "i", "î", "i",
		"ó", "o", "ò", "o", "ö", "o", "ô", "o",
		"ú", "u", "ù", "u", "ü", "u", "û", "u",
		"ñ", "n",
	)
	return strings.TrimSpace(replacer.Replace(strings.ToLower(s)))
}

func groupCount[T any](items []T, keyFn func(T) string, labelFn func(string, T) string) []countItem {
	type bucket struct {
		Label string
		Count int
	}
	buckets := map[string]bucket{}
	for _, item := range items {
		raw := keyFn(item)
		key := raw
		if strings.TrimSpace(key) == "" {
			key = "__empty__"
		}
		label := raw
		if labelFn != nil {
			label = labelFn(raw, item)
		} else if strings.TrimSpace(label) == "" {
			label = "Sin dato"
		}
		current := buckets[key]
		current.Label = label
		current.Count++
		buckets[key] = current
	}
	out := make([]countItem, 0, len(buckets))
	for _, item := range buckets {
		out = append(out, countItem{Label: item.Label, Count: item.Count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	return out
}

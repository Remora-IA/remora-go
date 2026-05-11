package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const stateFile = "temp/critico_state.json"

type State struct {
	SessionID   string     `json:"session_id"`
	Initialized bool       `json:"initialized"`
	Evaluations []Eval     `json:"evaluations"`
	Questions   []Question `json:"questions"`
	LastUpdate  string     `json:"last_update"`
}

type Eval struct {
	ID       string   `json:"id"`
	Proposal string   `json:"proposal"`
	Verdict  string   `json:"verdict"` // approved, rejected, needs_evidence, conditional
	Risks    []Risk   `json:"risks"`
	Notes    string   `json:"notes"`
}

type Risk struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`    // low, medium, high, blocker
	Category    string `json:"category"`    // coupling, assumption, regression, data_loss, performance
	Description string `json:"description"`
	Evidence    string `json:"evidence"`
}

type Question struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	TargetID string `json:"target_id,omitempty"`
	Answered bool   `json:"answered"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`Framework Critico - Evaluacion adversarial de propuestas

Comandos:
  init --session-id <id>
  evaluate --proposal "..." --context <path> --severity <normal|strict>
  challenge --assumption "..." --evidence "..."
  status
  readiness
  next-question
  ingest-answer --question-id <id> --answer <text>`)
		os.Exit(0)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		handleInit()
	case "evaluate":
		handleEvaluate()
	case "challenge":
		handleChallenge()
	case "status":
		handleStatus()
	case "readiness":
		handleReadiness()
	case "next-question":
		handleNextQuestion()
	case "ingest-answer":
		handleIngestAnswer()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", cmd)
		os.Exit(1)
	}
}

func loadState() *State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return &State{Evaluations: []Eval{}, Questions: []Question{}}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{Evaluations: []Eval{}, Questions: []Question{}}
	}
	return &s
}

func saveState(s *State) {
	os.MkdirAll(filepath.Dir(stateFile), 0755)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(stateFile, data, 0644)
}

func handleInit() {
	sessionID := flagValue("--session-id")
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "usage: init --session-id <id>")
		os.Exit(1)
	}
	s := &State{
		SessionID:   sessionID,
		Initialized: true,
		Evaluations: []Eval{},
		Questions:   []Question{},
		LastUpdate:  time.Now().Format(time.RFC3339),
	}
	saveState(s)
	fmt.Printf(`{"initialized":true,"session_id":"%s"}`+"\n", sessionID)
}

func handleEvaluate() {
	s := loadState()
	if !s.Initialized {
		fmt.Fprintln(os.Stderr, "error: no inicializado")
		os.Exit(1)
	}
	proposal := flagValue("--proposal")
	contextPath := flagValue("--context")
	severity := flagValueDefault("--severity", "normal")

	if proposal == "" {
		fmt.Fprintln(os.Stderr, "usage: evaluate --proposal '...' [--context path] [--severity normal|strict]")
		os.Exit(1)
	}

	// Leer contexto (repo_model.json) si existe
	contextText := ""
	if contextPath != "" {
		if data, err := os.ReadFile(contextPath); err == nil {
			contextText = string(data)
		}
	}

	eval, err := evaluateWithLLM(proposal, contextText, severity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: LLM no disponible: %v\n", err)
		os.Exit(1)
	}

	// Generar preguntas de follow-up para riesgos altos
	for i, r := range eval.Risks {
		if r.Severity == "high" || r.Severity == "blocker" {
			qid := fmt.Sprintf("q_ev_%s_r%d", eval.ID, i)
			qtext := fmt.Sprintf("Riesgo detectado: %s. ¿Tienes evidencia de que esto no ocurrira? (captura de tests, logs, o conteo de referencias)", r.Description)
			s.Questions = append(s.Questions, Question{ID: qid, Text: qtext, TargetID: r.ID})
		}
	}

	s.Evaluations = append(s.Evaluations, eval)
	s.LastUpdate = time.Now().Format(time.RFC3339)
	saveState(s)

	data, _ := json.MarshalIndent(eval, "", "  ")
	fmt.Println(string(data))
}

func handleChallenge() {
	s := loadState()
	assumption := flagValue("--assumption")
	evidence := flagValue("--evidence")
	if assumption == "" {
		fmt.Fprintln(os.Stderr, "usage: challenge --assumption '...' --evidence '...'")
		os.Exit(1)
	}

	// Stub: evaluar si la evidencia refuta la asuncion
	refuted := strings.Contains(strings.ToLower(evidence), "no") ||
		strings.Contains(strings.ToLower(evidence), "error") ||
		strings.Contains(strings.ToLower(evidence), "fallo")

	result := map[string]interface{}{
		"assumption": assumption,
		"refuted":    refuted,
		"reason":     "Evidencia insuficiente para confirmar la asuncion.",
	}
	if refuted {
		result["reason"] = "La evidencia contradice la asuncion."
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))

	// Guardar en estado
	s.Evaluations = append(s.Evaluations, Eval{
		ID:       fmt.Sprintf("ch_%d", len(s.Evaluations)+1),
		Proposal: assumption,
		Verdict:  map[bool]string{true: "rejected", false: "needs_evidence"}[refuted],
		Notes:    result["reason"].(string),
	})
	s.LastUpdate = time.Now().Format(time.RFC3339)
	saveState(s)
}

func handleStatus() {
	s := loadState()
	data, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(data))
}

func handleReadiness() {
	s := loadState()
	action := "ask_next_missing_fact"
	if !s.Initialized {
		action = "needs_init"
	} else if len(s.Evaluations) == 0 {
		action = "needs_proposal"
	} else {
		last := s.Evaluations[len(s.Evaluations)-1]
		if last.Verdict == "needs_evidence" {
			action = "ask_next_missing_fact"
		} else if last.Verdict == "approved" {
			action = "ready_for_implementacion"
		} else {
			action = "ready_for_debate"
		}
	}
	fmt.Printf(`{"ready":%t,"recommended_action":"%s","evaluations":%d}`+"\n",
		action == "ready_for_implementacion", action, len(s.Evaluations))
}

func handleNextQuestion() {
	s := loadState()
	if !s.Initialized {
		fmt.Println(`{}`)
		return
	}
	if len(s.Evaluations) == 0 {
		fmt.Println(`{}`)
		return
	}
	// Buscar primera pregunta no respondida
	for _, q := range s.Questions {
		if !q.Answered {
			fmt.Printf(`{"id":"%s","text":"%s","ask_via":""}`+"\n", q.ID, q.Text)
			return
		}
	}
	fmt.Println(`{}`)
}

func handleIngestAnswer() {
	s := loadState()
	qid := flagValue("--question-id")
	answer := flagValue("--answer")
	if answer == "" {
		fmt.Fprintln(os.Stderr, "usage: ingest-answer --question-id <id> --answer <text>")
		os.Exit(1)
	}

	// Marcar pregunta como respondida
	for i := range s.Questions {
		if s.Questions[i].ID == qid {
			s.Questions[i].Answered = true
		}
	}

	// Si la respuesta contiene evidencia fuerte, actualizar evaluacion
	strong := strings.Contains(strings.ToLower(answer), "si") ||
		strings.Contains(strings.ToLower(answer), "confirmo") ||
		strings.Contains(strings.ToLower(answer), "test")
	if strong && len(s.Evaluations) > 0 {
		last := &s.Evaluations[len(s.Evaluations)-1]
		if last.Verdict == "needs_evidence" {
			last.Verdict = "conditional"
			last.Notes = "Evidencia parcial recibida. Quedan riesgos sin descartar."
		}
	}

	s.LastUpdate = time.Now().Format(time.RFC3339)
	saveState(s)
	fmt.Println(`{"ingested":true}`)
}

func flagValue(name string) string {
	for i, a := range os.Args {
		if a == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func flagValueDefault(name, def string) string {
	v := flagValue(name)
	if v == "" {
		return def
	}
	return v
}

// ---------------------------------------------------------------------------
// LLM evaluation (OpenRouter / Groq / MiniMax)
// ---------------------------------------------------------------------------

func resolveCriticoProvider() (provider, apiKey, apiURL, model string) {
	loadEnvFiles()
	p := strings.ToLower(strings.TrimSpace(os.Getenv("CRITICO_LLM_PROVIDER")))
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
		return p, firstEnv("OPENROUTER_API_KEY"), "https://openrouter.ai/api/v1/chat/completions", firstNonEmpty(os.Getenv("CRITICO_LLM_MODEL"), "meta-llama/llama-4-scout-17b-16e-instruct")
	case "groq":
		return p, firstEnv("GROQ_API_KEY", "REMORA_GROQ_API_KEY"), "https://api.groq.com/openai/v1/chat/completions", firstNonEmpty(os.Getenv("CRITICO_LLM_MODEL"), "llama-3.3-70b-versatile")
	default:
		return "minimax", firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY"), "https://api.minimax.io/anthropic/v1/messages", firstNonEmpty(os.Getenv("CRITICO_LLM_MODEL"), "MiniMax-M2.7")
	}
}

func evaluateWithLLM(proposal, contextText, severity string) (Eval, error) {
	provider, apiKey, apiURL, model := resolveCriticoProvider()
	if apiKey == "" {
		return Eval{}, fmt.Errorf("falta API key para provider %s", provider)
	}

	system := buildCriticoSystemPrompt(severity)
	userMsg := buildCriticoUserPrompt(proposal, contextText)

	var rawText string
	var err error
	if provider == "minimax" {
		rawText, err = callMiniMax(apiKey, apiURL, model, system, userMsg)
	} else {
		rawText, err = callOAICompat(apiKey, apiURL, model, system, userMsg)
	}
	if err != nil {
		return Eval{}, err
	}

	fullText := rawText

	// Extraer JSON de la respuesta (puede estar en markdown code block o directo)
	jsonText := extractJSON(fullText)
	if jsonText == "" {
		return Eval{}, fmt.Errorf("no se encontró JSON válido en la respuesta")
	}

	var eval Eval
	if err := json.Unmarshal([]byte(jsonText), &eval); err != nil {
		return Eval{}, fmt.Errorf("json parse error: %w (raw: %s)", err, jsonText)
	}

	// Validar campos requeridos
	if eval.Verdict == "" {
		eval.Verdict = "needs_evidence"
	}
	if eval.ID == "" {
		eval.ID = fmt.Sprintf("ev_mm_%d", time.Now().Unix())
	}
	if eval.Proposal == "" {
		eval.Proposal = proposal
	}
	if eval.Notes == "" {
		eval.Notes = fmt.Sprintf("Evaluación generada por LLM (%s). Revisar riesgos detectados.", provider)
	}

	return eval, nil
}

func buildCriticoSystemPrompt(severity string) string {
	strictness := "normal"
	if severity == "strict" {
		strictness = "muy estricta: considera blockers cualquier asuncion no verificada y exige tests para todo cambio"
	}
	return fmt.Sprintf(`Sos Critico, un evaluador adversarial de propuestas de cambio en codebases Go.

Tu trabajo es analizar una propuesta, detectar riesgos concretos, y devolver un JSON estricto con esta estructura:

{
  "verdict": "approved" | "rejected" | "needs_evidence" | "conditional",
  "risks": [
    {
      "id": "r_xxx",
      "severity": "low" | "medium" | "high" | "blocker",
      "category": "coupling" | "assumption" | "regression" | "data_loss" | "performance",
      "description": "descripcion concreta del riesgo",
      "evidence": "que falta verificar"
    }
  ],
  "notes": "resumen de la evaluacion"
}

Reglas:
- NUNCA inventes riesgos sin base en la propuesta o el contexto del repo.
- Si no hay suficiente contexto, verdict="needs_evidence" y pedi mas info en notes.
- Evaluacion %s.
- Responde SOLO con el JSON, sin markdown ni explicaciones previas.`, strictness)
}

func buildCriticoUserPrompt(proposal, contextText string) string {
	var sb strings.Builder
	sb.WriteString("PROPUESTA:\n")
	sb.WriteString(proposal)
	sb.WriteString("\n\n")
	if contextText != "" {
		sb.WriteString("CONTEXTO DEL REPO (repo_model.json):\n")
		// Truncar si es muy largo
		if len(contextText) > 8000 {
			sb.WriteString(contextText[:8000])
			sb.WriteString("\n...(truncado)...\n")
		} else {
			sb.WriteString(contextText)
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString("Evalua la propuesta y devolve SOLO el JSON.")
	return sb.String()
}

func extractJSON(text string) string {
	// Buscar JSON en code block markdown
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	// Buscar objeto JSON directamente
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		return text[start : end+1]
	}
	return text
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func loadEnvFiles() {
	roots := []string{}
	if v := os.Getenv("REMORA_ROOT"); v != "" {
		roots = append(roots, v)
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
		if i := strings.LastIndex(cwd, "/"); i > 0 {
			roots = append(roots, cwd[:i])
		}
	}
	for _, r := range roots {
		for _, suffix := range []string{"/.env.local", "/.env", "/remora-flujo/.env.local"} {
			readEnvFile(r + suffix)
		}
	}
}

func readEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 1 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func callOAICompat(apiKey, apiURL, model, system, userMsg string) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := []msg{}
	if system != "" {
		msgs = append(msgs, msg{Role: "system", Content: system})
	}
	msgs = append(msgs, msg{Role: "user", Content: userMsg})
	body, _ := json.Marshal(map[string]interface{}{
		"model":       model,
		"messages":    msgs,
		"max_tokens":  4096,
		"temperature": 0.2,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(httpReq)
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
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llm parse: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm: respuesta sin choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func callMiniMax(apiKey, apiURL, model, system, userMsg string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 4096,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": userMsg}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("minimax HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var ar struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return "", fmt.Errorf("minimax parse: %w", err)
	}
	var sb strings.Builder
	for _, c := range ar.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}

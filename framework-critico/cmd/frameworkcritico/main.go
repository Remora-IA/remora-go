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

	// Llamar a MiniMax para evaluación real
	eval, err := evaluateWithMiniMax(proposal, contextText, severity)
	if err != nil {
		// Fallback: usar evaluación determinística básica si MiniMax falla
		fmt.Fprintf(os.Stderr, "warn: MiniMax no disponible (%v), usando evaluacion basica\n", err)
		eval = basicEvaluate(proposal, severity)
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

// basicEvaluate genera una evaluación determinística basada en keywords
// (fallback cuando MiniMax no está disponible).
func basicEvaluate(proposal, severity string) Eval {
	risks := []Risk{}
	p := strings.ToLower(proposal)

	if strings.Contains(p, "mover") || strings.Contains(p, "refactor") {
		risks = append(risks, Risk{
			ID:          "r_coupling",
			Severity:    pickSeverity("high", severity),
			Category:    "coupling",
			Description: "La propuesta asume que las referencias al codigo movido son conocidas. Sin un index completo, puede haber callers no detectados.",
			Evidence:    "Pendiente: contar referencias con grep o index.",
		})
	}
	if strings.Contains(p, "cambiar") || strings.Contains(p, "modificar") {
		risks = append(risks, Risk{
			ID:          "r_regression",
			Severity:    pickSeverity("medium", severity),
			Category:    "regression",
			Description: "Cambios en funciones usadas por multiples frameworks pueden romper contratos implicitos.",
			Evidence:    "Pendiente: lista de callers y tests existentes.",
		})
	}
	if strings.Contains(p, "nuevo") || strings.Contains(p, "agregar") {
		risks = append(risks, Risk{
			ID:          "r_assumption",
			Severity:    pickSeverity("medium", severity),
			Category:    "assumption",
			Description: "Nuevas abstracciones asumen que el problema es general cuando puede ser especifico.",
			Evidence:    "Pendiente: confirmar que al menos 2 casos de uso justifican la generalizacion.",
		})
	}
	if strings.Contains(p, "eliminar") || strings.Contains(p, "borrar") {
		risks = append(risks, Risk{
			ID:          "r_data_loss",
			Severity:    pickSeverity("blocker", severity),
			Category:    "data_loss",
			Description: "Eliminar codigo sin migrar datos o referencias puede causar fallos en produccion.",
			Evidence:    "Pendiente: confirmar que ningun otro framework o proceso depende de este codigo.",
		})
	}
	if len(risks) == 0 {
		risks = append(risks, Risk{
			ID:          "r_generic",
			Severity:    pickSeverity("low", severity),
			Category:    "assumption",
			Description: "La propuesta no describe los tests que validarian el cambio.",
			Evidence:    "Pendiente: lista de tests a agregar o modificar.",
		})
	}

	return Eval{
		ID:      "ev_basic",
		Verdict: "needs_evidence",
		Risks:   risks,
		Notes:   "Evaluacion basica (fallback). Se requiere evidencia para confirmar o descartar riesgos.",
	}
}

func pickSeverity(base, severity string) string {
	if severity == "strict" {
		switch base {
		case "low":
			return "medium"
		case "medium":
			return "high"
		case "high":
			return "blocker"
		}
	}
	return base
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
// MiniMax Anthropic-compatible evaluation
// ---------------------------------------------------------------------------

func evaluateWithMiniMax(proposal, contextText, severity string) (Eval, error) {
	apiKey := firstEnv("MINIMAX_API_KEY", "REMORA_MINIMAX_API_KEY")
	if apiKey == "" {
		return Eval{}, fmt.Errorf("falta MINIMAX_API_KEY o REMORA_MINIMAX_API_KEY")
	}
	model := firstEnv("CRITICO_LLM_MODEL", "MiniMax-M2.7")

	system := buildCriticoSystemPrompt(severity)
	userMsg := buildCriticoUserPrompt(proposal, contextText)

	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 4096,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": userMsg},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Eval{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.minimax.io/anthropic/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Eval{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Eval{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Eval{}, err
	}
	if resp.StatusCode >= 400 {
		return Eval{}, fmt.Errorf("minimax HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var ar struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return Eval{}, fmt.Errorf("minimax parse: %w", err)
	}

	var fullText string
	for _, c := range ar.Content {
		if c.Type == "text" {
			fullText += c.Text
		}
	}

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
		eval.Notes = "Evaluación generada por MiniMax. Revisar riesgos detectados."
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

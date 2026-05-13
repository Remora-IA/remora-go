package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"channel/manifest"
	"remora-flujo/handoff"
	"remora-flujo/internal/llm"
)

func (s *server) runFlowActionBranches(ctx context.Context, req flowRunRequest, artifacts map[string]flowRunArtifact) []flowBranchRun {
	options := flowActionOptionsFromArtifacts(artifacts)
	if len(options) == 0 {
		return nil
	}
	limit := flowBranchLimit(req)
	if len(options) < limit {
		limit = len(options)
	}
	branches := make([]flowBranchRun, limit)
	var wg sync.WaitGroup
	for i, option := range options {
		if i >= limit {
			break
		}
		i, option := i, option
		wg.Add(1)
		go func() {
			defer wg.Done()
			branchReq := req
			branchReq.DryRun = true
			branchReq.Approved = false
			branchReq.TestMode = false
			branchReq.BranchMode = true
			branchReq.InitialArtifacts = cloneInitialArtifacts(req.InitialArtifacts)
			branchReq.InitialArtifacts["action.selection.v1"] = map[string]interface{}{
				"artifact_type": "action.selection.v1",
				"id":            option["id"],
				"label":         option["label"],
				"description":   option["description"],
				"bound_id":      option["bound_id"],
				"source":        "dimension_branch",
				"selected_at":   time.Now().UTC().Format(time.RFC3339Nano),
			}
			for typ, artifact := range artifacts {
				if _, exists := branchReq.InitialArtifacts[typ]; exists || artifact.Payload == nil {
					continue
				}
				branchReq.InitialArtifacts[typ] = artifact.Payload
			}
			branchReq.Flow.ID = req.Flow.ID + "_dim_" + safeFilePart(optionIDForBranch(option, i))
			branchResult := s.runFlowManifest(ctx, branchReq, nil)
			if isDeepAnalysisOption(option) {
				s.simulateDeepAnalysisConversation(ctx, branchReq, &branchResult)
			}
			branches[i] = flowBranchRun{
				BranchID:       branchResult.RunID,
				Action:         option,
				Status:         branchResult.Status,
				Timeline:       branchResult.Timeline,
				Artifacts:      sortedKeys(flowArtifactTypes(branchResult.Artifacts)),
				NeedsInput:     branchResult.NeedsInput,
				CycleCompleted: branchResult.Artifacts["flow.cycle.completed.v1"].Type == "flow.cycle.completed.v1",
			}
		}()
	}
	wg.Wait()
	return branches
}

func isDeepAnalysisOption(option map[string]string) bool {
	id := strings.TrimSpace(option["id"])
	return strings.EqualFold(id, "deep_analysis")
}

func (s *server) simulateDeepAnalysisConversation(ctx context.Context, req flowRunRequest, result *flowRunResult) {
	if result == nil || result.Status != "completed" {
		return
	}
	sessionPayload := segmentSessionPayload(result.Artifacts)
	if len(sessionPayload) == 0 || jsonFirstString(sessionPayload, "status") != "active" {
		return
	}
	session := sessionInfoFromPayload(result.Artifacts[segmentSessionArtifact].Path, sessionPayload)
	if session == nil {
		return
	}
	conv := deepAnalysisSimulatedConversation(req, result, s.allManifests)
	if conv.BusinessID == "" {
		conv.BusinessID = result.BusinessID
	}
	if session.ConversationID == "" && session.Path != "" {
		s.claimSessionForConversation(session.Path, conv.ID)
		session.ConversationID = conv.ID
	}
	queue := handoff.NewQuestionsQueue(conv.Frameworks...)

	for _, input := range simulatedDeepAnalysisContinuePrompts(deepAnalysisStressTurnLimit()) {
		started := time.Now().UTC().Format(time.RFC3339Nano)
		result.Timeline = append(result.Timeline, flowRunStep{
			Node:         "deep_analysis_simulated_user",
			Framework:    "simulacion",
			Capability:   "analysis.followup.user",
			Role:         "human",
			Status:       "completed",
			HumanSummary: "Usuario simulado: " + input,
			StartedAt:    started,
			FinishedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			SegmentID:    session.SegmentID,
			SegmentMode:  segmentModeAnalytical,
			SegmentOwner: session.Framework,
			SegmentRole:  "user",
		})
		intent := classifySegmentIntent(input, session)
		if intent != segmentIntentContinue {
			continue
		}
		execution, err := s.executeSessionFollowupDetailed(ctx, s.channel, conv, s.allManifests, queue, session, input)
		if err != nil || !execution.OK {
			result.Timeline = append(result.Timeline, failedSimulatedConversationStep(session, input, err))
			continue
		}
		if len(execution.DelegationRequests) > 0 {
			result.Timeline = append(result.Timeline, simulatedDelegationPlanStep(session, input, execution.DelegationRequests))
		}
		for _, entry := range delegationResultEntries(execution.DelegationResults) {
			result.Timeline = append(result.Timeline, simulatedDelegationResultStep(session, entry))
		}
		turn := segmentSessionTurnCountFromPath(session.Path)
		if turn > 0 {
			session.TurnCount = turn
		}
		followupStatus := "completed"
		if execution.SynthesisAttempted && !execution.Synthesized {
			followupStatus = "failed"
		}
		result.Timeline = append(result.Timeline, flowRunStep{
			Node:          "radar_deep_analysis_followup",
			Framework:     session.Framework,
			Capability:    session.Capability,
			Command:       session.FollowupCmd,
			Role:          "owner",
			Status:        followupStatus,
			AnalysisPhase: execution.AnalysisPhase,
			Synthesized:   execution.Synthesized,
			Error:         execution.SynthesisError,
			HumanSummary:  execution.Question.Text,
			ArtifactTypes: []string{"analysis.followup.v1", "answer.grounded.v1"},
			StartedAt:     started,
			FinishedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			SegmentID:     session.SegmentID,
			SegmentMode:   segmentModeAnalytical,
			SegmentOwner:  session.Framework,
			SegmentRole:   "owner",
		})
	}

	operationalInput := "Con eso basta, avanza con la recomendación."
	started := time.Now().UTC().Format(time.RFC3339Nano)
	result.Timeline = append(result.Timeline, flowRunStep{
		Node:         "deep_analysis_simulated_user",
		Framework:    "simulacion",
		Capability:   "analysis.followup.user",
		Role:         "human",
		Status:       "completed",
		HumanSummary: "Usuario simulado: " + operationalInput,
		StartedAt:    started,
		FinishedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		SegmentID:    session.SegmentID,
		SegmentMode:  segmentModeAnalytical,
		SegmentOwner: session.Framework,
		SegmentRole:  "user",
	})
	if classifySegmentIntent(operationalInput, session) == segmentIntentOperational {
		s.concludeSessionOnDisk(session.Path, "simulated_operational: "+operationalInput)
		s.persistAnalysisHandoff(ctx, s.channel, conv, s.allManifests, session, operationalInput)
		attachLatestArtifact(result, s.latestFlowArtifactPath(conv.BusinessID, "analysis.handoff.v1"), "analysis.handoff.v1")
		result.Timeline = append(result.Timeline, flowRunStep{
			Node:          "analysis_handoff_to_foco",
			Framework:     "flow_engine",
			Capability:    "analysis.handoff",
			Role:          "handoff",
			Status:        "completed",
			HumanSummary:  "Radar cerró el tramo analítico y entregó analysis.handoff.v1 para que Foco retome la operación.",
			ArtifactTypes: []string{"analysis.handoff.v1"},
			StartedAt:     started,
			FinishedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			SegmentID:     session.SegmentID,
			SegmentMode:   segmentModeOperational,
			SegmentOwner:  "foco",
			SegmentRole:   "owner",
		})
	}
}

func deepAnalysisSimulatedConversation(req flowRunRequest, result *flowRunResult, manifests map[string]*manifest.Manifest) *Conversation {
	conv := &Conversation{
		ID:         result.RunID + "_simulated_conversation",
		Title:      "Prueba simulada deep_analysis",
		Frameworks: activeFrameworkNames(req.Flow, manifests),
		BusinessID: req.Flow.BusinessID,
	}
	if result == nil {
		return conv
	}
	if entityArt, ok := result.Artifacts["entity.ref.v1"]; ok {
		if payload, ok := entityArt.Payload.(map[string]interface{}); ok {
			conv.RuntimeContext = map[string]any{
				"active_entity": map[string]any{
					"id":   jsonFirstString(payload, "id", "entity_ref", "ref"),
					"type": canonicalDelegationEntityType(jsonFirstString(payload, "type", "entity_type")),
					"name": jsonFirstString(payload, "name", "display_name"),
				},
			}
		}
	}
	if conv.BusinessID == "" {
		conv.BusinessID = result.BusinessID
	}
	return conv
}

func simulatedDeepAnalysisContinuePrompts(limit int) []string {
	prompts := []string{
		"Ok, explícame mejor por qué este caso quedó primero y qué evidencia pesa más.",
		"Compáralo contra clientes similares de la cartera: mora, saldo y comportamiento relativo.",
		"¿Qué contradicciones, riesgos residuales o gaps ves en la data antes de actuar?",
		"Profundiza la mora y la antigüedad: qué pesa más y qué evidencia falta.",
		"Haz un análisis de sensibilidad del score: qué variable cambiaría más la prioridad.",
		"Plantea hipótesis alternativas que expliquen este caso sin asumir que la deuda es totalmente cobrable.",
		"Evalúa un escenario contrafactual: si el saldo fuese menor o la mora más reciente, ¿seguiría primero?",
		"Con toda la evidencia, dame recomendación final, confianza y límites antes de operar.",
	}
	if limit <= 0 || limit > len(prompts) {
		limit = len(prompts)
	}
	return prompts[:limit]
}

func deepAnalysisStressTurnLimit() int {
	raw := strings.TrimSpace(os.Getenv("REMORA_DEEP_ANALYSIS_STRESS_TURNS"))
	if raw == "" {
		return 2
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 2
	}
	return n
}

func activeFrameworkNames(flow flowManifest, manifests map[string]*manifest.Manifest) []string {
	seen := map[string]bool{}
	var names []string
	for _, node := range flow.Nodes {
		if node.Framework == "" || seen[node.Framework] {
			continue
		}
		if manifests != nil {
			if _, ok := manifests[node.Framework]; !ok {
				continue
			}
		}
		seen[node.Framework] = true
		names = append(names, node.Framework)
	}
	if len(names) == 0 {
		for name := range manifests {
			names = append(names, name)
		}
	}
	return names
}

func failedSimulatedConversationStep(session *activeSessionInfo, input string, err error) flowRunStep {
	msg := "No se pudo ejecutar el follow-up simulado contra Radar."
	if err != nil {
		msg = msg + " " + err.Error()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return flowRunStep{
		Node:         "radar_deep_analysis_followup",
		Framework:    session.Framework,
		Capability:   session.Capability,
		Command:      session.FollowupCmd,
		Role:         "owner",
		Status:       "failed",
		Error:        msg,
		HumanSummary: "Follow-up falló para input simulado: " + input,
		StartedAt:    now,
		FinishedAt:   now,
		SegmentID:    session.SegmentID,
		SegmentMode:  segmentModeAnalytical,
		SegmentOwner: session.Framework,
		SegmentRole:  "owner",
	}
}

func simulatedDelegationPlanStep(session *activeSessionInfo, input string, requests []map[string]interface{}) flowRunStep {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var caps []string
	for _, req := range requests {
		if capName, _ := req["capability"].(string); capName != "" {
			caps = append(caps, capName)
		}
	}
	summary := fmt.Sprintf("Radar interpreta la pregunta y solicita evidencia auxiliar (%s) antes de sintetizar: %s", strings.Join(caps, ", "), input)
	if len(caps) == 0 {
		summary = "Radar evaluó pedir evidencia auxiliar, pero no produjo delegaciones ejecutables: " + input
	}
	return flowRunStep{
		Node:         "radar_deep_analysis_delegation_plan",
		Framework:    session.Framework,
		Capability:   session.Capability,
		Command:      session.FollowupCmd,
		Role:         "owner",
		Status:       "completed",
		HumanSummary: summary,
		StartedAt:    now,
		FinishedAt:   now,
		SegmentID:    session.SegmentID,
		SegmentMode:  segmentModeAnalytical,
		SegmentOwner: session.Framework,
		SegmentRole:  "owner",
	}
}

func simulatedDelegationResultStep(session *activeSessionInfo, payload map[string]interface{}) flowRunStep {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	capName := firstNonEmptyPipelineString(jsonFirstString(payload, "capability"), jsonFirstString(payload, "resolved_capability"))
	framework := firstNonEmptyPipelineString(jsonFirstString(payload, "framework"), "auxiliar")
	if strings.Contains(capName, "quality") || strings.Contains(capName, "audit") {
		framework = "auditor"
	} else if strings.Contains(capName, "data.") {
		framework = "sabio"
	}
	summary := fmt.Sprintf("%s devuelve evidencia para %s.", framework, capName)
	if raw, err := json.Marshal(payload); err == nil && len(raw) > 0 {
		summary = summary + " " + truncate(string(raw), 240)
	}
	return flowRunStep{
		Node:          "deep_analysis_delegate_" + safeFilePart(capName),
		Framework:     framework,
		Capability:    capName,
		Role:          "delegate",
		Status:        "completed",
		HumanSummary:  summary,
		ArtifactTypes: []string{capName},
		StartedAt:     now,
		FinishedAt:    now,
		SegmentID:     session.SegmentID,
		SegmentMode:   segmentModeAnalytical,
		SegmentOwner:  session.Framework,
		SegmentRole:   "delegate",
	}
}

func segmentSessionTurnCountFromPath(path string) int {
	if strings.TrimSpace(path) == "" {
		return 0
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return 0
	}
	return jsonFirstInt(payload, "turn_count")
}

func attachLatestArtifact(result *flowRunResult, path, typ string) {
	if result == nil || strings.TrimSpace(path) == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	if result.Artifacts == nil {
		result.Artifacts = map[string]flowRunArtifact{}
	}
	result.Artifacts[typ] = flowRunArtifact{
		Type:      typ,
		Source:    "flow_engine.dimension_conversation",
		Node:      "analysis_handoff_to_foco",
		Path:      path,
		Payload:   payload,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func flowBranchLimit(req flowRunRequest) int {
	limit := req.MaxBranches
	if limit <= 0 {
		limit = 3
	}
	if maxRaw := strings.TrimSpace(os.Getenv("REMORA_FLOW_MAX_BRANCHES")); maxRaw != "" {
		if max, err := strconv.Atoi(maxRaw); err == nil && max > 0 && limit > max {
			limit = max
		}
	}
	return limit
}

func cloneInitialArtifacts(in map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func optionIDForBranch(option map[string]string, idx int) string {
	if id := strings.TrimSpace(option["id"]); id != "" {
		return id
	}
	if label := strings.TrimSpace(option["label"]); label != "" {
		return label
	}
	return fmt.Sprintf("option_%d", idx+1)
}

func flowArtifactTypes(artifacts map[string]flowRunArtifact) map[string]bool {
	out := map[string]bool{}
	for typ := range artifacts {
		out[typ] = true
	}
	return out
}

func (s *server) generateFlowBranchSimulation(ctx context.Context, req flowRunRequest, step flowRunStep, artifacts map[string]flowRunArtifact) flowRunStep {
	options := flowActionOptionsFromArtifacts(artifacts)
	if len(options) == 0 {
		return flowRunStep{}
	}
	providerName := s.entryProviderName(req.Flow)
	started := time.Now().UTC().Format(time.RFC3339Nano)
	return flowRunStep{
		Node:          step.Node + "_branches",
		Framework:     providerName,
		Capability:    "focus.branch_test",
		Role:          "human",
		Status:        "completed",
		HumanSummary:  s.generateBranchSimulationText(ctx, req, step, options),
		ActionOptions: options,
		StartedAt:     started,
		FinishedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func flowActionOptionsFromArtifacts(artifacts map[string]flowRunArtifact) []map[string]string {
	payload := artifacts["action.options.v1"].Payload
	rawOptions, ok := payload.([]interface{})
	if !ok {
		if m, ok := payload.(map[string]interface{}); ok {
			rawOptions, ok = m["action_options"].([]interface{})
			if !ok {
				if typed, ok := m["action_options"].([]map[string]interface{}); ok {
					rawOptions = make([]interface{}, 0, len(typed))
					for _, option := range typed {
						rawOptions = append(rawOptions, option)
					}
				}
			}
		}
	}
	options := []map[string]string{}
	for _, raw := range rawOptions {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id := jsonFirstString(m, "id")
		label := jsonFirstString(m, "label")
		description := jsonFirstString(m, "description")
		boundID := jsonFirstString(m, "bound_id", "type")
		if label == "" {
			continue
		}
		options = append(options, map[string]string{"id": id, "label": label, "description": description, "bound_id": boundID})
	}
	return options
}

func (s *server) generateBranchSimulationText(ctx context.Context, req flowRunRequest, step flowRunStep, options []map[string]string) string {
	fallback := fallbackBranchSimulationText(options)
	spec, err := modelSpecFromManifest(s.allManifests[s.entryProviderName(req.Flow)])
	if err != nil {
		return fallback
	}
	client, err := llm.New(spec)
	if err != nil {
		return fallback
	}
	rawOptions, _ := json.Marshal(options)
	system := "Eres Foco simulando una prueba UX de ramas. Genera texto breve en español. No inventes datos de negocio. No uses JSON. Debes mostrar las 3 opciones como ramas paralelas, cada una con una frase natural de usuario simulada y qué pasaría después en términos genéricos de framework."
	user := fmt.Sprintf("Flujo: %s\nPaso Foco: %s\nOpciones de acción: %s\n\nGenera una tarjeta breve titulada \"Prueba paralela de opciones\". La frase del usuario debe sonar humana y distinta por opción.", req.Flow.ID, step.HumanSummary, string(rawOptions))
	out, err := client.Complete(ctx, llm.CompletionRequest{System: system, User: user, MaxTokens: 260})
	if err != nil {
		return fallback
	}
	out = strings.TrimSpace(out)
	if out == "" || len(out) > 1800 {
		return fallback
	}
	return out
}

func fallbackBranchSimulationText(options []map[string]string) string {
	var sb strings.Builder
	sb.WriteString("Prueba paralela de opciones\n")
	for i, option := range options {
		if i >= 3 {
			break
		}
		label := strings.TrimSpace(option["label"])
		description := strings.TrimSpace(option["description"])
		fmt.Fprintf(&sb, "\nRama %d — %s\n", i+1, label)
		fmt.Fprintf(&sb, "Usuario simulado: quiero %s.\n", strings.ToLower(label))
		if description != "" {
			fmt.Fprintf(&sb, "Qué pasaría: %s\n", description)
		} else {
			sb.WriteString("Qué pasaría: Foco deriva el flujo a los frameworks adecuados para esa decisión.\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

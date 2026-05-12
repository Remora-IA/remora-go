package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"channel/adapter"
	"channel/manifest"
	"remora-flujo/handoff"
	"remora-flujo/internal/llm"

	pal "github.com/remora-go/framework-paladin/paladin"
)

// runLoop es el corazón de la API. Recibe la respuesta del usuario (con
// posibles recursos) y devuelve la próxima pregunta a mostrar.
//
// Pasos:
//  1. Aplicar reglas de composición (flow.rules.json) sobre el contexto
//     actual: pueden reordenar drivers (PrependSpeaker) o pedir
//     preprocesamiento (Preprocess: "vision").
//  2. Si hay pregunta pendiente, marcarla como respondida y entregar la
//     respuesta al driver dueño (IngestAnswer). Si la respuesta tiene
//     imágenes y la regla pidió "vision", las pasamos primero por el
//     modelo multimodal del framework de destino y enriquecemos el answer.
//  3. Pollear drivers (en el orden eventual) por la próxima pregunta.
//
// Es agnóstico al framework: drivers son intercambiables y las reglas no
// modifican el comportamiento de cada framework por separado, sólo deciden
// quién habla cuándo.
func (s *server) runLoop(ctx context.Context, ch *adapter.Client, conv *Conversation, rules *FlowRules, manifests map[string]*manifest.Manifest, userAnswer string, resources []MessageResource) (handoff.QueuedQuestion, bool, error) {
	// Paladin trace: cada invocación del runLoop genera un trace completo
	// con spans para intent classification, reglas, enrichment, ingest y
	// poll. El archivo queda en temp/paladin/trace_*.json y se puede
	// leer desde GET /api/v1/traces/latest.
	trace := pal.NewTrace(fmt.Sprintf("runLoop(%s)", conv.ID))
	rootCtx := trace.Start()
	rootCtx.Var("conv_id", conv.ID)
	rootCtx.Var("profile", envOr("REMORA_PROFILE", "default"))
	rootCtx.Var("user_answer", userAnswer)
	rootCtx.Var("frameworks_active", conv.Frameworks)
	rootCtx.Actor("api_rest", "orquesta frameworks activos sin acoplarlos entre sí")
	rootCtx.Goal("decidir qué framework debe hablar después y entregar la respuesta del usuario al dueño correcto")
	rootCtx.Expect("framework_chain", "solo un framework queda elegido como próximo speaker")
	defer func() {
		rootCtx.End()
		trace.Flush()
	}()

	queue, err := loadQueue(conv.ID)
	if err != nil {
		rootCtx.Error(err)
		return handoff.QueuedQuestion{}, false, err
	}
	if len(queue.Frameworks) == 0 {
		queue.Frameworks = append([]string(nil), conv.Frameworks...)
	}

	drivers := driversFor(conv)
	if len(drivers) == 0 {
		err := fmt.Errorf("no hay drivers activos para la conversación")
		rootCtx.Error(err)
		return handoff.QueuedQuestion{}, false, err
	}
	rootCtx.Var("drivers_initial", driverNames(drivers))

	sessionOperationalContext := ""

	// 0. Sesión conversacional activa: si existe un segment.session.v1
	//    activo para este negocio, Radar (u otro owner) mantiene el tramo
	//    analítico. El usuario controla cuándo salir ("avanza", "gestionar",
	//    etc.). Mientras la intención sea analítica, ruteamos al followup
	//    command del owner sin pasar por el pipeline normal de drivers.
	if businessID := conversationBusinessID(conv); businessID != "" && userAnswer != "" {
		if session, _ := s.loadActiveSessionFromDisk(businessID, conv.ID); session != nil {
			// Claim the session for this conversation on first access.
			if session.ConversationID == "" {
				s.claimSessionForConversation(session.Path, conv.ID)
				session.ConversationID = conv.ID
			}

			sessionSpan := rootCtx.Child("session_routing")
			sessionSpan.Var("session_owner", session.Framework)
			sessionSpan.Var("session_capability", session.Capability)
			sessionSpan.Var("session_turn", session.TurnCount)
			sessionSpan.Var("session_max_turns", session.MaxTurns)
			sessionSpan.Var("session_conversation", session.ConversationID)

			intent := classifySegmentIntent(userAnswer, session)
			sessionSpan.Var("segment_intent", string(intent))

			switch intent {
			case segmentIntentExit:
				// User wants to leave/skip without acting.
				sessionSpan.Decision("session_exit",
					fmt.Sprintf("usuario abandona tramo analítico: %q", truncate(userAnswer, 80)))
				s.concludeSessionOnDisk(session.Path, "user_exit: "+truncate(userAnswer, 120))
				s.persistSessionSummary(businessID, session)
				sessionSpan.End()
				// Fall through: el runLoop normal retoma sin handoff especial.

			case segmentIntentOperational:
				// User wants to act with available data → structured handoff to Foco.
				sessionSpan.Decision("session_operational",
					fmt.Sprintf("usuario decide actuar: %q → handoff estructurado a Foco", truncate(userAnswer, 80)))
				s.concludeSessionOnDisk(session.Path, "user_operational: "+truncate(userAnswer, 120))
				s.persistAnalysisHandoff(ctx, ch, conv, manifests, session, userAnswer)
				sessionOperationalContext = fmt.Sprintf(
					"[handoff_analitico] Radar cerró el tramo analítico de %s y creó analysis.handoff.v1 para Foco; intención operativa del usuario: %s. ",
					session.Capability,
					userAnswer,
				)
				sessionSpan.End()
				// Fall through: el runLoop normal retoma con handoff artifact disponible.

			default:
				// Intent = continue: ejecutar followup command del owner.
				sessionSpan.Decision("session_continue",
					fmt.Sprintf("turno %d, ruteando a %s.%s", session.TurnCount+1, session.Framework, session.FollowupCmd))

				q, ok, err := s.executeSessionFollowup(ctx, ch, conv, manifests, queue, session, userAnswer)
				sessionSpan.End()
				if err != nil {
					fmt.Printf("[api_rest] session followup error (degrading): %v\n", err)
				} else if ok {
					return q, true, nil
				}
			}
		}
	}

	// 1a. Capability-based routing (intent classification). Antes de las
	// reglas declarativas, miramos si la respuesta del usuario matchea
	// los intent_examples de algún framework activo. Si hay match, ese
	// framework habla primero. Esto reemplaza reglas name-based del estilo
	// `prepend_speaker: "<nombre>"` por routing emergente desde el manifest.
	intentSpan := rootCtx.Child("intent_classification")
	intentSpan.Rule("capability_intent_routing", "la intención del usuario puede adelantar el framework activo cuyo manifest matchea intent_examples", map[string]any{
		"active": conv.Frameworks,
	})
	intentMatch := classifyIntent(userAnswer, manifests, conv.Frameworks)
	intentSpan.Var("intent_match", intentMatch)
	intentSpan.Check("capability_intent_routing", "intent_match vacío o framework activo", fmt.Sprintf("intent_match=%s", intentMatch), intentMatch == "" || slices.Contains(conv.Frameworks, intentMatch))
	if intentMatch != "" {
		drivers = reorderDrivers(drivers, intentMatch)
		intentSpan.Decision("reorder_drivers",
			fmt.Sprintf("user_answer matched intent_examples de %s", intentMatch))
	} else {
		intentSpan.Decision("no_reorder",
			"ningún framework activo tiene intent_examples que matcheen user_answer")
	}
	intentSpan.End()

	// 1b. Evaluar reglas de composición declarativas (overrides finos).
	evalCtx := EvalContext{
		FrameworksActive: conv.Frameworks,
		UserAnswerCount:  conv.UserAnswerCount,
		UserAnswer:       userAnswer,
		UserResources:    resources,
	}
	wantPreprocess := ""
	if rules != nil {
		rootCtx.Rule("flow_rules", "flow.rules.json puede reordenar drivers, pedir preprocesamiento o delegar por capability", map[string]any{
			"rules_count": len(rules.Rules),
		})
		for _, action := range rules.Match(evalCtx) {
			if action.PrependSpeaker != "" {
				drivers = reorderDrivers(drivers, action.PrependSpeaker)
			}
			if action.PrependSpeakerProviderOf != "" {
				if name := providerOfModelCapability(action.PrependSpeakerProviderOf, manifests, conv.Frameworks); name != "" {
					drivers = reorderDrivers(drivers, name)
				}
			}
			if action.DelegateToProviderOf != "" {
				if name := providerOfProducedCapability(action.DelegateToProviderOf, manifests, conv.Frameworks); name != "" {
					drivers = reorderDrivers(drivers, name)
				}
			}
			if action.Preprocess != "" && wantPreprocess == "" {
				wantPreprocess = action.Preprocess
			}
		}
	}

	rootCtx.Var("drivers_final", driverNames(drivers))
	rootCtx.Check("driver_selection", "hay al menos un driver después de reglas", fmt.Sprintf("drivers=%v", driverNames(drivers)), len(drivers) > 0)

	// 2. Procesar respuesta si hay alguna.
	if userAnswer != "" || len(resources) > 0 {
		// Target framework para preprocesamiento: el primero del orden eventual
		// (puede haber sido reordenado por una regla).
		targetFramework := drivers[0].Name()

		enrichSpan := rootCtx.Child("enrich_answer")
		enrichSpan.Var("raw_user_answer", userAnswer)
		enrichedAnswer := userAnswer
		if sessionOperationalContext != "" {
			enrichedAnswer = sessionOperationalContext + enrichedAnswer
			enrichSpan.Decision("session_operational_handoff",
				"Radar cerró sesión analítica y entregó contexto explícito a Foco")
		}
		// Inyectar contexto de la tarea activa de Foco si el
		// profile es cobranza. Esto hace que Sabio/Mensajero sepan sobre qué
		// cliente hablar sin depender de history parsing. Commit B.
		if task := activeTaskContext(); task != nil {
			if ctxLine := buildActiveTaskLine(enrichedAnswer, task); ctxLine != "" {
				// Un solo espacio: Channel rechaza \n como unsafe arg (Axioma 4.3).
				enrichedAnswer = ctxLine + enrichedAnswer
				enrichSpan.Var("active_task_injected", task.Title)
				enrichSpan.Decision("inject_active_task",
					fmt.Sprintf("Foco tiene task activa (%s); inyectada como contexto", task.Title))
			}
		}
		// Si el usuario seleccionó "Gestionar: <deudor>", transformar en query 360°
		// explícita para que Sabio sepa exactamente qué analizar.
		if strings.HasPrefix(userAnswer, "Gestionar: ") && len(drivers) > 0 && drivers[0].Name() == "sabio" {
			deudor := strings.TrimPrefix(userAnswer, "Gestionar: ")
			enrichedAnswer = fmt.Sprintf(
				"Genera un análisis 360° completo del cliente/deudor '%s'. "+
					"Incluye todo lo que tengas en los datos: saldo total adeudado, días de mora, "+
					"facturas y documentos pendientes, historial de pagos reciente, "+
					"y las 3 acciones de cobranza más urgentes a tomar con este cliente.",
				deudor)
			enrichSpan.Decision("gestionar_expand",
				fmt.Sprintf("chip Gestionar → query 360° explícita para %s", deudor))
		}
		if wantPreprocess == "vision" && hasImageResource(resources) {
			out, perr := preprocessVision(ctx, conv, targetFramework, userAnswer, resources)
			if perr != nil {
				fmt.Printf("[api_rest] preprocessVision error (continuando con texto plano): %v\n", perr)
				enrichSpan.Error(perr)
			} else {
				enrichedAnswer = out
				enrichSpan.Decision("vision_preprocess", "imagen(es) resueltas por modelo multimodal")
			}
		}
		enrichSpan.Var("enriched_answer", truncate(enrichedAnswer, 400))
		enrichSpan.End()

		if pending, ok := queue.GetNextPending(); ok {
			queue.MarkAnswered(pending.ID, enrichedAnswer)
			qctx := QueuedAnswerCtx{
				QuestionID:   pending.ID,
				ExternalID:   pending.ExternalID,
				QuestionText: pending.Text,
				Answer:       enrichedAnswer,
				Resources:    resources,
			}
			for _, d := range drivers {
				if d.Name() != pending.Framework {
					continue
				}
				ingestSpan := rootCtx.Child("ingest_answer")
				ingestSpan.Var("driver", d.Name())
				ingestSpan.Var("question_id", pending.ID)
				if err := d.IngestAnswer(ctx, ch, conv, qctx); err != nil {
					fmt.Printf("[api_rest] driver %s.IngestAnswer error: %v\n", d.Name(), err)
					ingestSpan.Error(err)
					ingestSpan.End()
					return handoff.QueuedQuestion{}, false, fmt.Errorf("%s: %w", d.Name(), err)
				}
				ingestSpan.End()
				break
			}
		} else {
			d := drivers[0]
			rootCtx.Handoff("user", d.Name(), "respuesta inicial sin pregunta pendiente; se entrega al primer driver del orden actual")
			qctx := QueuedAnswerCtx{
				Answer:       enrichedAnswer,
				QuestionText: "(contexto inicial)",
				Resources:    resources,
			}
			ingestSpan := rootCtx.Child("ingest_answer_initial")
			ingestSpan.Var("driver", d.Name())
			if err := d.IngestAnswer(ctx, ch, conv, qctx); err != nil {
				fmt.Printf("[api_rest] driver %s.IngestAnswer (initial) error: %v\n", d.Name(), err)
				ingestSpan.Error(err)
				ingestSpan.End()
				return handoff.QueuedQuestion{}, false, fmt.Errorf("%s: %w", d.Name(), err)
			}
			ingestSpan.End()
		}
		if err := saveQueue(conv.ID, queue); err != nil {
			rootCtx.Error(err)
			return handoff.QueuedQuestion{}, false, err
		}
	}

	// 3. Pedir siguiente pregunta a cada driver en el orden eventual.
	// Usamos PollQuestionFull para capturar chips opcionales.
	type fullPoller interface {
		PollQuestionFull(context.Context, *adapter.Client, *Conversation, map[string]bool) (nextQuestionResponse, bool)
	}
	asked := alreadyAskedExternalIDs(queue)
	for _, d := range drivers {
		pollSpan := rootCtx.Child("poll_question")
		pollSpan.Var("driver", d.Name())
		var r nextQuestionResponse
		var ok bool
		if fp, hasFull := d.(fullPoller); hasFull {
			r, ok = fp.PollQuestionFull(ctx, ch, conv, asked[d.Name()])
		} else {
			var text, reasoning, extID, askVia string
			text, reasoning, extID, askVia, ok = d.PollQuestion(ctx, ch, conv, asked[d.Name()])
			r = nextQuestionResponse{ID: extID, Text: text, Reasoning: reasoning, AskVia: askVia}
		}
		pollSpan.Var("has_question", ok)
		if !ok {
			pollSpan.Decision("skip_driver", "driver no tiene pregunta pendiente")
			pollSpan.End()
			continue
		}
		pollSpan.Var("question_text", truncate(r.Text, 200))
		pollSpan.Decision("speaker_chosen",
			fmt.Sprintf("%s habla porque es el primero con pregunta lista", d.Name()))
		pollSpan.Handoff("api_rest", d.Name(), "driver tiene pregunta lista para el usuario")
		pollSpan.End()
		queue.SetSpeaker(d.Name())
		qid := queue.AddQuestionWithReasoning(d.Name(), r.ID, r.Text, r.Reasoning, r.AskVia, r.Chips)
		if err := saveQueue(conv.ID, queue); err != nil {
			rootCtx.Error(err)
			return handoff.QueuedQuestion{}, false, err
		}
		for _, qq := range queue.Questions {
			if qq.ID == qid {
				return qq, true, nil
			}
		}
	}

	if err := saveQueue(conv.ID, queue); err != nil {
		rootCtx.Error(err)
		return handoff.QueuedQuestion{}, false, err
	}
	rootCtx.Decision("no_speaker", "ningún driver tiene pregunta; el usuario debe iniciar")
	return handoff.QueuedQuestion{}, false, nil
}

// driverNames devuelve los nombres de los drivers en orden. Helper para
// instrumentación (paladin vars).
func driverNames(ds []FrameworkDriver) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Name())
	}
	return out
}

// truncate corta un string en n runas y agrega "..." si era más largo.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func hasImageResource(rs []MessageResource) bool {
	for _, r := range rs {
		if r.Type == "image" {
			return true
		}
	}
	return false
}

// alreadyAskedExternalIDs construye, por framework, el set de external_ids
// que ya fueron encolados (independiente de su estado), para evitar duplicar
// preguntas.
func alreadyAskedExternalIDs(q *handoff.QuestionsQueue) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, qq := range q.Questions {
		if qq.ExternalID == "" {
			continue
		}
		if _, ok := out[qq.Framework]; !ok {
			out[qq.Framework] = map[string]bool{}
		}
		out[qq.Framework][qq.ExternalID] = true
	}
	return out
}

// executeSessionFollowup runs the session owner's followup command and
// returns the response as a queued question. This bypasses the normal
// driver poll loop — the session owner speaks directly.
func (s *server) executeSessionFollowup(
	ctx context.Context,
	ch *adapter.Client,
	conv *Conversation,
	manifests map[string]*manifest.Manifest,
	queue *handoff.QuestionsQueue,
	session *activeSessionInfo,
	userAnswer string,
) (handoff.QueuedQuestion, bool, error) {
	m, ok := manifests[session.Framework]
	if !ok || m == nil {
		return handoff.QueuedQuestion{}, false, fmt.Errorf("manifest not found for session owner %s", session.Framework)
	}
	cmd, ok := m.Commands[session.FollowupCmd]
	if !ok {
		return handoff.QueuedQuestion{}, false, fmt.Errorf("followup command %s not found in %s manifest", session.FollowupCmd, session.Framework)
	}

	businessID := conversationBusinessID(conv)
	turnCount := s.incrementSessionOnDisk(session.Path)

	params := map[string]string{
		"input":       userAnswer,
		"business_id": businessID,
		"turn_count":  fmt.Sprintf("%d", turnCount),
		"entity_ref":  "",
		"entity_type": "customer",
	}
	if commandHasParam(cmd, "semantic_pack") {
		params["semantic_pack"] = s.businessSemanticPackPath(businessID)
	}
	if commandHasParam(cmd, "history") {
		params["history"] = encodeRecentHistory(conv.ID, "")
	}
	if commandHasParam(cmd, "context_b64") {
		params["context_b64"] = encodeConversationRuntimeContext(conv)
	}
	// Wire artifact params: previous analysis, priority list, delegation results.
	if commandHasParam(cmd, "previous_analysis_json") || commandHasParam(cmd, "previous_analysis_path") {
		if path := s.latestFlowArtifactPath(businessID, "analysis.case_review.v1"); path != "" {
			params["previous_analysis_path"] = path
		}
	}
	if commandHasParam(cmd, "priority_list_json") || commandHasParam(cmd, "priority_list_path") {
		if path := s.latestFlowArtifactPath(businessID, "collection.priority_list.v1"); path != "" {
			params["priority_list_path"] = path
		}
	}
	if commandHasParam(cmd, "llm_followup_json") {
		if draft := s.generateOwnerFollowupWithLLM(ctx, conv, m, userAnswer, params); draft != "" {
			params["llm_followup_json"] = draft
		}
	}

	args, err := cmd.ResolveArgs(params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
	if err != nil {
		return handoff.QueuedQuestion{}, false, fmt.Errorf("resolve args for %s.%s: %w", session.Framework, session.FollowupCmd, err)
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	resp, err := ch.ExecuteCommand(ctx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		return handoff.QueuedQuestion{}, false, fmt.Errorf("execute %s.%s: %w", session.Framework, session.FollowupCmd, err)
	}
	if !resp.Success || resp.ExitCode != 0 {
		detail := strings.TrimSpace(resp.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(resp.Stdout)
		}
		return handoff.QueuedQuestion{}, false, fmt.Errorf("%s.%s failed (exit %d): %s", session.Framework, session.FollowupCmd, resp.ExitCode, detail)
	}

	// Parse the followup response. The command produces analysis.followup.v1
	// with a "text" field. We convert it into a queued question.
	//
	// Delegation: if the response includes delegation_requests, execute them
	// inline (one hop) and re-invoke the followup with results so the owner
	// can integrate the delegated data into its answer.
	text, delegationRequests := extractFollowupTextAndDelegations(resp.Stdout)
	if len(delegationRequests) > 0 {
		delegationResults := s.executeDelegations(ctx, ch, conv, manifests, delegationRequests, session.AllowedDelegates)
		if len(delegationResults) > 0 {
			// Re-run followup with delegation results.
			drJSON, _ := json.Marshal(delegationResults)
			params["delegation_results_json"] = string(drJSON)
			args2, err := cmd.ResolveArgs(params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
			if err == nil {
				fullArgs2 := runtime.FullArgs(args2, m)
				if resp2, err := ch.ExecuteCommand(ctx, runtime.Command, fullArgs2, runtime.Cwd); err == nil && resp2.Success && resp2.ExitCode == 0 {
					text, _ = extractFollowupTextAndDelegations(resp2.Stdout)
				}
			}
		}
	}
	if text == "" {
		return handoff.QueuedQuestion{}, false, nil
	}

	// Build chips from session signals so the user sees quick-action buttons.
	var chips []string
	for _, sig := range session.ContinueSignals {
		chips = append(chips, sig)
	}
	for _, sig := range session.ExitSignals {
		chips = append(chips, sig)
	}

	extID := fmt.Sprintf("session_followup_%d", turnCount)
	queue.SetSpeaker(session.Framework)
	qid := queue.AddQuestionWithReasoning(
		session.Framework,
		extID,
		text,
		fmt.Sprintf("Sesión analítica turno %d/%d — %s mantiene el tramo", turnCount, session.MaxTurns, session.Framework),
		"text",
		chips,
	)
	if err := saveQueue(conv.ID, queue); err != nil {
		return handoff.QueuedQuestion{}, false, err
	}
	for _, qq := range queue.Questions {
		if qq.ID == qid {
			return qq, true, nil
		}
	}
	return handoff.QueuedQuestion{}, false, nil
}

// executeDelegations runs each delegation request against the framework that
// provides the requested capability. Only capabilities listed in
// allowedDelegates are executed; requests outside the whitelist are silently
// skipped. This enforces the session owner's declared delegation boundary.
func (s *server) executeDelegations(
	ctx context.Context,
	ch *adapter.Client,
	conv *Conversation,
	manifests map[string]*manifest.Manifest,
	requests []map[string]interface{},
	allowedDelegates []string,
) map[string]interface{} {
	allowed := make(map[string]bool, len(allowedDelegates))
	for _, d := range allowedDelegates {
		allowed[strings.ToLower(strings.TrimSpace(d))] = true
	}
	results := map[string]interface{}{}
	for _, req := range requests {
		fw, _ := req["framework"].(string)
		capName, _ := req["capability"].(string)
		if fw == "" || capName == "" {
			continue
		}
		// Enforce allowed_delegates whitelist.
		if !allowed[strings.ToLower(capName)] {
			fmt.Printf("[api_rest] delegation to %s/%s blocked: not in allowed_delegates\n", fw, capName)
			continue
		}
		m, ok := manifests[fw]
		if !ok || m == nil {
			// Try to find via capability routing.
			if provider := providerOfProducedCapability(capName, manifests, conv.Frameworks); provider != "" {
				m = manifests[provider]
				fw = provider
			}
			if m == nil {
				continue
			}
		}
		// Find the command for this capability in the manifest.
		cmdName := ""
		if cap, ok := findManifestCapability(m, capName); ok && cap.Command != "" {
			cmdName = cap.Command
		}
		if cmdName == "" {
			// Try default command names based on capability.
			for _, candidate := range []string{"query", "entity-360", "audit", "analyze"} {
				if _, exists := m.Commands[candidate]; exists {
					cmdName = candidate
					break
				}
			}
		}
		if cmdName == "" {
			continue
		}
		delegateCmd, ok := m.Commands[cmdName]
		if !ok {
			continue
		}
		dParams := map[string]string{
			"business_id": conversationBusinessID(conv),
		}
		if p, ok := req["params"].(map[string]interface{}); ok {
			for k, v := range p {
				if s, ok := v.(string); ok {
					dParams[k] = s
				}
			}
		}
		if commandHasParam(delegateCmd, "question") {
			if q, ok := dParams["question"]; ok {
				dParams["question"] = q
			}
		}
		if commandHasParam(delegateCmd, "db") {
			dParams["db"] = businessDataDBPath(s.rootDir, conversationBusinessID(conv))
		}
		if commandHasParam(delegateCmd, "semantic_pack") {
			dParams["semantic_pack"] = s.businessSemanticPackPath(conversationBusinessID(conv))
		}
		dArgs, err := delegateCmd.ResolveArgs(dParams, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
		if err != nil {
			continue
		}
		dRuntime := resolveManifestRuntime(s.rootDir, m)
		dFullArgs := dRuntime.FullArgs(dArgs, m)
		dResp, err := ch.ExecuteCommand(ctx, dRuntime.Command, dFullArgs, dRuntime.Cwd)
		if err != nil || !dResp.Success || dResp.ExitCode != 0 {
			continue
		}
		// Parse delegation output.
		var delegateOutput map[string]interface{}
		if json.Unmarshal([]byte(strings.TrimSpace(dResp.Stdout)), &delegateOutput) == nil {
			results[capName] = delegateOutput
			if text, ok := delegateOutput["text"].(string); ok {
				results["text"] = text
				results["answer"] = text
			}
		} else {
			results[capName] = dResp.Stdout
			results["text"] = dResp.Stdout
		}
	}
	return results
}

func (s *server) generateOwnerFollowupWithLLM(ctx context.Context, conv *Conversation, m *manifest.Manifest, userAnswer string, params map[string]string) string {
	spec, err := modelSpecFromManifest(m)
	if err != nil {
		return ""
	}
	client, err := llm.New(spec)
	if err != nil {
		return ""
	}
	contextParts := []string{}
	for _, key := range []string{"previous_analysis_path", "priority_list_path", "delegation_results_path"} {
		if path := strings.TrimSpace(params[key]); path != "" {
			if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
				contextParts = append(contextParts, fmt.Sprintf("%s:\n%s", key, truncate(string(raw), 5000)))
			}
		}
	}
	for _, key := range []string{"previous_analysis_json", "priority_list_json", "delegation_results_json"} {
		if raw := strings.TrimSpace(params[key]); raw != "" {
			contextParts = append(contextParts, fmt.Sprintf("%s:\n%s", key, truncate(raw, 5000)))
		}
	}
	system := "Eres Radar, owner conversacional de un tramo analítico. Responde en español, grounded en los artifacts recibidos. No ejecutes acciones operativas ni prometas side effects. Si faltan datos, dilo como gap. Devuelve solo JSON válido con campos: text, confidence, residual_risks, accepted_gaps, recommendation."
	user := fmt.Sprintf("Conversación: %s\nPregunta del usuario: %s\nContexto/artifacts:\n%s", conv.ID, userAnswer, strings.Join(contextParts, "\n\n---\n\n"))
	out, err := client.Complete(ctx, llm.CompletionRequest{System: system, User: user, MaxTokens: 700})
	if err != nil {
		return ""
	}
	out = strings.TrimSpace(out)
	out = strings.TrimPrefix(out, "```json")
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimSuffix(out, "```")
	out = strings.TrimSpace(out)
	var payload map[string]interface{}
	if json.Unmarshal([]byte(out), &payload) != nil {
		payload = map[string]interface{}{"text": out}
	}
	if jsonFirst(payload, "text", "answer", "analysis") == "" {
		return ""
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

// persistSessionSummary creates a minimal analysis.session_summary.v1 when
// the user exits without acting (dismiss/skip). No handoff context needed.
func (s *server) persistSessionSummary(businessID string, session *activeSessionInfo) {
	if businessID == "" || session == nil {
		return
	}
	summary := map[string]interface{}{
		"artifact_type":   "analysis.session_summary.v1",
		"business_id":     businessID,
		"conversation_id": session.ConversationID,
		"owner": map[string]string{
			"framework":  session.Framework,
			"capability": session.Capability,
		},
		"turns_completed": session.TurnCount,
		"max_turns":       session.MaxTurns,
		"concluded_at":    time.Now().UTC().Format(time.RFC3339),
		"reason":          "user_exit",
	}
	runID := "session_" + businessID
	_ = s.persistFlowArtifact(runID, "session_summary", "analysis.session_summary.v1", summary)
}

// persistAnalysisHandoff creates a structured analysis.handoff.v1 artifact
// when the user decides to act with available data (operational intent).
// This gives Foco rich context for the operational phase:
//   - analytical summary + recommendation
//   - residual risks and accepted gaps
//   - analysis confidence/sufficiency
//   - the user's trigger message
//
// The handoff is built from the last analysis artifacts on disk.
func (s *server) persistAnalysisHandoff(
	ctx context.Context,
	ch *adapter.Client,
	conv *Conversation,
	manifests map[string]*manifest.Manifest,
	session *activeSessionInfo,
	userTrigger string,
) {
	businessID := conversationBusinessID(conv)
	if businessID == "" || session == nil {
		return
	}

	// Gather context from the latest analysis artifacts.
	analysisSummary := ""
	recommendation := ""
	var residualRisks []string
	var acceptedGaps []string
	confidence := "partial"

	// Load last case review if available.
	if path := s.latestFlowArtifactPath(businessID, "analysis.case_review.v1"); path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			var review map[string]interface{}
			if json.Unmarshal(raw, &review) == nil {
				analysisSummary = jsonFirst(review, "text", "summary", "analysis")
				recommendation = jsonFirst(review, "recommendation", "next_action", "suggested_action")
				residualRisks = jsonStringSlice(review, "risks", "residual_risks", "risk_factors")
				acceptedGaps = jsonStringSlice(review, "gaps", "data_gaps", "blocking_gaps")
			}
		}
	}

	// Load last followup if it's more recent.
	if path := s.latestFlowArtifactPath(businessID, "analysis.followup.v1"); path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			var followup map[string]interface{}
			if json.Unmarshal(raw, &followup) == nil {
				if text := jsonFirst(followup, "text"); text != "" {
					analysisSummary = text
				}
			}
		}
	}

	// Determine confidence based on turns and data availability.
	if session.TurnCount >= 3 && len(acceptedGaps) == 0 {
		confidence = "high"
	} else if session.TurnCount >= 1 {
		confidence = "moderate"
	}

	handoff := map[string]interface{}{
		"artifact_type":   "analysis.handoff.v1",
		"business_id":     businessID,
		"conversation_id": conv.ID,
		"from": map[string]string{
			"framework":  session.Framework,
			"capability": session.Capability,
		},
		"to": map[string]string{
			"framework": "foco",
			"role":      "operational",
		},
		"user_trigger":       userTrigger,
		"turns_completed":    session.TurnCount,
		"analytical_summary": analysisSummary,
		"recommendation":     recommendation,
		"residual_risks":     residualRisks,
		"accepted_gaps":      acceptedGaps,
		"confidence":         confidence,
		"created_at":         time.Now().UTC().Format(time.RFC3339),
	}

	runID := "handoff_" + businessID
	_ = s.persistFlowArtifact(runID, "analysis_handoff", "analysis.handoff.v1", handoff)
}

// jsonFirst returns the first non-empty string value found under any of the
// given keys. Useful for reading from artifacts with varying schemas.
func jsonFirst(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// jsonStringSlice tries multiple keys and returns the first non-empty string
// slice found. Falls back to splitting a comma-separated string value.
func jsonStringSlice(m map[string]interface{}, keys ...string) []string {
	for _, k := range keys {
		if arr, ok := m[k].([]interface{}); ok && len(arr) > 0 {
			out := make([]string, 0, len(arr))
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.Split(s, ",")
		}
	}
	return nil
}

// extractFollowupTextAndDelegations parses the stdout of an analyze-followup
// command and extracts the human-readable text plus any delegation_requests.
func extractFollowupTextAndDelegations(stdout string) (string, []map[string]interface{}) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "", nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &payload); err == nil {
		text := ""
		if t, ok := payload["text"].(string); ok && strings.TrimSpace(t) != "" {
			text = t
		} else if ga, ok := payload["grounded_answer"].(map[string]interface{}); ok {
			if t, ok := ga["text"].(string); ok {
				text = t
			}
		}
		// Extract delegation_requests.
		var delegations []map[string]interface{}
		if raw, ok := payload["delegation_requests"].([]interface{}); ok {
			for _, item := range raw {
				if m, ok := item.(map[string]interface{}); ok {
					delegations = append(delegations, m)
				}
			}
		}
		if text != "" {
			return text, delegations
		}
	}
	return stdout, nil
}

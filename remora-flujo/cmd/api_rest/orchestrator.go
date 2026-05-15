package main

import (
	"context"
	"encoding/base64"
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
	sessionReentryDriver := ""

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
				consumed := consumePendingQuestionsForFramework(queue, session.Framework, userAnswer)
				sessionSpan.Var("session_question_consumed", consumed)
				sessionSpan.Decision("session_exit",
					fmt.Sprintf("usuario abandona tramo analítico: %q", truncate(userAnswer, 80)))
				s.concludeSessionOnDisk(session.Path, "user_exit: "+truncate(userAnswer, 120))
				s.persistSessionSummary(businessID, session)
				sessionSpan.End()
				// Fall through: el runLoop normal retoma sin handoff especial.

			case segmentIntentOperational:
				consumed := consumePendingQuestionsForFramework(queue, session.Framework, userAnswer)
				sessionSpan.Var("session_question_consumed", consumed)
				transition := s.concludeAnalysisTransition(ctx, ch, conv, manifests, session, userAnswer, false)
				sessionReentryDriver = operationalCaseManagerDriver(manifests, conv)
				if sessionReentryDriver != "" {
					sessionSpan.Var("session_reentry_driver", sessionReentryDriver)
				}
				if transition.ReadyForOperation {
					sessionSpan.Decision("session_operational",
						fmt.Sprintf("usuario decide actuar: %q → handoff estructurado a Foco", truncate(userAnswer, 80)))
					sessionOperationalContext = fmt.Sprintf(
						"[handoff_analitico] Radar cerró el tramo analítico de %s y creó analysis.handoff.v1 para Foco; intención operativa del usuario: %s. ",
						session.Capability,
						userAnswer,
					)
				} else {
					sessionSpan.Decision("session_review_pending",
						fmt.Sprintf("usuario quiere operar, pero Radar cierra sin handoff operativo: %s", transition.Reason))
					sessionOperationalContext = fmt.Sprintf(
						"[analysis_review_pending] Radar cerró el tramo analítico de %s sin handoff operativo; Foco retoma el stewardship del caso. Motivo: %s. Intención del usuario: %s. ",
						session.Capability,
						transition.Reason,
						userAnswer,
					)
				}
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
	if sessionReentryDriver != "" {
		drivers = reorderDrivers(drivers, sessionReentryDriver)
		rootCtx.Decision("session_case_manager_reentry",
			fmt.Sprintf("%s retoma el caso tras cerrar el tramo analítico", sessionReentryDriver))
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

func operationalCaseManagerDriver(manifests map[string]*manifest.Manifest, conv *Conversation) string {
	if conv == nil {
		return ""
	}
	if name := providerOfProducedCapability("task.next", manifests, conv.Frameworks); name != "" {
		return name
	}
	if name := providerOfProducedCapability("focus.next_task.v1", manifests, conv.Frameworks); name != "" {
		return name
	}
	if slices.Contains(conv.Frameworks, "foco") {
		return "foco"
	}
	return ""
}

func consumePendingQuestionsForFramework(queue *handoff.QuestionsQueue, framework, answer string) bool {
	if queue == nil || strings.TrimSpace(framework) == "" {
		return false
	}
	consumed := false
	for i := range queue.Questions {
		if queue.Questions[i].Framework != framework || queue.Questions[i].Status != handoff.QuestionPending {
			continue
		}
		queue.Questions[i].Status = handoff.QuestionAnswered
		queue.Questions[i].Answer = answer
		queue.Questions[i].AnsweredAt = time.Now()
		consumed = true
	}
	return consumed
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
	result, err := s.executeSessionFollowupDetailed(ctx, ch, conv, manifests, queue, session, userAnswer, sessionFollowupModeRuntime)
	if err != nil || !result.OK {
		return handoff.QueuedQuestion{}, false, err
	}
	return result.Question, true, nil
}

type sessionFollowupExecution struct {
	Question              handoff.QueuedQuestion
	OK                    bool
	DelegationRequests    []map[string]interface{}
	DelegationResults     map[string]interface{}
	DelegatedCapabilities []string
	AnalysisPhase         string
	SynthesisAttempted    bool
	Synthesized           bool
	SynthesisError        string
	DelegationResultsPath string
}

type sessionFollowupMode string

const (
	sessionFollowupModeRuntime    sessionFollowupMode = "runtime"
	sessionFollowupModeSimulation sessionFollowupMode = "simulation"
)

type analysisReadinessDecision struct {
	State             string
	Reason            string
	ReadyForOperation bool
	Confidence        string
	SourceArtifact    string
	SourcePath        string
	Summary           string
	Recommendation    string
	DataGaps          []string
	ResidualRisks     []string
	Simulation        bool
}

type analysisClosureResult struct {
	State             string
	Reason            string
	ReadyForOperation bool
	ArtifactType      string
	ArtifactPath      string
}

func (s *server) executeSessionFollowupDetailed(
	ctx context.Context,
	ch *adapter.Client,
	conv *Conversation,
	manifests map[string]*manifest.Manifest,
	queue *handoff.QuestionsQueue,
	session *activeSessionInfo,
	userAnswer string,
	mode sessionFollowupMode,
) (sessionFollowupExecution, error) {
	m, ok := manifests[session.Framework]
	if !ok || m == nil {
		return sessionFollowupExecution{}, fmt.Errorf("manifest not found for session owner %s", session.Framework)
	}
	cmd, ok := m.Commands[session.FollowupCmd]
	if !ok {
		return sessionFollowupExecution{}, fmt.Errorf("followup command %s not found in %s manifest", session.FollowupCmd, session.Framework)
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
		if draft := s.generateOwnerFollowupWithLLM(ctx, conv, m, userAnswer, params, "plan"); draft != "" {
			params["llm_followup_json"] = draft
		}
	}

	runID := fmt.Sprintf("session_followup_%s_%d", safeFilePart(businessID), turnCount)
	args, err := s.resolvePortableCommandArgs(runID, safeFilePart(session.Framework)+"_"+safeFilePart(session.FollowupCmd), cmd, params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
	if err != nil {
		return sessionFollowupExecution{}, fmt.Errorf("resolve args for %s.%s: %w", session.Framework, session.FollowupCmd, err)
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	resp, err := ch.ExecuteCommand(ctx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		return sessionFollowupExecution{}, fmt.Errorf("execute %s.%s: %w", session.Framework, session.FollowupCmd, err)
	}
	if !resp.Success || resp.ExitCode != 0 {
		detail := strings.TrimSpace(resp.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(resp.Stdout)
		}
		return sessionFollowupExecution{}, fmt.Errorf("%s.%s failed (exit %d): %s", session.Framework, session.FollowupCmd, resp.ExitCode, detail)
	}

	// Parse the followup response. The command produces analysis.followup.v1
	// with a "text" field. We convert it into a queued question.
	//
	// Delegation: if the response includes delegation_requests, execute them
	// inline (one hop) and re-invoke the followup with results so the owner
	// can integrate the delegated data into its answer.
	finalStdout := resp.Stdout
	text, delegationRequests := extractFollowupTextAndDelegations(resp.Stdout, userAnswer)
	execution := sessionFollowupExecution{
		DelegationRequests: delegationRequests,
		AnalysisPhase:      followupAnalysisPhase(resp.Stdout),
	}
	if len(delegationRequests) > 0 {
		delegationResults := s.executeDelegations(ctx, ch, conv, manifests, delegationRequests, session.AllowedDelegates)
		execution.DelegationResults = delegationResults
		execution.DelegatedCapabilities = sortedKeysInterfaceMap(delegationResults)
		if len(delegationResults) > 0 {
			// Re-run followup with delegation results.
			drJSON, _ := json.Marshal(delegationResults)
			params["delegation_results_json"] = string(drJSON)
			execution.SynthesisAttempted = true
			execution.DelegationResultsPath = s.materializePortableArtifactParam(runID, safeFilePart(session.Framework)+"_"+safeFilePart(session.FollowupCmd)+"_delegation_results", cmd, params, "delegation_results_json")
			if commandHasParam(cmd, "delegation_results_path") && execution.DelegationResultsPath == "" {
				execution.SynthesisError = fmt.Sprintf("%s.%s no materializó delegation_results_path para el second pass", session.Framework, session.FollowupCmd)
				finalStdout = synthesizeFollowupRuntimeFailureArtifact(businessID, turnCount, execution, userAnswer)
				text, _ = extractFollowupTextAndDelegations(finalStdout, userAnswer)
			} else {
				// Important: generate a fresh synthesis draft AFTER delegation.
				// Never reuse the plan-phase draft that did not know the new evidence.
				delete(params, "llm_followup_json")
				if commandHasParam(cmd, "llm_followup_json") {
					if draft := s.generateOwnerFollowupWithLLM(ctx, conv, m, userAnswer, params, "synthesis"); draft != "" {
						params["llm_followup_json"] = draft
					}
				}
				args2, err := s.resolvePortableCommandArgs(runID, safeFilePart(session.Framework)+"_"+safeFilePart(session.FollowupCmd)+"_synthesis", cmd, params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
				if err != nil {
					execution.SynthesisError = fmt.Sprintf("resolve synthesis args for %s.%s: %v", session.Framework, session.FollowupCmd, err)
					finalStdout = synthesizeFollowupRuntimeFailureArtifact(businessID, turnCount, execution, userAnswer)
					text, _ = extractFollowupTextAndDelegations(finalStdout, userAnswer)
				} else {
					fullArgs2 := runtime.FullArgs(args2, m)
					resp2, err := ch.ExecuteCommand(ctx, runtime.Command, fullArgs2, runtime.Cwd)
					if err != nil {
						execution.SynthesisError = fmt.Sprintf("execute synthesis for %s.%s: %v", session.Framework, session.FollowupCmd, err)
						finalStdout = synthesizeFollowupRuntimeFailureArtifact(businessID, turnCount, execution, userAnswer)
						text, _ = extractFollowupTextAndDelegations(finalStdout, userAnswer)
					} else if !resp2.Success || resp2.ExitCode != 0 {
						detail := strings.TrimSpace(firstNonEmptyPipelineString(resp2.Stderr, resp2.Stdout))
						execution.SynthesisError = fmt.Sprintf("%s.%s synthesis failed (exit %d): %s", session.Framework, session.FollowupCmd, resp2.ExitCode, detail)
						finalStdout = synthesizeFollowupRuntimeFailureArtifact(businessID, turnCount, execution, userAnswer)
						text, _ = extractFollowupTextAndDelegations(finalStdout, userAnswer)
					} else {
						finalStdout = resp2.Stdout
						text, _ = extractFollowupTextAndDelegations(resp2.Stdout, userAnswer)
						execution.AnalysisPhase = followupAnalysisPhase(resp2.Stdout)
						execution.Synthesized = strings.EqualFold(execution.AnalysisPhase, "synthesis")
						if !execution.Synthesized {
							execution.SynthesisError = fmt.Sprintf("%s.%s second pass no llegó a synthesis (phase=%s)", session.Framework, session.FollowupCmd, execution.AnalysisPhase)
							finalStdout = synthesizeFollowupRuntimeFailureArtifact(businessID, turnCount, execution, userAnswer)
							text, _ = extractFollowupTextAndDelegations(finalStdout, userAnswer)
						}
					}
				}
			}
		}
	}
	if text == "" {
		return sessionFollowupExecution{}, nil
	}
	s.persistFollowupArtifact(businessID, conv.ID, turnCount, finalStdout, execution, mode)
	if mode == sessionFollowupModeRuntime {
		s.persistAnalysisReadinessArtifact(businessID, conv.ID, session, finalStdout)
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
		return sessionFollowupExecution{}, err
	}
	for _, qq := range queue.Questions {
		if qq.ID == qid {
			execution.Question = qq
			execution.OK = true
			return execution, nil
		}
	}
	return sessionFollowupExecution{}, nil
}

func (s *server) persistFollowupArtifact(businessID, conversationID string, turnCount int, stdout string, execution sessionFollowupExecution, mode sessionFollowupMode) {
	if strings.TrimSpace(businessID) == "" || strings.TrimSpace(stdout) == "" {
		return
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload) != nil {
		return
	}
	if jsonFirstString(payload, "artifact_type", "type") == "" {
		payload["artifact_type"] = "analysis.followup.v1"
	}
	if jsonFirstString(payload, "business_id") == "" {
		payload["business_id"] = businessID
	}
	if strings.TrimSpace(conversationID) != "" && jsonFirstString(payload, "conversation_id") == "" {
		payload["conversation_id"] = conversationID
	}
	if _, ok := payload["turn_count"]; !ok {
		payload["turn_count"] = turnCount
	}
	payload["mode"] = string(mode)
	payload["simulation"] = mode == sessionFollowupModeSimulation
	if strings.TrimSpace(jsonFirstString(payload, "analysis_phase")) == "" && strings.TrimSpace(execution.AnalysisPhase) != "" {
		payload["analysis_phase"] = execution.AnalysisPhase
	}
	if _, ok := payload["synthesized"]; !ok {
		payload["synthesized"] = execution.Synthesized
	}
	if execution.SynthesisAttempted {
		payload["synthesis_attempted"] = true
	}
	if strings.TrimSpace(execution.SynthesisError) != "" {
		payload["synthesis_error"] = execution.SynthesisError
	}
	if len(execution.DelegatedCapabilities) > 0 {
		payload["delegated_capabilities"] = execution.DelegatedCapabilities
	}
	if strings.TrimSpace(execution.DelegationResultsPath) != "" {
		payload["delegation_results_path"] = execution.DelegationResultsPath
	}
	runID := fmt.Sprintf("followup_%s_%d", businessID, turnCount)
	_ = s.persistFlowArtifact(runID, "analysis_followup", "analysis.followup.v1", payload)
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
	envelope := map[string]interface{}{
		"requests": []interface{}{},
		"results":  []interface{}{},
		"summary": map[string]interface{}{
			"success_count": 0,
			"failure_count": 0,
			"partial_count": 0,
		},
	}
	for i, req := range requests {
		reqID := firstNonEmptyPipelineString(jsonFirstString(req, "request_id"), fmt.Sprintf("delegation_%d", i+1))
		semanticFramework := strings.TrimSpace(jsonFirstString(req, "framework"))
		semanticCapability := strings.TrimSpace(jsonFirstString(req, "capability"))
		resolvedFramework, resolvedCapability := resolveDelegationExecutionTarget(req)
		requestRecord := map[string]interface{}{
			"request_id":          reqID,
			"framework":           semanticFramework,
			"capability":          semanticCapability,
			"resolved_framework":  resolvedFramework,
			"resolved_capability": resolvedCapability,
			"params":              req["params"],
			"reason":              jsonFirstString(req, "reason"),
			"analysis_intent":     delegationAnalysisIntent(req),
			"entity_ref":          firstNonEmptyPipelineString(jsonFirstString(req, "entity_ref"), jsonFirstString(req, "deudor_id")),
			"entity_type":         canonicalDelegationEntityType(jsonFirstString(req, "entity_type", "type")),
		}
		envelope["requests"] = append(envelope["requests"].([]interface{}), requestRecord)
		if resolvedFramework == "" || resolvedCapability == "" {
			envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, "delegation sin target ejecutable"))
			incrementDelegationSummary(envelope, false, false)
			continue
		}
		// Enforce allowed_delegates whitelist on the resolved executable capability.
		if !allowed[strings.ToLower(resolvedCapability)] {
			fmt.Printf("[api_rest] delegation to %s/%s blocked: not in allowed_delegates\n", resolvedFramework, resolvedCapability)
			envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, "capability bloqueada por allowed_delegates"))
			incrementDelegationSummary(envelope, false, false)
			continue
		}
		m, ok := manifests[resolvedFramework]
		if !ok || m == nil {
			// Try to find via capability routing.
			if provider := providerOfProducedCapability(resolvedCapability, manifests, conv.Frameworks); provider != "" {
				m = manifests[provider]
				resolvedFramework = provider
				requestRecord["resolved_framework"] = resolvedFramework
			}
			if m == nil {
				envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, "manifest no encontrado para capability resuelta"))
				incrementDelegationSummary(envelope, false, false)
				continue
			}
		}
		// Find the command for this capability in the manifest.
		cmdName := ""
		if cap, ok := findManifestCapability(m, resolvedCapability); ok && cap.Command != "" {
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
			envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, "no se encontró command para capability resuelta"))
			incrementDelegationSummary(envelope, false, false)
			continue
		}
		delegateCmd, ok := m.Commands[cmdName]
		if !ok {
			envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, "command resuelto no existe en manifest"))
			incrementDelegationSummary(envelope, false, false)
			continue
		}
		businessID := conversationBusinessID(conv)
		dParams := map[string]string{}
		if p, ok := req["params"].(map[string]interface{}); ok {
			for k, v := range p {
				if s := delegationParamStringValue(v); s != "" {
					dParams[k] = s
				}
			}
		}
		setParamIfDeclared(delegateCmd, dParams, "business_id", businessID)
		setParamIfDeclared(delegateCmd, dParams, "capability", resolvedCapability)
		setParamIfDeclared(delegateCmd, dParams, "semantic_capability", semanticCapability)
		if entityRef, entityType := delegationEntityIdentity(req, conv); entityRef != "" {
			setParamIfDeclared(delegateCmd, dParams, "entity_ref", entityRef)
			setParamIfDeclared(delegateCmd, dParams, "entity_type", entityType)
			requestRecord["entity_ref"] = entityRef
			requestRecord["entity_type"] = entityType
		}
		if analysisIntent := delegationAnalysisIntent(req); analysisIntent != "" {
			setParamIfDeclared(delegateCmd, dParams, "analysis_intent", analysisIntent)
		}
		if commandHasParam(delegateCmd, "question") {
			if q, ok := dParams["question"]; ok {
				dParams["question"] = q
			}
		}
		if commandHasParam(delegateCmd, "db") {
			dParams["db"] = s.runtimeBusinessDBPath(businessID)
		}
		if commandHasParam(delegateCmd, "semantic_pack") {
			dParams["semantic_pack"] = s.businessSemanticPackPath(businessID)
		}
		if commandHasParam(delegateCmd, "context_b64") {
			dParams["context_b64"] = encodeDelegationRuntimeContext(conv, req)
		}
		dArgs, err := delegateCmd.ResolveArgs(dParams, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
		if err != nil {
			envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, err.Error()))
			incrementDelegationSummary(envelope, false, false)
			continue
		}
		dRuntime := resolveManifestRuntime(s.rootDir, m)
		dFullArgs := dRuntime.FullArgs(dArgs, m)
		dResp, err := ch.ExecuteCommand(ctx, dRuntime.Command, dFullArgs, dRuntime.Cwd)
		if err != nil || !dResp.Success || dResp.ExitCode != 0 {
			detail := ""
			if err != nil {
				detail = err.Error()
			} else {
				detail = strings.TrimSpace(firstNonEmptyPipelineString(dResp.Stderr, dResp.Stdout))
			}
			envelope["results"] = append(envelope["results"].([]interface{}), delegationFailureRecord(requestRecord, detail))
			incrementDelegationSummary(envelope, false, false)
			continue
		}
		resultRecord := map[string]interface{}{
			"request_id":          reqID,
			"framework":           resolvedFramework,
			"capability":          semanticCapability,
			"resolved_capability": resolvedCapability,
			"params":              req["params"],
		}
		if semanticCapability == "" {
			resultRecord["capability"] = resolvedCapability
		}
		// Parse delegation output.
		var delegateOutput map[string]interface{}
		if json.Unmarshal([]byte(strings.TrimSpace(dResp.Stdout)), &delegateOutput) == nil {
			resultRecord["artifact_type"] = jsonFirstString(delegateOutput, "artifact_type")
			resultRecord["text"] = jsonFirstString(delegateOutput, "text", "answer", "summary")
			resultRecord["structured"] = delegateOutput["structured"]
			resultRecord["trace"] = delegateOutput["trace"]
			resultRecord["payload"] = delegateOutput
			resultRecord["verified"] = delegationOutputVerified(delegateOutput)
			resultRecord["error"] = delegationOutputError(delegateOutput)
			resultRecord["partial_success"] = delegationOutputPartial(delegateOutput)
			envelope["results"] = append(envelope["results"].([]interface{}), resultRecord)
			incrementDelegationSummary(envelope, delegationOutputVerified(delegateOutput), delegationOutputPartial(delegateOutput))
		} else {
			resultRecord["artifact_type"] = "text/plain"
			resultRecord["text"] = strings.TrimSpace(dResp.Stdout)
			resultRecord["payload"] = dResp.Stdout
			resultRecord["verified"] = true
			resultRecord["partial_success"] = false
			envelope["results"] = append(envelope["results"].([]interface{}), resultRecord)
			incrementDelegationSummary(envelope, true, false)
		}
	}
	return envelope
}

func sortedKeysInterfaceMap(m map[string]interface{}) []string {
	if entries := delegationResultEntries(m); len(entries) > 0 {
		seen := map[string]bool{}
		out := make([]string, 0, len(entries))
		for _, entry := range entries {
			capName := firstNonEmptyPipelineString(jsonFirstString(entry, "capability"), jsonFirstString(entry, "resolved_capability"))
			if capName == "" || seen[capName] {
				continue
			}
			seen[capName] = true
			out = append(out, capName)
		}
		slices.Sort(out)
		return out
	}
	out := make([]string, 0, len(m))
	for k := range m {
		if k == "text" || k == "answer" {
			continue
		}
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func resolveDelegationExecutionTarget(req map[string]interface{}) (string, string) {
	framework := strings.TrimSpace(jsonFirstString(req, "framework"))
	capability := strings.TrimSpace(jsonFirstString(req, "capability"))
	switch capability {
	case "evidence.case_360":
		return "sabio", "data.entity_360"
	case "evidence.portfolio_comparison", "evidence.score_sensitivity", "evidence.counterfactual", "evidence.payment_behavior_summary":
		return "sabio", "data.query.sql"
	case "evidence.claim_audit", "evidence.data_reconciliation":
		return "auditor", "data.quality.audit"
	default:
		return framework, capability
	}
}

func delegationFailureRecord(requestRecord map[string]interface{}, errText string) map[string]interface{} {
	return map[string]interface{}{
		"request_id":          requestRecord["request_id"],
		"framework":           requestRecord["resolved_framework"],
		"capability":          requestRecord["capability"],
		"resolved_capability": requestRecord["resolved_capability"],
		"params":              requestRecord["params"],
		"verified":            false,
		"partial_success":     false,
		"error":               strings.TrimSpace(errText),
		"text":                strings.TrimSpace(errText),
	}
}

func incrementDelegationSummary(envelope map[string]interface{}, verified, partial bool) {
	summary, _ := envelope["summary"].(map[string]interface{})
	if summary == nil {
		return
	}
	switch {
	case verified:
		summary["success_count"] = asInt(summary["success_count"]) + 1
	case partial:
		summary["partial_count"] = asInt(summary["partial_count"]) + 1
	default:
		summary["failure_count"] = asInt(summary["failure_count"]) + 1
	}
}

func asInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func delegationParamStringValue(v interface{}) string {
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return strings.Join(typed, ",")
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := delegationParamStringValue(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		return stringifyDelegationValue(v)
	}
}

func delegationOutputVerified(delegateOutput map[string]interface{}) bool {
	if verified, ok := delegateOutput["verified"].(bool); ok {
		return verified
	}
	if errText := delegationOutputError(delegateOutput); strings.TrimSpace(errText) != "" {
		return false
	}
	return true
}

func delegationOutputError(delegateOutput map[string]interface{}) string {
	errText := firstNonEmptyPipelineString(
		jsonFirstString(delegateOutput, "error"),
	)
	if errText != "" {
		return errText
	}
	if trace, ok := delegateOutput["trace"].(map[string]interface{}); ok {
		return jsonFirstString(trace, "error")
	}
	return ""
}

func delegationOutputPartial(delegateOutput map[string]interface{}) bool {
	if partial, ok := delegateOutput["partial_success"].(bool); ok {
		return partial
	}
	if verified, ok := delegateOutput["verified"].(bool); ok {
		return !verified && strings.TrimSpace(jsonFirstString(delegateOutput, "text", "summary", "answer")) != ""
	}
	return false
}

func delegationResultEntries(envelope map[string]interface{}) []map[string]interface{} {
	raw, ok := envelope["results"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func (s *server) generateOwnerFollowupWithLLM(ctx context.Context, conv *Conversation, m *manifest.Manifest, userAnswer string, params map[string]string, phase string) string {
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
	system := "Eres Radar, owner conversacional de un tramo analítico. No ejecutes acciones operativas ni prometas side effects. Trabaja en dos fases: en phase=plan interpreta intención y decide evidencia/delegaciones; en phase=synthesis integra delegation_results_json o delegation_results_path y responde grounded. Devuelve solo JSON válido. Para phase=plan usa campos: analysis_intent, needs_delegation, delegation_requests, evidence_needed, reason, text. Contrato estricto de delegation_requests: cada item debe tener framework, capability, params y reason. No planifiques en capabilities técnicas; planifica en contracts de evidencia. Solo puedes usar estas capabilities semánticas: evidence.case_360, evidence.portfolio_comparison, evidence.score_sensitivity, evidence.counterfactual, evidence.payment_behavior_summary, evidence.claim_audit, evidence.data_reconciliation. Usa params estructurados como entity_ref, entity_type, analysis_intent, metrics, peer_strategy y question cuando ayude; no uses type, delegation_type, task ni esquemas libres. Si no necesitas delegación, devuelve needs_delegation=false y delegation_requests=[]. Para phase=synthesis usa campos: text, findings, evidence, confidence, data_gaps, residual_risks, next_best_question, recommendation. delegation_results_json o delegation_results_path contienen results por delegate; debes integrar evidencia estructurada, distinguir éxito parcial vs fallo parcial y decir explícitamente qué no pudiste verificar."
	user := fmt.Sprintf("phase=%s\nConversación: %s\nPregunta del usuario: %s\nContexto/artifacts:\n%s", phase, conv.ID, userAnswer, strings.Join(contextParts, "\n\n---\n\n"))
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

func (s *server) concludeAnalysisTransition(
	ctx context.Context,
	ch *adapter.Client,
	conv *Conversation,
	manifests map[string]*manifest.Manifest,
	session *activeSessionInfo,
	userTrigger string,
	simulated bool,
) analysisClosureResult {
	businessID := conversationBusinessID(conv)
	if businessID == "" || session == nil {
		return analysisClosureResult{}
	}
	decision := s.evaluateAnalysisReadiness(businessID, conv.ID, simulated)
	if simulated {
		s.concludeSessionOnDisk(session.Path, "simulation_complete: "+truncate(userTrigger, 120))
		path := s.persistAnalysisSimulationPreview(businessID, conv.ID, session, decision, userTrigger)
		return analysisClosureResult{
			State:        "review_pending",
			Reason:       decision.Reason,
			ArtifactType: "analysis.simulation.preview.v1",
			ArtifactPath: path,
		}
	}
	if decision.ReadyForOperation {
		s.concludeSessionOnDisk(session.Path, "user_operational: "+truncate(userTrigger, 120))
		s.persistAnalysisHandoff(ctx, ch, conv, manifests, session, userTrigger)
		return analysisClosureResult{
			State:             "handoff_operational",
			Reason:            decision.Reason,
			ReadyForOperation: true,
			ArtifactType:      "analysis.handoff.v1",
			ArtifactPath:      s.latestFlowArtifactPath(businessID, "analysis.handoff.v1"),
		}
	}
	s.concludeSessionOnDisk(session.Path, "review_pending: "+truncate(userTrigger, 120))
	path := s.persistAnalysisReviewPending(businessID, conv.ID, session, decision, userTrigger)
	return analysisClosureResult{
		State:        "review_pending",
		Reason:       decision.Reason,
		ArtifactType: "analysis.review_pending.v1",
		ArtifactPath: path,
	}
}

func (s *server) persistAnalysisReadinessArtifact(businessID, conversationID string, session *activeSessionInfo, stdout string) string {
	if strings.TrimSpace(businessID) == "" || session == nil {
		return ""
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload) != nil {
		return ""
	}
	decision := analysisReadinessDecisionFromPayload("analysis.followup.v1", "", payload)
	readiness := map[string]interface{}{
		"artifact_type":       "analysis.readiness.v1",
		"business_id":         businessID,
		"conversation_id":     conversationID,
		"state":               decision.State,
		"ready_for_operation": decision.ReadyForOperation,
		"reason":              decision.Reason,
		"confidence":          decision.Confidence,
		"source_artifact":     decision.SourceArtifact,
		"source_path":         decision.SourcePath,
		"analytical_summary":  decision.Summary,
		"recommendation":      decision.Recommendation,
		"data_gaps":           decision.DataGaps,
		"residual_risks":      decision.ResidualRisks,
		"owner_framework":     session.Framework,
		"owner_capability":    session.Capability,
		"turns_completed":     session.TurnCount + 1,
		"simulation":          false,
		"evaluated_at":        time.Now().UTC().Format(time.RFC3339Nano),
	}
	runID := "readiness_" + businessID
	return s.persistFlowArtifact(runID, "analysis_readiness", "analysis.readiness.v1", readiness)
}

func (s *server) evaluateAnalysisReadiness(businessID, conversationID string, allowSimulation bool) analysisReadinessDecision {
	if decision := s.latestAnalysisReadinessDecision(businessID, conversationID, allowSimulation); decision.SourceArtifact != "" {
		return s.enrichAnalysisDecisionFromHistory(businessID, conversationID, allowSimulation, decision)
	}
	if sourceType, sourcePath, payload := s.latestAnalysisSourcePayload(businessID, conversationID, allowSimulation); len(payload) > 0 {
		return s.enrichAnalysisDecisionFromHistory(businessID, conversationID, allowSimulation, analysisReadinessDecisionFromPayload(sourceType, sourcePath, payload))
	}
	return analysisReadinessDecision{
		State:      "review_pending",
		Reason:     "no hay artefacto analítico elegible para autorizar transición operacional",
		Confidence: "partial",
	}
}

func (s *server) enrichAnalysisDecisionFromHistory(businessID, conversationID string, allowSimulation bool, decision analysisReadinessDecision) analysisReadinessDecision {
	for _, path := range s.allFlowArtifactPaths(businessID, "analysis.followup.v1") {
		payload, ok := flowArtifactPayloadFromPath(path)
		if !ok {
			continue
		}
		if payloadConversation := jsonFirstString(payload, "conversation_id"); payloadConversation != "" && conversationID != "" && payloadConversation != conversationID {
			continue
		}
		if !allowSimulation && jsonFirstBool(payload, "simulation") {
			continue
		}
		if summary := jsonFirst(payload, "text", "summary", "analysis"); summary != "" {
			decision.Summary = summary
		}
		if recommendation := jsonFirst(payload, "recommendation", "next_action", "suggested_action"); recommendation != "" {
			decision.Recommendation = recommendation
		}
		if confidence := jsonFirst(payload, "confidence"); confidence != "" {
			decision.Confidence = confidence
		}
		for _, gap := range jsonStringSlice(payload, "data_gaps", "gaps", "blocking_gaps", "accepted_gaps") {
			if !containsString(decision.DataGaps, gap) {
				decision.DataGaps = append(decision.DataGaps, gap)
			}
		}
		for _, risk := range jsonStringSlice(payload, "residual_risks", "risks", "risk_factors") {
			if !containsString(decision.ResidualRisks, risk) {
				decision.ResidualRisks = append(decision.ResidualRisks, risk)
			}
		}
	}
	if path := s.latestFlowArtifactPath(businessID, "analysis.case_review.v1"); path != "" {
		if payload, ok := flowArtifactPayloadFromPath(path); ok {
			if decision.Summary == "" {
				decision.Summary = jsonFirst(payload, "text", "summary", "analysis")
			}
			if decision.Recommendation == "" {
				decision.Recommendation = jsonFirst(payload, "recommendation", "next_action", "suggested_action")
			}
			for _, gap := range jsonStringSlice(payload, "data_gaps", "gaps", "blocking_gaps", "accepted_gaps") {
				if !containsString(decision.DataGaps, gap) {
					decision.DataGaps = append(decision.DataGaps, gap)
				}
			}
			for _, risk := range jsonStringSlice(payload, "residual_risks", "risks", "risk_factors") {
				if !containsString(decision.ResidualRisks, risk) {
					decision.ResidualRisks = append(decision.ResidualRisks, risk)
				}
			}
		}
	}
	return decision
}

func (s *server) latestAnalysisReadinessDecision(businessID, conversationID string, allowSimulation bool) analysisReadinessDecision {
	paths := s.allFlowArtifactPaths(businessID, "analysis.readiness.v1")
	for i := len(paths) - 1; i >= 0; i-- {
		payload, ok := flowArtifactPayloadFromPath(paths[i])
		if !ok {
			continue
		}
		if payloadConversation := jsonFirstString(payload, "conversation_id"); payloadConversation != "" && conversationID != "" && payloadConversation != conversationID {
			continue
		}
		if !allowSimulation && jsonFirstBool(payload, "simulation") {
			continue
		}
		decision := analysisReadinessDecisionFromPayload("analysis.readiness.v1", paths[i], payload)
		if decision.SourceArtifact != "" {
			return decision
		}
	}
	return analysisReadinessDecision{}
}

func (s *server) latestAnalysisSourcePayload(businessID, conversationID string, allowSimulation bool) (string, string, map[string]interface{}) {
	paths := s.allFlowArtifactPaths(businessID, "analysis.followup.v1")
	for i := len(paths) - 1; i >= 0; i-- {
		payload, ok := flowArtifactPayloadFromPath(paths[i])
		if !ok {
			continue
		}
		if payloadConversation := jsonFirstString(payload, "conversation_id"); payloadConversation != "" && conversationID != "" && payloadConversation != conversationID {
			continue
		}
		if !allowSimulation && jsonFirstBool(payload, "simulation") {
			continue
		}
		return "analysis.followup.v1", paths[i], payload
	}
	if path := s.latestFlowArtifactPath(businessID, "analysis.case_review.v1"); path != "" {
		if payload, ok := flowArtifactPayloadFromPath(path); ok {
			return "analysis.case_review.v1", path, payload
		}
	}
	return "", "", nil
}

func analysisReadinessDecisionFromPayload(sourceType, sourcePath string, payload map[string]interface{}) analysisReadinessDecision {
	decision := analysisReadinessDecision{
		State:          "review_pending",
		Reason:         "falta señal explícita ready_for_operation=true",
		Confidence:     firstNonEmptyPipelineString(jsonFirst(payload, "confidence"), "partial"),
		SourceArtifact: sourceType,
		SourcePath:     sourcePath,
		Summary:        firstNonEmptyPipelineString(jsonFirst(payload, "text", "summary", "analysis"), jsonFirst(payload, "analytical_summary")),
		Recommendation: jsonFirst(payload, "recommendation", "next_action", "suggested_action"),
		DataGaps:       jsonStringSlice(payload, "data_gaps", "gaps", "blocking_gaps", "accepted_gaps"),
		ResidualRisks:  jsonStringSlice(payload, "residual_risks", "risks", "risk_factors"),
		Simulation:     jsonFirstBool(payload, "simulation"),
	}
	if jsonFirstBool(payload, "ready_for_operation", "analysis_sufficient", "operational_ready") {
		decision.State = "ready_for_operation"
		decision.Reason = "señal explícita ready_for_operation=true"
		decision.ReadyForOperation = true
		return decision
	}
	switch sourceType {
	case "analysis.readiness.v1":
		decision.State = firstNonEmptyPipelineString(jsonFirst(payload, "state"), decision.State)
		decision.Reason = firstNonEmptyPipelineString(jsonFirst(payload, "reason"), decision.Reason)
		decision.ReadyForOperation = jsonFirstBool(payload, "ready_for_operation")
		decision.Summary = firstNonEmptyPipelineString(jsonFirst(payload, "analytical_summary"), decision.Summary)
		decision.Recommendation = firstNonEmptyPipelineString(jsonFirst(payload, "recommendation"), decision.Recommendation)
	case "analysis.case_review.v1":
		decision.Reason = "hay case review, pero no existe gate explícito de suficiencia para operar"
	case "analysis.followup.v1":
		decision.Reason = "el followup más reciente no declaró ready_for_operation=true"
	}
	return decision
}

func flowArtifactPayloadFromPath(path string) (map[string]interface{}, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return nil, false
	}
	return payload, true
}

func (s *server) persistAnalysisReviewPending(businessID, conversationID string, session *activeSessionInfo, decision analysisReadinessDecision, userTrigger string) string {
	payload := map[string]interface{}{
		"artifact_type":       "analysis.review_pending.v1",
		"business_id":         businessID,
		"conversation_id":     conversationID,
		"state":               "review_pending",
		"ready_for_operation": false,
		"reason":              decision.Reason,
		"user_trigger":        userTrigger,
		"turns_completed":     session.TurnCount,
		"analytical_summary":  decision.Summary,
		"recommendation":      decision.Recommendation,
		"data_gaps":           decision.DataGaps,
		"residual_risks":      decision.ResidualRisks,
		"confidence":          decision.Confidence,
		"source_artifact":     decision.SourceArtifact,
		"source_path":         decision.SourcePath,
		"from": map[string]string{
			"framework":  session.Framework,
			"capability": session.Capability,
		},
		"to": map[string]string{
			"framework": "foco",
			"role":      "case_manager",
		},
		"created_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	runID := "review_pending_" + businessID
	return s.persistFlowArtifact(runID, "analysis_review_pending", "analysis.review_pending.v1", payload)
}

func (s *server) persistAnalysisSimulationPreview(businessID, conversationID string, session *activeSessionInfo, decision analysisReadinessDecision, userTrigger string) string {
	payload := map[string]interface{}{
		"artifact_type":       "analysis.simulation.preview.v1",
		"business_id":         businessID,
		"conversation_id":     conversationID,
		"state":               "review_pending",
		"ready_for_operation": false,
		"authoritative":       false,
		"preview_only":        true,
		"reason":              firstNonEmptyPipelineString(decision.Reason, "la simulación no autoriza handoff operativo"),
		"user_trigger":        userTrigger,
		"turns_completed":     session.TurnCount,
		"analytical_summary":  decision.Summary,
		"recommendation":      decision.Recommendation,
		"data_gaps":           decision.DataGaps,
		"residual_risks":      decision.ResidualRisks,
		"confidence":          decision.Confidence,
		"source_artifact":     decision.SourceArtifact,
		"source_path":         decision.SourcePath,
		"from": map[string]string{
			"framework":  session.Framework,
			"capability": session.Capability,
		},
		"to": map[string]string{
			"framework": "foco",
			"role":      "case_manager",
		},
		"created_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	runID := "simulation_preview_" + businessID
	return s.persistFlowArtifact(runID, "analysis_simulation_preview", "analysis.simulation.preview.v1", payload)
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

	// Aggregate findings from ALL followup artifacts across the analytical
	// conversation. The handoff must carry accumulated evidence, not just the
	// latest turn's output. Latest text/recommendation win; gaps/risks/findings
	// are unioned.
	for _, path := range s.allFlowArtifactPaths(businessID, "analysis.followup.v1") {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var followup map[string]interface{}
		if json.Unmarshal(raw, &followup) != nil {
			continue
		}
		if text := jsonFirst(followup, "text"); text != "" {
			analysisSummary = text
		}
		if r := jsonFirst(followup, "recommendation"); r != "" {
			recommendation = r
		}
		for _, risk := range jsonStringSlice(followup, "residual_risks", "risks", "risk_factors") {
			if !containsString(residualRisks, risk) {
				residualRisks = append(residualRisks, risk)
			}
		}
		for _, gap := range jsonStringSlice(followup, "data_gaps", "gaps", "blocking_gaps") {
			if !containsString(acceptedGaps, gap) {
				acceptedGaps = append(acceptedGaps, gap)
			}
		}
		if c := jsonFirst(followup, "confidence"); c != "" {
			confidence = c
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

func followupAnalysisPhase(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return ""
	}
	var payload map[string]interface{}
	if json.Unmarshal([]byte(stdout), &payload) != nil {
		return ""
	}
	return strings.TrimSpace(jsonFirstString(payload, "analysis_phase"))
}

func synthesizeFollowupRuntimeFailureArtifact(businessID string, turnCount int, execution sessionFollowupExecution, userAnswer string) string {
	msg := "Radar obtuvo evidencia delegada, pero el second pass no logró sintetizarla de forma confiable."
	if strings.TrimSpace(execution.SynthesisError) != "" {
		msg = msg + " " + execution.SynthesisError
	}
	dataGaps := []string{"second pass sin synthesis verificable"}
	if len(execution.DelegatedCapabilities) > 0 {
		dataGaps = append(dataGaps, "delegaciones involucradas: "+strings.Join(execution.DelegatedCapabilities, ", "))
	}
	payload := map[string]interface{}{
		"artifact_type":          "analysis.followup.v1",
		"business_id":            businessID,
		"turn_count":             turnCount,
		"user_input":             userAnswer,
		"analysis_phase":         "synthesis_error",
		"synthesized":            false,
		"synthesis_attempted":    execution.SynthesisAttempted,
		"synthesis_error":        execution.SynthesisError,
		"delegated_capabilities": execution.DelegatedCapabilities,
		"text":                   msg,
		"confidence":             "low",
		"data_gaps":              dataGaps,
		"grounded_answer": map[string]interface{}{
			"artifact_type": "answer.grounded.v1",
			"text":          msg,
		},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
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

func jsonFirstBool(m map[string]interface{}, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k].(bool); ok {
			return v
		}
	}
	return false
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
// command, normalizes delegation_requests to the executable contract and
// returns the user-facing text plus any executable delegation_requests.
func extractFollowupTextAndDelegations(stdout string, userAnswer string) (string, []map[string]interface{}) {
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
		delegations := normalizeFollowupDelegationRequests(payload, userAnswer)
		if strings.EqualFold(strings.TrimSpace(jsonFirstString(payload, "analysis_phase")), "plan") && len(delegations) == 0 {
			needsDelegation, _ := payload["needs_delegation"].(bool)
			if needsDelegation {
				text = fallbackNonExecutableDelegationText(payload, userAnswer)
			} else if strings.TrimSpace(text) == "" {
				text = "Radar no necesitó delegación adicional para responder este follow-up con la evidencia ya disponible."
			}
		}
		if text != "" {
			return text, delegations
		}
	}
	return stdout, nil
}

func normalizeFollowupDelegationRequests(payload map[string]interface{}, userAnswer string) []map[string]interface{} {
	rawDelegations := rawDelegationRequestsFromPayload(payload)
	intent := strings.ToLower(strings.TrimSpace(jsonFirstString(payload, "analysis_intent")))
	baseReason := jsonFirstString(payload, "reason")
	out := make([]map[string]interface{}, 0, len(rawDelegations))
	for _, req := range rawDelegations {
		if normalized := normalizeDelegationRequest(req, intent, userAnswer, baseReason); len(normalized) > 0 {
			out = append(out, normalized)
		}
	}
	if len(out) == 0 {
		if fallback := inferDelegationRequestsFromIntent(intent, userAnswer, baseReason); len(fallback) > 0 {
			out = append(out, fallback...)
		}
	}
	return completeDelegationRequestsForIntent(intent, userAnswer, baseReason, out)
}

func rawDelegationRequestsFromPayload(payload map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	if raw, ok := payload["delegation_requests"].([]interface{}); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
	}
	if len(out) == 0 {
		if single, ok := payload["delegation_request"].(map[string]interface{}); ok {
			out = append(out, single)
		}
	}
	return out
}

func normalizeDelegationRequest(req map[string]interface{}, analysisIntent, userAnswer, baseReason string) map[string]interface{} {
	if len(req) == 0 {
		return nil
	}
	framework := strings.ToLower(strings.TrimSpace(jsonFirstString(req, "framework")))
	capability := strings.TrimSpace(jsonFirstString(req, "capability"))
	reason := firstNonEmptyPipelineString(jsonFirstString(req, "reason", "why"), baseReason, defaultDelegationReason(analysisIntent, userAnswer))
	question := delegationQuestionFromMap(req)
	kind := strings.ToLower(strings.TrimSpace(jsonFirstString(req, "type", "delegation_type", "task", "intent")))
	if isAllowedAnalyticalCapability(capability) {
		framework = canonicalFrameworkForAnalyticalCapability(framework, capability)
	} else if !isAllowedAnalyticalDelegation(framework, capability) {
		switch {
		case strings.Contains(kind, "similar_customers"), strings.Contains(kind, "portfolio"), strings.Contains(kind, "compar"), strings.Contains(kind, "benchmark"):
			framework = "sabio"
			capability = "evidence.portfolio_comparison"
		case strings.Contains(kind, "entity_360"), strings.Contains(kind, "obtener_detalles_de_caso"), strings.Contains(kind, "case_details"), strings.Contains(kind, "detalles"):
			framework = "sabio"
			capability = "evidence.case_360"
		case strings.Contains(kind, "simulation"), strings.Contains(kind, "sensibilidad"), strings.Contains(kind, "contrafactual"), strings.Contains(kind, "scenario"):
			framework = "sabio"
			if analysisIntent == "counterfactual_scenario" || strings.Contains(kind, "contrafactual") {
				capability = "evidence.counterfactual"
			} else {
				capability = "evidence.score_sensitivity"
			}
		case strings.Contains(kind, "audit"), strings.Contains(kind, "quality"), strings.Contains(kind, "calidad"), strings.Contains(kind, "contradic"), strings.Contains(kind, "gap"):
			framework = "auditor"
			capability = "evidence.data_reconciliation"
		case hasDelegationAuditSignals(req):
			framework = "auditor"
			capability = "evidence.claim_audit"
		case hasEntity360Signals(req):
			framework = "sabio"
			capability = "evidence.case_360"
		case analysisIntent == "portfolio_comparison":
			framework = "sabio"
			capability = "evidence.portfolio_comparison"
		case analysisIntent == "score_sensitivity":
			framework = "sabio"
			capability = "evidence.score_sensitivity"
		case analysisIntent == "counterfactual_scenario":
			framework = "sabio"
			capability = "evidence.counterfactual"
		case analysisIntent == "data_reconciliation":
			framework = "auditor"
			capability = "evidence.data_reconciliation"
		case analysisIntent == "alternative_hypothesis":
			framework = "sabio"
			capability = "evidence.case_360"
		}
	}
	if !isAllowedAnalyticalDelegation(framework, capability) {
		return nil
	}
	if strings.TrimSpace(question) == "" {
		question = defaultDelegationQuestion(capability, analysisIntent, userAnswer, req)
	}
	if strings.TrimSpace(question) == "" {
		return nil
	}
	entityRef := firstNonEmptyPipelineString(
		jsonFirstString(req, "deudor_id", "entity_id", "entity_ref", "customer_id", "client_id", "id"),
	)
	entityType := canonicalDelegationEntityType(jsonFirstString(req, "entity_type"))
	params := map[string]interface{}{
		"question": question,
	}
	if strings.TrimSpace(analysisIntent) != "" {
		params["analysis_intent"] = analysisIntent
	}
	if entityRef != "" {
		params["entity_ref"] = entityRef
	}
	if entityType != "" {
		params["entity_type"] = entityType
	}
	for k, v := range evidenceContractDefaultParams(capability, analysisIntent, userAnswer, req) {
		if _, exists := params[k]; !exists {
			params[k] = v
		}
	}
	return map[string]interface{}{
		"framework":  framework,
		"capability": capability,
		"params":     params,
		"reason":     reason,
	}
}

func delegationQuestionFromMap(req map[string]interface{}) string {
	if p, ok := req["params"].(map[string]interface{}); ok {
		if q := jsonFirstString(p, "question"); strings.TrimSpace(q) != "" {
			return strings.TrimSpace(q)
		}
	}
	return firstNonEmptyPipelineString(
		jsonFirstString(req, "question", "prompt", "query"),
	)
}

func isAllowedAnalyticalDelegation(framework, capability string) bool {
	return framework == canonicalFrameworkForAnalyticalCapability(framework, capability) && isAllowedAnalyticalCapability(capability)
}

func isAllowedAnalyticalCapability(capability string) bool {
	switch {
	case capability == "evidence.case_360":
		return true
	case capability == "evidence.portfolio_comparison":
		return true
	case capability == "evidence.score_sensitivity":
		return true
	case capability == "evidence.counterfactual":
		return true
	case capability == "evidence.payment_behavior_summary":
		return true
	case capability == "evidence.claim_audit":
		return true
	case capability == "evidence.data_reconciliation":
		return true
	default:
		return false
	}
}

func canonicalFrameworkForAnalyticalCapability(framework, capability string) string {
	switch strings.TrimSpace(capability) {
	case "evidence.case_360", "evidence.portfolio_comparison", "evidence.score_sensitivity", "evidence.counterfactual", "evidence.payment_behavior_summary":
		return "sabio"
	case "evidence.claim_audit", "evidence.data_reconciliation":
		return "auditor"
	default:
		return strings.ToLower(strings.TrimSpace(framework))
	}
}

func defaultDelegationReason(analysisIntent, userAnswer string) string {
	switch analysisIntent {
	case "portfolio_comparison":
		return "comparar el caso contra la cartera requiere evidencia cuantitativa"
	case "data_reconciliation":
		return "validar contradicciones o gaps requiere auditoría contextual"
	case "score_sensitivity":
		return "calcular sensibilidad del score requiere evidencia cuantitativa del ranking"
	case "counterfactual_scenario":
		return "evaluar un contrafactual requiere métricas comparables de cartera"
	case "alternative_hypothesis":
		return "contrastar hipótesis alternativas requiere evidencia del caso"
	default:
		if strings.TrimSpace(userAnswer) != "" {
			return "profundizar el follow-up requiere evidencia auxiliar verificable"
		}
		return "se requiere evidencia auxiliar verificable"
	}
}

func defaultDelegationQuestion(capability, analysisIntent, userAnswer string, req map[string]interface{}) string {
	entityRef := firstNonEmptyPipelineString(
		jsonFirstString(req, "deudor_id", "entity_id", "entity_ref", "customer_id", "client_id", "id"),
	)
	fields := stringListFromUnknown(req["fields"])
	criteria := stringifyDelegationValue(req["criteria"])
	switch capability {
	case "evidence.case_360":
		base := "Obtén vista 360 del caso"
		if entityRef != "" {
			base = fmt.Sprintf("Obtén vista 360 del cliente %s", entityRef)
		}
		if len(fields) > 0 {
			return fmt.Sprintf("%s incluyendo %s.", base, strings.Join(fields, ", "))
		}
		if strings.TrimSpace(userAnswer) != "" {
			return fmt.Sprintf("%s para responder: %s", base, strings.TrimSpace(userAnswer))
		}
		return base + "."
	case "evidence.claim_audit", "evidence.data_reconciliation":
		base := "Audita contradicciones, gaps y calidad de datos del caso"
		if entityRef != "" {
			base = fmt.Sprintf("Audita contradicciones, gaps y calidad de datos del cliente %s", entityRef)
		}
		if strings.TrimSpace(userAnswer) != "" {
			return fmt.Sprintf("%s para responder: %s", base, strings.TrimSpace(userAnswer))
		}
		return base + "."
	case "evidence.portfolio_comparison":
		base := "Compara el caso contra la cartera: saldo abierto, mora, comportamiento relativo, percentiles y clientes similares"
		if entityRef != "" {
			base = fmt.Sprintf("Compara el cliente %s contra la cartera: saldo abierto, mora, comportamiento relativo, percentiles y clientes similares", entityRef)
		}
		if criteria != "" {
			base += ". Criterios: " + criteria
		}
		return base + "."
	case "evidence.score_sensitivity":
		base := "Calcula sensibilidad del score del caso"
		if entityRef != "" {
			base = fmt.Sprintf("Calcula sensibilidad del score del cliente %s", entityRef)
		}
		if strings.TrimSpace(userAnswer) != "" {
			base += ". Pregunta: " + strings.TrimSpace(userAnswer)
		}
		return base + "."
	case "evidence.counterfactual":
		base := "Evalúa escenarios contrafactuales del caso"
		if entityRef != "" {
			base = fmt.Sprintf("Evalúa escenarios contrafactuales del cliente %s", entityRef)
		}
		if strings.TrimSpace(userAnswer) != "" {
			base += ". Pregunta: " + strings.TrimSpace(userAnswer)
		}
		return base + "."
	case "evidence.payment_behavior_summary":
		base := "Resume el comportamiento de pago del caso"
		if entityRef != "" {
			base = fmt.Sprintf("Resume el comportamiento de pago del cliente %s", entityRef)
		}
		return base + "."
	case "data.query.sql":
		switch analysisIntent {
		case "portfolio_comparison":
			base := "Compara el caso contra la cartera: saldo abierto, mora, comportamiento relativo, percentiles y clientes similares"
			if entityRef != "" {
				base = fmt.Sprintf("Compara el cliente %s contra la cartera: saldo abierto, mora, comportamiento relativo, percentiles y clientes similares", entityRef)
			}
			if criteria != "" {
				base += ". Criterios: " + criteria
			}
			return base + "."
		case "score_sensitivity", "counterfactual_scenario":
			base := "Calcula sensibilidad del score y escenarios contrafactuales del caso"
			if entityRef != "" {
				base = fmt.Sprintf("Calcula sensibilidad del score y escenarios contrafactuales del cliente %s", entityRef)
			}
			if criteria != "" {
				base += ". Criterios: " + criteria
			}
			if strings.TrimSpace(userAnswer) != "" {
				base += ". Pregunta: " + strings.TrimSpace(userAnswer)
			}
			return base + "."
		default:
			base := "Consulta SQL para profundizar el caso con métricas verificables"
			if entityRef != "" {
				base = fmt.Sprintf("Consulta SQL para profundizar el cliente %s con métricas verificables", entityRef)
			}
			if criteria != "" {
				base += ". Criterios: " + criteria
			}
			if strings.TrimSpace(userAnswer) != "" {
				base += ". Pregunta: " + strings.TrimSpace(userAnswer)
			}
			return base + "."
		}
	}
	return ""
}

func evidenceContractDefaultParams(capability, analysisIntent, userAnswer string, req map[string]interface{}) map[string]interface{} {
	switch capability {
	case "evidence.portfolio_comparison":
		return map[string]interface{}{
			"metrics":       []interface{}{"open_amount", "days_past_due", "payment_behavior"},
			"peer_strategy": "similar_clients",
		}
	case "evidence.score_sensitivity":
		return map[string]interface{}{
			"metrics": []interface{}{"materiality", "payment_behavior", "legal_risk"},
		}
	case "evidence.payment_behavior_summary":
		return map[string]interface{}{
			"metrics": []interface{}{"payment_count", "payment_total", "payment_residue_total"},
		}
	default:
		return nil
	}
}

func inferDelegationRequestsFromIntent(analysisIntent, userAnswer, baseReason string) []map[string]interface{} {
	analysisIntent = strings.TrimSpace(strings.ToLower(analysisIntent))
	switch analysisIntent {
	case "portfolio_comparison":
		return []map[string]interface{}{
			{
				"framework":  "sabio",
				"capability": "evidence.case_360",
				"params": map[string]interface{}{
					"question":        defaultDelegationQuestion("evidence.case_360", "case_baseline", userAnswer, nil),
					"analysis_intent": "case_baseline",
				},
				"reason": "establecer la línea base verificable del caso antes de comparar cartera",
			},
			{
				"framework":  "sabio",
				"capability": "evidence.portfolio_comparison",
				"params": map[string]interface{}{
					"question":        defaultDelegationQuestion("evidence.portfolio_comparison", analysisIntent, userAnswer, nil),
					"analysis_intent": analysisIntent,
					"metrics":         []interface{}{"open_amount", "days_past_due", "payment_behavior"},
					"peer_strategy":   "similar_clients",
				},
				"reason": firstNonEmptyPipelineString(baseReason, defaultDelegationReason(analysisIntent, userAnswer)),
			}}
	case "data_reconciliation":
		return []map[string]interface{}{{
			"framework":  "auditor",
			"capability": "evidence.data_reconciliation",
			"params": map[string]interface{}{
				"question":        defaultDelegationQuestion("evidence.data_reconciliation", analysisIntent, userAnswer, nil),
				"analysis_intent": analysisIntent,
			},
			"reason": firstNonEmptyPipelineString(baseReason, defaultDelegationReason(analysisIntent, userAnswer)),
		}}
	case "score_sensitivity":
		return []map[string]interface{}{{
			"framework":  "sabio",
			"capability": "evidence.score_sensitivity",
			"params": map[string]interface{}{
				"question":        defaultDelegationQuestion("evidence.score_sensitivity", analysisIntent, userAnswer, nil),
				"analysis_intent": analysisIntent,
				"metrics":         []interface{}{"materiality", "payment_behavior", "legal_risk"},
			},
			"reason": firstNonEmptyPipelineString(baseReason, defaultDelegationReason(analysisIntent, userAnswer)),
		}}
	case "counterfactual_scenario":
		return []map[string]interface{}{{
			"framework":  "sabio",
			"capability": "evidence.counterfactual",
			"params": map[string]interface{}{
				"question":        defaultDelegationQuestion("evidence.counterfactual", analysisIntent, userAnswer, nil),
				"analysis_intent": analysisIntent,
			},
			"reason": firstNonEmptyPipelineString(baseReason, defaultDelegationReason(analysisIntent, userAnswer)),
		}}
	case "alternative_hypothesis":
		return []map[string]interface{}{
			{
				"framework":  "sabio",
				"capability": "evidence.case_360",
				"params": map[string]interface{}{
					"question":        defaultDelegationQuestion("evidence.case_360", analysisIntent, userAnswer, nil),
					"analysis_intent": analysisIntent,
				},
				"reason": firstNonEmptyPipelineString(baseReason, defaultDelegationReason(analysisIntent, userAnswer)),
			},
			{
				"framework":  "auditor",
				"capability": "evidence.claim_audit",
				"params": map[string]interface{}{
					"question":        defaultDelegationQuestion("evidence.claim_audit", analysisIntent, userAnswer, nil),
					"analysis_intent": analysisIntent,
				},
				"reason": "validar si la lectura del caso está contaminada por histórico pagado o gaps de interpretación",
			},
		}
	default:
		return nil
	}
}

func completeDelegationRequestsForIntent(analysisIntent, userAnswer, baseReason string, current []map[string]interface{}) []map[string]interface{} {
	analysisIntent = strings.TrimSpace(strings.ToLower(analysisIntent))
	out := append([]map[string]interface{}{}, current...)
	required := requiredDelegationCapabilitiesForIntent(analysisIntent)
	if len(required) == 0 {
		return dedupeDelegationRequests(out)
	}
	existing := map[string]bool{}
	for _, req := range out {
		if capName := strings.TrimSpace(jsonFirstString(req, "capability")); capName != "" {
			existing[capName] = true
		}
	}
	for _, req := range inferDelegationRequestsFromIntent(analysisIntent, userAnswer, baseReason) {
		capName := strings.TrimSpace(jsonFirstString(req, "capability"))
		if capName == "" || existing[capName] || !containsString(required, capName) {
			continue
		}
		out = append(out, req)
		existing[capName] = true
	}
	return dedupeDelegationRequests(out)
}

func requiredDelegationCapabilitiesForIntent(analysisIntent string) []string {
	switch strings.TrimSpace(strings.ToLower(analysisIntent)) {
	case "portfolio_comparison":
		return []string{"evidence.portfolio_comparison"}
	default:
		return nil
	}
}

func dedupeDelegationRequests(in []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(in))
	seen := map[string]bool{}
	for _, req := range in {
		if len(req) == 0 {
			continue
		}
		framework := strings.ToLower(strings.TrimSpace(jsonFirstString(req, "framework")))
		capability := strings.TrimSpace(jsonFirstString(req, "capability"))
		question := ""
		if p, ok := req["params"].(map[string]interface{}); ok {
			question = strings.TrimSpace(jsonFirstString(p, "question"))
		}
		key := framework + "|" + capability + "|" + question
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, req)
	}
	return out
}

func fallbackNonExecutableDelegationText(payload map[string]interface{}, userAnswer string) string {
	reason := firstNonEmptyPipelineString(jsonFirstString(payload, "reason"), defaultDelegationReason(strings.ToLower(strings.TrimSpace(jsonFirstString(payload, "analysis_intent"))), userAnswer))
	if strings.TrimSpace(reason) == "" {
		reason = "el plan no produjo delegaciones ejecutables"
	}
	if strings.TrimSpace(userAnswer) == "" {
		return "Radar quiso pedir evidencia auxiliar, pero el plan no produjo delegaciones ejecutables."
	}
	return fmt.Sprintf("Radar quiso pedir evidencia auxiliar para responder '%s', pero el plan no produjo delegaciones ejecutables. Motivo declarado: %s.", strings.TrimSpace(userAnswer), reason)
}

func delegationEntityIdentity(req map[string]interface{}, conv *Conversation) (string, string) {
	entityRef := firstNonEmptyPipelineString(
		jsonFirstString(req, "entity_ref", "entity_id", "deudor_id", "customer_id", "client_id", "id"),
	)
	entityType := canonicalDelegationEntityType(jsonFirstString(req, "entity_type"))
	if p, ok := req["params"].(map[string]interface{}); ok {
		entityRef = firstNonEmptyPipelineString(entityRef, jsonFirstString(p, "entity_ref", "entity_id"))
		entityType = canonicalDelegationEntityType(firstNonEmptyPipelineString(entityType, jsonFirstString(p, "entity_type")))
	}
	if entityRef == "" && conv != nil {
		if active, ok := conv.RuntimeContext["active_entity"].(map[string]interface{}); ok {
			entityRef = firstNonEmptyPipelineString(entityRef, jsonFirstString(active, "id", "entity_ref", "ref", "code"))
			entityType = canonicalDelegationEntityType(firstNonEmptyPipelineString(entityType, jsonFirstString(active, "type", "entity_type", "kind")))
		}
	}
	if entityType == "" {
		entityType = "client"
	}
	return entityRef, entityType
}

func canonicalDelegationEntityType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "customer", "cliente", "deudor", "debtor", "portfolio_client":
		return "client"
	default:
		return strings.TrimSpace(value)
	}
}

func delegationAnalysisIntent(req map[string]interface{}) string {
	intent := firstNonEmptyPipelineString(jsonFirstString(req, "analysis_intent"))
	if p, ok := req["params"].(map[string]interface{}); ok {
		intent = firstNonEmptyPipelineString(intent, jsonFirstString(p, "analysis_intent"))
	}
	return intent
}

func encodeDelegationRuntimeContext(conv *Conversation, req map[string]interface{}) string {
	if conv == nil {
		return ""
	}
	ctx := normalizeRuntimeContext(conversationBusinessID(conv), conv.RuntimeContext)
	entityRef, entityType := delegationEntityIdentity(req, conv)
	if entityRef != "" {
		active := map[string]any{
			"id":   entityRef,
			"type": firstNonEmptyPipelineString(entityType, "client"),
		}
		if existing, ok := ctx["active_entity"].(map[string]any); ok {
			for k, v := range existing {
				active[k] = v
			}
			active["id"] = entityRef
			active["type"] = firstNonEmptyPipelineString(entityType, fmt.Sprintf("%v", existing["type"]), "client")
		} else if existing, ok := ctx["active_entity"].(map[string]interface{}); ok {
			for k, v := range existing {
				active[k] = v
			}
			active["id"] = entityRef
			active["type"] = firstNonEmptyPipelineString(entityType, fmt.Sprintf("%v", existing["type"]), "client")
		}
		ctx["active_entity"] = active
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func hasDelegationAuditSignals(req map[string]interface{}) bool {
	for _, key := range []string{"type", "delegation_type", "task", "intent", "reason", "question", "prompt"} {
		if containsDelegationKeyword(jsonFirstString(req, key), "audit", "calidad", "quality", "contradic", "gap", "inconsisten", "valid") {
			return true
		}
	}
	return false
}

func hasEntity360Signals(req map[string]interface{}) bool {
	if len(stringListFromUnknown(req["fields"])) > 0 {
		return true
	}
	for _, key := range []string{"type", "delegation_type", "task", "intent", "reason", "question", "prompt"} {
		if containsDelegationKeyword(jsonFirstString(req, key), "entity", "360", "detalles", "detalle", "caso", "customer", "cliente") {
			return true
		}
	}
	return false
}

func containsDelegationKeyword(value string, keywords ...string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func stringListFromUnknown(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func stringifyDelegationValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := stringifyDelegationValue(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

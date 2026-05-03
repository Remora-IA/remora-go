package main

import (
	"context"
	"fmt"
	"strings"

	"channel/adapter"
	"channel/manifest"
	"remora-flujo/handoff"

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
func runLoop(ctx context.Context, ch *adapter.Client, conv *Conversation, rules *FlowRules, manifests map[string]*manifest.Manifest, userAnswer string, resources []MessageResource) (handoff.QueuedQuestion, bool, error) {
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

	// 1a. Capability-based routing (intent classification). Antes de las
	// reglas declarativas, miramos si la respuesta del usuario matchea
	// los intent_examples de algún framework activo. Si hay match, ese
	// framework habla primero. Esto reemplaza reglas name-based del estilo
	// `prepend_speaker: "<nombre>"` por routing emergente desde el manifest.
	intentSpan := rootCtx.Child("intent_classification")
	intentMatch := classifyIntent(userAnswer, manifests, conv.Frameworks)
	intentSpan.Var("intent_match", intentMatch)
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
		UserResources:    resources,
	}
	wantPreprocess := ""
	if rules != nil {
		for _, action := range rules.Match(evalCtx) {
			if action.PrependSpeaker != "" {
				drivers = reorderDrivers(drivers, action.PrependSpeaker)
			}
			if action.PrependSpeakerProviderOf != "" {
				if name := providerOfModelCapability(action.PrependSpeakerProviderOf, manifests, conv.Frameworks); name != "" {
					drivers = reorderDrivers(drivers, name)
				}
			}
			if action.Preprocess != "" && wantPreprocess == "" {
				wantPreprocess = action.Preprocess
			}
		}
	}

	rootCtx.Var("drivers_final", driverNames(drivers))

	// 2. Procesar respuesta si hay alguna.
	if userAnswer != "" || len(resources) > 0 {
		// Target framework para preprocesamiento: el primero del orden eventual
		// (puede haber sido reordenado por una regla).
		targetFramework := drivers[0].Name()

		enrichSpan := rootCtx.Child("enrich_answer")
		enrichSpan.Var("raw_user_answer", userAnswer)
		enrichedAnswer := userAnswer
		// Inyectar contexto de la tarea activa (ledger framework-tareas) si el
		// profile es cobranza. Esto hace que Sabio/Mensajero sepan sobre qué
		// cliente hablar sin depender de history parsing. Commit B.
		if task := activeTaskContext(); task != nil {
			if ctxLine := buildActiveTaskLine(enrichedAnswer, task); ctxLine != "" {
				enrichedAnswer = ctxLine + "\n\n" + enrichedAnswer
				enrichSpan.Var("active_task_injected", task.Title)
				enrichSpan.Decision("inject_active_task",
					fmt.Sprintf("ledger tiene task activa (%s); inyectada como contexto", task.Title))
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
				fmt.Printf("[flujo_api] preprocessVision error (continuando con texto plano): %v\n", perr)
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
					fmt.Printf("[flujo_api] driver %s.IngestAnswer error: %v\n", d.Name(), err)
					ingestSpan.Error(err)
				}
				ingestSpan.End()
				break
			}
		} else {
			d := drivers[0]
			qctx := QueuedAnswerCtx{
				Answer:       enrichedAnswer,
				QuestionText: "(contexto inicial)",
				Resources:    resources,
			}
			ingestSpan := rootCtx.Child("ingest_answer_initial")
			ingestSpan.Var("driver", d.Name())
			if err := d.IngestAnswer(ctx, ch, conv, qctx); err != nil {
				fmt.Printf("[flujo_api] driver %s.IngestAnswer (initial) error: %v\n", d.Name(), err)
				ingestSpan.Error(err)
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
			var text, extID, askVia string
			text, extID, askVia, ok = d.PollQuestion(ctx, ch, conv, asked[d.Name()])
			r = nextQuestionResponse{ID: extID, Text: text, AskVia: askVia}
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
		pollSpan.End()
		queue.SetSpeaker(d.Name())
		qid := queue.AddQuestion(d.Name(), r.ID, r.Text, r.AskVia, r.Chips)
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

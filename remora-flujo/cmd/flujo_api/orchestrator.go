package main

import (
	"context"
	"fmt"
	"strings"

	"channel/adapter"
	"channel/manifest"
	"remora-flujo/handoff"
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
	queue, err := loadQueue(conv.ID)
	if err != nil {
		return handoff.QueuedQuestion{}, false, err
	}
	if len(queue.Frameworks) == 0 {
		queue.Frameworks = append([]string(nil), conv.Frameworks...)
	}

	drivers := driversFor(conv)
	if len(drivers) == 0 {
		return handoff.QueuedQuestion{}, false, fmt.Errorf("no hay drivers activos para la conversación")
	}

	// 1a. Capability-based routing (intent classification). Antes de las
	// reglas declarativas, miramos si la respuesta del usuario matchea
	// los intent_examples de algún framework activo. Si hay match, ese
	// framework habla primero. Esto reemplaza reglas name-based del estilo
	// `prepend_speaker: "<nombre>"` por routing emergente desde el manifest.
	if intentMatch := classifyIntent(userAnswer, manifests, conv.Frameworks); intentMatch != "" {
		drivers = reorderDrivers(drivers, intentMatch)
	}

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

	// 2. Procesar respuesta si hay alguna.
	if userAnswer != "" || len(resources) > 0 {
		// Target framework para preprocesamiento: el primero del orden eventual
		// (puede haber sido reordenado por una regla).
		targetFramework := drivers[0].Name()

		enrichedAnswer := userAnswer
		// Si el usuario seleccionó "Gestionar: <deudor>", transformar en query 360°
		// explícita para que Sabio sepa exactamente qué analizar.
		if strings.HasPrefix(enrichedAnswer, "Gestionar: ") && len(drivers) > 0 && drivers[0].Name() == "sabio" {
			deudor := strings.TrimPrefix(enrichedAnswer, "Gestionar: ")
			enrichedAnswer = fmt.Sprintf(
				"Genera un análisis 360° completo del cliente/deudor '%s'. "+
					"Incluye todo lo que tengas en los datos: saldo total adeudado, días de mora, "+
					"facturas y documentos pendientes, historial de pagos reciente, "+
					"y las 3 acciones de cobranza más urgentes a tomar con este cliente.",
				deudor)
		}
		if wantPreprocess == "vision" && hasImageResource(resources) {
			out, perr := preprocessVision(ctx, conv, targetFramework, userAnswer, resources)
			if perr != nil {
				fmt.Printf("[flujo_api] preprocessVision error (continuando con texto plano): %v\n", perr)
			} else {
				enrichedAnswer = out
			}
		}

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
				if err := d.IngestAnswer(ctx, ch, conv, qctx); err != nil {
					fmt.Printf("[flujo_api] driver %s.IngestAnswer error: %v\n", d.Name(), err)
				}
				break
			}
		} else {
			d := drivers[0]
			qctx := QueuedAnswerCtx{
				Answer:       enrichedAnswer,
				QuestionText: "(contexto inicial)",
				Resources:    resources,
			}
			if err := d.IngestAnswer(ctx, ch, conv, qctx); err != nil {
				fmt.Printf("[flujo_api] driver %s.IngestAnswer (initial) error: %v\n", d.Name(), err)
			}
		}
		if err := saveQueue(conv.ID, queue); err != nil {
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
		var r nextQuestionResponse
		var ok bool
		if fp, hasFull := d.(fullPoller); hasFull {
			r, ok = fp.PollQuestionFull(ctx, ch, conv, asked[d.Name()])
		} else {
			var text, extID, askVia string
			text, extID, askVia, ok = d.PollQuestion(ctx, ch, conv, asked[d.Name()])
			r = nextQuestionResponse{ID: extID, Text: text, AskVia: askVia}
		}
		if !ok {
			continue
		}
		queue.SetSpeaker(d.Name())
		qid := queue.AddQuestion(d.Name(), r.ID, r.Text, r.AskVia, r.Chips)
		if err := saveQueue(conv.ID, queue); err != nil {
			return handoff.QueuedQuestion{}, false, err
		}
		for _, qq := range queue.Questions {
			if qq.ID == qid {
				return qq, true, nil
			}
		}
	}

	if err := saveQueue(conv.ID, queue); err != nil {
		return handoff.QueuedQuestion{}, false, err
	}
	return handoff.QueuedQuestion{}, false, nil
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

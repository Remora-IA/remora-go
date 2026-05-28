package main

import (
	"encoding/json"
	"os"
	"strings"
)

// FlowRules es el contrato declarativo de composición. Vive en disco
// (flow.rules.json) y se carga al arrancar la API. Define reglas que SOLO
// aplican cuando varios frameworks corren juntos en una conversación. Esto
// permite que cada framework siga siendo agnóstico: nada en framework-echo
// "sabe" que existe Alfa, y viceversa. El orquestador es quien las aplica.
//
// Para sumar comportamientos compuestos sin tocar los frameworks: editar
// flow.rules.json y agregar una rule. Si necesitas un nuevo tipo de
// condición o acción, extender FlowCondition / FlowAction aquí.
type FlowRules struct {
	Version     int        `json:"version"`
	Description string     `json:"description"`
	Rules       []FlowRule `json:"rules"`
}

type FlowRule struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	When        FlowCondition `json:"when"`
	Then        FlowAction    `json:"then"`
	// Deprecated marca reglas que ya no aplican porque su comportamiento se
	// resuelve ahora vía capability-based routing (ver intent.go) o porque
	// dependen de nombres de framework hardcodeados (regla 3 de
	// ARCHITECTURE.md). Se mantienen en el archivo como documentación
	// histórica pero no se ejecutan.
	Deprecated bool `json:"deprecated,omitempty"`
}

// FlowCondition declara cuándo aplica una regla. Todos los campos no-vacíos
// deben cumplirse (AND). Diseñado para ser fácil de extender.
type FlowCondition struct {
	FrameworksActiveAll        []string `json:"frameworks_active_all,omitempty"`
	FrameworksActiveAny        []string `json:"frameworks_active_any,omitempty"`
	UserAnswerCountMin         *int     `json:"user_answer_count_min,omitempty"`
	UserAnswerCountMax         *int     `json:"user_answer_count_max,omitempty"`
	UserMessageHasResourceType string   `json:"user_message_has_resource_type,omitempty"`
	UserIntentAny              []string `json:"user_intent_any,omitempty"`
	CapabilityMissing          string   `json:"capability_missing,omitempty"`
}

// FlowAction declara qué hacer si la condición se cumple.
type FlowAction struct {
	// PrependSpeaker mueve un framework al frente de la lista de polling
	// para el próximo turno (sin alterar la lista persistente).
	// DEPRECATED en reglas nuevas: usa name-based. Preferí
	// PrependSpeakerProviderOf (model capability) o dejar que intent.go
	// resuelva el routing por capabilities_semantic.
	PrependSpeaker string `json:"prepend_speaker,omitempty"`
	// PrependSpeakerProviderOf resuelve dinámicamente al primer framework
	// activo cuyo model.capabilities incluye la capability indicada (ej
	// "multimodal"). Reemplaza a PrependSpeaker para reglas que ya no
	// quieren mencionar nombres de frameworks específicos.
	PrependSpeakerProviderOf string `json:"prepend_speaker_provider_of,omitempty"`
	// Preprocess pide pre-procesar el input del usuario antes de entregarlo
	// al framework. Valores soportados: "vision".
	Preprocess           string `json:"preprocess,omitempty"`
	DelegateToProviderOf string `json:"delegate_to_provider_of,omitempty"`
	Note                 string `json:"note,omitempty"`
}

func loadFlowRules(path string) (*FlowRules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FlowRules{Version: 1}, nil
		}
		return nil, err
	}
	var fr FlowRules
	if err := json.Unmarshal(data, &fr); err != nil {
		return nil, err
	}
	return &fr, nil
}

// EvalContext es lo que el orquestador conoce al evaluar las reglas.
type EvalContext struct {
	FrameworksActive    []string
	UserAnswerCount     int
	UserAnswer          string
	UserResources       []MessageResource
	MissingCapabilities []string
}

// Match devuelve las acciones de las reglas cuyas condiciones se cumplen,
// en el orden declarativo del archivo.
func (fr *FlowRules) Match(ctx EvalContext) []FlowAction {
	var out []FlowAction
	for _, r := range fr.Rules {
		if r.Deprecated {
			continue
		}
		if condMatches(r.When, ctx) {
			out = append(out, r.Then)
		}
	}
	return out
}

func condMatches(c FlowCondition, ctx EvalContext) bool {
	if len(c.FrameworksActiveAll) > 0 {
		set := map[string]bool{}
		for _, f := range ctx.FrameworksActive {
			set[f] = true
		}
		for _, need := range c.FrameworksActiveAll {
			if !set[need] {
				return false
			}
		}
	}
	if len(c.FrameworksActiveAny) > 0 {
		set := map[string]bool{}
		for _, f := range ctx.FrameworksActive {
			set[f] = true
		}
		any := false
		for _, need := range c.FrameworksActiveAny {
			if set[need] {
				any = true
				break
			}
		}
		if !any {
			return false
		}
	}
	if c.UserAnswerCountMin != nil && ctx.UserAnswerCount < *c.UserAnswerCountMin {
		return false
	}
	if c.UserAnswerCountMax != nil && ctx.UserAnswerCount > *c.UserAnswerCountMax {
		return false
	}
	if c.UserMessageHasResourceType != "" {
		found := false
		for _, r := range ctx.UserResources {
			if r.Type == c.UserMessageHasResourceType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(c.UserIntentAny) > 0 {
		answer := strings.ToLower(ctx.UserAnswer)
		found := false
		for _, intent := range c.UserIntentAny {
			intent = strings.ToLower(strings.TrimSpace(intent))
			if intent != "" && strings.Contains(answer, intent) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if c.CapabilityMissing != "" {
		found := false
		for _, cap := range ctx.MissingCapabilities {
			if cap == c.CapabilityMissing {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// reorderDrivers aplica el efecto PrependSpeaker: mueve al frente el driver
// indicado, manteniendo el resto en su orden original.
func reorderDrivers(drivers []FrameworkDriver, prependName string) []FrameworkDriver {
	if prependName == "" {
		return drivers
	}
	var first FrameworkDriver
	rest := make([]FrameworkDriver, 0, len(drivers))
	for _, d := range drivers {
		if first == nil && d.Name() == prependName {
			first = d
			continue
		}
		rest = append(rest, d)
	}
	if first == nil {
		return drivers
	}
	out := make([]FrameworkDriver, 0, len(drivers))
	out = append(out, first)
	out = append(out, rest...)
	return out
}

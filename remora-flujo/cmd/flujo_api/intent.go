package main

import (
	"strings"

	"channel/manifest"
)

// classifyIntent matchea userAnswer contra capabilities_semantic.intent_examples
// de cada manifest activo y devuelve el nombre del framework con mejor match,
// o "" si no hay señal suficiente.
//
// Heurística (sin LLM, primera iteración):
//   - Substring full case-insensitive del ejemplo dentro de la respuesta = +2.0.
//   - Token overlap (palabras >=4 chars del ejemplo presentes en la respuesta).
//     Si ratio >= 0.5, suma el ratio.
//
// Umbral mínimo: bestScore >= 1.0 para devolver match (evita ruido).
//
// Esto reemplaza reglas name-based en flow.rules.json del estilo
// `prepend_speaker: "sabio"` por routing emergente desde el manifest.
func classifyIntent(userAnswer string, manifests map[string]*manifest.Manifest, active []string) string {
	user := strings.ToLower(strings.TrimSpace(userAnswer))
	if user == "" || len(manifests) == 0 {
		return ""
	}
	bestName := ""
	bestScore := 0.0
	for _, name := range active {
		m, ok := manifests[name]
		if !ok || m == nil {
			continue
		}
		examples := m.CapabilitiesSemantic.IntentExamples
		if len(examples) == 0 {
			continue
		}
		score := 0.0
		for _, ex := range examples {
			exLower := strings.ToLower(strings.TrimSpace(ex))
			if exLower == "" {
				continue
			}
			if strings.Contains(user, exLower) {
				score += 2.0
				continue
			}
			tokens := strings.Fields(exLower)
			matched, total := 0, 0
			for _, t := range tokens {
				if len(t) < 4 {
					continue
				}
				total++
				if strings.Contains(user, t) {
					matched++
				}
			}
			if total > 0 {
				ratio := float64(matched) / float64(total)
				if ratio >= 0.5 {
					score += ratio
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestName = name
		}
	}
	if bestScore < 1.0 {
		return ""
	}
	return bestName
}

// providerOfModelCapability busca el primer framework activo cuyo
// model.capabilities incluya la capability solicitada (ej "multimodal").
// Sirve para resolver acciones declarativas tipo
// `prepend_speaker_provider_of: "multimodal"` sin nombrar frameworks
// específicos en flow.rules.json.
func providerOfModelCapability(cap string, manifests map[string]*manifest.Manifest, active []string) string {
	if cap == "" {
		return ""
	}
	for _, name := range active {
		m, ok := manifests[name]
		if !ok || m == nil {
			continue
		}
		for _, c := range m.Model.Capabilities {
			if strings.EqualFold(c, cap) {
				return name
			}
		}
	}
	return ""
}

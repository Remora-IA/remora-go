package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"remora-flujo/internal/llm"
)

type flowSuggestRequest struct {
	BusinessID   string `json:"business_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Max          int    `json:"max,omitempty"`
	Language     string `json:"language,omitempty"`
	ExistingFlow []struct {
		Framework  string `json:"framework"`
		Capability string `json:"capability"`
	} `json:"existing_flow,omitempty"`
}

type flowCapabilitySuggestion struct {
	Framework   string  `json:"framework"`
	Capability  string  `json:"capability"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Reason      string  `json:"reason"`
	Category    string  `json:"category"`
	Confidence  float64 `json:"confidence"`
}

type flowSuggestResponse struct {
	Suggestions []flowCapabilitySuggestion `json:"suggestions"`
	Source      string                     `json:"source"`
}

func (s *server) suggestFlowCapabilities(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req flowSuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if req.BusinessID != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.BusinessID, nil); !ok {
			return
		}
	}
	max := req.Max
	if max <= 0 || max > 8 {
		max = 5
	}
	caps := dedupCapabilityInfos(buildCapabilityRegistry(s.allManifests))
	fallback := heuristicFlowSuggestions(req, caps, max)
	if strings.TrimSpace(req.Name+" "+req.Description) == "" {
		writeOK(w, flowSuggestResponse{Suggestions: fallback, Source: "heuristic"})
		return
	}
	suggestions, err := s.llmFlowSuggestions(r.Context(), user.Email, req, caps, max)
	if err != nil || len(suggestions) == 0 {
		writeOK(w, flowSuggestResponse{Suggestions: fallback, Source: "heuristic"})
		return
	}
	writeOK(w, flowSuggestResponse{Suggestions: normalizeSuggestions(suggestions, caps, max), Source: "ai"})
}

func (s *server) llmFlowSuggestions(ctx context.Context, userEmail string, req flowSuggestRequest, caps []capabilityProviderInfo, max int) ([]flowCapabilitySuggestion, error) {
	spec, err := modelSpecFor(&Conversation{ID: "flow_suggest", Models: map[string]string{}}, "sabio")
	if err != nil {
		return nil, err
	}
	client, err := llm.New(spec)
	if err != nil {
		return nil, err
	}
	catalog := make([]map[string]any, 0, len(caps))
	for _, c := range caps {
		catalog = append(catalog, map[string]any{
			"framework":   c.Framework,
			"capability":  c.Capability,
			"title":       humanCapabilityTitle(c),
			"description": c.Description,
			"category":    capabilityCategory(c),
			"requires":    c.Requires,
			"produces":    c.Produces,
			"policies":    c.Policies,
		})
	}
	rawCatalog, _ := json.Marshal(catalog)
	system := strings.Join([]string{
		"Eres un diseñador de automatizaciones para Remora.",
		"El usuario habla español y no entiende nombres técnicos.",
		"Debes elegir capabilities reales del catálogo para crear un flujo útil.",
		"Todo flujo tiene lifecycle: bootstrap prepara contexto antes de que el usuario hable; entry habla primero con el usuario; pipeline ejecuta lo elegido.",
		"Si existe una capability que prioriza/indexa/prepara contexto, inclúyela al inicio como bootstrap; luego elige un entry conversacional/operativo; luego el pipeline.",
		"Devuelve SOLO JSON válido con esta forma exacta:",
		`{"suggestions":[{"framework":"...","capability":"...","title":"...","description":"...","reason":"...","category":"...","confidence":0.0}]}`,
		"No inventes frameworks ni capabilities. Usa máximo las sugerencias pedidas.",
	}, " ")
	user := "Usuario: " + userEmail + "\nNegocio: " + req.BusinessID + "\nNombre de automatización: " + req.Name + "\nDescripción: " + req.Description + "\nMáximo: " + strconv.Itoa(max) + "\n\nCatálogo de capabilities reales:\n" + string(rawCatalog)
	out, err := client.Complete(ctx, llm.CompletionRequest{System: system, User: user, MaxTokens: 1400})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Suggestions []flowCapabilitySuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(cleanJSONText(out)), &parsed); err != nil {
		return nil, err
	}
	return parsed.Suggestions, nil
}

func dedupCapabilityInfos(reg capabilityRegistry) []capabilityProviderInfo {
	seen := map[string]bool{}
	out := []capabilityProviderInfo{}
	for _, providers := range reg {
		for _, p := range providers {
			if p.Framework == "" || p.Capability == "" || p.Command == "" {
				continue
			}
			key := p.Framework + "." + p.Capability
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Framework+"."+out[i].Capability < out[j].Framework+"."+out[j].Capability
	})
	return out
}

func heuristicFlowSuggestions(req flowSuggestRequest, caps []capabilityProviderInfo, max int) []flowCapabilitySuggestion {
	text := strings.ToLower(req.Name + " " + req.Description)
	scored := []struct {
		c     capabilityProviderInfo
		score int
	}{}
	for _, c := range caps {
		hay := strings.ToLower(c.Framework + " " + c.Capability + " " + c.Description + " " + strings.Join(c.Requires, " ") + " " + strings.Join(c.Produces, " "))
		score := 0
		for _, token := range strings.FieldsFunc(text, func(r rune) bool { return r < 'a' || r > 'z' }) {
			if len(token) < 4 {
				continue
			}
			if strings.Contains(hay, token) {
				score += 2
			}
		}
		switch {
		case strings.Contains(text, "cobran") || strings.Contains(text, "deud") || strings.Contains(text, "mora") || strings.Contains(text, "cartera"):
			if c.Framework == "radar" || c.Framework == "foco" || c.Framework == "sabio" || c.Framework == "mecanico" || c.Framework == "hosting" || c.Framework == "mensajero" {
				score += 5
			}
		case strings.Contains(text, "email") || strings.Contains(text, "correo") || strings.Contains(text, "mensaje"):
			if c.Framework == "mensajero" || c.Framework == "gmail" || c.Framework == "hosting" {
				score += 5
			}
		case strings.Contains(text, "dato") || strings.Contains(text, "tabla") || strings.Contains(text, "sql") || strings.Contains(text, "analiz"):
			if c.Framework == "sabio" || c.Framework == "indexa" {
				score += 5
			}
		}
		if score > 0 {
			scored = append(scored, struct {
				c     capabilityProviderInfo
				score int
			}{c, score})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].c.Framework+"."+scored[i].c.Capability < scored[j].c.Framework+"."+scored[j].c.Capability
		}
		return scored[i].score > scored[j].score
	})
	out := []flowCapabilitySuggestion{}
	for _, item := range scored {
		if len(out) >= max {
			break
		}
		out = append(out, suggestionFromCapability(item.c, "Encaja con la descripción de la automatización.", 0.65))
	}
	if len(out) == 0 {
		for _, c := range caps {
			if c.Framework == "foco" || c.Framework == "sabio" {
				out = append(out, suggestionFromCapability(c, "Buena capacidad general para comenzar un flujo de negocio.", 0.45))
				if len(out) >= max {
					break
				}
			}
		}
	}
	return out
}

func normalizeSuggestions(in []flowCapabilitySuggestion, caps []capabilityProviderInfo, max int) []flowCapabilitySuggestion {
	byKey := map[string]capabilityProviderInfo{}
	for _, c := range caps {
		byKey[c.Framework+"."+c.Capability] = c
	}
	out := []flowCapabilitySuggestion{}
	seen := map[string]bool{}
	for _, s := range in {
		key := strings.TrimSpace(s.Framework) + "." + strings.TrimSpace(s.Capability)
		c, ok := byKey[key]
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		normalized := suggestionFromCapability(c, s.Reason, s.Confidence)
		if s.Title != "" {
			normalized.Title = s.Title
		}
		if s.Description != "" {
			normalized.Description = s.Description
		}
		if s.Category != "" {
			normalized.Category = s.Category
		}
		out = append(out, normalized)
		if len(out) >= max {
			break
		}
	}
	return out
}

func suggestionFromCapability(c capabilityProviderInfo, reason string, confidence float64) flowCapabilitySuggestion {
	return flowCapabilitySuggestion{
		Framework:   c.Framework,
		Capability:  c.Capability,
		Title:       humanCapabilityTitle(c),
		Description: humanCapabilityDescription(c),
		Reason:      reason,
		Category:    capabilityCategory(c),
		Confidence:  confidence,
	}
}

func humanCapabilityTitle(c capabilityProviderInfo) string {
	id := strings.ToLower(c.Capability)
	switch {
	case strings.Contains(id, "priority"):
		return "Priorizar casos importantes"
	case strings.Contains(id, "entity_360"):
		return "Analizar una entidad en profundidad"
	case strings.Contains(id, "query.sql"):
		return "Consultar datos del negocio"
	case strings.Contains(id, "inventory"):
		return "Revisar inventario de datos"
	case strings.Contains(id, "email") && strings.Contains(id, "send"):
		return "Enviar correo"
	case strings.Contains(id, "email") || strings.Contains(id, "gmail"):
		return "Buscar o leer correos"
	case strings.Contains(id, "contact"):
		return "Buscar o guardar contactos"
	case strings.Contains(id, "credentials"):
		return "Configurar credenciales"
	case strings.Contains(id, "semantic"):
		return "Validar configuración del negocio"
	case strings.Contains(id, "fix"):
		return "Proponer o aplicar una corrección"
	default:
		parts := strings.FieldsFunc(c.Capability, func(r rune) bool { return r == '.' || r == '_' || r == '-' })
		for i, p := range parts {
			if p == "v1" || p == "data" || p == "business" {
				continue
			}
			parts[i] = strings.Title(p)
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	}
}

func humanCapabilityDescription(c capabilityProviderInfo) string {
	if strings.TrimSpace(c.Description) != "" {
		return c.Description
	}
	switch capabilityCategory(c) {
	case "Datos y análisis":
		return "Lee, consulta o resume información disponible del negocio."
	case "Comunicación":
		return "Ayuda a preparar, buscar o enviar comunicaciones."
	case "Contactos":
		return "Encuentra o administra destinatarios y contactos."
	case "Credenciales":
		return "Configura accesos necesarios para ejecutar acciones externas."
	case "Operaciones":
		return "Propone o ejecuta acciones operativas."
	default:
		return "Capacidad disponible del framework " + c.Framework + "."
	}
}

func capabilityCategory(c capabilityProviderInfo) string {
	id := strings.ToLower(c.Framework + "." + c.Capability)
	switch {
	case strings.Contains(id, "contact"):
		return "Contactos"
	case strings.Contains(id, "sabio") || strings.Contains(id, "data.") || strings.Contains(id, "inventory") || strings.Contains(id, "query"):
		return "Datos y análisis"
	case strings.Contains(id, "email") || strings.Contains(id, "gmail") || strings.Contains(id, "mensaje") || strings.Contains(id, "message"):
		return "Comunicación"
	case strings.Contains(id, "credential") || strings.Contains(id, "smtp") || strings.Contains(id, "cpanel"):
		return "Credenciales"
	case strings.Contains(id, "fix") || strings.Contains(id, "task") || strings.Contains(id, "deploy"):
		return "Operaciones"
	case strings.Contains(id, "foco") || strings.Contains(id, "priority"):
		return "Priorización"
	default:
		return "Otras capacidades"
	}
}

func cleanJSONText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

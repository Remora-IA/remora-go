package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"channel/manifest"
	"remora-flujo/internal/llm"
)

type flowSuggestRequest struct {
	BusinessID     string        `json:"business_id"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Max            int           `json:"max,omitempty"`
	Language       string        `json:"language,omitempty"`
	CapabilityHint string        `json:"capability_hint,omitempty"`
	Intent         flowIntent    `json:"intent,omitempty"`
	Lifecycle      flowLifecycle `json:"lifecycle,omitempty"`
	ExistingFlow   []struct {
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
	Proposal    *flowSuggestionProposal    `json:"proposal,omitempty"`
	Gaps        []flowIntegrationGap       `json:"gaps,omitempty"`
}

// flowIntegrationGap describe un sistema externo que el flujo necesita
// pero que ningún framework del catálogo cubre actualmente.
type flowIntegrationGap struct {
	System      string   `json:"system"`      // "Slack"
	Need        string   `json:"need"`        // "Para notificaciones en tiempo real"
	Alternatives []string `json:"alternatives"` // ["mensajero (email como alternativa)"]
	CanProceed  bool     `json:"can_proceed"` // si se puede armar el flujo igual sin esto
}

type flowSuggestionProposal struct {
	IntentPlan flowSuggestIntentPlan    `json:"intent_plan"`
	Bindings   []flowSuggestRoleBinding `json:"bindings,omitempty"`
	Manifest   flowManifest             `json:"manifest"`
	Derivation *flowDerivation          `json:"derivation,omitempty"`
	Compiled   flowCompiledManifest     `json:"compiled"`
}

type flowSuggestIntentPlan struct {
	Goal            string                `json:"goal,omitempty"`
	OperatorRole    string                `json:"operator_role,omitempty"`
	SuccessCriteria string                `json:"success_criteria,omitempty"`
	Description     string                `json:"description,omitempty"`
	Roles           []flowSuggestRolePlan `json:"roles,omitempty"`
	CapabilityHint  string                `json:"capability_hint,omitempty"`
}

type flowSuggestRolePlan struct {
	Role      string `json:"role"`
	Objective string `json:"objective,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type flowSuggestRoleBinding struct {
	Role             string  `json:"role"`
	Objective        string  `json:"objective,omitempty"`
	IntentReason     string  `json:"intent_reason,omitempty"`
	Framework        string  `json:"framework"`
	Capability       string  `json:"capability"`
	Title            string  `json:"title,omitempty"`
	SuggestionReason string  `json:"suggestion_reason,omitempty"`
	Category         string  `json:"category,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
}

func (s *server) suggestFlowCapabilities(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.requireCurrentUser(w, r)
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
	var business businessArtifactsResponse
	if req.BusinessID != "" {
		business = s.businessArtifacts(req.BusinessID)
	}
	max := req.Max
	if max <= 0 || max > 8 {
		max = 5
	}
	caps := dedupCapabilityInfos(buildCapabilityRegistry(s.allManifests))
	intentPlan := composeFlowSuggestIntentPlan(req, business)
	gaps := detectIntegrationGaps(req, s.allManifests)

	// Heurístico directo: lee semantic_rules de cada manifest y puntúa.
	// No usamos LLM aquí — el heurístico es determinista, testeable y no aluciná.
	suggestions := heuristicFlowSuggestions(req, intentPlan, caps, max)
	writeOK(w, flowSuggestResponse{
		Suggestions: suggestions,
		Source:      "heuristic",
		Proposal:    buildFlowSuggestionProposal(req, suggestions, s.allManifests, business),
		Gaps:        gaps,
	})
}

func (s *server) llmFlowSuggestions(ctx context.Context, userEmail string, req flowSuggestRequest, plan flowSuggestIntentPlan, caps []capabilityProviderInfo, max int) ([]flowCapabilitySuggestion, error) {
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
	// Construir descripción de frameworks dinámicamente desde semantic_rules de manifests
	fwRulesLines := []string{
		"Eres un diseñador de automatizaciones para Remora. El usuario habla español.",
		"",
		"REGLAS DE FRAMEWORKS (leídas de los manifests, obligatorias):",
	}
	// Agregar reglas de cada framework que tenga semantic_rules
	fwSeen := map[string]bool{}
	for _, c := range caps {
		if fwSeen[c.Framework] {
			continue
		}
		fwSeen[c.Framework] = true
		rules, hasRules := c.SemanticRules["use_when"]
		neverWithout := c.SemanticRules["never_without"]
		notFor := c.SemanticRules["not_for"]
		if !hasRules {
			continue
		}
		line := "- " + c.Framework + ": usar cuando → " + strings.Join(rules[:min(3, len(rules))], ", ")
		if len(neverWithout) > 0 {
			line += ". NUNCA sin: " + strings.Join(neverWithout, ", ")
		}
		if len(notFor) > 0 {
			line += ". NO usar para: " + notFor[0]
		}
		fwRulesLines = append(fwRulesLines, line)
	}
	fwRulesLines = append(fwRulesLines,
		"",
		"DEPENDENCIAS DURAS: si un framework tiene 'NUNCA sin: X', X debe aparecer también en las sugerencias.",
		"Elige SOLO capabilities reales del catálogo. Devuelve SOLO JSON:",
		`{"suggestions":[{"framework":"...","capability":"...","title":"...","description":"...","reason":"...","category":"...","confidence":0.0}]}`,
	)
	system := strings.Join(fwRulesLines, "\n")
	roleNames := make([]string, 0, len(plan.Roles))
	for _, role := range plan.Roles {
		roleNames = append(roleNames, role.Role)
	}
	user := "Usuario: " + userEmail +
		"\nNegocio: " + req.BusinessID +
		"\nNombre de automatización: " + req.Name +
		"\nDescripción: " + req.Description +
		"\nObjetivo: " + plan.Goal +
		"\nRoles objetivo: " + strings.Join(roleNames, ", ") +
		"\nHint técnico opcional: " + plan.CapabilityHint +
		"\nMáximo: " + strconv.Itoa(max) +
		"\n\nCatálogo de capabilities reales:\n" + string(rawCatalog)
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

// heuristicCandidatesToProviderInfos convierte las sugerencias del heurístico
// de vuelta a capabilityProviderInfo para pasarlas al LLM como catálogo restringido.
func heuristicCandidatesToProviderInfos(suggestions []flowCapabilitySuggestion, allCaps []capabilityProviderInfo) []capabilityProviderInfo {
	byKey := map[string]capabilityProviderInfo{}
	for _, c := range allCaps {
		byKey[c.Framework+"."+c.Capability] = c
	}
	out := []capabilityProviderInfo{}
	seen := map[string]bool{}
	for _, s := range suggestions {
		key := s.Framework + "." + s.Capability
		if seen[key] {
			continue
		}
		if info, ok := byKey[key]; ok {
			out = append(out, info)
			seen[key] = true
		}
	}
	return out
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

// heuristicFlowSuggestions puntúa capabilities leyendo semantic_rules de cada manifest.
// No tiene lógica hardcodeada por dominio — el conocimiento vive en los manifests.
func heuristicFlowSuggestions(req flowSuggestRequest, plan flowSuggestIntentPlan, caps []capabilityProviderInfo, max int) []flowCapabilitySuggestion {
	text := strings.ToLower(req.Name + " " + req.Description + " " + plan.Goal + " " + plan.Description + " " + plan.CapabilityHint)
	scored := []struct {
		c     capabilityProviderInfo
		score int
	}{}
	for _, c := range caps {
		hay := strings.ToLower(c.Framework + " " + c.Capability + " " + c.Description + " " + strings.Join(c.Requires, " ") + " " + strings.Join(c.Produces, " "))
		score := 0
		candidateRole := inferUniversalRoleForNode(flowNode{Framework: c.Framework, Capability: c.Capability}, nil)

		// Score by role match from intent plan
		for idx, role := range plan.Roles {
			if role.Role == candidateRole {
				score += 20 - idx*3
			}
		}

		// Score by capability hint
		if strings.TrimSpace(plan.CapabilityHint) != "" && strings.Contains(strings.ToLower(c.Capability), strings.ToLower(plan.CapabilityHint)) {
			score += 3
		}

		// Score by token overlap (capability description vs flow text)
		for _, token := range strings.FieldsFunc(text, func(r rune) bool { return r < 'a' || r > 'z' }) {
			if len(token) < 4 {
				continue
			}
			if strings.Contains(hay, token) {
				score += 2
			}
		}

		// Score by semantic_rules.use_when from manifest (data-driven, no hardcoding)
		if rules, ok := c.SemanticRules["use_when"]; ok {
			for _, signal := range rules {
				if strings.Contains(text, strings.ToLower(signal)) {
					score += 8 // strong signal from manifest
				}
			}
		}

		// Penalize if not_for matches the description
		if rules, ok := c.SemanticRules["not_for"]; ok {
			for _, signal := range rules {
				if strings.Contains(text, strings.ToLower(signal)) {
					score -= 8
				}
			}
		}

		// Umbral mínimo: al menos una señal semántica fuerte (≥8) o dos coincidencias de rol.
		// Evita incluir frameworks que solo pasan por overlap de tokens genéricos.
		if score >= 8 {
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
				out = append(out, suggestionFromCapability(c, "Capacidad general para empezar un flujo de negocio.", 0.45))
				if len(out) >= max {
					break
				}
			}
		}
	}
	return out
}

// knownExternalSystems mapea palabras clave a sistemas externos conocidos.
// Si ningún framework cubre el sistema → gap.
var knownExternalSystems = map[string]string{
	"slack":      "Slack",
	"hubspot":    "HubSpot",
	"salesforce": "Salesforce",
	"whatsapp":   "WhatsApp",
	"trello":     "Trello",
	"notion":     "Notion",
	"jira":       "Jira",
	"discord":    "Discord",
	"telegram":   "Telegram",
	"shopify":    "Shopify",
	"stripe":     "Stripe",
	"zendesk":    "Zendesk",
	"pipedrive":  "Pipedrive",
	"asana":      "Asana",
	"monday":     "Monday.com",
}

// detectIntegrationGaps identifica sistemas externos mencionados en el flujo
// que ningún framework del catálogo cubre. Lee covers_integrations de los manifests.
func detectIntegrationGaps(req flowSuggestRequest, manifests map[string]*manifest.Manifest) []flowIntegrationGap {
	text := strings.ToLower(req.Name + " " + req.Description)

	// Construir set de integraciones cubiertas por frameworks existentes
	covered := map[string]bool{}
	for _, m := range manifests {
		for _, integration := range m.SemanticRules.CoversIntegrations {
			covered[strings.ToLower(integration)] = true
		}
	}

	var gaps []flowIntegrationGap
	seen := map[string]bool{}
	for keyword, displayName := range knownExternalSystems {
		if !strings.Contains(text, keyword) {
			continue
		}
		if covered[keyword] || seen[displayName] {
			continue
		}
		seen[displayName] = true
		gaps = append(gaps, flowIntegrationGap{
			System:       displayName,
			Need:         "El flujo menciona " + displayName + " pero no hay un framework que lo conecte.",
			Alternatives: alternativesFor(keyword),
			CanProceed:   canProceedWithoutIntegration(keyword),
		})
	}
	return gaps
}

func alternativesFor(system string) []string {
	switch system {
	case "slack", "discord", "telegram":
		return []string{"Mensajero (email como alternativa de notificación)"}
	case "whatsapp":
		return []string{"Mensajero (email)", "WhatsApp requiere integración pendiente"}
	case "hubspot", "salesforce", "pipedrive":
		return []string{"Inspector puede conectar su API REST si tiene documentación pública"}
	case "shopify", "stripe":
		return []string{"Inspector puede conectar su API REST"}
	default:
		return []string{"Inspector puede conectar APIs REST con documentación pública"}
	}
}

func canProceedWithoutIntegration(system string) bool {
	// Sistemas de mensajería tienen alternativa (email)
	switch system {
	case "slack", "discord", "telegram", "whatsapp":
		return true
	default:
		return false
	}
}

func composeFlowSuggestIntentPlan(req flowSuggestRequest, business businessArtifactsResponse) flowSuggestIntentPlan {
	intent := req.Intent
	goal := firstNonEmptyString(strings.TrimSpace(intent.Goal), strings.TrimSpace(req.Name), strings.TrimSpace(req.Description))
	description := firstNonEmptyString(strings.TrimSpace(intent.Description), strings.TrimSpace(req.Description), strings.TrimSpace(req.Name))
	operatorRole := firstNonEmptyString(strings.TrimSpace(intent.OperatorRole), "staff")
	success := strings.TrimSpace(intent.SuccessCriteria)
	capabilityHint := firstNonEmptyString(strings.TrimSpace(req.CapabilityHint), strings.TrimSpace(intent.CapabilityHint))
	roles := composeIntentRoles(intent.Roles, goal, description, success, capabilityHint)
	planRoles := make([]flowSuggestRolePlan, 0, len(roles))
	for _, role := range roles {
		planRoles = append(planRoles, flowSuggestRolePlan{
			Role:      role,
			Objective: objectiveForIntentRole(role, goal, description),
			Reason:    reasonForIntentRole(role, description, business),
		})
	}
	return flowSuggestIntentPlan{
		Goal:            goal,
		OperatorRole:    operatorRole,
		SuccessCriteria: success,
		Description:     description,
		Roles:           planRoles,
		CapabilityHint:  capabilityHint,
	}
}

func composeIntentRoles(explicit []string, values ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(role string) {
		role = strings.TrimSpace(role)
		if role == "" || seen[role] {
			return
		}
		seen[role] = true
		out = append(out, role)
	}
	for _, role := range explicit {
		add(role)
	}
	text := strings.ToLower(strings.Join(values, " "))
	if strings.Contains(text, "analiz") || strings.Contains(text, "revis") || strings.Contains(text, "cartera") || strings.Contains(text, "mora") || strings.Contains(text, "deud") || strings.Contains(text, "scoring") || strings.Contains(text, "cobranza") {
		add("analizar")
	}
	if strings.Contains(text, "prioriz") || strings.Contains(text, "foco") || strings.Contains(text, "agenda") || strings.Contains(text, "siguiente") || strings.Contains(text, "tarea") || strings.Contains(text, "cobrador") {
		add("priorizar")
	}
	if strings.Contains(text, "valid") || strings.Contains(text, "audit") || strings.Contains(text, "verific") || strings.Contains(text, "aproba") {
		add("validar")
	}
	if strings.Contains(text, "redact") || strings.Contains(text, "borrador") || strings.Contains(text, "correo") || strings.Contains(text, "mensaje") || strings.Contains(text, "email") || strings.Contains(text, "prepar") {
		add("redactar")
	}
	if strings.Contains(text, "enviar") || strings.Contains(text, "aplicar") || strings.Contains(text, "ejecut") || strings.Contains(text, "provision") || strings.Contains(text, "import") {
		add("actuar")
	}
	if strings.Contains(text, "registr") || strings.Contains(text, "guardar") || strings.Contains(text, "document") {
		add("registrar")
	}
	if len(out) == 0 {
		add("analizar")
	}
	return out
}

func objectiveForIntentRole(role, goal, description string) string {
	switch role {
	case "analizar":
		return firstNonEmptyString(goal, description, "entender el caso antes de actuar")
	case "priorizar":
		return firstNonEmptyString(goal, description, "decidir qué hacer primero según urgencia y contexto")
	case "redactar":
		return firstNonEmptyString(description, goal, "preparar una propuesta o mensaje revisable")
	case "actuar":
		return firstNonEmptyString(goal, description, "ejecutar una acción operativa")
	case "validar":
		return firstNonEmptyString(goal, description, "verificar calidad y seguridad")
	case "registrar":
		return firstNonEmptyString(goal, description, "dejar trazabilidad del proceso")
	default:
		return firstNonEmptyString(goal, description)
	}
}

func reasonForIntentRole(role, description string, business businessArtifactsResponse) string {
	if role == "analizar" && containsString(business.Artifacts, "data.sqlite_db.v1") {
		return "Hay datos del negocio disponibles para empezar entendiendo el caso."
	}
	if role == "redactar" && strings.Contains(strings.ToLower(description), "correo") {
		return "La intención habla de preparar mensajes antes del binding técnico."
	}
	return "El plan conserva qué rol interviene antes de elegir framework o capability."
}

func buildFlowSuggestionBindings(plan flowSuggestIntentPlan, suggestions []flowCapabilitySuggestion, manifests map[string]*manifest.Manifest) []flowSuggestRoleBinding {
	bindings := make([]flowSuggestRoleBinding, 0, len(plan.Roles))
	used := map[string]bool{}
	for _, rolePlan := range plan.Roles {
		candidate, ok := selectSuggestionForRole(rolePlan.Role, plan.CapabilityHint, suggestions, manifests, used)
		if !ok {
			continue
		}
		key := candidate.Framework + "." + candidate.Capability
		used[key] = true
		bindings = append(bindings, flowSuggestRoleBinding{
			Role:             rolePlan.Role,
			Objective:        rolePlan.Objective,
			IntentReason:     rolePlan.Reason,
			Framework:        candidate.Framework,
			Capability:       candidate.Capability,
			Title:            candidate.Title,
			SuggestionReason: candidate.Reason,
			Category:         candidate.Category,
			Confidence:       candidate.Confidence,
		})
	}
	if len(bindings) > 0 {
		return bindings
	}
	for _, suggestion := range suggestions {
		if len(bindings) >= 4 || strings.TrimSpace(suggestion.Framework) == "" || strings.TrimSpace(suggestion.Capability) == "" {
			continue
		}
		role := inferUniversalRoleForNode(flowNode{Framework: suggestion.Framework, Capability: suggestion.Capability}, manifests)
		rolePlan, ok := findFlowSuggestRolePlan(plan, role)
		binding := flowSuggestRoleBinding{
			Role:             role,
			Framework:        suggestion.Framework,
			Capability:       suggestion.Capability,
			Title:            suggestion.Title,
			SuggestionReason: suggestion.Reason,
			Category:         suggestion.Category,
			Confidence:       suggestion.Confidence,
		}
		if ok {
			binding.Objective = rolePlan.Objective
			binding.IntentReason = rolePlan.Reason
		} else {
			binding.Objective = firstNonEmptyString(plan.Goal, plan.Description)
			binding.IntentReason = "El binding técnico permanece subordinado al objetivo y los roles del plan."
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func bindIntentPlanToSuggestions(plan flowSuggestIntentPlan, bindings []flowSuggestRoleBinding) []flowSuggestRoleBinding {
	if len(bindings) == 0 {
		return nil
	}
	ordered := make([]flowSuggestRoleBinding, 0, len(bindings))
	used := map[string]bool{}
	for _, rolePlan := range plan.Roles {
		for _, binding := range bindings {
			key := binding.Role + "|" + binding.Framework + "." + binding.Capability
			if used[key] || binding.Role != rolePlan.Role {
				continue
			}
			used[key] = true
			ordered = append(ordered, binding)
			break
		}
	}
	for _, binding := range bindings {
		key := binding.Role + "|" + binding.Framework + "." + binding.Capability
		if used[key] {
			continue
		}
		used[key] = true
		ordered = append(ordered, binding)
	}
	return ordered
}

func buildFlowNodesFromBindings(bindings []flowSuggestRoleBinding) []flowNode {
	nodes := make([]flowNode, 0, len(bindings))
	for _, binding := range bindings {
		if len(nodes) >= 4 || strings.TrimSpace(binding.Framework) == "" || strings.TrimSpace(binding.Capability) == "" {
			continue
		}
		nodes = append(nodes, flowNode{
			ID:         fmt.Sprintf("proposal_%d_%s", len(nodes)+1, strings.ReplaceAll(flowSafeIDStr(binding.Capability), "__", "_")),
			Framework:  binding.Framework,
			Capability: binding.Capability,
		})
	}
	return nodes
}

func findFlowSuggestRolePlan(plan flowSuggestIntentPlan, role string) (flowSuggestRolePlan, bool) {
	for _, item := range plan.Roles {
		if item.Role == role {
			return item, true
		}
	}
	return flowSuggestRolePlan{}, false
}

func selectSuggestionForRole(role, capabilityHint string, suggestions []flowCapabilitySuggestion, manifests map[string]*manifest.Manifest, used map[string]bool) (flowCapabilitySuggestion, bool) {
	best := flowCapabilitySuggestion{}
	bestScore := 0
	for idx, suggestion := range suggestions {
		key := suggestion.Framework + "." + suggestion.Capability
		if used[key] || strings.TrimSpace(suggestion.Framework) == "" || strings.TrimSpace(suggestion.Capability) == "" {
			continue
		}
		candidateRole := inferUniversalRoleForNode(flowNode{Framework: suggestion.Framework, Capability: suggestion.Capability}, manifests)
		score := 0
		if candidateRole == role {
			score += 100
		}
		if strings.TrimSpace(capabilityHint) != "" && suggestion.Capability == capabilityHint {
			score += 5
		}
		if bonus := 10 - idx; bonus > 0 {
			score += bonus
		}
		if score > bestScore {
			bestScore = score
			best = suggestion
		}
	}
	return best, bestScore >= 100
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

// filterDependencyRules elimina nodos que violan dependencias declaradas en
// semantic_rules.never_without de los manifests. Data-driven, sin hardcoding.
func filterDependencyRules(nodes []flowNode, manifests map[string]*manifest.Manifest) []flowNode {
	// Construir set de frameworks presentes
	present := map[string]bool{}
	for _, n := range nodes {
		present[n.Framework] = true
	}
	out := []flowNode{}
	for _, n := range nodes {
		m, ok := manifests[n.Framework]
		if !ok || m == nil {
			out = append(out, n)
			continue
		}
		// Verificar cada dependencia declarada en never_without
		valid := true
		for _, required := range m.SemanticRules.NeverWithout {
			if !present[required] {
				valid = false
				break
			}
		}
		if valid {
			out = append(out, n)
		}
	}
	return out
}

// canonicalDomain describe un dominio conocido con su pipeline canónico.
// Cuando el texto del flujo coincide con las señales del dominio, se usa
// este pipeline directamente — sin inferencia, sin heurístico, sin LLM.
type canonicalDomain struct {
	Signals  []string   // palabras clave que identifican el dominio
	Pipeline []flowNode // pipeline canónico, en orden
	Roles    []string   // roles semánticos del dominio (para el intent plan)
}

// canonicalDomains es la tabla de verdad del sistema.
// Cada dominio define el pipeline completo y correcto para ese caso de uso.
// Agregar un nuevo dominio = agregar una entrada aquí. Nada más cambia.
//
// Arquitectura de roles:
//   Inspector  → ingesta de fuentes externas (se hace una vez, no entra en el pipeline diario)
//   Sabio      → habla SQL, produce dataset.raw.v1 desde la DB ya poblada
//   Auditor    → verifica calidad de datos (saldos faltantes, registros incompletos)
//   Mecánico   → avisa al usuario si Auditor detecta brechas insalvables
//   Radar      → analiza bajo configuración semántica del negocio (umbrales, criterios)
//   Foco       → traduce el análisis en tareas concretas del día según contexto y eventos
var canonicalDomains = []canonicalDomain{
	{
		// Cobranzas: pipeline completo de gestión de cartera.
		// Sabio extrae, Auditor valida calidad, Mecánico alerta si falta data,
		// Radar puntúa y prioriza, Foco define las tareas del día.
		Signals: []string{
			"cobranza", "cobro", "mora", "moroso", "deuda", "deudor",
			"factura vencida", "cartera", "cobrador", "recupero", "impago",
			"analizar cobranza", "análisis de cobranza", "gestión de cobro",
		},
		Pipeline: []flowNode{
			{ID: "node_sabio_extract",   Framework: "sabio",   Capability: "data.query.sql"},
			{ID: "node_auditor_quality", Framework: "auditor", Capability: "data.quality.audit"},
			{ID: "node_mecanico_alert",  Framework: "mecanico", Capability: "message.draft.collection_email"},
			{ID: "node_radar_priority",  Framework: "radar",   Capability: "collection.priority_list"},
			{ID: "node_foco_task",       Framework: "foco",    Capability: "focus.next_collection_task"},
		},
		Roles: []string{"analizar", "validar", "priorizar"},
	},
	{
		// Solo auditoría/calidad de datos (sin análisis de negocio posterior).
		Signals: []string{
			"calidad de dato", "duplicado", "dato incorrecto", "integridad",
			"brecha de dato", "error en dato", "limpiar dato", "auditoría de datos",
		},
		Pipeline: []flowNode{
			{ID: "node_auditor_quality", Framework: "auditor",  Capability: "data.quality.audit"},
			{ID: "node_mecanico_fix",    Framework: "mecanico", Capability: "message.draft.collection_email"},
		},
		Roles: []string{"validar"},
	},
	{
		// Envío de comunicaciones / notificaciones.
		Signals: []string{
			"enviar correo", "enviar email", "notificar por email",
			"reporte por correo", "smtp", "notificación",
		},
		Pipeline: []flowNode{
			{ID: "node_mensajero_send", Framework: "mensajero", Capability: "email.send"},
		},
		Roles: []string{"actuar"},
	},
	{
		// Consulta de datos sin análisis posterior.
		Signals: []string{
			"consultar datos", "vista 360", "reporte sql", "query sql", "reporte de datos",
		},
		Pipeline: []flowNode{
			{ID: "node_sabio_query", Framework: "sabio", Capability: "data.query.sql"},
		},
		Roles: []string{"analizar"},
	},
}

// detectCanonicalDomain retorna el pipeline canónico si el texto del flujo
// encaja con un dominio conocido. Prioriza el dominio con más señales coincidentes.
func detectCanonicalDomain(text string) (canonicalDomain, bool) {
	text = strings.ToLower(text)
	bestMatch := canonicalDomain{}
	bestCount := 0
	for _, domain := range canonicalDomains {
		count := 0
		for _, signal := range domain.Signals {
			if strings.Contains(text, strings.ToLower(signal)) {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			bestMatch = domain
		}
	}
	return bestMatch, bestCount > 0
}

func buildFlowSuggestionProposal(req flowSuggestRequest, suggestions []flowCapabilitySuggestion, manifests map[string]*manifest.Manifest, business businessArtifactsResponse) *flowSuggestionProposal {
	intentPlan := composeFlowSuggestIntentPlan(req, business)

	// 1. Intentar pipeline canónico primero (determinista, sin inferencia).
	//    Si el dominio es conocido, usamos el pipeline exacto — punto final.
	flowText := req.Name + " " + req.Description + " " + intentPlan.Goal + " " + intentPlan.Description
	var nodes []flowNode
	if domain, ok := detectCanonicalDomain(flowText); ok {
		// Usar pipeline canónico directamente, sin heurístico ni role-binding
		for i, n := range domain.Pipeline {
			node := n
			if node.ID == "" {
				node.ID = fmt.Sprintf("node_%d_%s", i+1, flowSafeIDStr(node.Capability))
			}
			nodes = append(nodes, node)
		}
		// Sobreescribir roles del intent plan con los del dominio
		intentPlan.Roles = make([]flowSuggestRolePlan, 0, len(domain.Roles))
		for _, role := range domain.Roles {
			intentPlan.Roles = append(intentPlan.Roles, flowSuggestRolePlan{
				Role:      role,
				Objective: objectiveForIntentRole(role, intentPlan.Goal, intentPlan.Description),
				Reason:    "Dominio conocido: pipeline canónico aplicado.",
			})
		}
	} else {
		// 2. Fallback: tomar top sugerencias del heurístico, una por framework.
		//    Sin role-binding frágil — si el heurístico lo puntúa alto, entra.
		if len(suggestions) == 0 {
			return nil
		}
		seenFW := map[string]bool{}
		for _, s := range suggestions {
			if len(nodes) >= 3 || seenFW[s.Framework] || s.Framework == "" || s.Capability == "" {
				continue
			}
			seenFW[s.Framework] = true
			nodes = append(nodes, flowNode{
				ID:         fmt.Sprintf("node_%d_%s", len(nodes)+1, flowSafeIDStr(s.Capability)),
				Framework:  s.Framework,
				Capability: s.Capability,
			})
		}
	}

	nodes = filterDependencyRules(nodes, manifests)
	if len(nodes) == 0 {
		return nil
	}

	roles := flowSuggestRoleNames(intentPlan.Roles)
	if len(roles) == 0 {
		roles = []string{"analizar"}
	}
	manifest := flowManifest{
		BusinessID: req.BusinessID,
		Intent: flowIntent{
			Goal:            intentPlan.Goal,
			OperatorRole:    intentPlan.OperatorRole,
			SuccessCriteria: intentPlan.SuccessCriteria,
			Description:     intentPlan.Description,
			Roles:           roles,
			CapabilityHint:  intentPlan.CapabilityHint,
		},
		Lifecycle: req.Lifecycle,
		Nodes:     nodes,
		Policies:  []string{"trace_required"},
	}
	compilation := compileFlowManifest(manifest, manifests, business)
	return &flowSuggestionProposal{
		IntentPlan: intentPlan,
		Manifest:   compilation.Authored,
		Derivation: compilation.Derivation,
		Compiled:   compilation.Compiled,
	}
}

func flowSuggestRoleNames(items []flowSuggestRolePlan) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if role := strings.TrimSpace(item.Role); role != "" {
			out = append(out, role)
		}
	}
	return out
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

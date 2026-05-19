package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"channel/manifest"
	"remora-flujo/internal/llm"
)

func finishFlowRunStep(step flowRunStep) flowRunStep {
	step.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return step
}

func hasExternalSideEffect(policies []string) bool {
	for _, policy := range policies {
		if isExternalSideEffectPolicy(policy) {
			return true
		}
	}
	return false
}

func nodeRequiresRuntimeApproval(node flowNode, contract nodeContract, req flowRunRequest) (bool, string) {
	if req.Approved {
		return false, ""
	}
	mode := resolutionModeFromPolicies(contract.Policies)
	switch {
	case hasExternalSideEffect(contract.Policies):
		if req.TestMode {
			return false, ""
		}
		if req.DryRun {
			return true, "side effect externo omitido en prueba segura"
		}
		return true, "side effect externo requiere approved=true"
	case hasRuntimeMutationPolicy(contract.Policies):
		if req.DryRun {
			return true, "mutación de estado omitida en prueba segura"
		}
		return true, "mutación de estado requiere approved=true"
	case hasPolicy(contract.Policies, "approval_required"):
		if mode == resolutionInteractive {
			return true, "framework interactivo requiere approved=true"
		}
		if mode == resolutionHybrid {
			return true, "framework híbrido requiere approved=true para esta acción"
		}
		return true, "acción requiere approved=true"
	default:
		return false, ""
	}
}

func hasRuntimeMutationPolicy(policies []string) bool {
	for _, policy := range policies {
		p := strings.ToLower(strings.TrimSpace(policy))
		if p == "state_mutation" || p == "operator_authorized_write" || p == "external_mutation" {
			return true
		}
	}
	return false
}

func runtimeApprovalSummary(node flowNode, contract nodeContract) string {
	mode := resolutionModeFromPolicies(contract.Policies)
	action := node.Capability
	if action == "" {
		action = contract.Command
	}
	subject := action
	switch mode {
	case resolutionInteractive:
		return fmt.Sprintf("%s necesita confirmación del usuario antes de ejecutar %s.", subject, action)
	case resolutionHybrid:
		return fmt.Sprintf("%s preparó una acción híbrida y necesita aprobación antes de aplicar cambios.", subject)
	default:
		return fmt.Sprintf("%s requiere aprobación antes de ejecutar %s.", subject, action)
	}
}

func (s *server) generateHumanAcceptance(ctx context.Context, req flowRunRequest, step flowRunStep) string {
	summary := strings.TrimSpace(step.HumanSummary)
	if summary == "" {
		summary = "el análisis inicial quedó listo"
	}
	fallback := "ok, sigamos con eso"
	if shouldEmitHumanAcceptanceFromStep(step) {
		fallback = "acepto esta configuración"
	}
	m := s.allManifests[step.Framework]
	if m == nil {
		m, _, _ = s.findProviderForCapability(step.Capability)
	}
	spec, err := modelSpecFromManifest(m)
	if err != nil {
		return fallback
	}
	client, err := llm.New(spec)
	if err != nil {
		return fallback
	}
	system := "Imita a un usuario humano ocupado en una prueba de UX. Responde en español, en minúsculas, muy breve y natural. No expliques. No uses JSON. Máximo 8 palabras."
	user := "El sistema acaba de presentar este análisis inicial para un flujo de negocio:\n" + summary + "\n\nEl usuario acepta continuar. Escribe solo lo que diría el usuario."
	if shouldEmitHumanAcceptanceFromStep(step) {
		user = "El sistema acaba de proponer una configuración/algoritmo de análisis para reutilizar en el flujo:\n" + summary + "\n\nEl usuario acepta esa configuración. Escribe solo lo que diría el usuario."
	}
	out, err := client.Complete(ctx, llm.CompletionRequest{System: system, User: user, MaxTokens: 40})
	if err != nil {
		return fallback
	}
	out = strings.Trim(strings.TrimSpace(out), "\"'")
	out = strings.ReplaceAll(out, "\n", " ")
	if out == "" || len(out) > 80 {
		return fallback
	}
	return out
}

func (s *server) entryProviderName(flow flowManifest) string {
	for _, node := range flow.Nodes {
		if isFocoNode(node, s.allManifests) {
			return node.Framework
		}
	}
	if flow.Lifecycle.Entry.Framework != "" {
		return flow.Lifecycle.Entry.Framework
	}
	if flow.Lifecycle.Entry.Capability != "" {
		_, providerName, _ := s.findProviderForCapability(flow.Lifecycle.Entry.Capability)
		return providerName
	}
	if flow.Lifecycle.Tutela.Framework != "" {
		return flow.Lifecycle.Tutela.Framework
	}
	if flow.Lifecycle.Tutela.Capability != "" {
		_, providerName, _ := s.findProviderForCapability(flow.Lifecycle.Tutela.Capability)
		return providerName
	}
	if len(flow.Nodes) > 0 {
		return flow.Nodes[0].Framework
	}
	return ""
}

func modelSpecFromManifest(m *manifest.Manifest) (llm.Spec, error) {
	spec := llm.Spec{
		Provider:     envOr("REMORA_DEFAULT_LLM_PROVIDER", "groq"),
		Name:         envOr("REMORA_DEFAULT_LLM_MODEL", "meta-llama/llama-4-scout-17b-16e-instruct"),
		EnvKey:       envOr("REMORA_DEFAULT_LLM_ENV_KEY", "GROQ_API_KEY"),
		Capabilities: []string{"text"},
	}
	if m != nil && m.Model.Provider != "" && m.Model.Name != "" && m.Model.EnvKey != "" {
		spec.Provider = m.Model.Provider
		spec.Name = m.Model.Name
		spec.EnvKey = m.Model.EnvKey
		spec.Capabilities = m.Model.Capabilities
		spec.BaseURL = m.Model.BaseURL
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("REMORA_LLM_PROVIDER"))) {
	case "openrouter":
		spec.Provider = "openrouter"
		spec.EnvKey = "OPENROUTER_API_KEY"
		spec.Name = envOr("REMORA_OPENROUTER_MODEL", "meta-llama/llama-4-scout-17b-16e-instruct")
		spec.BaseURL = "https://openrouter.ai/api/v1/chat/completions"
	case "groq":
		spec.Provider = "groq"
		spec.EnvKey = "GROQ_API_KEY"
		spec.Name = envOr("REMORA_GROQ_MODEL", "meta-llama/llama-4-scout-17b-16e-instruct")
		spec.BaseURL = ""
	case "minimax":
		spec.Provider = "minimax"
		spec.EnvKey = "MINIMAX_API_KEY"
		spec.Name = envOr("REMORA_MINIMAX_MODEL", "MiniMax-Text-01")
		spec.BaseURL = ""
	}
	if os.Getenv(spec.EnvKey) == "" {
		if m != nil && m.Model.Fallback != nil && m.Model.Fallback.Provider != "" && os.Getenv(m.Model.Fallback.EnvKey) != "" {
			spec.Provider = m.Model.Fallback.Provider
			spec.Name = m.Model.Fallback.Name
			spec.EnvKey = m.Model.Fallback.EnvKey
			spec.BaseURL = m.Model.Fallback.BaseURL
		} else {
			return llm.Spec{}, fmt.Errorf("no hay API key para %s (env: %s)", spec.Provider, spec.EnvKey)
		}
	}
	return spec, nil
}

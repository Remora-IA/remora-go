package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"
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
			rawOptions, _ = m["action_options"].([]interface{})
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
		if label == "" {
			continue
		}
		options = append(options, map[string]string{"id": id, "label": label, "description": description})
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

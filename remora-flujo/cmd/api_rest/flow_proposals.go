package main

import (
	"context"
	"strings"
	"time"
)

func shouldApplyMecanicoProposals(artifacts map[string]flowRunArtifact) bool {
	id, ok := artifactString(artifacts["action.selection.v1"].Payload, "id")
	if !ok {
		return false
	}
	id = strings.ToLower(strings.TrimSpace(id))
	return id == "apply_mecanico_proposals" || id == "apply_all_mecanico_proposals" || id == "aplicar_propuestas_mecanico"
}

func (s *server) applyApprovedMecanicoProposals(ctx context.Context, runID string, req flowRunRequest, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	m, providerName, providerOK := s.findProviderForCapability("action.fix.apply_all")
	node := flowNode{ID: "mecanico_apply_approved", Framework: providerName, Capability: "action.fix.apply_all", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	step := flowRunStep{
		Node:           node.ID,
		Framework:      node.Framework,
		Capability:     node.Capability,
		Role:           flowRoleResolution,
		ResolutionMode: resolutionAutonomous,
		CycleIndex:     cycleIdx,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_start", step)
	if !req.Approved {
		step.Status = "awaiting_approval"
		step.Error = "aplicar propuestas de Mecánico requiere approved=true"
		step.HumanSummary = "Mecánico necesita aprobación explícita para aplicar propuestas sobre el dataset."
		result.Status = "needs_approval"
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return false
	}
	if !providerOK {
		step.Status = "failed"
		step.Error = "provider no encontrado para capability action.fix.apply_all"
		result.Status = "failed"
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return false
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		result.Status = "failed"
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return false
	}
	step.ResolutionMode = resolutionModeFromPolicies(contract.Policies)
	step.Command = contract.Command
	step.Inputs = contract.Inputs
	step.Requires = contract.Requires
	step.Outputs = contract.Outputs
	step.Produces = contract.Produces
	step.Policies = contract.Policies
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
		result.Status = "failed"
	} else {
		step.ExitCode = resp.ExitCode
		step.DurationMs = resp.DurationMs
		step.StdoutPreview = previewText(resp.Stdout)
		step.StderrPreview = previewText(resp.Stderr)
		if !resp.Success || resp.ExitCode != 0 {
			step.Status = "failed"
			step.Error = strings.TrimSpace(resp.Error)
			if step.Error == "" {
				step.Error = strings.TrimSpace(resp.Stderr)
			}
			result.Status = "failed"
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
			step.HumanSummary = extractHumanSummary(resp.Stdout)
			if step.HumanSummary == "" {
				step.HumanSummary = "Mecánico aplicó las propuestas aprobadas y dejó el dataset actualizado."
			}
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	return step.Status == "completed"
}

// dataGap represents a parsed data gap from the data.gaps.v1 artifact.
type dataGap struct {
	Kind        string
	Description string
	Severity    string
	Field       string
}

// parseDataGaps extracts structured gaps from the data.gaps.v1 artifact.
func parseDataGaps(artifacts map[string]flowRunArtifact) []dataGap {
	gapArt, ok := artifacts["data.gaps.v1"]
	if !ok {
		return nil
	}
	var gapsRaw []interface{}
	if payload, ok := gapArt.Payload.(map[string]interface{}); ok {
		gapsRaw, _ = payload["gaps"].([]interface{})
		if len(gapsRaw) == 0 {
			gapsRaw, _ = payload["data_gaps"].([]interface{})
		}
	} else {
		gapsRaw, _ = gapArt.Payload.([]interface{})
	}
	if len(gapsRaw) == 0 {
		return nil
	}
	var gaps []dataGap
	for _, g := range gapsRaw {
		gmap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		kind := jsonFirstString(gmap, "rule", "kind", "type", "category", "field", "check_id")
		desc := jsonFirstString(gmap, "description", "message", "label", "gap")
		sev := jsonFirstString(gmap, "severity", "level", "priority")
		field := jsonFirstString(gmap, "field", "column", "property")
		if kind == "" && desc != "" {
			kind = "unknown"
		}
		if kind != "" {
			gaps = append(gaps, dataGap{Kind: kind, Description: desc, Severity: sev, Field: field})
		}
	}
	return gaps
}

func requestMecanicoProposalApproval(result *flowRunResult, step flowRunStep) {
	if result == nil || result.Status == "needs_input" {
		return
	}
	result.Status = "needs_input"
	result.NeedsInput = append(result.NeedsInput, flowRequiredInput{
		Artifact:   "mecanico.proposals.v1",
		Kind:       "approval",
		Framework:  step.Framework,
		Capability: "action.fix.apply",
		Title:      "Mecánico propuso remediaciones",
		Message:    "Revisá las propuestas de Mecánico y aprobá cuáles aplicar antes de modificar datos.",
		Suggestions: []string{
			"aplicar propuestas aprobadas",
			"rechazar propuestas",
			"pedir ajuste de propuestas",
		},
		Context: map[string]string{
			"source_node": step.Node,
			"mode":        resolutionHybrid,
		},
	})
}

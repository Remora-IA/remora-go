package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *server) ensureSabioDataMediation(ctx context.Context, runID string, req flowRunRequest, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int, trigger *flowStepTrigger) {
	const nodeID = "sabio_data_mediation"
	m, providerName, providerOK := s.findProviderForCapability("dataset.export")
	node := flowNode{ID: nodeID, Framework: providerName, Capability: "dataset.export", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	step := flowRunStep{
		Node:           nodeID,
		Framework:      providerName,
		Capability:     "dataset.export",
		Role:           flowRoleResolution,
		Visibility:     flowStepVisibilityInfrastructure,
		TriggeredBy:    trigger,
		ResolutionMode: resolutionModeForCapability(m, "dataset.export"),
		CycleIndex:     cycleIdx,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_start", step)
	if !providerOK {
		step.Status = "failed"
		step.Error = "provider no encontrado para capability dataset.export"
		step.HumanSummary = "No se pudo mediar datos porque el provider de dataset.export no está cargado."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		step.HumanSummary = "Sabio no pudo preparar el contrato de mediación de datos."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return
	}
	step.Command = contract.Command
	step.Inputs = contract.Inputs
	step.Requires = contract.Requires
	step.Outputs = contract.Outputs
	step.Produces = contract.Produces
	step.Policies = contract.Policies
	for _, reqArt := range contract.Requires {
		if reqArt != "" && !available[reqArt] {
			step.MissingArtifacts = append(step.MissingArtifacts, reqArt)
		}
	}
	if len(step.MissingArtifacts) > 0 {
		step.Status = "skipped"
		step.HumanSummary = "Sabio no medió datos porque faltan artefactos requeridos."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return
	}
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
		step.HumanSummary = "Sabio no pudo exportar el dataset canónico para el pipeline."
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
			step.HumanSummary = "Sabio intentó mediar datos pero la exportación no completó correctamente."
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, nodeID, contract, resp.Stdout, available, result.Artifacts)
			step.HumanSummary = "Sabio preparó el dataset canónico para Radar, Auditor y frameworks downstream."
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
}

func (s *server) shouldMediateSabioDataBeforeNode(node flowNode, available map[string]bool) bool {
	if !available["data.sqlite_db.v1"] {
		return false
	}
	m := s.allManifests[node.Framework]
	if m == nil {
		return false
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		return false
	}
	if containsString(contract.Policies, "data_mediator") {
		return false
	}
	for _, artifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
		if (artifact == "dataset.raw.v1" || artifact == "external.api.dump.v1") && !available[artifact] {
			return true
		}
	}
	return false
}

func (s *server) resolveDataRequestsAndRerunNode(ctx context.Context, runID string, req flowRunRequest, node flowNode, contract nodeContract, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) []flowRunStep {
	const maxDataRequestPasses = 3
	steps := []flowRunStep{}
	for pass := 0; pass < maxDataRequestPasses; pass++ {
		artifact, ok := result.Artifacts["data.request.v1"]
		if !ok || artifact.Payload == nil || !isResolvableSabioDataRequest(artifact.Payload) {
			break
		}
		requestPayload := artifact.Payload
		delete(available, "data.request.v1")
		delete(result.Artifacts, "data.request.v1")
		step, ok := s.resolveDataRequestAndRerunNode(ctx, runID, req, node, contract, available, result, emitStep, cycleIdx, pass, requestPayload)
		if step.Node != "" {
			steps = append(steps, step)
		}
		if !ok || !containsString(step.ArtifactTypes, "data.request.v1") {
			break
		}
	}
	if _, stillNeedsData := result.Artifacts["data.request.v1"]; stillNeedsData {
		payload := map[string]interface{}{
			"artifact_type": "data.request.limit_reached.v1",
			"node":          node.ID,
			"framework":     node.Framework,
			"max_passes":    maxDataRequestPasses,
			"reached_at":    time.Now().UTC().Format(time.RFC3339Nano),
		}
		path := s.persistFlowArtifact(runID, node.ID+"_data_request_limit", "data.request.limit_reached.v1", payload)
		available["data.request.limit_reached.v1"] = true
		result.Artifacts["data.request.limit_reached.v1"] = flowRunArtifact{Type: "data.request.limit_reached.v1", Source: "flow.engine", Node: node.ID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	return steps
}

func isResolvableSabioDataRequest(request interface{}) bool {
	reqPayload, _ := request.(map[string]interface{})
	capability := strings.TrimSpace(jsonFirstString(reqPayload, "capability"))
	return capability == "" || capability == "dataset.export"
}

func (s *server) resolveDataRequestAndRerunNode(ctx context.Context, runID string, req flowRunRequest, node flowNode, contract nodeContract, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int, pass int, requestPayload interface{}) (flowRunStep, bool) {
	if containsString(contract.Policies, "data_mediator") || !available["data.sqlite_db.v1"] {
		return flowRunStep{}, false
	}
	if !isResolvableSabioDataRequest(requestPayload) {
		return flowRunStep{}, false
	}
	reqPayload, _ := requestPayload.(map[string]interface{})
	reason := jsonFirstString(reqPayload, "reason", "message", "description", "configuration_reason")
	s.ensureSabioDataMediation(ctx, runID, req, available, result, emitStep, cycleIdx, triggerForFlowNode(node, "data_request"))
	if !available["dataset.raw.v1"] && !available["external.api.dump.v1"] {
		return flowRunStep{}, false
	}
	rerunNode := node
	rerunNode.ID = fmt.Sprintf("%s_after_data_request_%d", node.ID, pass+1)
	rerunNode.Role = flowRoleResolution
	recordDynamicFlowNode(result, rerunNode)
	step := flowRunStep{
		Node:           rerunNode.ID,
		Framework:      rerunNode.Framework,
		Capability:     rerunNode.Capability,
		Command:        contract.Command,
		Role:           flowRoleResolution,
		ResolutionMode: resolutionModeFromPolicies(contract.Policies),
		CycleIndex:     cycleIdx,
		Status:         "running",
		Inputs:         uniqueStrings(contract.Inputs),
		Requires:       uniqueStrings(contract.Requires),
		Outputs:        uniqueStrings(contract.Outputs),
		Produces:       uniqueStrings(contract.Produces),
		Policies:       uniqueStrings(contract.Policies),
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	if reason != "" {
		step.HumanSummary = "El framework pidió más datos: " + reason
	}
	emitStep("step_start", step)
	resp, execErr := s.executeFlowNode(ctx, runID, req, rerunNode, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
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
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, rerunNode.ID, contract, resp.Stdout, available, result.Artifacts)
			if summary := extractHumanSummary(resp.Stdout); summary != "" {
				step.HumanSummary = appendUniqueSummary(step.HumanSummary, summary)
			} else if step.HumanSummary == "" {
				step.HumanSummary = "Sabio respondió la solicitud de datos y el nodo se re-ejecutó con artifacts actualizados."
			}
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	return finished, step.Status == "completed"
}

package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *server) flowRequiresArtifact(flow flowManifest, artifact string) bool {
	for _, node := range flow.Nodes {
		m := s.allManifests[node.Framework]
		if m == nil {
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			continue
		}
		if containsString(append(contract.Inputs, contract.Requires...), artifact) {
			return true
		}
	}
	return false
}

func (s *server) shouldRunPreflightAudit(node flowNode, contract nodeContract, available map[string]bool) bool {
	if node.Capability == "data.quality.audit" {
		return false
	}
	if _, _, ok := s.findProviderForCapability("data.quality.audit"); !ok {
		return false
	}
	if !hasExternalSideEffect(contract.Policies) && !contractNeedsOperationalReadinessAudit(contract) {
		return false
	}
	return available["external.api.dump.v1"] || available["dataset.raw.v1"] || available["data.sqlite_db.v1"]
}

func contractNeedsOperationalReadinessAudit(contract nodeContract) bool {
	artifacts := append([]string{}, contract.Inputs...)
	artifacts = append(artifacts, contract.Requires...)
	artifacts = append(artifacts, contract.Produces...)
	for _, artifact := range uniqueStrings(artifacts) {
		switch artifact {
		case "contact.destination.v1", "message.draft.v1", "message.sent.v1", "credentials.smtp":
			return true
		}
	}
	return false
}

func (s *server) ensureFlowPreflightAudit(ctx context.Context, runID string, req flowRunRequest, target flowNode, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	if !available["external.api.dump.v1"] && !available["dataset.raw.v1"] && available["data.sqlite_db.v1"] {
		s.ensureSabioDataMediation(ctx, runID, req, available, result, emitStep, cycleIdx, triggerForFlowNode(target, "preflight"))
	}
	if !available["external.api.dump.v1"] && !available["dataset.raw.v1"] {
		return true
	}
	m, providerName, ok := s.findProviderForCapability("data.quality.audit")
	if !ok {
		return true
	}
	node := flowNode{ID: "preflight_audit_" + safeFilePart(target.ID), Framework: providerName, Capability: "data.quality.audit", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		return true
	}
	step := flowRunStep{
		Node:           node.ID,
		Framework:      node.Framework,
		Capability:     node.Capability,
		Command:        contract.Command,
		Role:           flowRoleResolution,
		Visibility:     flowStepVisibilityInfrastructure,
		TriggeredBy:    triggerForFlowNode(target, "preflight"),
		ResolutionMode: resolutionModeFromPolicies(contract.Policies),
		CycleIndex:     cycleIdx,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		Inputs:         uniqueStrings(contract.Inputs),
		Requires:       uniqueStrings(contract.Requires),
		Outputs:        uniqueStrings(contract.Outputs),
		Produces:       uniqueStrings(contract.Produces),
		Policies:       uniqueStrings(contract.Policies),
	}
	for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
		if reqArtifact != "" && !available[reqArtifact] {
			step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
		}
	}
	if len(step.MissingArtifacts) > 0 {
		step.Status = "skipped"
		step.HumanSummary = "Auditor no pudo validar preflight porque faltan datos para auditar."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return true
	}
	emitStep("step_start", step)
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
		result.Status = "failed"
	} else {
		step.ExitCode = resp.ExitCode
		step.DurationMs = resp.DurationMs
		step.HumanSummary = extractHumanSummary(resp.Stdout)
		step.StdoutPreview = previewText(resp.Stdout)
		step.StderrPreview = previewText(resp.Stderr)
		if !resp.Success || resp.ExitCode != 0 {
			step.Status = "failed"
			step.Error = strings.TrimSpace(resp.Error)
			if step.Error == "" {
				step.Error = strings.TrimSpace(resp.Stderr)
			}
			if step.Error == "" {
				step.Error = fmt.Sprintf("exit code %d", resp.ExitCode)
			}
			result.Status = "failed"
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
			if gapSummary := summarizeAuditorGaps(result.Artifacts); gapSummary != "" {
				if step.HumanSummary != "" {
					step.HumanSummary += "\n"
				}
				step.HumanSummary += gapSummary
			}
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	if step.Status != "completed" {
		s.recordFlowPreflight(runID, target, false, available, result.Artifacts, result.NeedsInput)
		return false
	}
	s.resolveFlowGapsIteratively(ctx, runID, req, node, available, result, emitStep, cycleIdx)
	ready := result.Status != "needs_input" && result.Status != "failed"
	s.recordFlowPreflight(runID, target, ready, available, result.Artifacts, result.NeedsInput)
	return ready
}

func (s *server) recordFlowPreflight(runID string, target flowNode, ready bool, available map[string]bool, artifacts map[string]flowRunArtifact, needs []flowRequiredInput) string {
	payload := map[string]interface{}{
		"artifact_type":      "flow.preflight.v1",
		"ready":              ready,
		"target_node":        target.ID,
		"target_framework":   target.Framework,
		"target_capability":  target.Capability,
		"checked_by":         "auditor",
		"checked_at":         time.Now().UTC().Format(time.RFC3339Nano),
		"auditor_available":  available["auditor.findings.v1"],
		"dataset_available":  available["external.api.dump.v1"] || available["dataset.raw.v1"],
		"readiness_artifact": "flow.readiness.v1",
	}
	if gapArt, ok := artifacts["data.gaps.v1"]; ok && gapArt.Payload != nil {
		payload["data_gaps"] = gapArt.Payload
	}
	if len(needs) > 0 {
		blockers := []map[string]interface{}{}
		for _, need := range needs {
			blockers = append(blockers, map[string]interface{}{
				"artifact":   need.Artifact,
				"kind":       need.Kind,
				"framework":  need.Framework,
				"capability": need.Capability,
				"message":    need.Message,
			})
		}
		payload["blockers"] = blockers
	}
	path := s.persistFlowArtifact(runID, "flow_preflight_"+safeFilePart(target.ID), "flow.preflight.v1", payload)
	available["flow.preflight.v1"] = true
	artifacts["flow.preflight.v1"] = flowRunArtifact{Type: "flow.preflight.v1", Source: "flow_engine", Node: target.ID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "flow.preflight.v1"
}

func triggerForFlowNode(node flowNode, reason string) *flowStepTrigger {
	if node.ID == "" && node.Framework == "" && node.Capability == "" {
		return nil
	}
	return &flowStepTrigger{Node: node.ID, Framework: node.Framework, Capability: node.Capability, Reason: reason}
}

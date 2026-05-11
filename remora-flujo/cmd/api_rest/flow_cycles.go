package main

import (
	"context"
	"strings"
	"time"

	"path/filepath"
)

func resetCycleArtifacts(available map[string]bool, artifacts map[string]flowRunArtifact) {
	cycleSpecific := []string{
		"entity.ref.v1",
		"collection.priority_item.v1",
		"focus.next_task.v1",
		"task.next",
		"entity_360.v1",
		"answer.grounded.v1",
		"contact.destination.v1",
		"message.draft.v1",
		"message.channel.v1",
		"message.sent.v1",
		"cycle.result.v1",
		"flow.cycle.completed.v1",
		"focus.cycle_status.v1",
		"task.done",
		"data.gaps.v1",
		"auditor.findings.v1",
		"action.options.v1",
		"action.selection.v1",
	}
	for _, a := range cycleSpecific {
		delete(available, a)
		delete(artifacts, a)
	}
}

func isCycleTerminalStep(node flowNode, contract nodeContract, step flowRunStep) bool {
	return step.Status == "completed" && (containsString(contract.Policies, "cycle_terminal") || containsString(step.ArtifactTypes, "message.sent.v1"))
}

func (s *server) recordFlowCycleCompleted(runID, nodeID, capability string, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	cycleKind := strings.TrimSpace(capability)
	if _, ok := artifacts["message.sent.v1"]; ok {
		cycleKind = "message_sent"
	}
	if cycleKind == "" {
		cycleKind = nodeID
	}
	payload := map[string]interface{}{
		"artifact_type": "flow.cycle.completed.v1",
		"cycle_kind":    cycleKind,
		"completed_by":  nodeID,
		"completed_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if entity, ok := artifacts["entity.ref.v1"].Payload.(map[string]interface{}); ok {
		payload["entity_ref"] = jsonFirstString(entity, "entity_ref", "id", "ref", "code")
		payload["entity_type"] = jsonFirstString(entity, "entity_type", "type", "kind")
		payload["entity_name"] = jsonFirstString(entity, "name", "label")
	}
	if task, ok := artifacts["task.next"].Payload.(map[string]interface{}); ok {
		payload["task_id"] = jsonFirstString(task, "task_id", "id")
	}
	if sent, ok := artifacts["message.sent.v1"].Payload.(map[string]interface{}); ok {
		payload["evidence"] = map[string]interface{}{
			"message_id": jsonFirstString(sent, "message_id", "id"),
			"to":         jsonFirstString(sent, "to", "destination"),
			"channel":    jsonFirstString(sent, "channel"),
		}
	} else {
		payload["evidence"] = map[string]interface{}{
			"completed_by": nodeID,
			"capability":   capability,
		}
	}
	path := s.persistFlowArtifact(runID, nodeID+"_cycle_completed", "flow.cycle.completed.v1", payload)
	available["flow.cycle.completed.v1"] = true
	artifacts["flow.cycle.completed.v1"] = flowRunArtifact{Type: "flow.cycle.completed.v1", Source: "flow_engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "flow.cycle.completed.v1"
}

func (s *server) notifyFocoCycleCompleted(ctx context.Context, runID, businessID, flowID string, available map[string]bool, artifacts map[string]flowRunArtifact) bool {
	m, providerName, ok := s.findProviderForCapability("focus.complete_cycle")
	if !ok {
		return false
	}
	cmd, ok := m.Commands["complete-cycle"]
	if !ok {
		return false
	}
	cycle, _ := artifacts["flow.cycle.completed.v1"].Payload.(map[string]interface{})
	if cycle == nil {
		return false
	}
	evidence, _ := cycle["evidence"].(map[string]interface{})
	params := map[string]string{
		"task_id":     jsonFirstString(cycle, "task_id"),
		"entity_ref":  jsonFirstString(cycle, "entity_ref"),
		"entity_name": jsonFirstString(cycle, "entity_name"),
		"cycle_kind":  jsonFirstString(cycle, "cycle_kind"),
		"message_id":  jsonFirstString(evidence, "message_id"),
		"to":          jsonFirstString(evidence, "to"),
	}
	if businessID != "" {
		setParamIfDeclared(cmd, params, "conv_id", focoFlowStateConvID(businessID, flowID))
	}
	if params["cycle_kind"] == "" {
		params["cycle_kind"] = "message_sent"
	}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + providerName
	}
	execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := s.scoped(runID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, filepath.Join(s.rootDir, cwdRel))
	if err != nil || resp.ExitCode != 0 {
		return false
	}
	payload := parseArtifactPayload(resp.Stdout)
	if typ, _ := payload["artifact_type"].(string); typ == "focus.cycle_status.v1" {
		nodeID := providerName + "_cycle_complete"
		path := s.persistFlowArtifact(runID, nodeID, "focus.cycle_status.v1", payload)
		available["focus.cycle_status.v1"] = true
		available["task.done"] = true
		artifacts["focus.cycle_status.v1"] = flowRunArtifact{Type: "focus.cycle_status.v1", Source: providerName + ".complete-cycle", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		artifacts["task.done"] = flowRunArtifact{Type: "task.done", Source: providerName + ".complete-cycle", Node: nodeID, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		return true
	}
	return false
}

func (s *server) recordTaskLedgerCycleCompleted(artifacts map[string]flowRunArtifact) {
}

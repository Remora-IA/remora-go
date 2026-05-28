package main

import (
	"context"
	"strings"
	"time"
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
		"work.context.v1",
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

func (s *server) recordCycleResult(runID, nodeID, capability string, step flowRunStep, cycleIdx int, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)
	payload := map[string]interface{}{
		"artifact_type":            "cycle.result.v1",
		"status":                   cycleResultStatus(step),
		"completed_by_capability":  capability,
		"completed_by_node":        nodeID,
		"result_summary":           strings.TrimSpace(step.HumanSummary),
		"cycle_index":              cycleIdx,
		"completed_at":             completedAt,
		"terminal_step_status":     step.Status,
		"terminal_step_artifacts":  append([]string(nil), step.ArtifactTypes...),
		"terminal_step_started_at": step.StartedAt,
	}
	if step.FinishedAt != "" {
		payload["terminal_step_finished_at"] = step.FinishedAt
	}
	if work, ok := artifacts["work.context.v1"].Payload.(map[string]interface{}); ok {
		if taskID := jsonFirstString(work, "task_id"); taskID != "" {
			payload["task_id"] = taskID
		}
		if taskTitle := jsonFirstString(work, "task_title"); taskTitle != "" {
			payload["task_title"] = taskTitle
		}
		if entityRef := jsonFirstString(work, "entity_ref"); entityRef != "" {
			payload["entity_ref"] = entityRef
		}
		if entityName := jsonFirstString(work, "entity_name"); entityName != "" {
			payload["entity_name"] = entityName
		}
		if selected, ok := work["selected_action"]; ok {
			payload["selected_action"] = selected
		}
	}
	if payload["task_id"] == nil {
		if task, ok := artifacts["task.next"].Payload.(map[string]interface{}); ok {
			if taskID := jsonFirstString(task, "task_id", "id"); taskID != "" {
				payload["task_id"] = taskID
			}
		}
	}
	if payload["entity_ref"] == nil {
		if entity, ok := artifacts["entity.ref.v1"].Payload.(map[string]interface{}); ok {
			if entityRef := jsonFirstString(entity, "entity_ref", "id", "ref", "code"); entityRef != "" {
				payload["entity_ref"] = entityRef
			}
		}
	}
	if cycle, ok := artifacts["flow.cycle.completed.v1"].Payload.(map[string]interface{}); ok {
		if evidence, ok := cycle["evidence"]; ok {
			payload["evidence"] = evidence
		}
		if kind := jsonFirstString(cycle, "cycle_kind"); kind != "" {
			payload["cycle_kind"] = kind
		}
	}
	if _, ok := payload["evidence"]; !ok {
		payload["evidence"] = map[string]interface{}{
			"completed_by": nodeID,
			"capability":   capability,
			"artifacts":    append([]string(nil), step.ArtifactTypes...),
		}
	}
	path := s.persistFlowArtifact(runID, nodeID+"_cycle_result", "cycle.result.v1", payload)
	available["cycle.result.v1"] = true
	artifacts["cycle.result.v1"] = flowRunArtifact{Type: "cycle.result.v1", Source: "flow_engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: completedAt}
	return "cycle.result.v1"
}

func cycleResultStatus(step flowRunStep) string {
	switch step.Status {
	case "completed", "max_cycles_reached":
		return "done"
	case "skipped":
		return "skipped"
	case "needs_input", "needs_approval", "blocked":
		return "needs_followup"
	default:
		if step.Status == "" {
			return "needs_followup"
		}
		return "failed"
	}
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
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := s.scoped(runID).ExecuteCommand(execCtx, runtime.Command, fullArgs, runtime.Cwd)
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

func (s *server) emitCycleNotification(businessID, flowID, runID string, artifacts map[string]flowRunArtifact) {
	cycle, ok := artifacts["cycle.result.v1"]
	if !ok {
		return
	}
	payload, _ := cycle.Payload.(map[string]interface{})
	if payload == nil {
		return
	}

	title := "Ciclo completado"
	if summary, ok := payload["result_summary"].(string); ok && summary != "" {
		title = summary
	}
	if entityName, ok := payload["entity_name"].(string); ok && entityName != "" {
		title = entityName + ": " + title
	}

	s.CreateNotification(businessID, createNotificationReq{
		FlowID:  flowID,
		RunID:   runID,
		Type:    "cycle.completed",
		Title:   title,
		Summary: cycleNotificationSummary(payload),
		Payload: payload,
	})
}

func cycleNotificationSummary(payload map[string]interface{}) string {
	parts := []string{}
	if kind, ok := payload["cycle_kind"].(string); ok && kind != "" {
		parts = append(parts, kind)
	}
	if status, ok := payload["status"].(string); ok && status != "" {
		parts = append(parts, status)
	}
	if cap, ok := payload["completed_by_capability"].(string); ok && cap != "" {
		parts = append(parts, cap)
	}
	return strings.Join(parts, " — ")
}

package main

import "time"

func (s *server) recordWorkContext(runID string, req flowRunRequest, nodeID string, cycleIdx int, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	payload := buildWorkContextPayload(req, runID, cycleIdx, artifacts)
	if payload == nil {
		return ""
	}
	path := s.persistFlowArtifact(runID, nodeID+"_work_context", "work.context.v1", payload)
	available["work.context.v1"] = true
	artifacts["work.context.v1"] = flowRunArtifact{Type: "work.context.v1", Source: "flow_engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "work.context.v1"
}

func buildWorkContextPayload(req flowRunRequest, runID string, cycleIdx int, artifacts map[string]flowRunArtifact) map[string]interface{} {
	task := firstArtifactMap(artifacts, "task.next", "focus.next_task.v1")
	entity := firstArtifactMap(artifacts, "entity.ref.v1")
	selectedAction := selectedActionPayload(artifacts)
	if len(task) == 0 && len(entity) == 0 {
		return nil
	}
	payload := map[string]interface{}{
		"artifact_type": "work.context.v1",
		"cycle_id":      workContextCycleID(runID, cycleIdx),
		"cycle_index":   cycleIdx,
		"date":          time.Now().Format("2006-01-02"),
		"flow_id":       req.Flow.ID,
		"business_id":   req.Flow.BusinessID,
	}
	if id := jsonFirstString(task, "task_id", "id"); id != "" {
		payload["task_id"] = id
	}
	if title := jsonFirstString(task, "task_title", "title", "name"); title != "" {
		payload["task_title"] = title
	}
	if len(entity) > 0 {
		payload["entity"] = entity
		if ref := jsonFirstString(entity, "entity_ref", "id", "ref", "code"); ref != "" {
			payload["entity_ref"] = ref
		}
		if typ := jsonFirstString(entity, "entity_type", "type", "kind"); typ != "" {
			payload["entity_type"] = typ
		}
		if name := jsonFirstString(entity, "name", "label"); name != "" {
			payload["entity_name"] = name
		}
	}
	if len(selectedAction) > 0 {
		payload["selected_action"] = selectedAction
		if id := jsonFirstString(selectedAction, "id", "action_id"); id != "" {
			payload["selected_action_id"] = id
		}
	}
	if expected := workContextExpectedOutcome(req, task); expected != "" {
		payload["expected_outcome"] = expected
	}
	if intentPayload, ok := artifacts["flow.intent.v1"].Payload.(map[string]interface{}); ok {
		payload["flow_intent"] = intentPayload
	}
	return payload
}

func firstArtifactMap(artifacts map[string]flowRunArtifact, types ...string) map[string]interface{} {
	for _, typ := range types {
		if payload, ok := artifacts[typ].Payload.(map[string]interface{}); ok && len(payload) > 0 {
			return payload
		}
	}
	return nil
}

func selectedActionPayload(artifacts map[string]flowRunArtifact) map[string]interface{} {
	if selected, ok := artifacts["action.selection.v1"].Payload.(map[string]interface{}); ok && len(selected) > 0 {
		return selected
	}
	return nil
}

func workContextExpectedOutcome(req flowRunRequest, task map[string]interface{}) string {
	if expected := jsonFirstString(task, "expected", "expected_outcome", "success_criteria"); expected != "" {
		return expected
	}
	return req.Flow.Intent.SuccessCriteria
}

func workContextCycleID(runID string, cycleIdx int) string {
	return runID + "_cycle_" + jsonNumberString(cycleIdx)
}

func jsonNumberString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

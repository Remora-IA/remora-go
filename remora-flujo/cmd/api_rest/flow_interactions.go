package main

import "strings"

func flowInputFromNode(need flowRequiredInput, node flowNode) flowRequiredInput {
	if strings.TrimSpace(need.Node) == "" {
		need.Node = strings.TrimSpace(node.ID)
	}
	if strings.TrimSpace(need.Framework) == "" {
		need.Framework = strings.TrimSpace(node.Framework)
	}
	if strings.TrimSpace(need.Capability) == "" {
		need.Capability = strings.TrimSpace(node.Capability)
	}
	if strings.TrimSpace(need.Role) == "" {
		need.Role = strings.TrimSpace(node.Role)
	}
	if strings.TrimSpace(need.Visibility) == "" {
		need.Visibility = flowStepVisibilityUserFacing
	}
	return need
}

func normalizeFlowRequiredInputs(needs []flowRequiredInput) []flowRequiredInput {
	for i := range needs {
		if strings.TrimSpace(needs[i].Visibility) == "" {
			needs[i].Visibility = flowStepVisibilityUserFacing
		}
	}
	return needs
}

func flowActionSelectionProvided(req flowRunRequest) bool {
	if req.InitialArtifacts == nil {
		return false
	}
	_, ok := req.InitialArtifacts["action.selection.v1"]
	return ok
}

func flowInputActionsFromOptions(options []map[string]string) []flowInputAction {
	out := make([]flowInputAction, 0, len(options))
	for _, option := range options {
		label := strings.TrimSpace(option["label"])
		if label == "" {
			continue
		}
		out = append(out, flowInputAction{
			ID:          strings.TrimSpace(option["id"]),
			Label:       label,
			Description: strings.TrimSpace(option["description"]),
		})
	}
	return out
}

func inputRequestForActionSelection(node flowNode, step flowRunStep, artifacts map[string]flowRunArtifact) flowRequiredInput {
	task := firstArtifactMap(artifacts, "task.next", "focus.next_task.v1")
	entity := firstArtifactMap(artifacts, "entity.ref.v1")
	message := strings.TrimSpace(step.HumanSummary)
	if message == "" {
		if title := jsonFirstString(task, "task_title", "title", "name"); title != "" {
			message = title
		} else {
			message = "Seleccioná la siguiente acción para continuar el flujo."
		}
	}
	title := "Seleccionar siguiente acción"
	if name := jsonFirstString(entity, "name", "label"); name != "" {
		title = "Seleccionar siguiente caso"
		if message == "" || message == "Seleccioná la siguiente acción para continuar el flujo." {
			message = "Elegí cómo querés continuar con " + name + "."
		}
	}
	ctx := map[string]string{}
	if taskID := jsonFirstString(task, "task_id", "id"); taskID != "" {
		ctx["task_id"] = taskID
	}
	if taskTitle := jsonFirstString(task, "task_title", "title", "name"); taskTitle != "" {
		ctx["task_title"] = taskTitle
	}
	if entityRef := jsonFirstString(entity, "entity_ref", "id", "ref", "code"); entityRef != "" {
		ctx["entity_ref"] = entityRef
	}
	if entityName := jsonFirstString(entity, "name", "label"); entityName != "" {
		ctx["entity_name"] = entityName
	}
	return flowInputFromNode(flowRequiredInput{
		Artifact: "action.selection.v1",
		Kind:     "action_selection",
		Title:    title,
		Message:  message,
		Actions:  flowInputActionsFromOptions(step.ActionOptions),
		Context:  ctx,
	}, node)
}

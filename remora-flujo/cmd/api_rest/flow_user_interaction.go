package main

import "strings"

const flowInteractionAnswerArtifact = "flow.interaction.answer.v1"

type providerQuestion struct {
	ID               string
	Text             string
	Framework        string
	Capability       string
	Title            string
	Kind             string
	Field            string
	FieldLabel       string
	InputType        string
	Placeholder      string
	Step             string
	NextTransition   string
	RequiredArtifact string
	Secret           bool
}

type flowInteractionAnswer struct {
	ArtifactType string `json:"artifact_type,omitempty"`
	Node         string `json:"node,omitempty"`
	Framework    string `json:"framework,omitempty"`
	Capability   string `json:"capability,omitempty"`
	Artifact     string `json:"artifact,omitempty"`
	QuestionID   string `json:"question_id,omitempty"`
	Field        string `json:"field,omitempty"`
	Step         string `json:"step,omitempty"`
	Value        string `json:"value,omitempty"`
}

func flowInteractionAnswerFromArtifacts(artifacts map[string]flowRunArtifact) (flowInteractionAnswer, bool) {
	art, ok := artifacts[flowInteractionAnswerArtifact]
	if !ok {
		return flowInteractionAnswer{}, false
	}
	payload, _ := art.Payload.(map[string]interface{})
	if payload == nil {
		return flowInteractionAnswer{}, false
	}
	answer := flowInteractionAnswer{
		ArtifactType: jsonFirstString(payload, "artifact_type"),
		Node:         jsonFirstString(payload, "node"),
		Framework:    jsonFirstString(payload, "framework"),
		Capability:   jsonFirstString(payload, "capability"),
		Artifact:     jsonFirstString(payload, "artifact"),
		QuestionID:   jsonFirstString(payload, "question_id"),
		Field:        jsonFirstString(payload, "field"),
		Step:         jsonFirstString(payload, "step"),
		Value:        jsonFirstString(payload, "value", "answer"),
	}
	if strings.TrimSpace(answer.Value) == "" {
		return flowInteractionAnswer{}, false
	}
	return answer, true
}

func (a flowInteractionAnswer) matchesFramework(framework string) bool {
	if strings.TrimSpace(framework) == "" {
		return true
	}
	if strings.TrimSpace(a.Framework) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a.Framework), strings.TrimSpace(framework))
}

func (a flowInteractionAnswer) matchesCapability(capabilities ...string) bool {
	if strings.TrimSpace(a.Capability) == "" {
		return false
	}
	for _, capability := range capabilities {
		if strings.EqualFold(strings.TrimSpace(a.Capability), strings.TrimSpace(capability)) {
			return true
		}
	}
	return false
}

func (a flowInteractionAnswer) matchesArtifact(artifact string) bool {
	if strings.TrimSpace(artifact) == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(a.Artifact), strings.TrimSpace(artifact))
}

func (a flowInteractionAnswer) matchesField(fields ...string) bool {
	if strings.TrimSpace(a.Field) == "" {
		return len(fields) == 0
	}
	for _, field := range fields {
		if strings.EqualFold(strings.TrimSpace(a.Field), strings.TrimSpace(field)) {
			return true
		}
	}
	return false
}

func providerQuestionToFlowInput(node flowNode, requiredArtifact string, q providerQuestion) flowRequiredInput {
	fieldName := firstNonEmptyPipelineString(q.Field, strings.TrimSpace(requiredArtifact))
	fieldType := "text"
	if q.Secret {
		fieldType = "password"
	} else if strings.TrimSpace(q.InputType) != "" {
		fieldType = strings.TrimSpace(q.InputType)
	}
	return flowInputFromNode(flowRequiredInput{
		Artifact:       firstNonEmptyPipelineString(q.RequiredArtifact, requiredArtifact),
		Kind:           firstNonEmptyPipelineString(q.Kind, "conversational_question"),
		Title:          firstNonEmptyPipelineString(q.Title, "Continuar con "+strings.Title(firstNonEmptyPipelineString(node.Framework, q.Framework, "el framework"))),
		Message:        q.Text,
		QuestionID:     q.ID,
		Field:          fieldName,
		Step:           q.Step,
		NextTransition: q.NextTransition,
		Fields: []flowInputField{{
			Name:        fieldName,
			Label:       firstNonEmptyPipelineString(q.FieldLabel, fieldName),
			Type:        fieldType,
			Required:    true,
			Placeholder: q.Placeholder,
		}},
	}, flowNode{
		ID:         firstNonEmptyPipelineString(node.ID, node.Framework),
		Framework:  firstNonEmptyPipelineString(node.Framework, q.Framework),
		Capability: firstNonEmptyPipelineString(q.Capability, node.Capability),
		Role:       firstNonEmptyPipelineString(node.Role, flowRoleResolution),
	})
}

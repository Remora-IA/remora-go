package main

import "strings"

func flowIntentDefined(intent flowIntent) bool {
	return strings.TrimSpace(intent.Goal) != "" ||
		strings.TrimSpace(intent.OperatorRole) != "" ||
		strings.TrimSpace(intent.SuccessCriteria) != "" ||
		strings.TrimSpace(intent.Description) != "" ||
		len(intent.Constraints) > 0
}

func flowIntentArtifactPayload(flow flowManifest) map[string]interface{} {
	intent := flow.Intent
	return map[string]interface{}{
		"artifact_type":     "flow.intent.v1",
		"flow_id":           flow.ID,
		"business_id":       flow.BusinessID,
		"audience":          flow.Audience,
		"goal":              strings.TrimSpace(intent.Goal),
		"operator_role":     strings.TrimSpace(intent.OperatorRole),
		"success_criteria":  strings.TrimSpace(intent.SuccessCriteria),
		"constraints":       compactIntentConstraints(intent.Constraints),
		"description":       strings.TrimSpace(intent.Description),
		"human_description": flowIntentHumanDescription(intent),
	}
}

func compactIntentConstraints(in []string) []string {
	out := []string{}
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func flowIntentHumanDescription(intent flowIntent) string {
	parts := []string{}
	if value := strings.TrimSpace(intent.Goal); value != "" {
		parts = append(parts, "Objetivo: "+value)
	}
	if value := strings.TrimSpace(intent.OperatorRole); value != "" {
		parts = append(parts, "Operador: "+value)
	}
	if value := strings.TrimSpace(intent.SuccessCriteria); value != "" {
		parts = append(parts, "Éxito: "+value)
	}
	if constraints := compactIntentConstraints(intent.Constraints); len(constraints) > 0 {
		parts = append(parts, "Restricciones: "+strings.Join(constraints, "; "))
	}
	if value := strings.TrimSpace(intent.Description); value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, ". ")
}

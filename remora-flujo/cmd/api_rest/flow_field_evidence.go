package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type flowFieldEvidence struct {
	Field      string `json:"field"`
	Label      string `json:"label"`
	Artifact   string `json:"artifact,omitempty"`
	Status     string `json:"status"`
	Value      string `json:"value,omitempty"`
	Source     string `json:"source,omitempty"`
	ValuePath  string `json:"value_path,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Structured bool   `json:"structured"`
}

type fieldEvidenceCandidate struct {
	Artifact string
	Paths    [][]string
	Validate func(string) bool
}

func buildFlowFieldEvidence(requiredFields []prerequisiteDataField, entityRef map[string]interface{}, artifacts map[string]flowRunArtifact, available map[string]bool) []flowFieldEvidence {
	out := make([]flowFieldEvidence, 0, len(requiredFields))
	for _, f := range requiredFields {
		out = append(out, flowFieldEvidenceForField(f, entityRef, artifacts, available))
	}
	return out
}

func flowFieldEvidenceForField(f prerequisiteDataField, entityRef map[string]interface{}, artifacts map[string]flowRunArtifact, available map[string]bool) flowFieldEvidence {
	ev := flowFieldEvidence{
		Field:      f.Field,
		Label:      f.Label,
		Status:     prereqMissing,
		Structured: false,
	}
	if value, path, ok := fieldEvidenceFromEntityRef(f, entityRef); ok {
		ev.Status = prereqAvailable
		ev.Value = value
		ev.Source = "entity.ref.v1"
		ev.Artifact = "entity.ref.v1"
		ev.ValuePath = path
		ev.Reason = "Campo evidenciado en la entidad priorizada."
		ev.Structured = true
		return ev
	}
	for _, candidate := range flowFieldEvidenceCandidates(f) {
		artifact := artifacts[candidate.Artifact]
		payload, ok := artifact.Payload.(map[string]interface{})
		if !ok {
			continue
		}
		for _, path := range candidate.Paths {
			value, ok := artifactValueAtPath(payload, path...)
			if !ok {
				continue
			}
			if candidate.Validate != nil && !candidate.Validate(value) {
				continue
			}
			ev.Status = prereqAvailable
			ev.Value = value
			ev.Source = artifact.Source
			if strings.TrimSpace(ev.Source) == "" {
				ev.Source = artifactLabelForAPI(candidate.Artifact)
			}
			ev.Artifact = candidate.Artifact
			ev.ValuePath = strings.Join(path, ".")
			ev.Reason = "Campo evidenciado en un artifact estructurado del run."
			ev.Structured = true
			return ev
		}
	}
	if available[f.Artifact] {
		ev.Artifact = f.Artifact
		ev.Source = "artifact disponible"
		if f.Derived {
			ev.Status = prereqDerivable
			ev.Reason = "El artifact requerido existe, pero este campo no quedó evidenciado de forma estructurada."
			return ev
		}
		ev.Reason = "El artifact requerido existe, pero no aporta evidencia estructurada para este campo."
		return ev
	}
	if f.Derived {
		ev.Status = prereqDerivable
		ev.Artifact = f.Artifact
		ev.Source = "derivable"
		ev.Reason = "El campo es derivable, pero en este run todavía no hay evidencia estructurada."
		return ev
	}
	ev.Artifact = f.Artifact
	ev.Reason = "No hay evidencia estructurada disponible para este campo."
	return ev
}

func flowFieldEvidenceCandidates(f prerequisiteDataField) []fieldEvidenceCandidate {
	field := strings.ToLower(strings.TrimSpace(f.Field))
	label := strings.ToLower(strings.TrimSpace(f.Label))
	switch {
	case field == "email" || strings.Contains(label, "correo") || strings.Contains(label, "email"):
		return []fieldEvidenceCandidate{
			{Artifact: "contact.destination.v1", Paths: [][]string{{"email"}, {"destination"}, {"to"}, {"value"}}, Validate: isLikelyEmail},
			{Artifact: "entity_360.v1", Paths: [][]string{{"email"}, {"contact_email"}, {"structured", "email"}}, Validate: isLikelyEmail},
			{Artifact: "message.draft.v1", Paths: [][]string{{"to"}, {"draft", "to"}}, Validate: isLikelyEmail},
		}
	case field == "name" || strings.Contains(label, "nombre"):
		return []fieldEvidenceCandidate{
			{Artifact: "entity.ref.v1", Paths: [][]string{{"name"}, {"display_name"}}},
			{Artifact: "entity_360.v1", Paths: [][]string{{"name"}, {"structured", "name"}}},
		}
	case field == "amount" || strings.Contains(label, "monto") || strings.Contains(label, "saldo"):
		return []fieldEvidenceCandidate{
			{Artifact: "collection.priority_item.v1", Paths: [][]string{{"saldo_total"}, {"monto"}, {"amount"}}},
			{Artifact: "entity_360.v1", Paths: [][]string{{"saldo_total"}, {"saldo"}, {"amount"}, {"structured", "amount"}, {"structured", "saldo_total"}}},
			{Artifact: "message.draft.v1", Paths: [][]string{{"draft", "saldo"}, {"saldo"}, {"structured", "amount"}}},
		}
	case isPastDueField(field, label):
		return []fieldEvidenceCandidate{
			{Artifact: "collection.priority_item.v1", Paths: [][]string{{"dias_mora_max"}, {"dias_mora"}, {"days_past_due"}}},
			{Artifact: "entity_360.v1", Paths: [][]string{{"dias_mora_max"}, {"dias_mora"}, {"days_past_due"}, {"structured", "days_past_due"}}},
			{Artifact: "message.draft.v1", Paths: [][]string{{"draft", "dias_mora"}, {"dias_mora"}, {"structured", "days_past_due"}}},
		}
	case isInvoiceNumberField(field, label):
		return []fieldEvidenceCandidate{
			{Artifact: "entity_360.v1", Paths: [][]string{{"invoice_number"}, {"document_number"}, {"structured", "invoice_number"}, {"structured", "document_number"}}},
			{Artifact: "message.draft.v1", Paths: [][]string{{"draft", "invoice_number"}, {"invoice_number"}, {"structured", "invoice_number"}}},
		}
	default:
		return nil
	}
}

func isPastDueField(field, label string) bool {
	return field == "days_past_due" || field == "dias_mora" || field == "date" || strings.Contains(label, "mora")
}

func isInvoiceNumberField(field, label string) bool {
	return field == "invoice_number" || field == "document_number" || (field == "number" && strings.Contains(label, "factura")) || strings.Contains(label, "factura")
}

func fieldEvidenceFromEntityRef(f prerequisiteDataField, entityRef map[string]interface{}) (string, string, bool) {
	if entityRef == nil {
		return "", "", false
	}
	switch strings.ToLower(strings.TrimSpace(f.Field)) {
	case "name":
		if value := entityRefFieldValue(entityRef, "name"); value != "" {
			if direct, ok := entityRef["name"].(string); ok && strings.TrimSpace(direct) != "" {
				return value, "name", true
			}
			return value, "display_name", true
		}
	case "email":
		if value, ok := entityRef["email"].(string); ok && isLikelyEmail(value) {
			return value, "email", true
		}
	default:
		if value, ok := artifactValueAtPath(entityRef, f.Field); ok {
			return value, f.Field, true
		}
	}
	return "", "", false
}

func artifactValueAtPath(payload map[string]interface{}, path ...string) (string, bool) {
	current := interface{}(payload)
	for _, part := range path {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return "", false
		}
		current = obj[part]
	}
	switch v := current.(type) {
	case string:
		v = strings.TrimSpace(v)
		return v, v != ""
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%.2f", v), true
	case int:
		return fmt.Sprintf("%d", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case json.Number:
		return v.String(), v.String() != ""
	default:
		return "", false
	}
}

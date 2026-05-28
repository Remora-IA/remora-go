package alfa

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func ExportBravo(spec *AlfaSpec, generated time.Time) *BravoIdealFlow {
	if generated.IsZero() {
		generated = time.Now()
	}
	rules := make([]BravoRule, 0, len(spec.BusinessRules)+len(spec.OpenQuestions))
	for _, rule := range spec.BusinessRules {
		rules = append(rules, BravoRule{
			Name:        rule.Name,
			Description: rule.Description,
			When:        rule.When,
			Then:        rule.Then,
			Importance:  rule.Importance,
		})
	}
	for _, question := range spec.OpenQuestions {
		rules = append(rules, BravoRule{
			Name:        "Pregunta abierta: " + question.ID,
			Description: question.Reason + " Pregunta para Echo: " + question.QuestionForEcho,
			Then:        "No implementar esta parte como definitiva hasta resolver la pregunta abierta.",
			Importance:  1,
		})
	}

	return &BravoIdealFlow{
		TraceID:       fmt.Sprintf("alfa_%d", generated.UnixNano()),
		Generated:     generated.Format(time.RFC3339),
		Description:   spec.AutomationIntent,
		Verbalization: buildVerbalization(spec),
		Intent:        spec.AutomationIntent,
		Rules:         rules,
		CriticalVars:  spec.CriticalVariables,
		CriticalPath:  criticalPath(spec),
	}
}

func SaveBravo(flow *BravoIdealFlow, path string) error {
	if path == "" {
		path = "ideal_flow.json"
	}
	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func buildVerbalization(spec *AlfaSpec) string {
	var b strings.Builder
	b.WriteString("Este IdealFlow fue generado por Framework Alfa desde un árbol Echo.\n\n")
	b.WriteString("Intención: ")
	b.WriteString(spec.AutomationIntent)
	b.WriteString("\n\nDolores confirmados:\n")
	for _, pain := range spec.ConfirmedPains {
		b.WriteString("- ")
		b.WriteString(pain.Title)
		if pain.ValidationAnswer != "" {
			b.WriteString(" (confirmado: ")
			b.WriteString(pain.ValidationAnswer)
			b.WriteString(")")
		}
		b.WriteString("\n")
	}

	b.WriteString("\nFlujo ideal esperado:\n")
	for i, step := range spec.IdealSteps {
		b.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, step.Name, step.Description))
	}

	if len(spec.DataModel.NormalizedTarget.Entities) > 0 {
		b.WriteString("\nMERE normalizado propuesto:\n")
		for _, entity := range spec.DataModel.NormalizedTarget.Entities {
			b.WriteString("- ")
			b.WriteString(entity.Name)
			if len(entity.Fields) > 0 {
				b.WriteString(" (")
				b.WriteString(strings.Join(entity.Fields, ", "))
				b.WriteString(")")
			}
			b.WriteString(": ")
			b.WriteString(entity.Description)
			b.WriteString("\n")
		}
	}

	if len(spec.DataModel.NormalizedTarget.Relationships) > 0 {
		b.WriteString("\nRelaciones MERE:\n")
		for _, relationship := range spec.DataModel.NormalizedTarget.Relationships {
			b.WriteString("- ")
			b.WriteString(relationship.From)
			b.WriteString(" -> ")
			b.WriteString(relationship.To)
			b.WriteString(" [")
			b.WriteString(relationship.Type)
			b.WriteString("]: ")
			b.WriteString(relationship.Description)
			b.WriteString("\n")
		}
	}

	if len(spec.SuccessCriteria) > 0 {
		b.WriteString("\nCriterios de éxito:\n")
		for _, criterion := range spec.SuccessCriteria {
			b.WriteString("- ")
			b.WriteString(criterion)
			b.WriteString("\n")
		}
	}

	if len(spec.Perceptions) > 0 {
		b.WriteString("\nPercepciones Echo que deben respetarse:\n")
		for _, perception := range spec.Perceptions {
			b.WriteString("- ")
			b.WriteString(perception)
			b.WriteString("\n")
		}
	}

	if len(spec.OpenQuestions) > 0 {
		b.WriteString("\nPreguntas abiertas. Estas partes NO deben tratarse como definitivas:\n")
		for _, question := range spec.OpenQuestions {
			b.WriteString("- ")
			b.WriteString(question.ID)
			b.WriteString(": ")
			b.WriteString(question.QuestionForEcho)
			b.WriteString(" (")
			b.WriteString(question.Reason)
			b.WriteString(")\n")
		}
	}

	return b.String()
}

func criticalPath(spec *AlfaSpec) []string {
	out := make([]string, 0, len(spec.IdealSteps))
	for _, step := range spec.IdealSteps {
		out = append(out, step.Name)
	}
	return out
}

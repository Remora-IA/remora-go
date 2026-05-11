package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"encoding/json"
	"path/filepath"
)

func (s *server) resolveMissingFlowArtifacts(ctx context.Context, runID string, req flowRunRequest, node flowNode, missing []string, available map[string]bool, artifacts map[string]flowRunArtifact, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) ([]string, []flowRequiredInput) {
	resolved := []string{}
	needs := []flowRequiredInput{}
	for _, artifact := range missing {
		switch artifact {
		case "contact.destination.v1":
			if payload, ok := contactDestinationFromArtifacts(artifacts); ok {
				available[artifact] = true
				path := s.persistFlowArtifact(runID, node.ID+"_resolver", artifact, payload)
				artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "resolver", Node: node.ID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				s.storeUserContactDestinationIfPossible(runID, req.Flow.BusinessID, artifacts)
				resolved = append(resolved, artifact)
				continue
			}
			if payload, ok := s.lookupSabioContactDestination(ctx, req.Flow.BusinessID, artifacts); ok {
				available[artifact] = true
				path := s.persistFlowArtifact(runID, node.ID+"_sabio_lookup", artifact, payload)
				artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "sabio.contact-lookup", Node: node.ID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				resolved = append(resolved, artifact)
				continue
			}
			gaps := []dataGap{{Kind: "contact.destination", Description: "Falta un destino de contacto para continuar el flujo.", Field: "contact.destination.v1"}}
			if questions, hasQuestions := s.invokeMecanicoResolveGaps(ctx, runID, req, gaps, artifacts, available, result, emitStep, cycleIdx); hasQuestions {
				providerName := s.providerNameForCapability("action.fix.resolve_gaps_conversational")
				for _, q := range questions {
					needs = append(needs, flowRequiredInput{
						Artifact:   "framework.question.v1",
						Kind:       "framework_question",
						Framework:  providerName,
						Capability: "action.fix.resolve_gaps_conversational",
						Title:      "Resolución de contacto faltante",
						Message:    jsonFirstString(q, "text", "message", "question"),
						QuestionID: jsonFirstString(q, "id", "question_id"),
					})
				}
				continue
			}
			needs = append(needs, s.inputRequestForContactDestination(node, artifacts))
		case "credentials.smtp":
			if credentialAvailableFromArtifacts("credentials.smtp", artifacts) {
				available[artifact] = true
				artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "vault_check", Node: node.ID, Payload: map[string]interface{}{"from_vault": true}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				resolved = append(resolved, artifact)
				continue
			}
			// Fallback: consultar vault directamente vía provider de credentials.smtp.check
			if m, providerName, ok := s.findProviderForCapability("credentials.smtp.check"); ok {
				if cmd, ok := m.Commands["has-smtp"]; ok {
					convID := businessVaultConvID(req.Flow.BusinessID)
					params := map[string]string{"conv_id": convID}
					args, err := cmd.ResolveArgs(params, nil, nil)
					if err == nil {
						fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
						fullArgs = append(fullArgs, args...)
						cwdRel := m.Cwd
						if cwdRel == "" {
							cwdRel = "framework-" + providerName
						}
						cwd := filepath.Join(s.rootDir, cwdRel)
						execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
						resp, err := s.scoped(convID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
						cancel()
						if err == nil && resp.ExitCode == 0 {
							var result map[string]interface{}
							if uerr := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &result); uerr == nil {
								if avail, _ := result["available"].(bool); avail {
									available[artifact] = true
									artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "vault_check", Node: node.ID, Payload: map[string]interface{}{"from_vault": true}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
									resolved = append(resolved, artifact)
									continue
								}
							}
						}
					}
				}
			}
			// Activar el provider conversacionalmente: invocar next-question para obtener
			// la primera pregunta del asistente.
			if qID, qText, providerName, ok := s.invokeProviderNextQuestion(ctx, req.Flow.BusinessID, "credentials.smtp.check"); ok {
				needs = append(needs, flowRequiredInput{
					Artifact:   "credentials.smtp",
					Kind:       "framework_question",
					Framework:  providerName,
					Capability: "credentials.smtp.check",
					Title:      "Configurar SMTP con Hosting",
					Message:    qText,
					QuestionID: qID,
				})
			} else {
				needs = append(needs, s.inputRequestForHostingConnect())
			}
		default:
			needs = append(needs, inputRequestsForMissingArtifacts(node, []string{artifact})...)
		}
	}
	return resolved, needs
}

// summarizeAuditorGaps extracts human-readable gap descriptions from data.gaps.v1.
// Groups findings by rule+endpoint+field to avoid flooding the UI with
// hundreds of individual records (e.g. "130 registros en agreements con campo
// name vacío" instead of listing each one).
func summarizeAuditorGaps(artifacts map[string]flowRunArtifact) string {
	gapArt, ok := artifacts["data.gaps.v1"]
	if !ok {
		return ""
	}
	gaps := gapsFromPayload(gapArt.Payload)
	if len(gaps) == 0 {
		return ""
	}

	// Group gaps by (rule, endpoint, field) and count occurrences.
	type groupKey struct{ rule, endpoint, field string }
	counts := map[groupKey]int{}
	var order []groupKey // preserve first-seen order
	for _, g := range gaps {
		gmap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		endpoint := jsonFirstString(gmap, "endpoint")
		field := jsonFirstString(gmap, "field")
		if endpoint == "" || field == "" {
			endpoint, field = parseEndpointFieldFromGapText(jsonFirstString(gmap, "message", "description", "label", "gap"))
		}
		key := groupKey{
			rule:     jsonFirstString(gmap, "rule", "type", "kind"),
			endpoint: endpoint,
			field:    field,
		}
		if key.rule == "" && key.endpoint == "" {
			// Fallback: use the raw description for ungroupable gaps.
			desc := jsonFirstString(gmap, "description", "message", "label", "gap")
			if desc != "" {
				fk := groupKey{rule: desc}
				if counts[fk] == 0 {
					order = append(order, fk)
				}
				counts[fk]++
			}
			continue
		}
		if counts[key] == 0 {
			order = append(order, key)
		}
		if n := jsonFirstInt(gmap, "count", "total"); n > 0 {
			counts[key] += n
		} else {
			counts[key]++
		}
	}

	if len(counts) == 0 {
		return ""
	}

	// Build concise summary lines.
	ruleLabels := map[string]string{
		"empty_required":              "campo %s vacío",
		"null_required":               "campo %s nulo",
		"missing_contact_destination": "sin email de contacto",
		"missing_contact":             "sin email de contacto",
		"schema_contact_gap":          "sin columna de email en esquema",
		"fk_orphan":                   "referencia rota en %s",
		"invalid_date":                "fecha inválida en %s",
		"stale_advance":               "anticipo sin consumir",
		"duplicate_record":            "registro duplicado",
	}

	var lines []string
	for _, key := range order {
		n := counts[key]
		if key.endpoint == "" {
			// Ungroupable gap — use the raw description as-is.
			if n > 1 {
				lines = append(lines, fmt.Sprintf("%s (×%d)", key.rule, n))
			} else {
				lines = append(lines, key.rule)
			}
			continue
		}
		tpl, ok := ruleLabels[key.rule]
		if !ok {
			tpl = key.rule
			if key.field != "" {
				tpl += " en " + key.field
			}
		} else if strings.Contains(tpl, "%s") {
			tpl = fmt.Sprintf(tpl, key.field)
		}
		if n == 1 {
			lines = append(lines, fmt.Sprintf("%s: %s", key.endpoint, tpl))
		} else {
			lines = append(lines, fmt.Sprintf("%d registros en %s: %s", n, key.endpoint, tpl))
		}
	}
	if len(lines) > 6 {
		remaining := len(lines) - 6
		lines = append(lines[:6], fmt.Sprintf("%d tipo(s) de brecha adicionales resumidos en la evidencia", remaining))
	}
	return fmt.Sprintf("Brechas de datos detectadas: %s", strings.Join(lines, "; "))
}

func gapsFromPayload(payload interface{}) []interface{} {
	if gaps, ok := payload.([]interface{}); ok {
		return gaps
	}
	obj, _ := payload.(map[string]interface{})
	if obj == nil {
		return nil
	}
	if gaps, ok := obj["gaps"].([]interface{}); ok {
		return gaps
	}
	if gaps, ok := obj["data_gaps"].([]interface{}); ok {
		return gaps
	}
	return nil
}

func parseEndpointFieldFromGapText(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	if idx := strings.LastIndex(text, ":"); idx >= 0 {
		text = strings.TrimSpace(text[idx+1:])
	}
	bracket := strings.LastIndex(text, "[")
	dot := strings.LastIndex(text, ".")
	if bracket <= 0 || dot <= bracket+1 || dot == len(text)-1 {
		return "", ""
	}
	return strings.TrimSpace(text[:bracket]), strings.TrimSpace(text[dot+1:])
}

// resolveFlowGapsIteratively attempts to resolve data gaps found by Auditor
// using other frameworks (Sabio for contacts, Mecánico for data quality fixes,
// Hosting for credentials). It emits resolution steps to the timeline and
// optionally re-runs Auditor for validation.

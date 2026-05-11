package main

import (
	"fmt"
	"strings"
	"time"

	"encoding/json"
)

func extractHumanSummary(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &data); err != nil {
		if len(stdout) > 800 {
			return stdout[:800] + "..."
		}
		return stdout
	}

	var parts []string

	// Natural language answer (sabio, critico, etc)
	if answer, ok := data["answer"].(string); ok && answer != "" {
		if len(answer) > 800 {
			answer = answer[:800] + "..."
		}
		parts = append(parts, answer)
	}
	if text, ok := data["text"].(string); ok && text != "" && len(parts) == 0 {
		if len(text) > 800 {
			text = text[:800] + "..."
		}
		parts = append(parts, text)
	}

	// Email/message draft
	if body, ok := data["body"].(string); ok && body != "" {
		at, _ := data["artifact_type"].(string)
		if strings.Contains(at, "draft") || strings.Contains(at, "message") {
			var s string
			if subject, ok := data["subject"].(string); ok && subject != "" {
				s = "Asunto: " + subject + "\n"
			}
			if to, ok := data["to"].(string); ok && to != "" {
				s += "Para: " + to + "\n"
			}
			if len(body) > 500 {
				body = body[:500] + "..."
			}
			s += "\n" + body
			parts = append(parts, s)
		}
	}

	// Selected item (foco)
	_, hasItems := data["items"].([]interface{})
	if sel, ok := data["selected"].(map[string]interface{}); ok && len(parts) == 0 && !hasItems {
		name := jsonFirstString(sel, "name", "id")
		if name != "" {
			s := "Seleccionado: " + name
			if id := jsonFirstString(sel, "entity_id", "id"); id != "" && id != name {
				s += " (ID " + id + ")"
			}
			if task, ok := data["task"].(map[string]interface{}); ok {
				if why := jsonFirstString(task, "why"); why != "" {
					s += "\nPor qué: " + why
				}
			}
			if actions, ok := data["action_options"].([]interface{}); ok && len(actions) > 0 {
				var opts []string
				for _, action := range actions {
					if m, ok := action.(map[string]interface{}); ok {
						if label := jsonFirstString(m, "label"); label != "" {
							opts = append(opts, label)
						}
					}
				}
				if len(opts) > 0 {
					s += "\nOpciones: " + strings.Join(opts, " · ")
				}
			}
			parts = append(parts, s)
		}
	}

	// Items with count (radar priority list, etc)
	if items, ok := data["items"].([]interface{}); ok && len(parts) == 0 {
		count := len(items)
		if c, ok := data["count"].(float64); ok {
			count = int(c)
		}
		s := fmt.Sprintf("Encontré %d resultados.", count)
		var topItems []string
		for i, item := range items {
			if i >= 3 {
				break
			}
			if m, ok := item.(map[string]interface{}); ok {
				name := jsonFirstString(m, "name", "deudor", "entity_name")
				if ref, ok := m["entity_ref"].(map[string]interface{}); ok {
					if n := jsonFirstString(ref, "name"); n != "" {
						name = n
					}
				}
				amount := jsonFirstNumber(m, "saldo_total", "monto", "amount", "score")
				if name != "" {
					entry := name
					if amount != "" {
						entry += " — " + amount
					}
					if score := jsonFirstNumber(m, "score"); score != "" {
						entry += " (score " + score + ")"
					}
					topItems = append(topItems, entry)
				}
			}
		}
		if len(topItems) > 0 {
			s += "\nPrincipales: " + strings.Join(topItems, ", ")
		}
		if item, ok := data["priority_item"].(map[string]interface{}); ok {
			if strategy := jsonFirstString(item, "strategy"); strategy != "" {
				s += "\nEstrategia sugerida: " + strategy
			}
			if actions, ok := item["quick_actions"].([]interface{}); ok && len(actions) > 0 {
				var labels []string
				for _, a := range actions {
					if label, ok := a.(string); ok && label != "" {
						labels = append(labels, label)
					}
				}
				if len(labels) > 0 {
					s += "\nAcciones rápidas: " + strings.Join(labels, " · ")
				}
			}
		}
		parts = append(parts, s)
	}

	// Availability check (hosting)
	if avail, ok := data["available"].(bool); ok && len(parts) == 0 {
		cap, _ := data["capability"].(string)
		if cap == "" {
			cap = "Recurso"
		}
		if avail {
			parts = append(parts, cap+" disponible y listo.")
		} else {
			parts = append(parts, cap+" no disponible.")
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

func appendUniqueSummary(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	parts := strings.Split(existing, "\n")
	for _, part := range parts {
		if strings.TrimSpace(part) == next {
			return existing
		}
	}
	if strings.Contains(existing, next) {
		return existing
	}
	return existing + "\n" + next
}

func jsonFirstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func jsonFirstNumber(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			if v == float64(int64(v)) {
				return fmt.Sprintf("%d", int64(v))
			}
			return fmt.Sprintf("%.2f", v)
		}
	}
	return ""
}

func jsonFirstInt(m map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		switch v := m[k].(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case json.Number:
			n, _ := v.Int64()
			return int(n)
		}
	}
	return 0
}

func previewText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 2000 {
		return s
	}
	return s[:2000]
}

func newFlowRunID(flowID string) string {
	flowID = safeFilePart(flowID)
	if flowID == "" {
		flowID = "flow"
	}
	return fmt.Sprintf("%s_%d", flowID, time.Now().UnixNano())
}

func safeFilePart(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

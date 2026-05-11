package main

import (
	"fmt"
	"sort"
	"strings"

	"channel/manifest"
	"encoding/base64"
	"encoding/json"
)

func setParamIfDeclared(cmd manifest.Command, params map[string]string, key, value string) {
	if commandHasParam(cmd, key) {
		params[key] = value
	}
}

func applyArtifactParamDefaults(cmd manifest.Command, params map[string]string, artifacts map[string]flowRunArtifact) {
	setFromArtifact := func(param, artifactType string, fields ...string) {
		if !commandHasParam(cmd, param) || params[param] != "" {
			return
		}
		for _, field := range fields {
			if value, ok := artifactString(artifacts[artifactType].Payload, field); ok {
				params[param] = value
				return
			}
		}
	}
	setFromArtifact("entity_type", "entity.ref.v1", "type", "entity_type")
	setFromArtifact("entity_ref", "entity.ref.v1", "id", "entity_ref", "ref")
	setFromArtifact("channel", "message.channel.v1", "channel")
	setFromArtifact("channel", "message.draft.v1", "channel")
	setFromArtifact("to", "contact.destination.v1", "destination", "value", "to")
	setFromArtifact("to", "entity_360.v1", "email", "contact_email", "to")
	setFromArtifact("to", "message.draft.v1", "to", "destination")
	if commandHasParam(cmd, "to") && params["to"] == "" {
		params["to"] = ""
	}
	setFromArtifact("subject", "message.draft.v1", "subject")
	setFromArtifact("body_b64", "message.draft.v1", "body_b64")
	if commandHasParam(cmd, "body_b64") && params["body_b64"] == "" {
		if body, ok := artifactString(artifacts["message.draft.v1"].Payload, "body"); ok {
			params["body_b64"] = base64.StdEncoding.EncodeToString([]byte(body))
		}
	}
	setFromArtifact("deudor", "collection.priority_item.v1", "deudor", "name")
	setFromArtifact("deudor", "entity.ref.v1", "name", "id")
	setFromArtifact("saldo", "collection.priority_item.v1", "saldo_total")
	setFromArtifact("dias_mora", "collection.priority_item.v1", "dias_mora_max")
	setFromArtifact("action_id", "action.selection.v1", "id", "action_id")
	setFromArtifact("action_label", "action.selection.v1", "label", "action_label")
	// Pass artifact payloads inline so Mecanico does not need hardcoded paths.
	if commandHasParam(cmd, "findings_json") && params["findings_json"] == "" {
		art := artifacts["auditor.findings.v1"]
		if art.Payload != nil {
			if raw, err := json.Marshal(art.Payload); err == nil {
				params["findings_json"] = string(raw)
			}
		}
	}
	if commandHasParam(cmd, "dataset_json") && params["dataset_json"] == "" {
		// Prefer dataset.raw.v1, fallback to external.api.dump.v1.
		art := artifacts["dataset.raw.v1"]
		if art.Payload == nil {
			art = artifacts["external.api.dump.v1"]
		}
		if art.Payload != nil {
			if raw, err := json.Marshal(art.Payload); err == nil {
				params["dataset_json"] = string(raw)
			}
		}
	}
	if commandHasParam(cmd, "dataset_artifact") && params["dataset_artifact"] == "" {
		art := artifacts["dataset.raw.v1"]
		if art.Path != "" {
			params["dataset_artifact"] = art.Path
		}
	}
	if commandHasParam(cmd, "dataset_path") && params["dataset_path"] == "" {
		art := artifacts["dataset.raw.v1"]
		if art.Path == "" {
			art = artifacts["external.api.dump.v1"]
		}
		if art.Path != "" {
			params["dataset_path"] = art.Path
		}
	}
	if commandHasParam(cmd, "source") && params["source"] == "" {
		art := artifacts["external.api.dump.v1"]
		if art.Path == "" {
			art = artifacts["dataset.raw.v1"]
		}
		if art.Path != "" {
			params["source"] = art.Path
		}
	}
	if commandHasParam(cmd, "strategy_json") && params["strategy_json"] == "" {
		art := artifacts["strategy.recommendation.v1"]
		if art.Payload != nil {
			if raw, err := json.Marshal(art.Payload); err == nil {
				params["strategy_json"] = string(raw)
			}
		}
	}
	if commandHasParam(cmd, "strategy_path") && params["strategy_path"] == "" {
		art := artifacts["strategy.recommendation.v1"]
		if art.Path != "" {
			params["strategy_path"] = art.Path
		}
	}
	if commandHasParam(cmd, "priority_list_json") && params["priority_list_json"] == "" {
		art := artifacts["collection.priority_list.v1"]
		if art.Payload != nil {
			if raw, err := json.Marshal(art.Payload); err == nil {
				params["priority_list_json"] = string(raw)
			}
		}
	}
	if commandHasParam(cmd, "tono") && params["tono"] == "" {
		params["tono"] = "formal"
	}
}

func applyFlowTestModeParamOverrides(req flowRunRequest, contract nodeContract, cmd manifest.Command, params map[string]string) {
	if !req.TestMode || !hasExternalSideEffect(contract.Policies) {
		return
	}
	recipient := flowTestRecipient(req)
	if recipient == "" {
		return
	}
	originalTo := strings.TrimSpace(params["to"])
	if commandHasParam(cmd, "to") {
		params["to"] = recipient
	}
	if commandHasParam(cmd, "subject") {
		subject := strings.TrimSpace(params["subject"])
		if !strings.HasPrefix(subject, "[TEST") {
			if originalTo == "" {
				originalTo = "(sin destinatario)"
			}
			params["subject"] = fmt.Sprintf("[TEST → %s] %s", originalTo, subject)
		}
	}
}

func flowTestRecipient(req flowRunRequest) string {
	if recipient := strings.TrimSpace(req.TestRecipient); recipient != "" {
		return recipient
	}
	return devRecipient()
}

func artifactString(payload interface{}, field string) (string, bool) {
	obj, ok := payload.(map[string]interface{})
	if !ok {
		return "", false
	}
	value, ok := obj[field]
	if !ok || value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, v != ""
	case float64, bool:
		return fmt.Sprint(v), true
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(raw), true
	}
}

func resolveFlowParamTemplate(value string, artifacts map[string]flowRunArtifact) (string, error) {
	out := value
	for {
		start := strings.Index(out, "{artifacts.")
		if start < 0 {
			return out, nil
		}
		endRel := strings.Index(out[start:], "}")
		if endRel < 0 {
			return "", fmt.Errorf("template de artifact sin cierre: %q", value)
		}
		token := out[start+len("{artifacts.") : start+endRel]
		resolved, err := resolveArtifactToken(token, artifacts)
		if err != nil {
			return "", err
		}
		out = out[:start] + resolved + out[start+endRel+1:]
	}
}

func resolveArtifactToken(token string, artifacts map[string]flowRunArtifact) (string, error) {
	types := make([]string, 0, len(artifacts))
	for typ := range artifacts {
		types = append(types, typ)
	}
	sort.Slice(types, func(i, j int) bool { return len(types[i]) > len(types[j]) })
	for _, typ := range types {
		if token != typ && !strings.HasPrefix(token, typ+".") {
			continue
		}
		value := artifacts[typ].Payload
		if token == typ {
			raw, err := json.Marshal(value)
			if err != nil {
				return "", err
			}
			return string(raw), nil
		}
		path := strings.TrimPrefix(token, typ+".")
		return artifactFieldString(value, strings.Split(path, "."))
	}
	return "", fmt.Errorf("artifact no disponible en template: %s", token)
}

func artifactFieldString(value interface{}, path []string) (string, error) {
	current := value
	for _, part := range path {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("artifact field %q no es objeto", part)
		}
		next, ok := obj[part]
		if !ok {
			return "", fmt.Errorf("artifact field no encontrado: %s", part)
		}
		current = next
	}
	switch v := current.(type) {
	case string:
		return v, nil
	case float64, bool:
		return fmt.Sprint(v), nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}

func encodeFlowRunContext(req flowRunRequest) string {
	ctx := map[string]interface{}{
		"business_id": req.Flow.BusinessID,
		"audience":    req.Flow.Audience,
		"flow_id":     req.Flow.ID,
		"dry_run":     req.DryRun,
	}
	if flowIntentDefined(req.Flow.Intent) {
		ctx["intent"] = flowIntentArtifactPayload(req.Flow)
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

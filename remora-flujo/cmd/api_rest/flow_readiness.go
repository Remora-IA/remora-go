package main

import (
	"context"
	"strings"
	"time"
)

func (s *server) recordFlowReadiness(runID, nodeID string, ready bool, needs []flowRequiredInput, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	blockers := []map[string]interface{}{}
	for _, need := range needs {
		blockers = append(blockers, map[string]interface{}{
			"required_artifact": need.Artifact,
			"kind":              need.Kind,
			"framework":         need.Framework,
			"capability":        need.Capability,
			"reason":            need.Message,
			"resolution": map[string]interface{}{
				"framework":  need.Framework,
				"capability": need.Capability,
				"mode":       resolutionModeForNeed(need),
			},
		})
	}
	payload := map[string]interface{}{
		"artifact_type": "flow.readiness.v1",
		"ready":         ready,
		"checked_at":    time.Now().UTC().Format(time.RFC3339Nano),
		"blockers":      blockers,
	}
	if ready {
		payload["message"] = "Todos los prerequisitos declarados del flujo están disponibles."
	} else {
		payload["message"] = "El flujo necesita resolver prerequisitos antes de continuar."
	}
	path := s.persistFlowArtifact(runID, nodeID+"_readiness", "flow.readiness.v1", payload)
	available["flow.readiness.v1"] = true
	artifacts["flow.readiness.v1"] = flowRunArtifact{Type: "flow.readiness.v1", Source: "flow_engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "flow.readiness.v1"
}

func resolutionModeForNeed(need flowRequiredInput) string {
	switch need.Kind {
	case "contact_email", "hosting_connect":
		return "ask_user"
	default:
		return "provide_artifact"
	}
}

func (s *server) lookupSabioContactDestination(ctx context.Context, businessID string, artifacts map[string]flowRunArtifact) (map[string]interface{}, bool) {
	entityType, entityRef, ok := contactIdentityFromArtifacts(artifacts)
	if !ok {
		return nil, false
	}
	profile := flowBusinessProfile(businessID)
	for _, typ := range contactEntityTypeCandidates(entityType) {
		select {
		case <-ctx.Done():
			return nil, false
		default:
		}
		res, err := contactosLookupProfile(profile, typ, entityRef, "email")
		if err != nil || res == nil || !res.Found || !isLikelyEmail(res.Value) {
			continue
		}
		return map[string]interface{}{
			"artifact_type": "contact.destination.v1",
			"channel":       "email",
			"destination":   res.Value,
			"value":         res.Value,
			"to":            res.Value,
			"source":        "sabio.contact-lookup",
			"entity_type":   typ,
			"entity_ref":    entityRef,
			"verified_at":   res.VerifiedAt,
		}, true
	}
	return nil, false
}

func (s *server) storeUserContactDestinationIfPossible(runID, businessID string, artifacts map[string]flowRunArtifact) {
	contact := artifacts["contact.destination.v1"]
	payload, ok := contact.Payload.(map[string]interface{})
	if !ok {
		return
	}
	if stored, _ := payload["stored_in_sabio"].(bool); stored {
		return
	}
	email := jsonFirstString(payload, "email", "to", "destination", "value")
	if !isLikelyEmail(email) {
		return
	}
	entityType, entityRef, ok := contactIdentityFromArtifacts(artifacts)
	if !ok {
		if typ := jsonFirstString(payload, "entity_type"); typ != "" {
			if ref := jsonFirstString(payload, "entity_ref", "ref", "id"); ref != "" {
				entityType, entityRef, ok = typ, ref, true
			}
		}
	}
	if !ok {
		return
	}
	profile := flowBusinessProfile(businessID)
	storedTypes := []string{}
	for _, typ := range contactEntityTypeCandidates(entityType) {
		if _, err := contactosStoreProfile(profile, typ, entityRef, "email", email, "flow_user_input"); err == nil {
			storedTypes = append(storedTypes, typ)
		}
	}
	if len(storedTypes) == 0 {
		return
	}
	payload["stored_in_sabio"] = true
	payload["stored_entity_types"] = storedTypes
	payload["entity_ref"] = entityRef
	if payload["entity_type"] == nil {
		payload["entity_type"] = entityType
	}
	path := s.persistFlowArtifact(runID, "contact_store", "contact.destination.v1", payload)
	contact.Payload = payload
	contact.Path = path
	contact.Source = strings.TrimSpace(contact.Source + "+sabio.contact-store")
	artifacts["contact.destination.v1"] = contact
}

func contactIdentityFromArtifacts(artifacts map[string]flowRunArtifact) (string, string, bool) {
	payload, _ := artifacts["entity.ref.v1"].Payload.(map[string]interface{})
	entityType := jsonFirstString(payload, "entity_type", "type", "kind")
	entityRef := jsonFirstString(payload, "entity_ref", "id", "ref", "code")
	if entityRef == "" {
		if item, ok := artifacts["collection.priority_item.v1"].Payload.(map[string]interface{}); ok {
			entityRef = jsonFirstString(item, "deudor_id", "entity_id", "id", "client_id")
			if entityType == "" {
				entityType = jsonFirstString(item, "entity_type", "type")
			}
		}
	}
	if entityType == "" {
		entityType = "entity"
	}
	return entityType, entityRef, entityRef != ""
}

func contactEntityTypeCandidates(entityType string) []string {
	base := strings.ToLower(strings.TrimSpace(entityType))
	if base == "" {
		base = "entity"
	}
	out := []string{base}
	aliases := map[string][]string{
		"customer": {"client", "debtor"},
		"client":   {"customer", "debtor"},
		"debtor":   {"client", "customer"},
		"deudor":   {"client", "customer", "debtor"},
	}
	out = append(out, aliases[base]...)
	return uniqueStrings(out)
}

func flowBusinessProfile(businessID string) string {
	if profile := strings.TrimSpace(envOr("REMORA_PROFILE", "")); profile != "" {
		return profile
	}
	if strings.TrimSpace(businessID) != "" {
		return strings.TrimSpace(businessID)
	}
	return "default"
}

func contactDestinationFromArtifacts(artifacts map[string]flowRunArtifact) (map[string]interface{}, bool) {
	for _, typ := range []string{"entity_360.v1", "message.draft.v1"} {
		payload := artifacts[typ].Payload
		for _, field := range []string{"email", "contact_email", "to", "destination"} {
			if v, ok := artifactString(payload, field); ok && isLikelyEmail(v) {
				return map[string]interface{}{
					"artifact_type": "contact.destination.v1",
					"channel":       "email",
					"destination":   v,
					"value":         v,
					"to":            v,
					"source":        typ,
				}, true
			}
		}
		if v, ok := artifactStringNested(payload, []string{"structured", "email"}); ok && isLikelyEmail(v) {
			return map[string]interface{}{
				"artifact_type": "contact.destination.v1",
				"channel":       "email",
				"destination":   v,
				"value":         v,
				"to":            v,
				"source":        typ + ".structured",
			}, true
		}
	}
	return nil, false
}

func credentialAvailableFromArtifacts(cap string, artifacts map[string]flowRunArtifact) bool {
	if artifacts[cap].Type == cap {
		return true
	}
	status, _ := artifacts["credentials.status.v1"].Payload.(map[string]interface{})
	if status == nil {
		return false
	}
	available, _ := status["available"].(bool)
	capability, _ := status["capability"].(string)
	return available && capability == cap
}

func (s *server) inputRequestForHostingConnect() flowRequiredInput {
	providerName := s.providerNameForCapability("credentials.cpanel.connect")
	return flowRequiredInput{
		Artifact:   "credentials.smtp",
		Kind:       "hosting_connect",
		Framework:  providerName,
		Capability: "credentials.cpanel.connect",
		Title:      "Conectar hosting cPanel",
		Message:    "Para enviar correos necesito una configuración asistida con Hosting. Remora pedirá el dominio, descubrirá cPanel y preparará automáticamente una casilla de envío.",
		Fields: []flowInputField{
			{Name: "domain", Label: "Dominio del negocio", Type: "text", Required: true, Placeholder: "tudominio.com"},
			{Name: "user", Label: "Usuario cPanel", Type: "text", Required: true},
			{Name: "pass", Label: "Contraseña cPanel", Type: "password", Required: true},
		},
	}
}

func (s *server) inputRequestForContactDestination(node flowNode, artifacts map[string]flowRunArtifact) flowRequiredInput {
	providerName := s.providerNameForCapability("contact.lookup")
	ctx := map[string]string{}
	if name, ok := artifactString(artifacts["entity.ref.v1"].Payload, "name"); ok {
		ctx["entity_name"] = name
	}
	if typ, ref, ok := contactIdentityFromArtifacts(artifacts); ok {
		ctx["entity_type"] = typ
		ctx["entity_ref"] = ref
	}
	message := "No encontré un correo válido para el cliente/caso. Necesito un destinatario antes de enviar."
	if gap := contactGapFromArtifacts(artifacts); gap != "" {
		ctx["data_gap"] = gap
		ctx["reported_by"] = "auditor"
		message = "Auditor marcó una brecha de datos de contacto: " + gap + " Necesito un destinatario antes de enviar."
	}
	return flowRequiredInput{
		Artifact:   "contact.destination.v1",
		Kind:       "contact_email",
		Framework:  providerName,
		Capability: "contact.lookup",
		Title:      "Falta email del destinatario",
		Message:    message,
		Fields: []flowInputField{
			{Name: "email", Label: "Email destinatario", Type: "email", Required: true, Placeholder: "cliente@empresa.com"},
		},
		Context: ctx,
	}
}

func contactGapFromArtifacts(artifacts map[string]flowRunArtifact) string {
	payload := artifacts["data.gaps.v1"].Payload
	gaps, ok := payload.([]interface{})
	if !ok {
		if m, ok := payload.(map[string]interface{}); ok {
			gaps, _ = m["data_gaps"].([]interface{})
		}
	}
	for _, raw := range gaps {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		rule := strings.ToLower(jsonFirstString(m, "rule"))
		field := strings.ToLower(jsonFirstString(m, "field"))
		msg := jsonFirstString(m, "message")
		if strings.Contains(rule, "contact") || strings.Contains(field, "email") {
			if msg != "" {
				return msg
			}
			return "contacto/email faltante"
		}
	}
	return ""
}

func inputRequestsForMissingArtifacts(node flowNode, missing []string) []flowRequiredInput {
	out := []flowRequiredInput{}
	for _, artifact := range missing {
		out = append(out, flowRequiredInput{
			Artifact: artifact,
			Kind:     "artifact",
			Title:    "Falta información para continuar",
			Message:  "El paso " + node.ID + " necesita " + artifactLabelForAPI(artifact) + ".",
		})
	}
	return out
}

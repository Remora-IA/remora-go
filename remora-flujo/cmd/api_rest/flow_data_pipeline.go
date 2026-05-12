package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func isJustInTimeDataArtifact(artifact string) bool {
	switch artifact {
	case "contact.destination.v1", "credentials.smtp", "dataset.raw.v1", "external.api.dump.v1":
		return true
	default:
		return isBusinessDataArtifact(artifact)
	}
}

func (s *server) ensureDataPipeline(ctx context.Context, runID string, req flowRunRequest, neededArtifact string, entityRef string, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	if strings.TrimSpace(neededArtifact) == "" || available[neededArtifact] {
		return true
	}
	switch neededArtifact {
	case "contact.destination.v1":
		return s.ensureContactDestinationPipeline(ctx, runID, req, entityRef, available, result, emitStep, cycleIdx)
	case "credentials.smtp":
		return s.ensureSMTPCredentialsPipeline(ctx, runID, req, available, result, emitStep, cycleIdx)
	case "dataset.raw.v1", "external.api.dump.v1":
		if available["data.sqlite_db.v1"] {
			s.ensureSabioDataMediation(ctx, runID, req, available, result, emitStep, cycleIdx, &flowStepTrigger{Capability: neededArtifact, Reason: "jit_data_pipeline"})
		}
		return available[neededArtifact] || (neededArtifact == "dataset.raw.v1" && available["external.api.dump.v1"])
	default:
		return false
	}
}

func (s *server) ensureContactDestinationPipeline(ctx context.Context, runID string, req flowRunRequest, entityRef string, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	if payload, ok := contactDestinationFromArtifacts(result.Artifacts); ok {
		s.recordContactDestination(runID, "jit_contact_from_artifacts", payload, "resolver", available, result)
		s.emitSabioPersistContactStep(runID, req.Flow.BusinessID, result.Artifacts, emitStep, cycleIdx)
		return true
	}
	if answer := strings.TrimSpace(req.Input); isLikelyEmail(answer) {
		payload := map[string]interface{}{
			"artifact_type": "contact.destination.v1",
			"channel":       "email",
			"destination":   answer,
			"value":         answer,
			"to":            answer,
			"source":        "user_input",
		}
		if typ, ref, ok := contactIdentityFromArtifacts(result.Artifacts); ok {
			payload["entity_type"] = typ
			payload["entity_ref"] = ref
		}
		s.recordContactDestination(runID, "jit_contact_user_input", payload, "user_input", available, result)
		s.emitSabioPersistContactStep(runID, req.Flow.BusinessID, result.Artifacts, emitStep, cycleIdx)
		return true
	}
	providerName := s.providerNameForCapability("contact.lookup")
	lookupStep := flowRunStep{
		Node:         "sabio_lookup_contact_" + safeFilePart(entityRef),
		Framework:    providerName,
		Capability:   "contact.lookup",
		Role:         flowRoleResolution,
		Visibility:   flowStepVisibilityInfrastructure,
		CycleIndex:   cycleIdx,
		Status:       "running",
		HumanSummary: "Sabio busca el dato de contacto necesario para continuar.",
		StartedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
	if lookupStep.Framework == "" {
		lookupStep.Framework = "sabio"
	}
	emitStep("step_start", lookupStep)
	if dest, ok := s.lookupSabioContactDestination(ctx, req.Flow.BusinessID, result.Artifacts); ok {
		s.recordContactDestination(runID, lookupStep.Node, dest, lookupStep.Framework+".contact-lookup", available, result)
		lookupStep.Status = "completed"
		lookupStep.ArtifactTypes = []string{"contact.destination.v1"}
		lookupStep.HumanSummary = fmt.Sprintf("Sabio encontró un email para %s.", entityDisplayName(result.Artifacts))
		finished := finishFlowRunStep(lookupStep)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return true
	}
	lookupStep.Status = "completed"
	lookupStep.HumanSummary = fmt.Sprintf("Sabio no encontró un email guardado para %s.", entityDisplayName(result.Artifacts))
	finished := finishFlowRunStep(lookupStep)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)

	gap := dataGap{Kind: "missing_contact_destination", Description: "Falta email de contacto para la entidad actual.", Field: "email"}
	questions, hasQuestions := s.invokeMecanicoResolveGaps(ctx, runID, req, []dataGap{gap}, result.Artifacts, available, result, emitStep, cycleIdx)
	if !hasQuestions {
		need := s.inputRequestForContactDestination(flowNode{ID: "jit_contact", Framework: providerName, Capability: "contact.lookup"}, result.Artifacts)
		need.Kind = "conversational_question"
		need.Framework = s.providerNameForCapabilityOrCommand("action.fix.resolve_gaps_conversational", "resolve-gaps")
		need.Capability = "action.fix.resolve_gaps_conversational"
		need.Message = fmt.Sprintf("Para enviar el cobro a %s necesito su email. ¿Cuál es?", entityDisplayName(result.Artifacts))
		need.EntityRef = entityRef
		need.GapType = gap.Kind
		need.Field = "email"
		result.NeedsInput = append(result.NeedsInput, need)
	} else {
		q := questions[0]
		result.NeedsInput = append(result.NeedsInput, flowRequiredInput{
			Artifact:   "contact.destination.v1",
			Kind:       "conversational_question",
			Framework:  s.providerNameForCapabilityOrCommand("action.fix.resolve_gaps_conversational", "resolve-gaps"),
			Capability: "action.fix.resolve_gaps_conversational",
			Title:      "Falta email de contacto",
			Message:    jsonFirstString(q, "text", "message", "question"),
			QuestionID: jsonFirstString(q, "id", "question_id"),
			EntityRef:  entityRef,
			GapType:    firstNonEmptyPipelineString(jsonFirstString(q, "gap_type"), gap.Kind),
			Field:      firstNonEmptyPipelineString(jsonFirstString(q, "field"), "email"),
			Context:    contactNeedContext(result.Artifacts),
		})
	}
	result.Status = "needs_input"
	s.recordFlowReadiness(runID, "jit_contact_pipeline", false, result.NeedsInput, available, result.Artifacts)
	return false
}

func (s *server) ensureSMTPCredentialsPipeline(ctx context.Context, runID string, req flowRunRequest, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	if credentialAvailableFromArtifacts("credentials.smtp", result.Artifacts) {
		available["credentials.smtp"] = true
		return true
	}
	if answer := strings.TrimSpace(req.Input); answer != "" {
		if responseText, ok := s.ingestProviderAnswer(ctx, req.Flow.BusinessID, "credentials.smtp.check", "", answer); ok {
			step := flowRunStep{
				Node:          "hosting_persist_smtp",
				Framework:     "hosting",
				Capability:    "credentials.smtp.check",
				Role:          flowRoleResolution,
				Visibility:    flowStepVisibilityInfrastructure,
				CycleIndex:    cycleIdx,
				Status:        "completed",
				HumanSummary:  hostingCredentialVerificationSummary(responseText),
				ArtifactTypes: []string{"credentials.smtp.input.v1"},
				StartedAt:     time.Now().UTC().Format(time.RFC3339Nano),
				FinishedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			}
			emitStep("step_complete", step)
			result.Timeline = append(result.Timeline, step)
		}
	}
	if s.checkSMTPCredentialsAvailable(ctx, runID, req.Flow.BusinessID, available, result, emitStep, cycleIdx) {
		if verifyErr := s.verifySMTPCredentialsReal(ctx, runID, req.Flow.BusinessID, available, result, emitStep, cycleIdx); verifyErr != "" {
			result.Artifacts["credentials.smtp.verification.v1"] = flowRunArtifact{
				Type: "credentials.smtp.verification.v1", Source: "hosting", Node: "hosting_verify_smtp",
				Payload:   map[string]interface{}{"verified": false, "error": verifyErr},
				CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			delete(available, "credentials.smtp")
		} else {
			return true
		}
	}
	gap := dataGap{Kind: "credentials_smtp", Description: "Faltan credenciales de correo para enviar emails.", Field: "credentials.smtp"}
	questions, hasQuestions := s.invokeMecanicoResolveGaps(ctx, runID, req, []dataGap{gap}, result.Artifacts, available, result, emitStep, cycleIdx)
	message := "Para poder enviar correos necesito acceso a tu cuenta de email. ¿Cuál es tu proveedor y los datos SMTP?"
	if last := lastHostingCredentialFailure(result.Artifacts); last != "" {
		message = "No pude verificar esas credenciales de correo: " + last + "\nProbemos de nuevo. Enviame host SMTP, usuario, contraseña y remitente."
	}
	questionID := ""
	field := "credentials.smtp"
	if hasQuestions {
		rawQField := jsonFirstString(questions[0], "field")
		qField := firstNonEmptyPipelineString(rawQField, field)
		qGap := jsonFirstString(questions[0], "gap_type")
		if strings.Contains(strings.ToLower(rawQField+" "+qGap), "smtp") || strings.Contains(strings.ToLower(rawQField+" "+qGap), "credential") {
			message = jsonFirstString(questions[0], "text", "message", "question")
			questionID = jsonFirstString(questions[0], "id", "question_id")
			field = qField
		}
	}
	result.NeedsInput = append(result.NeedsInput, flowRequiredInput{
		Artifact:   "credentials.smtp",
		Kind:       "conversational_question",
		Framework:  s.providerNameForCapabilityOrCommand("action.fix.resolve_gaps_conversational", "resolve-gaps"),
		Capability: "action.fix.resolve_gaps_conversational",
		Title:      "Faltan credenciales SMTP",
		Message:    message,
		QuestionID: questionID,
		GapType:    gap.Kind,
		Field:      field,
	})
	result.Status = "needs_input"
	s.recordFlowReadiness(runID, "jit_smtp_pipeline", false, result.NeedsInput, available, result.Artifacts)
	return false
}

func (s *server) checkSMTPCredentialsAvailable(ctx context.Context, runID, businessID string, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	if m, providerName, ok := s.findProviderForCapability("credentials.smtp.check"); ok {
		node := flowNode{ID: "hosting_check_smtp", Framework: providerName, Capability: "credentials.smtp.check", Role: flowRoleResolution}
		recordDynamicFlowNode(result, node)
		contract, err := resolveFlowNodeContract(node, m)
		if err == nil {
			req := flowRunRequest{Flow: flowManifest{BusinessID: businessID}}
			step := flowRunStep{Node: node.ID, Framework: providerName, Capability: node.Capability, Command: contract.Command, Role: flowRoleResolution, Visibility: flowStepVisibilityInfrastructure, CycleIndex: cycleIdx, Status: "running", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			emitStep("step_start", step)
			resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
			if execErr == nil && resp.Success && resp.ExitCode == 0 {
				step.Status = "completed"
				step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
			} else {
				step.Status = "completed"
				step.HumanSummary = "No hay credenciales SMTP guardadas todavía."
			}
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
		}
	}
	return available["credentials.smtp"]
}

// verifySMTPCredentialsReal invokes Hosting's verify-smtp capability to do
// a real SMTP login. Returns "" on success, or an error description on failure.
func (s *server) verifySMTPCredentialsReal(ctx context.Context, runID, businessID string, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) string {
	m, providerName, ok := s.findProviderForCapability("credentials.smtp.verify")
	if !ok {
		return ""
	}
	node := flowNode{ID: "hosting_verify_smtp", Framework: providerName, Capability: "credentials.smtp.verify", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		return ""
	}
	req := flowRunRequest{Flow: flowManifest{BusinessID: businessID}}
	step := flowRunStep{Node: node.ID, Framework: providerName, Capability: node.Capability, Command: contract.Command, Role: flowRoleResolution, Visibility: flowStepVisibilityInfrastructure, CycleIndex: cycleIdx, Status: "running", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	emitStep("step_start", step)
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.HumanSummary = "Error al verificar credenciales SMTP: " + execErr.Error()
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return step.HumanSummary
	}
	step.Status = "completed"
	step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Stdout), &parsed); err == nil {
		if verified, _ := parsed["verified"].(bool); verified {
			step.HumanSummary = "Credenciales SMTP verificadas correctamente."
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			return ""
		}
		errMsg := jsonFirstString(parsed, "error")
		if errMsg == "" {
			errMsg = "Las credenciales SMTP no son válidas."
		}
		step.HumanSummary = "Verificación SMTP falló: " + errMsg
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return errMsg
	}
	step.HumanSummary = "Verificación SMTP completada."
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	return ""
}

func hostingCredentialVerificationSummary(responseText string) string {
	responseText = strings.TrimSpace(responseText)
	if responseText == "" || responseText == "ok" {
		return "Hosting verificó internamente las credenciales de correo."
	}
	return "Hosting verificó las credenciales de correo: " + responseText
}

func lastHostingCredentialFailure(artifacts map[string]flowRunArtifact) string {
	for _, key := range []string{"credentials.smtp.verification.v1", "credentials.status.v1", "credentials.smtp.input.v1"} {
		if art, ok := artifacts[key]; ok {
			if payload, ok := art.Payload.(map[string]interface{}); ok {
				if err := jsonFirstString(payload, "error", "reason", "message"); err != "" {
					return err
				}
				if avail, ok := payload["available"].(bool); ok && !avail {
					return "las credenciales todavía no están disponibles"
				}
			}
		}
	}
	return ""
}

func (s *server) recordContactDestination(runID, nodeID string, payload map[string]interface{}, source string, available map[string]bool, result *flowRunResult) {
	path := s.persistFlowArtifact(runID, nodeID, "contact.destination.v1", payload)
	available["contact.destination.v1"] = true
	result.Artifacts["contact.destination.v1"] = flowRunArtifact{Type: "contact.destination.v1", Source: source, Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
}

func (s *server) emitSabioPersistContactStep(runID, businessID string, artifacts map[string]flowRunArtifact, emitStep func(string, flowRunStep), cycleIdx int) {
	before := artifacts["contact.destination.v1"]
	s.storeUserContactDestinationIfPossible(runID, businessID, artifacts)
	after := artifacts["contact.destination.v1"]
	payload, _ := after.Payload.(map[string]interface{})
	if before.Source == after.Source || payload == nil {
		return
	}
	entity := jsonFirstString(payload, "entity_ref", "ref", "id")
	if entity == "" {
		entity = "actual"
	}
	step := flowRunStep{
		Node:          "sabio_persist_contact_" + safeFilePart(entity),
		Framework:     "sabio",
		Capability:    "contact.store",
		Role:          flowRoleResolution,
		Visibility:    flowStepVisibilityInfrastructure,
		CycleIndex:    cycleIdx,
		Status:        "completed",
		HumanSummary:  "Dato guardado para " + entityDisplayName(artifacts) + ".",
		ArtifactTypes: []string{"contact.record.v1", "contact.destination.v1"},
		StartedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		FinishedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_complete", step)
}

func entityDisplayName(artifacts map[string]flowRunArtifact) string {
	if name, ok := artifactString(artifacts["entity.ref.v1"].Payload, "name"); ok {
		return name
	}
	if _, ref, ok := contactIdentityFromArtifacts(artifacts); ok && ref != "" {
		return ref
	}
	return "esta empresa"
}

func contactNeedContext(artifacts map[string]flowRunArtifact) map[string]string {
	ctx := map[string]string{}
	if name := entityDisplayName(artifacts); name != "" {
		ctx["entity_name"] = name
	}
	if typ, ref, ok := contactIdentityFromArtifacts(artifacts); ok {
		ctx["entity_type"] = typ
		ctx["entity_ref"] = ref
	}
	ctx["searched_by"] = "sabio"
	ctx["reported_by"] = "auditor"
	return ctx
}

func firstNonEmptyPipelineString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

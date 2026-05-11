package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"channel/manifest"
	"encoding/json"
	"path/filepath"
)

func (s *server) resolveFlowGapsIteratively(ctx context.Context, runID string, req flowRunRequest, auditorNode flowNode, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) {
	const maxResolutionPasses = 2
	for pass := 0; pass < maxResolutionPasses; pass++ {
		gaps := parseDataGaps(result.Artifacts)
		if len(gaps) == 0 {
			break
		}
		anyResolved := false

		// 1. Contact/email gaps: try Sabio contact-lookup once for all contact gaps
		hasContactGap := false
		for _, gap := range gaps {
			if strings.Contains(strings.ToLower(gap.Kind), "contact") || strings.Contains(strings.ToLower(gap.Kind), "email") {
				hasContactGap = true
				break
			}
		}
		if hasContactGap && !available["contact.destination.v1"] {
			contactProvider, contactProviderName, contactProviderOK := s.findProviderForCapability("contact.lookup")
			if dest, ok := s.lookupSabioContactDestination(ctx, req.Flow.BusinessID, result.Artifacts); ok && contactProviderOK {
				resNode := flowNode{ID: fmt.Sprintf("gap_resolve_contact_%d", pass), Framework: contactProviderName, Capability: "contact.lookup", Role: flowRoleResolution}
				recordDynamicFlowNode(result, resNode)
				available["contact.destination.v1"] = true
				path := s.persistFlowArtifact(runID, "gap_resolve_contact", "contact.destination.v1", dest)
				result.Artifacts["contact.destination.v1"] = flowRunArtifact{Type: "contact.destination.v1", Source: contactProviderName + ".contact-lookup", Node: "gap_resolve_contact", Path: path, Payload: dest, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				resStep := flowRunStep{
					Node:           resNode.ID,
					Framework:      contactProviderName,
					Capability:     "contact.lookup",
					Role:           flowRoleResolution,
					ResolutionMode: resolutionModeForCapability(contactProvider, "contact.lookup"),
					CycleIndex:     cycleIdx,
					Status:         "completed",
					HumanSummary:   fmt.Sprintf("%s resolvió el contacto faltante: %s", contactProviderName, jsonFirstString(dest, "to", "destination", "value")),
					ArtifactTypes:  []string{"contact.destination.v1"},
					StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
					FinishedAt:     time.Now().UTC().Format(time.RFC3339Nano),
				}
				emitStep("step_complete", resStep)
				result.Timeline = append(result.Timeline, resStep)
				anyResolved = true
			} else if s.flowRequiresArtifact(req.Flow, "contact.destination.v1") {
				// Antes de poner needs_input directo, delegamos a Mecánico como
				// resolvedor conversacional estándar de gaps.
				if questions, hasQuestions := s.invokeMecanicoResolveGaps(ctx, runID, req, gaps, result.Artifacts, available, result, emitStep, cycleIdx); hasQuestions {
					questionProviderName := s.providerNameForCapabilityOrCommand("action.fix.resolve_gaps_conversational", "resolve-gaps")
					for _, q := range questions {
						qText := ""
						if t, ok := q["text"].(string); ok {
							qText = t
						}
						qID := ""
						if id, ok := q["id"].(string); ok {
							qID = id
						}
						gapType := ""
						if gt, ok := q["gap_type"].(string); ok {
							gapType = gt
						}
						result.NeedsInput = append(result.NeedsInput, flowRequiredInput{
							Artifact:   "contact.destination.v1",
							Kind:       "conversational_question",
							Framework:  questionProviderName,
							Capability: "action.fix.resolve_gaps_conversational",
							Title:      "Resolución de gap: " + gapType,
							Message:    qText,
							QuestionID: qID,
							EntityRef:  jsonFirstString(q, "entity_ref"),
							GapType:    gapType,
							Field:      jsonFirstString(q, "field"),
						})
					}
					result.Status = "needs_input"
					s.recordFlowReadiness(runID, auditorNode.ID, false, result.NeedsInput, available, result.Artifacts)
					return
				}
				// Fallback: needs_input directo tradicional
				result.Status = "needs_input"
				result.NeedsInput = append(result.NeedsInput, s.inputRequestForContactDestination(auditorNode, result.Artifacts))
				s.recordFlowReadiness(runID, auditorNode.ID, false, result.NeedsInput, available, result.Artifacts)
				return
			}
		}

		// 2. Hybrid data-quality gaps: collect all gap kinds that the provider can handle and invoke once
		remediationGaps := []dataGap{}
		var remediationStrategy gapResolution
		var remediationProvider *manifest.Manifest
		remediationProviderName := ""
		for _, gap := range gaps {
			strategy, ok := findGapResolution(gap.Kind)
			if !ok {
				continue
			}
			m, providerName, providerOK := s.findProviderForCapability(strategy.Capability)
			if !providerOK {
				continue
			}
			if resolutionModeForCapability(m, strategy.Capability) == resolutionHybrid {
				remediationGaps = append(remediationGaps, gap)
				remediationStrategy = strategy
				remediationProvider = m
				remediationProviderName = providerName
			}
		}
		if len(remediationGaps) > 0 {
			m := remediationProvider
			if m != nil {
				resNode := flowNode{
					ID:         fmt.Sprintf("gap_resolve_%s_%d", safeFilePart(remediationProviderName), pass),
					Framework:  remediationProviderName,
					Capability: remediationStrategy.Capability,
					Role:       flowRoleResolution,
				}
				recordDynamicFlowNode(result, resNode)
				contract, err := resolveFlowNodeContract(resNode, m)
				if err == nil {
					canRun := true
					for _, reqArt := range contract.Requires {
						if reqArt != "" && !available[reqArt] {
							canRun = false
							break
						}
					}
					if canRun {
						resStep := flowRunStep{
							Node:           resNode.ID,
							Framework:      remediationProviderName,
							Capability:     remediationStrategy.Capability,
							Command:        contract.Command,
							Role:           flowRoleResolution,
							ResolutionMode: resolutionModeFromPolicies(contract.Policies),
							CycleIndex:     cycleIdx,
							Status:         "running",
							StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
						}
						emitStep("step_start", resStep)
						resp, execErr := s.executeFlowNode(ctx, runID, req, resNode, contract, result.Artifacts)
						if execErr != nil {
							resStep.Status = "failed"
							resStep.Error = execErr.Error()
							resStep.HumanSummary = fmt.Sprintf("%s no pudo generar propuestas de fix: %s", remediationProviderName, execErr.Error())
						} else if !resp.Success || resp.ExitCode != 0 {
							resStep.Status = "failed"
							resStep.Error = strings.TrimSpace(resp.Error)
							if resStep.Error == "" {
								resStep.Error = strings.TrimSpace(resp.Stderr)
							}
							resStep.HumanSummary = remediationProviderName + " intentó resolver pero no pudo completar la propuesta."
						} else {
							resStep.Status = "completed"
							resStep.ArtifactTypes = s.recordNodeArtifacts(runID, resNode.ID, contract, resp.Stdout, available, result.Artifacts)
							resStep.HumanSummary = extractHumanSummary(resp.Stdout)
							if resStep.HumanSummary == "" {
								resStep.HumanSummary = fmt.Sprintf("%s generó propuestas de remediación para %d brechas.", remediationProviderName, len(remediationGaps))
							}
							resStep.ExitCode = resp.ExitCode
							resStep.DurationMs = resp.DurationMs
							anyResolved = true
							if containsString(resStep.ArtifactTypes, "mecanico.proposals.v1") || containsString(resStep.ArtifactTypes, "mecanico.proposal.v1") {
								requestMecanicoProposalApproval(result, resStep)
							}
						}
						finished := finishFlowRunStep(resStep)
						emitStep("step_complete", finished)
						result.Timeline = append(result.Timeline, finished)
						if result.Status == "needs_input" {
							return
						}
					}
				}
			}
		}

		if !anyResolved {
			break // No progress, stop iterating
		}
		// Re-run Auditor for validation after resolutions (only on first pass)
		if pass == 0 && s.allManifests[auditorNode.Framework] != nil {
			reAuditNode := flowNode{
				ID:         auditorNode.ID + "_revalidation",
				Framework:  auditorNode.Framework,
				Capability: auditorNode.Capability,
				Role:       flowRoleResolution,
			}
			recordDynamicFlowNode(result, reAuditNode)
			contract, err := resolveFlowNodeContract(reAuditNode, s.allManifests[auditorNode.Framework])
			if err != nil {
				break
			}
			canRun := true
			for _, reqArt := range contract.Requires {
				if reqArt != "" && !available[reqArt] {
					canRun = false
					break
				}
			}
			if !canRun {
				break
			}
			reStep := flowRunStep{
				Node:           reAuditNode.ID,
				Framework:      auditorNode.Framework,
				Capability:     auditorNode.Capability,
				Command:        contract.Command,
				Role:           flowRoleResolution,
				ResolutionMode: resolutionModeFromPolicies(contract.Policies),
				CycleIndex:     cycleIdx,
				Status:         "running",
				StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
			}
			emitStep("step_start", reStep)
			resp, execErr := s.executeFlowNode(ctx, runID, req, reAuditNode, contract, result.Artifacts)
			if execErr != nil {
				reStep.Status = "failed"
				reStep.Error = execErr.Error()
			} else {
				reStep.Status = "completed"
				reStep.ArtifactTypes = s.recordNodeArtifacts(runID, reAuditNode.ID, contract, resp.Stdout, available, result.Artifacts)
				reStep.HumanSummary = "Re-validación de Auditor tras resolución de gaps."
				if newSummary := summarizeAuditorGaps(result.Artifacts); newSummary != "" {
					reStep.HumanSummary += "\n" + newSummary
				}
				reStep.ExitCode = resp.ExitCode
				reStep.DurationMs = resp.DurationMs
			}
			finished := finishFlowRunStep(reStep)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
		}
	}
}

func (s *server) invokeMecanicoResolveGaps(ctx context.Context, runID string, req flowRunRequest, gaps []dataGap, artifacts map[string]flowRunArtifact, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) ([]map[string]interface{}, bool) {
	const capability = "action.fix.resolve_gaps_conversational"
	m, providerName, ok := s.findProviderForCapability(capability)
	if !ok {
		m, providerName, ok = s.findProviderWithCommand("resolve-gaps")
		if !ok {
			return nil, false
		}
	}
	cmd, ok := m.Commands["resolve-gaps"]
	if !ok {
		return nil, false
	}
	gapPayload := []map[string]interface{}{}
	for _, g := range gaps {
		gapPayload = append(gapPayload, map[string]interface{}{"type": g.Kind, "description": g.Description, "field": g.Field})
	}
	gapsJSON, _ := json.Marshal(gapPayload)
	params := map[string]string{"data_gaps_json": string(gapsJSON)}
	if findingsArt, ok := artifacts["auditor.findings.v1"]; ok {
		if payload, err := json.Marshal(findingsArt.Payload); err == nil {
			params["findings_json"] = string(payload)
		}
	}
	if entityArt, ok := artifacts["entity.ref.v1"]; ok {
		if payload, err := json.Marshal(entityArt.Payload); err == nil {
			params["entity_ref_json"] = string(payload)
		}
	}
	for _, p := range cmd.Params {
		if _, ok := params[p]; !ok {
			params[p] = ""
		}
	}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return nil, false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + providerName
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	resp, err := s.scoped(runID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
	cancel()
	if err != nil {
		return nil, false
	}
	if resp.ExitCode != 0 {
		return nil, false
	}
	var parsed map[string]interface{}
	if uerr := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &parsed); uerr != nil {
		return nil, false
	}
	qs, _ := parsed["questions"].([]interface{})
	questions := []map[string]interface{}{}
	for _, q := range qs {
		if qmap, ok := q.(map[string]interface{}); ok {
			questions = append(questions, qmap)
		}
	}
	// Record artifacts from the gap resolution provider.
	node := flowNode{ID: fmt.Sprintf("%s_resolve_gaps_%d", safeFilePart(providerName), cycleIdx), Framework: providerName, Capability: capability, Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	_, auditProviderName, _ := s.findProviderForCapability("data.quality.audit")
	if artType, _ := parsed["artifact_type"].(string); artType != "" {
		path := s.persistFlowArtifact(runID, node.ID, artType, parsed)
		artifacts[artType] = flowRunArtifact{Type: artType, Source: providerName + ".resolve-gaps", Node: node.ID, Path: path, Payload: parsed, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if !available[artType] {
			available[artType] = true
		}
	}
	step := flowRunStep{
		Node:           node.ID,
		Framework:      providerName,
		Capability:     capability,
		Role:           flowRoleResolution,
		Visibility:     flowStepVisibilityInfrastructure,
		TriggeredBy:    triggerForFlowNode(flowNode{Framework: auditProviderName, Capability: "data.quality.audit"}, "gap_resolution"),
		ResolutionMode: resolutionModeForCapability(m, node.Capability),
		CycleIndex:     cycleIdx,
		Status:         "completed",
		HumanSummary:   fmt.Sprintf("%s generó %d preguntas para resolver gaps conversacionalmente.", providerName, len(questions)),
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		FinishedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_complete", step)
	result.Timeline = append(result.Timeline, step)
	return questions, len(questions) > 0
}

func (s *server) findProviderWithCommand(command string) (*manifest.Manifest, string, bool) {
	for name, m := range s.allManifests {
		if m == nil {
			continue
		}
		if _, ok := m.Commands[command]; ok {
			return m, name, true
		}
	}
	return nil, "", false
}

func (s *server) providerNameForCapabilityOrCommand(capability, command string) string {
	if _, providerName, ok := s.findProviderForCapability(capability); ok {
		return providerName
	}
	if _, providerName, ok := s.findProviderWithCommand(command); ok {
		return providerName
	}
	return ""
}

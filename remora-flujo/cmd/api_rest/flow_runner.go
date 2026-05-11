package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *server) runFlowManifest(ctx context.Context, req flowRunRequest, onStep flowStepCallback) flowRunResult {
	prepareFlowManifestLifecycle(&req.Flow, s.allManifests)
	runID := newFlowRunID(req.Flow.ID)
	createdAt := time.Now().UTC()
	var businessArtifacts []string
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		businessArtifacts = s.businessArtifacts(req.Flow.BusinessID).Artifacts
	}
	autoArtifacts := uniqueStrings(append(businessArtifacts, req.FixtureArtifacts...))
	if flowIntentDefined(req.Flow.Intent) {
		autoArtifacts = append(autoArtifacts, "flow.intent.v1")
	}
	validation := validateFlowManifestWithArtifacts(req.Flow, s.allManifests, autoArtifacts)
	nodeOrder, err := flowExecutionOrder(req.Flow)
	if err != nil {
		nodeOrder = req.Flow.Nodes
	}
	result := flowRunResult{
		RunID:             runID,
		FlowID:            req.Flow.ID,
		Status:            "completed",
		Valid:             validation.Valid,
		DryRun:            req.DryRun,
		Approved:          req.Approved,
		TestMode:          req.TestMode,
		TestRecipient:     flowTestRecipient(req),
		BusinessID:        req.Flow.BusinessID,
		BusinessArtifacts: sortedStringSlice(businessArtifacts),
		ExecutionOrder:    make([]string, 0, len(nodeOrder)),
		Artifacts:         map[string]flowRunArtifact{},
		Validation:        validation,
		Warnings:          validation.Warnings,
		CreatedAt:         createdAt.Format(time.RFC3339Nano),
	}
	if !validation.Valid {
		result.Status = "invalid"
		result.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_ = s.persistFlowRun(result)
		return result
	}

	available := systemFlowArtifacts()
	for artifact := range available {
		result.Artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "system", CreatedAt: result.CreatedAt}
	}
	for _, artifact := range append(autoArtifacts, req.Flow.ProvidedArtifacts...) {
		artifact = strings.TrimSpace(artifact)
		if artifact == "" {
			continue
		}
		available[artifact] = true
		result.Artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "provided", CreatedAt: result.CreatedAt}
	}
	if flowIntentDefined(req.Flow.Intent) {
		payload := flowIntentArtifactPayload(req.Flow)
		result.Artifacts["flow.intent.v1"] = flowRunArtifact{Type: "flow.intent.v1", Source: "flow_manifest", Payload: payload, CreatedAt: result.CreatedAt}
		available["flow.intent.v1"] = true
	}
	for artifact, payload := range req.InitialArtifacts {
		artifact = strings.TrimSpace(artifact)
		if artifact == "" {
			continue
		}
		available[artifact] = true
		result.Artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "initial_payload", Payload: payload, CreatedAt: result.CreatedAt}
	}
	s.storeUserContactDestinationIfPossible(runID, req.Flow.BusinessID, result.Artifacts)

	totalSteps := len(nodeOrder)
	emitStep := func(event string, step flowRunStep) {
		if onStep != nil {
			onStep(event, step, totalSteps)
		}
	}

	maxCycles := req.MaxCycles
	if maxCycles <= 0 {
		maxCycles = 1
	}
	cyclesDone := 0
	cycleCompletedThisPass := false
	sabioMediated := false
	dataValidationApplied := false
	mecanicoApprovalApplied := false

cycleStart:
	cycleCompletedThisPass = false
	sabioMediated = false
	dataValidationApplied = false
	for _, node := range nodeOrder {
		if cyclesDone > 0 && node.Role == flowRoleBootstrap {
			result.ExecutionOrder = append(result.ExecutionOrder, node.ID+"_skip_cycle")
			continue
		}
		if s.shouldSkipInstalledAnalysis(req, node) {
			result.ExecutionOrder = append(result.ExecutionOrder, node.ID+"_installed")
			step := flowRunStep{
				Node:           node.ID,
				Framework:      node.Framework,
				Capability:     node.Capability,
				Role:           node.Role,
				ResolutionMode: resolutionAutonomous,
				CycleIndex:     cyclesDone,
				Status:         "skipped",
				HumanSummary:   "La configuración de análisis de Radar ya está instalada para este negocio; se reutiliza el plan tangible existente.",
				ArtifactTypes:  []string{s.recordFlowInstallation(runID, node.ID, req.Flow.BusinessID, available, result.Artifacts)},
				StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
				FinishedAt:     time.Now().UTC().Format(time.RFC3339Nano),
			}
			emitStep("step_complete", step)
			result.Timeline = append(result.Timeline, step)
			continue
		}
		if !sabioMediated && s.shouldMediateSabioDataBeforeNode(node, available) {
			s.ensureSabioDataMediation(ctx, runID, req, available, &result, emitStep, cyclesDone, triggerForFlowNode(node, "dataset_required"))
			sabioMediated = true
		}
		if !mecanicoApprovalApplied && shouldApplyMecanicoProposals(result.Artifacts) && (available["external.api.dump.v1"] || available["dataset.raw.v1"]) {
			mecanicoApprovalApplied = true
			if !s.applyApprovedMecanicoProposals(ctx, runID, req, available, &result, emitStep, cyclesDone) {
				break
			}
		}
		result.ExecutionOrder = append(result.ExecutionOrder, node.ID)
		step := flowRunStep{
			Node:           node.ID,
			Framework:      node.Framework,
			Capability:     node.Capability,
			Role:           node.Role,
			ResolutionMode: resolutionAutonomous,
			CycleIndex:     cyclesDone,
			Status:         "running",
			StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		}
		emitStep("step_start", step)
		m := s.allManifests[node.Framework]
		if m == nil {
			step.Status = "failed"
			step.Error = "framework no encontrado: " + node.Framework
			result.Status = "failed"
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		contract, contractErr := resolveFlowNodeContract(node, m)
		if contractErr != nil {
			step.Status = "failed"
			step.Error = contractErr.Error()
			result.Status = "failed"
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		step.Command = contract.Command
		step.Inputs = uniqueStrings(contract.Inputs)
		step.Requires = uniqueStrings(contract.Requires)
		step.Outputs = uniqueStrings(contract.Outputs)
		step.Produces = uniqueStrings(contract.Produces)
		step.Policies = uniqueStrings(contract.Policies)
		step.ResolutionMode = resolutionModeFromPolicies(contract.Policies)
		dataValidationRequired := !dataValidationApplied && s.shouldRunLayeredDataValidation(node, contract, available)
		preflightRequired := s.shouldRunPreflightAudit(node, contract, available)
		if dataValidationRequired || preflightRequired {
			if !s.ensureFlowPreflightAudit(ctx, runID, req, node, available, &result, emitStep, cyclesDone) {
				step.Status = result.Status
				if step.Status == "" || step.Status == "completed" {
					step.Status = "needs_input"
				}
				if step.HumanSummary == "" {
					step.HumanSummary = "Auditor detuvo el paso hasta resolver los prerequisitos del flujo."
				}
				finished := finishFlowRunStep(step)
				emitStep("step_complete", finished)
				result.Timeline = append(result.Timeline, finished)
				break
			}
			if dataValidationRequired {
				dataValidationApplied = true
			}
		}
		for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
			if reqArtifact != "" && !available[reqArtifact] {
				step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
			}
		}
		if len(step.MissingArtifacts) > 0 {
			entityRef := ""
			if _, ref, ok := contactIdentityFromArtifacts(result.Artifacts); ok {
				entityRef = ref
			}
			remaining := []string{}
			for _, missingArtifact := range step.MissingArtifacts {
				if isJustInTimeDataArtifact(missingArtifact) && s.ensureDataPipeline(ctx, runID, req, missingArtifact, entityRef, available, &result, emitStep, cyclesDone) {
					step.ArtifactTypes = append(step.ArtifactTypes, missingArtifact)
					continue
				}
				remaining = append(remaining, missingArtifact)
			}
			step.MissingArtifacts = remaining
			if result.Status == "needs_input" {
				step.Status = "needs_input"
				readinessType := s.recordFlowReadiness(runID, node.ID, false, result.NeedsInput, available, result.Artifacts)
				step.ArtifactTypes = append(step.ArtifactTypes, readinessType)
				finished := finishFlowRunStep(step)
				emitStep("step_complete", finished)
				result.Timeline = append(result.Timeline, finished)
				break
			}
		}
		if len(step.MissingArtifacts) > 0 {
			resolved, needs := s.resolveMissingFlowArtifacts(ctx, runID, req, node, step.MissingArtifacts, available, result.Artifacts, &result, emitStep, cyclesDone)
			if len(resolved) > 0 {
				step.ArtifactTypes = append(step.ArtifactTypes, resolved...)
			}
			step.MissingArtifacts = nil
			for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
				if reqArtifact != "" && !available[reqArtifact] {
					step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
				}
			}
			if len(step.MissingArtifacts) == 0 {
				step.Status = "running"
			} else {
				step.Status = "needs_input"
				result.Status = "needs_input"
				result.NeedsInput = append(result.NeedsInput, needs...)
				if len(result.NeedsInput) == 0 {
					result.NeedsInput = append(result.NeedsInput, inputRequestsForMissingArtifacts(node, step.MissingArtifacts)...)
				}
				readinessType := s.recordFlowReadiness(runID, node.ID, false, result.NeedsInput, available, result.Artifacts)
				step.ArtifactTypes = append(step.ArtifactTypes, readinessType)
				finished := finishFlowRunStep(step)
				emitStep("step_complete", finished)
				result.Timeline = append(result.Timeline, finished)
				break
			}
		}
		if len(step.MissingArtifacts) > 0 {
			step.Status = "blocked"
			result.Status = "blocked"
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		if needsApproval, reason := nodeRequiresRuntimeApproval(node, contract, req); needsApproval {
			step.Status = "awaiting_approval"
			step.Error = reason
			step.HumanSummary = runtimeApprovalSummary(node, contract)
			result.Status = "needs_approval"
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
		if execErr != nil {
			step.Status = "failed"
			step.Error = execErr.Error()
			result.Status = "failed"
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		step.ExitCode = resp.ExitCode
		step.DurationMs = resp.DurationMs
		step.HumanSummary = extractHumanSummary(resp.Stdout)
		step.StdoutPreview = previewText(resp.Stdout)
		step.StderrPreview = previewText(resp.Stderr)
		if !resp.Success || resp.ExitCode != 0 {
			step.Status = "failed"
			step.Error = strings.TrimSpace(resp.Error)
			if step.Error == "" {
				step.Error = strings.TrimSpace(resp.Stderr)
			}
			if step.Error == "" {
				step.Error = fmt.Sprintf("exit code %d", resp.ExitCode)
			}
			result.Status = "failed"
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		step.Status = "completed"
		step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
		analysisAccepted := flowAnalysisAccepted(req)
		if containsString(step.ArtifactTypes, "analysis.schema.v1") && !req.DryRun && (!shouldEmitHumanAcceptance(contract) || analysisAccepted || req.SimulateHuman) {
			s.markFlowInstalled(req.Flow)
			step.ArtifactTypes = append(step.ArtifactTypes, s.recordFlowInstallation(runID, node.ID, req.Flow.BusinessID, available, result.Artifacts))
		}
		if shouldPauseForAnalysisAcceptance(req, contract, step.ArtifactTypes) {
			step.Status = "needs_input"
			result.Status = "needs_input"
			result.NeedsInput = append(result.NeedsInput, inputRequestForAnalysisAcceptance(node, result.Artifacts))
			step.ArtifactTypes = append(step.ArtifactTypes, s.recordFlowReadiness(runID, node.ID, false, result.NeedsInput, available, result.Artifacts))
			finished := finishFlowRunStep(step)
			emitStep("step_complete", finished)
			result.Timeline = append(result.Timeline, finished)
			break
		}
		if containsString(step.ArtifactTypes, "action.options.v1") {
			step.ActionOptions = flowActionOptionsFromArtifacts(result.Artifacts)
		}
		if node.Role == flowRoleEntry {
			if artifactType := s.recordWorkContext(runID, req, node.ID, cyclesDone, available, result.Artifacts); artifactType != "" {
				step.ArtifactTypes = append(step.ArtifactTypes, artifactType)
			}
		}
		if containsString(step.ArtifactTypes, "data.request.v1") {
			rerunSteps := s.resolveDataRequestsAndRerunNode(ctx, runID, req, node, contract, available, &result, emitStep, cyclesDone)
			for _, rerunStep := range rerunSteps {
				step.ArtifactTypes = uniqueStrings(append(step.ArtifactTypes, rerunStep.ArtifactTypes...))
				step.HumanSummary = appendUniqueSummary(step.HumanSummary, rerunStep.HumanSummary)
			}
		}
		// Post-Auditor: iteratively resolve data gaps using other frameworks
		if containsString(step.ArtifactTypes, "data.gaps.v1") || containsString(step.ArtifactTypes, "auditor.findings.v1") {
			if gapSummary := summarizeAuditorGaps(result.Artifacts); gapSummary != "" {
				if step.HumanSummary != "" {
					step.HumanSummary += "\n"
				}
				step.HumanSummary += gapSummary
			}
			s.resolveFlowGapsIteratively(ctx, runID, req, node, available, &result, emitStep, cyclesDone)
			if result.Status == "needs_input" {
				finished := finishFlowRunStep(step)
				emitStep("step_complete", finished)
				result.Timeline = append(result.Timeline, finished)
				break
			}
		}
		if isCycleTerminalStep(node, contract, step) {
			step.ArtifactTypes = append(step.ArtifactTypes, s.recordFlowCycleCompleted(runID, node.ID, node.Capability, available, result.Artifacts))
			step.ArtifactTypes = append(step.ArtifactTypes, s.recordCycleResult(runID, node.ID, node.Capability, step, cyclesDone, available, result.Artifacts))
			if !req.DryRun {
				s.recordTaskLedgerCycleCompleted(result.Artifacts)
			}
			if s.notifyFocoCycleCompleted(ctx, runID, req.Flow.BusinessID, req.Flow.ID, available, result.Artifacts) {
				step.ArtifactTypes = append(step.ArtifactTypes, "focus.cycle_status.v1", "task.done")
			}
			cyclesDone++
			cycleCompletedThisPass = true
			if req.MaxCycles > 0 && cyclesDone >= maxCycles {
				limitPayload := map[string]interface{}{
					"artifact_type": "flow.cycle.limit_reached.v1",
					"run_id":        runID,
					"max_cycles":    maxCycles,
					"cycles_done":   cyclesDone,
					"reached_at":    time.Now().UTC().Format(time.RFC3339Nano),
				}
				path := s.persistFlowArtifact(runID, "flow_limit", "flow.cycle.limit_reached.v1", limitPayload)
				available["flow.cycle.limit_reached.v1"] = true
				result.Artifacts["flow.cycle.limit_reached.v1"] = flowRunArtifact{Type: "flow.cycle.limit_reached.v1", Source: "flow.engine", Node: node.ID, Path: path, Payload: limitPayload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				step.ArtifactTypes = append(step.ArtifactTypes, "flow.cycle.limit_reached.v1")
				step.Status = "max_cycles_reached"
				step.HumanSummary = fmt.Sprintf("Se completaron %d de %d ciclo(s).", cyclesDone, maxCycles)
				result.Status = "max_cycles_reached"
				result.CyclesDone = cyclesDone
				finished := finishFlowRunStep(step)
				emitStep("step_complete", finished)
				result.Timeline = append(result.Timeline, finished)
				break
			}
		}
		s.storeUserContactDestinationIfPossible(runID, req.Flow.BusinessID, result.Artifacts)
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		if req.SimulateHuman && shouldEmitHumanAcceptanceFromStep(finished) && onStep != nil {
			emitStep("human_acceptance", flowRunStep{
				Node:         node.ID + "_human_acceptance",
				Framework:    "simulacion",
				Capability:   "human.acceptance",
				Role:         "human",
				Status:       "completed",
				HumanSummary: s.generateHumanAcceptance(ctx, req, finished),
				StartedAt:    time.Now().UTC().Format(time.RFC3339Nano),
				FinishedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
		if node.Role == flowRoleEntry && !req.BranchMode {
			if req.SimulateHuman {
				if branch := s.generateFlowBranchSimulation(ctx, req, finished, result.Artifacts); branch.HumanSummary != "" {
					emitStep("branch_simulation", branch)
				}
			}
			branches := s.runFlowActionBranches(ctx, req, result.Artifacts)
			if len(branches) > 0 {
				result.Branches = branches
				if onStep != nil {
					emitStep("branch_runs", flowRunStep{Node: node.ID + "_branch_runs", Framework: node.Framework, Capability: "focus.dimension_runs", Role: "branch", Status: "completed", HumanSummary: fmt.Sprintf("%d dimensiones ejecutadas en dry-run seguro", len(branches)), ActionOptions: flowActionOptionsFromArtifacts(result.Artifacts), StartedAt: time.Now().UTC().Format(time.RFC3339Nano), FinishedAt: time.Now().UTC().Format(time.RFC3339Nano)})
				}
			}
		}
	}
	// Multi-cycle: if this pass completed a cycle and we haven't hit the limit, loop back
	if result.Status == "completed" && cycleCompletedThisPass && cyclesDone < maxCycles {
		emitStep("cycle_complete", flowRunStep{
			Node:         fmt.Sprintf("cycle_%d_complete", cyclesDone),
			Framework:    "flow_engine",
			Capability:   "multi_cycle",
			Role:         "cycle",
			CycleIndex:   cyclesDone,
			Status:       "completed",
			HumanSummary: fmt.Sprintf("Ciclo %d completado. Iniciando ciclo %d de %d.", cyclesDone, cyclesDone+1, maxCycles),
			StartedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			FinishedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		})
		resetCycleArtifacts(available, result.Artifacts)
		goto cycleStart
	}
	result.CyclesDone = cyclesDone
	if result.Status == "completed" {
		s.recordFlowReadiness(runID, "flow", true, nil, available, result.Artifacts)
	}
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = s.persistFlowRun(result)
	return result
}

// resetCycleArtifacts removes cycle-specific artifacts so the next cycle
// starts fresh while keeping bootstrap/infrastructure artifacts.

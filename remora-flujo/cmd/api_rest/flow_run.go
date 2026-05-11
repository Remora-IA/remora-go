package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"channel/adapter"
	"channel/manifest"
	"remora-flujo/internal/llm"
)

type flowRunRequest struct {
	Flow             flowManifest           `json:"flow"`
	Input            string                 `json:"input,omitempty"`
	DryRun           bool                   `json:"dry_run"`
	Approved         bool                   `json:"approved,omitempty"`
	TestMode         bool                   `json:"test_mode,omitempty"`
	TestRecipient    string                 `json:"test_recipient,omitempty"`
	FixtureArtifacts []string               `json:"fixture_artifacts,omitempty"`
	InitialArtifacts map[string]interface{} `json:"initial_artifacts,omitempty"`
	BranchMode       bool                   `json:"branch_mode,omitempty"`
	MaxBranches      int                    `json:"max_branches,omitempty"`
	MaxCycles        int                    `json:"max_cycles,omitempty"`
	SimulateHuman    bool                   `json:"simulate_human,omitempty"`
}

type flowRunResult struct {
	RunID             string                     `json:"run_id"`
	Status            string                     `json:"status"`
	CyclesDone        int                        `json:"cycles_done,omitempty"`
	Valid             bool                       `json:"valid"`
	DryRun            bool                       `json:"dry_run"`
	Approved          bool                       `json:"approved,omitempty"`
	TestMode          bool                       `json:"test_mode,omitempty"`
	TestRecipient     string                     `json:"test_recipient,omitempty"`
	BusinessID        string                     `json:"business_id,omitempty"`
	BusinessArtifacts []string                   `json:"business_artifacts,omitempty"`
	ExecutionOrder    []string                   `json:"execution_order"`
	Timeline          []flowRunStep              `json:"timeline"`
	Artifacts         map[string]flowRunArtifact `json:"artifacts"`
	Validation        flowValidationResult       `json:"validation"`
	Warnings          []flowValidationIssue      `json:"warnings,omitempty"`
	NeedsInput        []flowRequiredInput        `json:"needs_input,omitempty"`
	Branches          []flowBranchRun            `json:"branches,omitempty"`
	DynamicNodes      []flowNode                 `json:"dynamic_nodes,omitempty"`
	CreatedAt         string                     `json:"created_at"`
	FinishedAt        string                     `json:"finished_at,omitempty"`
}

type flowBranchRun struct {
	BranchID       string              `json:"branch_id"`
	Action         map[string]string   `json:"action"`
	Status         string              `json:"status"`
	Timeline       []flowRunStep       `json:"timeline"`
	Artifacts      []string            `json:"artifacts,omitempty"`
	NeedsInput     []flowRequiredInput `json:"needs_input,omitempty"`
	CycleCompleted bool                `json:"cycle_completed,omitempty"`
}

type flowRunStep struct {
	Node             string              `json:"node"`
	Framework        string              `json:"framework"`
	Capability       string              `json:"capability,omitempty"`
	Command          string              `json:"command,omitempty"`
	Role             string              `json:"role,omitempty"`
	ResolutionMode   string              `json:"resolution_mode,omitempty"`
	CycleIndex       int                 `json:"cycle_index,omitempty"`
	Status           string              `json:"status"`
	Inputs           []string            `json:"inputs,omitempty"`
	Requires         []string            `json:"requires,omitempty"`
	Outputs          []string            `json:"outputs,omitempty"`
	Produces         []string            `json:"produces,omitempty"`
	Policies         []string            `json:"policies,omitempty"`
	MissingArtifacts []string            `json:"missing_artifacts,omitempty"`
	ArtifactTypes    []string            `json:"artifact_types,omitempty"`
	StartedAt        string              `json:"started_at,omitempty"`
	FinishedAt       string              `json:"finished_at,omitempty"`
	ExitCode         int                 `json:"exit_code,omitempty"`
	DurationMs       int64               `json:"duration_ms,omitempty"`
	Error            string              `json:"error,omitempty"`
	HumanSummary     string              `json:"human_summary,omitempty"`
	StdoutPreview    string              `json:"stdout_preview,omitempty"`
	StderrPreview    string              `json:"stderr_preview,omitempty"`
	ActionOptions    []map[string]string `json:"action_options,omitempty"`
}

type flowRunArtifact struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	Node      string      `json:"node,omitempty"`
	Path      string      `json:"path,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	CreatedAt string      `json:"created_at"`
}

type flowRequiredInput struct {
	Artifact    string            `json:"artifact"`
	Kind        string            `json:"kind"`
	Framework   string            `json:"framework,omitempty"`
	Capability  string            `json:"capability,omitempty"`
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	Fields      []flowInputField  `json:"fields,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
	QuestionID  string            `json:"question_id,omitempty"`
}

type flowInputField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
}

const inlineArtifactArgMaxBytes = 100 * 1024

func (s *server) runFlow(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var req flowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.Flow.BusinessID, nil); !ok {
			return
		}
	}
	result := s.runFlowManifest(r.Context(), req, nil)
	status := http.StatusOK
	if result.Status == "invalid" {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, APIResponse{Success: status < 400, Data: result})
}

func (s *server) runFlowStream(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var req flowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.Flow.BusinessID, nil); !ok {
			return
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sendSSE := func(event string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	result := s.runFlowManifest(r.Context(), req, func(event string, step flowRunStep, totalSteps int) {
		sendSSE(event, map[string]interface{}{
			"step":        step,
			"total_steps": totalSteps,
		})
	})

	sendSSE("flow_complete", result)
}

type flowStepCallback func(event string, step flowRunStep, totalSteps int)

func (s *server) runFlowManifest(ctx context.Context, req flowRunRequest, onStep flowStepCallback) flowRunResult {
	prepareFlowManifestLifecycle(&req.Flow)
	runID := newFlowRunID(req.Flow.ID)
	createdAt := time.Now().UTC()
	var businessArtifacts []string
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		businessArtifacts = s.businessArtifacts(req.Flow.BusinessID).Artifacts
	}
	autoArtifacts := uniqueStrings(append(businessArtifacts, req.FixtureArtifacts...))
	validation := validateFlowManifestWithArtifacts(req.Flow, s.allManifests, autoArtifacts)
	nodeOrder, err := flowExecutionOrder(req.Flow)
	if err != nil {
		nodeOrder = req.Flow.Nodes
	}
	result := flowRunResult{
		RunID:             runID,
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
	mecanicoApprovalApplied := false

cycleStart:
	cycleCompletedThisPass = false
	sabioMediated = false
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
				ResolutionMode: frameworkResolutionMode(node.Framework),
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
			s.ensureSabioDataMediation(ctx, runID, req, available, &result, emitStep, cyclesDone)
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
			ResolutionMode: frameworkResolutionMode(node.Framework),
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
		if s.shouldRunPreflightAudit(node, contract, available) {
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
		}
		for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
			if reqArtifact != "" && !available[reqArtifact] {
				step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
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
		if containsString(step.ArtifactTypes, "analysis.schema.v1") && !req.DryRun {
			s.markFlowInstalled(req.Flow)
			step.ArtifactTypes = append(step.ArtifactTypes, s.recordFlowInstallation(runID, node.ID, req.Flow.BusinessID, available, result.Artifacts))
		}
		if containsString(step.ArtifactTypes, "action.options.v1") {
			step.ActionOptions = flowActionOptionsFromArtifacts(result.Artifacts)
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
		if containsString(step.ArtifactTypes, "message.sent.v1") {
			step.ArtifactTypes = append(step.ArtifactTypes, s.recordFlowCycleCompleted(runID, node.ID, available, result.Artifacts))
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
		if req.SimulateHuman && shouldEmitHumanAcceptance(node) && onStep != nil {
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
					emitStep("branch_runs", flowRunStep{Node: node.ID + "_branch_runs", Framework: "foco", Capability: "focus.dimension_runs", Role: "branch", Status: "completed", HumanSummary: fmt.Sprintf("%d dimensiones ejecutadas en dry-run seguro", len(branches)), ActionOptions: flowActionOptionsFromArtifacts(result.Artifacts), StartedAt: time.Now().UTC().Format(time.RFC3339Nano), FinishedAt: time.Now().UTC().Format(time.RFC3339Nano)})
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
func resetCycleArtifacts(available map[string]bool, artifacts map[string]flowRunArtifact) {
	cycleSpecific := []string{
		"entity.ref.v1",
		"collection.priority_item.v1",
		"focus.next_task.v1",
		"task.next",
		"entity_360.v1",
		"answer.grounded.v1",
		"contact.destination.v1",
		"message.draft.v1",
		"message.channel.v1",
		"message.sent.v1",
		"flow.cycle.completed.v1",
		"focus.cycle_status.v1",
		"task.done",
		"data.gaps.v1",
		"auditor.findings.v1",
		"action.options.v1",
		"action.selection.v1",
	}
	for _, a := range cycleSpecific {
		delete(available, a)
		delete(artifacts, a)
	}
}

func (s *server) shouldSkipInstalledAnalysis(req flowRunRequest, node flowNode) bool {
	if node.Framework != "radar" || node.Capability != "analysis.configure" {
		return false
	}
	if req.InitialArtifacts != nil {
		if _, ok := req.InitialArtifacts["flow.reconfigure.v1"]; ok {
			return false
		}
	}
	return s.flowMarkedInstalled(req.Flow.BusinessID, req.Flow.ID) && s.radarAnalysisInstalled(req.Flow.BusinessID)
}

func (s *server) radarAnalysisInstalled(businessID string) bool {
	return nonEmptyFileExists(s.radarAnalysisPlanPath(businessID))
}

func (s *server) radarAnalysisPlanPath(businessID string) string {
	businessID = strings.TrimSpace(businessID)
	if businessID == "" {
		return ""
	}
	return filepath.Join(s.rootDir, "framework-radar", "temp", "radar", safeFilePart(businessID), "collection_analysis_plan.json")
}

func (s *server) recordFlowInstallation(runID, nodeID, businessID string, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	payload := map[string]interface{}{
		"artifact_type": "flow.installation.v1",
		"status":        "installed",
		"business_id":   businessID,
		"analysis_plan": s.radarAnalysisPlanPath(businessID),
		"installed_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	path := s.persistFlowArtifact(runID, nodeID+"_installation", "flow.installation.v1", payload)
	available["flow.installation.v1"] = true
	artifacts["flow.installation.v1"] = flowRunArtifact{Type: "flow.installation.v1", Source: "flow_engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "flow.installation.v1"
}

func (s *server) markFlowInstalled(flow flowManifest) {
	if s.flows == nil || strings.TrimSpace(flow.ID) == "" || strings.TrimSpace(flow.BusinessID) == "" {
		return
	}
	_ = s.flows.updateFlowStatusByManifestID(flow.BusinessID, flow.ID, "installed")
}

func (s *server) flowMarkedInstalled(businessID, manifestID string) bool {
	if s == nil || s.flows == nil || strings.TrimSpace(businessID) == "" || strings.TrimSpace(manifestID) == "" {
		return false
	}
	rows, err := s.flows.db.Query(`SELECT status, manifest_json FROM flows WHERE business_id = ?`, businessID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var status, raw string
		if rows.Scan(&status, &raw) != nil {
			return false
		}
		if status != "installed" && status != "active" {
			continue
		}
		var m flowManifest
		if json.Unmarshal([]byte(raw), &m) == nil && m.ID == manifestID {
			return true
		}
	}
	return false
}

func shouldEmitHumanAcceptance(node flowNode) bool {
	return node.Role == flowRoleBootstrap && node.Framework == "radar" && node.Capability == "analysis.configure"
}

func (s *server) executeFlowNode(ctx context.Context, runID string, req flowRunRequest, node flowNode, contract nodeContract, artifacts map[string]flowRunArtifact) (*adapter.Response, error) {
	m := s.allManifests[node.Framework]
	cmd, ok := m.Commands[contract.Command]
	if !ok {
		return nil, fmt.Errorf("command %q no existe en manifest %s", contract.Command, m.Name)
	}
	params := map[string]string{}
	for k, v := range node.Params {
		resolved, err := resolveFlowParamTemplate(v, artifacts)
		if err != nil {
			return nil, fmt.Errorf("node %s param %s: %w", node.ID, k, err)
		}
		params[k] = resolved
	}
	params["flow_run_id"] = runID
	setParamIfDeclared(cmd, params, "capability", node.Capability)
	setParamIfDeclared(cmd, params, "node_id", node.ID)
	if req.Input != "" {
		setParamIfDeclared(cmd, params, "input", req.Input)
		setParamIfDeclared(cmd, params, "query", req.Input)
		setParamIfDeclared(cmd, params, "question", req.Input)
		setParamIfDeclared(cmd, params, "answer", req.Input)
	}
	convID := runID
	if node.Framework == "foco" && req.Flow.BusinessID != "" {
		convID = focoFlowStateConvID(req.Flow.BusinessID, req.Flow.ID)
	}
	if req.Flow.BusinessID != "" && flowNodeUsesBusinessVault(node) {
		convID = businessVaultConvID(req.Flow.BusinessID)
	}
	setParamIfDeclared(cmd, params, "conv_id", convID)
	setParamIfDeclared(cmd, params, "conversation_id", convID)
	if req.Flow.BusinessID != "" {
		setParamIfDeclared(cmd, params, "business_id", req.Flow.BusinessID)
		setParamIfDeclared(cmd, params, "profile", req.Flow.BusinessID)
	}
	setParamIfDeclared(cmd, params, "dry_run", fmt.Sprintf("%t", req.DryRun))
	if commandHasParam(cmd, "db") && node.Framework == "sabio" {
		dbPath := s.businessSQLitePath(req.Flow.BusinessID)
		if dbPath == "" {
			dbPath = businessDataDBPath(s.rootDir, req.Flow.BusinessID)
		}
		params["db"] = dbPath
	}
	if commandHasParam(cmd, "semantic_pack") {
		params["semantic_pack"] = s.businessSemanticPackPath(req.Flow.BusinessID)
	}
	if commandHasParam(cmd, "context_b64") {
		params["context_b64"] = encodeFlowRunContext(req)
	}
	if commandHasParam(cmd, "history") {
		params["history"] = ""
	}
	applyArtifactParamDefaults(cmd, params, artifacts)
	applyCapabilityParamDefaults(node, cmd, params, artifacts)
	applyFlowTestModeParamOverrides(req, contract, cmd, params)
	s.materializePortableArtifactParams(runID, node.ID, cmd, params)
	args, err := cmd.ResolveArgs(params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
	if err != nil {
		return nil, err
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + m.Name
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	nodeTimeout := 300 * time.Second
	if node.Framework == "mecanico" && (node.Capability == "action.fix.apply_all" || node.Capability == "action.fix.apply") {
		nodeTimeout = 600 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
	defer cancel()
	return s.scoped(runID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
}

func (s *server) materializePortableArtifactParams(runID, nodeID string, cmd manifest.Command, params map[string]string) {
	for key, value := range params {
		if !strings.HasSuffix(key, "_json") || value == "" {
			continue
		}
		base := strings.TrimSuffix(key, "_json")
		target := firstDeclaredParam(cmd, base+"_path", base+"_artifact")
		if target == "" {
			if len(value) <= inlineArtifactArgMaxBytes {
				continue
			}
			continue
		}
		if strings.TrimSpace(params[target]) == "" {
			tmpPath := filepath.Join(s.rootDir, "temp", "flow_runs", runID, "materialized", safeFilePart(nodeID)+"__"+safeFilePart(base)+".json")
			_ = os.MkdirAll(filepath.Dir(tmpPath), 0755)
			if err := os.WriteFile(tmpPath, []byte(value), 0644); err != nil {
				continue
			}
			params[target] = tmpPath
		}
		params[key] = ""
	}
}

func firstDeclaredParam(cmd manifest.Command, names ...string) string {
	for _, name := range names {
		if commandHasParam(cmd, name) {
			return name
		}
	}
	return ""
}

func flowNodeUsesBusinessVault(node flowNode) bool {
	switch node.Framework {
	case "hosting", "mensajero":
		return true
	default:
		return false
	}
}

func focoFlowStateConvID(businessID, flowID string) string {
	businessID = strings.TrimSpace(businessID)
	flowID = strings.TrimSpace(flowID)
	if businessID == "" {
		return ""
	}
	if flowID == "" {
		flowID = "flow"
	}
	return "business_" + safeFilePart(businessID) + "__flow_" + safeFilePart(flowID) + "__" + time.Now().Format("2006-01-02")
}

func applyCapabilityParamDefaults(node flowNode, cmd manifest.Command, params map[string]string, artifacts map[string]flowRunArtifact) {
	if node.Capability == "data.entity_360" && commandHasParam(cmd, "question") {
		name := ""
		if v, ok := artifactString(artifacts["entity.ref.v1"].Payload, "name"); ok {
			name = v
		}
		ref := ""
		if v, ok := artifactString(artifacts["entity.ref.v1"].Payload, "id"); ok {
			ref = v
		}
		if name == "" {
			name = ref
		}
		if name == "" {
			name = "la entidad priorizada"
		}
		params["question"] = "Genera una vista 360 de " + name + " para preparar una gestión de cobranza. Incluye deuda, mora, contexto relevante y evidencia desde la base de datos."
	}

}

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
				for _, q := range questions {
					needs = append(needs, flowRequiredInput{
						Artifact:   "framework.question.v1",
						Kind:       "framework_question",
						Framework:  "mecanico",
						Capability: "action.fix.resolve_gaps_conversational",
						Title:      "Resolución de contacto faltante",
						Message:    jsonFirstString(q, "text", "message", "question"),
						QuestionID: jsonFirstString(q, "id", "question_id"),
					})
				}
				continue
			}
			needs = append(needs, inputRequestForContactDestination(node, artifacts))
		case "credentials.smtp":
			if credentialAvailableFromArtifacts("credentials.smtp", artifacts) {
				available[artifact] = true
				artifacts[artifact] = flowRunArtifact{Type: artifact, Source: "vault_check", Node: node.ID, Payload: map[string]interface{}{"from_vault": true}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				resolved = append(resolved, artifact)
				continue
			}
			// Fallback: consultar vault directamente vía hosting has-smtp
			if m := s.allManifests["hosting"]; m != nil {
				if cmd, ok := m.Commands["has-smtp"]; ok {
					convID := businessVaultConvID(req.Flow.BusinessID)
					params := map[string]string{"conv_id": convID}
					args, err := cmd.ResolveArgs(params, nil, nil)
					if err == nil {
						fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
						fullArgs = append(fullArgs, args...)
						cwdRel := m.Cwd
						if cwdRel == "" {
							cwdRel = "framework-hosting"
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
			// Activar Hosting conversacionalmente: invocar next-question para obtener
			// la primera pregunta del asistente de hosting.
			if qID, qText, ok := s.invokeHostingNextQuestion(ctx, req.Flow.BusinessID); ok {
				needs = append(needs, flowRequiredInput{
					Artifact:   "credentials.smtp",
					Kind:       "framework_question",
					Framework:  "hosting",
					Capability: "hosting.next-question",
					Title:      "Configurar SMTP con Hosting",
					Message:    qText,
					QuestionID: qID,
				})
			} else {
				needs = append(needs, inputRequestForHostingConnect())
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
			if dest, ok := s.lookupSabioContactDestination(ctx, req.Flow.BusinessID, result.Artifacts); ok {
				resNode := flowNode{ID: fmt.Sprintf("gap_resolve_contact_%d", pass), Framework: "sabio", Capability: "contact.lookup", Role: flowRoleResolution}
				recordDynamicFlowNode(result, resNode)
				available["contact.destination.v1"] = true
				path := s.persistFlowArtifact(runID, "gap_resolve_contact", "contact.destination.v1", dest)
				result.Artifacts["contact.destination.v1"] = flowRunArtifact{Type: "contact.destination.v1", Source: "sabio.contact-lookup", Node: "gap_resolve_contact", Path: path, Payload: dest, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				resStep := flowRunStep{
					Node:           resNode.ID,
					Framework:      "sabio",
					Capability:     "contact.lookup",
					Role:           flowRoleResolution,
					ResolutionMode: frameworkResolutionMode("sabio"),
					CycleIndex:     cycleIdx,
					Status:         "completed",
					HumanSummary:   fmt.Sprintf("Sabio resolvió el contacto faltante: %s", jsonFirstString(dest, "to", "destination", "value")),
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
							Artifact:   "framework.question.v1",
							Kind:       "framework_question",
							Framework:  "mecanico",
							Capability: "action.fix.resolve_gaps_conversational",
							Title:      "Resolución de gap: " + gapType,
							Message:    qText,
							QuestionID: qID,
						})
					}
					result.Status = "needs_input"
					s.recordFlowReadiness(runID, auditorNode.ID, false, result.NeedsInput, available, result.Artifacts)
					return
				}
				// Fallback: needs_input directo tradicional
				result.Status = "needs_input"
				result.NeedsInput = append(result.NeedsInput, inputRequestForContactDestination(auditorNode, result.Artifacts))
				s.recordFlowReadiness(runID, auditorNode.ID, false, result.NeedsInput, available, result.Artifacts)
				return
			}
		}

		// 2. Mecánico gaps: collect all gap kinds that Mecánico can handle and invoke once
		mecanicoGaps := []dataGap{}
		for _, gap := range gaps {
			strategy, ok := findGapResolution(gap.Kind)
			if !ok {
				continue
			}
			if strategy.Framework == "mecanico" && strategy.Mode == resolutionHybrid {
				mecanicoGaps = append(mecanicoGaps, gap)
			}
		}
		if len(mecanicoGaps) > 0 {
			m := s.allManifests["mecanico"]
			if m != nil {
				resNode := flowNode{
					ID:         fmt.Sprintf("gap_resolve_mecanico_%d", pass),
					Framework:  "mecanico",
					Capability: "action.fix.propose_all_auto",
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
							Framework:      "mecanico",
							Capability:     "action.fix.propose_all_auto",
							Command:        contract.Command,
							Role:           flowRoleResolution,
							ResolutionMode: frameworkResolutionMode("mecanico"),
							CycleIndex:     cycleIdx,
							Status:         "running",
							StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
						}
						emitStep("step_start", resStep)
						resp, execErr := s.executeFlowNode(ctx, runID, req, resNode, contract, result.Artifacts)
						if execErr != nil {
							resStep.Status = "failed"
							resStep.Error = execErr.Error()
							resStep.HumanSummary = fmt.Sprintf("Mecánico no pudo generar propuestas de fix: %s", execErr.Error())
						} else if !resp.Success || resp.ExitCode != 0 {
							resStep.Status = "failed"
							resStep.Error = strings.TrimSpace(resp.Error)
							if resStep.Error == "" {
								resStep.Error = strings.TrimSpace(resp.Stderr)
							}
							resStep.HumanSummary = "Mecánico intentó resolver pero no pudo completar la propuesta."
						} else {
							resStep.Status = "completed"
							resStep.ArtifactTypes = s.recordNodeArtifacts(runID, resNode.ID, contract, resp.Stdout, available, result.Artifacts)
							resStep.HumanSummary = extractHumanSummary(resp.Stdout)
							if resStep.HumanSummary == "" {
								resStep.HumanSummary = fmt.Sprintf("Mecánico generó propuestas de remediación para %d brechas.", len(mecanicoGaps))
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
				ResolutionMode: frameworkResolutionMode(auditorNode.Framework),
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
	m := s.allManifests["mecanico"]
	if m == nil {
		return nil, false
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
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return nil, false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-mecanico"
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
	// Record artifacts from Mecánico
	node := flowNode{ID: fmt.Sprintf("mecanico_resolve_gaps_%d", cycleIdx), Framework: "mecanico", Capability: "action.fix.resolve_gaps_conversational", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	if artType, _ := parsed["artifact_type"].(string); artType != "" {
		path := s.persistFlowArtifact(runID, node.ID, artType, parsed)
		artifacts[artType] = flowRunArtifact{Type: artType, Source: "mecanico.resolve-gaps", Node: node.ID, Path: path, Payload: parsed, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if !available[artType] {
			available[artType] = true
		}
	}
	step := flowRunStep{
		Node:           node.ID,
		Framework:      "mecanico",
		Capability:     "action.fix.resolve_gaps_conversational",
		Role:           flowRoleResolution,
		ResolutionMode: frameworkResolutionMode("mecanico"),
		CycleIndex:     cycleIdx,
		Status:         "completed",
		HumanSummary:   fmt.Sprintf("Mecánico generó %d preguntas para resolver gaps conversacionalmente.", len(questions)),
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		FinishedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_complete", step)
	result.Timeline = append(result.Timeline, step)
	return questions, len(questions) > 0
}

func (s *server) invokeHostingNextQuestion(ctx context.Context, businessID string) (questionID, questionText string, ok bool) {
	m := s.allManifests["hosting"]
	if m == nil {
		return "", "", false
	}
	cmd, ok := m.Commands["next-question"]
	if !ok {
		return "", "", false
	}
	convID := businessVaultConvID(businessID)
	params := map[string]string{"conv_id": convID}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return "", "", false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-hosting"
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := s.scoped("hosting_"+businessID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
	cancel()
	if err != nil || resp.ExitCode != 0 {
		return "", "", false
	}
	var parsed map[string]interface{}
	if uerr := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &parsed); uerr != nil {
		return "", "", false
	}
	if len(parsed) == 0 {
		return "", "", false
	}
	qID, _ := parsed["id"].(string)
	qText, _ := parsed["text"].(string)
	if qText == "" {
		return "", "", false
	}
	return qID, qText, true
}

func (s *server) ingestHostingAnswer(ctx context.Context, businessID, questionID, answer string) (responseText string, ok bool) {
	m := s.allManifests["hosting"]
	if m == nil {
		return "", false
	}
	cmd, ok := m.Commands["ingest-answer"]
	if !ok {
		return "", false
	}
	convID := businessVaultConvID(businessID)
	params := map[string]string{"conv_id": convID, "question_id": questionID, "answer": answer}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return "", false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-hosting"
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := s.scoped("hosting_"+businessID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
	cancel()
	if err != nil || resp.ExitCode != 0 {
		return "", false
	}
	// ingest-answer no devuelve stdout directo; la respuesta queda en el state para next-question
	return "ok", true
}

func (s *server) flowRequiresArtifact(flow flowManifest, artifact string) bool {
	for _, node := range flow.Nodes {
		m := s.allManifests[node.Framework]
		if m == nil {
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			continue
		}
		if containsString(append(contract.Inputs, contract.Requires...), artifact) {
			return true
		}
	}
	return false
}

func (s *server) shouldRunPreflightAudit(node flowNode, contract nodeContract, available map[string]bool) bool {
	if node.Framework == "auditor" || available["auditor.findings.v1"] {
		return false
	}
	if s.allManifests["auditor"] == nil {
		return false
	}
	if !hasExternalSideEffect(contract.Policies) && !contractNeedsOperationalReadinessAudit(contract) {
		return false
	}
	return available["external.api.dump.v1"] || available["dataset.raw.v1"] || available["data.sqlite_db.v1"]
}

func contractNeedsOperationalReadinessAudit(contract nodeContract) bool {
	artifacts := append([]string{}, contract.Inputs...)
	artifacts = append(artifacts, contract.Requires...)
	artifacts = append(artifacts, contract.Produces...)
	for _, artifact := range uniqueStrings(artifacts) {
		switch artifact {
		case "contact.destination.v1", "message.draft.v1", "message.sent.v1", "credentials.smtp":
			return true
		}
	}
	return false
}

func (s *server) ensureFlowPreflightAudit(ctx context.Context, runID string, req flowRunRequest, target flowNode, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	if !available["external.api.dump.v1"] && !available["dataset.raw.v1"] && available["data.sqlite_db.v1"] {
		s.ensureSabioDataMediation(ctx, runID, req, available, result, emitStep, cycleIdx)
	}
	if !available["external.api.dump.v1"] && !available["dataset.raw.v1"] {
		return true
	}
	m := s.allManifests["auditor"]
	if m == nil {
		return true
	}
	node := flowNode{ID: "preflight_audit_" + safeFilePart(target.ID), Framework: "auditor", Capability: "data.quality.audit", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		return true
	}
	step := flowRunStep{
		Node:           node.ID,
		Framework:      node.Framework,
		Capability:     node.Capability,
		Command:        contract.Command,
		Role:           flowRoleResolution,
		ResolutionMode: frameworkResolutionMode(node.Framework),
		CycleIndex:     cycleIdx,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		Inputs:         uniqueStrings(contract.Inputs),
		Requires:       uniqueStrings(contract.Requires),
		Outputs:        uniqueStrings(contract.Outputs),
		Produces:       uniqueStrings(contract.Produces),
		Policies:       uniqueStrings(contract.Policies),
	}
	for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
		if reqArtifact != "" && !available[reqArtifact] {
			step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
		}
	}
	if len(step.MissingArtifacts) > 0 {
		step.Status = "skipped"
		step.HumanSummary = "Auditor no pudo validar preflight porque faltan datos para auditar."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return true
	}
	emitStep("step_start", step)
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
		result.Status = "failed"
	} else {
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
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
			if gapSummary := summarizeAuditorGaps(result.Artifacts); gapSummary != "" {
				if step.HumanSummary != "" {
					step.HumanSummary += "\n"
				}
				step.HumanSummary += gapSummary
			}
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	if step.Status != "completed" {
		s.recordFlowPreflight(runID, target, false, available, result.Artifacts, result.NeedsInput)
		return false
	}
	s.resolveFlowGapsIteratively(ctx, runID, req, node, available, result, emitStep, cycleIdx)
	ready := result.Status != "needs_input" && result.Status != "failed"
	s.recordFlowPreflight(runID, target, ready, available, result.Artifacts, result.NeedsInput)
	return ready
}

func (s *server) recordFlowPreflight(runID string, target flowNode, ready bool, available map[string]bool, artifacts map[string]flowRunArtifact, needs []flowRequiredInput) string {
	payload := map[string]interface{}{
		"artifact_type":      "flow.preflight.v1",
		"ready":              ready,
		"target_node":        target.ID,
		"target_framework":   target.Framework,
		"target_capability":  target.Capability,
		"checked_by":         "auditor",
		"checked_at":         time.Now().UTC().Format(time.RFC3339Nano),
		"auditor_available":  available["auditor.findings.v1"],
		"dataset_available":  available["external.api.dump.v1"] || available["dataset.raw.v1"],
		"readiness_artifact": "flow.readiness.v1",
	}
	if gapArt, ok := artifacts["data.gaps.v1"]; ok && gapArt.Payload != nil {
		payload["data_gaps"] = gapArt.Payload
	}
	if len(needs) > 0 {
		blockers := []map[string]interface{}{}
		for _, need := range needs {
			blockers = append(blockers, map[string]interface{}{
				"artifact":   need.Artifact,
				"kind":       need.Kind,
				"framework":  need.Framework,
				"capability": need.Capability,
				"message":    need.Message,
			})
		}
		payload["blockers"] = blockers
	}
	path := s.persistFlowArtifact(runID, "flow_preflight_"+safeFilePart(target.ID), "flow.preflight.v1", payload)
	available["flow.preflight.v1"] = true
	artifacts["flow.preflight.v1"] = flowRunArtifact{Type: "flow.preflight.v1", Source: "flow_engine", Node: target.ID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "flow.preflight.v1"
}

func (s *server) ensureSabioDataMediation(ctx context.Context, runID string, req flowRunRequest, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) {
	const nodeID = "sabio_data_mediation"
	m := s.allManifests["sabio"]
	node := flowNode{ID: nodeID, Framework: "sabio", Capability: "dataset.export", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	step := flowRunStep{
		Node:           nodeID,
		Framework:      "sabio",
		Capability:     "dataset.export",
		Role:           flowRoleResolution,
		ResolutionMode: frameworkResolutionMode("sabio"),
		CycleIndex:     cycleIdx,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_start", step)
	if m == nil {
		step.Status = "failed"
		step.Error = "framework no encontrado: sabio"
		step.HumanSummary = "Sabio no pudo mediar datos porque el framework no está cargado."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		step.HumanSummary = "Sabio no pudo preparar el contrato de mediación de datos."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return
	}
	step.Command = contract.Command
	step.Inputs = contract.Inputs
	step.Requires = contract.Requires
	step.Outputs = contract.Outputs
	step.Produces = contract.Produces
	step.Policies = contract.Policies
	for _, reqArt := range contract.Requires {
		if reqArt != "" && !available[reqArt] {
			step.MissingArtifacts = append(step.MissingArtifacts, reqArt)
		}
	}
	if len(step.MissingArtifacts) > 0 {
		step.Status = "skipped"
		step.HumanSummary = "Sabio no medió datos porque faltan artefactos requeridos."
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return
	}
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
		step.HumanSummary = "Sabio no pudo exportar el dataset canónico para el pipeline."
	} else {
		step.ExitCode = resp.ExitCode
		step.DurationMs = resp.DurationMs
		step.StdoutPreview = previewText(resp.Stdout)
		step.StderrPreview = previewText(resp.Stderr)
		if !resp.Success || resp.ExitCode != 0 {
			step.Status = "failed"
			step.Error = strings.TrimSpace(resp.Error)
			if step.Error == "" {
				step.Error = strings.TrimSpace(resp.Stderr)
			}
			step.HumanSummary = "Sabio intentó mediar datos pero la exportación no completó correctamente."
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, nodeID, contract, resp.Stdout, available, result.Artifacts)
			step.HumanSummary = "Sabio preparó el dataset canónico para Radar, Auditor y frameworks downstream."
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
}

func (s *server) shouldMediateSabioDataBeforeNode(node flowNode, available map[string]bool) bool {
	if node.Framework == "sabio" || !available["data.sqlite_db.v1"] {
		return false
	}
	m := s.allManifests[node.Framework]
	if m == nil {
		return false
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		return false
	}
	for _, artifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
		if (artifact == "dataset.raw.v1" || artifact == "external.api.dump.v1") && !available[artifact] {
			return true
		}
	}
	return false
}

func (s *server) resolveDataRequestsAndRerunNode(ctx context.Context, runID string, req flowRunRequest, node flowNode, contract nodeContract, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) []flowRunStep {
	const maxDataRequestPasses = 3
	steps := []flowRunStep{}
	for pass := 0; pass < maxDataRequestPasses; pass++ {
		artifact, ok := result.Artifacts["data.request.v1"]
		if !ok || artifact.Payload == nil || !isResolvableSabioDataRequest(artifact.Payload) {
			break
		}
		requestPayload := artifact.Payload
		delete(available, "data.request.v1")
		delete(result.Artifacts, "data.request.v1")
		step, ok := s.resolveDataRequestAndRerunNode(ctx, runID, req, node, contract, available, result, emitStep, cycleIdx, pass, requestPayload)
		if step.Node != "" {
			steps = append(steps, step)
		}
		if !ok || !containsString(step.ArtifactTypes, "data.request.v1") {
			break
		}
	}
	if _, stillNeedsData := result.Artifacts["data.request.v1"]; stillNeedsData {
		payload := map[string]interface{}{
			"artifact_type": "data.request.limit_reached.v1",
			"node":          node.ID,
			"framework":     node.Framework,
			"max_passes":    maxDataRequestPasses,
			"reached_at":    time.Now().UTC().Format(time.RFC3339Nano),
		}
		path := s.persistFlowArtifact(runID, node.ID+"_data_request_limit", "data.request.limit_reached.v1", payload)
		available["data.request.limit_reached.v1"] = true
		result.Artifacts["data.request.limit_reached.v1"] = flowRunArtifact{Type: "data.request.limit_reached.v1", Source: "flow.engine", Node: node.ID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	return steps
}

func isResolvableSabioDataRequest(request interface{}) bool {
	reqPayload, _ := request.(map[string]interface{})
	target := strings.ToLower(strings.TrimSpace(jsonFirstString(reqPayload, "target_framework", "framework", "target")))
	return target == "" || target == "sabio"
}

func (s *server) resolveDataRequestAndRerunNode(ctx context.Context, runID string, req flowRunRequest, node flowNode, contract nodeContract, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int, pass int, requestPayload interface{}) (flowRunStep, bool) {
	if node.Framework == "sabio" || !available["data.sqlite_db.v1"] {
		return flowRunStep{}, false
	}
	if !isResolvableSabioDataRequest(requestPayload) {
		return flowRunStep{}, false
	}
	reqPayload, _ := requestPayload.(map[string]interface{})
	reason := jsonFirstString(reqPayload, "reason", "message", "description", "configuration_reason")
	s.ensureSabioDataMediation(ctx, runID, req, available, result, emitStep, cycleIdx)
	if !available["dataset.raw.v1"] && !available["external.api.dump.v1"] {
		return flowRunStep{}, false
	}
	rerunNode := node
	rerunNode.ID = fmt.Sprintf("%s_after_data_request_%d", node.ID, pass+1)
	rerunNode.Role = flowRoleResolution
	recordDynamicFlowNode(result, rerunNode)
	step := flowRunStep{
		Node:           rerunNode.ID,
		Framework:      rerunNode.Framework,
		Capability:     rerunNode.Capability,
		Command:        contract.Command,
		Role:           flowRoleResolution,
		ResolutionMode: frameworkResolutionMode(rerunNode.Framework),
		CycleIndex:     cycleIdx,
		Status:         "running",
		Inputs:         uniqueStrings(contract.Inputs),
		Requires:       uniqueStrings(contract.Requires),
		Outputs:        uniqueStrings(contract.Outputs),
		Produces:       uniqueStrings(contract.Produces),
		Policies:       uniqueStrings(contract.Policies),
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	if reason != "" {
		step.HumanSummary = "El framework pidió más datos: " + reason
	}
	emitStep("step_start", step)
	resp, execErr := s.executeFlowNode(ctx, runID, req, rerunNode, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
	} else {
		step.ExitCode = resp.ExitCode
		step.DurationMs = resp.DurationMs
		step.StdoutPreview = previewText(resp.Stdout)
		step.StderrPreview = previewText(resp.Stderr)
		if !resp.Success || resp.ExitCode != 0 {
			step.Status = "failed"
			step.Error = strings.TrimSpace(resp.Error)
			if step.Error == "" {
				step.Error = strings.TrimSpace(resp.Stderr)
			}
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, rerunNode.ID, contract, resp.Stdout, available, result.Artifacts)
			if summary := extractHumanSummary(resp.Stdout); summary != "" {
				step.HumanSummary = appendUniqueSummary(step.HumanSummary, summary)
			} else if step.HumanSummary == "" {
				step.HumanSummary = "Sabio respondió la solicitud de datos y el nodo se re-ejecutó con artifacts actualizados."
			}
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	return finished, step.Status == "completed"
}

func shouldApplyMecanicoProposals(artifacts map[string]flowRunArtifact) bool {
	id, ok := artifactString(artifacts["action.selection.v1"].Payload, "id")
	if !ok {
		return false
	}
	id = strings.ToLower(strings.TrimSpace(id))
	return id == "apply_mecanico_proposals" || id == "apply_all_mecanico_proposals" || id == "aplicar_propuestas_mecanico"
}

func (s *server) applyApprovedMecanicoProposals(ctx context.Context, runID string, req flowRunRequest, available map[string]bool, result *flowRunResult, emitStep func(string, flowRunStep), cycleIdx int) bool {
	node := flowNode{ID: "mecanico_apply_approved", Framework: "mecanico", Capability: "action.fix.apply_all", Role: flowRoleResolution}
	recordDynamicFlowNode(result, node)
	step := flowRunStep{
		Node:           node.ID,
		Framework:      node.Framework,
		Capability:     node.Capability,
		Role:           flowRoleResolution,
		ResolutionMode: frameworkResolutionMode(node.Framework),
		CycleIndex:     cycleIdx,
		Status:         "running",
		StartedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	emitStep("step_start", step)
	if !req.Approved {
		step.Status = "awaiting_approval"
		step.Error = "aplicar propuestas de Mecánico requiere approved=true"
		step.HumanSummary = "Mecánico necesita aprobación explícita para aplicar propuestas sobre el dataset."
		result.Status = "needs_approval"
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return false
	}
	m := s.allManifests["mecanico"]
	if m == nil {
		step.Status = "failed"
		step.Error = "framework no encontrado: mecanico"
		result.Status = "failed"
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return false
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
		result.Status = "failed"
		finished := finishFlowRunStep(step)
		emitStep("step_complete", finished)
		result.Timeline = append(result.Timeline, finished)
		return false
	}
	step.Command = contract.Command
	step.Inputs = contract.Inputs
	step.Requires = contract.Requires
	step.Outputs = contract.Outputs
	step.Produces = contract.Produces
	step.Policies = contract.Policies
	resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
	if execErr != nil {
		step.Status = "failed"
		step.Error = execErr.Error()
		result.Status = "failed"
	} else {
		step.ExitCode = resp.ExitCode
		step.DurationMs = resp.DurationMs
		step.StdoutPreview = previewText(resp.Stdout)
		step.StderrPreview = previewText(resp.Stderr)
		if !resp.Success || resp.ExitCode != 0 {
			step.Status = "failed"
			step.Error = strings.TrimSpace(resp.Error)
			if step.Error == "" {
				step.Error = strings.TrimSpace(resp.Stderr)
			}
			result.Status = "failed"
		} else {
			step.Status = "completed"
			step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
			step.HumanSummary = extractHumanSummary(resp.Stdout)
			if step.HumanSummary == "" {
				step.HumanSummary = "Mecánico aplicó las propuestas aprobadas y dejó el dataset actualizado."
			}
		}
	}
	finished := finishFlowRunStep(step)
	emitStep("step_complete", finished)
	result.Timeline = append(result.Timeline, finished)
	return step.Status == "completed"
}

// dataGap represents a parsed data gap from the data.gaps.v1 artifact.
type dataGap struct {
	Kind        string
	Description string
	Severity    string
	Field       string
}

// parseDataGaps extracts structured gaps from the data.gaps.v1 artifact.
func parseDataGaps(artifacts map[string]flowRunArtifact) []dataGap {
	gapArt, ok := artifacts["data.gaps.v1"]
	if !ok {
		return nil
	}
	var gapsRaw []interface{}
	if payload, ok := gapArt.Payload.(map[string]interface{}); ok {
		gapsRaw, _ = payload["gaps"].([]interface{})
		if len(gapsRaw) == 0 {
			gapsRaw, _ = payload["data_gaps"].([]interface{})
		}
	} else {
		gapsRaw, _ = gapArt.Payload.([]interface{})
	}
	if len(gapsRaw) == 0 {
		return nil
	}
	var gaps []dataGap
	for _, g := range gapsRaw {
		gmap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		kind := jsonFirstString(gmap, "rule", "kind", "type", "category", "field", "check_id")
		desc := jsonFirstString(gmap, "description", "message", "label", "gap")
		sev := jsonFirstString(gmap, "severity", "level", "priority")
		field := jsonFirstString(gmap, "field", "column", "property")
		if kind == "" && desc != "" {
			kind = "unknown"
		}
		if kind != "" {
			gaps = append(gaps, dataGap{Kind: kind, Description: desc, Severity: sev, Field: field})
		}
	}
	return gaps
}

func requestMecanicoProposalApproval(result *flowRunResult, step flowRunStep) {
	if result == nil || result.Status == "needs_input" {
		return
	}
	result.Status = "needs_input"
	result.NeedsInput = append(result.NeedsInput, flowRequiredInput{
		Artifact:   "mecanico.proposals.v1",
		Kind:       "approval",
		Framework:  "mecanico",
		Capability: "action.fix.apply",
		Title:      "Mecánico propuso remediaciones",
		Message:    "Revisá las propuestas de Mecánico y aprobá cuáles aplicar antes de modificar datos.",
		Suggestions: []string{
			"aplicar propuestas aprobadas",
			"rechazar propuestas",
			"pedir ajuste de propuestas",
		},
		Context: map[string]string{
			"source_node": step.Node,
			"mode":        resolutionHybrid,
		},
	})
}

func (s *server) recordFlowCycleCompleted(runID, nodeID string, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	payload := map[string]interface{}{
		"artifact_type": "flow.cycle.completed.v1",
		"cycle_kind":    "message_sent",
		"completed_by":  nodeID,
		"completed_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if entity, ok := artifacts["entity.ref.v1"].Payload.(map[string]interface{}); ok {
		payload["entity_ref"] = jsonFirstString(entity, "entity_ref", "id", "ref", "code")
		payload["entity_type"] = jsonFirstString(entity, "entity_type", "type", "kind")
		payload["entity_name"] = jsonFirstString(entity, "name", "label")
	}
	if task, ok := artifacts["task.next"].Payload.(map[string]interface{}); ok {
		payload["task_id"] = jsonFirstString(task, "task_id", "id")
	}
	if sent, ok := artifacts["message.sent.v1"].Payload.(map[string]interface{}); ok {
		payload["evidence"] = map[string]interface{}{
			"message_id": jsonFirstString(sent, "message_id", "id"),
			"to":         jsonFirstString(sent, "to", "destination"),
			"channel":    jsonFirstString(sent, "channel"),
		}
	}
	path := s.persistFlowArtifact(runID, nodeID+"_cycle_completed", "flow.cycle.completed.v1", payload)
	available["flow.cycle.completed.v1"] = true
	artifacts["flow.cycle.completed.v1"] = flowRunArtifact{Type: "flow.cycle.completed.v1", Source: "flow_engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return "flow.cycle.completed.v1"
}

func (s *server) notifyFocoCycleCompleted(ctx context.Context, runID, businessID, flowID string, available map[string]bool, artifacts map[string]flowRunArtifact) bool {
	m := s.allManifests["foco"]
	if m == nil {
		return false
	}
	cmd, ok := m.Commands["complete-cycle"]
	if !ok {
		return false
	}
	cycle, _ := artifacts["flow.cycle.completed.v1"].Payload.(map[string]interface{})
	if cycle == nil {
		return false
	}
	evidence, _ := cycle["evidence"].(map[string]interface{})
	params := map[string]string{
		"task_id":     jsonFirstString(cycle, "task_id"),
		"entity_ref":  jsonFirstString(cycle, "entity_ref"),
		"entity_name": jsonFirstString(cycle, "entity_name"),
		"cycle_kind":  jsonFirstString(cycle, "cycle_kind"),
		"message_id":  jsonFirstString(evidence, "message_id"),
		"to":          jsonFirstString(evidence, "to"),
	}
	if businessID != "" {
		setParamIfDeclared(cmd, params, "conv_id", focoFlowStateConvID(businessID, flowID))
	}
	if params["cycle_kind"] == "" {
		params["cycle_kind"] = "message_sent"
	}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-foco"
	}
	execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := s.scoped(runID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, filepath.Join(s.rootDir, cwdRel))
	if err != nil || resp.ExitCode != 0 {
		return false
	}
	payload := parseArtifactPayload(resp.Stdout)
	if typ, _ := payload["artifact_type"].(string); typ == "focus.cycle_status.v1" {
		path := s.persistFlowArtifact(runID, "foco_cycle_complete", "focus.cycle_status.v1", payload)
		available["focus.cycle_status.v1"] = true
		available["task.done"] = true
		artifacts["focus.cycle_status.v1"] = flowRunArtifact{Type: "focus.cycle_status.v1", Source: "foco.complete-cycle", Node: "foco_cycle_complete", Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		artifacts["task.done"] = flowRunArtifact{Type: "task.done", Source: "foco.complete-cycle", Node: "foco_cycle_complete", Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		return true
	}
	return false
}

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

func inputRequestForHostingConnect() flowRequiredInput {
	return flowRequiredInput{
		Artifact:   "credentials.smtp",
		Kind:       "hosting_connect",
		Framework:  "hosting",
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

func inputRequestForContactDestination(node flowNode, artifacts map[string]flowRunArtifact) flowRequiredInput {
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
		Framework:  "sabio",
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

func recordDynamicFlowNode(result *flowRunResult, node flowNode) {
	if result == nil || node.ID == "" {
		return
	}
	for _, existing := range result.DynamicNodes {
		if existing.ID == node.ID {
			return
		}
	}
	if node.Role == "" {
		node.Role = flowRoleResolution
	}
	result.DynamicNodes = append(result.DynamicNodes, node)
	if !containsString(result.ExecutionOrder, node.ID) {
		result.ExecutionOrder = append(result.ExecutionOrder, node.ID)
	}
}

func artifactStringNested(payload interface{}, path []string) (string, bool) {
	current := payload
	for _, part := range path {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return "", false
		}
		current = obj[part]
	}
	s, ok := current.(string)
	return s, ok && s != ""
}

func isLikelyEmail(s string) bool {
	s = strings.TrimSpace(s)
	return strings.Contains(s, "@") && strings.Contains(s, ".") && !strings.Contains(strings.ToLower(s), "sin destinatario") && !strings.Contains(strings.ToLower(s), "@ejemplo.")
}

func artifactLabelForAPI(artifact string) string {
	switch artifact {
	case "credentials.smtp":
		return "credenciales de correo del negocio"
	case "contact.destination.v1":
		return "email del destinatario"
	case "message.draft.v1":
		return "borrador del mensaje"
	default:
		return artifact
	}
}

// inferCheckedCredential extracts the credential capability being checked
// from the contract. Convention: a capability like "credentials.smtp.check"
// checks "credentials.smtp".
func inferCheckedCredential(contract nodeContract) string {
	for _, out := range contract.Outputs {
		if out == "credentials.status.v1" || out == "message.send_readiness.v1" {
			// Look through requires/inputs for the pattern
			for _, inp := range append(contract.Inputs, contract.Requires...) {
				if strings.HasPrefix(inp, "credentials.") && inp != "credentials.status.v1" {
					return inp
				}
			}
			// Fallback: infer from command name
			if strings.Contains(contract.Command, "smtp") || strings.Contains(contract.Command, "has-smtp") {
				return "credentials.smtp"
			}
			if strings.Contains(contract.Command, "can-send") {
				return "credentials.smtp"
			}
		}
	}
	return ""
}

func (s *server) recordNodeArtifacts(runID, nodeID string, contract nodeContract, stdout string, available map[string]bool, artifacts map[string]flowRunArtifact) []string {
	types := uniqueStrings(append(contract.Outputs, contract.Produces...))
	payload := parseArtifactPayload(stdout)
	if typ, ok := payload["artifact_type"].(string); ok && typ != "" && !containsString(types, typ) {
		types = append(types, typ)
	}
	if declared, ok := payload["artifacts"].([]interface{}); ok {
		for _, item := range declared {
			if typ, ok := item.(string); ok && typ != "" && !containsString(types, typ) {
				types = append(types, typ)
			}
		}
	}
	if !payloadDeclaresDataRequest(payload) {
		types = removeString(types, "data.request.v1")
	}
	for _, typ := range types {
		if typ == "" {
			continue
		}
		available[typ] = true
		artifactPayload := payloadForArtifactType(typ, payload)
		path := s.persistFlowArtifact(runID, nodeID, typ, artifactPayload)
		artifacts[typ] = flowRunArtifact{Type: typ, Source: "framework_stdout", Node: nodeID, Path: path, Payload: artifactPayload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	// credential check promotion: when a credential-check node reports
	// available=true for a capability, mark that capability as available
	// so downstream nodes (e.g. mensajero.send) can use it.
	promoteCredentialCheckArtifacts(payload, available, artifacts, runID, nodeID)
	return types
}

func payloadDeclaresDataRequest(payload map[string]interface{}) bool {
	if req, ok := payload["request"]; ok && req != nil {
		return true
	}
	if typ, _ := payload["artifact_type"].(string); typ == "data.request.v1" {
		return true
	}
	if declared, ok := payload["artifacts"].([]interface{}); ok {
		for _, item := range declared {
			if typ, _ := item.(string); typ == "data.request.v1" {
				return true
			}
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

// promoteCredentialCheckArtifacts inspects the stdout payload of credential-
// check commands (hosting has-smtp, mensajero can-send). If the output
// indicates the credential is available ({"available": true, "capability": X}),
// we add the capability artifact to the available set. This bridges the gap
// between vault-based credentials and the flow artifact graph.
func promoteCredentialCheckArtifacts(payload map[string]interface{}, available map[string]bool, artifacts map[string]flowRunArtifact, runID, nodeID string) {
	avail, _ := payload["available"].(bool)
	cap, _ := payload["capability"].(string)
	if cap == "" {
		// Also check missing_capability for mensajero can-send negative case
		cap, _ = payload["missing_capability"].(string)
	}
	if cap == "" {
		return
	}
	if avail {
		available[cap] = true
		artifacts[cap] = flowRunArtifact{
			Type:      cap,
			Source:    "vault_check",
			Node:      nodeID,
			Payload:   map[string]interface{}{"from_vault": true, "checked_by": nodeID},
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
}

func payloadForArtifactType(typ string, payload map[string]interface{}) interface{} {
	if typ == "action.options.v1" {
		if options, ok := payload["action_options"]; ok {
			return options
		}
	}
	if typ == "data.gaps.v1" {
		if gaps, ok := payload["data_gaps"]; ok {
			return gaps
		}
	}
	if typ == "data.request.v1" {
		if req, ok := payload["request"]; ok {
			return req
		}
	}
	if typ == "dataset.raw.v1" || typ == "external.api.dump.v1" {
		if updated, ok := payload["updated_dataset"]; ok {
			return updated
		}
	}
	if typ == "mecanico.applied.v1" {
		if applied, ok := payload["applied"]; ok {
			return applied
		}
	}
	if selected, ok := payload["selected"].(map[string]interface{}); ok {
		if selectedType, _ := selected["artifact_type"].(string); selectedType == typ {
			return selected
		}
	}
	if item, ok := payload["priority_item"].(map[string]interface{}); ok && typ == "collection.priority_item.v1" {
		return item
	}
	return payload
}

func parseArtifactPayload(stdout string) map[string]interface{} {
	payload := map[string]interface{}{"raw_stdout": stdout}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &decoded); err == nil {
		return decoded
	}
	return payload
}

func (s *server) persistFlowRun(result flowRunResult) error {
	dir := filepath.Join(s.rootDir, "temp", "flow_runs", result.RunID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "run.json"), raw, 0644)
}

func (s *server) persistFlowArtifact(runID, nodeID, typ string, payload interface{}) string {
	dir := filepath.Join(s.rootDir, "temp", "flow_runs", runID, "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	path := filepath.Join(dir, safeFilePart(nodeID)+"__"+safeFilePart(typ)+".json")
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return ""
	}
	return path
}

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
	raw, err := json.Marshal(ctx)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func finishFlowRunStep(step flowRunStep) flowRunStep {
	step.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return step
}

func hasExternalSideEffect(policies []string) bool {
	for _, policy := range policies {
		if isExternalSideEffectPolicy(policy) {
			return true
		}
	}
	return false
}

func nodeRequiresRuntimeApproval(node flowNode, contract nodeContract, req flowRunRequest) (bool, string) {
	if req.Approved {
		return false, ""
	}
	mode := frameworkResolutionMode(node.Framework)
	switch {
	case hasExternalSideEffect(contract.Policies):
		if req.TestMode {
			return false, ""
		}
		if req.DryRun {
			return true, "side effect externo omitido en prueba segura"
		}
		return true, "side effect externo requiere approved=true"
	case hasRuntimeMutationPolicy(contract.Policies):
		if req.DryRun {
			return true, "mutación de estado omitida en prueba segura"
		}
		return true, "mutación de estado requiere approved=true"
	case hasPolicy(contract.Policies, "approval_required"):
		if mode == resolutionInteractive {
			return true, "framework interactivo requiere approved=true"
		}
		if mode == resolutionHybrid {
			return true, "framework híbrido requiere approved=true para esta acción"
		}
		return true, "acción requiere approved=true"
	default:
		return false, ""
	}
}

func hasRuntimeMutationPolicy(policies []string) bool {
	for _, policy := range policies {
		p := strings.ToLower(strings.TrimSpace(policy))
		if p == "state_mutation" || p == "operator_authorized_write" || p == "external_mutation" {
			return true
		}
	}
	return false
}

func runtimeApprovalSummary(node flowNode, contract nodeContract) string {
	mode := frameworkResolutionMode(node.Framework)
	action := node.Capability
	if action == "" {
		action = contract.Command
	}
	switch mode {
	case resolutionInteractive:
		return fmt.Sprintf("%s necesita confirmación del usuario antes de ejecutar %s.", node.Framework, action)
	case resolutionHybrid:
		return fmt.Sprintf("%s preparó una acción híbrida y necesita aprobación antes de aplicar cambios.", node.Framework)
	default:
		return fmt.Sprintf("%s requiere aprobación antes de ejecutar %s.", node.Framework, action)
	}
}

func (s *server) generateHumanAcceptance(ctx context.Context, req flowRunRequest, step flowRunStep) string {
	summary := strings.TrimSpace(step.HumanSummary)
	if summary == "" {
		summary = "el análisis inicial quedó listo"
	}
	fallback := "ok, sigamos con eso"
	if step.Framework == "radar" && step.Capability == "analysis.configure" {
		fallback = "acepto esta configuración"
	}
	spec, err := modelSpecFor(&Conversation{ID: "flow_human_simulation", Models: map[string]string{}}, "sabio")
	if err != nil {
		return fallback
	}
	client, err := llm.New(spec)
	if err != nil {
		return fallback
	}
	system := "Imita a un usuario humano ocupado en una prueba de UX. Responde en español, en minúsculas, muy breve y natural. No expliques. No uses JSON. Máximo 8 palabras."
	user := "El sistema acaba de presentar este análisis inicial para un flujo de negocio:\n" + summary + "\n\nEl usuario acepta continuar. Escribe solo lo que diría el usuario."
	if step.Framework == "radar" && step.Capability == "analysis.configure" {
		user = "Radar acaba de proponer una configuración/algoritmo de análisis para reutilizar en código:\n" + summary + "\n\nEl usuario acepta esa configuración. Escribe solo lo que diría el usuario."
	}
	out, err := client.Complete(ctx, llm.CompletionRequest{System: system, User: user, MaxTokens: 40})
	if err != nil {
		return fallback
	}
	out = strings.Trim(strings.TrimSpace(out), "\"'")
	out = strings.ReplaceAll(out, "\n", " ")
	if out == "" || len(out) > 80 {
		return fallback
	}
	return out
}

func (s *server) runFlowActionBranches(ctx context.Context, req flowRunRequest, artifacts map[string]flowRunArtifact) []flowBranchRun {
	options := flowActionOptionsFromArtifacts(artifacts)
	if len(options) == 0 {
		return nil
	}
	limit := req.MaxBranches
	if limit <= 0 || limit > 3 {
		limit = 3
	}
	if len(options) < limit {
		limit = len(options)
	}
	branches := make([]flowBranchRun, limit)
	var wg sync.WaitGroup
	for i, option := range options {
		if i >= limit {
			break
		}
		i, option := i, option
		wg.Add(1)
		go func() {
			defer wg.Done()
			branchReq := req
			if req.TestMode {
				branchReq.DryRun = false
				branchReq.Approved = true
			} else {
				branchReq.DryRun = true
				branchReq.Approved = false
			}
			branchReq.BranchMode = true
			branchReq.InitialArtifacts = cloneInitialArtifacts(req.InitialArtifacts)
			branchReq.InitialArtifacts["action.selection.v1"] = map[string]interface{}{
				"artifact_type": "action.selection.v1",
				"id":            option["id"],
				"label":         option["label"],
				"description":   option["description"],
				"source":        "dimension_branch",
				"selected_at":   time.Now().UTC().Format(time.RFC3339Nano),
			}
			for typ, artifact := range artifacts {
				if _, exists := branchReq.InitialArtifacts[typ]; exists || artifact.Payload == nil {
					continue
				}
				branchReq.InitialArtifacts[typ] = artifact.Payload
			}
			branchReq.Flow.ID = req.Flow.ID + "_dim_" + safeFilePart(optionIDForBranch(option, i))
			branchResult := s.runFlowManifest(ctx, branchReq, nil)
			branches[i] = flowBranchRun{
				BranchID:       branchResult.RunID,
				Action:         option,
				Status:         branchResult.Status,
				Timeline:       branchResult.Timeline,
				Artifacts:      sortedKeys(flowArtifactTypes(branchResult.Artifacts)),
				NeedsInput:     branchResult.NeedsInput,
				CycleCompleted: branchResult.Artifacts["flow.cycle.completed.v1"].Type == "flow.cycle.completed.v1",
			}
		}()
	}
	wg.Wait()
	return branches
}

func cloneInitialArtifacts(in map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func optionIDForBranch(option map[string]string, idx int) string {
	if id := strings.TrimSpace(option["id"]); id != "" {
		return id
	}
	if label := strings.TrimSpace(option["label"]); label != "" {
		return label
	}
	return fmt.Sprintf("option_%d", idx+1)
}

func flowArtifactTypes(artifacts map[string]flowRunArtifact) map[string]bool {
	out := map[string]bool{}
	for typ := range artifacts {
		out[typ] = true
	}
	return out
}

func (s *server) generateFlowBranchSimulation(ctx context.Context, req flowRunRequest, step flowRunStep, artifacts map[string]flowRunArtifact) flowRunStep {
	options := flowActionOptionsFromArtifacts(artifacts)
	if len(options) == 0 {
		return flowRunStep{}
	}
	started := time.Now().UTC().Format(time.RFC3339Nano)
	return flowRunStep{
		Node:          step.Node + "_branches",
		Framework:     "foco",
		Capability:    "focus.branch_test",
		Role:          "human",
		Status:        "completed",
		HumanSummary:  s.generateBranchSimulationText(ctx, req, step, options),
		ActionOptions: options,
		StartedAt:     started,
		FinishedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func flowActionOptionsFromArtifacts(artifacts map[string]flowRunArtifact) []map[string]string {
	payload := artifacts["action.options.v1"].Payload
	rawOptions, ok := payload.([]interface{})
	if !ok {
		if m, ok := payload.(map[string]interface{}); ok {
			rawOptions, _ = m["action_options"].([]interface{})
		}
	}
	options := []map[string]string{}
	for _, raw := range rawOptions {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id := jsonFirstString(m, "id")
		label := jsonFirstString(m, "label")
		description := jsonFirstString(m, "description")
		if label == "" {
			continue
		}
		options = append(options, map[string]string{"id": id, "label": label, "description": description})
	}
	return options
}

func (s *server) generateBranchSimulationText(ctx context.Context, req flowRunRequest, step flowRunStep, options []map[string]string) string {
	fallback := fallbackBranchSimulationText(options)
	spec, err := modelSpecFor(&Conversation{ID: "flow_branch_simulation", Models: map[string]string{}}, "foco")
	if err != nil {
		return fallback
	}
	client, err := llm.New(spec)
	if err != nil {
		return fallback
	}
	rawOptions, _ := json.Marshal(options)
	system := "Eres Foco simulando una prueba UX de ramas. Genera texto breve en español. No inventes datos de negocio. No uses JSON. Debes mostrar las 3 opciones como ramas paralelas, cada una con una frase natural de usuario simulada y qué pasaría después en términos genéricos de framework."
	user := fmt.Sprintf("Flujo: %s\nPaso Foco: %s\nOpciones de acción: %s\n\nGenera una tarjeta breve titulada \"Prueba paralela de opciones\". La frase del usuario debe sonar humana y distinta por opción.", req.Flow.ID, step.HumanSummary, string(rawOptions))
	out, err := client.Complete(ctx, llm.CompletionRequest{System: system, User: user, MaxTokens: 260})
	if err != nil {
		return fallback
	}
	out = strings.TrimSpace(out)
	if out == "" || len(out) > 1800 {
		return fallback
	}
	return out
}

func fallbackBranchSimulationText(options []map[string]string) string {
	var sb strings.Builder
	sb.WriteString("Prueba paralela de opciones\n")
	for i, option := range options {
		if i >= 3 {
			break
		}
		label := strings.TrimSpace(option["label"])
		description := strings.TrimSpace(option["description"])
		fmt.Fprintf(&sb, "\nRama %d — %s\n", i+1, label)
		fmt.Fprintf(&sb, "Usuario simulado: quiero %s.\n", strings.ToLower(label))
		if description != "" {
			fmt.Fprintf(&sb, "Qué pasaría: %s\n", description)
		} else {
			sb.WriteString("Qué pasaría: Foco deriva el flujo a los frameworks adecuados para esa decisión.\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

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

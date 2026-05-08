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
	"time"

	"channel/adapter"
	"channel/manifest"
)

type flowRunRequest struct {
	Flow             flowManifest           `json:"flow"`
	Input            string                 `json:"input,omitempty"`
	DryRun           bool                   `json:"dry_run"`
	Approved         bool                   `json:"approved,omitempty"`
	FixtureArtifacts []string               `json:"fixture_artifacts,omitempty"`
	InitialArtifacts map[string]interface{} `json:"initial_artifacts,omitempty"`
}

type flowRunResult struct {
	RunID             string                     `json:"run_id"`
	Status            string                     `json:"status"`
	Valid             bool                       `json:"valid"`
	DryRun            bool                       `json:"dry_run"`
	Approved          bool                       `json:"approved,omitempty"`
	BusinessID        string                     `json:"business_id,omitempty"`
	BusinessArtifacts []string                   `json:"business_artifacts,omitempty"`
	ExecutionOrder    []string                   `json:"execution_order"`
	Timeline          []flowRunStep              `json:"timeline"`
	Artifacts         map[string]flowRunArtifact `json:"artifacts"`
	Validation        flowValidationResult       `json:"validation"`
	Warnings          []flowValidationIssue      `json:"warnings,omitempty"`
	CreatedAt         string                     `json:"created_at"`
	FinishedAt        string                     `json:"finished_at,omitempty"`
}

type flowRunStep struct {
	Node             string   `json:"node"`
	Framework        string   `json:"framework"`
	Capability       string   `json:"capability,omitempty"`
	Command          string   `json:"command,omitempty"`
	Status           string   `json:"status"`
	Inputs           []string `json:"inputs,omitempty"`
	Requires         []string `json:"requires,omitempty"`
	Outputs          []string `json:"outputs,omitempty"`
	Produces         []string `json:"produces,omitempty"`
	Policies         []string `json:"policies,omitempty"`
	MissingArtifacts []string `json:"missing_artifacts,omitempty"`
	ArtifactTypes    []string `json:"artifact_types,omitempty"`
	StartedAt        string   `json:"started_at,omitempty"`
	FinishedAt       string   `json:"finished_at,omitempty"`
	ExitCode         int      `json:"exit_code,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
	Error            string   `json:"error,omitempty"`
	StdoutPreview    string   `json:"stdout_preview,omitempty"`
	StderrPreview    string   `json:"stderr_preview,omitempty"`
}

type flowRunArtifact struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	Node      string      `json:"node,omitempty"`
	Path      string      `json:"path,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	CreatedAt string      `json:"created_at"`
}

func (s *server) runFlow(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	var req flowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	result := s.runFlowManifest(r.Context(), req)
	status := http.StatusOK
	if result.Status == "invalid" {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, APIResponse{Success: status < 400, Data: result})
}

func (s *server) runFlowManifest(ctx context.Context, req flowRunRequest) flowRunResult {
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

	for _, node := range nodeOrder {
		result.ExecutionOrder = append(result.ExecutionOrder, node.ID)
		step := flowRunStep{
			Node:       node.ID,
			Framework:  node.Framework,
			Capability: node.Capability,
			Status:     "running",
			StartedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		}
		m := s.allManifests[node.Framework]
		if m == nil {
			step.Status = "failed"
			step.Error = "framework no encontrado: " + node.Framework
			result.Status = "failed"
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			break
		}
		contract, contractErr := resolveFlowNodeContract(node, m)
		if contractErr != nil {
			step.Status = "failed"
			step.Error = contractErr.Error()
			result.Status = "failed"
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			break
		}
		step.Command = contract.Command
		step.Inputs = uniqueStrings(contract.Inputs)
		step.Requires = uniqueStrings(contract.Requires)
		step.Outputs = uniqueStrings(contract.Outputs)
		step.Produces = uniqueStrings(contract.Produces)
		step.Policies = uniqueStrings(contract.Policies)
		for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
			if reqArtifact != "" && !available[reqArtifact] {
				step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
			}
		}
		if len(step.MissingArtifacts) > 0 {
			step.Status = "blocked"
			result.Status = "blocked"
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			break
		}
		if req.DryRun {
			step.Status = "would_run"
			step.ArtifactTypes = s.recordSyntheticArtifacts(runID, node.ID, contract, available, result.Artifacts)
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			continue
		}
		if hasExternalSideEffect(contract.Policies) && !req.Approved {
			step.Status = "awaiting_approval"
			step.Error = "side effect externo requiere approved=true"
			result.Status = "needs_approval"
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			break
		}
		resp, execErr := s.executeFlowNode(ctx, runID, req, node, contract, result.Artifacts)
		if execErr != nil {
			step.Status = "failed"
			step.Error = execErr.Error()
			result.Status = "failed"
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			break
		}
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
			if step.Error == "" {
				step.Error = fmt.Sprintf("exit code %d", resp.ExitCode)
			}
			result.Status = "failed"
			result.Timeline = append(result.Timeline, finishFlowRunStep(step))
			break
		}
		step.Status = "completed"
		step.ArtifactTypes = s.recordNodeArtifacts(runID, node.ID, contract, resp.Stdout, available, result.Artifacts)
		result.Timeline = append(result.Timeline, finishFlowRunStep(step))
	}
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = s.persistFlowRun(result)
	return result
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
	setParamIfDeclared(cmd, params, "conv_id", runID)
	setParamIfDeclared(cmd, params, "conversation_id", runID)
	if req.Flow.BusinessID != "" {
		setParamIfDeclared(cmd, params, "business_id", req.Flow.BusinessID)
		setParamIfDeclared(cmd, params, "profile", req.Flow.BusinessID)
	}
	if commandHasParam(cmd, "db") {
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
	execCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	return s.scoped(runID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
}

func (s *server) recordSyntheticArtifacts(runID, nodeID string, contract nodeContract, available map[string]bool, artifacts map[string]flowRunArtifact) []string {
	types := uniqueStrings(append(contract.Outputs, contract.Produces...))
	for _, typ := range types {
		if typ == "" {
			continue
		}
		available[typ] = true
		payload := map[string]interface{}{
			"artifact_type": typ,
			"dry_run":       true,
			"node":          nodeID,
			"run_id":        runID,
		}
		path := s.persistFlowArtifact(runID, nodeID, typ, payload)
		artifacts[typ] = flowRunArtifact{Type: typ, Source: "dry_run", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	return types
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
	for _, typ := range types {
		if typ == "" {
			continue
		}
		available[typ] = true
		artifactPayload := payloadForArtifactType(typ, payload)
		path := s.persistFlowArtifact(runID, nodeID, typ, artifactPayload)
		artifacts[typ] = flowRunArtifact{Type: typ, Source: "framework_stdout", Node: nodeID, Path: path, Payload: artifactPayload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	return types
}

func payloadForArtifactType(typ string, payload map[string]interface{}) interface{} {
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
	setFromArtifact("to", "message.draft.v1", "to", "destination")
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
	if commandHasParam(cmd, "tono") && params["tono"] == "" {
		params["tono"] = "formal"
	}
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

func previewText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 500 {
		return s
	}
	return s[:500]
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

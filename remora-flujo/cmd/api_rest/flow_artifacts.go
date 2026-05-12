package main

import (
	"os"
	"strings"
	"time"

	"channel/manifest"

	"encoding/json"
	"path/filepath"
)

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

func (s *server) validateActionOptionsForNode(runID, nodeID string, m *manifest.Manifest, available map[string]bool, artifacts map[string]flowRunArtifact) []map[string]string {
	if m == nil || len(m.ActionBounds) == 0 {
		return flowActionOptionsFromArtifacts(artifacts)
	}
	payload, ok := artifacts["action.options.v1"].Payload.(map[string]interface{})
	if !ok {
		return nil
	}
	rawOptions, _ := payload["action_options"].([]interface{})
	if len(rawOptions) == 0 {
		return nil
	}
	allowed := map[string]manifest.ActionBoundSpec{}
	for _, bound := range m.ActionBounds {
		if bound.Type != "" {
			allowed[bound.Type] = bound
		}
	}
	accepted := []map[string]interface{}{}
	rejected := []map[string]interface{}{}
	used := map[string]bool{}
	for _, raw := range rawOptions {
		opt, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		boundID := jsonFirstString(opt, "bound_id", "type")
		if _, ok := allowed[boundID]; ok && !used[boundID] {
			accepted = append(accepted, opt)
			used[boundID] = true
		} else {
			rejected = append(rejected, opt)
		}
	}
	if len(accepted) == 0 {
		for _, bound := range m.ActionBounds {
			accepted = append(accepted, fallbackActionOptionForBound(bound))
			break
		}
	}
	for _, bound := range m.ActionBounds {
		if len(accepted) >= 3 {
			break
		}
		if bound.Type == "" || used[bound.Type] {
			continue
		}
		accepted = append(accepted, fallbackActionOptionForBound(bound))
		used[bound.Type] = true
	}
	payload["action_options"] = accepted
	payload["action_bounds"] = m.ActionBounds
	if len(rejected) > 0 {
		payload["rejected_action_options"] = rejected
	}
	path := s.persistFlowArtifact(runID, nodeID+"_action_bounds", "action.bounds.validation.v1", payload)
	available["action.bounds.validation.v1"] = true
	artifacts["action.bounds.validation.v1"] = flowRunArtifact{Type: "action.bounds.validation.v1", Source: "flow.engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	artifacts["action.options.v1"] = flowRunArtifact{Type: "action.options.v1", Source: "flow.engine", Node: nodeID, Path: path, Payload: payload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return flowActionOptionsFromArtifacts(artifacts)
}

func fallbackActionOptionForBound(bound manifest.ActionBoundSpec) map[string]interface{} {
	label := ""
	if len(bound.Examples) > 0 {
		label = strings.TrimSpace(bound.Examples[0])
	}
	if label == "" {
		label = strings.TrimSpace(bound.Description)
	}
	if label == "" {
		label = bound.Type
	}
	desc := strings.TrimSpace(bound.Description)
	if desc == "" {
		desc = label
	}
	return map[string]interface{}{
		"id":          safeFilePart(bound.Type),
		"bound_id":    bound.Type,
		"label":       label,
		"description": desc,
		"fallback":    true,
	}
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
	if typ == "task.next" {
		if task, ok := payload["task_next"]; ok {
			return task
		}
		if task, ok := payload["task"]; ok {
			return task
		}
	}
	if typ == "action.options.v1" {
		if options, ok := payload["action_options"]; ok {
			return map[string]interface{}{"action_options": options}
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
	if err := os.WriteFile(filepath.Join(dir, "run.json"), raw, 0644); err != nil {
		return err
	}
	if s.flows == nil {
		return nil
	}
	if err := s.flows.recordRun(result); err != nil {
		return err
	}
	for _, artifact := range result.Artifacts {
		if artifact.Path == "" {
			continue
		}
		createdAt := artifact.CreatedAt
		if createdAt == "" {
			createdAt = result.FinishedAt
		}
		if createdAt == "" {
			createdAt = result.CreatedAt
		}
		if err := s.flows.recordArtifact(result.RunID, result.FlowID, result.BusinessID, artifact.Node, artifact.Type, artifact.Source, artifact.Path, createdAt); err != nil {
			return err
		}
	}
	return nil
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

func (s *server) latestFlowArtifactPath(businessID, typ string) string {
	businessID = strings.TrimSpace(businessID)
	typ = strings.TrimSpace(typ)
	if businessID == "" || typ == "" {
		return ""
	}
	if s.flows != nil {
		if path := s.flows.latestArtifactPath(businessID, typ); path != "" {
			return path
		}
	}
	root := filepath.Join(s.rootDir, "temp", "flow_runs")
	var latestPath string
	var latestMod time.Time
	typeFileSuffix := "__" + safeFilePart(typ) + ".json"
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(info.Name(), typeFileSuffix) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var payload map[string]interface{}
		if json.Unmarshal(raw, &payload) != nil {
			return nil
		}
		if jsonFirstString(payload, "artifact_type", "type") != typ {
			return nil
		}
		if payloadBusiness := jsonFirstString(payload, "business_id"); payloadBusiness != "" && payloadBusiness != businessID {
			return nil
		}
		if latestPath == "" || info.ModTime().After(latestMod) {
			latestPath = path
			latestMod = info.ModTime()
		}
		return nil
	})
	return latestPath
}

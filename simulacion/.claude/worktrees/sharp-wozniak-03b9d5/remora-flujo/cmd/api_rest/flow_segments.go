package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/manifest"
)

const (
	segmentModeAnalytical  = "analytical"
	segmentModeOperational = "operational"
)

func (s *server) ensureSemanticSegmentInitialized(runID string, flow flowManifest, available map[string]bool, artifacts map[string]flowRunArtifact) {
	if existing := activeSemanticSegmentPayload(artifacts); len(existing) > 0 {
		s.ensureSemanticSegmentSupportArtifacts(runID, flow, available, artifacts, existing)
		return
	}
	selected := selectedActionPayload(artifacts)
	if len(selected) == 0 {
		return
	}
	mode := semanticSegmentModeForSelection(selected)
	if mode == "" {
		return
	}
	ownerNode := semanticSegmentOwnerNode(flow, selected)
	segmentID := semanticSegmentID(selected, mode)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	active := map[string]interface{}{
		"artifact_type":         "segment.active.v1",
		"id":                    segmentID,
		"status":                "active",
		"mode":                  mode,
		"entered_at":            now,
		"selected_action_id":    jsonFirstString(selected, "id", "action_id"),
		"selected_action_label": jsonFirstString(selected, "label", "action_label"),
		"selected_bound_id":     semanticSelectionBoundID(selected),
		"owner_framework":       ownerNode.Framework,
		"owner_capability":      ownerNode.Capability,
		"owner_node":            ownerNode.ID,
		"source_artifact":       "action.selection.v1",
	}
	applySemanticSegmentSubject(active, artifacts)
	s.recordSemanticSegmentArtifact(runID, "segment_transition", "segment.transition.v1", map[string]interface{}{
		"artifact_type":         "segment.transition.v1",
		"segment_id":            segmentID,
		"status":                "entered",
		"mode":                  mode,
		"selected_action_id":    jsonFirstString(selected, "id", "action_id"),
		"selected_action_label": jsonFirstString(selected, "label", "action_label"),
		"selected_bound_id":     semanticSelectionBoundID(selected),
		"owner_framework":       ownerNode.Framework,
		"owner_capability":      ownerNode.Capability,
		"transitioned_at":       now,
	}, available, artifacts)
	s.recordSemanticSegmentArtifact(runID, "segment_active", "segment.active.v1", active, available, artifacts)
	s.ensureSemanticSegmentSupportArtifacts(runID, flow, available, artifacts, active)
}

func (s *server) ensureSemanticSegmentSupportArtifacts(runID string, flow flowManifest, available map[string]bool, artifacts map[string]flowRunArtifact, active map[string]interface{}) {
	if len(active) == 0 {
		return
	}
	segmentID := jsonFirstString(active, "id")
	mode := jsonFirstString(active, "mode")
	ownerNode := semanticSegmentOwnerNode(flow, selectedActionPayload(artifacts))
	if artifacts["segment.constraints.v1"].Type == "" {
		s.recordSemanticSegmentArtifact(runID, "segment_constraints", "segment.constraints.v1", map[string]interface{}{
			"artifact_type":         "segment.constraints.v1",
			"segment_id":            segmentID,
			"mode":                  mode,
			"blocked_artifacts":     blockedArtifactsForSemanticMode(mode),
			"return_to_owner_on":    returnArtifactsForSemanticMode(mode),
			"owner_retains_control": true,
			"delegate_mode":         "temporary",
		}, available, artifacts)
	}
	if artifacts["segment.owner.v1"].Type == "" {
		s.recordSemanticSegmentArtifact(runID, "segment_owner", "segment.owner.v1", map[string]interface{}{
			"artifact_type":   "segment.owner.v1",
			"segment_id":      segmentID,
			"framework":       ownerNode.Framework,
			"capability":      ownerNode.Capability,
			"node":            ownerNode.ID,
			"ownership_state": "retained",
		}, available, artifacts)
	}
}

func (s *server) maybeRefreshSemanticSegment(runID string, available map[string]bool, artifacts map[string]flowRunArtifact) {
	active, ok := artifacts["segment.active.v1"]
	if !ok {
		return
	}
	payload, _ := active.Payload.(map[string]interface{})
	if len(payload) == 0 {
		return
	}
	updated := cloneStringAnyMap(payload)
	applySemanticSegmentSubject(updated, artifacts)
	if !mapsEqualStringAny(payload, updated) {
		s.recordSemanticSegmentArtifact(runID, "segment_active", "segment.active.v1", updated, available, artifacts)
	}
}

func (s *server) recordSemanticSegmentDelegation(runID string, node flowNode, available map[string]bool, artifacts map[string]flowRunArtifact) {
	active := activeSemanticSegmentPayload(artifacts)
	if len(active) == 0 {
		return
	}
	ownerFramework := jsonFirstString(active, "owner_framework")
	ownerCapability := jsonFirstString(active, "owner_capability")
	if ownerFramework == "" || (node.Framework == ownerFramework && (ownerCapability == "" || node.Capability == ownerCapability)) {
		return
	}
	if payload, ok := artifacts["segment.delegate.v1"].Payload.(map[string]interface{}); ok &&
		jsonFirstString(payload, "delegate_framework") == node.Framework &&
		jsonFirstString(payload, "delegate_capability") == node.Capability {
		return
	}
	s.recordSemanticSegmentArtifact(runID, "segment_delegate_"+safeFilePart(node.ID), "segment.delegate.v1", map[string]interface{}{
		"artifact_type":       "segment.delegate.v1",
		"segment_id":          jsonFirstString(active, "id"),
		"owner_framework":     ownerFramework,
		"owner_capability":    ownerCapability,
		"delegate_framework":  node.Framework,
		"delegate_capability": node.Capability,
		"delegate_node":       node.ID,
		"delegated_at":        time.Now().UTC().Format(time.RFC3339Nano),
	}, available, artifacts)
}

func (s *server) recordSemanticSegmentReturn(runID string, node flowNode, produced []string, available map[string]bool, artifacts map[string]flowRunArtifact) {
	active := activeSemanticSegmentPayload(artifacts)
	if len(active) == 0 {
		return
	}
	if len(returnArtifactsForSemanticMode(jsonFirstString(active, "mode"))) == 0 {
		return
	}
	ownerFramework := jsonFirstString(active, "owner_framework")
	ownerCapability := jsonFirstString(active, "owner_capability")
	if ownerFramework == "" || (node.Framework == ownerFramework && (ownerCapability == "" || node.Capability == ownerCapability)) {
		return
	}
	returnArtifact := ""
	for _, artifact := range produced {
		if semanticArtifactMatchesRules(artifact, returnArtifactsForSemanticMode(jsonFirstString(active, "mode"))) {
			returnArtifact = artifact
			break
		}
	}
	if returnArtifact == "" {
		return
	}
	if payload, ok := artifacts["segment.return.v1"].Payload.(map[string]interface{}); ok &&
		jsonFirstString(payload, "delegate_framework") == node.Framework &&
		jsonFirstString(payload, "delegate_capability") == node.Capability &&
		jsonFirstString(payload, "return_artifact") == returnArtifact {
		return
	}
	s.recordSemanticSegmentArtifact(runID, "segment_return_"+safeFilePart(node.ID), "segment.return.v1", map[string]interface{}{
		"artifact_type":       "segment.return.v1",
		"segment_id":          jsonFirstString(active, "id"),
		"owner_framework":     ownerFramework,
		"owner_capability":    ownerCapability,
		"delegate_framework":  node.Framework,
		"delegate_capability": node.Capability,
		"return_artifact":     returnArtifact,
		"returned_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}, available, artifacts)
}

func (s *server) recordSemanticSegmentArtifact(runID, nodeID, artifactType string, payload map[string]interface{}, available map[string]bool, artifacts map[string]flowRunArtifact) {
	path := s.persistFlowArtifact(runID, nodeID, artifactType, payload)
	available[artifactType] = true
	artifacts[artifactType] = flowRunArtifact{
		Type:      artifactType,
		Source:    "flow_engine.segment",
		Node:      nodeID,
		Path:      path,
		Payload:   payload,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func semanticSegmentOwnerNode(flow flowManifest, selected map[string]interface{}) flowNode {
	if strings.EqualFold(strings.TrimSpace(jsonFirstString(selected, "id", "action_id")), "deep_analysis") {
		return flowNode{
			ID:         "radar_deep_analysis",
			Framework:  "radar",
			Capability: "analysis.deep_dive",
			Role:       flowRoleEntry,
		}
	}
	for _, node := range flow.Nodes {
		if node.Role == flowRoleEntry {
			return node
		}
	}
	for _, node := range flow.Nodes {
		if node.Framework == "foco" {
			return node
		}
	}
	if len(flow.Nodes) > 0 {
		return flow.Nodes[0]
	}
	return flowNode{}
}

func semanticSegmentModeForSelection(selected map[string]interface{}) string {
	boundID := semanticSelectionBoundID(selected)
	switch boundID {
	case "escalate", "review", "analyze", "analysis":
		return segmentModeAnalytical
	case "proceed", "execute", "send":
		return segmentModeOperational
	}
	actionID := strings.ToLower(strings.TrimSpace(jsonFirstString(selected, "id", "action_id")))
	label := strings.ToLower(strings.TrimSpace(jsonFirstString(selected, "label", "action_label")))
	switch actionID {
	case "deep_analysis":
		return segmentModeAnalytical
	case "quick_action", "send_email", "apply_mecanico_proposals":
		return segmentModeOperational
	}
	if strings.Contains(label, "analisis") || strings.Contains(label, "analysis") {
		return segmentModeAnalytical
	}
	return ""
}

func semanticSegmentID(selected map[string]interface{}, mode string) string {
	key := jsonFirstString(selected, "id", "action_id")
	if key == "" {
		key = semanticSelectionBoundID(selected)
	}
	if key == "" {
		key = mode
	}
	return "segment_" + safeFilePart(key)
}

func semanticSelectionBoundID(selected map[string]interface{}) string {
	if boundID := strings.TrimSpace(jsonFirstString(selected, "bound_id")); boundID != "" {
		return boundID
	}
	switch strings.ToLower(strings.TrimSpace(jsonFirstString(selected, "id", "action_id"))) {
	case "deep_analysis":
		return "escalate"
	case "quick_action", "send_email", "apply_mecanico_proposals":
		return "proceed"
	case "skip_case":
		return "postpone"
	default:
		return ""
	}
}

func activeSemanticSegmentPayload(artifacts map[string]flowRunArtifact) map[string]interface{} {
	if payload, ok := artifacts["segment.active.v1"].Payload.(map[string]interface{}); ok && len(payload) > 0 {
		return payload
	}
	return nil
}

func blockedArtifactsForSemanticMode(mode string) []string {
	switch mode {
	case segmentModeAnalytical:
		return []string{
			"contact.destination.v1",
			"message.draft.v1",
			"message.sent.v1",
			"credentials.smtp",
			"credentials.cpanel",
			"credentials.status.v1",
		}
	default:
		return nil
	}
}

func returnArtifactsForSemanticMode(mode string) []string {
	switch mode {
	case segmentModeAnalytical:
		return []string{
			"analysis.schema.v1",
			"analysis.plan.v1",
			"analysis.proposal.v1",
			"collection.priority_list.v1",
			"entity_360.v1",
			"answer.grounded.v1",
			"auditor.findings.v1",
			"data.gaps.v1",
		}
	default:
		return nil
	}
}

func semanticSegmentBlockedByNode(node flowNode, contract nodeContract, artifacts map[string]flowRunArtifact) (bool, string) {
	active := activeSemanticSegmentPayload(artifacts)
	if len(active) == 0 || jsonFirstString(active, "mode") != segmentModeAnalytical {
		return false, ""
	}
	ownerFramework := jsonFirstString(active, "owner_framework")
	ownerCapability := jsonFirstString(active, "owner_capability")
	if node.Framework == ownerFramework && (ownerCapability == "" || node.Capability == ownerCapability) {
		return false, ""
	}
	if node.Role == flowRoleEntry {
		return true, fmt.Sprintf("segmento analítico transferido a %s: %s no retoma control hasta retorno explícito", ownerFramework, node.Framework)
	}
	if containsString(uniqueStrings(append(append([]string{}, contract.Outputs...), contract.Produces...)), "action.options.v1") {
		return true, "segmento analítico activo: se suprime la reemisión de action_options hasta retorno explícito del owner"
	}
	if hasExternalSideEffect(contract.Policies) || hasRuntimeMutationPolicy(contract.Policies) {
		return true, "segmento analítico activo: se bloquean acciones operativas con side effects o mutación de estado"
	}
	candidates := uniqueStrings(append(append(append([]string{}, contract.Inputs...), contract.Requires...), append(contract.Outputs, contract.Produces...)...))
	for _, artifact := range candidates {
		if semanticArtifactBlocked(artifact, artifacts) {
			return true, fmt.Sprintf("segmento analítico activo: %s queda fuera del tramo por requerir o producir %s", node.Capability, artifact)
		}
	}
	return false, ""
}

func semanticArtifactBlocked(artifact string, artifacts map[string]flowRunArtifact) bool {
	active := activeSemanticSegmentPayload(artifacts)
	if len(active) == 0 {
		return false
	}
	return semanticArtifactMatchesRules(artifact, blockedArtifactsForSemanticMode(jsonFirstString(active, "mode")))
}

func semanticArtifactMatchesRules(artifact string, rules []string) bool {
	artifact = strings.TrimSpace(artifact)
	if artifact == "" {
		return false
	}
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if artifact == rule || strings.HasPrefix(artifact, rule+".") || strings.HasPrefix(artifact, rule+"_") {
			return true
		}
	}
	return false
}

func (s *server) flowTerminalRequirementsForArtifacts(flow flowManifest, artifacts map[string]flowRunArtifact) []string {
	nodes := flow.Nodes
	for i := len(nodes) - 1; i >= 0; i-- {
		node := nodes[i]
		m := s.allManifests[node.Framework]
		if m == nil {
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			continue
		}
		if blocked, _ := semanticSegmentBlockedByNode(node, contract, artifacts); blocked {
			continue
		}
		if containsString(contract.Policies, "cycle_terminal") ||
			containsString(contract.Produces, "message.sent.v1") ||
			node.Framework == "mensajero" {
			return uniqueStrings(append(append([]string{}, contract.Requires...), contract.Inputs...))
		}
	}
	if len(activeSemanticSegmentPayload(artifacts)) > 0 && jsonFirstString(activeSemanticSegmentPayload(artifacts), "mode") == segmentModeAnalytical {
		return nil
	}
	return flowTerminalRequirements(flow)
}

func (s *server) flowRequiresArtifactInActiveSegment(flow flowManifest, artifacts map[string]flowRunArtifact, artifact string) bool {
	for _, node := range flow.Nodes {
		m := s.allManifests[node.Framework]
		if m == nil {
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			continue
		}
		if blocked, _ := semanticSegmentBlockedByNode(node, contract, artifacts); blocked {
			continue
		}
		if containsString(append(contract.Inputs, contract.Requires...), artifact) {
			return true
		}
	}
	return false
}

func applySemanticSegmentSubject(payload map[string]interface{}, artifacts map[string]flowRunArtifact) {
	entity := firstArtifactMap(artifacts, "entity.ref.v1")
	if len(entity) > 0 {
		if ref := jsonFirstString(entity, "entity_ref", "id", "ref", "code"); ref != "" {
			payload["subject_ref"] = ref
		}
		if typ := jsonFirstString(entity, "entity_type", "type", "kind"); typ != "" {
			payload["subject_type"] = typ
		}
		if name := jsonFirstString(entity, "name", "label"); name != "" {
			payload["subject_name"] = name
		}
	}
	if task := firstArtifactMap(artifacts, "task.next", "focus.next_task.v1"); len(task) > 0 {
		if id := jsonFirstString(task, "task_id", "id"); id != "" {
			payload["task_id"] = id
		}
		if title := jsonFirstString(task, "task_title", "title", "name"); title != "" {
			payload["task_title"] = title
		}
	}
}

func applySemanticSegmentToStep(step *flowRunStep, artifacts map[string]flowRunArtifact) {
	active := activeSemanticSegmentPayload(artifacts)
	if len(active) == 0 {
		return
	}
	step.SegmentID = jsonFirstString(active, "id")
	step.SegmentMode = jsonFirstString(active, "mode")
	step.SegmentOwner = jsonFirstString(active, "owner_framework")
	if step.Framework == step.SegmentOwner && (jsonFirstString(active, "owner_capability") == "" || step.Capability == jsonFirstString(active, "owner_capability")) {
		step.SegmentRole = "owner"
		return
	}
	step.SegmentRole = "delegate"
}

func normalizeFlowTimelineSegments(steps []flowRunStep, artifacts map[string]flowRunArtifact) []flowRunStep {
	for i := range steps {
		applySemanticSegmentToStep(&steps[i], artifacts)
	}
	return steps
}

func normalizeFlowRequiredInputsWithSegments(needs []flowRequiredInput, artifacts map[string]flowRunArtifact) []flowRequiredInput {
	active := activeSemanticSegmentPayload(artifacts)
	out := make([]flowRequiredInput, 0, len(needs))
	for i := range needs {
		if strings.TrimSpace(needs[i].Visibility) == "" {
			needs[i].Visibility = flowStepVisibilityUserFacing
		}
		if len(active) == 0 {
			out = append(out, needs[i])
			continue
		}
		if semanticArtifactBlocked(needs[i].Artifact, artifacts) {
			continue
		}
		needs[i].SegmentID = jsonFirstString(active, "id")
		needs[i].SegmentMode = jsonFirstString(active, "mode")
		needs[i].SegmentOwner = jsonFirstString(active, "owner_framework")
		if needs[i].Framework == needs[i].SegmentOwner && (jsonFirstString(active, "owner_capability") == "" || needs[i].Capability == jsonFirstString(active, "owner_capability")) {
			needs[i].SegmentRole = "owner"
		} else {
			needs[i].SegmentRole = "delegate"
		}
		out = append(out, needs[i])
	}
	return out
}

func semanticSegmentOwnerRuntimeNode(artifacts map[string]flowRunArtifact) (flowNode, bool) {
	active := activeSemanticSegmentPayload(artifacts)
	if len(active) == 0 {
		return flowNode{}, false
	}
	node := flowNode{
		ID:         jsonFirstString(active, "owner_node"),
		Framework:  jsonFirstString(active, "owner_framework"),
		Capability: jsonFirstString(active, "owner_capability"),
		Role:       flowRoleEntry,
	}
	if node.ID == "" || node.Framework == "" || node.Capability == "" {
		return flowNode{}, false
	}
	return node, true
}

func injectSemanticSegmentOwnerNode(nodes []flowNode, owner flowNode) []flowNode {
	if owner.ID == "" {
		return nodes
	}
	if flowNodeExists(nodes, owner.ID) {
		return nodes
	}
	insertAt := 0
	for idx, node := range nodes {
		if node.Role == flowRoleBootstrap {
			insertAt = idx + 1
		}
	}
	out := make([]flowNode, 0, len(nodes)+1)
	out = append(out, nodes[:insertAt]...)
	out = append(out, owner)
	out = append(out, nodes[insertAt:]...)
	return out
}

func flowNodeExists(nodes []flowNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func cloneStringAnyMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mapsEqualStringAny(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", b[key]) {
			return false
		}
	}
	return true
}

// --- Segment session: multi-turn conversational ownership ---

const segmentSessionArtifact = "segment.session.v1"

// segmentSessionPayload returns the active session payload if present.
func segmentSessionPayload(artifacts map[string]flowRunArtifact) map[string]interface{} {
	if art, ok := artifacts[segmentSessionArtifact]; ok {
		if payload, ok := art.Payload.(map[string]interface{}); ok && len(payload) > 0 {
			return payload
		}
	}
	return nil
}

// segmentSessionActive returns true when there is an active conversational session.
func segmentSessionActive(artifacts map[string]flowRunArtifact) bool {
	session := segmentSessionPayload(artifacts)
	return len(session) > 0 && jsonFirstString(session, "status") == "active"
}

// segmentSessionOwner returns the owner framework and capability of the active session.
func segmentSessionOwner(artifacts map[string]flowRunArtifact) (framework, capability, followupCmd string, ok bool) {
	session := segmentSessionPayload(artifacts)
	if len(session) == 0 || jsonFirstString(session, "status") != "active" {
		return "", "", "", false
	}
	framework = jsonFirstString(session, "owner_framework")
	capability = jsonFirstString(session, "owner_capability")
	followupCmd = jsonFirstString(session, "followup_command")
	if framework == "" || followupCmd == "" {
		return "", "", "", false
	}
	return framework, capability, followupCmd, true
}

// segmentSessionTurnCount returns the current turn count.
func segmentSessionTurnCount(artifacts map[string]flowRunArtifact) int {
	session := segmentSessionPayload(artifacts)
	if len(session) == 0 {
		return 0
	}
	return jsonFirstInt(session, "turn_count")
}

// segmentSessionMaxTurns returns the configured max turns (0 = unlimited).
func segmentSessionMaxTurns(artifacts map[string]flowRunArtifact) int {
	session := segmentSessionPayload(artifacts)
	if len(session) == 0 {
		return 0
	}
	return jsonFirstInt(session, "max_turns")
}

// segmentSessionAllowedDelegates returns the list of capabilities allowed as delegates.
func segmentSessionAllowedDelegates(artifacts map[string]flowRunArtifact) []string {
	session := segmentSessionPayload(artifacts)
	if len(session) == 0 {
		return nil
	}
	raw, ok := session["allowed_delegates"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// createSegmentSession persists a segment.session.v1 artifact for a capability
// that declared session.conversational = true.
func (s *server) createSegmentSession(runID, businessID string, node flowNode, cap manifest.CapabilitySpec, available map[string]bool, artifacts map[string]flowRunArtifact) {
	if cap.Session == nil || !cap.Session.Conversational {
		return
	}
	active := activeSemanticSegmentPayload(artifacts)
	segmentID := ""
	if len(active) > 0 {
		segmentID = jsonFirstString(active, "id")
	}
	allowedDelegates := make([]interface{}, 0, len(cap.Session.AllowedDelegates))
	for _, d := range cap.Session.AllowedDelegates {
		allowedDelegates = append(allowedDelegates, d)
	}
	continueSignals := make([]interface{}, 0, len(cap.Session.ContinueSignals))
	for _, sig := range cap.Session.ContinueSignals {
		continueSignals = append(continueSignals, sig)
	}
	operationalSignals := make([]interface{}, 0, len(cap.Session.OperationalSignals))
	for _, sig := range cap.Session.OperationalSignals {
		operationalSignals = append(operationalSignals, sig)
	}
	exitSignals := make([]interface{}, 0, len(cap.Session.ExitSignals))
	for _, sig := range cap.Session.ExitSignals {
		exitSignals = append(exitSignals, sig)
	}
	payload := map[string]interface{}{
		"artifact_type":        segmentSessionArtifact,
		"segment_id":           segmentID,
		"business_id":          businessID,
		"mode":                 segmentModeAnalytical,
		"status":               "active",
		"owner_framework":      node.Framework,
		"owner_capability":     node.Capability,
		"owner_conversational": true,
		"followup_command":     cap.Session.FollowupCommand,
		"turn_count":           0,
		"max_turns":            cap.Session.MaxTurns,
		"allowed_delegates":    allowedDelegates,
		"continue_signals":     continueSignals,
		"operational_signals":  operationalSignals,
		"exit_signals":         exitSignals,
		"created_at":           time.Now().UTC().Format(time.RFC3339Nano),
	}
	applySemanticSegmentSubject(payload, artifacts)
	s.recordSemanticSegmentArtifact(runID, "segment_session", segmentSessionArtifact, payload, available, artifacts)
}

// concludeSegmentSession marks the session as concluded and returns the updated payload.
func (s *server) concludeSegmentSession(runID, reason string, available map[string]bool, artifacts map[string]flowRunArtifact) {
	session := segmentSessionPayload(artifacts)
	if len(session) == 0 {
		return
	}
	updated := cloneStringAnyMap(session)
	updated["status"] = "concluded"
	updated["concluded_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	updated["concluded_reason"] = reason
	s.recordSemanticSegmentArtifact(runID, "segment_session", segmentSessionArtifact, updated, available, artifacts)
}

// incrementSegmentSessionTurn bumps the turn count and persists.
func (s *server) incrementSegmentSessionTurn(runID string, available map[string]bool, artifacts map[string]flowRunArtifact) int {
	session := segmentSessionPayload(artifacts)
	if len(session) == 0 {
		return 0
	}
	updated := cloneStringAnyMap(session)
	count := jsonFirstInt(session, "turn_count") + 1
	updated["turn_count"] = count
	updated["last_turn_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	s.recordSemanticSegmentArtifact(runID, "segment_session", segmentSessionArtifact, updated, available, artifacts)
	return count
}

// --- Session lookup from disk (for runLoop integration) ---

// activeSessionInfo holds the essential fields of an active segment session
// needed by the orchestrator's Step 0.
type activeSessionInfo struct {
	Framework          string
	Capability         string
	FollowupCmd        string
	TurnCount          int
	MaxTurns           int
	ContinueSignals    []string
	OperationalSignals []string
	ExitSignals        []string
	AllowedDelegates   []string
	ConversationID     string
	SegmentID          string
	// Path is the on-disk artifact path so the orchestrator can update it.
	Path string
}

// loadActiveSessionFromDisk looks up the latest segment.session.v1 artifact
// for a given businessID scoped to a conversation. A session is visible if:
//   - it belongs to this conversationID, OR
//   - it has no conversation_id yet (unclaimed — created by the flow runner
//     before any runLoop claimed it).
//
// This prevents one conversation from hijacking another's analytical session.
func (s *server) loadActiveSessionFromDisk(businessID, conversationID string) (*activeSessionInfo, error) {
	candidates := s.activeSessionCandidates(businessID)
	if len(candidates) == 0 {
		return nil, nil
	}
	// Prefer an already-claimed session for this exact conversation.
	for _, c := range candidates {
		if c.ConversationID == conversationID {
			return c, nil
		}
	}
	// Then allow the newest unclaimed session to be claimed by this conversation.
	for _, c := range candidates {
		if c.ConversationID == "" {
			return c, nil
		}
	}
	return nil, nil
}

// activeSessionCandidates returns active segment.session.v1 artifacts for a
// business ordered newest-first. Claimed sessions for other conversations are
// kept in the result so the caller can prefer exact matches over unclaimed
// sessions.
func (s *server) activeSessionCandidates(businessID string) []*activeSessionInfo {
	root := filepath.Join(s.rootDir, "temp", "flow_runs")
	typeFileSuffix := "__" + safeFilePart(segmentSessionArtifact) + ".json"
	type candidate struct {
		info *activeSessionInfo
		mod  time.Time
	}
	var candidates []candidate
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
		if jsonFirstString(payload, "artifact_type", "type") != segmentSessionArtifact {
			return nil
		}
		if jsonFirstString(payload, "status") != "active" {
			return nil
		}
		if payloadBusiness := jsonFirstString(payload, "business_id"); payloadBusiness != "" && payloadBusiness != businessID {
			return nil
		}
		parsed := sessionInfoFromPayload(path, payload)
		if parsed == nil {
			return nil
		}
		candidates = append(candidates, candidate{info: parsed, mod: info.ModTime()})
		return nil
	})
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].mod.After(candidates[i].mod) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	out := make([]*activeSessionInfo, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.info)
	}
	return out
}

func sessionInfoFromPayload(path string, payload map[string]interface{}) *activeSessionInfo {
	fw := jsonFirstString(payload, "owner_framework")
	cmd := jsonFirstString(payload, "followup_command")
	if fw == "" || cmd == "" {
		return nil
	}
	info := &activeSessionInfo{
		Framework:      fw,
		Capability:     jsonFirstString(payload, "owner_capability"),
		FollowupCmd:    cmd,
		TurnCount:      jsonFirstInt(payload, "turn_count"),
		MaxTurns:       jsonFirstInt(payload, "max_turns"),
		ConversationID: jsonFirstString(payload, "conversation_id"),
		SegmentID:      jsonFirstString(payload, "segment_id"),
		Path:           path,
	}
	info.ContinueSignals = extractStringSlice(payload, "continue_signals")
	info.OperationalSignals = extractStringSlice(payload, "operational_signals")
	info.ExitSignals = extractStringSlice(payload, "exit_signals")
	info.AllowedDelegates = extractStringSlice(payload, "allowed_delegates")
	return info
}

// claimSessionForConversation stamps the conversation_id into the session
// artifact on its first access from a runLoop. This binds the session to
// exactly one conversation so parallel conversations of the same business
// don't interfere.
func (s *server) claimSessionForConversation(path, conversationID string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	if existing := jsonFirstString(payload, "conversation_id"); existing != "" {
		return // already claimed
	}
	payload["conversation_id"] = conversationID
	payload["claimed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	updated, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, updated, 0644)
}

// extractStringSlice pulls a []string from a JSON array field.
func extractStringSlice(payload map[string]interface{}, key string) []string {
	raw, ok := payload[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// concludeSessionOnDisk marks the session artifact as concluded directly on
// disk, without requiring an active flow run context.
func (s *server) concludeSessionOnDisk(path, reason string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	payload["status"] = "concluded"
	payload["concluded_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	payload["concluded_reason"] = reason
	updated, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, updated, 0644)
}

// incrementSessionOnDisk bumps turn_count directly on disk.
func (s *server) incrementSessionOnDisk(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var payload map[string]interface{}
	if json.Unmarshal(raw, &payload) != nil {
		return 0
	}
	count := jsonFirstInt(payload, "turn_count") + 1
	payload["turn_count"] = count
	payload["last_turn_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	updated, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return count
	}
	_ = os.WriteFile(path, updated, 0644)
	return count
}

// --- Segment intent classification ---

// segmentIntentType represents the classified intent of a user message within
// a conversational session.
type segmentIntentType string

const (
	segmentIntentContinue    segmentIntentType = "continue"
	segmentIntentOperational segmentIntentType = "operational"
	segmentIntentExit        segmentIntentType = "exit"
)

// classifySegmentIntent determines the user's intent within an active
// conversational session. Three-way classification:
//
//   - continue:    user wants to keep analyzing ("profundiza", "por qué?")
//   - operational: user wants to act with available data ("avanza", "manda")
//   - exit:        user wants to leave/skip ("siguiente caso", "skip")
//
// The classifier uses manifest signals first (highest priority), then
// structural heuristics. A key rule: if the message contains BOTH an
// affirmation ("ok", "dale") AND an analytical keyword ("profundiza",
// "riesgo"), it's continue — this prevents premature session closure on
// phrases like "ok, pero profundiza más el riesgo".
func classifySegmentIntent(userAnswer string, session *activeSessionInfo) segmentIntentType {
	if session == nil {
		return segmentIntentOperational
	}
	lower := strings.ToLower(strings.TrimSpace(userAnswer))
	if lower == "" {
		return segmentIntentContinue
	}

	// Max turns exceeded → force operational handoff.
	if session.MaxTurns > 0 && session.TurnCount >= session.MaxTurns {
		return segmentIntentOperational
	}

	hasContinueSignal := matchesAnySignal(lower, session.ContinueSignals)
	hasOperationalSignal := matchesAnySignal(lower, session.OperationalSignals)
	hasExitSignal := matchesAnySignal(lower, session.ExitSignals)

	// Rule: continue signal always wins when present. This handles
	// "ok, pero profundiza más" — the "profundiza" overrides the "ok".
	if hasContinueSignal {
		return segmentIntentContinue
	}

	// Explicit exit signal (dismiss/skip) without continue → exit.
	if hasExitSignal {
		return segmentIntentExit
	}

	// Explicit operational signal without continue → operational.
	if hasOperationalSignal {
		return segmentIntentOperational
	}

	// Structural heuristic: questions are analytical.
	if strings.HasSuffix(strings.TrimSpace(userAnswer), "?") {
		return segmentIntentContinue
	}

	// Default bias: stay in analysis. The user controls exit explicitly.
	// This is the "autonomía controlada" principle — the session persists
	// until the user makes a clear decision to leave or act.
	return segmentIntentContinue
}

// matchesAnySignal checks if the input contains any of the signals.
// Signals are matched case-insensitively as substrings.
func matchesAnySignal(lower string, signals []string) bool {
	for _, sig := range signals {
		if strings.Contains(lower, strings.ToLower(sig)) {
			return true
		}
	}
	return false
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"channel/manifest"
	"github.com/gorilla/mux"
)

type capabilityProviderInfo struct {
	Capability  string   `json:"capability"`
	Framework   string   `json:"framework"`
	Command     string   `json:"command,omitempty"`
	Description string   `json:"description,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
	Requires    []string `json:"requires,omitempty"`
	Produces    []string `json:"produces,omitempty"`
	Execution   string   `json:"execution,omitempty"`
	Policies    []string `json:"policies,omitempty"`
	Source      string   `json:"source"`
}

type capabilityRegistry map[string][]capabilityProviderInfo

type flowManifest struct {
	ID                string          `json:"id"`
	BusinessID        string          `json:"business_id,omitempty"`
	Audience          string          `json:"audience,omitempty"`
	Intent            flowIntent      `json:"intent,omitempty"`
	Lifecycle         flowLifecycle   `json:"lifecycle,omitempty"`
	ProvidedArtifacts []string        `json:"provided_artifacts,omitempty"`
	Nodes             []flowNode      `json:"nodes"`
	Edges             []flowEdge       `json:"edges,omitempty"`
	Policies          []string        `json:"policies,omitempty"`
	Provenance        flowProvenance   `json:"provenance,omitempty"`
	Derivation        *flowDerivation  `json:"derivation,omitempty"`
}

type flowIntent struct {
	Goal            string   `json:"goal,omitempty"`
	OperatorRole    string   `json:"operator_role,omitempty"`
	SuccessCriteria string   `json:"success_criteria,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	Description     string   `json:"description,omitempty"`
	CapabilityHint  string   `json:"capability_hint,omitempty"`
}

type flowLifecycle struct {
	Entry  flowLifecycleEntry `json:"entry,omitempty"`
	Tutela flowLifecycleEntry `json:"tutela,omitempty"`
}

type flowLifecycleEntry struct {
	Framework  string `json:"framework,omitempty"`
	Capability string `json:"capability,omitempty"`
}

type flowNode struct {
	ID         string            `json:"id"`
	Framework  string            `json:"framework"`
	Capability string            `json:"capability,omitempty"`
	Command    string            `json:"command,omitempty"`
	Role       string            `json:"role,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	Inputs     []string          `json:"inputs,omitempty"`
	Outputs    []string          `json:"outputs,omitempty"`
	Requires   []string          `json:"requires,omitempty"`
	Produces   []string          `json:"produces,omitempty"`
	Policies   []string          `json:"policies,omitempty"`
}

type flowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

const (
	flowRoleBootstrap  = "bootstrap"
	flowRoleEntry      = "entry"
	flowRolePipeline   = "pipeline"
	flowRoleResolution = "resolution"
)

// Framework resolution modes: how a framework handles issues it encounters.
const (
	resolutionAutonomous  = "autonomous"  // resolves by querying services (Sabio, Radar)
	resolutionInteractive = "interactive" // needs user input (Hosting, Mensajero)
	resolutionHybrid      = "hybrid"      // tries autonomous, falls back to user (Mecánico)
)

func resolutionModeFromPolicies(policies []string) string {
	if containsString(policies, "resolution_interactive") {
		return resolutionInteractive
	}
	if containsString(policies, "resolution_hybrid") {
		return resolutionHybrid
	}
	return resolutionAutonomous
}

func resolutionModeForCapability(m *manifest.Manifest, capability string) string {
	if m == nil {
		return resolutionAutonomous
	}
	if cap, ok := findManifestCapability(m, capability); ok {
		return resolutionModeFromPolicies(cap.Policies)
	}
	return resolutionAutonomous
}

// gapResolution maps a gap kind to the capability/artifact that can resolve it.
type gapResolution struct {
	GapKind    string
	Capability string
	Produces   string
}

// gapResolutionRegistry returns strategies for resolving different data gap kinds.
func gapResolutionRegistry() []gapResolution {
	return []gapResolution{
		{GapKind: "contact", Capability: "contact.lookup", Produces: "contact.destination.v1"},
		{GapKind: "email", Capability: "contact.lookup", Produces: "contact.destination.v1"},
		{GapKind: "correo", Capability: "contact.lookup", Produces: "contact.destination.v1"},
		{GapKind: "credentials", Capability: "credentials.smtp.check", Produces: "credentials.status.v1"},
		{GapKind: "smtp", Capability: "credentials.smtp.check", Produces: "credentials.status.v1"},
		{GapKind: "data_quality", Capability: "action.fix.propose_all_auto", Produces: "mecanico.proposals.v1"},
		{GapKind: "schema", Capability: "action.fix.propose_all_auto", Produces: "mecanico.proposals.v1"},
		{GapKind: "empty_required", Capability: "action.fix.propose_all_auto", Produces: "mecanico.proposals.v1"},
		{GapKind: "fk_orphan", Capability: "action.fix.propose_all_auto", Produces: "mecanico.proposals.v1"},
	}
}

// findGapResolution returns the best resolution strategy for a given gap kind.
func findGapResolution(kind string) (gapResolution, bool) {
	kindLower := strings.ToLower(kind)
	for _, r := range gapResolutionRegistry() {
		if strings.Contains(kindLower, r.GapKind) {
			return r, true
		}
	}
	return gapResolution{}, false
}

func normalizeFlowLifecycleRoles(f *flowManifest, manifestSets ...map[string]*manifest.Manifest) {
	if f == nil {
		return
	}
	var manifests map[string]*manifest.Manifest
	if len(manifestSets) > 0 {
		manifests = manifestSets[0]
	}
	hasFoco := false
	for _, n := range f.Nodes {
		if isFocoNode(n, manifests) {
			hasFoco = true
			break
		}
	}
	hasEntry := false
	for i := range f.Nodes {
		f.Nodes[i].Role = normalizeFlowNodeRole(f.Nodes[i].Role)
		if isFocoNode(f.Nodes[i], manifests) {
			f.Nodes[i].Role = flowRoleEntry
		} else if hasFoco && f.Nodes[i].Role == flowRoleEntry {
			f.Nodes[i].Role = flowRolePipeline
		}
		if f.Nodes[i].Role == flowRoleEntry {
			hasEntry = true
		}
	}
	if len(f.Nodes) == 0 {
		return
	}
	if f.Nodes[0].Role == "" && isBootstrapCandidate(f.Nodes[0]) && len(f.Nodes) > 1 {
		f.Nodes[0].Role = flowRoleBootstrap
	}
	if !hasEntry {
		for i := range f.Nodes {
			if f.Nodes[i].Role == flowRoleBootstrap {
				continue
			}
			f.Nodes[i].Role = flowRoleEntry
			hasEntry = true
			break
		}
	}
	for i := range f.Nodes {
		if f.Nodes[i].Role == "" {
			f.Nodes[i].Role = flowRolePipeline
		}
	}
}

func prepareFlowManifestLifecycle(f *flowManifest, manifestSets ...map[string]*manifest.Manifest) {
	var manifests map[string]*manifest.Manifest
	if len(manifestSets) > 0 {
		manifests = manifestSets[0]
	}
	migrateLegacyFlowNodes(f)
	normalizeFlowLifecycleRoles(f, manifests)
	applyConfiguredFlowEntry(f)
	ensureFocoEntry(f, manifests)
	normalizeFlowLifecycleRoles(f, manifests)
	applyConfiguredFlowEntry(f)
	promotePriorityListProducersBeforeFocoEntry(f, manifests)
	orderFlowLifecycleNodes(f)
}

func migrateLegacyFlowNodes(f *flowManifest) {
	if f == nil {
		return
	}
}

func ensureFocoEntry(f *flowManifest, manifests map[string]*manifest.Manifest) {
	if f == nil || len(f.Nodes) == 0 {
		return
	}
	if entryFramework, _ := configuredFlowEntry(f); entryFramework != "" {
		return
	}
	if strings.TrimSpace(f.BusinessID) == "" {
		return
	}
	for _, n := range f.Nodes {
		if isFocoNode(n, manifests) {
			return
		}
	}
	insertAt := 0
	hasPriorityList := false
	for i, n := range f.Nodes {
		if isBootstrapCandidate(n) {
			insertAt = i + 1
		}
		if producesArtifact(n, manifests, "collection.priority_list.v1") {
			hasPriorityList = true
		}
	}
	capability := "focus.entry_briefing"
	if hasPriorityList {
		capability = "focus.next_collection_task"
	}
	_, provider, ok := findProviderForCapabilityInManifests(manifests, capability)
	if !ok {
		return
	}
	node := flowNode{
		ID:         uniqueFlowNodeID(f.Nodes, "node_foco_entry"),
		Framework:  provider,
		Capability: capability,
		Role:       flowRoleEntry,
	}
	f.Nodes = append(f.Nodes, flowNode{})
	copy(f.Nodes[insertAt+1:], f.Nodes[insertAt:])
	f.Nodes[insertAt] = node
	rebuildLinearFlowEdges(f)
}

func configuredFlowEntry(f *flowManifest) (string, string) {
	if f == nil {
		return "", ""
	}
	return strings.TrimSpace(f.Lifecycle.Entry.Framework), strings.TrimSpace(f.Lifecycle.Entry.Capability)
}

func applyConfiguredFlowEntry(f *flowManifest) {
	framework, capability := configuredFlowEntry(f)
	if f == nil || framework == "" {
		return
	}
	for i := range f.Nodes {
		if f.Nodes[i].Framework != framework {
			continue
		}
		if capability != "" && f.Nodes[i].Capability != capability {
			continue
		}
		f.Nodes[i].Role = flowRoleEntry
	}
}

func promotePriorityListProducersBeforeFocoEntry(f *flowManifest, manifests map[string]*manifest.Manifest) {
	if f == nil {
		return
	}
	needsPriorityList := false
	for _, n := range f.Nodes {
		if isFocoNode(n, manifests) && requiresArtifact(n, manifests, "collection.priority_list.v1") {
			needsPriorityList = true
			break
		}
	}
	if !needsPriorityList {
		return
	}
	for i := range f.Nodes {
		if isFocoNode(f.Nodes[i], manifests) {
			continue
		}
		if producesPriorityList(f.Nodes[i], manifests) {
			f.Nodes[i].Role = flowRoleBootstrap
		}
	}
}

func producesPriorityList(n flowNode, manifests map[string]*manifest.Manifest) bool {
	return producesArtifact(n, manifests, "collection.priority_list.v1")
}

func orderFlowLifecycleNodes(f *flowManifest) {
	if f == nil || len(f.Nodes) == 0 {
		return
	}
	bootstraps := []flowNode{}
	entries := []flowNode{}
	pipeline := []flowNode{}
	for _, n := range f.Nodes {
		switch n.Role {
		case flowRoleBootstrap:
			bootstraps = append(bootstraps, n)
		case flowRoleEntry:
			entries = append(entries, n)
		default:
			n.Role = flowRolePipeline
			pipeline = append(pipeline, n)
		}
	}
	f.Nodes = append(append(bootstraps, entries...), pipeline...)
	rebuildLinearFlowEdges(f)
}

func isFocoNode(n flowNode, manifests map[string]*manifest.Manifest) bool {
	return n.Role == flowRoleEntry || nodeHasPolicy(n, manifests, "entrypoint")
}

func producesArtifact(n flowNode, manifests map[string]*manifest.Manifest, artifact string) bool {
	if containsString(append(n.Produces, n.Outputs...), artifact) {
		return true
	}
	if m := manifests[n.Framework]; m != nil {
		if cap, ok := findManifestCapability(m, n.Capability); ok {
			return containsString(append(cap.Produces, cap.Outputs...), artifact)
		}
	}
	return false
}

func requiresArtifact(n flowNode, manifests map[string]*manifest.Manifest, artifact string) bool {
	if containsString(append(n.Requires, n.Inputs...), artifact) {
		return true
	}
	if m := manifests[n.Framework]; m != nil {
		if cap, ok := findManifestCapability(m, n.Capability); ok {
			return containsString(append(cap.Requires, cap.Inputs...), artifact)
		}
	}
	return false
}

func nodeHasPolicy(n flowNode, manifests map[string]*manifest.Manifest, policy string) bool {
	if containsString(n.Policies, policy) {
		return true
	}
	if m := manifests[n.Framework]; m != nil {
		if cap, ok := findManifestCapability(m, n.Capability); ok {
			return containsString(cap.Policies, policy)
		}
	}
	return false
}

func findProviderForCapabilityInManifests(manifests map[string]*manifest.Manifest, capability string) (*manifest.Manifest, string, bool) {
	if len(manifests) == 0 {
		return nil, "", false
	}
	names := make([]string, 0, len(manifests))
	for name := range manifests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		m := manifests[name]
		if m == nil {
			continue
		}
		if _, ok := findManifestCapability(m, capability); ok {
			return m, name, true
		}
	}
	return nil, "", false
}

func uniqueFlowNodeID(nodes []flowNode, base string) string {
	seen := map[string]bool{}
	for _, n := range nodes {
		seen[n.ID] = true
	}
	if !seen[base] {
		return base
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s_%d", base, i)
		if !seen[id] {
			return id
		}
	}
}

func rebuildLinearFlowEdges(f *flowManifest) {
	if f == nil {
		return
	}
	f.Edges = nil
	for i := 1; i < len(f.Nodes); i++ {
		f.Edges = append(f.Edges, flowEdge{From: f.Nodes[i-1].ID, To: f.Nodes[i].ID})
	}
}

func normalizeFlowNodeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case flowRoleBootstrap, "primer", "preflight", "setup", "preparacion", "preparación":
		return flowRoleBootstrap
	case flowRoleEntry, "inicio", "frontdesk", "first_contact":
		return flowRoleEntry
	case flowRolePipeline, "main", "principal":
		return flowRolePipeline
	default:
		return ""
	}
}

func isBootstrapCandidate(n flowNode) bool {
	key := strings.ToLower(n.Framework + "." + n.Capability)
	if strings.Contains(key, "radar.") || strings.Contains(key, "indexa.") {
		return true
	}
	for _, p := range append(n.Produces, n.Outputs...) {
		p = strings.ToLower(p)
		if strings.Contains(p, "priority") || strings.Contains(p, "inventory") || strings.Contains(p, "semantic_pack") {
			return true
		}
	}
	return false
}

type flowValidationIssue struct {
	Code     string   `json:"code"`
	Node     string   `json:"node,omitempty"`
	Edge     string   `json:"edge,omitempty"`
	Message  string   `json:"message"`
	Hints    []string `json:"hints,omitempty"`
	Severity string   `json:"severity"`
}

type flowValidationResult struct {
	Valid              bool                  `json:"valid"`
	Errors             []flowValidationIssue `json:"errors"`
	Warnings           []flowValidationIssue `json:"warnings"`
	ProvidedArtifacts  []string              `json:"provided_artifacts"`
	ProducedArtifacts  []string              `json:"produced_artifacts"`
	CapabilityRegistry capabilityRegistry    `json:"capability_registry,omitempty"`
}

type flowSimulationRequest struct {
	Flow             flowManifest `json:"flow"`
	Input            string       `json:"input,omitempty"`
	DryRun           bool         `json:"dry_run"`
	FixtureArtifacts []string     `json:"fixture_artifacts,omitempty"`
}

type flowSimulationResult struct {
	Valid             bool                  `json:"valid"`
	DryRun            bool                  `json:"dry_run"`
	Input             string                `json:"input,omitempty"`
	BusinessArtifacts []string              `json:"business_artifacts,omitempty"`
	ExecutionOrder    []string              `json:"execution_order"`
	Timeline          []flowSimulationStep  `json:"timeline"`
	Artifacts         []string              `json:"artifacts"`
	Validation        flowValidationResult  `json:"validation"`
	Warnings          []flowValidationIssue `json:"warnings,omitempty"`
}

type flowSimulationStep struct {
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
	AvailableBefore  []string `json:"available_before,omitempty"`
	AvailableAfter   []string `json:"available_after,omitempty"`
}

type nodeContract struct {
	Command  string
	Inputs   []string
	Outputs  []string
	Requires []string
	Produces []string
	Policies []string
}

func (s *server) listCapabilities(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	writeOK(w, buildCapabilityRegistry(s.allManifests))
}

func (s *server) listCapabilityProviders(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	id := strings.TrimSpace(muxVar(r, "id"))
	registry := buildCapabilityRegistry(s.allManifests)
	writeOK(w, registry[id])
}

func (s *server) validateFlow(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var f flowManifest
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	prepareFlowManifestLifecycle(&f, s.allManifests)
	var businessArtifacts []string
	if strings.TrimSpace(f.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, f.BusinessID, nil); !ok {
			return
		}
		businessArtifacts = s.businessArtifacts(f.BusinessID).Artifacts
	}
	writeOK(w, validateFlowManifestWithArtifacts(f, s.allManifests, businessArtifacts))
}

func (s *server) simulateFlow(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var req flowSimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	prepareFlowManifestLifecycle(&req.Flow, s.allManifests)
	var businessArtifacts []string
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.Flow.BusinessID, nil); !ok {
			return
		}
		businessArtifacts = s.businessArtifacts(req.Flow.BusinessID).Artifacts
	}
	writeOK(w, simulateFlowManifest(req, s.allManifests, businessArtifacts))
}

func buildCapabilityRegistry(manifests map[string]*manifest.Manifest) capabilityRegistry {
	registry := capabilityRegistry{}
	for name, m := range manifests {
		if m == nil {
			continue
		}
		for _, cap := range m.Capabilities {
			info := capabilityProviderInfo{
				Capability:  cap.ID,
				Framework:   name,
				Command:     cap.Command,
				Description: cap.Description,
				Inputs:      append([]string(nil), cap.Inputs...),
				Outputs:     append([]string(nil), cap.Outputs...),
				Requires:    append([]string(nil), cap.Requires...),
				Produces:    append([]string(nil), cap.Produces...),
				Execution:   cap.Execution,
				Policies:    append([]string(nil), cap.Policies...),
				Source:      "capabilities",
			}
			addCapabilityProvider(registry, cap.ID, info)
			for _, produced := range cap.Produces {
				addCapabilityProvider(registry, produced, info)
			}
			for _, output := range cap.Outputs {
				addCapabilityProvider(registry, output, info)
			}
		}
		for _, produced := range m.CapabilitiesSemantic.Produces {
			info := capabilityProviderInfo{
				Capability: produced,
				Framework:  name,
				Produces:   []string{produced},
				Source:     "capabilities_semantic.produces",
			}
			addCapabilityProvider(registry, produced, info)
		}
	}
	for cap := range registry {
		sort.Slice(registry[cap], func(i, j int) bool {
			if registry[cap][i].Framework == registry[cap][j].Framework {
				return registry[cap][i].Command < registry[cap][j].Command
			}
			return registry[cap][i].Framework < registry[cap][j].Framework
		})
	}
	return registry
}

func addCapabilityProvider(registry capabilityRegistry, key string, info capabilityProviderInfo) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	registry[key] = append(registry[key], info)
}

func validateFlowManifest(f flowManifest, manifests map[string]*manifest.Manifest) flowValidationResult {
	return validateFlowManifestWithArtifacts(f, manifests, nil)
}

func validateFlowManifestWithArtifacts(f flowManifest, manifests map[string]*manifest.Manifest, autoArtifacts []string) flowValidationResult {
	normalizeFlowLifecycleRoles(&f)
	registry := buildCapabilityRegistry(manifests)
	result := flowValidationResult{
		Valid:              true,
		ProvidedArtifacts:  sortedKeys(systemFlowArtifacts()),
		CapabilityRegistry: registry,
	}
	available := systemFlowArtifacts()
	for _, artifact := range autoArtifacts {
		artifact = strings.TrimSpace(artifact)
		if artifact != "" {
			available[artifact] = true
		}
	}
	for _, artifact := range f.ProvidedArtifacts {
		artifact = strings.TrimSpace(artifact)
		if artifact != "" {
			available[artifact] = true
		}
	}
	nodeIDs := map[string]bool{}

	if strings.TrimSpace(f.ID) == "" {
		result.addError("flow.id_missing", "", "", "flow.id es requerido", nil)
	}
	if len(f.Nodes) == 0 {
		result.addError("flow.nodes_empty", "", "", "flow.nodes debe tener al menos un nodo", nil)
	}

	for _, edge := range f.Edges {
		if edge.From == "" || edge.To == "" {
			result.addError("flow.edge_invalid", "", edge.From+"->"+edge.To, "edge.from y edge.to son requeridos", nil)
			continue
		}
	}

	for idx, node := range f.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			result.addError("node.id_missing", "", "", fmt.Sprintf("nodes[%d].id es requerido", idx), nil)
			continue
		}
		if nodeIDs[node.ID] {
			result.addError("node.id_duplicate", node.ID, "", "id de nodo duplicado", nil)
			continue
		}
		nodeIDs[node.ID] = true
	}

	for _, edge := range f.Edges {
		if edge.From != "" && !nodeIDs[edge.From] {
			result.addError("flow.edge_unknown_from", "", edge.From+"->"+edge.To, "edge.from referencia un nodo inexistente", nil)
		}
		if edge.To != "" && !nodeIDs[edge.To] {
			result.addError("flow.edge_unknown_to", "", edge.From+"->"+edge.To, "edge.to referencia un nodo inexistente", nil)
		}
	}

	nodeOrder, orderErr := flowExecutionOrder(f)
	if orderErr != nil {
		result.addError("flow.graph_cycle", "", "", orderErr.Error(), nil)
		nodeOrder = f.Nodes
	}

	for _, node := range nodeOrder {
		m, ok := manifests[node.Framework]
		if strings.TrimSpace(node.Framework) == "" {
			result.addError("node.framework_missing", node.ID, "", "framework es requerido", nil)
			continue
		}
		if !ok || m == nil {
			result.addError("node.framework_unknown", node.ID, "", "framework no encontrado: "+node.Framework, nil)
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			result.addError("node.contract_invalid", node.ID, "", err.Error(), nil)
			continue
		}
		for _, req := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
			if req == "" || available[req] {
				continue
			}
			if canResolveArtifactStructurally(req, available) {
				available[req] = true
				continue
			}
			hints := providerHints(registry, req)
			result.addError("node.requirement_missing", node.ID, "", "artifact/capability requerido no disponible antes del nodo: "+req, hints)
		}
		for _, policy := range contract.Policies {
			if isExternalSideEffectPolicy(policy) && !hasPolicy(f.Policies, "approval_required") && !hasPolicy(contract.Policies, "approval_required") {
				result.addWarning("node.approval_recommended", node.ID, "", "capability con side effect externo debería declarar approval_required", nil)
			}
		}
		for _, out := range uniqueStrings(append(contract.Outputs, contract.Produces...)) {
			if out != "" {
				available[out] = true
			}
		}
		// In validation, credential-check nodes promote the checked
		// credential so downstream nodes are not marked as invalid.
		if promoted := inferCheckedCredential(contract); promoted != "" && !available[promoted] {
			available[promoted] = true
		}
	}

	result.ProvidedArtifacts = sortedProvidedArtifacts(available)
	result.ProducedArtifacts = sortedProducedArtifacts(available)
	result.Valid = len(result.Errors) == 0
	return result
}

func simulateFlowManifest(req flowSimulationRequest, manifests map[string]*manifest.Manifest, businessArtifacts []string) flowSimulationResult {
	autoArtifacts := uniqueStrings(append(businessArtifacts, req.FixtureArtifacts...))
	validation := validateFlowManifestWithArtifacts(req.Flow, manifests, autoArtifacts)
	available := systemFlowArtifacts()
	for _, artifact := range append(autoArtifacts, req.Flow.ProvidedArtifacts...) {
		artifact = strings.TrimSpace(artifact)
		if artifact != "" {
			available[artifact] = true
		}
	}
	nodeOrder, err := flowExecutionOrder(req.Flow)
	if err != nil {
		nodeOrder = req.Flow.Nodes
	}
	result := flowSimulationResult{
		Valid:             validation.Valid,
		DryRun:            true,
		Input:             req.Input,
		BusinessArtifacts: sortedStringSlice(businessArtifacts),
		ExecutionOrder:    make([]string, 0, len(nodeOrder)),
		Validation:        validation,
		Warnings:          validation.Warnings,
	}
	for _, node := range nodeOrder {
		result.ExecutionOrder = append(result.ExecutionOrder, node.ID)
		step := flowSimulationStep{
			Node:            node.ID,
			Framework:       node.Framework,
			Capability:      node.Capability,
			Status:          "would_run",
			AvailableBefore: sortedKeys(available),
		}
		m := manifests[node.Framework]
		if m == nil {
			step.Status = "blocked"
			result.Timeline = append(result.Timeline, step)
			continue
		}
		contract, contractErr := resolveFlowNodeContract(node, m)
		if contractErr != nil {
			step.Status = "blocked"
			result.Timeline = append(result.Timeline, step)
			continue
		}
		step.Command = contract.Command
		step.Inputs = uniqueStrings(contract.Inputs)
		step.Requires = uniqueStrings(contract.Requires)
		step.Outputs = uniqueStrings(contract.Outputs)
		step.Produces = uniqueStrings(contract.Produces)
		step.Policies = uniqueStrings(contract.Policies)
		for _, reqArtifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
			if reqArtifact != "" && !available[reqArtifact] {
				if canResolveArtifactStructurally(reqArtifact, available) {
					available[reqArtifact] = true
					continue
				}
				step.MissingArtifacts = append(step.MissingArtifacts, reqArtifact)
			}
		}
		if len(step.MissingArtifacts) > 0 {
			step.Status = "blocked"
		} else {
			for _, out := range uniqueStrings(append(contract.Outputs, contract.Produces...)) {
				if out != "" {
					available[out] = true
				}
			}
			// In simulation, credential-check nodes optimistically promote
			// the checked credential so downstream nodes are not blocked.
			if promoted := inferCheckedCredential(contract); promoted != "" && !available[promoted] {
				available[promoted] = true
			}
		}
		step.AvailableAfter = sortedKeys(available)
		result.Timeline = append(result.Timeline, step)
	}
	result.Artifacts = sortedProvidedArtifacts(available)
	return result
}

func resolveFlowNodeContract(node flowNode, m *manifest.Manifest) (nodeContract, error) {
	contract := nodeContract{
		Command:  node.Command,
		Inputs:   append([]string(nil), node.Inputs...),
		Outputs:  append([]string(nil), node.Outputs...),
		Requires: append([]string(nil), node.Requires...),
		Produces: append([]string(nil), node.Produces...),
		Policies: append([]string(nil), node.Policies...),
	}
	if node.Capability != "" {
		cap, ok := findManifestCapability(m, node.Capability)
		if !ok {
			return contract, fmt.Errorf("capability %q no existe en manifest %s", node.Capability, m.Name)
		}
		if contract.Command == "" {
			contract.Command = cap.Command
		}
		contract.Inputs = uniqueStrings(append(contract.Inputs, cap.Inputs...))
		contract.Outputs = uniqueStrings(append(contract.Outputs, cap.Outputs...))
		contract.Requires = uniqueStrings(append(contract.Requires, cap.Requires...))
		contract.Produces = uniqueStrings(append(contract.Produces, cap.Produces...))
		contract.Policies = uniqueStrings(append(contract.Policies, cap.Policies...))
	}
	if contract.Command != "" {
		if _, ok := m.Commands[contract.Command]; !ok {
			return contract, fmt.Errorf("command %q no existe en manifest %s", contract.Command, m.Name)
		}
	}
	return contract, nil
}

func findManifestCapability(m *manifest.Manifest, id string) (manifest.CapabilitySpec, bool) {
	for _, cap := range m.Capabilities {
		if cap.ID == id {
			return cap, true
		}
	}
	return manifest.CapabilitySpec{}, false
}

func (s *server) findProviderForCapability(capability string) (*manifest.Manifest, string, bool) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return nil, "", false
	}
	names := make([]string, 0, len(s.allManifests))
	for name := range s.allManifests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		m := s.allManifests[name]
		if m == nil {
			continue
		}
		if _, ok := findManifestCapability(m, capability); ok {
			return m, name, true
		}
	}
	return nil, "", false
}

func systemFlowArtifacts() map[string]bool {
	return map[string]bool{
		"business.context.v1": true,
		"business.id":         true,
		"session.context":     true,
		"session.context.v1":  true,
		"user.input.v1":       true,
		"user.question":       true,
	}
}

func canResolveArtifactStructurally(artifact string, available map[string]bool) bool {
	switch artifact {
	case "contact.destination.v1":
		return available["entity.ref.v1"] || available["entity_360.v1"] || available["message.draft.v1"]
	case "work.context.v1":
		return available["focus.next_task.v1"] || available["task.next"] || available["entity.ref.v1"]
	case "dataset.raw.v1", "external.api.dump.v1":
		return available["data.sqlite_db.v1"]
	case "credentials.smtp":
		return true
	default:
		return false
	}
}

func (r *flowValidationResult) addError(code, node, edge, message string, hints []string) {
	r.Errors = append(r.Errors, flowValidationIssue{Code: code, Node: node, Edge: edge, Message: message, Hints: hints, Severity: "error"})
}

func (r *flowValidationResult) addWarning(code, node, edge, message string, hints []string) {
	r.Warnings = append(r.Warnings, flowValidationIssue{Code: code, Node: node, Edge: edge, Message: message, Hints: hints, Severity: "warning"})
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedStringSlice(in []string) []string {
	out := uniqueStrings(in)
	sort.Strings(out)
	return out
}

func sortedProducedArtifacts(m map[string]bool) []string {
	system := systemFlowArtifacts()
	out := []string{}
	for k := range m {
		if !system[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedProvidedArtifacts(m map[string]bool) []string {
	return sortedKeys(m)
}

func providerHints(registry capabilityRegistry, cap string) []string {
	providers := registry[cap]
	hints := make([]string, 0, len(providers))
	seen := map[string]bool{}
	for _, p := range providers {
		hint := p.Framework
		if p.Command != "" {
			hint += "." + p.Command
		}
		if seen[hint] {
			continue
		}
		seen[hint] = true
		hints = append(hints, hint)
	}
	return hints
}

func isExternalSideEffectPolicy(policy string) bool {
	p := strings.ToLower(strings.TrimSpace(policy))
	return p == "external_side_effect" || p == "side_effect" || p == "external_mutation"
}

func hasPolicy(policies []string, want string) bool {
	for _, policy := range policies {
		if strings.EqualFold(strings.TrimSpace(policy), want) {
			return true
		}
	}
	return false
}

func flowExecutionOrder(f flowManifest) ([]flowNode, error) {
	if len(f.Edges) == 0 {
		return append([]flowNode(nil), f.Nodes...), nil
	}
	nodes := map[string]flowNode{}
	inDegree := map[string]int{}
	next := map[string][]string{}
	for _, node := range f.Nodes {
		if node.ID == "" {
			continue
		}
		nodes[node.ID] = node
		inDegree[node.ID] = 0
	}
	for _, edge := range f.Edges {
		if _, ok := nodes[edge.From]; !ok {
			continue
		}
		if _, ok := nodes[edge.To]; !ok {
			continue
		}
		next[edge.From] = append(next[edge.From], edge.To)
		inDegree[edge.To]++
	}
	ready := make([]string, 0, len(nodes))
	for id, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	out := make([]flowNode, 0, len(nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		out = append(out, nodes[id])
		sort.Strings(next[id])
		for _, to := range next[id] {
			inDegree[to]--
			if inDegree[to] == 0 {
				ready = append(ready, to)
				sort.Strings(ready)
			}
		}
	}
	if len(out) != len(nodes) {
		return out, fmt.Errorf("flow graph tiene ciclo o dependencias circulares")
	}
	return out, nil
}

func muxVar(r *http.Request, key string) string {
	return strings.TrimSpace(mux.Vars(r)[key])
}

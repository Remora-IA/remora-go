package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"channel/manifest"
)

type flowProvenance struct {
	Source     string `json:"source,omitempty"`
	Template   bool   `json:"template,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
}

type flowDerivation struct {
	Grounding  flowDataGrounding     `json:"grounding,omitempty"`
	Amendments []flowAmendment       `json:"amendments,omitempty"`
	Contracts  []flowDerivedContract `json:"contracts,omitempty"`
	Handoffs   []flowDerivedHandoff  `json:"handoffs,omitempty"`
	Install    flowInstallPreview    `json:"install"`
	Executable flowExecutablePlan    `json:"executable,omitempty"`
}

type flowDataGrounding struct {
	DesiredCapability string            `json:"desired_capability,omitempty"`
	BusinessArtifacts []string          `json:"business_artifacts,omitempty"`
	ArtifactSources   map[string]string `json:"artifact_sources,omitempty"`
	RequiredArtifacts []string          `json:"required_artifacts,omitempty"`
	MissingArtifacts  []string          `json:"missing_artifacts,omitempty"`
	UniversalRoles    []string          `json:"universal_roles,omitempty"`
}

type flowAmendment struct {
	Kind    string `json:"kind"`
	NodeID  string `json:"node_id,omitempty"`
	Summary string `json:"summary"`
	Reason  string `json:"reason,omitempty"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
}

type flowExecutablePlan struct {
	Nodes     []flowNode    `json:"nodes,omitempty"`
	Edges     []flowEdge    `json:"edges,omitempty"`
	Lifecycle flowLifecycle `json:"lifecycle,omitempty"`
}

type flowCompiledManifest struct {
	ID   string       `json:"id"`
	Flow flowManifest `json:"flow"`
}

type flowCompilation struct {
	Authored   flowManifest
	Derivation *flowDerivation
	Compiled   flowCompiledManifest
}

func stripFlowDerivedState(f *flowManifest) {
	if f == nil {
		return
	}
	f.Derivation = nil
}

func cloneFlowManifest(in flowManifest) flowManifest {
	out := in
	out.Intent.Constraints = append([]string(nil), in.Intent.Constraints...)
	out.Intent.Roles = append([]string(nil), in.Intent.Roles...)
	out.ProvidedArtifacts = append([]string(nil), in.ProvidedArtifacts...)
	out.Nodes = cloneFlowNodes(in.Nodes)
	out.Edges = cloneFlowEdges(in.Edges)
	out.Policies = append([]string(nil), in.Policies...)
	out.Derivation = nil
	return out
}

func cloneFlowNodes(in []flowNode) []flowNode {
	if len(in) == 0 {
		return nil
	}
	out := make([]flowNode, len(in))
	for i, node := range in {
		out[i] = node
		if len(node.Params) > 0 {
			out[i].Params = map[string]string{}
			for key, value := range node.Params {
				out[i].Params[key] = value
			}
		}
		out[i].Inputs = append([]string(nil), node.Inputs...)
		out[i].Outputs = append([]string(nil), node.Outputs...)
		out[i].Requires = append([]string(nil), node.Requires...)
		out[i].Produces = append([]string(nil), node.Produces...)
		out[i].Policies = append([]string(nil), node.Policies...)
	}
	return out
}

func cloneFlowEdges(in []flowEdge) []flowEdge {
	if len(in) == 0 {
		return nil
	}
	out := make([]flowEdge, len(in))
	copy(out, in)
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func deriveFlowManifest(flow flowManifest, manifests map[string]*manifest.Manifest, business businessArtifactsResponse) *flowDerivation {
	authorial := cloneFlowManifest(flow)
	stripFlowDerivedState(&authorial)
	executable := deriveExecutableFlow(authorial, manifests)
	return &flowDerivation{
		Grounding:  deriveFlowGrounding(authorial, manifests, business),
		Amendments: deriveFlowAmendments(authorial, executable, manifests),
		Contracts:  buildDerivedContracts(executable, manifests),
		Handoffs:   buildDerivedHandoffs(executable, manifests),
		Install:    buildFlowInstallPreview(executable, manifests),
		Executable: flowExecutablePlan{
			Nodes:     cloneFlowNodes(executable.Nodes),
			Edges:     cloneFlowEdges(executable.Edges),
			Lifecycle: executable.Lifecycle,
		},
	}
}

func deriveExecutableFlow(flow flowManifest, manifests map[string]*manifest.Manifest) flowManifest {
	executable := cloneFlowManifest(flow)
	deriveExecutableLifecycle(&executable, manifests)
	return executable
}

func compileFlowManifest(flow flowManifest, manifests map[string]*manifest.Manifest, business businessArtifactsResponse) flowCompilation {
	authored := cloneFlowManifest(flow)
	stripFlowDerivedState(&authored)
	derivation := deriveFlowManifest(authored, manifests, business)
	executable := flowManifestWithExecutablePlan(authored, derivation)
	return flowCompilation{
		Authored:   authored,
		Derivation: derivation,
		Compiled: flowCompiledManifest{
			ID:   flowCompiledManifestID(authored, executable),
			Flow: executable,
		},
	}
}

func flowCompiledManifestID(authored, executable flowManifest) string {
	raw, err := json.Marshal(struct {
		Authored   flowManifest `json:"authored"`
		Executable flowManifest `json:"executable"`
	}{
		Authored:   authored,
		Executable: executable,
	})
	if err != nil {
		raw = []byte(authored.ID + "|" + executable.ID)
	}
	sum := sha256.Sum256(raw)
	return "cmp_" + hex.EncodeToString(sum[:8])
}

func flowManifestWithExecutablePlan(flow flowManifest, derivation *flowDerivation) flowManifest {
	out := cloneFlowManifest(flow)
	if derivation == nil {
		return out
	}
	out.Nodes = cloneFlowNodes(derivation.Executable.Nodes)
	out.Edges = cloneFlowEdges(derivation.Executable.Edges)
	out.Lifecycle = derivation.Executable.Lifecycle
	out.Derivation = nil
	return out
}

func deriveFlowGrounding(flow flowManifest, manifests map[string]*manifest.Manifest, business businessArtifactsResponse) flowDataGrounding {
	required := map[string]bool{}
	missing := map[string]bool{}
	internalArtifacts := map[string]bool{}
	available := systemFlowArtifacts()
	roleSet := map[string]bool{}
	for _, role := range flow.Intent.Roles {
		role = strings.TrimSpace(role)
		if role != "" {
			roleSet[role] = true
		}
	}
	for _, artifact := range business.Artifacts {
		available[artifact] = true
	}
	for _, artifact := range flow.ProvidedArtifacts {
		available[artifact] = true
	}
	for _, node := range flow.Nodes {
		if role := inferUniversalRoleForNode(node, manifests); role != "" {
			roleSet[role] = true
		}
		m := manifests[node.Framework]
		if m == nil {
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			continue
		}
		for _, produced := range uniqueStrings(append(contract.Outputs, contract.Produces...)) {
			if produced != "" {
				internalArtifacts[produced] = true
			}
		}
	}
	for _, node := range flow.Nodes {
		m := manifests[node.Framework]
		if m == nil {
			continue
		}
		contract, err := resolveFlowNodeContract(node, m)
		if err != nil {
			continue
		}
		for _, artifact := range uniqueStrings(append(contract.Inputs, contract.Requires...)) {
			if artifact == "" || available[artifact] || internalArtifacts[artifact] || canResolveArtifactStructurally(artifact, available) {
				continue
			}
			required[artifact] = true
			missing[artifact] = true
		}
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return flowDataGrounding{
		DesiredCapability: firstNonEmptyString(strings.TrimSpace(flow.Intent.Goal), strings.TrimSpace(flow.Intent.Description)),
		BusinessArtifacts: sortedStringSlice(business.Artifacts),
		ArtifactSources:   copyStringMap(business.Sources),
		RequiredArtifacts: sortedKeys(required),
		MissingArtifacts:  sortedKeys(missing),
		UniversalRoles:    roles,
	}
}

func inferUniversalRoleForNode(node flowNode, manifests map[string]*manifest.Manifest) string {
	fields := []string{node.Framework, node.Capability, node.Command}
	m := manifests[node.Framework]
	if m != nil {
		if contract, err := resolveFlowNodeContract(node, m); err == nil {
			fields = append(fields,
				strings.Join(contract.Inputs, " "),
				strings.Join(contract.Requires, " "),
				strings.Join(contract.Outputs, " "),
				strings.Join(contract.Produces, " "),
				strings.Join(contract.Policies, " "),
			)
			if containsString(contract.Policies, "external_side_effect") {
				return "actuar"
			}
		}
	}
	key := strings.ToLower(strings.Join(fields, " "))
	switch {
	case strings.Contains(key, "draft"):
		return "redactar"
	case strings.Contains(key, "audit") || strings.Contains(key, "validate") || strings.Contains(key, "quality"):
		return "validar"
	case strings.Contains(key, "send") || strings.Contains(key, "import") || strings.Contains(key, "provision") || strings.Contains(key, "apply"):
		return "actuar"
	case strings.Contains(key, "ledger") || strings.Contains(key, "state") || strings.Contains(key, "record"):
		return "registrar"
	// foco → priorizar (antes de analizar, para que no colisione con radar/sabio)
	case strings.Contains(key, "foco") || (strings.Contains(key, "focus") && strings.Contains(key, "next")):
		return "priorizar"
	case strings.Contains(key, "query") || strings.Contains(key, "analysis") || strings.Contains(key, "priority") || strings.Contains(key, "entity_360"):
		return "analizar"
	case strings.Contains(key, "lookup") || strings.Contains(key, "dataset") || strings.Contains(key, "read"):
		return "leer"
	default:
		return ""
	}
}

func deriveFlowAmendments(authorial, executable flowManifest, manifests map[string]*manifest.Manifest) []flowAmendment {
	authorByID := map[string]flowNode{}
	execByID := map[string]flowNode{}
	authorOrder := make([]string, 0, len(authorial.Nodes))
	execOrder := make([]string, 0, len(executable.Nodes))
	for _, node := range authorial.Nodes {
		authorByID[node.ID] = node
		authorOrder = append(authorOrder, node.ID)
	}
	for _, node := range executable.Nodes {
		execByID[node.ID] = node
		execOrder = append(execOrder, node.ID)
	}
	out := []flowAmendment{}
	for _, node := range executable.Nodes {
		if _, ok := authorByID[node.ID]; ok {
			continue
		}
		reason := "El sistema hizo explícito un paso que el runtime necesitaba para poder arrancar con un contrato visible."
		if isFocoNode(node, manifests) {
			reason = "El diseño no tenía entry explícito y se propuso uno para separar primer contacto de pipeline."
		}
		out = append(out, flowAmendment{
			Kind:    "node_inserted",
			NodeID:  node.ID,
			Summary: fmt.Sprintf("Se propuso insertar %s.%s como paso visible del plan ejecutable.", node.Framework, node.Capability),
			Reason:  reason,
			After:   fmt.Sprintf("%s.%s", node.Framework, node.Capability),
		})
	}
	for _, node := range executable.Nodes {
		original, ok := authorByID[node.ID]
		if !ok {
			continue
		}
		before := normalizeFlowNodeRole(original.Role)
		after := normalizeFlowNodeRole(node.Role)
		if before == after {
			continue
		}
		reason := "Se hizo explícito el rol operativo del nodo en el plan ejecutable."
		if after == flowRoleBootstrap {
			reason = "El nodo produce contexto o artifacts que deben existir antes del entry."
		} else if after == flowRoleEntry {
			reason = "El plan derivado necesita un punto de entrada visible y no implícito."
		}
		out = append(out, flowAmendment{
			Kind:    "role_changed",
			NodeID:  node.ID,
			Summary: fmt.Sprintf("Se marcó %s como %s en la derivación ejecutable.", node.ID, after),
			Reason:  reason,
			Before:  before,
			After:   after,
		})
	}
	if !sameFlowLifecycleBinding(authorial.Lifecycle.Entry, executable.Lifecycle.Entry) {
		reason := "La derivación hizo explícito quién abre el flow para que el lifecycle no dependa de una inferencia implícita."
		if !emptyFlowLifecycleBinding(authorial.Lifecycle.Entry) {
			reason = "La configuración autoral del entry no pudo sostenerse tal cual en el plan ejecutable y quedó corregida de forma visible."
		}
		out = append(out, flowAmendment{
			Kind:    "lifecycle_entry_changed",
			Summary: fmt.Sprintf("Se hizo explícito el entry ejecutable como %s.", flowLifecycleBindingSummary(executable.Lifecycle.Entry)),
			Reason:  reason,
			Before:  flowLifecycleBindingSummary(authorial.Lifecycle.Entry),
			After:   flowLifecycleBindingSummary(executable.Lifecycle.Entry),
		})
	}
	if !sameFlowLifecycleBinding(authorial.Lifecycle.Tutela, executable.Lifecycle.Tutela) {
		reason := "La derivación hizo explícita la tutela o conducción del caso en el plan ejecutable."
		if !emptyFlowLifecycleBinding(authorial.Lifecycle.Tutela) {
			reason = "La tutela autoral tuvo que corregirse para que la conducción del caso quedara alineada con el plan ejecutable."
		}
		out = append(out, flowAmendment{
			Kind:    "lifecycle_tutela_changed",
			Summary: fmt.Sprintf("Se hizo explícita la tutela ejecutable como %s.", flowLifecycleBindingSummary(executable.Lifecycle.Tutela)),
			Reason:  reason,
			Before:  flowLifecycleBindingSummary(authorial.Lifecycle.Tutela),
			After:   flowLifecycleBindingSummary(executable.Lifecycle.Tutela),
		})
	}
	if !sameStringSlices(authorOrder, execOrder) {
		out = append(out, flowAmendment{
			Kind:    "nodes_reordered",
			Summary: "Se reordenó el plan ejecutable para que los productores corran antes del entry y del pipeline.",
			Reason:  "La derivación hace explícita la secuencia necesaria; ya no depende de una reinterpretación silenciosa en runtime.",
			Before:  strings.Join(authorOrder, " -> "),
			After:   strings.Join(execOrder, " -> "),
		})
	}
	if !sameFlowEdges(authorial.Edges, executable.Edges) {
		out = append(out, flowAmendment{
			Kind:    "edges_rebuilt",
			Summary: "Se recalculó la secuencia lineal del plan ejecutable.",
			Reason:  "El diseño autoral y la derivación ejecutable quedan separados, con edges explícitos en el plan final.",
			Before:  flowEdgeSummary(authorial.Edges),
			After:   flowEdgeSummary(executable.Edges),
		})
	}
	return out
}

func sameStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sameFlowEdges(left, right []flowEdge) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func flowEdgeSummary(edges []flowEdge) string {
	if len(edges) == 0 {
		return ""
	}
	parts := make([]string, 0, len(edges))
	for _, edge := range edges {
		parts = append(parts, edge.From+"->"+edge.To)
	}
	return strings.Join(parts, ", ")
}

func deriveExecutableLifecycle(flow *flowManifest, manifests map[string]*manifest.Manifest) {
	if flow.Lifecycle.Entry.Framework != "" {
		return
	}
	for _, node := range flow.Nodes {
		if node.Role == flowRoleEntry {
			flow.Lifecycle.Entry = flowLifecycleEntry{
				Framework:  node.Framework,
				Capability: node.Capability,
			}
			break
		}
	}
}

func sameFlowLifecycleBinding(a, b flowLifecycleEntry) bool {
	return a.Framework == b.Framework && a.Capability == b.Capability
}

func emptyFlowLifecycleBinding(b flowLifecycleEntry) bool {
	return b.Framework == "" && b.Capability == ""
}

func flowLifecycleBindingSummary(b flowLifecycleEntry) string {
	if b.Framework == "" {
		return "(vacío)"
	}
	if b.Capability == "" {
		return b.Framework
	}
	return b.Framework + "/" + b.Capability
}

func findInstallableFlowNode(flow flowManifest, manifests map[string]*manifest.Manifest) (flowNode, bool) {
	for _, node := range flow.Nodes {
		if producesArtifact(node, manifests, "analysis.schema.v1") || nodeHasPolicy(node, manifests, "install_once") {
			return node, true
		}
	}
	return flowNode{}, false
}

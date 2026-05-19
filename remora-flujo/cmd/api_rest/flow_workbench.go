package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"channel/manifest"
)

type flowDerivedContract struct {
	NodeID         string   `json:"node_id"`
	Framework      string   `json:"framework"`
	Capability     string   `json:"capability,omitempty"`
	Role           string   `json:"role,omitempty"`
	Command        string   `json:"command,omitempty"`
	Inputs         []string `json:"inputs,omitempty"`
	Requires       []string `json:"requires,omitempty"`
	Outputs        []string `json:"outputs,omitempty"`
	Produces       []string `json:"produces,omitempty"`
	Policies       []string `json:"policies,omitempty"`
	ResolutionMode string   `json:"resolution_mode,omitempty"`
}

type flowDerivedHandoff struct {
	FromNode      string   `json:"from_node"`
	ToNode        string   `json:"to_node"`
	FromFramework string   `json:"from_framework"`
	ToFramework   string   `json:"to_framework"`
	Artifacts     []string `json:"artifacts,omitempty"`
	Ownership     string   `json:"ownership,omitempty"`
	Summary       string   `json:"summary"`
}

type flowInstallPreview struct {
	RequiresInstall bool     `json:"requires_install"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

type flowObservedHandoff struct {
	FromNode      string   `json:"from_node"`
	ToNode        string   `json:"to_node"`
	FromFramework string   `json:"from_framework"`
	ToFramework   string   `json:"to_framework"`
	Artifacts     []string `json:"artifacts,omitempty"`
	Visibility    string   `json:"visibility,omitempty"`
	Status        string   `json:"status,omitempty"`
	CycleIndex    int      `json:"cycle_index,omitempty"`
	SegmentOwner  string   `json:"segment_owner,omitempty"`
	Summary       string   `json:"summary"`
}

type flowWorkbenchCompileRequest struct {
	Flow flowManifest `json:"flow"`
}

func buildDerivedContracts(flow flowManifest, manifests map[string]*manifest.Manifest) []flowDerivedContract {
	contracts := make([]flowDerivedContract, 0, len(flow.Nodes))
	for _, node := range flow.Nodes {
		derived := flowDerivedContract{
			NodeID:     node.ID,
			Framework:  node.Framework,
			Capability: node.Capability,
			Role:       node.Role,
		}
		if m := manifests[node.Framework]; m != nil {
			if contract, err := resolveFlowNodeContract(node, m); err == nil {
				derived.Command = contract.Command
				derived.Inputs = uniqueStrings(contract.Inputs)
				derived.Requires = uniqueStrings(contract.Requires)
				derived.Outputs = uniqueStrings(contract.Outputs)
				derived.Produces = uniqueStrings(contract.Produces)
				derived.Policies = uniqueStrings(contract.Policies)
				derived.ResolutionMode = resolutionModeFromPolicies(contract.Policies)
			}
		}
		contracts = append(contracts, derived)
	}
	return contracts
}

func buildDerivedHandoffs(flow flowManifest, manifests map[string]*manifest.Manifest) []flowDerivedHandoff {
	pairs := plannedFlowPairs(flow)
	handoffs := make([]flowDerivedHandoff, 0, len(pairs))
	for _, pair := range pairs {
		artifacts := sharedStrings(contractArtifacts(pair[0], manifests), requiredArtifacts(pair[1], manifests))
		ownership := pair[1].Role
		if ownership == "" {
			ownership = flowRolePipeline
		}
		handoffs = append(handoffs, flowDerivedHandoff{
			FromNode:      pair[0].ID,
			ToNode:        pair[1].ID,
			FromFramework: pair[0].Framework,
			ToFramework:   pair[1].Framework,
			Artifacts:     artifacts,
			Ownership:     ownership,
			Summary:       fmt.Sprintf("%s.%s entrega a %s.%s.", pair[0].Framework, pair[0].Capability, pair[1].Framework, pair[1].Capability),
		})
	}
	return handoffs
}

func buildFlowInstallPreview(flow flowManifest, manifests map[string]*manifest.Manifest) flowInstallPreview {
	preview := flowInstallPreview{}
	for _, node := range flow.Nodes {
		if !producesArtifact(node, manifests, "analysis.schema.v1") && !nodeHasPolicy(node, manifests, "install_once") {
			continue
		}
		preview.RequiresInstall = true
		if node.Capability != "" {
			preview.Capabilities = append(preview.Capabilities, node.Capability)
		} else {
			preview.Capabilities = append(preview.Capabilities, node.Framework)
		}
	}
	preview.Capabilities = uniqueStrings(preview.Capabilities)
	sort.Strings(preview.Capabilities)
	return preview
}

func buildObservedFlowHandoffs(steps []flowRunStep) []flowObservedHandoff {
	out := []flowObservedHandoff{}
	for i := 1; i < len(steps); i++ {
		prev := steps[i-1]
		next := steps[i]
		out = append(out, flowObservedHandoff{
			FromNode:      prev.Node,
			ToNode:        next.Node,
			FromFramework: prev.Framework,
			ToFramework:   next.Framework,
			Artifacts:     sharedStrings(prev.ArtifactTypes, uniqueStrings(append(next.Inputs, next.Requires...))),
			Visibility:    next.Visibility,
			Status:        next.Status,
			CycleIndex:    next.CycleIndex,
			SegmentOwner:  next.SegmentOwner,
			Summary:       fmt.Sprintf("%s.%s paso a %s.%s.", prev.Framework, prev.Capability, next.Framework, next.Capability),
		})
	}
	return out
}

func plannedFlowPairs(flow flowManifest) [][2]flowNode {
	nodeByID := map[string]flowNode{}
	for _, node := range flow.Nodes {
		nodeByID[node.ID] = node
	}
	pairs := [][2]flowNode{}
	seen := map[string]bool{}
	if len(flow.Edges) > 0 {
		for _, edge := range flow.Edges {
			from, okFrom := nodeByID[edge.From]
			to, okTo := nodeByID[edge.To]
			if !okFrom || !okTo {
				continue
			}
			key := from.ID + "->" + to.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			pairs = append(pairs, [2]flowNode{from, to})
		}
		return pairs
	}
	for i := 1; i < len(flow.Nodes); i++ {
		pairs = append(pairs, [2]flowNode{flow.Nodes[i-1], flow.Nodes[i]})
	}
	return pairs
}

func contractArtifacts(node flowNode, manifests map[string]*manifest.Manifest) []string {
	artifacts := uniqueStrings(append(append([]string(nil), node.Outputs...), node.Produces...))
	if m := manifests[node.Framework]; m != nil {
		if contract, err := resolveFlowNodeContract(node, m); err == nil {
			artifacts = uniqueStrings(append(append(artifacts, contract.Outputs...), contract.Produces...))
		}
	}
	return artifacts
}

func requiredArtifacts(node flowNode, manifests map[string]*manifest.Manifest) []string {
	artifacts := uniqueStrings(append(append([]string(nil), node.Inputs...), node.Requires...))
	if m := manifests[node.Framework]; m != nil {
		if contract, err := resolveFlowNodeContract(node, m); err == nil {
			artifacts = uniqueStrings(append(append(artifacts, contract.Inputs...), contract.Requires...))
		}
	}
	return artifacts
}

func sharedStrings(left, right []string) []string {
	seen := map[string]bool{}
	for _, item := range left {
		item = strings.TrimSpace(item)
		if item != "" {
			seen[item] = true
		}
	}
	out := []string{}
	for _, item := range right {
		item = strings.TrimSpace(item)
		if item == "" || !seen[item] {
			continue
		}
		out = append(out, item)
	}
	out = uniqueStrings(out)
	sort.Strings(out)
	return out
}

func (s *server) compileFlowWorkbench(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	var req flowWorkbenchCompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	var business businessArtifactsResponse
	if strings.TrimSpace(req.Flow.BusinessID) != "" {
		if _, _, ok := s.requireMembershipContext(w, r, req.Flow.BusinessID, nil); !ok {
			return
		}
		business = s.businessArtifacts(req.Flow.BusinessID)
	}
	compilation := s.compileAndPersistFlowManifest(req.Flow, s.allManifests, business)
	writeOK(w, flowSuggestionProposal{
		Manifest:   compilation.Authored,
		Derivation: compilation.Derivation,
		Compiled:   compilation.Compiled,
	})
}

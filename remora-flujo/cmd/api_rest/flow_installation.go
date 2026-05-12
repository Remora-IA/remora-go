package main

import (
	"strings"
	"time"

	"encoding/json"
)

func (s *server) shouldSkipInstalledAnalysis(req flowRunRequest, node flowNode) bool {
	m := s.allManifests[node.Framework]
	if m == nil {
		return false
	}
	contract, err := resolveFlowNodeContract(node, m)
	if err != nil || !containsString(contract.Policies, "install_once") {
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
	if path := s.latestFlowArtifactPath(businessID, "analysis.plan.v1"); path != "" {
		return path
	}
	if path := s.legacyAnalysisPlanPath(businessID); nonEmptyFileExists(path) {
		return path
	}
	return ""
}

func (s *server) recordFlowInstallation(runID, nodeID, businessID string, available map[string]bool, artifacts map[string]flowRunArtifact) string {
	analysisPlan := ""
	if art := artifacts["analysis.plan.v1"]; art.Path != "" {
		analysisPlan = art.Path
	} else {
		analysisPlan = s.radarAnalysisPlanPath(businessID)
	}
	payload := map[string]interface{}{
		"artifact_type": "flow.installation.v1",
		"status":        "installed",
		"business_id":   businessID,
		"analysis_plan": analysisPlan,
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

func shouldEmitHumanAcceptance(contract nodeContract) bool {
	return containsString(contract.Policies, "human_acceptance_before_continue")
}

func shouldEmitHumanAcceptanceFromStep(step flowRunStep) bool {
	return containsString(step.Policies, "human_acceptance_before_continue")
}

func flowAnalysisAccepted(req flowRunRequest) bool {
	if req.InitialArtifacts == nil {
		return false
	}
	for _, artifact := range []string{"analysis.accepted.v1", "human.acceptance.v1", "flow.bootstrap.accepted.v1", "flow.reconfigure.v1"} {
		if _, ok := req.InitialArtifacts[artifact]; ok {
			return true
		}
	}
	return false
}

func shouldPauseForAnalysisAcceptance(req flowRunRequest, contract nodeContract, artifactTypes []string) bool {
	return shouldEmitHumanAcceptance(contract) && containsString(artifactTypes, "analysis.schema.v1") && !req.SimulateHuman && !flowAnalysisAccepted(req)
}

func inputRequestForAnalysisAcceptance(node flowNode, artifacts map[string]flowRunArtifact) flowRequiredInput {
	message := "Radar propuso una configuración de análisis. Aceptala o pedí ajustar los pesos antes de priorizar deudores."
	if summary := extractAnalysisProposalSummary(artifacts); summary != "" {
		message = summary
	}
	return flowRequiredInput{
		Artifact:   "analysis.accepted.v1",
		Kind:       "analysis_acceptance",
		Framework:  node.Framework,
		Capability: node.Capability,
		Title:      "Aceptar configuración de análisis",
		Message:    message,
		Suggestions: []string{
			"aceptar configuración",
			"ajustar pesos",
			"reconfigurar análisis",
		},
	}
}

func extractAnalysisProposalSummary(artifacts map[string]flowRunArtifact) string {
	for _, typ := range []string{"analysis.proposal.v1", "analysis.schema.v1", "analysis.plan.v1"} {
		payload, _ := artifacts[typ].Payload.(map[string]interface{})
		if payload == nil {
			continue
		}
		if text := jsonFirstString(payload, "text", "summary", "description", "configuration_reason"); text != "" {
			return text
		}
	}
	return ""
}

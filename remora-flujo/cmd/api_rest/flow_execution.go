package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"channel/adapter"
	"channel/manifest"
	"path/filepath"
)

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
	if containsString(contract.Policies, "flow_state_scoped") && req.Flow.BusinessID != "" {
		convID = focoFlowStateConvID(req.Flow.BusinessID, req.Flow.ID)
	}
	if req.Flow.BusinessID != "" && flowNodeUsesBusinessVault(contract) {
		convID = businessVaultConvID(req.Flow.BusinessID)
	}
	setParamIfDeclared(cmd, params, "conv_id", convID)
	setParamIfDeclared(cmd, params, "conversation_id", convID)
	if req.Flow.BusinessID != "" {
		setParamIfDeclared(cmd, params, "business_id", req.Flow.BusinessID)
		setParamIfDeclared(cmd, params, "profile", req.Flow.BusinessID)
	}
	setParamIfDeclared(cmd, params, "dry_run", fmt.Sprintf("%t", req.DryRun))
	if commandHasParam(cmd, "db") && containsString(contract.Policies, "business_sqlite_param") {
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
	if containsString(contract.Policies, "long_running") {
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

func flowNodeUsesBusinessVault(contract nodeContract) bool {
	return containsString(contract.Policies, "vault_scoped")
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

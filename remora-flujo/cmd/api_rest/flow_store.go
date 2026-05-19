package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/manifest"
	_ "modernc.org/sqlite"
)

type storedFlow struct {
	ID           string `json:"id"`
	BusinessID   string `json:"business_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ManifestJSON string `json:"manifest_json"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type flowWithManifest struct {
	storedFlow
	Manifest    *flowManifest            `json:"manifest"`
	CompiledID  string                   `json:"compiled_id,omitempty"`
	Operational *flowOperationalSnapshot `json:"operational,omitempty"`
	Installed   *flowInstalledSnapshot   `json:"installed,omitempty"`
}

type storedFlowTemplate struct {
	ID           string `json:"id"`
	BusinessID   string `json:"business_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ManifestJSON string `json:"manifest_json"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type flowTemplateWithManifest struct {
	storedFlowTemplate
	Manifest *flowManifest `json:"manifest"`
}

type flowOperationalSnapshot struct {
	Date             string `json:"date,omitempty"`
	TotalTasks       int    `json:"total_tasks"`
	PendingTasks     int    `json:"pending_tasks"`
	CompletedTasks   int    `json:"completed_tasks"`
	CurrentTaskID    string `json:"current_task_id,omitempty"`
	CurrentTaskTitle string `json:"current_task_title,omitempty"`
	StatePath        string `json:"-"`
}

type flowInstalledSnapshot struct {
	Installed       bool                   `json:"installed"`
	AnalysisPlan    string                 `json:"analysis_plan,omitempty"`
	AnalysisSchema  string                 `json:"analysis_schema,omitempty"`
	SchemaID        string                 `json:"schema_id,omitempty"`
	UpdatedAt       string                 `json:"updated_at,omitempty"`
	Weights         map[string]interface{} `json:"weights,omitempty"`
	ReconfigureHint []string               `json:"reconfigure_hint,omitempty"`
}

type flowInstallationResult struct {
	FlowID       string                 `json:"flow_id"`
	BusinessID   string                 `json:"business_id"`
	Status       string                 `json:"status"`
	CompiledID   string                 `json:"compiled_id,omitempty"`
	Already      bool                   `json:"already_installed,omitempty"`
	ArtifactType string                 `json:"artifact_type,omitempty"`
	Artifacts    []string               `json:"artifacts,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Payload      map[string]interface{} `json:"payload,omitempty"`
}

type flowInstallOptions struct {
	CompiledID  string `json:"compiled_id,omitempty"`
	Reconfigure bool   `json:"reconfigure"`
}

type flowStore struct {
	db *sql.DB
}

func openFlowStore(dbPath string) (*flowStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	fs := &flowStore{db: db}
	if err := fs.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return fs, nil
}

func (fs *flowStore) migrate() error {
	if _, err := fs.db.Exec(`CREATE TABLE IF NOT EXISTS flows (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		manifest_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'draft',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	if _, err := fs.db.Exec(`CREATE TABLE IF NOT EXISTS flow_templates (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		manifest_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'available',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_flows_business_updated ON flows (business_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_templates_business_updated ON flow_templates (business_id, updated_at DESC)`,
		`CREATE TABLE IF NOT EXISTS flow_runs (
			run_id TEXT PRIMARY KEY,
			flow_id TEXT NOT NULL,
			business_id TEXT NOT NULL,
			status TEXT NOT NULL,
			dry_run INTEGER NOT NULL DEFAULT 0,
			approved INTEGER NOT NULL DEFAULT 0,
			test_mode INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_runs_business_created ON flow_runs (business_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_runs_flow_created ON flow_runs (flow_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS flow_artifacts (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			flow_id TEXT NOT NULL,
			business_id TEXT NOT NULL,
			type TEXT NOT NULL,
			node_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_artifacts_lookup ON flow_artifacts (business_id, type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_artifacts_run ON flow_artifacts (run_id)`,
		`CREATE TABLE IF NOT EXISTS flow_installations (
			flow_id TEXT PRIMARY KEY,
			business_id TEXT NOT NULL,
			installed INTEGER NOT NULL DEFAULT 0,
			analysis_plan_path TEXT NOT NULL DEFAULT '',
			analysis_schema_path TEXT NOT NULL DEFAULT '',
			schema_id TEXT NOT NULL DEFAULT '',
			weights_json TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_installations_business ON flow_installations (business_id)`,
	}
	for _, stmt := range stmts {
		if _, err := fs.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (fs *flowStore) createFlow(name, description, businessID string, manifest *flowManifest) (*storedFlow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name es requerido")
	}
	businessID = strings.TrimSpace(businessID)
	if businessID == "" {
		return nil, fmt.Errorf("business_id es requerido")
	}

	manifest.ID = "flow_" + flowSafeIDStr(name)
	manifest.BusinessID = businessID
	stripFlowDerivedState(manifest)

	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("no se pudo serializar manifest: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("flw_%d", time.Now().UnixNano())

	_, err = fs.db.Exec(
		`INSERT INTO flows (id, business_id, name, description, manifest_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'draft', ?, ?)`,
		id, businessID, name, description, string(manifestRaw), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear flow: %w", err)
	}

	return &storedFlow{
		ID:           id,
		BusinessID:   businessID,
		Name:         name,
		Description:  description,
		ManifestJSON: string(manifestRaw),
		Status:       "draft",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (fs *flowStore) listFlowsByBusiness(businessID string) ([]flowWithManifest, error) {
	rows, err := fs.db.Query(
		`SELECT id, business_id, name, description, manifest_json, status, created_at, updated_at
		 FROM flows WHERE business_id = ? ORDER BY updated_at DESC`, businessID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []flowWithManifest
	for rows.Next() {
		var f storedFlow
		if err := rows.Scan(&f.ID, &f.BusinessID, &f.Name, &f.Description, &f.ManifestJSON, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		fwm := flowWithManifest{storedFlow: f}
		var m flowManifest
		if json.Unmarshal([]byte(f.ManifestJSON), &m) == nil {
			fwm.Manifest = &m
		}
		out = append(out, fwm)
	}
	if out == nil {
		out = []flowWithManifest{}
	}
	return out, nil
}

func (fs *flowStore) getFlow(id string) (*flowWithManifest, error) {
	var f storedFlow
	err := fs.db.QueryRow(
		`SELECT id, business_id, name, description, manifest_json, status, created_at, updated_at
		 FROM flows WHERE id = ?`, id,
	).Scan(&f.ID, &f.BusinessID, &f.Name, &f.Description, &f.ManifestJSON, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	fwm := &flowWithManifest{storedFlow: f}
	var m flowManifest
	if json.Unmarshal([]byte(f.ManifestJSON), &m) == nil {
		fwm.Manifest = &m
	}
	return fwm, nil
}

func (fs *flowStore) updateFlow(id, name, description string, manifest *flowManifest) (*storedFlow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name es requerido")
	}

	stripFlowDerivedState(manifest)
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("no se pudo serializar manifest: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := fs.db.Exec(
		`UPDATE flows SET name = ?, description = ?, manifest_json = ?, updated_at = ? WHERE id = ?`,
		name, description, string(manifestRaw), now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("no se pudo actualizar flow: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("flow no encontrado: %s", id)
	}

	return &storedFlow{
		ID:           id,
		Name:         name,
		Description:  description,
		ManifestJSON: string(manifestRaw),
		UpdatedAt:    now,
	}, nil
}

func (fs *flowStore) deleteFlow(id string) error {
	_, err := fs.db.Exec(`DELETE FROM flows WHERE id = ?`, id)
	return err
}

func (fs *flowStore) updateFlowStatus(id, status string) error {
	_, err := fs.db.Exec(`UPDATE flows SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (fs *flowStore) recordRun(result flowRunResult) error {
	if result.RunID == "" || result.BusinessID == "" {
		return nil
	}
	_, err := fs.db.Exec(
		`INSERT OR REPLACE INTO flow_runs (run_id, flow_id, business_id, status, dry_run, approved, test_mode, created_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.RunID, result.FlowID, result.BusinessID, result.Status, boolInt(result.DryRun), boolInt(result.Approved), boolInt(result.TestMode), result.CreatedAt, result.FinishedAt,
	)
	return err
}

func (fs *flowStore) recordArtifact(runID, flowID, businessID, nodeID, typ, source, path, createdAt string) error {
	if runID == "" || businessID == "" || typ == "" || path == "" {
		return nil
	}
	id := strings.Join([]string{runID, safeFilePart(nodeID), safeFilePart(typ)}, "::")
	_, err := fs.db.Exec(
		`INSERT OR REPLACE INTO flow_artifacts (id, run_id, flow_id, business_id, type, node_id, source, path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, runID, flowID, businessID, typ, nodeID, source, path, createdAt,
	)
	return err
}

func (fs *flowStore) latestArtifactPath(businessID, typ string) string {
	if businessID == "" || typ == "" {
		return ""
	}
	var path string
	err := fs.db.QueryRow(
		`SELECT path FROM flow_artifacts WHERE business_id = ? AND type = ? ORDER BY created_at DESC LIMIT 1`,
		businessID, typ,
	).Scan(&path)
	if err != nil {
		return ""
	}
	return path
}

func (fs *flowStore) upsertInstallation(flowID, businessID string, snapshot *flowInstalledSnapshot) error {
	if flowID == "" || businessID == "" || snapshot == nil {
		return nil
	}
	weightsRaw := ""
	if len(snapshot.Weights) > 0 {
		if raw, err := json.Marshal(snapshot.Weights); err == nil {
			weightsRaw = string(raw)
		}
	}
	updatedAt := snapshot.UpdatedAt
	if updatedAt == "" {
		updatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := fs.db.Exec(
		`INSERT INTO flow_installations (flow_id, business_id, installed, analysis_plan_path, analysis_schema_path, schema_id, weights_json, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(flow_id) DO UPDATE SET
		   business_id = excluded.business_id,
		   installed = excluded.installed,
		   analysis_plan_path = excluded.analysis_plan_path,
		   analysis_schema_path = excluded.analysis_schema_path,
		   schema_id = excluded.schema_id,
		   weights_json = excluded.weights_json,
		   updated_at = excluded.updated_at`,
		flowID, businessID, boolInt(snapshot.Installed), snapshot.AnalysisPlan, snapshot.AnalysisSchema, snapshot.SchemaID, weightsRaw, updatedAt,
	)
	return err
}

func (fs *flowStore) installation(flowID string) *flowInstalledSnapshot {
	if flowID == "" {
		return nil
	}
	var snapshot flowInstalledSnapshot
	var weightsRaw string
	var installed int
	err := fs.db.QueryRow(
		`SELECT installed, analysis_plan_path, analysis_schema_path, schema_id, weights_json, updated_at
		 FROM flow_installations WHERE flow_id = ?`,
		flowID,
	).Scan(&installed, &snapshot.AnalysisPlan, &snapshot.AnalysisSchema, &snapshot.SchemaID, &weightsRaw, &snapshot.UpdatedAt)
	if err != nil {
		return nil
	}
	snapshot.Installed = installed != 0
	if weightsRaw != "" {
		_ = json.Unmarshal([]byte(weightsRaw), &snapshot.Weights)
	}
	return &snapshot
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func mapStringInterface(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func (fs *flowStore) updateFlowStatusByManifestID(businessID, manifestID, status string) error {
	rows, err := fs.db.Query(`SELECT id, manifest_json FROM flows WHERE business_id = ?`, businessID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		var m flowManifest
		if json.Unmarshal([]byte(raw), &m) != nil || m.ID != manifestID {
			continue
		}
		_, err := fs.db.Exec(`UPDATE flows SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339), id)
		return err
	}
	return rows.Err()
}

func (fs *flowStore) close() error {
	return fs.db.Close()
}

func flowSafeIDStr(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32 // lowercase
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		case r == ' ':
			return '_'
		default:
			return -1
		}
	}, strings.ToLower(s))
}

func (s *server) enrichFlowRuntimeStatus(flow *flowWithManifest) {
	if flow == nil || flow.Manifest == nil {
		return
	}
	if s.flows != nil {
		flow.Installed = s.flows.installation(flow.ID)
	}
	if flow.Status == "installed" || flow.Status == "active" {
		flow.Operational = s.flowOperationalSnapshot(flow.BusinessID, flow.Manifest)
		return
	}
	if flow.Installed != nil {
		flow.Installed.Installed = false
	}
	flow.Operational = s.flowOperationalSnapshot(flow.BusinessID, flow.Manifest)
}

func (s *server) enrichFlowDesignSemantics(flow *flowWithManifest, business businessArtifactsResponse) {
	if flow == nil || flow.Manifest == nil {
		return
	}
	compilation := s.compileAndPersistFlowManifest(*flow.Manifest, s.allManifests, business)
	flow.CompiledID = compilation.Compiled.ID
	flow.Manifest = &compilation.Authored
	flow.Manifest.Derivation = compilation.Derivation
}

func flowUsesInstallableAnalysis(flow flowManifest, manifests map[string]*manifest.Manifest) bool {
	for _, node := range flow.Nodes {
		if producesArtifact(node, manifests, "analysis.schema.v1") || nodeHasPolicy(node, manifests, "install_once") {
			return true
		}
	}
	return false
}

func (s *server) flowInstalledSnapshot(businessID string) *flowInstalledSnapshot {
	planPath := s.radarAnalysisPlanPath(businessID)
	snapshot := &flowInstalledSnapshot{Installed: nonEmptyFileExists(planPath), AnalysisPlan: planPath}
	if !snapshot.Installed {
		return snapshot
	}
	schemaPath := s.latestFlowArtifactPath(businessID, "analysis.schema.v1")
	if schemaPath == "" {
		schemaPath = s.legacyAnalysisSchemaPath(businessID)
	}
	if !nonEmptyFileExists(schemaPath) {
		return snapshot
	}
	snapshot.AnalysisSchema = schemaPath
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return snapshot
	}
	var schema map[string]interface{}
	if json.Unmarshal(raw, &schema) != nil {
		return snapshot
	}
	snapshot.SchemaID = jsonFirstString(schema, "schema_id", "plan_id")
	snapshot.UpdatedAt = jsonFirstString(schema, "updated_at", "generated_at")
	if weights, ok := schema["weights"].(map[string]interface{}); ok {
		snapshot.Weights = weights
	}
	if notes, ok := schema["notes"].([]interface{}); ok {
		for _, note := range notes {
			if text, _ := note.(string); text != "" {
				snapshot.ReconfigureHint = append(snapshot.ReconfigureHint, text)
			}
		}
	}
	return snapshot
}

func (s *server) installFlowAnalysis(ctx context.Context, flow *flowWithManifest, opts flowInstallOptions) (flowInstallationResult, error) {
	if flow == nil || flow.Manifest == nil {
		return flowInstallationResult{Status: "installed"}, fmt.Errorf("flow sin manifest")
	}
	result := flowInstallationResult{FlowID: flow.ID, BusinessID: flow.BusinessID, Status: "installed"}
	var compilation flowCompilation
	if strings.TrimSpace(opts.CompiledID) != "" {
		record, err := s.loadCompiledRecord(opts.CompiledID)
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(record.Authored.BusinessID) != "" && record.Authored.BusinessID != flow.BusinessID {
			return result, fmt.Errorf("compiled_id no pertenece al business del flow")
		}
		if strings.TrimSpace(record.Authored.ID) != "" && flow.Manifest != nil && strings.TrimSpace(flow.Manifest.ID) != "" && record.Authored.ID != flow.Manifest.ID {
			return result, fmt.Errorf("compiled_id no pertenece al flow solicitado")
		}
		compilation = flowCompilation{
			Authored:   cloneFlowManifest(record.Authored),
			Derivation: record.Derivation,
			Compiled:   record.Compiled,
		}
	} else {
		compilation = s.compileAndPersistFlowManifest(*flow.Manifest, s.allManifests, s.businessArtifacts(flow.BusinessID))
	}
	result.CompiledID = compilation.Compiled.ID
	if s.radarAnalysisInstalled(flow.BusinessID) && !opts.Reconfigure {
		_ = s.flows.updateFlowStatus(flow.ID, "installed")
		if s.flows != nil {
			_ = s.flows.upsertInstallation(flow.ID, flow.BusinessID, s.flowInstalledSnapshot(flow.BusinessID))
		}
		result.Already = true
		result.ArtifactType = "flow.installation.v1"
		result.Artifacts = []string{"flow.installation.v1", "analysis.plan.v1"}
		result.Summary = "El flujo ya estaba instalado; se reutiliza el plan de análisis existente."
		return result, nil
	}
	installNode, ok := findInstallableFlowNode(compilation.Compiled.Flow, s.allManifests)
	if !ok {
		return result, fmt.Errorf("el flow no declara un paso instalable de análisis")
	}
	m := s.allManifests[installNode.Framework]
	if m == nil {
		return result, fmt.Errorf("framework no encontrado para instalar: %s", installNode.Framework)
	}
	contract, err := resolveFlowNodeContract(installNode, m)
	if err != nil {
		return result, err
	}
	cmd, ok := m.Commands[contract.Command]
	if !ok {
		return result, fmt.Errorf("%s no expone %s", installNode.Framework, contract.Command)
	}
	semanticPath := s.businessSemanticPackPath(flow.BusinessID)
	if semanticPath == "" {
		return result, fmt.Errorf("falta business.semantic_pack.v1 para instalar el análisis")
	}
	params := map[string]string{"business_id": flow.BusinessID}
	setParamIfDeclared(cmd, params, "business_id", flow.BusinessID)
	setParamIfDeclared(cmd, params, "semantic_pack", semanticPath)
	if dbPath := s.businessSQLitePath(flow.BusinessID); dbPath != "" {
		setParamIfDeclared(cmd, params, "db", dbPath)
	}
	args, err := cmd.ResolveArgs(params, frameworkIOPaths(s.rootDir, m.Inputs), frameworkIOPaths(s.rootDir, m.Outputs))
	if err != nil {
		return result, err
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	execCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	resp, err := s.scoped("install_"+flow.ID).ExecuteCommand(execCtx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		if isChannelUnavailableError(err) {
			return result, fmt.Errorf("%s", channelUnavailableMessage(s.channel.BaseURL))
		}
		return result, err
	}
	if !resp.Success || resp.ExitCode != 0 {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = strings.TrimSpace(resp.Stderr)
		}
		if msg == "" {
			msg = fmt.Sprintf("%s %s terminó con exit code %d", installNode.Framework, contract.Command, resp.ExitCode)
		}
		return result, fmt.Errorf("%s", msg)
	}
	payload := parseArtifactPayload(resp.Stdout)
	if typ, _ := payload["artifact_type"].(string); typ != "analysis.schema.v1" {
		return result, fmt.Errorf("%s no devolvió analysis.schema.v1", installNode.Framework)
	}
	runID := "flow_install_" + safeFilePart(flow.ID)
	schemaArtifactPath := s.persistFlowArtifact(runID, "install", "analysis.schema.v1", payload)
	if s.flows != nil {
		_ = s.flows.recordArtifact(runID, flow.ID, flow.BusinessID, "install", "analysis.schema.v1", "framework_stdout", schemaArtifactPath, time.Now().UTC().Format(time.RFC3339Nano))
	}
	planPayload := map[string]interface{}{
		"artifact_type": "analysis.plan.v1",
		"business_id":   flow.BusinessID,
		"schema":        payload,
	}
	if planPath := jsonFirstString(payload, "plan_path"); planPath != "" {
		if raw, err := os.ReadFile(filepath.Join(s.rootDir, m.Cwd, planPath)); err == nil {
			var decoded map[string]interface{}
			if json.Unmarshal(raw, &decoded) == nil {
				planPayload = decoded
			}
		}
	}
	planArtifactPath := s.persistFlowArtifact(runID, "install", "analysis.plan.v1", planPayload)
	if s.flows != nil {
		_ = s.flows.recordArtifact(runID, flow.ID, flow.BusinessID, "install", "analysis.plan.v1", "flow_engine", planArtifactPath, time.Now().UTC().Format(time.RFC3339Nano))
		_ = s.flows.recordRun(flowRunResult{
			RunID:      runID,
			FlowID:     flow.ID,
			BusinessID: flow.BusinessID,
			Status:     "installed",
			CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
			FinishedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
		_ = s.flows.upsertInstallation(flow.ID, flow.BusinessID, &flowInstalledSnapshot{
			Installed:      true,
			AnalysisPlan:   planArtifactPath,
			AnalysisSchema: schemaArtifactPath,
			SchemaID:       jsonFirstString(payload, "schema_id", "plan_id"),
			UpdatedAt:      jsonFirstString(payload, "updated_at", "generated_at"),
			Weights:        mapStringInterface(payload["weights"]),
		})
	}
	_ = s.flows.updateFlowStatus(flow.ID, "installed")
	result.ArtifactType = "flow.installation.v1"
	result.Artifacts = []string{"flow.installation.v1", "analysis.schema.v1", "analysis.plan.v1"}
	result.Summary = "Configuración de análisis aceptada e instalada para este flujo."
	result.Payload = payload
	return result, nil
}

func (s *server) flowOperationalSnapshot(businessID string, flow *flowManifest) *flowOperationalSnapshot {
	path := s.flowStatePath(businessID, flow)
	raw, err := os.ReadFile(path)
	if err != nil {
		return &flowOperationalSnapshot{Date: time.Now().Format("2006-01-02"), StatePath: path}
	}
	var state struct {
		Date  string `json:"date"`
		Tasks []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return &flowOperationalSnapshot{Date: time.Now().Format("2006-01-02"), StatePath: path}
	}
	snapshot := &flowOperationalSnapshot{Date: state.Date, TotalTasks: len(state.Tasks), StatePath: path}
	if snapshot.Date == "" {
		snapshot.Date = time.Now().Format("2006-01-02")
	}
	for _, task := range state.Tasks {
		if task.Status == "done" {
			snapshot.CompletedTasks++
			continue
		}
		snapshot.PendingTasks++
		if snapshot.CurrentTaskID == "" {
			snapshot.CurrentTaskID = task.ID
			snapshot.CurrentTaskTitle = task.Title
		}
	}
	return snapshot
}

func (s *server) flowStatePath(businessID string, flow *flowManifest) string {
	if flow == nil {
		return ""
	}
	providerName := ""
	for _, node := range flow.Nodes {
		if isFocoNode(node, s.allManifests) {
			providerName = node.Framework
			break
		}
	}
	if providerName == "" {
		_, providerName, _ = s.findProviderForCapability("focus.complete_cycle")
	}
	m := s.allManifests[providerName]
	if m == nil || strings.TrimSpace(m.State.Dir) == "" {
		return ""
	}
	fileName := "state.json"
	for _, candidate := range m.State.Files {
		candidate = strings.TrimSpace(candidate)
		if candidate == fileName {
			fileName = candidate
			break
		}
	}
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + providerName
	}
	return filepath.Join(s.rootDir, cwdRel, m.State.Dir, "sessions", focoFlowStateConvID(businessID, flow.ID), fileName)
}

// --- HTTP handlers ---

func (s *server) handleListFlows(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	flows, err := s.flows.listFlowsByBusiness(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "no se pudieron listar flujos: "+err.Error())
		return
	}
	business := s.businessArtifacts(businessID)
	for i := range flows {
		s.enrichFlowRuntimeStatus(&flows[i])
		s.enrichFlowDesignSemantics(&flows[i], business)
	}
	writeOK(w, flows)
}

func (s *server) handleCreateFlow(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	var req struct {
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Manifest    *flowManifest `json:"manifest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if req.Manifest == nil {
		req.Manifest = &flowManifest{Nodes: []flowNode{}}
	}
	created, err := s.flows.createFlow(req.Name, req.Description, businessID, materializeTemplateInstantiation(req.Manifest))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	flow, err := s.flows.getFlow(created.ID)
	if err != nil {
		writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: created})
		return
	}
	business := s.businessArtifacts(businessID)
	s.enrichFlowRuntimeStatus(flow)
	s.enrichFlowDesignSemantics(flow, business)
	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: flow})
}

func (s *server) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	id := muxVar(r, "id")
	flow, err := s.flows.getFlow(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "flow no encontrado: "+id)
		return
	}
	if _, _, ok := s.requireMembershipContext(w, r, flow.BusinessID, nil); !ok {
		return
	}
	s.enrichFlowRuntimeStatus(flow)
	s.enrichFlowDesignSemantics(flow, s.businessArtifacts(flow.BusinessID))
	writeOK(w, flow)
}

func (s *server) handleInstallFlow(w http.ResponseWriter, r *http.Request) {
	id := muxVar(r, "id")
	flow, err := s.flows.getFlow(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "flow no encontrado: "+id)
		return
	}
	if _, _, ok := s.requireMembershipContext(w, r, flow.BusinessID, nil); !ok {
		return
	}
	var opts flowInstallOptions
	_ = json.NewDecoder(r.Body).Decode(&opts)
	result, err := s.installFlowAnalysis(r.Context(), flow, opts)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, result)
}

func (s *server) handleUpdateFlow(w http.ResponseWriter, r *http.Request) {
	id := muxVar(r, "id")
	existing, err := s.flows.getFlow(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "flow no encontrado: "+id)
		return
	}
	if _, _, ok := s.requireMembershipContext(w, r, existing.BusinessID, nil); !ok {
		return
	}
	var req struct {
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Manifest    *flowManifest `json:"manifest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if req.Manifest == nil {
		req.Manifest = existing.Manifest
	}
	updated, err := s.flows.updateFlow(id, req.Name, req.Description, req.Manifest)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	flow, err := s.flows.getFlow(updated.ID)
	if err != nil {
		writeOK(w, updated)
		return
	}
	s.enrichFlowRuntimeStatus(flow)
	s.enrichFlowDesignSemantics(flow, s.businessArtifacts(flow.BusinessID))
	writeOK(w, flow)
}

func (s *server) handleDeleteFlow(w http.ResponseWriter, r *http.Request) {
	id := muxVar(r, "id")
	existing, err := s.flows.getFlow(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "flow no encontrado: "+id)
		return
	}
	if _, _, ok := s.requireMembershipContext(w, r, existing.BusinessID, nil); !ok {
		return
	}
	if err := s.flows.deleteFlow(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "no se pudo eliminar flow: "+err.Error())
		return
	}
	writeOK(w, map[string]string{"deleted": id})
}

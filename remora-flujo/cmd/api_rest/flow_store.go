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
	Operational *flowOperationalSnapshot `json:"operational,omitempty"`
	Installed   *flowInstalledSnapshot   `json:"installed,omitempty"`
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
	Already      bool                   `json:"already_installed,omitempty"`
	ArtifactType string                 `json:"artifact_type,omitempty"`
	Artifacts    []string               `json:"artifacts,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Payload      map[string]interface{} `json:"payload,omitempty"`
}

type flowInstallOptions struct {
	Reconfigure bool `json:"reconfigure"`
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
	_, err := fs.db.Exec(`CREATE TABLE IF NOT EXISTS flows (
		id TEXT PRIMARY KEY,
		business_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		manifest_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'draft',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	return err
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
	prepareFlowManifestLifecycle(manifest)

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

	prepareFlowManifestLifecycle(manifest)
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
	flow.Installed = s.flowInstalledSnapshot(flow.BusinessID)
	if flow.Status == "installed" || flow.Status == "active" {
		flow.Operational = s.flowOperationalSnapshot(flow.BusinessID, flow.Manifest.ID)
		return
	}
	if flow.Installed != nil {
		flow.Installed.Installed = false
	}
	flow.Operational = s.flowOperationalSnapshot(flow.BusinessID, flow.Manifest.ID)
}

func flowUsesRadarAnalysis(flow flowManifest) bool {
	for _, node := range flow.Nodes {
		if node.Framework == "radar" && node.Capability == "analysis.configure" {
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
	schemaPath := filepath.Join(filepath.Dir(planPath), "collection_analysis_schema.json")
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
	result := flowInstallationResult{FlowID: flow.ID, BusinessID: flow.BusinessID, Status: "installed"}
	if flow == nil || flow.Manifest == nil {
		return result, fmt.Errorf("flow sin manifest")
	}
	if s.radarAnalysisInstalled(flow.BusinessID) && !opts.Reconfigure {
		_ = s.flows.updateFlowStatus(flow.ID, "installed")
		result.Already = true
		result.ArtifactType = "flow.installation.v1"
		result.Artifacts = []string{"flow.installation.v1", "analysis.plan.v1"}
		result.Summary = "El flujo ya estaba instalado; se reutiliza el plan de análisis existente."
		return result, nil
	}
	m := s.allManifests["radar"]
	if m == nil {
		return result, fmt.Errorf("framework radar no encontrado")
	}
	cmd, ok := m.Commands["configure-analysis"]
	if !ok {
		return result, fmt.Errorf("radar.configure-analysis no está disponible")
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
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-radar"
	}
	execCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	resp, err := s.scoped("install_"+flow.ID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, filepath.Join(s.rootDir, cwdRel))
	if err != nil {
		return result, err
	}
	if !resp.Success || resp.ExitCode != 0 {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = strings.TrimSpace(resp.Stderr)
		}
		if msg == "" {
			msg = fmt.Sprintf("radar configure-analysis terminó con exit code %d", resp.ExitCode)
		}
		return result, fmt.Errorf("%s", msg)
	}
	payload := parseArtifactPayload(resp.Stdout)
	if typ, _ := payload["artifact_type"].(string); typ != "analysis.schema.v1" {
		return result, fmt.Errorf("radar no devolvió analysis.schema.v1")
	}
	_ = s.flows.updateFlowStatus(flow.ID, "installed")
	result.ArtifactType = "flow.installation.v1"
	result.Artifacts = []string{"flow.installation.v1", "analysis.schema.v1", "analysis.plan.v1"}
	result.Summary = "Configuración de análisis aceptada e instalada para este flujo."
	result.Payload = payload
	return result, nil
}

func (s *server) flowOperationalSnapshot(businessID, flowID string) *flowOperationalSnapshot {
	path := filepath.Join(s.rootDir, "framework-foco", "temp", "foco", "sessions", focoFlowStateConvID(businessID, flowID), "state.json")
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
	for i := range flows {
		s.enrichFlowRuntimeStatus(&flows[i])
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
	flow, err := s.flows.createFlow(req.Name, req.Description, businessID, req.Manifest)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
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
	flow, err := s.flows.updateFlow(id, req.Name, req.Description, req.Manifest)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
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

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/manifest"
)

type flowCompiledRecord struct {
	Authored   flowManifest         `json:"authored"`
	Derivation *flowDerivation      `json:"derivation,omitempty"`
	Compiled   flowCompiledManifest `json:"compiled"`
	CreatedAt  string               `json:"created_at,omitempty"`
}

func flowCompiledRecordFromCompilation(compilation flowCompilation) flowCompiledRecord {
	return flowCompiledRecord{
		Authored:   compilation.Authored,
		Derivation: compilation.Derivation,
		Compiled:   compilation.Compiled,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func (s *server) flowCompiledPath(compiledID string) string {
	if s == nil {
		return ""
	}
	compiledID = strings.TrimSpace(compiledID)
	if compiledID == "" {
		return ""
	}
	return filepath.Join(s.rootDir, "temp", "flow_compilations", safeFilePart(compiledID)+".json")
}

func (s *server) persistCompiledRecord(compilation flowCompilation) error {
	if s == nil {
		return nil
	}
	path := s.flowCompiledPath(compilation.Compiled.ID)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(flowCompiledRecordFromCompilation(compilation), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func (s *server) loadCompiledRecord(compiledID string) (*flowCompiledRecord, error) {
	path := s.flowCompiledPath(compiledID)
	if path == "" {
		return nil, fmt.Errorf("compiled_id es requerido")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("compiled plan no encontrado: %s", compiledID)
	}
	var record flowCompiledRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, fmt.Errorf("compiled plan inválido: %w", err)
	}
	return &record, nil
}

func (s *server) compileAndPersistFlowManifest(flow flowManifest, manifests map[string]*manifest.Manifest, business businessArtifactsResponse) flowCompilation {
	compilation := compileFlowManifest(flow, manifests, business)
	_ = s.persistCompiledRecord(compilation)
	return compilation
}

func (s *server) loadFlowRun(runID string) (*flowRunResult, error) {
	if s == nil || strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("run_id es requerido")
	}
	path := filepath.Join(s.rootDir, "temp", "flow_runs", safeFilePart(runID), "run.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("run no encontrado: %s", runID)
	}
	var result flowRunResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("run inválido: %w", err)
	}
	return &result, nil
}

func (s *server) handleGetCompiledFlow(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	record, err := s.loadCompiledRecord(muxVar(r, "compiled_id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if _, _, ok := s.requireMembershipContext(w, r, record.Authored.BusinessID, nil); !ok {
		return
	}
	writeOK(w, record)
}

func (s *server) handleGetFlowRun(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	result, err := s.loadFlowRun(muxVar(r, "id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if _, _, ok := s.requireMembershipContext(w, r, result.BusinessID, nil); !ok {
		return
	}
	writeOK(w, result)
}

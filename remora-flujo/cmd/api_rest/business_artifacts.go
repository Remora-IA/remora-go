package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type businessArtifactsResponse struct {
	BusinessID string            `json:"business_id"`
	Artifacts  []string          `json:"artifacts"`
	Sources    map[string]string `json:"sources"`
}

func (s *server) handleBusinessArtifacts(w http.ResponseWriter, r *http.Request) {
	businessID := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	writeOK(w, s.businessArtifacts(businessID))
}

func (s *server) businessArtifacts(businessID string) businessArtifactsResponse {
	available := map[string]bool{}
	sources := map[string]string{}
	add := func(artifact, source string) {
		if artifact == "" {
			return
		}
		available[artifact] = true
		if source != "" {
			sources[artifact] = source
		}
	}

	add("business.context.v1", "api_rest.membership_context")
	add("business.id", "api_rest.business_id")
	add("session.context", "api_rest.conversation_runtime_context")
	add("session.context.v1", "api_rest.conversation_runtime_context")
	add("user.input.v1", "api_rest.conversation")
	add("user.question", "api_rest.conversation")

	if dbPath := s.businessSQLitePath(businessID); dbPath != "" {
		add("data.sqlite_db.v1", dbPath)
	}
	if packPath := s.businessSemanticPackPath(businessID); packPath != "" {
		add("business.semantic_pack.v1", packPath)
	}
	if auditorDataset := filepath.Join(s.rootDir, "framework-auditor", "data", "dataset.working.json"); nonEmptyFileExists(auditorDataset) {
		add("external.api.dump.v1", auditorDataset)
	}
	if s.businessHasTasksLedger(businessID) {
		add("task.ledger.v1", filepath.Join(s.rootDir, "profiles", businessID, "tasks.db"))
	}

	// Consultar vault del negocio para credenciales SMTP
	if m := s.allManifests["hosting"]; m != nil {
		if cmd, ok := m.Commands["has-smtp"]; ok {
			convID := businessVaultConvID(businessID)
			params := map[string]string{"conv_id": convID}
			args, err := cmd.ResolveArgs(params, nil, nil)
			if err == nil {
				fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
				fullArgs = append(fullArgs, args...)
				cwdRel := m.Cwd
				if cwdRel == "" {
					cwdRel = "framework-hosting"
				}
				cwd := filepath.Join(s.rootDir, cwdRel)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				resp, err := s.scoped(convID).ExecuteCommand(ctx, m.Binary.Command, fullArgs, cwd)
				cancel()
				if err == nil && resp.ExitCode == 0 {
					var result map[string]interface{}
					if uerr := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &result); uerr == nil {
						if avail, _ := result["available"].(bool); avail {
							add("credentials.smtp", "vault.hosting")
						}
					}
				}
			}
		}
	}

	return businessArtifactsResponse{
		BusinessID: businessID,
		Artifacts:  sortedKeys(available),
		Sources:    sources,
	}
}

func (s *server) businessSQLitePath(businessID string) string {
	if businessID == "" {
		return ""
	}
	path := businessDataDBPath(s.rootDir, businessID)
	if nonEmptyFileExists(path) {
		return path
	}
	packPath := s.businessSemanticPackPath(businessID)
	if packPath == "" {
		return ""
	}
	if dataPath := dataSourcePathFromSemanticPack(s.rootDir, packPath); dataPath != "" {
		return dataPath
	}
	return ""
}

func (s *server) businessSemanticPackPath(businessID string) string {
	if businessID == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(s.rootDir, "framework-sabio", "businesses", businessID, "sabio.business.json"),
		filepath.Join(s.rootDir, "profiles", businessID, "sabio.business.json"),
	}
	for _, path := range candidates {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func (s *server) businessHasTasksLedger(businessID string) bool {
	if businessID == "" {
		return false
	}
	return fileExists(filepath.Join(s.rootDir, "profiles", businessID, "tasks.db"))
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func nonEmptyFileExists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Size() > 0
}

func dataSourcePathFromSemanticPack(rootDir, packPath string) string {
	raw, err := os.ReadFile(packPath)
	if err != nil {
		return ""
	}
	var doc struct {
		DataSource struct {
			DefaultPath string `json:"default_path"`
		} `json:"data_source"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	declared := doc.DataSource.DefaultPath
	if declared == "" {
		return ""
	}
	for _, candidate := range resolveBusinessDataSourceCandidates(rootDir, packPath, declared) {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func resolveBusinessDataSourceCandidates(rootDir, packPath, declared string) []string {
	if filepath.IsAbs(declared) {
		return []string{filepath.Clean(declared)}
	}
	return []string{
		filepath.Clean(filepath.Join(filepath.Dir(packPath), declared)),
		filepath.Clean(filepath.Join(rootDir, declared)),
		filepath.Clean(filepath.Join(rootDir, "framework-sabio", declared)),
	}
}

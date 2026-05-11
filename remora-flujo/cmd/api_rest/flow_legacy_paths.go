package main

import (
	"path/filepath"
	"strings"
)

func (s *server) legacyAnalysisPlanPath(businessID string) string {
	return s.legacyAnalysisPath(businessID, "collection_analysis_plan.json")
}

func (s *server) legacyAnalysisSchemaPath(businessID string) string {
	return s.legacyAnalysisPath(businessID, "collection_analysis_schema.json")
}

func (s *server) legacyAnalysisPath(businessID, fileName string) string {
	businessID = strings.TrimSpace(businessID)
	if businessID == "" {
		return ""
	}
	providerName := s.providerNameForCapability("analysis.configure")
	m := s.allManifests[providerName]
	cwdRel := ""
	if m != nil {
		cwdRel = m.Cwd
	}
	defaultCwd := "framework-" + providerName
	if cwdRel == "" {
		cwdRel = defaultCwd
	}
	path := filepath.Join(s.rootDir, cwdRel, "temp", providerName, safeFilePart(businessID), fileName)
	if nonEmptyFileExists(path) || cwdRel == defaultCwd {
		return path
	}
	return filepath.Join(s.rootDir, defaultCwd, "temp", providerName, safeFilePart(businessID), fileName)
}

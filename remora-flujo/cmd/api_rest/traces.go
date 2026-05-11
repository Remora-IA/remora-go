package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
)

// handleTracesLatest lee el trace paladin más reciente persistido por el
// orchestrator en temp/paladin/trace_*.json y lo devuelve tal cual.
//
// Endpoint útil para debug manual (curl) y para que el frontend en dev
// mode pinte un panel mostrando el flujo del último runLoop.
//
// Query params:
//
//	?n=N  devuelve los últimos N archivos (default 1)
func (s *server) handleTracesLatest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	traceDir := envOr("REMORA_PALADIN_DIR", "temp/paladin")
	if !filepath.IsAbs(traceDir) {
		// Relativo al cwd del proceso (normalmente /workspace/remora-flujo
		// en Cloud Run, local dir en dev). Si la lectura falla, devolvemos
		// { found: false } con un hint.
	}

	entries, err := os.ReadDir(traceDir)
	if err != nil {
		writeOK(w, map[string]interface{}{
			"found": false,
			"dir":   traceDir,
			"error": err.Error(),
			"hint":  "no hay traces todavía; dispará al menos un turn del runLoop",
		})
		return
	}

	type entry struct {
		name string
		mod  int64
	}
	files := []entry{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 10 || name[:6] != "trace_" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, entry{name: name, mod: info.ModTime().UnixNano()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	if len(files) == 0 {
		writeOK(w, map[string]interface{}{
			"found": false,
			"dir":   traceDir,
			"hint":  "directorio existe pero no hay traces (trace_*.json)",
		})
		return
	}

	path := filepath.Join(traceDir, files[0].name)
	raw, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read trace: "+err.Error())
		return
	}
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		writeErr(w, http.StatusInternalServerError, "parse trace: "+err.Error())
		return
	}
	writeOK(w, map[string]interface{}{
		"found": true,
		"file":  files[0].name,
		"trace": parsed,
	})
}

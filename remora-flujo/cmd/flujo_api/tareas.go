// tareas.go: cliente liviano para invocar el binario `frameworktareas` desde
// el orquestador y endpoints REST que el frontend consume para listar/avanzar
// el ledger del día.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
)

// resolveTareas localiza el binario frameworktareas y su cwd.
func resolveTareas() (string, string) {
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "/workspace"))
	cwd := filepath.Join(root, "framework-tareas")
	bin := envOr("REMORA_TAREAS_BIN", filepath.Join(cwd, "frameworktareas"))
	return bin, cwd
}

// runTareas ejecuta `frameworktareas <args>` y devuelve el JSON parseado.
func runTareas(args ...string) (map[string]interface{}, error) {
	bin, cwd := resolveTareas()
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	out, runErr := cmd.Output()
	var res map[string]interface{}
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("tareas respuesta inválida: %v stderr=%s out=%s", jerr, stderr, string(out))
	}
	return res, nil
}

func currentProfile() string {
	return envOr("REMORA_PROFILE", "default")
}

// emitTaskEvent es un helper para que otros handlers (ej. /send-email) emitan
// eventos al ledger sin acoplarse al binario. Falla silenciosa: el envío del
// email es la operación principal, registrar el evento es best-effort.
func emitTaskEvent(taskID, actor, kind string, data map[string]interface{}) {
	if taskID == "" {
		return
	}
	dataJSON, _ := json.Marshal(data)
	_, err := runTareas("event",
		"--profile", currentProfile(),
		"--id", taskID,
		"--actor", actor,
		"--kind", kind,
		"--data", string(dataJSON),
	)
	if err != nil {
		// log y continuar
		fmt.Printf("[tareas] emit event %s/%s err: %v\n", taskID, kind, err)
	}
}

// ─── HTTP Handlers ────────────────────────────────────────────────────────

func (s *server) handleTasksList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	args := []string{"list", "--profile", currentProfile()}
	if status != "" {
		args = append(args, "--status", status)
	}
	res, err := runTareas(args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, res)
}

func (s *server) handleTasksNext(w http.ResponseWriter, r *http.Request) {
	res, err := runTareas("next", "--profile", currentProfile())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, res)
}

type createTaskReq struct {
	EntityType string `json:"entity_type,omitempty"`
	EntityRef  string `json:"entity_ref,omitempty"`
	Action     string `json:"action"`
	Title      string `json:"title,omitempty"`
	Priority   int    `json:"priority,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

func (s *server) handleTasksCreate(w http.ResponseWriter, r *http.Request) {
	var req createTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.Action == "" {
		writeErr(w, http.StatusBadRequest, "action requerido")
		return
	}
	if req.Priority == 0 {
		req.Priority = 100
	}
	args := []string{"create", "--profile", currentProfile(),
		"--entity-type", req.EntityType,
		"--entity-ref", req.EntityRef,
		"--action", req.Action,
		"--title", req.Title,
		"--priority", fmt.Sprintf("%d", req.Priority),
	}
	res, err := runTareas(args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, res)
}

type taskEventReq struct {
	Actor string                 `json:"actor"`
	Kind  string                 `json:"kind"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

func (s *server) handleTaskEvent(w http.ResponseWriter, r *http.Request) {
	taskID := taskIDFromPath(r.URL.Path)
	if taskID == "" {
		writeErr(w, http.StatusBadRequest, "task id requerido")
		return
	}
	var req taskEventReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.Kind == "" {
		writeErr(w, http.StatusBadRequest, "kind requerido")
		return
	}
	dataJSON := []byte("{}")
	if req.Data != nil {
		dataJSON, _ = json.Marshal(req.Data)
	}
	res, err := runTareas("event",
		"--profile", currentProfile(),
		"--id", taskID,
		"--actor", req.Actor,
		"--kind", req.Kind,
		"--data", string(dataJSON),
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, res)
}

// taskIDFromPath extrae el id de /api/v1/tasks/{id}/event.
func taskIDFromPath(p string) string {
	// muxer ya valida la forma; usamos filepath.Base sobre el segmento previo
	// a "/event" o sobre el último segmento de /tasks/{id}.
	const eventSuffix = "/event"
	base := p
	if l := len(eventSuffix); len(p) > l && p[len(p)-l:] == eventSuffix {
		base = p[:len(p)-l]
	}
	return filepath.Base(base)
}

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ActiveTask es la tarea pendiente #1 del ledger framework-tareas.
// Representa "sobre qué entidad está trabajando el usuario ahora mismo".
// Se usa para inyectar contexto en enrichedAnswer antes de pasarle la
// respuesta al framework que habla.
type ActiveTask struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"`
	EntityRef  string `json:"entity_ref"`
	Title      string `json:"title"`
	Action     string `json:"action"`
	Notes      string `json:"notes"`
}

// activeTaskCache cachea la respuesta del ledger para no hacer exec del
// binario en cada turn. TTL corto: la ventana importante es dentro de la
// misma conversación, y el ledger cambia cuando llega un email_sent.
var (
	activeTaskMu    sync.RWMutex
	activeTaskValue *ActiveTask
	activeTaskAt    time.Time
)

const activeTaskTTL = 15 * time.Second

// activeTaskContext devuelve la tarea activa (la primera pendiente o en
// curso) del ledger del profile actual. Devuelve nil si:
//   - no hay binario frameworktareas accesible
//   - el ledger no tiene tareas pendientes
//   - hay un error (silencioso, best-effort)
func activeTaskContext() *ActiveTask {
	activeTaskMu.RLock()
	if activeTaskValue != nil && time.Since(activeTaskAt) < activeTaskTTL {
		t := *activeTaskValue
		activeTaskMu.RUnlock()
		return &t
	}
	activeTaskMu.RUnlock()

	bin := tareasBinPath()
	if bin == "" {
		return nil
	}
	profile := envOr("REMORA_PROFILE", "default")
	cmd := exec.Command(bin, "next", "--profile", profile)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var resp struct {
		Found bool        `json:"found"`
		Task  *ActiveTask `json:"task"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil
	}

	activeTaskMu.Lock()
	defer activeTaskMu.Unlock()
	if resp.Found && resp.Task != nil {
		activeTaskValue = resp.Task
	} else {
		activeTaskValue = nil
	}
	activeTaskAt = time.Now()
	if activeTaskValue == nil {
		return nil
	}
	t := *activeTaskValue
	return &t
}

// invalidateActiveTaskCache fuerza una relectura del ledger en la próxima
// llamada. Se usa desde handlers que saben que el ledger cambió (ej.
// después de un email_sent con task_id).
func invalidateActiveTaskCache() {
	activeTaskMu.Lock()
	activeTaskValue = nil
	activeTaskAt = time.Time{}
	activeTaskMu.Unlock()
}

// tareasBinPath localiza el binario frameworktareas.
func tareasBinPath() string {
	if v := os.Getenv("REMORA_TAREAS_BIN"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	root := resolveRemoraRoot()
	bin := filepath.Join(root, "framework-tareas", "frameworktareas")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	return ""
}

// buildActiveTaskLine produce una línea de contexto para inyectar al
// principio del enrichedAnswer. Devuelve "" cuando no es necesario
// inyectar (ej. el userAnswer ya menciona explícitamente al deudor, o es
// la primera interacción tipo "Gestionar: X" que el enrichment posterior
// va a manejar con más detalle).
//
// Formato: prosa natural sin metacaracteres. El Channel rechaza `[]`, `"`,
// `\n`, `|`, `;`, “ ` “, `<`, `>` (Axioma 4.3 sanitización de paths), así
// que la inyección debe ser texto plano.
func buildActiveTaskLine(userAnswer string, task *ActiveTask) string {
	if task == nil || task.Title == "" {
		return ""
	}
	// Si el chip Gestionar ya menciona al deudor, no duplicamos: el bloque
	// posterior va a armar una query 360° explícita.
	if strings.HasPrefix(userAnswer, "Gestionar: ") {
		return ""
	}
	// Si el userAnswer ya contiene el nombre del deudor literal, no
	// necesitamos inyectar contexto.
	if strings.Contains(strings.ToLower(userAnswer), strings.ToLower(task.Title)) {
		return ""
	}
	// Sanitizamos el título por si traía metacaracteres. Los reemplazos
	// son conservadores: el nombre del cliente en un ledger casi nunca
	// tiene estos chars, pero defendemos igual.
	clean := sanitizeForArg(task.Title)
	if clean == "" {
		return ""
	}
	return "Contexto: el usuario está trabajando sobre el cliente " + clean +
		" (ref " + sanitizeForArg(task.EntityRef) + ", acción " +
		sanitizeForArg(task.Action) + "). "
}

// sanitizeForArg elimina caracteres que el Channel rechaza en args
// (Axioma 4.3). Reemplaza por espacio cuando aparecen.
func sanitizeForArg(s string) string {
	replacer := strings.NewReplacer(
		"\n", " ", "\r", " ",
		"|", " ", ";", " ",
		"`", " ", "$(", " ",
		"&&", " ", "||", " ",
		">", " ", "<", " ",
		"..", " ",
	)
	return strings.TrimSpace(replacer.Replace(s))
}

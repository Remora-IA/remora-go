// frameworktareas: ledger canónico de tareas por perfil.
//
// Tabla `tasks`:
//   id (text PK), profile, entity_type, entity_ref, action, title, priority,
//   status (pending|in_progress|completed|skipped|failed),
//   created_at, started_at, completed_at, result_ref, notes
//
// Tabla `task_events` (event-sourced log):
//   id, task_id, at, actor, kind, data (JSON)
//
// Comandos: list, next, create, complete, event, seed-from-foco
// + next-question / ingest-answer no-op para el contrato conversacional.
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: frameworktareas <comando> [flags]")
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "next-question":
		fmt.Println(`{}`)
	case "ingest-answer":
		fmt.Println(`{"ok":true}`)
	case "list":
		cmdList(args)
	case "next":
		cmdNext(args)
	case "create":
		cmdCreate(args)
	case "complete":
		cmdComplete(args)
	case "event":
		cmdEvent(args)
	case "seed-from-foco":
		cmdSeedFromFoco(args)
	case "init":
		cmdInit(args)
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", cmd)
		os.Exit(2)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func envOr(k, fb string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fb
}

func resolveDBPath(profile string) string {
	if v := os.Getenv("TASKS_DB_PATH"); v != "" {
		return v
	}
	if profile == "" {
		profile = envOr("REMORA_PROFILE", "default")
	}
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "."))
	return filepath.Join(root, "profiles", profile, "tasks.db")
}

func openDB(profile string) (*sql.DB, error) {
	path := resolveDBPath(profile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir tasks dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS tasks (
  id           TEXT PRIMARY KEY,
  profile      TEXT NOT NULL,
  entity_type  TEXT,
  entity_ref   TEXT,
  action       TEXT NOT NULL,
  title        TEXT,
  priority     INTEGER NOT NULL DEFAULT 100,
  status       TEXT NOT NULL DEFAULT 'pending',
  created_at   TEXT NOT NULL,
  started_at   TEXT,
  completed_at TEXT,
  result_ref   TEXT,
  notes        TEXT
);
CREATE INDEX IF NOT EXISTS tasks_status_priority
  ON tasks(profile, status, priority);
CREATE INDEX IF NOT EXISTS tasks_entity
  ON tasks(profile, entity_type, entity_ref);

CREATE TABLE IF NOT EXISTS task_events (
  id       TEXT PRIMARY KEY,
  task_id  TEXT NOT NULL,
  at       TEXT NOT NULL,
  actor    TEXT,
  kind     TEXT NOT NULL,
  data     TEXT,
  FOREIGN KEY (task_id) REFERENCES tasks(id)
);
CREATE INDEX IF NOT EXISTS task_events_by_task
  ON task_events(task_id, at);
`

func writeJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(v)
}

func writeErr(msg string) {
	writeJSON(map[string]interface{}{"success": false, "error": msg})
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// ─── Commands ─────────────────────────────────────────────────────────────

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	_ = fs.Parse(args)
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	writeJSON(map[string]interface{}{"success": true, "db_path": resolveDBPath(*profile)})
}

type taskRow struct {
	ID          string `json:"id"`
	Profile     string `json:"profile"`
	EntityType  string `json:"entity_type,omitempty"`
	EntityRef   string `json:"entity_ref,omitempty"`
	Action      string `json:"action"`
	Title       string `json:"title,omitempty"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	ResultRef   string `json:"result_ref,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

func scanTasks(rows *sql.Rows) ([]taskRow, error) {
	var out []taskRow
	for rows.Next() {
		var t taskRow
		var et, er, title, started, completed, result, notes sql.NullString
		if err := rows.Scan(&t.ID, &t.Profile, &et, &er, &t.Action, &title, &t.Priority, &t.Status, &t.CreatedAt, &started, &completed, &result, &notes); err != nil {
			return nil, err
		}
		t.EntityType, t.EntityRef = et.String, er.String
		t.Title = title.String
		t.StartedAt, t.CompletedAt = started.String, completed.String
		t.ResultRef, t.Notes = result.String, notes.String
		out = append(out, t)
	}
	return out, nil
}

const tasksColumns = `id, profile, entity_type, entity_ref, action, title, priority, status, created_at, started_at, completed_at, result_ref, notes`

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	status := fs.String("status", "", "filtrar por status (vacío = todos)")
	limit := fs.Int("limit", 100, "")
	_ = fs.Parse(args)
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	prof := *profile
	if prof == "" {
		prof = envOr("REMORA_PROFILE", "default")
	}
	q := `SELECT ` + tasksColumns + ` FROM tasks WHERE profile = ?`
	argsSQL := []interface{}{prof}
	if *status != "" {
		q += ` AND status = ?`
		argsSQL = append(argsSQL, *status)
	}
	q += ` ORDER BY (status = 'completed') ASC, priority ASC, created_at ASC LIMIT ?`
	argsSQL = append(argsSQL, *limit)

	rows, err := db.Query(q, argsSQL...)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer rows.Close()
	tasks, err := scanTasks(rows)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	writeJSON(map[string]interface{}{"tasks": tasks, "count": len(tasks)})
}

func cmdNext(args []string) {
	fs := flag.NewFlagSet("next", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	_ = fs.Parse(args)
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	prof := *profile
	if prof == "" {
		prof = envOr("REMORA_PROFILE", "default")
	}
	rows, err := db.Query(`SELECT `+tasksColumns+` FROM tasks
		WHERE profile = ? AND status IN ('pending','in_progress')
		ORDER BY (status = 'in_progress') DESC, priority ASC, created_at ASC LIMIT 1`, prof)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer rows.Close()
	tasks, err := scanTasks(rows)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	// Contadores para que Foco diga "tarea X de N"
	var pending, inProg, completed int
	_ = db.QueryRow(`SELECT
		SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)
		FROM tasks WHERE profile = ?`, prof).Scan(&pending, &inProg, &completed)
	resp := map[string]interface{}{
		"pending":    pending,
		"in_progress": inProg,
		"completed":  completed,
	}
	if len(tasks) > 0 {
		resp["task"] = tasks[0]
		resp["found"] = true
	} else {
		resp["found"] = false
	}
	writeJSON(resp)
}

func cmdCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	entityType := fs.String("entity-type", "", "")
	entityRef := fs.String("entity-ref", "", "")
	action := fs.String("action", "", "ej: send_reminder_email")
	title := fs.String("title", "", "título legible")
	priority := fs.Int("priority", 100, "menor = más prioritario")
	notes := fs.String("notes", "", "")
	_ = fs.Parse(args)
	if *action == "" {
		writeErr("--action requerido")
		os.Exit(1)
	}
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	prof := *profile
	if prof == "" {
		prof = envOr("REMORA_PROFILE", "default")
	}
	id := newID("task")
	now := nowUTC()
	if _, err := db.Exec(`INSERT INTO tasks(id, profile, entity_type, entity_ref, action, title, priority, status, created_at, notes)
		VALUES(?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), ?, 'pending', ?, NULLIF(?, ''))`,
		id, prof, *entityType, *entityRef, *action, *title, *priority, now, *notes); err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	insertEvent(db, id, "system", "task_created", map[string]interface{}{"action": *action})
	writeJSON(map[string]interface{}{"success": true, "id": id, "created_at": now})
}

func cmdComplete(args []string) {
	fs := flag.NewFlagSet("complete", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	id := fs.String("id", "", "task id")
	resultRef := fs.String("result-ref", "", "puntero al resultado (ej. message:msg_123)")
	_ = fs.Parse(args)
	if *id == "" {
		writeErr("--id requerido")
		os.Exit(1)
	}
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	now := nowUTC()
	res, err := db.Exec(`UPDATE tasks
		SET status = 'completed', completed_at = ?, result_ref = NULLIF(?, '')
		WHERE id = ? AND status != 'completed'`,
		now, *resultRef, *id)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	rows, _ := res.RowsAffected()
	if rows > 0 {
		insertEvent(db, *id, "system", "task_completed", map[string]interface{}{"result_ref": *resultRef})
	}
	writeJSON(map[string]interface{}{"success": true, "id": *id, "rows": rows})
}

func cmdEvent(args []string) {
	fs := flag.NewFlagSet("event", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	taskID := fs.String("id", "", "task id")
	actor := fs.String("actor", "", "quién emitió el evento (ej. mensajero)")
	kind := fs.String("kind", "", "ej. email_sent, email_failed, started")
	data := fs.String("data", "{}", "JSON arbitrario")
	_ = fs.Parse(args)
	if *taskID == "" || *kind == "" {
		writeErr("--id y --kind son requeridos")
		os.Exit(1)
	}
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Validar JSON
	var parsed interface{}
	if err := json.Unmarshal([]byte(*data), &parsed); err != nil {
		writeErr("data debe ser JSON válido: " + err.Error())
		os.Exit(1)
	}
	insertEvent(db, *taskID, *actor, *kind, parsed)

	// Side-effects automáticos según kind
	now := nowUTC()
	autoTransition := ""
	switch *kind {
	case "started":
		_, _ = db.Exec(`UPDATE tasks SET status = 'in_progress', started_at = ? WHERE id = ? AND status = 'pending'`, now, *taskID)
		autoTransition = "in_progress"
	case "email_sent", "completed", "task_done":
		// Marca completed si aún no lo está. result_ref viene del data si existe.
		resultRef := ""
		if m, ok := parsed.(map[string]interface{}); ok {
			if v, ok := m["result_ref"].(string); ok {
				resultRef = v
			} else if v, ok := m["message_id"].(string); ok {
				resultRef = "message:" + v
			}
		}
		_, _ = db.Exec(`UPDATE tasks
			SET status = 'completed', completed_at = ?, result_ref = COALESCE(NULLIF(?, ''), result_ref)
			WHERE id = ? AND status != 'completed'`,
			now, resultRef, *taskID)
		autoTransition = "completed"
	case "email_failed", "failed":
		_, _ = db.Exec(`UPDATE tasks SET status = 'failed' WHERE id = ?`, *taskID)
		autoTransition = "failed"
	}
	writeJSON(map[string]interface{}{
		"success":         true,
		"event_kind":      *kind,
		"auto_transition": autoTransition,
	})
}

func insertEvent(db *sql.DB, taskID, actor, kind string, data interface{}) {
	dataJSON, _ := json.Marshal(data)
	id := newID("evt")
	_, _ = db.Exec(`INSERT INTO task_events(id, task_id, at, actor, kind, data) VALUES(?, ?, ?, NULLIF(?, ''), ?, ?)`,
		id, taskID, nowUTC(), actor, kind, string(dataJSON))
}

// cmdSeedFromFoco crea tareas desde un JSON con la priority_list de Foco.
// Formato esperado del file (ejemplo, también acepta la salida cruda de Foco):
//   {"items": [{"rank":1, "deudor":"Gislason Ltd", "client_id":"3", "saldo_total": 2500000, "dias_mora_max": 45}, ...]}
// Si el JSON es solo un array, también lo acepta.
func cmdSeedFromFoco(args []string) {
	fs := flag.NewFlagSet("seed-from-foco", flag.ExitOnError)
	profile := fs.String("profile", "", "")
	file := fs.String("file", "", "ruta al JSON de priority_list")
	limit := fs.Int("limit", 10, "máx tareas a crear")
	action := fs.String("action", "send_reminder_email", "")
	_ = fs.Parse(args)
	if *file == "" {
		writeErr("--file requerido")
		os.Exit(1)
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	var arr []map[string]interface{}
	// Probar como objeto con .items, si no como array directo
	var asObj struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(raw, &asObj); err == nil && len(asObj.Items) > 0 {
		arr = asObj.Items
	} else if err := json.Unmarshal(raw, &arr); err != nil {
		writeErr("JSON inválido: " + err.Error())
		os.Exit(1)
	}

	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	prof := *profile
	if prof == "" {
		prof = envOr("REMORA_PROFILE", "default")
	}

	created := 0
	skipped := 0
	now := nowUTC()
	for i, it := range arr {
		if i >= *limit {
			break
		}
		ref := firstString(it, "client_id", "entity_ref", "id", "ref")
		title := firstString(it, "deudor", "name", "client_name", "title")
		if ref == "" {
			skipped++
			continue
		}
		// Idempotencia: una task pendiente por (entity_type, entity_ref, action)
		var existing string
		_ = db.QueryRow(`SELECT id FROM tasks
			WHERE profile = ? AND entity_type = 'client' AND entity_ref = ? AND action = ? AND status IN ('pending','in_progress')`,
			prof, ref, *action).Scan(&existing)
		if existing != "" {
			skipped++
			continue
		}
		id := newID("task")
		notes := summarize(it)
		if _, err := db.Exec(`INSERT INTO tasks(id, profile, entity_type, entity_ref, action, title, priority, status, created_at, notes)
			VALUES(?, ?, 'client', ?, ?, NULLIF(?, ''), ?, 'pending', ?, NULLIF(?, ''))`,
			id, prof, ref, *action, title, i+1, now, notes); err != nil {
			skipped++
			continue
		}
		insertEvent(db, id, "foco", "task_created", it)
		created++
	}
	writeJSON(map[string]interface{}{
		"success": true,
		"created": created,
		"skipped": skipped,
	})
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case string:
				if x != "" {
					return x
				}
			case float64:
				return strconv.FormatFloat(x, 'f', -1, 64)
			case int:
				return strconv.Itoa(x)
			}
		}
	}
	return ""
}

func summarize(m map[string]interface{}) string {
	parts := []string{}
	if v, ok := m["saldo_total"]; ok {
		parts = append(parts, fmt.Sprintf("saldo=%v", v))
	}
	if v, ok := m["dias_mora_max"]; ok {
		parts = append(parts, fmt.Sprintf("mora=%v", v))
	}
	return strings.Join(parts, " ")
}

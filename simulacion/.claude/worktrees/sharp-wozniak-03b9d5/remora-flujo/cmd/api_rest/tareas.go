package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type focoTaskPlan struct {
	Date      string          `json:"date"`
	Version   string          `json:"version"`
	Result    string          `json:"result,omitempty"`
	Objective string          `json:"objective"`
	Notes     []focoTaskNote  `json:"notes"`
	Events    []focoTaskEvent `json:"events,omitempty"`
	Tasks     []focoTask      `json:"tasks"`
}

type focoTaskNote struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Time string `json:"time"`
}

type focoTaskEvent struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	Time   string `json:"time,omitempty"`
	Result string `json:"result,omitempty"`
	Why    string `json:"why,omitempty"`
	Status string `json:"status"`
}

type focoTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	EventID     string `json:"event_id,omitempty"`
	Why         string `json:"why"`
	Expected    string `json:"expected"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Priority    string `json:"priority,omitempty"`
	PreConflict string `json:"pre_conflict,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	CarriedFrom string `json:"carried_from,omitempty"`
	Importance  int    `json:"importance,omitempty"`
	Evidence    string `json:"evidence,omitempty"`
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

func currentProfile() string {
	return envOr("REMORA_PROFILE", "default")
}

func emitTaskEvent(taskID, actor, kind string, data map[string]interface{}) {
	if taskID == "" {
		return
	}
	if _, err := applyFocoTaskEvent(currentProfile(), taskID, actor, kind, data); err != nil {
		fmt.Printf("[foco_tasks] emit event %s/%s err: %v\n", taskID, kind, err)
	}
}

func (s *server) handleTasksList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	res, err := focoTasksList(currentProfile(), status)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, res)
}

func (s *server) handleTasksNext(w http.ResponseWriter, r *http.Request) {
	res, err := focoTasksNext(currentProfile())
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
	res, err := createFocoTask(currentProfile(), req)
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
	res, err := applyFocoTaskEvent(currentProfile(), taskID, req.Actor, req.Kind, req.Data)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, res)
}

func taskIDFromPath(p string) string {
	const eventSuffix = "/event"
	base := p
	if l := len(eventSuffix); len(p) > l && p[len(p)-l:] == eventSuffix {
		base = p[:len(p)-l]
	}
	return filepath.Base(base)
}

func focoTasksList(profile, status string) (map[string]interface{}, error) {
	plan, _, err := loadFocoTaskPlan(profile)
	if err != nil {
		return nil, err
	}
	tasks := focoTaskRows(profile, plan.Tasks, status)
	return map[string]interface{}{"tasks": tasks, "count": len(tasks)}, nil
}

func focoTasksNext(profile string) (map[string]interface{}, error) {
	plan, _, err := loadFocoTaskPlan(profile)
	if err != nil {
		return nil, err
	}
	all := focoTaskRows(profile, plan.Tasks, "")
	resp := map[string]interface{}{"pending": 0, "in_progress": 0, "completed": 0, "found": false}
	for _, task := range all {
		switch task.Status {
		case "pending":
			resp["pending"] = resp["pending"].(int) + 1
		case "in_progress":
			resp["in_progress"] = resp["in_progress"].(int) + 1
		case "completed":
			resp["completed"] = resp["completed"].(int) + 1
		}
	}
	for _, task := range all {
		if task.Status == "in_progress" || task.Status == "pending" {
			resp["task"] = task
			resp["found"] = true
			break
		}
	}
	return resp, nil
}

func activeTaskFromFoco(profile string) (*ActiveTask, error) {
	res, err := focoTasksNext(profile)
	if err != nil {
		return nil, err
	}
	if found, _ := res["found"].(bool); !found {
		return nil, nil
	}
	task, ok := res["task"].(taskRow)
	if !ok {
		return nil, nil
	}
	return &ActiveTask{ID: task.ID, EntityType: task.EntityType, EntityRef: task.EntityRef, Title: task.Title, Action: task.Action, Notes: task.Notes}, nil
}

func createFocoTask(profile string, req createTaskReq) (map[string]interface{}, error) {
	plan, path, err := loadFocoTaskPlan(profile)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := nextFocoTaskID(plan.Tasks)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(req.EntityRef)
	}
	if title == "" {
		title = req.Action
	}
	task := focoTask{
		ID:        id,
		Title:     title,
		Why:       "Creada desde endpoint REST de tareas; Foco es la fuente de verdad.",
		Expected:  req.Action,
		Status:    "todo",
		CreatedAt: now,
		Priority:  fmt.Sprintf("%d", req.Priority),
		DueDate:   time.Now().Format("2006-01-02"),
		Evidence:  focoTaskEvidence(req.EntityType, req.EntityRef, req.Action, req.Notes),
	}
	plan.Tasks = append(plan.Tasks, task)
	plan.Notes = append(plan.Notes, focoTaskNote{Kind: "flow", Text: "task_created: " + id + " " + title, Time: now})
	if err := saveFocoTaskPlan(path, plan); err != nil {
		return nil, err
	}
	invalidateActiveTaskCache()
	return map[string]interface{}{"success": true, "id": id, "created_at": now}, nil
}

func applyFocoTaskEvent(profile, taskID, actor, kind string, data map[string]interface{}) (map[string]interface{}, error) {
	if data == nil {
		data = map[string]interface{}{}
	}
	plan, path, err := loadFocoTaskPlan(profile)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	autoTransition := ""
	for i := range plan.Tasks {
		if plan.Tasks[i].ID != taskID {
			continue
		}
		switch kind {
		case "started":
			if plan.Tasks[i].Status == "todo" || plan.Tasks[i].Status == "pending" {
				plan.Tasks[i].Status = "in_progress"
			}
			autoTransition = "in_progress"
		case "email_sent", "completed", "task_done":
			plan.Tasks[i].Status = "done"
			plan.Tasks[i].CompletedAt = now
			autoTransition = "completed"
		case "email_failed", "failed":
			plan.Tasks[i].Status = "failed"
			autoTransition = "failed"
		}
		if resultRef := taskStringFromMap(data, "result_ref", "message_id"); resultRef != "" {
			plan.Tasks[i].Evidence = strings.TrimSpace(plan.Tasks[i].Evidence + " result_ref=" + resultRef)
		}
		break
	}
	dataJSON, _ := json.Marshal(data)
	plan.Notes = append(plan.Notes, focoTaskNote{Kind: "flow", Text: fmt.Sprintf("task_event: %s actor=%s kind=%s data=%s", taskID, actor, kind, string(dataJSON)), Time: now})
	if err := saveFocoTaskPlan(path, plan); err != nil {
		return nil, err
	}
	invalidateActiveTaskCache()
	return map[string]interface{}{"success": true, "event_kind": kind, "auto_transition": autoTransition}, nil
}

func loadFocoTaskPlan(profile string) (focoTaskPlan, string, error) {
	path := focoTaskStatePath()
	plan, err := readFocoTaskPlan(path)
	if err != nil && !os.IsNotExist(err) {
		return focoTaskPlan{}, path, err
	}
	if os.IsNotExist(err) {
		plan = newFocoTaskPlan()
	}
	if len(plan.Tasks) == 0 {
		if migrated, ok, migrateErr := migrateLegacyTasksToFoco(profile, plan); migrateErr != nil {
			plan.Notes = append(plan.Notes, focoTaskNote{Kind: "flow", Text: "legacy_tasks_migration_warning: " + migrateErr.Error(), Time: time.Now().UTC().Format(time.RFC3339)})
		} else if ok {
			plan = migrated
			if saveErr := saveFocoTaskPlan(path, plan); saveErr != nil {
				return focoTaskPlan{}, path, saveErr
			}
		}
	}
	return plan, path, nil
}

func focoTaskStatePath() string {
	if v := strings.TrimSpace(os.Getenv("REMORA_FOCO_TASK_STATE_PATH")); v != "" {
		return v
	}
	root := resolveRemoraRoot()
	frameworkDir := filepath.Join(root, "framework-foco")
	globalPath := filepath.Join(frameworkDir, "foco_state.json")
	if plan, err := readFocoTaskPlan(globalPath); err == nil && len(plan.Tasks) > 0 {
		return globalPath
	}
	if path := latestFocoPersistentStatePath(frameworkDir); path != "" {
		return path
	}
	return globalPath
}

func latestFocoPersistentStatePath(frameworkDir string) string {
	root := filepath.Join(frameworkDir, "temp", "foco", "persistent")
	var best string
	var bestMod time.Time
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || info.Name() != "flow_state.json" {
			return nil
		}
		plan, readErr := readFocoTaskPlan(path)
		if readErr != nil || len(plan.Tasks) == 0 {
			return nil
		}
		if best == "" || info.ModTime().After(bestMod) {
			best, bestMod = path, info.ModTime()
		}
		return nil
	})
	return best
}

func readFocoTaskPlan(path string) (focoTaskPlan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return focoTaskPlan{}, err
	}
	var plan focoTaskPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return focoTaskPlan{}, err
	}
	if plan.Notes == nil {
		plan.Notes = []focoTaskNote{}
	}
	if plan.Events == nil {
		plan.Events = []focoTaskEvent{}
	}
	if plan.Tasks == nil {
		plan.Tasks = []focoTask{}
	}
	return plan, nil
}

func saveFocoTaskPlan(path string, plan focoTaskPlan) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func newFocoTaskPlan() focoTaskPlan {
	return focoTaskPlan{Date: time.Now().Format("2006-01-02"), Version: "api_tasks", Notes: []focoTaskNote{}, Events: []focoTaskEvent{}, Tasks: []focoTask{}}
}

func focoTaskRows(profile string, tasks []focoTask, status string) []taskRow {
	rows := make([]taskRow, 0, len(tasks))
	for _, task := range tasks {
		row := focoTaskToRow(profile, task)
		if status != "" && row.Status != status {
			continue
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Status == "completed" && rows[j].Status != "completed" {
			return false
		}
		if rows[j].Status == "completed" && rows[i].Status != "completed" {
			return true
		}
		if rows[i].Priority != rows[j].Priority {
			return rows[i].Priority < rows[j].Priority
		}
		return rows[i].CreatedAt < rows[j].CreatedAt
	})
	return rows
}

func focoTaskToRow(profile string, task focoTask) taskRow {
	meta := parseFocoTaskEvidence(task.Evidence)
	priority := 100
	if task.Importance > 0 {
		priority = task.Importance
	} else if task.Priority != "" {
		_, _ = fmt.Sscanf(task.Priority, "%d", &priority)
	}
	return taskRow{
		ID:          task.ID,
		Profile:     profile,
		EntityType:  meta["entity_type"],
		EntityRef:   meta["entity_ref"],
		Action:      firstNonEmptyString(meta["action"], task.Expected),
		Title:       task.Title,
		Priority:    priority,
		Status:      focoStatusToAPI(task.Status),
		CreatedAt:   task.CreatedAt,
		CompletedAt: task.CompletedAt,
		ResultRef:   meta["result_ref"],
		Notes:       firstNonEmptyString(meta["notes"], task.Why),
	}
}

func focoStatusToAPI(status string) string {
	switch strings.TrimSpace(status) {
	case "done", "completed":
		return "completed"
	case "in_progress":
		return "in_progress"
	case "failed":
		return "failed"
	default:
		return "pending"
	}
}

func nextFocoTaskID(tasks []focoTask) string {
	max := 0
	for _, task := range tasks {
		var n int
		if _, err := fmt.Sscanf(task.ID, "task_%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("task_%03d", max+1)
}

func focoTaskEvidence(entityType, entityRef, action, notes string) string {
	parts := []string{}
	for key, value := range map[string]string{"entity_type": entityType, "entity_ref": entityRef, "action": action, "notes": notes} {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, key+"="+strings.ReplaceAll(value, " ", "_"))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func parseFocoTaskEvidence(evidence string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Fields(evidence) {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			out[key] = strings.ReplaceAll(value, "_", " ")
		}
	}
	return out
}

func migrateLegacyTasksToFoco(profile string, plan focoTaskPlan) (focoTaskPlan, bool, error) {
	path := legacyTasksDBPath(profile)
	if _, err := os.Stat(path); err != nil {
		return plan, false, nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return plan, false, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, profile, entity_type, entity_ref, action, title, priority, status, created_at, started_at, completed_at, result_ref, notes FROM tasks WHERE profile = ? ORDER BY (status = 'completed') ASC, priority ASC, created_at ASC`, profile)
	if err != nil {
		return plan, false, err
	}
	defer rows.Close()
	migrated := 0
	for rows.Next() {
		var row taskRow
		var entityType, entityRef, title, startedAt, completedAt, resultRef, notes sql.NullString
		if err := rows.Scan(&row.ID, &row.Profile, &entityType, &entityRef, &row.Action, &title, &row.Priority, &row.Status, &row.CreatedAt, &startedAt, &completedAt, &resultRef, &notes); err != nil {
			return plan, false, err
		}
		row.EntityType, row.EntityRef, row.Title = entityType.String, entityRef.String, title.String
		row.StartedAt, row.CompletedAt, row.ResultRef, row.Notes = startedAt.String, completedAt.String, resultRef.String, notes.String
		plan.Tasks = append(plan.Tasks, focoTask{
			ID:          row.ID,
			Title:       firstNonEmptyString(row.Title, row.EntityRef, row.Action),
			Why:         firstNonEmptyString(row.Notes, "Migrada desde tasks.db legado."),
			Expected:    row.Action,
			Status:      apiStatusToFoco(row.Status),
			CreatedAt:   row.CreatedAt,
			CompletedAt: row.CompletedAt,
			Priority:    fmt.Sprintf("%d", row.Priority),
			Evidence:    focoTaskEvidence(row.EntityType, row.EntityRef, row.Action, row.Notes),
		})
		migrated++
	}
	if migrated == 0 {
		return plan, false, rows.Err()
	}
	plan.Notes = append(plan.Notes, focoTaskNote{Kind: "flow", Text: fmt.Sprintf("legacy_tasks_migrated: %d tareas desde %s", migrated, path), Time: time.Now().UTC().Format(time.RFC3339)})
	return plan, true, rows.Err()
}

func legacyTasksDBPath(profile string) string {
	if v := os.Getenv("TASKS_DB_PATH"); v != "" {
		return v
	}
	return filepath.Join(resolveRemoraRoot(), "profiles", profile, "tasks.db")
}

func apiStatusToFoco(status string) string {
	switch status {
	case "completed":
		return "done"
	case "in_progress", "failed":
		return status
	default:
		return "todo"
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func taskStringFromMap(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

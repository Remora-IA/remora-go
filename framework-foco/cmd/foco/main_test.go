package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersistNextCollectionTaskCreatesPendingTaskIdempotently(t *testing.T) {
	tmp := t.TempDir()
	oldState, oldMD := statePath, mdPath
	oldLegacyState, oldLegacyMD := legacyStatePath, legacyMDPath
	oldPending, oldPriority := pendingReplyPath, priorityListPath
	oldPersistent := persistentStatePath
	defer func() {
		statePath, mdPath = oldState, oldMD
		legacyStatePath, legacyMDPath = oldLegacyState, oldLegacyMD
		pendingReplyPath, priorityListPath = oldPending, oldPriority
		persistentStatePath = oldPersistent
	}()
	statePath = filepath.Join(tmp, "state.json")
	mdPath = filepath.Join(tmp, "state.md")
	legacyStatePath = filepath.Join(tmp, "legacy.json")
	legacyMDPath = filepath.Join(tmp, "legacy.md")
	pendingReplyPath = filepath.Join(tmp, "pending.json")
	priorityListPath = filepath.Join(tmp, "priorities.json")
	persistentStatePath = ""

	first := persistNextCollectionTask("panalbit", "184", "Thiel-Effertz", "Cobrar a Thiel-Effertz", "Preparar cobranza", "quick_collection", "Preparar cobranza rápida")
	if first.ID == "" || first.Status != "pending" {
		t.Fatalf("unexpected first task %#v", first)
	}
	second := persistNextCollectionTask("panalbit", "184", "Thiel-Effertz", "Cobrar a Thiel-Effertz", "Preparar cobranza", "", "")
	if second.ID != first.ID {
		t.Fatalf("expected same open task, got first=%#v second=%#v", first, second)
	}
	plan, err := load()
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected one task, got %#v", plan.Tasks)
	}
}

func TestFirstOpenPriorityCandidateSkipsCompletedEntity(t *testing.T) {
	plan := newSessionPlan()
	plan.Tasks = []Task{{
		ID:        "collection_panalbit_184",
		Title:     "Cobrar a Thiel-Effertz",
		Expected:  "Preparar cobranza",
		Status:    "done",
		Evidence:  "entity_ref=184; entity_name=Thiel-Effertz",
		CreatedAt: "2026-05-10T00:00:00Z",
	}}
	priorityList := `{
		"items": [
			{"deudor_id":"184","deudor":"Thiel-Effertz","entity_ref":{"id":"184","name":"Thiel-Effertz","type":"customer"}},
			{"deudor_id":"185","deudor":"Nicolas, Hickle and Conroy","entity_ref":{"id":"185","name":"Nicolas, Hickle and Conroy","type":"customer"}}
		]
	}`

	candidate, ok := firstOpenPriorityCandidate(plan, priorityList)
	if !ok {
		t.Fatal("expected candidate")
	}
	if candidate.ID != "185" {
		t.Fatalf("expected second candidate, got %#v", candidate)
	}
}

func TestPriorityCandidatePreservesLedgerTaskID(t *testing.T) {
	priorityList := `{"items":[{"task_id":"task_001","deudor_id":"184","deudor":"Thiel-Effertz","entity_ref":{"id":"184","name":"Thiel-Effertz","type":"customer"}}]}`

	candidate, ok := firstOpenPriorityCandidate(newSessionPlan(), priorityList)
	if !ok {
		t.Fatal("expected candidate")
	}
	if candidate.TaskID != "task_001" {
		t.Fatalf("task_id=%q want task_001", candidate.TaskID)
	}
}

func TestNormalizeActionOptionsFromNil(t *testing.T) {
	result := buildActionOptionsFromStrategy(nil, "")
	if len(result) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(result), result)
	}
	if result[2]["id"] != "skip_case" {
		t.Fatalf("third option id=%#v want skip_case", result[2]["id"])
	}
	for _, option := range result {
		for _, value := range option {
			text, ok := value.(string)
			if !ok {
				continue
			}
			lower := strings.ToLower(text)
			if strings.Contains(lower, "cobrar") || strings.Contains(lower, "cobranza") {
				t.Fatalf("fallback should be generic, got %#v", option)
			}
		}
	}
}

func TestNormalizeActionOptionsFromOneRecommendation(t *testing.T) {
	result := buildActionOptionsFromStrategy(map[string]interface{}{
		"recommendations": []interface{}{
			map[string]interface{}{"action_id": "call_client", "label": "Llamar", "description": "Contactar ahora."},
		},
	}, "")
	if len(result) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(result), result)
	}
	if result[0]["id"] != "call_client" || result[0]["label"] != "Llamar" || result[0]["description"] != "Contactar ahora." {
		t.Fatalf("first option should come from strategy, got %#v", result[0])
	}
	if result[2]["id"] != "skip_case" {
		t.Fatalf("third option id=%#v want skip_case", result[2]["id"])
	}
}

func TestNormalizeActionOptionsFromFiveRecommendations(t *testing.T) {
	result := buildActionOptionsFromStrategy(map[string]interface{}{
		"recommendations": []interface{}{
			map[string]interface{}{"action_id": "first", "label": "Primera"},
			map[string]interface{}{"action_id": "second", "label": "Segunda"},
			map[string]interface{}{"action_id": "third", "label": "Tercera"},
			map[string]interface{}{"action_id": "fourth", "label": "Cuarta"},
			map[string]interface{}{"action_id": "fifth", "label": "Quinta"},
		},
	}, "")
	if len(result) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(result), result)
	}
	if result[0]["id"] != "first" || result[1]["id"] != "second" {
		t.Fatalf("expected first two strategy options, got %#v", result)
	}
	if result[2]["id"] != "skip_case" {
		t.Fatalf("third option id=%#v want skip_case", result[2]["id"])
	}
}

func TestNormalizeActionOptionsSkipCaseAlwaysLast(t *testing.T) {
	result := buildActionOptionsFromStrategy(map[string]interface{}{
		"recommendations": []interface{}{
			map[string]interface{}{"action_id": "skip_case", "label": "Pasar", "description": "Pasar ahora."},
			map[string]interface{}{"action_id": "first", "label": "Primera"},
			map[string]interface{}{"action_id": "second", "label": "Segunda"},
		},
	}, "")
	if len(result) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(result), result)
	}
	if result[0]["id"] != "first" || result[1]["id"] != "second" {
		t.Fatalf("expected non-skip options in original order, got %#v", result)
	}
	if result[2]["id"] != "skip_case" {
		t.Fatalf("third option id=%#v want skip_case", result[2]["id"])
	}
}

func TestNormalizeActionOptionsExactlyThree(t *testing.T) {
	result := buildActionOptionsFromStrategy(map[string]interface{}{
		"recommendations": []interface{}{
			map[string]interface{}{"action_id": "first", "label": "Primera", "description": "Uno."},
			map[string]interface{}{"action_id": "second", "label": "Segunda", "description": "Dos."},
			map[string]interface{}{"action_id": "skip_case", "label": "Pasar custom", "description": "Tres."},
		},
	}, "")
	if len(result) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(result), result)
	}
	if result[0]["id"] != "first" || result[0]["label"] != "Primera" || result[0]["description"] != "Uno." {
		t.Fatalf("first option changed: %#v", result[0])
	}
	if result[1]["id"] != "second" || result[1]["label"] != "Segunda" || result[1]["description"] != "Dos." {
		t.Fatalf("second option changed: %#v", result[1])
	}
	if result[2]["id"] != "skip_case" || result[2]["label"] != "Pasar custom" || result[2]["description"] != "Tres." {
		t.Fatalf("skip option changed or moved incorrectly: %#v", result[2])
	}
}

func TestCarryOverPendingTasks(t *testing.T) {
	configureTempFocoSession(t, "business_acme__flow_main__"+testToday())
	yesterday := testYesterday()
	writeTestPlan(t, persistentStatePath, DayPlan{Date: yesterday, Version: "persistent", Tasks: []Task{
		{ID: "task_001", Title: "Pendiente 1", Status: "pending", DueDate: yesterday},
		{ID: "task_002", Title: "Pendiente 2", Status: "pending", DueDate: yesterday},
		{ID: "task_003", Title: "Hecha", Status: "done", DueDate: yesterday},
	}})

	plan, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 carried tasks, got %#v", plan.Tasks)
	}
	for _, id := range []string{"task_001", "task_002"} {
		task, ok := testTaskByID(plan, id)
		if !ok {
			t.Fatalf("missing carried task %s in %#v", id, plan.Tasks)
		}
		if task.CarriedFrom != yesterday {
			t.Fatalf("task %s carried_from=%q want %q", id, task.CarriedFrom, yesterday)
		}
		if task.DueDate != plan.Date {
			t.Fatalf("task %s due_date=%q want %q", id, task.DueDate, plan.Date)
		}
	}
	if _, ok := testTaskByID(plan, "task_003"); ok {
		t.Fatalf("done task should not be carried: %#v", plan.Tasks)
	}
}

func TestNoCarryOverWhenAllDone(t *testing.T) {
	configureTempFocoSession(t, "business_acme__flow_main__"+testToday())
	yesterday := testYesterday()
	writeTestPlan(t, persistentStatePath, DayPlan{Date: yesterday, Version: "persistent", Tasks: []Task{
		{ID: "task_001", Title: "Hecha 1", Status: "done", DueDate: yesterday},
		{ID: "task_002", Title: "Hecha 2", Status: "done", DueDate: yesterday},
	}})

	plan, err := load()
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range plan.Tasks {
		if task.CarriedFrom != "" {
			t.Fatalf("expected no carried tasks, got %#v", plan.Tasks)
		}
	}
}

func TestSaveSyncsToPersistent(t *testing.T) {
	configureTempFocoSession(t, "business_acme__flow_main__"+testToday())
	plan := newSessionPlan()
	plan.Tasks = append(plan.Tasks, Task{ID: "task_001", Title: "Cerrar", Status: "done", DueDate: plan.Date})
	if err := save(plan); err != nil {
		t.Fatal(err)
	}

	persistent := readTestPlan(t, persistentStatePath)
	task, ok := testTaskByID(persistent, "task_001")
	if !ok {
		t.Fatalf("missing task in persistent state: %#v", persistent.Tasks)
	}
	if task.Status != "done" {
		t.Fatalf("persistent task status=%q want done", task.Status)
	}
}

func TestCarryOverDoesNotDuplicate(t *testing.T) {
	configureTempFocoSession(t, "business_acme__flow_main__"+testToday())
	yesterday := testYesterday()
	writeTestPlan(t, persistentStatePath, DayPlan{Date: yesterday, Version: "persistent", Tasks: []Task{
		{ID: "task_001", Title: "Persistente", Status: "pending", DueDate: yesterday},
	}})
	writeTestPlan(t, statePath, DayPlan{Date: testToday(), Version: "session", Tasks: []Task{
		{ID: "task_001", Title: "Diaria", Status: "pending", DueDate: testToday()},
	}})

	plan, err := load()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, task := range plan.Tasks {
		if task.ID == "task_001" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("task_001 count=%d want 1 in %#v", count, plan.Tasks)
	}
}

func configureTempFocoSession(t *testing.T, convID string) {
	t.Helper()
	tmp := t.TempDir()
	oldState, oldMD := statePath, mdPath
	oldLegacyState, oldLegacyMD := legacyStatePath, legacyMDPath
	oldPending, oldPriority := pendingReplyPath, priorityListPath
	oldPersistent := persistentStatePath
	t.Chdir(tmp)
	configureFocoSession(convID)
	t.Cleanup(func() {
		statePath, mdPath = oldState, oldMD
		legacyStatePath, legacyMDPath = oldLegacyState, oldLegacyMD
		pendingReplyPath, priorityListPath = oldPending, oldPriority
		persistentStatePath = oldPersistent
	})
}

func writeTestPlan(t *testing.T, path string, plan DayPlan) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func readTestPlan(t *testing.T, path string) DayPlan {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan DayPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

func testTaskByID(plan DayPlan, id string) (Task, bool) {
	for _, task := range plan.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return Task{}, false
}

func testToday() string {
	return time.Now().Format("2006-01-02")
}

func testYesterday() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

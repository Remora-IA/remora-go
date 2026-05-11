package main

import (
	"path/filepath"
	"testing"
)

func TestPersistNextCollectionTaskCreatesPendingTaskIdempotently(t *testing.T) {
	tmp := t.TempDir()
	oldState, oldMD := statePath, mdPath
	oldLegacyState, oldLegacyMD := legacyStatePath, legacyMDPath
	oldPending, oldPriority := pendingReplyPath, priorityListPath
	defer func() {
		statePath, mdPath = oldState, oldMD
		legacyStatePath, legacyMDPath = oldLegacyState, oldLegacyMD
		pendingReplyPath, priorityListPath = oldPending, oldPriority
	}()
	statePath = filepath.Join(tmp, "state.json")
	mdPath = filepath.Join(tmp, "state.md")
	legacyStatePath = filepath.Join(tmp, "legacy.json")
	legacyMDPath = filepath.Join(tmp, "legacy.md")
	pendingReplyPath = filepath.Join(tmp, "pending.json")
	priorityListPath = filepath.Join(tmp, "priorities.json")

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

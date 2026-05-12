package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadActiveSessionScopedByConversation(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	payload := map[string]interface{}{
		"artifact_type":     segmentSessionArtifact,
		"business_id":       "biz-1",
		"status":            "active",
		"owner_framework":   "radar",
		"owner_capability":  "analysis.deep_dive",
		"followup_command":  "analyze-followup",
		"conversation_id":   "conv-a",
		"segment_id":        "seg-1",
		"allowed_delegates": []interface{}{"data.entity_360"},
	}
	s.persistFlowArtifact("run-a", "segment_session", segmentSessionArtifact, payload)

	got, err := s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil {
		t.Fatalf("load active session: %v", err)
	}
	if got == nil {
		t.Fatalf("expected session for owning conversation")
	}
	if got.ConversationID != "conv-a" || got.SegmentID != "seg-1" {
		t.Fatalf("unexpected scope: conversation=%q segment=%q", got.ConversationID, got.SegmentID)
	}

	other, err := s.loadActiveSessionFromDisk("biz-1", "conv-b")
	if err != nil {
		t.Fatalf("load active session for other conversation: %v", err)
	}
	if other != nil {
		t.Fatalf("expected no session for different conversation, got %+v", other)
	}
}

func TestUnclaimedSessionCanBeClaimedBySingleConversation(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	payload := map[string]interface{}{
		"artifact_type":    segmentSessionArtifact,
		"business_id":      "biz-1",
		"status":           "active",
		"owner_framework":  "radar",
		"owner_capability": "analysis.deep_dive",
		"followup_command": "analyze-followup",
		"segment_id":       "seg-1",
	}
	path := s.persistFlowArtifact("run-a", "segment_session", segmentSessionArtifact, payload)

	got, err := s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil {
		t.Fatalf("load unclaimed session: %v", err)
	}
	if got == nil || got.ConversationID != "" {
		t.Fatalf("expected unclaimed session, got %+v", got)
	}
	s.claimSessionForConversation(path, "conv-a")

	got, err = s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil || got == nil || got.ConversationID != "conv-a" {
		t.Fatalf("expected claimed session for conv-a, got session=%+v err=%v", got, err)
	}
	other, err := s.loadActiveSessionFromDisk("biz-1", "conv-b")
	if err != nil {
		t.Fatalf("load claimed session for other conversation: %v", err)
	}
	if other != nil {
		t.Fatalf("expected claimed session to be invisible to conv-b, got %+v", other)
	}
}

func TestLoadActiveSessionPrefersConversationClaimOverNewerUnclaimedSession(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	s.persistFlowArtifact("run-a", "segment_session", segmentSessionArtifact, map[string]interface{}{
		"artifact_type":    segmentSessionArtifact,
		"business_id":      "biz-1",
		"status":           "active",
		"owner_framework":  "radar",
		"owner_capability": "analysis.deep_dive",
		"followup_command": "analyze-followup",
		"conversation_id":  "conv-a",
		"segment_id":       "seg-a",
	})
	s.persistFlowArtifact("run-b", "segment_session", segmentSessionArtifact, map[string]interface{}{
		"artifact_type":    segmentSessionArtifact,
		"business_id":      "biz-1",
		"status":           "active",
		"owner_framework":  "radar",
		"owner_capability": "analysis.deep_dive",
		"followup_command": "analyze-followup",
		"segment_id":       "seg-b",
	})

	got, err := s.loadActiveSessionFromDisk("biz-1", "conv-a")
	if err != nil {
		t.Fatalf("load active session: %v", err)
	}
	if got == nil || got.SegmentID != "seg-a" {
		t.Fatalf("expected conv-a to keep its claimed segment, got %+v", got)
	}
}

func TestExecuteDelegationsEnforcesAllowedDelegates(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
	requests := []map[string]interface{}{
		{
			"framework":  "sabio",
			"capability": "data.query.sql",
			"params":     map[string]interface{}{"question": "portfolio"},
		},
	}
	results := s.executeDelegations(context.Background(), nil, &Conversation{Frameworks: []string{"sabio"}}, nil, requests, []string{"data.entity_360"})
	if len(results) != 0 {
		t.Fatalf("expected blocked delegation to produce no results, got %+v", results)
	}
}

func TestClassifySegmentIntentSeparatesContinueOperationalExit(t *testing.T) {
	session := &activeSessionInfo{
		ContinueSignals:    []string{"profundiza", "detalle", "riesgo", "mora"},
		OperationalSignals: []string{"avanza", "con eso basta", "manda"},
		ExitSignals:        []string{"siguiente caso", "déjalo"},
	}
	cases := []struct {
		input string
		want  segmentIntentType
	}{
		{"ok, pero profundiza más el riesgo", segmentIntentContinue},
		{"dale más detalle de la mora", segmentIntentContinue},
		{"ok, con eso basta, avanza", segmentIntentOperational},
		{"déjalo ahí, siguiente caso", segmentIntentExit},
	}
	for _, tc := range cases {
		got := classifySegmentIntent(tc.input, session)
		if got != tc.want {
			t.Fatalf("classifySegmentIntent(%q)=%s, want %s", tc.input, got, tc.want)
		}
	}
}

func TestPersistAnalysisHandoffCreatesStructuredArtifact(t *testing.T) {
	root := t.TempDir()
	s := &server{rootDir: root}
	s.persistFlowArtifact("review", "radar", "analysis.case_review.v1", map[string]interface{}{
		"artifact_type":  "analysis.case_review.v1",
		"business_id":    "biz-1",
		"text":           "El cliente tiene mora alta y riesgo moderado.",
		"recommendation": "Contactar con mensaje de cobranza empático.",
		"residual_risks": []string{"historial incompleto"},
		"data_gaps":      []string{"sin contacto alternativo"},
	})
	session := &activeSessionInfo{
		Framework:      "radar",
		Capability:     "analysis.deep_dive",
		ConversationID: "conv-a",
		TurnCount:      2,
	}
	conv := &Conversation{ID: "conv-a", BusinessID: "biz-1"}

	s.persistAnalysisHandoff(context.Background(), nil, conv, nil, session, "ok, con eso basta, avanza")

	path := filepath.Join(root, "temp", "flow_runs", "handoff_biz-1", "artifacts", "analysis_handoff__analysis.handoff.v1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read handoff artifact: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse handoff artifact: %v", err)
	}
	if payload["artifact_type"] != "analysis.handoff.v1" {
		t.Fatalf("unexpected artifact_type: %v", payload["artifact_type"])
	}
	if payload["conversation_id"] != "conv-a" {
		t.Fatalf("handoff not scoped to conversation: %v", payload["conversation_id"])
	}
	if payload["analytical_summary"] == "" || payload["recommendation"] == "" || payload["confidence"] == "" {
		t.Fatalf("handoff missing operational context: %+v", payload)
	}
}

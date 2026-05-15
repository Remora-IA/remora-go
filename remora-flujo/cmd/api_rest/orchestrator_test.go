package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"channel/adapter"
	"channel/manifest"
	"remora-flujo/handoff"
)

type stubFrameworkDriver struct {
	name       string
	poll       nextQuestionResponse
	pollOK     bool
	ingestions []QueuedAnswerCtx
}

func (d *stubFrameworkDriver) Name() string { return d.name }

func (d *stubFrameworkDriver) Init(context.Context, *adapter.Client, *Conversation) error { return nil }

func (d *stubFrameworkDriver) IngestAnswer(_ context.Context, _ *adapter.Client, _ *Conversation, qctx QueuedAnswerCtx) error {
	d.ingestions = append(d.ingestions, qctx)
	return nil
}

func (d *stubFrameworkDriver) PollQuestion(context.Context, *adapter.Client, *Conversation, map[string]bool) (string, string, string, string, bool) {
	if !d.pollOK {
		return "", "", "", "", false
	}
	return d.poll.Text, d.poll.Reasoning, d.poll.ID, d.poll.AskVia, true
}

func TestRunLoopOperationalTransitionReentersThroughCaseManager(t *testing.T) {
	cases := []struct {
		name       string
		ready      bool
		wantMarker string
	}{
		{name: "review_pending", wantMarker: "[analysis_review_pending]"},
		{name: "handoff", ready: true, wantMarker: "[handoff_analitico]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			s := &server{
				rootDir: root,
				allManifests: map[string]*manifest.Manifest{
					"foco": {
						Name: "foco",
						CapabilitiesSemantic: manifest.CapabilitiesSemantic{
							Produces: []string{"task.next", "focus.next_task.v1"},
						},
					},
					"sabio": {Name: "sabio"},
					"radar": {Name: "radar"},
				},
			}
			conv := &Conversation{
				ID:         "conv_operational_" + tc.name,
				BusinessID: "biz-1",
				Frameworks: []string{"sabio", "foco", "radar"},
			}
			t.Cleanup(func() { _ = deleteConv(conv.ID) })

			queue := handoff.NewQuestionsQueue(conv.Frameworks...)
			queue.SetSpeaker("radar")
			queue.AddQuestionWithReasoning("radar", "session_followup_1", "Radar pide siguiente input", "", "text")
			if err := saveQueue(conv.ID, queue); err != nil {
				t.Fatalf("save queue: %v", err)
			}

			s.persistFlowArtifact("review", "radar", "analysis.case_review.v1", map[string]interface{}{
				"artifact_type":  "analysis.case_review.v1",
				"business_id":    "biz-1",
				"text":           "Radar cerró el análisis del caso.",
				"recommendation": "Foco debe decidir siguiente acción.",
				"confidence":     "moderate",
			})
			s.persistFlowArtifact("session", "segment_session", segmentSessionArtifact, map[string]interface{}{
				"artifact_type":       segmentSessionArtifact,
				"business_id":         "biz-1",
				"status":              "active",
				"owner_framework":     "radar",
				"owner_capability":    "analysis.deep_dive",
				"followup_command":    "analyze-followup",
				"conversation_id":     conv.ID,
				"turn_count":          2,
				"segment_id":          "seg-1",
				"operational_signals": []string{"avanza"},
				"continue_signals":    []string{"profundiza"},
				"allowed_delegates":   []string{"data.query.sql"},
			})
			if tc.ready {
				session, err := s.loadActiveSessionFromDisk("biz-1", conv.ID)
				if err != nil || session == nil {
					t.Fatalf("load session: session=%+v err=%v", session, err)
				}
				s.persistAnalysisReadinessArtifact("biz-1", conv.ID, session, `{"artifact_type":"analysis.followup.v1","text":"Radar considera suficiente la evidencia para operar.","recommendation":"Contactar al cliente hoy.","confidence":"high","ready_for_operation":true,"data_gaps":[]}`)
			}

			radar := &stubFrameworkDriver{name: "radar"}
			foco := &stubFrameworkDriver{name: "foco", pollOK: true, poll: nextQuestionResponse{ID: "foco_q1", Text: "Foco retoma el caso", AskVia: "text"}}
			sabio := &stubFrameworkDriver{name: "sabio", pollOK: true, poll: nextQuestionResponse{ID: "sabio_q1", Text: "Sabio pregunta algo", AskVia: "text"}}
			prevRegistry := driverRegistry
			driverRegistry = map[string]FrameworkDriver{
				"radar": radar,
				"foco":  foco,
				"sabio": sabio,
			}
			defer func() { driverRegistry = prevRegistry }()

			activeTaskMu.Lock()
			prevTaskValue, prevTaskAt := activeTaskValue, activeTaskAt
			activeTaskValue = &ActiveTask{}
			activeTaskAt = time.Now()
			activeTaskMu.Unlock()
			defer func() {
				activeTaskMu.Lock()
				activeTaskValue = prevTaskValue
				activeTaskAt = prevTaskAt
				activeTaskMu.Unlock()
			}()
			t.Setenv("PALADIN_SILENT", "1")

			q, ok, err := s.runLoop(context.Background(), nil, conv, nil, s.allManifests, "ok, avanza", nil)
			if err != nil {
				t.Fatalf("runLoop: %v", err)
			}
			if !ok {
				t.Fatalf("expected next question after operational transition")
			}
			if q.Framework != "foco" {
				t.Fatalf("expected foco to retake the case, got question=%+v", q)
			}
			if len(foco.ingestions) != 1 {
				t.Fatalf("expected one ingest into foco, got %+v", foco.ingestions)
			}
			if len(radar.ingestions) != 0 {
				t.Fatalf("expected no ingest into radar after operational transition, got %+v", radar.ingestions)
			}
			if len(sabio.ingestions) != 0 {
				t.Fatalf("expected no ingest into sabio before foco reentry, got %+v", sabio.ingestions)
			}
			answer := foco.ingestions[0].Answer
			if !strings.Contains(answer, tc.wantMarker) || !strings.Contains(answer, "ok, avanza") {
				t.Fatalf("expected foco ingest to include transition context and user trigger, got %q", answer)
			}
			updatedQueue, err := loadQueue(conv.ID)
			if err != nil {
				t.Fatalf("load updated queue: %v", err)
			}
			if len(updatedQueue.Questions) == 0 || updatedQueue.Questions[0].Framework != "radar" || updatedQueue.Questions[0].Status != handoff.QuestionAnswered {
				t.Fatalf("expected radar session question to be consumed before foco reentry, got %+v", updatedQueue.Questions)
			}
		})
	}
}

package main

import (
	"strings"
	"testing"
	"time"

	"channel/manifest"
)

func TestUniversalSingleMessageExposesStandardSession(t *testing.T) {
	s := &server{}
	conv := &Conversation{ID: "conv_test", Frameworks: []string{"demo"}, CreatedAt: time.Now()}
	m := &manifest.Manifest{
		Name: "demo",
		Commands: map[string]manifest.Command{
			"ask": {Args: []string{"ask", "{params.input}"}, Params: []string{"input"}},
		},
	}

	msg := s.createUniversalSingleMessage(conv, m)
	if msg == nil {
		t.Fatal("expected initial universal message")
	}
	if msg.Status != "needs_input" {
		t.Fatalf("status = %q, want needs_input", msg.Status)
	}
	if len(msg.Events) == 0 || msg.Events[0].Type != "framework.needs_input" {
		t.Fatalf("expected framework.needs_input event, got %#v", msg.Events)
	}
	if !strings.Contains(msg.Content, "Comandos disponibles") {
		t.Fatalf("content should expose available commands: %q", msg.Content)
	}
}

func TestSelectUniversalCommandPrefersInputCommand(t *testing.T) {
	m := &manifest.Manifest{
		Name: "demo",
		Commands: map[string]manifest.Command{
			"list": {Args: []string{"list"}, Params: []string{}},
			"ask":  {Args: []string{"ask", "{params.query}"}, Params: []string{"query"}},
		},
	}

	name, value, explicit := selectUniversalCommand(m, "quiero saber algo")
	if name != "ask" || value != "quiero saber algo" || explicit {
		t.Fatalf("auto select = (%q,%q,%v), want ask/free text/false", name, value, explicit)
	}

	name, value, explicit = selectUniversalCommand(m, "/list")
	if name != "list" || value != "" || !explicit {
		t.Fatalf("explicit select = (%q,%q,%v), want list/empty/true", name, value, explicit)
	}
}

func TestInferUniversalParamsFromFreeTextAndJSON(t *testing.T) {
	conv := &Conversation{ID: "conv_123"}
	cmd := manifest.Command{Params: []string{"conv_id", "query", "profile", "limit"}}
	params, missing := inferUniversalParams(conv, cmd, "buscar deuda", nil)
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
	if params["conv_id"] != "conv_123" || params["query"] != "buscar deuda" || params["limit"] != "10" {
		t.Fatalf("unexpected params: %#v", params)
	}

	cmd = manifest.Command{Params: []string{"finding_id"}}
	params, missing = inferUniversalParams(conv, cmd, `{"finding_id":"f1"}`, nil)
	if len(missing) != 0 || params["finding_id"] != "f1" {
		t.Fatalf("json params=%#v missing=%v", params, missing)
	}
}

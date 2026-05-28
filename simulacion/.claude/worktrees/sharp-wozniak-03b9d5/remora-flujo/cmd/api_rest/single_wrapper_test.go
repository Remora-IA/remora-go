package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"channel/manifest"
)

func TestUniversalSingleMessageExposesStandardSession(t *testing.T) {
	s := &server{rootDir: t.TempDir()}
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
	if msg.Status != "error" {
		t.Fatalf("status = %q, want error", msg.Status)
	}
	if len(msg.Events) == 0 {
		t.Fatalf("expected at least one event, got %#v", msg.Events)
	}
	if msg.Content == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestEncodeConversationRuntimeContextIncludesBusinessAndScope(t *testing.T) {
	conv := &Conversation{
		ID:         "conv_test",
		BusinessID: "panalbit",
		RuntimeContext: map[string]any{
			"audience": "collector",
			"scope": map[string]any{
				"allowed_client_ids": []any{"1", "2"},
			},
		},
	}
	encoded := encodeConversationRuntimeContext(conv)
	if encoded == "" {
		t.Fatal("expected encoded context")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["business_id"] != "panalbit" {
		t.Fatalf("missing business_id: %#v", out)
	}
	if out["audience"] != "collector" {
		t.Fatalf("missing audience: %#v", out)
	}
	if _, ok := out["scope"].(map[string]any); !ok {
		t.Fatalf("missing scope: %#v", out)
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

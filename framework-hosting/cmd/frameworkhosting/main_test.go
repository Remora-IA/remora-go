package main

import "testing"

func TestQuestionForStateStartsWithDomain(t *testing.T) {
	s := &state{}
	q := questionForState(s)
	if q == nil {
		t.Fatal("expected initial question")
	}
	if q.Capability != "credentials.cpanel.connect" {
		t.Fatalf("capability=%q want credentials.cpanel.connect", q.Capability)
	}
	if q.Step != "cpanel_domain" || q.Field != "domain" {
		t.Fatalf("unexpected initial step: %#v", q)
	}
	if q.RequiredArtifact != "credentials.smtp" {
		t.Fatalf("required_artifact=%q want credentials.smtp", q.RequiredArtifact)
	}
}

func TestHandleConnectWizardDirectHostAdvancesToUser(t *testing.T) {
	s := &state{SetupStep: "cpanel_domain"}
	q := handleConnectWizard("cpanel.midominio.com", "", "ignored", "conv_1", s)
	if q == nil {
		t.Fatal("expected next question")
	}
	if s.Host != "cpanel.midominio.com" {
		t.Fatalf("host=%q want cpanel.midominio.com", s.Host)
	}
	if s.SetupStep != "cpanel_user" {
		t.Fatalf("setup_step=%q want cpanel_user", s.SetupStep)
	}
	if q.Field != "cpanel_user" || q.Capability != "credentials.cpanel.connect" {
		t.Fatalf("unexpected next question %#v", q)
	}
}

func TestQuestionForStatePasswordStepIsSecret(t *testing.T) {
	s := &state{SetupStep: "cpanel_pass"}
	q := questionForState(s)
	if q == nil {
		t.Fatal("expected password question")
	}
	if q.InputType != "password" || !q.Secret {
		t.Fatalf("expected secret password field, got %#v", q)
	}
	if q.NextTransition != "connect_cpanel" {
		t.Fatalf("next_transition=%q want connect_cpanel", q.NextTransition)
	}
}

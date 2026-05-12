package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (s *server) invokeProviderNextQuestion(ctx context.Context, businessID, capability string) (providerQuestion, bool) {
	m, providerName, providerOK := s.findProviderForCapability(capability)
	if !providerOK {
		return providerQuestion{}, false
	}
	nextQuestionCmd := m.UserInput.NextQuestionCmd
	if nextQuestionCmd == "" {
		nextQuestionCmd = "next-question"
	}
	cmd, ok := m.Commands[nextQuestionCmd]
	if !ok {
		return providerQuestion{}, false
	}
	convID := businessVaultConvID(businessID)
	params := map[string]string{"conv_id": convID}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return providerQuestion{}, false
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := s.scoped(providerName+"_"+businessID).ExecuteCommand(execCtx, runtime.Command, fullArgs, runtime.Cwd)
	cancel()
	if err != nil || resp.ExitCode != 0 {
		return providerQuestion{}, false
	}
	var parsed map[string]interface{}
	if uerr := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &parsed); uerr != nil {
		return providerQuestion{}, false
	}
	if len(parsed) == 0 {
		return providerQuestion{}, false
	}
	question := providerQuestion{
		ID:               jsonFirstString(parsed, "id", "question_id"),
		Text:             jsonFirstString(parsed, "text", "message", "question"),
		Framework:        firstNonEmptyPipelineString(jsonFirstString(parsed, "framework"), providerName),
		Capability:       firstNonEmptyPipelineString(jsonFirstString(parsed, "capability"), capability),
		Title:            jsonFirstString(parsed, "title"),
		Kind:             jsonFirstString(parsed, "kind"),
		Field:            jsonFirstString(parsed, "field"),
		FieldLabel:       jsonFirstString(parsed, "field_label", "label"),
		InputType:        jsonFirstString(parsed, "input_type", "type"),
		Placeholder:      jsonFirstString(parsed, "placeholder"),
		Step:             jsonFirstString(parsed, "step"),
		NextTransition:   jsonFirstString(parsed, "next_transition"),
		RequiredArtifact: jsonFirstString(parsed, "required_artifact", "artifact"),
	}
	if secret, _ := parsed["secret"].(bool); secret {
		question.Secret = true
	}
	if question.Text == "" {
		return providerQuestion{}, false
	}
	return question, true
}

func (s *server) ingestProviderAnswer(ctx context.Context, businessID, capability, questionID, answer string) (responseText string, ok bool) {
	m, providerName, providerOK := s.findProviderForCapability(capability)
	if !providerOK {
		return "", false
	}
	ingestAnswerCmd := m.UserInput.IngestAnswerCmd
	if ingestAnswerCmd == "" {
		ingestAnswerCmd = "ingest-answer"
	}
	cmd, ok := m.Commands[ingestAnswerCmd]
	if !ok {
		return "", false
	}
	convID := businessVaultConvID(businessID)
	params := map[string]string{"conv_id": convID, "question_id": questionID, "answer": answer}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return "", false
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := s.scoped(providerName+"_"+businessID).ExecuteCommand(execCtx, runtime.Command, fullArgs, runtime.Cwd)
	cancel()
	if err != nil || resp.ExitCode != 0 {
		return "", false
	}
	if next, nextOK := s.invokeProviderNextQuestion(ctx, businessID, capability); nextOK {
		return next.Text, true
	}
	// ingest-answer normalmente no devuelve stdout directo; la respuesta puede quedar en el state para next-question
	return "ok", true
}

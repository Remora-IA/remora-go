package main

import (
	"context"
	"strings"
	"time"

	"encoding/json"
	"path/filepath"
)

func (s *server) invokeProviderNextQuestion(ctx context.Context, businessID, capability string) (questionID, questionText, providerName string, ok bool) {
	m, providerName, providerOK := s.findProviderForCapability(capability)
	if !providerOK {
		return "", "", "", false
	}
	nextQuestionCmd := m.UserInput.NextQuestionCmd
	if nextQuestionCmd == "" {
		nextQuestionCmd = "next-question"
	}
	cmd, ok := m.Commands[nextQuestionCmd]
	if !ok {
		return "", "", "", false
	}
	convID := businessVaultConvID(businessID)
	params := map[string]string{"conv_id": convID}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		return "", "", "", false
	}
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + providerName
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := s.scoped(providerName+"_"+businessID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
	cancel()
	if err != nil || resp.ExitCode != 0 {
		return "", "", "", false
	}
	var parsed map[string]interface{}
	if uerr := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &parsed); uerr != nil {
		return "", "", "", false
	}
	if len(parsed) == 0 {
		return "", "", "", false
	}
	qID, _ := parsed["id"].(string)
	qText, _ := parsed["text"].(string)
	if qText == "" {
		return "", "", "", false
	}
	return qID, qText, providerName, true
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
	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, args...)
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + providerName
	}
	cwd := filepath.Join(s.rootDir, cwdRel)
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := s.scoped(providerName+"_"+businessID).ExecuteCommand(execCtx, m.Binary.Command, fullArgs, cwd)
	cancel()
	if err != nil || resp.ExitCode != 0 {
		return "", false
	}
	// ingest-answer no devuelve stdout directo; la respuesta queda en el state para next-question
	return "ok", true
}

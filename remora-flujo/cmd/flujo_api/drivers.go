package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"channel/adapter"
)

// FrameworkDriver es el adaptador entre la API y un framework concreto.
//
// El contrato estandarizado: cada framework expone en su CLI dos comandos
// declarados en framework.manifest.json:
//
//   - next-question  → JSON {"id":"...","text":"...","ask_via":""}  o  {}
//   - ingest-answer  → recibe --question-id y --answer
//
// El driver solo añade el bootstrap (Init) y wraps esos comandos vía Channel.
// Para sumar un framework nuevo basta con: implementar esos dos comandos en
// su CLI, declarar user_input en su manifest y registrarlo aquí (o, en la
// próxima iteración, autoregistro vía discovery del manifest).
type FrameworkDriver interface {
	Name() string
	Init(ctx context.Context, ch *adapter.Client, conv *Conversation) error
	IngestAnswer(ctx context.Context, ch *adapter.Client, conv *Conversation, qctx QueuedAnswerCtx) error
	PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (text, externalID, askVia string, ok bool)
}

// QueuedAnswerCtx es el contexto que el orquestador entrega al driver al
// inyectar la respuesta del usuario.
//
// Resources son los recursos no-textuales del usuario (imágenes, archivos).
// Si el orquestador ya pre-procesó (ej: pasó las imágenes por un modelo
// multimodal y obtuvo descripción estructurada), Answer YA contiene el
// texto enriquecido. Resources se entregan igual por si el driver quiere
// referenciar paths en su evidencia.
type QueuedAnswerCtx struct {
	QuestionID   string
	ExternalID   string
	QuestionText string
	Answer       string
	Resources    []MessageResource
}

var driverRegistry = map[string]FrameworkDriver{
	"echo": &echoDriver{},
	"alfa": &alfaDriver{},
}

func driversFor(conv *Conversation) []FrameworkDriver {
	out := []FrameworkDriver{}
	for _, name := range conv.Frameworks {
		if d, ok := driverRegistry[name]; ok {
			out = append(out, d)
		}
	}
	return out
}

// nextQuestionResponse es el contrato JSON común de `next-question` entre
// frameworks. Campos opcionales se ignoran si están vacíos.
type nextQuestionResponse struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	AskVia string `json:"ask_via"`
}

func parseNextQuestion(stdout string) (nextQuestionResponse, bool) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" || stdout == "{}" {
		return nextQuestionResponse{}, false
	}
	var r nextQuestionResponse
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		return nextQuestionResponse{}, false
	}
	if r.ID == "" || r.Text == "" {
		return nextQuestionResponse{}, false
	}
	return r, true
}

// ---------------------------------------------------------------------------
// Echo driver
// ---------------------------------------------------------------------------

type echoDriver struct{}

func (e *echoDriver) Name() string { return "echo" }

func (e *echoDriver) Init(ctx context.Context, ch *adapter.Client, conv *Conversation) error {
	_, _ = ch.ExecuteCommand(ctx, "go", []string{"run", "./cmd/frameworkecho", "reset"}, "framework-echo")
	today := time.Now().Format("2006-01-02")
	clientName := conv.Title
	if clientName == "" {
		clientName = "anonimo"
	}
	resp, err := ch.ExecuteCommand(ctx, "go", []string{
		"run", "./cmd/frameworkecho", "init",
		"--project-id", conv.ID,
		"--client", clientName,
		"--date", today,
	}, "framework-echo")
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("echo init: %s", resp.Error)
	}
	return nil
}

func (e *echoDriver) IngestAnswer(ctx context.Context, ch *adapter.Client, conv *Conversation, qctx QueuedAnswerCtx) error {
	args := []string{
		"run", "./cmd/frameworkecho", "ingest-answer",
		"--question-id", qctx.ExternalID,
		"--answer", qctx.Answer,
	}
	resp, err := ch.ExecuteCommand(ctx, "go", args, "framework-echo")
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("echo ingest-answer: %s", resp.Error)
	}
	return nil
}

func (e *echoDriver) PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (string, string, string, bool) {
	resp, err := ch.ExecuteCommand(ctx, "go", []string{"run", "./cmd/frameworkecho", "next-question"}, "framework-echo")
	if err != nil || !resp.Success {
		return "", "", "", false
	}
	r, ok := parseNextQuestion(resp.Stdout)
	if !ok {
		return "", "", "", false
	}
	if alreadyAsked[r.ID] {
		return "", "", "", false
	}
	return r.Text, r.ID, r.AskVia, true
}

// ---------------------------------------------------------------------------
// Alfa driver
// ---------------------------------------------------------------------------

type alfaDriver struct{}

func (a *alfaDriver) Name() string { return "alfa" }

func (a *alfaDriver) Init(ctx context.Context, ch *adapter.Client, conv *Conversation) error {
	return nil
}

// alfaSpecPath construye el path del spec por conversación.
func alfaSpecPath(conv *Conversation) (relPath, absPath string) {
	relPath = "framework-alfa/temp/alfa_spec_api_" + conv.ID + ".json"
	absPath = "/Users/alcless_a1234_cursor/remora-go/" + relPath
	return
}

func (a *alfaDriver) IngestAnswer(ctx context.Context, ch *adapter.Client, conv *Conversation, qctx QueuedAnswerCtx) error {
	if qctx.ExternalID == "" {
		return nil
	}
	_, specAbs := alfaSpecPath(conv)
	resp, err := ch.ExecuteCommand(ctx, "go", []string{
		"run", "./cmd/frameworkalfa", "ingest-answer",
		"--spec", specAbs,
		"--question-id", qctx.ExternalID,
		"--answer", qctx.Answer,
	}, "framework-alfa")
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("alfa ingest-answer: %s", resp.Error)
	}
	return nil
}

func (a *alfaDriver) PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (string, string, string, bool) {
	_, specAbs := alfaSpecPath(conv)
	echoTreeAbs := "/Users/alcless_a1234_cursor/remora-go/framework-echo/frameworkecho.json"
	// Compilar/recompilar draft cada vez para reflejar avances de Echo.
	_, _ = ch.ExecuteCommand(ctx, "go", []string{
		"run", "./cmd/frameworkalfa", "compile",
		"--echo-tree", echoTreeAbs,
		"--out", specAbs,
		"--allow-draft=true",
	}, "framework-alfa")
	resp, err := ch.ExecuteCommand(ctx, "go", []string{
		"run", "./cmd/frameworkalfa", "next-question",
		"--spec", specAbs,
		"--echo-tree", echoTreeAbs,
	}, "framework-alfa")
	if err != nil || !resp.Success {
		return "", "", "", false
	}
	r, ok := parseNextQuestion(resp.Stdout)
	if !ok {
		return "", "", "", false
	}
	if alreadyAsked[r.ID] {
		return "", "", "", false
	}
	askVia := r.AskVia
	if askVia == "" {
		askVia = "echo"
	}
	return r.Text, r.ID, askVia, true
}

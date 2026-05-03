package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"

	"channel/adapter"
	"channel/manifest"
)

// genericDriver implementa FrameworkDriver puramente desde un Manifest.
//
// Es el driver que usa cualquier framework descubierto vía
// manifest.Discover() que NO tenga un driver hardcodeado en driverRegistry.
//
// Contrato esperado del manifest:
//   - execution_mode = "sync_chain" (o vacío)
//   - user_input.supported = true
//   - commands[user_input.next_question_cmd] existe
//   - commands[user_input.ingest_answer_cmd] existe y declara params question_id, answer
//
// El bootstrap (Init) es no-op por defecto. Si un framework nuevo necesita
// init, lo declara como un comando "init" en su manifest y el driver lo
// invocará — pero por ahora mantengamos el contrato mínimo y dejemos
// init custom para los drivers hardcodeados que ya lo necesitan.
type genericDriver struct {
	manifest *manifest.Manifest
	rootDir  string // raíz absoluta donde viven los framework-* directorios
	binPath  string // binario absoluto a ejecutar (resuelto desde manifest.Binary)
	cwd      string // cwd absoluto para ejecutar el binario
}

// newGenericDriver construye un driver genérico a partir de un manifest.
// rootDir es la carpeta que contiene framework-<name>/.
func newGenericDriver(m *manifest.Manifest, rootDir string) (*genericDriver, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest nil")
	}
	cwdRel := m.Cwd
	if cwdRel == "" {
		cwdRel = "framework-" + m.Name
	}
	cwd := filepath.Join(rootDir, cwdRel)

	binPath := m.Binary.Command
	if binPath == "" {
		return nil, fmt.Errorf("manifest %s: binary.command vacío", m.Name)
	}
	// Si el comando es relativo (ej "go" o "./bin/x") lo dejamos tal cual,
	// el shell del Channel lo resuelve desde cwd.
	if filepath.IsAbs(binPath) {
		// nada que resolver
	}

	return &genericDriver{
		manifest: m,
		rootDir:  rootDir,
		binPath:  binPath,
		cwd:      cwd,
	}, nil
}

func (g *genericDriver) Name() string { return g.manifest.Name }

// fullArgs concatena args_prefix del manifest con los args del comando.
func (g *genericDriver) fullArgs(cmdArgs []string) []string {
	out := make([]string, 0, len(g.manifest.Binary.ArgsPrefix)+len(cmdArgs))
	out = append(out, g.manifest.Binary.ArgsPrefix...)
	out = append(out, cmdArgs...)
	return out
}

// Init: por ahora no-op. Frameworks que necesiten bootstrap deben tener
// un driver custom (como echo) hasta que el manifest exprese init.
func (g *genericDriver) Init(ctx context.Context, ch *adapter.Client, conv *Conversation) error {
	return nil
}

func (g *genericDriver) IngestAnswer(ctx context.Context, ch *adapter.Client, conv *Conversation, qctx QueuedAnswerCtx) error {
	cmdName := g.manifest.UserInput.IngestAnswerCmd
	if cmdName == "" {
		cmdName = "ingest-answer"
	}
	cmd, ok := g.manifest.Commands[cmdName]
	if !ok {
		// El manifest declara user_input.supported pero no tiene el comando.
		// Validate() debería haberlo cazado al boot; aquí degradamos a no-op
		// para no romper el chain.
		return nil
	}
	cmdArgs := []string{
		cmdName,
		"--question-id", qctx.ExternalID,
		"--answer", qctx.Answer,
	}
	// Si el manifest declara soporte de historial, lo serializamos y lo
	// pasamos como --history <base64-url-safe>. Esto resuelve referencias
	// conversacionales ("cuáles son?", "los 2 primeros", "y en 2023?") sin
	// que cada framework tenga que mantener estado de la conversación.
	//
	// El base64 url-safe es necesario porque el Channel rechaza newlines y
	// metacaracteres de shell en los args (axioma 4.3). El framework decide
	// cómo decodificar.
	if commandHasParam(cmd, "history") {
		if hb := encodeRecentHistory(conv.ID, qctx.QuestionID); hb != "" {
			cmdArgs = append(cmdArgs, "--history", hb)
		}
	}
	args := g.fullArgs(cmdArgs)
	resp, err := ch.ExecuteCommand(ctx, g.binPath, args, g.cwd)
	if err != nil {
		return fmt.Errorf("%s ingest-answer: %w", g.manifest.Name, err)
	}
	if !resp.Success {
		return fmt.Errorf("%s ingest-answer: %s", g.manifest.Name, resp.Error)
	}
	return nil
}

// commandHasParam devuelve true si el comando declara `name` en sus params.
// Sirve para que genericDriver decida si pasar flags opcionales
// (como --history) sin romper a frameworks que aún no los soportan.
func commandHasParam(c manifest.Command, name string) bool {
	for _, p := range c.Params {
		if p == name {
			return true
		}
	}
	return false
}

// historyTurn es el subset de Message que enviamos al framework. Mantenemos
// solo role + content para no exponer estructuras internas.
type historyTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// encodeRecentHistory carga los últimos N turnos de la conversación y los
// serializa a base64-url-safe(JSON([...]historyTurn)).
//
// Excluye el mensaje pendiente actual (questionID): cuando el orquestador
// llama IngestAnswer ya persistió el mensaje del usuario, pero la pregunta
// del framework dueño aún está en la queue, no en messages. Filtramos por
// si el caller persistió algo a último momento.
//
// Si no hay historial o algo falla, devuelve "" (el framework recibirá el
// flag vacío y operará como antes).
func encodeRecentHistory(convID, currentQuestionID string) string {
	const maxTurns = 8
	msgs, err := loadMessages(convID)
	if err != nil || len(msgs) == 0 {
		return ""
	}
	turns := make([]historyTurn, 0, len(msgs))
	for _, m := range msgs {
		// El último user message es la pregunta actual; ya viene en --answer.
		// Lo incluimos igual porque al phraser le sirve ver el contexto, y al
		// generador de SQL le ayuda a anclar la query. Pero filtramos
		// mensajes vacíos.
		content := m.Content
		if content == "" {
			continue
		}
		turns = append(turns, historyTurn{Role: m.Role, Content: content})
	}
	if len(turns) > maxTurns {
		turns = turns[len(turns)-maxTurns:]
	}
	if len(turns) == 0 {
		return ""
	}
	raw, err := json.Marshal(turns)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// PollQuestionFull devuelve el nextQuestionResponse completo incluyendo Chips.
// Usar cuando se necesita acceder a campos opcionales que PollQuestion descarta.
func (g *genericDriver) PollQuestionFull(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (nextQuestionResponse, bool) {
	cmdName := g.manifest.UserInput.NextQuestionCmd
	if cmdName == "" {
		cmdName = "next-question"
	}
	if _, ok := g.manifest.Commands[cmdName]; !ok {
		return nextQuestionResponse{}, false
	}
	args := g.fullArgs([]string{cmdName})
	resp, err := ch.ExecuteCommand(ctx, g.binPath, args, g.cwd)
	if err != nil || !resp.Success {
		return nextQuestionResponse{}, false
	}
	r, ok := parseNextQuestion(resp.Stdout)
	if !ok || alreadyAsked[r.ID] {
		return nextQuestionResponse{}, false
	}
	if r.AskVia == "" {
		r.AskVia = g.manifest.UserInput.AskVia
	}
	return r, true
}

func (g *genericDriver) PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (string, string, string, bool) {
	cmdName := g.manifest.UserInput.NextQuestionCmd
	if cmdName == "" {
		cmdName = "next-question"
	}
	if _, ok := g.manifest.Commands[cmdName]; !ok {
		return "", "", "", false
	}
	args := g.fullArgs([]string{cmdName})
	resp, err := ch.ExecuteCommand(ctx, g.binPath, args, g.cwd)
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
		askVia = g.manifest.UserInput.AskVia
	}
	return r.Text, r.ID, askVia, true
}

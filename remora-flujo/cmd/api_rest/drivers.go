package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"channel/adapter"
	"channel/manifest"
)

// keysOfManifests devuelve las llaves de un map[string]*manifest.Manifest
// ordenadas (solo para logs).
func keysOfManifests(m map[string]*manifest.Manifest) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

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
	PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (text, reasoning, externalID, askVia string, ok bool)
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

// driverRegistry es la fuente de verdad de qué frameworks puede correr el
// orquestador. Se inicializa al boot vía initDriverRegistry. Los drivers
// hardcodeados (echo, alfa) tienen lógica especial de bootstrap; cualquier
// otro framework con manifest válido entra automáticamente vía genericDriver.
var driverRegistry = map[string]FrameworkDriver{}

// hardcodedDrivers son los drivers con lógica custom que NO deben ser
// reemplazados por un genericDriver aunque tengan manifest válido.
var hardcodedDrivers = map[string]FrameworkDriver{
	"echo": &echoDriver{},
	"alfa": &alfaDriver{},
}

// initDriverRegistry escanea rootDir buscando framework-*/framework.manifest.json,
// valida cada uno, y construye el driverRegistry. Los frameworks listados en
// hardcodedDrivers se registran tal cual (su manifest se usa solo para
// metadata, no para construir el driver). Los demás obtienen un genericDriver.
//
// Devuelve los manifests cargados (válidos e inválidos por separado) para
// que el caller pueda loguear o exponer en /frameworks.
func initDriverRegistry(rootDir string, logger *log.Logger) (loaded map[string]*manifest.Manifest, skipped map[string]error) {
	loaded = map[string]*manifest.Manifest{}
	skipped = map[string]error{}

	// DEBUG: listar rootDir y chequear manifests esperados
	logger.Printf("DEBUG discover rootDir=%q", rootDir)
	if entries, err := os.ReadDir(rootDir); err == nil {
		names := []string{}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "framework-") {
				manPath := filepath.Join(rootDir, e.Name(), "framework.manifest.json")
				st, serr := os.Stat(manPath)
				names = append(names, fmt.Sprintf("%s{dir=%v,manifest_exists=%v,stat_err=%v,size=%d}",
					e.Name(), e.IsDir(), serr == nil, serr, func() int64 {
						if st != nil {
							return st.Size()
						}
						return -1
					}()))
			}
		}
		logger.Printf("DEBUG rootDir framework-* entries: %v", names)
	} else {
		logger.Printf("DEBUG ReadDir(%s) error: %v", rootDir, err)
	}

	manifests, derrs := manifest.Discover(rootDir)
	logger.Printf("DEBUG manifest.Discover returned %d manifests: %v", len(manifests), keysOfManifests(manifests))
	for _, e := range derrs {
		logger.Printf("manifest discover warn: %v", e)
	}

	// 1. Drivers hardcodeados: siempre presentes, manifest opcional.
	for name, drv := range hardcodedDrivers {
		driverRegistry[name] = drv
		if m, ok := manifests[name]; ok {
			if err := m.Validate(); err != nil {
				logger.Printf("manifest %s inválido (driver hardcoded): %v", name, err)
			}
			loaded[name] = m
		}
	}

	// 2. Manifests descubiertos sin driver hardcoded → genericDriver.
	for name, m := range manifests {
		if _, isHardcoded := hardcodedDrivers[name]; isHardcoded {
			continue
		}
		if err := m.Validate(); err != nil {
			skipped[name] = err
			logger.Printf("manifest %s skip: %v", name, err)
			continue
		}
		// Solo frameworks sync_chain entran al driverRegistry. Los async_trigger
		// se invocan fuera del round-robin (no implementan next-question).
		if m.EffectiveExecutionMode() != manifest.ExecutionModeSync {
			loaded[name] = m
			logger.Printf("manifest %s descubierto (execution_mode=%s, fuera del chain)", name, m.EffectiveExecutionMode())
			continue
		}
		drv, err := newGenericDriver(m, rootDir, nil)
		if err != nil {
			skipped[name] = err
			logger.Printf("manifest %s skip (driver build): %v", name, err)
			continue
		}
		driverRegistry[name] = drv
		loaded[name] = m
		logger.Printf("framework registrado vía genericDriver: %s", name)
	}

	logger.Printf("discovery completo: %d frameworks activos en el chain (%v), %d manifests cargados, %d omitidos",
		len(driverRegistry), keysOf(driverRegistry), len(loaded), len(skipped))
	return loaded, skipped
}

// keysOf devuelve las llaves de un map ordenadas, para logs reproducibles.
func keysOf(m map[string]FrameworkDriver) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Orden estable.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
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
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Reasoning string   `json:"reasoning,omitempty"`
	AskVia    string   `json:"ask_via"`
	Chips     []string `json:"chips,omitempty"`
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
	root := resolveRemoraRoot()
	cwd := filepath.Join(root, "framework-echo")
	bin, argsPrefix := runtimeCommand(cwd, "REMORA_ECHO_BIN", "frameworkecho", []string{"run", "./cmd/frameworkecho"})
	_, _ = ch.ExecuteCommand(ctx, bin, append(argsPrefix, "reset"), cwd)
	today := time.Now().Format("2006-01-02")
	clientName := conv.Title
	if clientName == "" {
		clientName = "anonimo"
	}
	resp, err := ch.ExecuteCommand(ctx, bin, append(argsPrefix,
		"init",
		"--project-id", conv.ID,
		"--client", clientName,
		"--date", today,
	), cwd)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("echo init: %s", resp.Error)
	}
	return nil
}

func (e *echoDriver) IngestAnswer(ctx context.Context, ch *adapter.Client, conv *Conversation, qctx QueuedAnswerCtx) error {
	root := resolveRemoraRoot()
	cwd := filepath.Join(root, "framework-echo")
	bin, argsPrefix := runtimeCommand(cwd, "REMORA_ECHO_BIN", "frameworkecho", []string{"run", "./cmd/frameworkecho"})
	args := append(argsPrefix,
		"ingest-answer",
		"--question-id", qctx.ExternalID,
		"--answer", qctx.Answer,
	)
	resp, err := ch.ExecuteCommand(ctx, bin, args, cwd)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("echo ingest-answer: %s", resp.Error)
	}
	return nil
}

func (e *echoDriver) PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (string, string, string, string, bool) {
	root := resolveRemoraRoot()
	cwd := filepath.Join(root, "framework-echo")
	bin, argsPrefix := runtimeCommand(cwd, "REMORA_ECHO_BIN", "frameworkecho", []string{"run", "./cmd/frameworkecho"})
	resp, err := ch.ExecuteCommand(ctx, bin, append(argsPrefix, "next-question"), cwd)
	if err != nil || !resp.Success {
		return "", "", "", "", false
	}
	r, ok := parseNextQuestion(resp.Stdout)
	if !ok {
		return "", "", "", "", false
	}
	if alreadyAsked[r.ID] {
		return "", "", "", "", false
	}
	return r.Text, r.Reasoning, r.ID, r.AskVia, true
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
	absPath = filepath.Join(resolveRemoraRoot(), relPath)
	return
}

func (a *alfaDriver) IngestAnswer(ctx context.Context, ch *adapter.Client, conv *Conversation, qctx QueuedAnswerCtx) error {
	if qctx.ExternalID == "" {
		return nil
	}
	_, specAbs := alfaSpecPath(conv)
	root := resolveRemoraRoot()
	cwd := filepath.Join(root, "framework-alfa")
	bin, argsPrefix := runtimeCommand(cwd, "REMORA_ALFA_BIN", "frameworkalfa", []string{"run", "./cmd/frameworkalfa"})
	args := append(argsPrefix,
		"ingest-answer",
		"--spec", specAbs,
		"--question-id", qctx.ExternalID,
		"--answer", qctx.Answer,
	)
	resp, err := ch.ExecuteCommand(ctx, bin, args, cwd)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("alfa ingest-answer: %s", resp.Error)
	}
	return nil
}

func (a *alfaDriver) PollQuestion(ctx context.Context, ch *adapter.Client, conv *Conversation, alreadyAsked map[string]bool) (string, string, string, string, bool) {
	_, specAbs := alfaSpecPath(conv)
	root := resolveRemoraRoot()
	echoTreeAbs := filepath.Join(root, "framework-echo", "frameworkecho.json")
	cwd := filepath.Join(root, "framework-alfa")
	bin, argsPrefix := runtimeCommand(cwd, "REMORA_ALFA_BIN", "frameworkalfa", []string{"run", "./cmd/frameworkalfa"})
	// Compilar/recompilar draft cada vez para reflejar avances de Echo.
	_, _ = ch.ExecuteCommand(ctx, bin, append(argsPrefix,
		"compile",
		"--echo-tree", echoTreeAbs,
		"--out", specAbs,
		"--allow-draft=true",
	), cwd)
	resp, err := ch.ExecuteCommand(ctx, bin, append(argsPrefix,
		"next-question",
		"--spec", specAbs,
		"--echo-tree", echoTreeAbs,
	), cwd)
	if err != nil || !resp.Success {
		return "", "", "", "", false
	}
	r, ok := parseNextQuestion(resp.Stdout)
	if !ok {
		return "", "", "", "", false
	}
	if alreadyAsked[r.ID] {
		return "", "", "", "", false
	}
	askVia := r.AskVia
	if askVia == "" {
		askVia = "echo"
	}
	return r.Text, r.Reasoning, r.ID, askVia, true
}

func runtimeCommand(cwd, envName, builtName string, goArgs []string) (string, []string) {
	if override := os.Getenv(envName); override != "" {
		if override != "go" {
			return override, nil
		}
		return override, append([]string{}, goArgs...)
	}
	if _, err := os.Stat(filepath.Join(cwd, builtName)); err == nil {
		return "./" + builtName, nil
	}
	return "go", append([]string{}, goArgs...)
}

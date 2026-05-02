// Package manifest define el contrato declarativo que cada framework expone
// para que el orquestador pueda usarlo sin código específico por framework.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest es el contrato declarativo de un framework.
type Manifest struct {
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Build       BuildSpec          `json:"build"`
	Binary      BinarySpec         `json:"binary"`
	Cwd         string             `json:"cwd"`
	Inputs      []IOPort           `json:"inputs"`
	Outputs     []IOPort           `json:"outputs"`
	Commands    map[string]Command `json:"commands"`
	AsksHuman   AsksHuman          `json:"asks_human"`
	UserInput   UserInputSpec      `json:"user_input"`
	Model       ModelSpec          `json:"model"`

	// ExecutionMode declara cómo el orquestador ejecuta el framework.
	//   - "sync_chain"    (default) participa del round-robin conversacional
	//                     vía next-question / ingest-answer.
	//   - "async_trigger" se invoca fuera del chain (cron, webhook, CLI).
	//                     No participa en next-question. Útil para ingesta,
	//                     indexación, batch jobs.
	ExecutionMode string `json:"execution_mode,omitempty"`

	// CapabilitiesSemantic permite que un planner LLM o un feasibility check
	// razonen sobre qué framework usar para una tarea sin tener código
	// específico por framework. Todos los campos son opcionales.
	CapabilitiesSemantic CapabilitiesSemantic `json:"capabilities_semantic,omitempty"`

	// RequiresInfra declara dependencias de infraestructura que el orquestador
	// debe proveer vía env vars. Ej: ["postgres+pgvector"].
	RequiresInfra []string `json:"requires_infra,omitempty"`
}

// CapabilitiesSemantic describe el rol del framework en términos
// machine-readable para un planner. No reemplaza a la descripción humana.
type CapabilitiesSemantic struct {
	// Tags: etiquetas libres, ej ["rag","data-expert","discovery"].
	Tags []string `json:"tags,omitempty"`
	// IntentExamples: ejemplos en lenguaje natural de qué pide el usuario
	// cuando este framework es la opción correcta. Sirve al planner.
	IntentExamples []string `json:"intent_examples,omitempty"`
	// Produces: artefactos lógicos que el framework genera. Pueden tener
	// el formato "key" o "key:<scope>" (ej "data_indexed:<schema_id>").
	Produces []string `json:"produces,omitempty"`
	// Requires: artefactos que el framework necesita encontrar disponibles.
	// El feasibility check empareja Produces de unos con Requires de otros.
	Requires []string `json:"requires,omitempty"`
}

// ExecutionModeSync identifica frameworks que participan del round-robin.
const ExecutionModeSync = "sync_chain"

// ExecutionModeAsync identifica frameworks invocados fuera del chain.
const ExecutionModeAsync = "async_trigger"

// EffectiveExecutionMode devuelve el modo declarado o el default sync_chain.
func (m *Manifest) EffectiveExecutionMode() string {
	if m.ExecutionMode == "" {
		return ExecutionModeSync
	}
	return m.ExecutionMode
}

// ModelSpec declara qué modelo de IA usa el framework cuando el orquestador
// le pre-procesa input no-textual (imágenes, audio, etc) o post-procesa una
// respuesta. El framework en sí mismo NO llama al modelo: lo hace la API.
//
// Capabilities:
//   - "text"        modelo de texto plano (default)
//   - "multimodal"  acepta imágenes en input
//   - "vision"      especializado en visión
//
// EnvKey: nombre de la variable de entorno donde la API toma el API key.
type ModelSpec struct {
	Provider     string   `json:"provider"`     // "groq", "minimax", "openai", ...
	Name         string   `json:"name"`         // p.ej "meta-llama/llama-4-scout-17b-16e-instruct"
	EnvKey       string   `json:"env_key"`      // p.ej "GROQ_API_KEY"
	Capabilities []string `json:"capabilities"` // ["text"], ["text","multimodal"]
	BaseURL      string   `json:"base_url,omitempty"`
}

// UserInputSpec declara cómo el framework interactúa con el usuario humano.
// Es la pieza estandarizada que la API REST consume para decidir cómo enrutar
// preguntas y respuestas. Si Supported=false el framework no necesita input
// directo del usuario (ej: un compilador puro).
type UserInputSpec struct {
	Supported bool     `json:"supported"`
	// AskVia "" = pregunta directa al usuario; "echo" = otro framework
	// (típicamente echo) reformula la pregunta antes de mostrarla.
	AskVia    string   `json:"ask_via,omitempty"`
	// Modes: tipos de input aceptados, ej "short_answer", "resource_upload".
	Modes     []string `json:"modes,omitempty"`
	// NextQuestionCmd: nombre del comando declarado en Commands que devuelve
	// la próxima pregunta pendiente. Convención: stdout es JSON
	//   {"id":"...","text":"...","ask_via":""}  (vacío si no hay pregunta).
	NextQuestionCmd string `json:"next_question_cmd,omitempty"`
	// IngestAnswerCmd: comando que recibe la respuesta del usuario.
	// Convención: acepta --question-id y --answer.
	IngestAnswerCmd string `json:"ingest_answer_cmd,omitempty"`
}

// BuildSpec dice cómo compilar el framework.
type BuildSpec struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// BinarySpec dice cómo invocar el binario (o `go run`) del framework.
type BinarySpec struct {
	Command    string   `json:"command"`     // "go" o "./frameworkX"
	ArgsPrefix []string `json:"args_prefix"` // ["run", "./cmd/frameworkX"] o []
}

// IOPort describe un input o output del framework.
type IOPort struct {
	Name        string `json:"name"`
	Format      string `json:"format"`   // identificador semántico, ej "echo.tree.v1"
	Path        string `json:"path"`     // path relativo a BaseDir (puede usar templates)
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// Command describe una acción que el framework ofrece.
type Command struct {
	Description string            `json:"description"`
	Args        []string          `json:"args"`     // template, ej "{params.title}", "{inputs.echo_tree}"
	Params      []string          `json:"params"`   // nombres de params requeridos
	Defaults    map[string]string `json:"defaults"` // valores default por param
}

// AsksHuman declara cómo el framework pide input al humano.
type AsksHuman struct {
	Via         string `json:"via"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// Load lee un manifest desde disco.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: parse %s: %w", path, err)
	}
	return &m, nil
}

// ResolveArgs sustituye placeholders {params.X}, {inputs.Y}, {outputs.Z} en los args.
//   - params: valores provistos por el caller
//   - inputs: paths absolutos de los inputs (resuelto por el caller)
//   - outputs: paths absolutos de los outputs (resuelto por el caller)
func (c Command) ResolveArgs(params, inputs, outputs map[string]string) ([]string, error) {
	merged := make(map[string]string, len(c.Defaults)+len(params))
	for k, v := range c.Defaults {
		merged[k] = v
	}
	for k, v := range params {
		merged[k] = v
	}
	for _, p := range c.Params {
		if _, ok := merged[p]; !ok {
			return nil, fmt.Errorf("missing param: %s", p)
		}
	}

	out := make([]string, 0, len(c.Args))
	for _, a := range c.Args {
		resolved, err := substitute(a, merged, inputs, outputs)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

func substitute(s string, params, inputs, outputs map[string]string) (string, error) {
	// Sustitución simple: {params.X}, {inputs.X}, {outputs.X}
	for {
		open := strings.Index(s, "{")
		if open < 0 {
			break
		}
		close := strings.Index(s[open:], "}")
		if close < 0 {
			break
		}
		token := s[open+1 : open+close]
		var val string
		switch {
		case strings.HasPrefix(token, "params."):
			key := strings.TrimPrefix(token, "params.")
			v, ok := params[key]
			if !ok {
				return "", fmt.Errorf("unknown param: %s", key)
			}
			val = v
		case strings.HasPrefix(token, "inputs."):
			key := strings.TrimPrefix(token, "inputs.")
			v, ok := inputs[key]
			if !ok {
				return "", fmt.Errorf("unknown input: %s", key)
			}
			val = v
		case strings.HasPrefix(token, "outputs."):
			key := strings.TrimPrefix(token, "outputs.")
			v, ok := outputs[key]
			if !ok {
				return "", fmt.Errorf("unknown output: %s", key)
			}
			val = v
		default:
			return "", fmt.Errorf("unknown token: {%s}", token)
		}
		s = s[:open] + val + s[open+close+1:]
	}
	return s, nil
}

// FindOutput busca un output por nombre.
func (m *Manifest) FindOutput(name string) (IOPort, bool) {
	for _, o := range m.Outputs {
		if o.Name == name {
			return o, true
		}
	}
	return IOPort{}, false
}

// FindInput busca un input por nombre.
func (m *Manifest) FindInput(name string) (IOPort, bool) {
	for _, i := range m.Inputs {
		if i.Name == name {
			return i, true
		}
	}
	return IOPort{}, false
}

// CanChain devuelve los pares (out, in) compatibles entre m (productor) y next (consumidor).
func (m *Manifest) CanChain(next *Manifest) []ChainLink {
	var links []ChainLink
	for _, out := range m.Outputs {
		for _, in := range next.Inputs {
			if out.Format == in.Format {
				links = append(links, ChainLink{
					From:   m.Name,
					Output: out.Name,
					To:     next.Name,
					Input:  in.Name,
					Format: out.Format,
				})
			}
		}
	}
	return links
}

// ChainLink describe una conexión válida output→input entre frameworks.
type ChainLink struct {
	From   string `json:"from"`
	Output string `json:"output"`
	To     string `json:"to"`
	Input  string `json:"input"`
	Format string `json:"format"`
}

// Discover escanea rootDir y devuelve todos los manifests encontrados en
// subdirectorios cuyo nombre matchea "framework-*". Devuelve un mapa por
// Name del manifest. Si un archivo está corrupto, lo registra y sigue.
func Discover(rootDir string) (map[string]*Manifest, []error) {
	out := map[string]*Manifest{}
	var errs []error
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return out, []error{fmt.Errorf("manifest discover: %w", err)}
	}
	// Orden determinístico para tests reproducibles.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), "framework-") {
			continue
		}
		path := filepath.Join(rootDir, e.Name(), "framework.manifest.json")
		if _, err := os.Stat(path); err != nil {
			// Sin manifest = framework no estandarizado todavía. No es error fatal.
			continue
		}
		m, err := Load(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		if m.Name == "" {
			errs = append(errs, fmt.Errorf("%s: manifest sin name", e.Name()))
			continue
		}
		out[m.Name] = m
	}
	return out, errs
}

// Validate revisa invariantes mínimos del manifest. Devuelve error con
// todos los problemas concatenados, o nil si está OK. Es la puerta única
// que Quine y el orquestador deben usar para considerar un manifest válido.
func (m *Manifest) Validate() error {
	var problems []string

	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, "name vacío")
	}
	if strings.TrimSpace(m.Version) == "" {
		problems = append(problems, "version vacío")
	}
	if m.Binary.Command == "" {
		problems = append(problems, "binary.command vacío")
	}

	mode := m.EffectiveExecutionMode()
	switch mode {
	case ExecutionModeSync, ExecutionModeAsync:
	default:
		problems = append(problems, fmt.Sprintf("execution_mode inválido: %q (esperado %q o %q)", mode, ExecutionModeSync, ExecutionModeAsync))
	}

	// Si participa del chain conversacional con UserInput.Supported,
	// debe declarar los dos comandos del contrato.
	if mode == ExecutionModeSync && m.UserInput.Supported {
		if m.UserInput.NextQuestionCmd == "" {
			problems = append(problems, "user_input.next_question_cmd vacío (requerido cuando supported=true)")
		} else if _, ok := m.Commands[m.UserInput.NextQuestionCmd]; !ok {
			problems = append(problems, fmt.Sprintf("user_input.next_question_cmd %q no existe en commands", m.UserInput.NextQuestionCmd))
		}
		if m.UserInput.IngestAnswerCmd == "" {
			problems = append(problems, "user_input.ingest_answer_cmd vacío (requerido cuando supported=true)")
		} else if _, ok := m.Commands[m.UserInput.IngestAnswerCmd]; !ok {
			problems = append(problems, fmt.Sprintf("user_input.ingest_answer_cmd %q no existe en commands", m.UserInput.IngestAnswerCmd))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("manifest %s inválido: %s", m.Name, strings.Join(problems, "; "))
}

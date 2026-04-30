// Package manifest define el contrato declarativo que cada framework expone
// para que el orquestador pueda usarlo sin código específico por framework.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
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

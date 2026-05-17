package internal

import (
	"encoding/json"
	"os"
)

const StateFile = "inspector_state.json"

// Pasos de la conversación
const (
	StepIntro    = 0 // "¿A qué API querés conectarte?"
	StepURL      = 1 // "¿Cuál es la URL base?" (con docs de Exa)
	StepAuth     = 2 // "¿Necesitás autenticación?"
	StepTesting  = 3 // Testeo HTTP en curso
	StepName     = 4 // "¿Cómo la llamamos?"
	StepDone     = 5 // Conexión lista
)

type State struct {
	Step       int         `json:"step"`
	APIName    string      `json:"api_name"`
	BaseURL    string      `json:"base_url"`
	AuthToken  string      `json:"auth_token"`
	AuthHeader string      `json:"auth_header"` // default: "Authorization"
	ConnName   string      `json:"conn_name"`
	TestResult *TestResult `json:"test_result,omitempty"`
	ExaDocs    []ExaResult `json:"exa_docs,omitempty"`
	Done       bool        `json:"done"`
	Error      string      `json:"error,omitempty"`
}

type TestResult struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	BodySnippet string            `json:"body_snippet"`
	FullDump    string            `json:"full_dump,omitempty"`
	Diagnosis   string            `json:"diagnosis"`
	Success     bool              `json:"success"`
	LatencyMS   int64             `json:"latency_ms"`
	ErrorMsg    string            `json:"error_msg,omitempty"`
}

type ExaResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{Step: StepIntro}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{Step: StepIntro}, nil
	}
	return &s, nil
}

func SaveState(path string, s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

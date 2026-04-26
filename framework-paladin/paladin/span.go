package paladin

// Span representa un segmento de ejecución (función o bloque).
type Span struct {
	Name       string          `json:"name"`
	File       string          `json:"file,omitempty"`
	Line       int             `json:"line,omitempty"`
	StartNs    int64           `json:"start_ns"`
	DurationMs int64           `json:"duration_ms"`
	Vars       map[string]any  `json:"vars,omitempty"`
	Children   []*Span         `json:"children,omitempty"`
	Errors     []string        `json:"errors,omitempty"`
	Decisions  []Decision      `json:"decisions,omitempty"`
	Semantic   []SemanticEvent `json:"semantic,omitempty"`
}

type Decision struct {
	What string `json:"what"`
	Why  string `json:"why"`
}

// SemanticEvent records business meaning, not implementation detail.
//
// Paladin's source of truth is the structured semantic event emitted by Go code.
// AI, humans, or CLI explainers can translate these events, but they should not
// invent business semantics from raw vars alone.
type SemanticEvent struct {
	Kind     string         `json:"kind"`
	Subject  string         `json:"subject,omitempty"`
	Summary  string         `json:"summary"`
	Expected string         `json:"expected,omitempty"`
	Actual   string         `json:"actual,omitempty"`
	Passed   *bool          `json:"passed,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

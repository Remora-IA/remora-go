package paladin

// Span representa un segmento de ejecución (función o bloque).
type Span struct {
	Name       string         `json:"name"`
	File       string         `json:"file,omitempty"`
	Line       int            `json:"line,omitempty"`
	StartNs    int64          `json:"start_ns"`
	DurationMs int64          `json:"duration_ms"`
	Vars       map[string]any `json:"vars,omitempty"`
	Children   []*Span        `json:"children,omitempty"`
	Errors     []string       `json:"errors,omitempty"`
	Decisions  []Decision     `json:"decisions,omitempty"`
}

type Decision struct {
	What string `json:"what"`
	Why  string `json:"why"`
}

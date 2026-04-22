package flowguard

// Span representa una función o bloque de ejecución dentro del trace.
// Forma un árbol: cada span puede tener hijos (Children).
type Span struct {
	Name       string            `json:"name"`
	File       string            `json:"file"`
	Line       int               `json:"line"`
	StartNs    int64             `json:"start_ns"`
	DurationMs int64             `json:"duration_ms"`
	Vars       map[string]any    `json:"vars,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
	Decisions  []Decision        `json:"decisions,omitempty"`
	Bottleneck bool              `json:"bottleneck,omitempty"`
	Children   []*Span           `json:"children,omitempty"`
}

// Decision registra una decisión lógica tomada dentro de una función.
// Esto permite que la IA entienda POR QUÉ se tomó un camino y no otro.
type Decision struct {
	What string `json:"what"`
	Why  string `json:"why"`
}

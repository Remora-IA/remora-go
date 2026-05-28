package frameworkgmail

// Spec define la estructura del framework.
type Spec struct {
	Name        string
	Role        string
	Description string
	CreatedAt   string
}

// Result representa el resultado de una operacion.
type Result struct {
	Success   bool
	Data      interface{}
	Error     string
	Timestamp string
}

package internal

import "time"

// Response es el contrato de respuesta fijo (Axioma 3)
// Exactamente 6 campos en este orden exacto, sin excepción.
type Response struct {
	Success   bool   `json:"success"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error"`
	DurationMs int64  `json:"duration_ms"`
}

// NewSuccessResponse crea una respuesta de éxito (Axioma 3)
func NewSuccessResponse(stdout, stderr string, exitCode int, duration time.Duration) Response {
	return Response{
		Success:    true,
		ExitCode:   exitCode,
		Stdout:     stdout,
		Stderr:     stderr,
		Error:      "",
		DurationMs: duration.Milliseconds(),
	}
}

// NewErrorResponse crea una respuesta de error (Axioma 3, 9)
// Siempre HTTP 200 con success:false
func NewErrorResponse(errMsg string, duration time.Duration) Response {
	return Response{
		Success:    false,
		ExitCode:   -1,
		Stdout:     "",
		Stderr:     "",
		Error:      errMsg,
		DurationMs: duration.Milliseconds(),
	}
}

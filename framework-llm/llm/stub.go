package llm

import "context"

// StubClient devuelve respuestas pre-cargadas. Útil para tests y para
// que un MVP corra sin gastar tokens cuando no hay API key.
//
// Si el consumidor provee menos respuestas que turnos asistente que se
// soliciten, retorna FallbackText (default: "[stub] sin más respuestas").
type StubClient struct {
	Responses    []string
	FallbackText string
}

// NewStub crea un stub con respuestas pre-cargadas en orden.
func NewStub(responses ...string) *StubClient {
	return &StubClient{Responses: responses}
}

// Complete devuelve la siguiente respuesta indexada por el número de
// turnos de assistant en la historia. Esto hace que el stub sea idempotente
// frente a pausa/resume: si la conversación se persiste y se retoma, el
// turno se infiere del estado, no de un contador en memoria.
func (s *StubClient) Complete(_ context.Context, req Request) (*Response, error) {
	turn := 0
	for _, m := range req.Messages {
		if m.Role == "assistant" {
			turn++
		}
	}
	if turn < len(s.Responses) {
		return &Response{Text: s.Responses[turn], StopReason: "end_turn"}, nil
	}
	fallback := s.FallbackText
	if fallback == "" {
		fallback = "[stub] sin más respuestas"
	}
	return &Response{Text: fallback, StopReason: "end_turn"}, nil
}

// NewClientOrStub intenta crear un AnthropicClient real; si no hay
// credenciales, devuelve un stub con las respuestas dadas. Es el helper
// para MVPs: producción usa la API, desarrollo usa el stub, sin código
// condicional en el consumidor.
func NewClientOrStub(stubResponses ...string) Client {
	if c, err := NewAnthropic(); err == nil {
		return c
	}
	return NewStub(stubResponses...)
}

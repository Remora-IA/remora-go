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

// NewClientOrStub intenta crear un cliente real en orden de preferencia:
//  1. AnthropicClient si ANTHROPIC_API_KEY está seteada (más rápido, producción).
//  2. ClaudeCLIClient si el binario `claude` está en PATH (usa Max plan OAuth,
//     ideal para dev local sin costo marginal por mensaje).
//  3. StubClient con las respuestas dadas (sin costo, determinístico).
//
// Es el helper para MVPs: el consumidor escribe el mismo código y el helper
// decide qué backend usar según lo que esté disponible.
func NewClientOrStub(stubResponses ...string) Client {
	if c, err := NewAnthropic(); err == nil {
		return c
	}
	if c, err := NewClaudeCLI(); err == nil {
		return c
	}
	return NewStub(stubResponses...)
}

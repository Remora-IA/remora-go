// Package agent provee la primitiva de Remora para agentes conversacionales
// con estado, traza Paladin y LLM compartido.
//
// El developer escribe un Behavior; la runtime maneja history, span,
// llamada al LLM, y detección de estados terminales.
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/remora-go/framework-llm/llm"
	"github.com/remora-go/framework-paladin/paladin"
)

// State es el estado mutable de un agente. Opaco al runtime; el Behavior
// lo interpreta. Usar map[string]any permite que cualquier struct se
// pueda persistir/restaurar sin reflexión.
type State map[string]any

// Outcome es un estado terminal. Status conviene mantenerlo en un set
// pequeño y conocido por consumidores ("agreed", "escalated", "abandoned").
type Outcome struct {
	Status string
	Reason string
	Data   State
}

// Decision es lo que un hook del Behavior le dice a la runtime.
type Decision struct {
	// Update aplica mutación sobre el state. Si nil, no toca state.
	Update func(State)
	// ShortCircuitReply: si no es "", la runtime usa este texto en vez de
	// llamar al LLM (válido solo en OnInput). En OnReply este campo se ignora.
	ShortCircuitReply string
	// Outcome marca terminal. Si no nil, la conversación termina.
	Outcome *Outcome
	// Traces son decisiones que la runtime registra en Paladin.
	Traces []Trace
}

// Trace es una entrada en el span Paladin: decision o event.
type Trace struct {
	What string
	Why  string
}

// Behavior es lo que el developer implementa. Encapsula la lógica de
// negocio sin tener que reescribir el turn loop, paladin spans, ni llm call.
type Behavior interface {
	// Name del actor para Paladin ("Carolina", "Synthesizer", etc).
	Name() string
	// Responsibility breve para Paladin.
	Responsibility() string
	// Goal del agente (constante o derivado del state).
	Goal(state State) string
	// SystemPrompt para el LLM, derivado del state actual.
	SystemPrompt(state State) string
	// OnInput se ejecuta antes de llamar al LLM. Puede short-circuit
	// (ej: detectar hostilidad y escalar sin pagar tokens).
	OnInput(state State, input string) Decision
	// OnReply se ejecuta después del LLM. Update state, detectar acuerdos.
	OnReply(state State, input, reply string) Decision
}

// Agent es la runtime que envuelve un Behavior.
type Agent struct {
	llm      llm.Client
	behavior Behavior
	history  []llm.Message
	state    State
	outcome  *Outcome
	turn     int
}

// New crea un agente con state inicial. state puede ser nil.
func New(behavior Behavior, llmClient llm.Client, state State) *Agent {
	if state == nil {
		state = State{}
	}
	return &Agent{
		llm:      llmClient,
		behavior: behavior,
		state:    state,
	}
}

// Turn procesa un mensaje del usuario y devuelve la respuesta del agente.
// Si input es "", el agente genera el turno inicial (saludo, etc).
// Después de un turno terminal, retorna el último reply sin avanzar.
func (a *Agent) Turn(parent *paladin.Context, input string) (string, error) {
	if a.outcome != nil {
		return "", fmt.Errorf("agent: turn after terminal outcome %q", a.outcome.Status)
	}

	a.turn++
	ctx := parent.Child(fmt.Sprintf("turno_%d", a.turn))
	defer ctx.End()

	ctx.Actor(a.behavior.Name(), a.behavior.Responsibility())
	ctx.Goal(a.behavior.Goal(a.state))
	for k, v := range a.state {
		ctx.Var(k, v)
	}

	if input != "" {
		a.history = append(a.history, llm.Message{Role: "user", Content: input})
		ctx.Event("input_recibido", trunc(input, 80), nil)

		pre := a.behavior.OnInput(a.state, input)
		applyDecision(ctx, &a.state, pre)
		if pre.Outcome != nil {
			a.outcome = pre.Outcome
			reply := pre.ShortCircuitReply
			if reply != "" {
				a.history = append(a.history, llm.Message{Role: "assistant", Content: reply})
				ctx.Event("reply_emitido", trunc(reply, 80), nil)
			}
			return reply, nil
		}
		if pre.ShortCircuitReply != "" {
			a.history = append(a.history, llm.Message{Role: "assistant", Content: pre.ShortCircuitReply})
			ctx.Event("reply_emitido", trunc(pre.ShortCircuitReply, 80), nil)
			return pre.ShortCircuitReply, nil
		}
	}

	system := a.behavior.SystemPrompt(a.state)
	ctx.Decision("invocar_llm", fmt.Sprintf("turno %d, history %d msgs", a.turn, len(a.history)))

	resp, err := a.llm.Complete(context.Background(), llm.Request{
		System:   system,
		Messages: a.history,
	})
	if err != nil {
		ctx.Error(err)
		return "", err
	}
	reply := resp.Text
	a.history = append(a.history, llm.Message{Role: "assistant", Content: reply})
	ctx.Event("reply_emitido", trunc(reply, 80), nil)

	post := a.behavior.OnReply(a.state, input, reply)
	applyDecision(ctx, &a.state, post)
	if post.Outcome != nil {
		a.outcome = post.Outcome
	}

	return reply, nil
}

// Done devuelve el Outcome si el agente terminó, nil si sigue activo.
func (a *Agent) Done() *Outcome { return a.outcome }

// Snapshot captura el estado completo del agente para persistencia.
// Behavior NO se persiste — se vuelve a inyectar al restaurar.
type Snapshot struct {
	History []llm.Message
	State   State
	Outcome *Outcome
	Turn    int
}

// Snapshot devuelve una copia inmutable del estado actual.
func (a *Agent) Snapshot() *Snapshot {
	history := make([]llm.Message, len(a.history))
	copy(history, a.history)
	state := State{}
	for k, v := range a.state {
		state[k] = v
	}
	var outcome *Outcome
	if a.outcome != nil {
		oc := *a.outcome
		outcome = &oc
	}
	return &Snapshot{History: history, State: state, Outcome: outcome, Turn: a.turn}
}

// Restore crea un agente desde un Snapshot. El Behavior y el LLM se
// inyectan; el state/history/outcome viene del snapshot.
func Restore(behavior Behavior, llmClient llm.Client, snap *Snapshot) *Agent {
	a := &Agent{
		llm:      llmClient,
		behavior: behavior,
		history:  snap.History,
		state:    snap.State,
		turn:     snap.Turn,
	}
	if snap.Outcome != nil {
		oc := *snap.Outcome
		a.outcome = &oc
	}
	return a
}

// State devuelve el state actual (referencia mutable — el caller no
// debería modificar fuera del Behavior, pero no se enforce).
func (a *Agent) State() State { return a.state }

// History devuelve la historia completa de mensajes.
func (a *Agent) History() []llm.Message { return a.history }

func applyDecision(ctx *paladin.Context, state *State, d Decision) {
	if d.Update != nil {
		d.Update(*state)
	}
	for _, t := range d.Traces {
		ctx.Decision(t.What, t.Why)
	}
	if d.Outcome != nil {
		ctx.Decision("outcome_"+d.Outcome.Status, d.Outcome.Reason)
	}
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

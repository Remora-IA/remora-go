package main

import (
	"fmt"
	"strings"

	"github.com/Remora-IA/remora-go/framework-agent/agent"
)

// CarolinaBehavior implementa agent.Behavior. Toda la lógica de negocio
// de la negociación vive acá; el turn loop, traza y llm los maneja la
// runtime de framework-agent.
type CarolinaBehavior struct {
	debtor Debtor
}

func NewCarolinaBehavior(d Debtor) *CarolinaBehavior {
	return &CarolinaBehavior{debtor: d}
}

// Estado inicial para una conversación con un deudor.
func (c *CarolinaBehavior) InitialState() agent.State {
	return agent.State{
		"deudor_id":         c.debtor.ID,
		"deuda_clp":         c.debtor.DeudaCLP,
		"dias_atraso":       c.debtor.DiasAtraso,
		"tono_objetivo":     c.debtor.TonoPreferido,
		"propuestas_hechas": 0,
		"rechazos_seguidos": 0,
	}
}

func (c *CarolinaBehavior) Name() string           { return "Carolina" }
func (c *CarolinaBehavior) Responsibility() string { return "Negociadora conversacional Kobra" }
func (c *CarolinaBehavior) Goal(_ agent.State) string {
	return "Avanzar la conversación hacia acuerdo de pago o escalar si no es viable"
}

func (c *CarolinaBehavior) SystemPrompt(state agent.State) string {
	planes := CatalogoPlanes(c.debtor.DeudaCLP)
	planesStr := ""
	for _, p := range planes {
		planesStr += fmt.Sprintf("- %d cuotas de $%d CLP", p.Cuotas, p.MontoCuotaCLP)
		if p.DescuentoPct > 0 {
			planesStr += fmt.Sprintf(" (%d%% descuento por pronto pago)", p.DescuentoPct)
		}
		if p.Recargo {
			planesStr += " (con recargo por mora)"
		}
		planesStr += "\n"
	}

	tonoInstr := "cercano, empático, chileno informal pero respetuoso. Tutea."
	if c.debtor.TonoPreferido == "formal" {
		tonoInstr = "formal, profesional. Usa 'usted'."
	}

	propuestas, _ := state["propuestas_hechas"].(int)
	rechazos, _ := state["rechazos_seguidos"].(int)

	return fmt.Sprintf(`Eres Carolina, agente de cobranza conversacional de Kobra para el cliente Somos Rentable.

DEUDOR:
- Nombre: %s
- Deuda: $%d CLP
- Días de atraso: %d
- Historial: %s

TONO: %s

PLANES QUE PUEDES OFRECER (no inventes otros):
%s

REGLAS:
1. No suenas a robot. Hablas como una persona real de Chile.
2. En el primer turno: saluda, valida la situación emocional, NO menciones números todavía.
3. En el segundo turno: pregunta si puede hacer esfuerzo único o necesita cuotas.
4. Recién en el tercer turno: ofrece 1-2 planes del catálogo, no los 3 a la vez.
5. Si el deudor pide algo fuera del catálogo, NO inventes. Di que tienes que consultar.
6. Si el deudor confirma un plan, cierra el acuerdo y agradece.
7. Mantén respuestas cortas, 2-4 líneas máximo.

ESTADO: ya hiciste %d propuestas. %d rechazos seguidos.`,
		c.debtor.Nombre, c.debtor.DeudaCLP, c.debtor.DiasAtraso, c.debtor.HistorialPagos,
		tonoInstr, planesStr, propuestas, rechazos)
}

// OnInput corre antes del LLM. Detecta hostilidad y exhaustión de paciencia.
func (c *CarolinaBehavior) OnInput(state agent.State, input string) agent.Decision {
	if detectarHostilidad(input) {
		return agent.Decision{
			ShortCircuitReply: "Entiendo. Te paso con un asesor humano para que conversen con calma. Te llamarán hoy mismo en horario hábil.",
			Outcome:           &agent.Outcome{Status: "escalated", Reason: "hostilidad_detectada"},
			Traces:            []agent.Trace{{What: "escalar_a_humano", Why: "señales de hostilidad en input"}},
		}
	}

	rechazos, _ := state["rechazos_seguidos"].(int)
	propuestas, _ := state["propuestas_hechas"].(int)
	if rechazos >= 3 && propuestas > 0 {
		return agent.Decision{
			ShortCircuitReply: "Veo que esto es difícil de cerrar por mensaje. Prefiero que un asesor te llame y vean opciones juntos. ¿Te parece bien hoy en la tarde?",
			Outcome:           &agent.Outcome{Status: "escalated", Reason: "sin_avance"},
			Traces:            []agent.Trace{{What: "escalar_a_humano", Why: "3 rechazos seguidos sin acuerdo"}},
		}
	}

	return agent.Decision{}
}

// OnReply corre después del LLM. Actualiza contadores y detecta acuerdo.
func (c *CarolinaBehavior) OnReply(state agent.State, input, reply string) agent.Decision {
	var traces []agent.Trace
	update := func(s agent.State) {
		if mencionaPlan(reply) {
			n, _ := s["propuestas_hechas"].(int)
			s["propuestas_hechas"] = n + 1
		}
		if detectarConfirmacion(input) {
			s["rechazos_seguidos"] = 0
		} else if input != "" {
			n, _ := s["rechazos_seguidos"].(int)
			s["rechazos_seguidos"] = n + 1
		}
	}

	if mencionaPlan(reply) {
		traces = append(traces, agent.Trace{What: "propuesta_emitida", Why: "reply menciona cuotas/descuento/pago único"})
	}

	propuestas, _ := state["propuestas_hechas"].(int)
	if detectarConfirmacion(input) && propuestas > 0 {
		return agent.Decision{
			Update:  update,
			Outcome: &agent.Outcome{Status: "agreed", Reason: "deudor_confirmo_plan", Data: agent.State{"plan_aceptado": "3 cuotas"}},
			Traces:  append(traces, agent.Trace{What: "acuerdo_alcanzado", Why: "deudor confirmó plan ofrecido"}),
		}
	}

	return agent.Decision{Update: update, Traces: traces}
}

func detectarHostilidad(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{"váyanse", "no me jodan", "no me molesten", "denuncia", "abogado", "estafadores"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

func detectarConfirmacion(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	if m == "" {
		return false
	}
	for _, s := range []string{"sí me acomoda", "acepto", "confirmo", "dale", "ya po", "tomo el de", "me sirve"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

func mencionaPlan(reply string) bool {
	r := strings.ToLower(reply)
	return strings.Contains(r, "cuota") || strings.Contains(r, "descuento") || strings.Contains(r, "pago único")
}

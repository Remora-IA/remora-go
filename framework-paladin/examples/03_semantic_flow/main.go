package main

import "github.com/Remora-IA/remora-go/framework-paladin/paladin"

func main() {
	trace := paladin.NewTrace("semantic-handoff-demo")
	ctx := trace.Start()
	defer trace.Flush()

	runFlow(ctx)
}

func runFlow(ctx *paladin.Context) {
	flow := ctx.Child("runFlow")
	defer flow.End()

	flow.Actor("echo", "descubre el proceso real del usuario")
	flow.Actor("alfa", "convierte discovery en flujo ideal verificable")
	flow.Goal("activar Alfa solo cuando existan 2 respuestas reales de usuario")

	flow.Rule("echo_to_alfa", "Alfa se activa cuando user_answers >= 2 o Echo declara readiness", nil)
	flow.Event("user_answered_echo", "usuario respondio una vez a Echo", map[string]any{"user_answers": 1})
	flow.Check("echo_to_alfa", "user_answers >= 2", "user_answers = 1", false)
	flow.Decision("activar_alfa", "turn_count calculado como 2")
	flow.Handoff("echo", "alfa", "turn_count calculado como 2")
	flow.Violation("echo_to_alfa", "mantener Echo hasta 2 respuestas reales", "Alfa fue activada con 1 respuesta real")
	flow.Expect("next_actor", "echo")
}

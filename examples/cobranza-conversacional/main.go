package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Remora-IA/remora-go/framework-agent/agent"
	"github.com/Remora-IA/remora-go/framework-channels/channels"
	"github.com/Remora-IA/remora-go/framework-channels/channels/console"
	"github.com/Remora-IA/remora-go/framework-paladin/paladin"
	"github.com/Remora-IA/remora-go/framework-store/store"
	filestore "github.com/Remora-IA/remora-go/framework-store/store/file"
)

func main() {
	trace := paladin.NewTrace("FinCrowdCarolina")
	defer trace.Flush()
	root := trace.Start()
	defer root.End()

	root.Actor("Cobranza-Conversacional", "Negociadora conversacional para FinCrowd")
	root.Goal("Acordar plan de pago con Patricia Morales o escalar a humano")
	root.Var("debtor_id", DebtorSeed.ID)
	root.Var("deuda_inicial_clp", DebtorSeed.DeudaCLP)
	root.Var("api_mode", apiMode())

	ctx := context.Background()

	// Permite escoger perfil con DEBTOR_PROFILE=roberto|marta|patricia
	debtor := DebtorSeed
	if profile := os.Getenv("DEBTOR_PROFILE"); profile != "" {
		if d, ok := DebtorScenarios[profile]; ok {
			debtor = d
		}
	}
	conversationID := debtor.ID

	st, err := filestore.New("./conversations")
	if err != nil {
		fail(err, 3)
	}

	behavior := NewCarolinaBehavior(debtor)

	// Si existe snapshot previo: restaurar. Si no: arrancar limpio.
	carolina, resumed := loadOrNew(ctx, st, conversationID, behavior)
	root.Var("conversation_resumed", resumed)

	ch := console.New("Patricia: ", "Carolina")
	defer ch.Close()

	fmt.Println("=== Cobranza-Conversacional MVP — simulación de WhatsApp ===")
	fmt.Printf("Deudor: %s | Deuda: $%d CLP | Atraso: %d días\n",
		debtor.Nombre, debtor.DeudaCLP, debtor.DiasAtraso)
	fmt.Printf("Modo API: %s | Canal: console | Conversación: %s%s\n",
		apiMode(), conversationID, resumedTag(resumed))
	fmt.Println("Escribe como el deudor. ENTER vacío termina la conversación.")
	fmt.Println(strings.Repeat("-", 60))

	// Si ya está terminada, no se reabre.
	if outcome := carolina.Done(); outcome != nil {
		fmt.Printf("\n[conversación %s ya cerrada — status: %s, razón: %s]\n",
			conversationID, outcome.Status, outcome.Reason)
		os.Exit(exitCode(outcome))
	}

	// Solo enviar saludo inicial si es conversación nueva.
	if !resumed {
		first, err := carolina.Turn(root, "")
		if err != nil {
			fail(err, 3)
		}
		_ = ch.Send(ctx, conversationID, first)
		_ = st.Save(ctx, conversationID, carolina.Snapshot())
	} else {
		fmt.Printf("\n[retomando conversación: %d turnos previos]\n", len(carolina.History())/2)
		if len(carolina.History()) > 0 {
			last := carolina.History()[len(carolina.History())-1]
			fmt.Printf("\n(último mensaje de Carolina: %q)\n", trunc(last.Content, 100))
		}
	}

	for carolina.Done() == nil {
		msg, err := ch.Receive(ctx)
		if errors.Is(err, channels.ErrClosed) {
			root.Decision("conversacion_pausada", "canal cerrado — snapshot persistido")
			_ = st.Save(ctx, conversationID, carolina.Snapshot())
			fmt.Println("\n[conversación pausada — snapshot guardado en ./conversations]")
			os.Exit(2)
		}
		if err != nil {
			fail(err, 3)
		}

		reply, err := carolina.Turn(root, msg.Text)
		if err != nil {
			fail(err, 3)
		}
		_ = ch.Send(ctx, msg.From, reply)
		_ = st.Save(ctx, conversationID, carolina.Snapshot())
	}

	outcome := carolina.Done()
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("RESULTADO: %s (razón: %s)\n", outcome.Status, outcome.Reason)
	root.Decision("cierre_"+outcome.Status, outcome.Reason)
	os.Exit(exitCode(outcome))
}

func loadOrNew(ctx context.Context, st store.Store, id string, b *CarolinaBehavior) (*agent.Agent, bool) {
	snap, err := st.Load(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return agent.New(b, LLMClient, b.InitialState()), false
	}
	if err != nil {
		fail(err, 3)
	}
	return agent.Restore(b, LLMClient, snap), true
}

func exitCode(o *agent.Outcome) int {
	switch o.Status {
	case "agreed":
		return 0
	case "escalated":
		return 1
	default:
		return 2
	}
}

func resumedTag(b bool) string {
	if b {
		return " (resumida)"
	}
	return " (nueva)"
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fail(err error, code int) {
	fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
	os.Exit(code)
}

func apiMode() string {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "live (Claude API)"
	}
	return "stub (sin API key — respuestas determinísticas)"
}

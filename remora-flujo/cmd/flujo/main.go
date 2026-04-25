package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"framework-paladin/paladin"
	"remora-flujo/handoff"
	"remora-flujo/nativeagent"
)

const statePath = "temp/handoff/state.json"

func main() {
	if len(os.Args) < 2 {
		cmdStatus()
		fmt.Println()
		fmt.Println("Para empezar: go run ./cmd/flujo run")
		return
	}

	switch os.Args[1] {
	case "status":
		cmdStatus()
	case "next":
		cmdNext()
	case "start":
		cmdStart(os.Args[2:])
	case "done":
		cmdDone(os.Args[2:])
	case "ask-echo":
		cmdAskEcho(os.Args[2:])
	case "reset":
		cmdReset(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	case "chat":
		cmdChat(os.Args[2:])
	case "reply":
		cmdReply(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fail(fmt.Errorf("comando desconocido: %s", os.Args[1]))
	}
}

func cmdStatus() {
	state := mustLoad()
	fmt.Println("Remora Flujo handoff")
	for _, role := range []handoff.Role{handoff.RoleEcho, handoff.RoleAlfa, handoff.RoleBravo} {
		rs := state.Roles[role]
		fmt.Printf("  %s: %s, on_runs=%d\n", role, rs.Status, rs.OnRuns)
	}
	if last, ok := state.LastEvent(); ok {
		fmt.Printf("  ultimo_evento: %s/%s %s\n", last.Role, last.Type, last.Message)
	} else {
		fmt.Println("  ultimo_evento: ninguno")
	}
	role, reason, runnable := state.NextRole()
	if runnable {
		fmt.Printf("  siguiente: %s (%s)\n", role, reason)
	} else {
		fmt.Printf("  siguiente: ninguno (%s)\n", reason)
	}
}

func cmdNext() {
	state := mustLoad()
	role, reason, runnable := state.NextRole()
	if !runnable {
		fmt.Printf("none %s\n", reason)
		return
	}
	fmt.Printf("%s %s\n", role, reason)
}

func cmdStart(args []string) {
	if len(args) < 1 {
		fail(fmt.Errorf("uso: flujo start <echo|alfa|bravo> [--prompt nombre]"))
	}
	roleArg := args[0]
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	prompt := fs.String("prompt", "", "descripcion corta del prompt usado")
	_ = fs.Parse(args[1:])
	role, err := handoff.ParseRole(roleArg)
	if err != nil {
		fail(err)
	}
	state := mustLoad()
	state.Start(role, *prompt)
	mustSave(state)
	fmt.Printf("on %s\n", role)
}

func cmdDone(args []string) {
	if len(args) < 1 {
		fail(fmt.Errorf("uso: flujo done <echo|alfa|bravo> --event <evento> [--message nota]"))
	}
	roleArg := args[0]
	fs := flag.NewFlagSet("done", flag.ExitOnError)
	eventValue := fs.String("event", "", "evento de salida")
	message := fs.String("message", "", "nota breve")
	_ = fs.Parse(args[1:])
	if *eventValue == "" {
		fail(fmt.Errorf("uso: flujo done <echo|alfa|bravo> --event <evento> [--message nota]"))
	}
	role, err := handoff.ParseRole(roleArg)
	if err != nil {
		fail(err)
	}
	event, err := handoff.ParseEvent(*eventValue)
	if err != nil {
		fail(err)
	}
	state := mustLoad()
	state.Done(role, event, *message)
	mustSave(state)
	fmt.Printf("off %s event=%s\n", role, event)
}

func cmdAskEcho(args []string) {
	fs := flag.NewFlagSet("ask-echo", flag.ExitOnError)
	fromValue := fs.String("from", "", "alfa|bravo")
	question := fs.String("question", "", "pregunta para que Echo formule al usuario")
	_ = fs.Parse(args)
	if *fromValue == "" || *question == "" {
		fail(fmt.Errorf("uso: flujo ask-echo --from <alfa|bravo> --question <texto>"))
	}
	role, err := handoff.ParseRole(*fromValue)
	if err != nil {
		fail(err)
	}
	event := handoff.EventAlfaNeedsEcho
	if role == handoff.RoleBravo {
		event = handoff.EventBravoNeedsEcho
	}
	state := mustLoad()
	state.Done(role, event, *question)
	mustSave(state)
	fmt.Printf("handoff echo question=%q\n", *question)
}

func cmdReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	all := fs.Bool("all", false, "borra handoff, sesiones y artefactos de Echo/Alfa/Bravo")
	_ = fs.Parse(args)

	_ = os.RemoveAll("temp/handoff")
	_ = os.RemoveAll("temp/sessions")
	if *all {
		root := repoRoot()
		removePaths(
			filepath.Join(root, "framework-echo", "frameworkecho.json"),
			filepath.Join(root, "framework-alfa", "temp"),
			filepath.Join(root, "framework-bravo", "temp"),
			filepath.Join(root, "remora-flujo", "temp"),
		)
	}
	mustSave(handoff.NewState())
	fmt.Println("reset_ok")
}

func cmdRun(args []string) {
	trace := paladin.NewTrace("remora-flujo.run")
	ctx := trace.Start()
	defer trace.Flush()
	defer ctx.End()

	fs := flag.NewFlagSet("run", flag.ExitOnError)
	once := fs.Bool("once", true, "ejecuta solo el siguiente rol")
	dryRun := fs.Bool("dry-run", false, "solo muestra que rol se encenderia")
	_ = fs.Parse(args)

	for {
		state := mustLoad()
		role, reason, runnable := state.NextRole()
		if !runnable {
			ctx.Decision("handoff_idle", reason)
			fmt.Printf("handoff_idle: %s\n", reason)
			return
		}
		ctx.Var("next_role", role)
		ctx.Var("next_reason", reason)
		fmt.Printf("handoff_next: %s (%s)\n", role, reason)
		if *dryRun {
			return
		}
		if role == handoff.RoleEcho {
			if err := chatEcho(ctx, reason); err != nil {
				ctx.Error(err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			return
		}
		if err := runRole(ctx, role, reason); err != nil {
			ctx.Error(err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		if *once {
			return
		}
	}
}

func cmdChat(args []string) {
	trace := paladin.NewTrace("remora-flujo.chat")
	ctx := trace.Start()
	defer trace.Flush()
	defer ctx.End()

	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	_ = fs.Parse(args)

	state := mustLoad()
	_, reason, _ := state.NextRole()
	if reason == "" {
		reason = "chat_manual"
	}
	if err := chatEcho(ctx, reason); err != nil {
		ctx.Error(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
}

func cmdReply(args []string) {
	trace := paladin.NewTrace("remora-flujo.reply")
	ctx := trace.Start()
	defer trace.Flush()
	defer ctx.End()

	if len(args) == 0 {
		fail(fmt.Errorf("uso: flujo reply \"respuesta del usuario\""))
	}
	message := strings.Join(args, " ")
	state := mustLoad()
	state.Start(handoff.RoleEcho, "respuesta_usuario")
	mustSave(state)

	prompt := fmt.Sprintf(`Respuesta nueva del usuario para Echo:

%s

Lee y actualiza el arbol en ../framework-echo/frameworkecho.json usando Framework Echo.

Si ya hay suficiente para Alfa, termina con:
go run ./cmd/flujo done echo --event echo_ready_for_alfa --message "discovery listo"

Si necesitas preguntar otra cosa al usuario, haz solo una pregunta clara y termina con:
go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta hecha"
`, message)

	if err := promptRole(ctx, handoff.RoleEcho, prompt); err != nil {
		ctx.Error(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
}

func chatEcho(parent *paladin.Context, reason string) error {
	ctx := parent.Child("chatEcho")
	defer ctx.End()
	ctx.Var("reason", reason)

	state := mustLoad()
	state.Start(handoff.RoleEcho, reason)
	mustSave(state)
	ctx.Decision("handoff_start", "echo")

	initialPrompt, err := buildPrompt(handoff.RoleEcho, reason)
	if err != nil {
		return err
	}
	if err := promptRole(ctx, handoff.RoleEcho, initialPrompt); err != nil {
		return err
	}

	reader := bufio.NewScanner(os.Stdin)
	for {
		if shouldLeaveEchoChat() {
			return nil
		}

		fmt.Print("\nTu > ")
		if !reader.Scan() {
			fmt.Println()
			return reader.Err()
		}
		text := strings.TrimSpace(reader.Text())
		if text == "" {
			ctx.Decision("empty_user_input", "ignored")
			continue
		}
		switch text {
		case "/salir", "/exit", "/quit":
			return nil
		case "/status":
			cmdStatus()
			continue
		}

		state := mustLoad()
		state.Start(handoff.RoleEcho, "respuesta_usuario")
		mustSave(state)
		ctx.Var("user_message_length", len(text))

		prompt := fmt.Sprintf(`El usuario respondio:

%s

Continua como Echo. Actualiza el arbol con Framework Echo cuando corresponda.

Si necesitas otra respuesta del usuario, haz una sola pregunta clara y ejecuta:
go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta hecha"

Si ya esta listo para Alfa, ejecuta:
go run ./cmd/flujo done echo --event echo_ready_for_alfa --message "discovery listo"
`, text)
		if err := promptRole(ctx, handoff.RoleEcho, prompt); err != nil {
			return err
		}
	}
}

func shouldLeaveEchoChat() bool {
	state := mustLoad()
	last, ok := state.LastEvent()
	if !ok {
		return false
	}
	switch last.Type {
	case handoff.EventEchoReadyForAlfa:
		fmt.Println("\n[handoff] Echo dejo listo el flujo para Alfa. Ejecuta: go run ./cmd/flujo run")
		return true
	case handoff.EventAlfaReadyBravo, handoff.EventBravoDone:
		return true
	default:
		return false
	}
}

func runRole(parent *paladin.Context, role handoff.Role, reason string) error {
	ctx := parent.Child("runRole")
	defer ctx.End()
	ctx.Var("role", role)
	ctx.Var("reason", reason)

	prompt, err := buildPrompt(role, reason)
	if err != nil {
		ctx.Error(err)
		return err
	}
	state := mustLoad()
	state.Start(role, reason)
	mustSave(state)

	fmt.Printf("%s pensando\n", roleTitle(role))
	return promptRole(ctx, role, prompt)
}

func promptRole(parent *paladin.Context, role handoff.Role, prompt string) error {
	ctx := parent.Child("promptRole")
	defer ctx.End()
	ctx.Var("role", role)
	ctx.Var("prompt_length", len(prompt))

	agent, err := nativeagent.New(nativeagent.Options{
		CWD:          "/Users/alcless_a1234_cursor/remora-go/remora-flujo",
		SessionPath:  filepath.Join("temp", "sessions", string(role), "native.json"),
		AllowedTools: allowedTools(role),
		Trace:        ctx,
	})
	if err != nil {
		ctx.Error(err)
		return err
	}
	resp, err := agent.Prompt(prompt)
	if err != nil {
		ctx.Error(err)
		return err
	}
	resp = strings.TrimSpace(resp)
	ctx.Var("response_length", len(resp))
	if resp != "" {
		fmt.Printf("\n%s:\n%s\n", roleTitle(role), resp)
	}
	return nil
}

func allowedTools(role handoff.Role) []string {
	switch role {
	case handoff.RoleEcho:
		return []string{"bash", "read_file", "list_files"}
	case handoff.RoleAlfa:
		return []string{"bash", "read_file", "list_files"}
	case handoff.RoleBravo:
		return []string{"bash", "read_file", "write_file", "list_files"}
	default:
		return []string{"bash", "read_file", "list_files"}
	}
}

func buildPrompt(role handoff.Role, reason string) (string, error) {
	root := repoRoot()
	var initialPath string
	switch role {
	case handoff.RoleEcho:
		initialPath = filepath.Join(root, "framework-echo", "INITIAL_PROMPT.md")
	case handoff.RoleAlfa:
		initialPath = filepath.Join(root, "framework-alfa", "INITIAL_PROMPT.md")
	case handoff.RoleBravo:
		initialPath = filepath.Join(root, "framework-bravo", "INITIAL_PROMPT.md")
	default:
		return "", fmt.Errorf("rol no soportado: %s", role)
	}
	initial, err := os.ReadFile(initialPath)
	if err != nil {
		return "", err
	}

	tools := fmt.Sprintf(`

HANDOFF 100%% CODIGO
Ruta repo: %s
Motivo de encendido: %s
Evento pendiente: %s

No pases contexto de otra IA como prompt. Lee artefactos persistidos:
- Echo: %s
- Alfa: %s
- Bravo: %s

Cuando termines, apagate con uno de estos comandos desde /Users/alcless_a1234_cursor/remora-go/remora-flujo:
- Echo listo para Alfa: go run ./cmd/flujo done echo --event echo_ready_for_alfa --message "resumen breve"
- Echo espera respuesta del usuario: go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta hecha"
- Alfa listo para Bravo: go run ./cmd/flujo done alfa --event alfa_ready_for_bravo --message "ideal_flow listo"
- Alfa necesita a Echo: go run ./cmd/flujo ask-echo --from alfa --question "pregunta concreta"
- Bravo termino: go run ./cmd/flujo done bravo --event bravo_done --message "resultado listo"
- Bravo necesita a Echo: go run ./cmd/flujo ask-echo --from bravo --question "pregunta concreta"

Tu respuesta visible debe ser breve. El trabajo importante debe quedar en los archivos del framework correspondiente.
%s
`, root, reason, pendingEventMessage(),
		filepath.Join(root, "framework-echo", "frameworkecho.json"),
		filepath.Join(root, "framework-alfa", "temp"),
		filepath.Join(root, "framework-bravo", "temp"),
		roleContract(role))

	return string(initial) + tools, nil
}

func roleContract(role handoff.Role) string {
	switch role {
	case handoff.RoleEcho:
		return `
CONTRATO ESTRICTO PARA ECHO
- Tu primera accion interna siempre debe ser usar bash para ejecutar:
  cd /Users/alcless_a1234_cursor/remora-go/framework-echo && ./frameworkecho status && ./frameworkecho show-tree && ./frameworkecho selected-opportunities && ./frameworkecho readiness && ./frameworkecho config
- Usa solo el CLI ./frameworkecho para modificar el arbol. No uses write_file sobre frameworkecho.json.
- No inventes respuestas del usuario. No ejecutes validate si el usuario no acaba de confirmar explicitamente ese punto.
- No crees PAIN, OPPORTUNITY ni select-opportunity en la misma vuelta inicial donde el usuario solo describio un problema general.
- Despues de una respuesta del usuario, avanza como maximo un nivel semantico real: AXIOM/THEORY/TASK/PAIN/OPPORTUNITY segun corresponda.
- Antes de OPPORTUNITY debe existir pain real confirmado. Antes de pasar a Alfa debe existir transporte de datos confirmado.
- No des opciones A/B de solucion temprano. Pregunta una sola cosa sobre comportamiento actual o hueco critico.
- Si haces una pregunta al usuario, termina ejecutando:
  cd /Users/alcless_a1234_cursor/remora-go/remora-flujo && go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta hecha"
- Si readiness recomienda pass_to_alfa o select_opportunity y ya ejecutaste lo necesario, termina ejecutando el evento echo_ready_for_alfa.
`
	case handoff.RoleAlfa:
		return `
CONTRATO ESTRICTO PARA ALFA
- Usa los CLIs de Framework Alfa. No inventes reglas de negocio.
- Si faltan datos de Echo, usa ask-echo con una pregunta concreta.
- No hables con el usuario directamente.
`
	case handoff.RoleBravo:
		return `
CONTRATO ESTRICTO PARA BRAVO
- Trabaja local-first.
- Implementa y verifica con evidencia.
- Si falta dato semantico, usa ask-echo con una pregunta concreta.
- No hables con el usuario directamente.
`
	default:
		return ""
	}
}

func pendingEventMessage() string {
	state, err := handoff.Load(statePath)
	if err != nil {
		return ""
	}
	last, ok := state.LastEvent()
	if !ok || strings.TrimSpace(last.Message) == "" {
		return "ninguno"
	}
	return fmt.Sprintf("%s/%s: %s", last.Role, last.Type, last.Message)
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if filepath.Base(wd) == "remora-flujo" {
		return filepath.Dir(wd)
	}
	return filepath.Dir(wd)
}

func roleTitle(role handoff.Role) string {
	switch role {
	case handoff.RoleEcho:
		return "Echo"
	case handoff.RoleAlfa:
		return "Alfa"
	case handoff.RoleBravo:
		return "Bravo"
	default:
		return string(role)
	}
}

func removePaths(paths ...string) {
	for _, path := range paths {
		_ = os.RemoveAll(path)
	}
}

func mustLoad() *handoff.State {
	state, err := handoff.Load(statePath)
	if err != nil {
		fail(err)
	}
	return state
}

func mustSave(state *handoff.State) {
	if err := handoff.Save(statePath, state); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func usage() {
	fmt.Println(`Remora Flujo - handoff por estado/eventos

USO:
  go run ./cmd/flujo status
  go run ./cmd/flujo next
  go run ./cmd/flujo run
  go run ./cmd/flujo chat
  go run ./cmd/flujo run --once --dry-run
  go run ./cmd/flujo reply "respuesta del usuario"
  go run ./cmd/flujo done <echo|alfa|bravo> --event <evento> [--message nota]
  go run ./cmd/flujo ask-echo --from <alfa|bravo> --question <texto>
  go run ./cmd/flujo reset [--all]
  go run ./cmd/agentrpc

EVENTOS:
  echo_ready_for_alfa, echo_waiting_user, alfa_ready_for_bravo,
  alfa_needs_echo, bravo_needs_echo, bravo_done, error`)
}

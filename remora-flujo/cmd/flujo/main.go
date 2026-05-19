package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/remora-go/framework-paladin/paladin"
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
	case "flow":
		cmdFlow(os.Args[2:])
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
	if provider, model, note, err := nativeagent.RuntimeInfo(); err == nil {
		fmt.Printf("  modelo_activo: %s / %s\n", provider, model)
		fmt.Printf("  modelo_razon: %s\n", note)
	} else {
		fmt.Printf("  modelo_activo: no_configurado (%s)\n", err)
	}
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
	if role == handoff.RoleEcho && event == handoff.EventEchoReadyForAlfa {
		ready, question := echoReadyForAlfa(nil)
		if !ready {
			if strings.TrimSpace(question) == "" {
				question = "Echo readiness indica que falta contexto antes de Alfa."
			}
			fail(fmt.Errorf("rechazado echo_ready_for_alfa: frameworkecho readiness=false; siguiente pregunta: %s", question))
		}
	}
	if role == handoff.RoleEcho && event == handoff.EventEchoWaitingUser && !containsQuestion(*message) {
		fail(fmt.Errorf("rechazado echo_waiting_user: --message debe contener la pregunta exacta que el usuario debe responder"))
	}
	// Manejar eventos de cola de preguntas
	if event == handoff.EventEchoUserAnswered {
		queue := mustLoadQueue()
		if queue != nil && queue.CurrentSpeaker == handoff.SpeakerAlfa {
			if qq, ok := queue.GetNextAlfaQuestion(); ok {
				queue.MarkQuestionAnswered(handoff.SpeakerAlfa, qq.ID, *message)
				mustSaveQueue(queue)
			}
		}
	}
	// Manejar evento alfa_asks_question: agregar pregunta a la cola
	if event == handoff.EventAlfaAsksQuestion && role == handoff.RoleAlfa {
		queue := mustLoadQueue()
		if queue == nil {
			queue = handoff.NewQuestionsQueue()
		}
		queue.SetSpeaker(handoff.SpeakerAlfa)
		queue.AddAlfaQuestion(*message)
		mustSaveQueue(queue)
		fmt.Printf("[cola] Alfa agregó pregunta: %s\n", *message)
	}
	state := mustLoad()
	state.Done(role, event, *message)
	mustSave(state)
	fmt.Printf("off %s event=%s\n", role, event)
}

func mustLoadQueue() *handoff.QuestionsQueue {
	queue, err := handoff.LoadQuestionsQueue("")
	if err != nil {
		return handoff.NewQuestionsQueue()
	}
	return queue
}

func mustSaveQueue(queue *handoff.QuestionsQueue) {
	if err := handoff.SaveQuestionsQueue("", queue); err != nil {
		fail(err)
	}
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
	if role != handoff.RoleAlfa && role != handoff.RoleBravo {
		fail(fmt.Errorf("ask-echo solo acepta --from alfa o --from bravo"))
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
	_ = os.RemoveAll("temp/questions_queue.json")
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

		// Manejar razones específicas de la cola de preguntas
		switch reason {
		case "alfa_pregunta", "respuesta_para_alfa":
			// Alfa necesita procesar respuesta del usuario
			if err := runRole(ctx, handoff.RoleAlfa, reason); err != nil {
				ctx.Error(err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			return

		case "echo_tiene_palabra", "echo_continua":
			// Alfa cedió, Echo tiene la palabra
			if err := chatEcho(ctx, reason); err != nil {
				ctx.Error(err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			return
		}

		// Manejo de Alfa
		if role == handoff.RoleAlfa {
			// Inicializar cola si no existe
			queue, err := handoff.LoadQuestionsQueue("")
			if err != nil || queue == nil || queue.CurrentSpeaker == "" {
				queue = handoff.NewQuestionsQueue()
				queue.SetSpeaker(handoff.SpeakerAlfa)
				handoff.SaveQuestionsQueue("", queue)
				fmt.Println("[cola] Inicializada para Alfa")
			}
			// Ejecutar Alfa
			if err := runRole(ctx, handoff.RoleAlfa, reason); err != nil {
				ctx.Error(err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			return
		}

		// Manejo de Echo
		if role == handoff.RoleEcho {
			if err := chatEcho(ctx, reason); err != nil {
				ctx.Error(err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			return
		}

		// Manejo genérico para Bravo u otros
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

func echoReadyForAlfa(parent *paladin.Context) (bool, string) {
	var ctx *paladin.Context
	if parent != nil {
		ctx = parent.Child("echoReadyForAlfa")
		defer ctx.End()
	}

	cmd := exec.Command("/bin/zsh", "-lc", "cd /Users/alcless_a1234_cursor/remora-go/framework-echo && ./frameworkecho readiness")
	output, err := cmd.CombinedOutput()
	text := string(output)
	if ctx != nil {
		ctx.Var("readiness_output", text)
	}
	if err != nil {
		if ctx != nil {
			ctx.Error(err)
		}
		return false, "No pude leer readiness de Echo"
	}
	ready, question := parseEchoReadiness(text)
	if ctx != nil {
		ctx.Decision("ready_for_alfa", fmt.Sprintf("%t", ready))
	}
	return ready, question
}

func parseEchoReadiness(text string) (bool, string) {
	if strings.Contains(text, "ready_for_alfa: true") {
		return true, ""
	}
	question := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "next_question:") {
			question = strings.TrimSpace(strings.TrimPrefix(line, "next_question:"))
			break
		}
	}
	return false, question
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
go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta exacta que debe responder el usuario"
`, message)

	if _, err := promptRole(ctx, handoff.RoleEcho, prompt); err != nil {
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
	ctx.Actor("echo", "descubre el proceso real del usuario")
	ctx.Actor("alfa", "convierte discovery en flujo ideal verificable")
	ctx.Goal("mantener Echo hasta que existan 2 respuestas reales o Echo declare readiness")
	ctx.Rule("echo_to_alfa", "Alfa se activa si Echo declara readiness o si hay al menos 2 respuestas reales de usuario", nil)
	ctx.Expect("next_actor", "echo")

	initialPrompt, err := buildPrompt(handoff.RoleEcho, reason)
	if err != nil {
		return err
	}
	resp, err := promptRole(ctx, handoff.RoleEcho, initialPrompt)
	if err != nil {
		return err
	}
	printEchoQuestionIfMissing(resp)

	reader := bufio.NewScanner(os.Stdin)
	for {
		if echoReadyToHandOff() {
			ctx.Decision("handoff_echo_ready_for_alfa", "estado echo_ready_for_alfa")
			ctx.Handoff("echo", "alfa", "Echo declaro echo_ready_for_alfa")
			ctx.Expect("next_actor", "alfa")
			return runRole(ctx, handoff.RoleAlfa, "echo_listo_para_alfa")
		}
		pendingRole, pending := pendingQuestionRole()

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

		if pending {
			state := mustLoad()
			state.Done(handoff.RoleEcho, handoff.EventEchoUserAnswered, text)
			mustSave(state)
			ctx.Decision("route_user_answer_to_role", string(pendingRole))
			ctx.Handoff("user", string(pendingRole), "respuesta a pregunta pendiente")
			ctx.Expect("next_actor", string(pendingRole))
			if err := runRole(ctx, pendingRole, "respuesta_usuario"); err != nil {
				return err
			}
			if printPendingUserQuestion(ctx) {
				continue
			}
			return nil
		}

		state := mustLoad()
		state.Start(handoff.RoleEcho, "respuesta_usuario")
		mustSave(state)
		ctx.Var("user_message_length", len(text))

		turnCount := countUserResponses()
		ctx.Var("turn_count", turnCount)
		ctx.Event("user_answered_echo", "usuario respondio a Echo", map[string]any{"user_answers": turnCount})
		ctx.Check("echo_to_alfa", "user_answers >= 2 OR echo_ready_for_alfa", fmt.Sprintf("user_answers = %d", turnCount), turnCount >= 2)
		if turnCount >= 2 {
			ctx.Decision("handoff_turn_count_for_alfa", fmt.Sprintf("turn_count=%d", turnCount))
			ctx.Handoff("echo", "alfa", "2 respuestas reales de usuario")
			ctx.Expect("next_actor", "alfa")
			state.Done(handoff.RoleEcho, handoff.EventEchoReadyForAlfa, "turn_count_auto")
			mustSave(state)
			if err := runRole(ctx, handoff.RoleAlfa, "turn_count_auto"); err != nil {
				return err
			}
			if printPendingUserQuestion(ctx) {
				continue
			}
			return nil
		}

		imagePaths, cleanedText, err := parseImageInput(text)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		prompt := promptForEchoReply(cleanedText, len(imagePaths))
		resp, err := promptRoleWithImages(ctx, handoff.RoleEcho, prompt, imagePaths)
		if err != nil {
			return err
		}
		printEchoQuestionIfMissing(resp)
	}
}

func echoReadyToHandOff() bool {
	state := mustLoad()
	last, ok := state.LastEvent()
	return ok && last.Type == handoff.EventEchoReadyForAlfa
}

func printPendingUserQuestion(ctx *paladin.Context) bool {
	state := mustLoad()
	last, ok := state.LastEvent()
	if !ok {
		return false
	}
	switch last.Type {
	case handoff.EventAlfaNeedsEcho, handoff.EventAlfaAsksQuestion, handoff.EventBravoNeedsEcho:
		question := strings.TrimSpace(last.Message)
		if question == "" {
			return false
		}
		ctx.Decision("pending_user_question", fmt.Sprintf("%s/%s", last.Role, last.Type))
		fmt.Printf("\n%s:\n%s\n", roleTitle(last.Role), question)
		return true
	default:
		return false
	}
}

func pendingQuestionRole() (handoff.Role, bool) {
	state := mustLoad()
	last, ok := state.LastEvent()
	if !ok {
		return "", false
	}
	switch last.Type {
	case handoff.EventAlfaNeedsEcho, handoff.EventAlfaAsksQuestion:
		return handoff.RoleAlfa, true
	case handoff.EventBravoNeedsEcho:
		return handoff.RoleBravo, true
	default:
		return "", false
	}
}

func printEchoQuestionIfMissing(response string) {
	state := mustLoad()
	last, ok := state.LastEvent()
	if !ok || last.Type != handoff.EventEchoWaitingUser {
		return
	}
	question := strings.TrimSpace(last.Message)
	if !containsQuestion(question) || containsNormalized(response, question) {
		return
	}
	fmt.Printf("\nEcho:\n%s\n", question)
}

func countUserResponses() int {
	state, err := handoff.Load(statePath)
	if err != nil {
		return 0
	}
	count := 0
	for _, event := range state.Events {
		if event.Role == handoff.RoleEcho && event.Type == handoff.EventStarted && event.Message == "respuesta_usuario" {
			count++
		}
	}
	return count
}

func shouldLeaveEchoChat() bool {
	state := mustLoad()
	last, ok := state.LastEvent()
	if !ok {
		return false
	}
	switch last.Type {
	case handoff.EventEchoReadyForAlfa:
		fmt.Println("\n[handoff] Echo dejó listo el flujo para Alfa. Ejecutando Alfa...")
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
	_, err = promptRole(ctx, role, prompt)
	return err
}

func promptRole(parent *paladin.Context, role handoff.Role, prompt string) (string, error) {
	return promptRoleWithImages(parent, role, prompt, nil)
}

func promptRoleWithImages(parent *paladin.Context, role handoff.Role, prompt string, imagePaths []string) (string, error) {
	ctx := parent.Child("promptRole")
	defer ctx.End()
	ctx.Var("role", role)
	ctx.Var("prompt_length", len(prompt))
	ctx.Var("image_count", len(imagePaths))

	agent, err := nativeagent.New(nativeagent.Options{
		CWD:          "/Users/alcless_a1234_cursor/remora-go/remora-flujo",
		Role:         string(role),
		SessionPath:  filepath.Join("temp", "sessions", string(role), "native.json"),
		AllowedTools: allowedTools(role),
		Trace:        ctx,
	})
	if err != nil {
		ctx.Error(err)
		return "", err
	}
	ctx.Var("llm_provider", agent.Provider())
	ctx.Var("llm_model", agent.Model())
	ctx.Var("llm_provider_reason", agent.ProviderNote())
	fmt.Printf("modelo_activo: %s / %s\n", agent.Provider(), agent.Model())
	fmt.Printf("modelo_razon: %s\n", agent.ProviderNote())
	images := make([]nativeagent.ImageInput, 0, len(imagePaths))
	for _, path := range imagePaths {
		images = append(images, nativeagent.ImageInput{Path: path})
	}
	resp, err := agent.PromptWithImages(prompt, images)
	if err != nil {
		ctx.Error(err)
		return "", err
	}
	resp = strings.TrimSpace(resp)
	ctx.Var("response_length", len(resp))
	if resp != "" {
		fmt.Printf("\n%s:\n%s\n", roleTitle(role), resp)
	}
	return resp, nil
}

func parseImageInput(text string) ([]string, string, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil, text, nil
	}
	command := fields[0]
	if command != "/imagen" && command != "/image" {
		return nil, text, nil
	}
	if len(fields) < 2 {
		return nil, "", fmt.Errorf("uso: /imagen <ruta_imagen> [texto]")
	}
	imagePath := fields[1]
	cleaned := strings.TrimSpace(strings.TrimPrefix(text, command))
	cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, imagePath))
	if cleaned == "" {
		cleaned = "Analiza la imagen adjunta y extrae lo relevante para Framework Echo."
	}
	return []string{imagePath}, cleaned, nil
}

func promptForEchoReply(message string, imageCount int) string {
	imageNote := ""
	if imageCount > 0 {
		imageNote = fmt.Sprintf("\nEl usuario adjunto %d imagen(es). Debes mirarlas directamente; no le pidas describir lo visible si puedes extraerlo de la imagen.\n", imageCount)
	}
	return fmt.Sprintf(`El usuario respondió:

%s
%s
Continúa como Echo. Actualiza el árbol con Framework Echo cuando corresponda.

Si necesitas otra respuesta del usuario, haz una sola pregunta clara y ejecuta:
go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta exacta que acabas de hacer"

Si ya está listo para Alfa, ejecuta:
go run ./cmd/flujo done echo --event echo_ready_for_alfa --message "discovery listo"
`, message, imageNote)
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

	// Cargar cola de preguntas para pasar contexto
	queue, _ := handoff.LoadQuestionsQueue("")
	queueNote := ""
	if queue != nil {
		if queue.CurrentSpeaker == handoff.SpeakerAlfa {
			if qq, ok := queue.GetNextAlfaQuestion(); ok {
				queueNote = fmt.Sprintf("\n\nCOLA: current_speaker=alfa, pregunta_pendiente=\"%s\"\n", qq.Text)
			}
		} else {
			queueNote = "\n\nCOLA: current_speaker=echo\n"
		}
	}

	tools := fmt.Sprintf(`

HANDOFF 100%% CODIGO
Ruta repo: %s
Motivo de encendido: %s
Evento pendiente: %s%s

No pases contexto de otra IA como prompt. Lee artefactos persistidos:
- Echo: %s
- Alfa: %s
- Bravo: %s

Cuando termines, apágate con uno de estos comandos desde /Users/alcless_a1234_cursor/remora-go/remora-flujo:
- Echo listo para Alfa: go run ./cmd/flujo done echo --event echo_ready_for_alfa --message "resumen breve"
- Echo espera respuesta del usuario: go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta exacta que debe responder el usuario"
- Echo usuario respondió (para Alfa): go run ./cmd/flujo done echo --event echo_user_answered --message "resumen respuesta"
- Alfa listo para Bravo: go run ./cmd/flujo done alfa --event alfa_ready_for_bravo --message "ideal_flow listo"
- Alfa cede a Echo: go run ./cmd/flujo done alfa --event alfa_ceded_to_echo --message "sin más preguntas"
- Alfa quiere hacer pregunta: go run ./cmd/flujo done alfa --event alfa_asks_question --message "tu pregunta"
- Alfa necesita a Echo: go run ./cmd/flujo ask-echo --from alfa --question "pregunta concreta"
- Bravo terminó: go run ./cmd/flujo done bravo --event bravo_done --message "resultado listo"
- Bravo necesita a Echo: go run ./cmd/flujo ask-echo --from bravo --question "pregunta concreta"

Tu respuesta visible debe ser breve. El trabajo importante debe quedar en los archivos del framework correspondiente.
%s
`, root, reason, pendingEventMessage(), queueNote,
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

=== MANEJO DE COLA DE PREGUNTAS ===
Hay un archivo temp/questions_queue.json que controla quien tiene la palabra:
- Si current_speaker = "alfa": TRANSMITE la pregunta de Alfa al usuario SIN reformularla
- Si current_speaker = "echo": Conversa naturalmente, puedes crear nodos básicos del árbol

=== TURN 1-3: ECHO NATURAL ===
- Tu primera acción interna siempre debe ser usar bash para ejecutar:
  cd /Users/alcless_a1234_cursor/remora-go/framework-echo && ./frameworkecho status && ./frameworkecho show-tree && ./frameworkecho selected-opportunities && ./frameworkecho readiness && ./frameworkecho config
- Usa solo el CLI ./frameworkecho para modificar el árbol. No uses write_file sobre frameworkecho.json.
- No inventes respuestas del usuario. No ejecutes validate si el usuario no acaba de confirmar explícitamente ese punto.
- No crees PAIN, OPPORTUNITY ni select-opportunity en la misma vuelta inicial donde el usuario solo describió un problema general.
- Después de una respuesta del usuario, avanza como máximo un nivel semántico real: AXIOM/THEORY/TASK/PAIN/OPPORTUNITY según corresponda.
- Antes de OPPORTUNITY debe existir pain real confirmado. Antes de pasar a Alfa debe existir transporte de datos confirmado.
- No des opciones A/B de solución temprano. Pregunta una sola cosa sobre comportamiento actual o hueco crítico.
- Si haces una pregunta al usuario, termina ejecutando:
  cd /Users/alcless_a1234_cursor/remora-go/remora-flujo && go run ./cmd/flujo done echo --event echo_waiting_user --message "pregunta exacta que debe responder el usuario"
- El --message de echo_waiting_user debe contener signos de pregunta y la pregunta completa, no una descripción como "pregunta enviada".
- Solo puedes ejecutar echo_ready_for_alfa cuando ./frameworkecho readiness diga literalmente: ready_for_alfa: true.
- Si readiness dice ready_for_alfa: false, no pases a Alfa aunque creas tener suficiente contexto; haz la pregunta indicada por next_question.

=== TURN 3+: ACTIVAR ALFA ===
Si ya conversaste 2-3 veces con el usuario y tienes contexto inicial:
- La cola de preguntas ya debe existir (creada por ti o por el sistema)
- Cuando current_speaker="alfa", transmites las preguntas de Alfa
- Cuando current_speaker="echo", puedes retomar tu flujo conversacional
`
	case handoff.RoleAlfa:
		return `
CONTRATO ESTRICTO PARA ALFA - MODO ITERATIVO

=== AL ACTIVARTE (por turn_3 o por echo_ready_for_alfa) ===
1. Lee el árbol de Echo: ../framework-echo/frameworkecho.json
2. Verifica/actualiza temp/questions_queue.json con current_speaker="alfa"
3. Analiza qué información falta para el MERE

=== MIENTRAS ACTIVO: SOLO 1 PREGUNTA A LA VEZ ===
- NO hagas múltiples preguntas en una vuelta
- Haz UNA pregunta concreta que reduzca la incertidumbre del MERE
- Después de que el usuario responda (via Echo), analiza la respuesta
- Decide:
  a) ¿Necesito OTRA pregunta específica? → Ejecuta: go run ./cmd/flujo done alfa --event alfa_asks_question --message "tu pregunta"
  b) ¿Puedo ceder a Echo temporalmente? → Ejecuta: go run ./cmd/flujo done alfa --event alfa_ceded_to_echo --message "sin más preguntas por ahora"
  c) ¿MERE completo y listo para Bravo? → Ejecuta: go run ./cmd/flujo done alfa --event alfa_ready_for_bravo --message "ideal_flow listo"

=== BUENA PREGUNTA ALFA ===
- Una pregunta que revele entidad, relación o regla de negocio
- Ejemplo: "¿Cómo sabe hoy cuál transferencia pertenece a qué cliente/factura?"
- Ejemplo: "¿El monto final se calcula o viene dado?"
- Ejemplo: "¿Qué pasa si no encuentra el rastro?"

=== IMPORTANTE ===
- No inventes MERE. Solo deduce de lo que Echo validó y lo que el usuario confirme
- Si el árbol de Echo tiene poca info, pregunta desde cero sobre el proceso
- Tu output en temp/alfa_spec.json debe ser un draft, puede estar incompleto
`
	case handoff.RoleBravo:
		return `
CONTRATO ESTRICTO PARA BRAVO
- Trabaja local-first.
- Implementa y verifica con evidencia.
- Si falta dato semántico, usa ask-echo con una pregunta concreta.
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

func containsQuestion(text string) bool {
	return strings.Contains(text, "?") || strings.Contains(text, "¿")
}

func containsNormalized(haystack, needle string) bool {
	return strings.Contains(normalizeForContains(haystack), normalizeForContains(needle))
}

func normalizeForContains(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
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
  go run ./cmd/flujo flow create --business <business_id> [--name <nombre>] [--description <texto>]
  go run ./cmd/flujo flow draft --business <business_id> --name <nombre> --description <texto> [--create]
  go run ./cmd/flujo flow compile --id <flow_id>
  go run ./cmd/flujo flow inspect --id <flow_id>
  go run ./cmd/flujo flow validate --id <flow_id>
  go run ./cmd/flujo flow simulate --id <flow_id> [--fixtures a,b] [--input texto]
  go run ./cmd/flujo flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]
  go run ./cmd/flujo flow install --id <flow_id> [--reconfigure]
  go run ./cmd/flujo flow replay --run <run_id>
  go run ./cmd/flujo flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
  go run ./cmd/flujo status
  go run ./cmd/flujo next
  go run ./cmd/flujo run
  go run ./cmd/flujo chat
  En chat: /imagen <ruta_imagen> [texto]
  go run ./cmd/flujo run --once --dry-run
  go run ./cmd/flujo reply "respuesta del usuario"
  go run ./cmd/flujo done <echo|alfa|bravo> --event <evento> [--message nota]
  go run ./cmd/flujo ask-echo --from <alfa|bravo> --question <texto>
  go run ./cmd/flujo reset [--all]
  go run ./cmd/agentrpc

EVENTOS:
  echo_ready_for_alfa, echo_waiting_user, echo_user_answered,
  alfa_ready_for_bravo, alfa_ceded_to_echo, alfa_asks_question,
  alfa_needs_echo, bravo_needs_echo, bravo_done, error`)
}

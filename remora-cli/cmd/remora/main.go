package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Puerto 8084 coincide con el default de api_rest en desarrollo local
// (ver scripts/dev-local.sh). Para prod, exportar REMORA_API_URL.
const apiBaseDefault = "http://localhost:8084/api/v1"

// ANSI escape codes
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
)

var frameworkColors = map[string]string{
	"arquitecto": cyan,
	"critico":    magenta,
	"paladin":    yellow,
	"echo":       green,
	"alfa":       blue,
}

func fwColor(name string) string {
	if c, ok := frameworkColors[name]; ok {
		return c
	}
	return white
}

const white = "\033[37m"

type Client struct {
	BaseURL string
	Token   string
}

func newClient() *Client {
	base := os.Getenv("REMORA_API_URL")
	if base == "" {
		base = apiBaseDefault
	}
	return &Client{
		BaseURL: strings.TrimSuffix(base, "/"),
		Token:   os.Getenv("REMORA_API_TOKEN"),
	}
}

func (c *Client) post(path string, body map[string]interface{}) (map[string]interface{}, error) {
	url := c.BaseURL + path
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("HTTP %d: %v", resp.StatusCode, out["error"])
	}
	return out, nil
}

// SSEEvent es un evento del stream del servidor.
type SSEEvent struct {
	Type string                 // "tool_start" | "tool_end" | "assistant_final" | ...
	Data map[string]interface{} // payload JSON
}

// streamChat envía un mensaje POST con Accept: text/event-stream y entrega
// cada evento al callback. Devuelve nil cuando llega "done".
func (c *Client) streamChat(path string, body map[string]interface{}, onEvent func(SSEEvent) bool) error {
	url := c.BaseURL + path
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := &http.Client{Timeout: 0} // sin timeout para streaming
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}

	// SSE parser simple: bloques separados por línea vacía. Cada bloque
	// tiene "event: X" + "data: Y" (data puede ser multiline pero acá es 1 línea).
	reader := bufio.NewReader(resp.Body)
	var eventType string
	var dataLines []string
	flush := func() bool {
		if len(dataLines) == 0 {
			eventType = ""
			return true
		}
		dataStr := strings.Join(dataLines, "\n")
		var data map[string]interface{}
		_ = json.Unmarshal([]byte(dataStr), &data)
		evt := SSEEvent{Type: eventType, Data: data}
		eventType = ""
		dataLines = nil
		if !onEvent(evt) {
			return false
		}
		return true
	}
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\n")
			line = strings.TrimRight(line, "\r")
			if line == "" {
				if !flush() {
					return nil
				}
			} else if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if err == io.EOF {
			flush()
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (c *Client) get(path string) (map[string]interface{}, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("HTTP %d: %v", resp.StatusCode, out["error"])
	}
	return out, nil
}

var sessionID string

func main() {
	// Auto-levantar backend si estamos en local y no está corriendo.
	ensureBackendRunning()

	// Sin args: modo pair-programming por defecto (arquitecto).
	// Equivalente a tau: el usuario escribe `remora` y ya está conversando.
	if len(os.Args) < 2 {
		handleCode()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "session":
		handleSession()
	case "send":
		handleSend()
	case "poll":
		handlePoll()
	case "answer":
		handleAnswer()
	case "invoke":
		handleInvoke()
	case "chat":
		handleChat()
	case "code", "c":
		handleCode()
	case "frameworks":
		handleFrameworks()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// handleCode es el shortcut "remora code" (o simplemente "remora") que
// arranca una sesión con el framework arquitecto — el asistente para
// entender y modificar el codebase. Equivalente a `remora chat --frameworks arquitecto`.
//
// Reusa sesión existente si hay .remora_session en el cwd; si no, crea una nueva.
func handleCode() {
	// Inyectar --frameworks arquitecto si no viene nada
	hasFrameworks := false
	hasSession := false
	for _, a := range os.Args[1:] {
		if a == "--frameworks" {
			hasFrameworks = true
		}
		if a == "--session" {
			hasSession = true
		}
	}
	// Si tenemos .remora_session pero no se pasó ni --session ni --frameworks,
	// dejamos que handleChat reuse la sesión existente. Si no hay session file,
	// inyectamos arquitecto como default.
	if !hasFrameworks && !hasSession {
		if _, err := os.Stat(".remora_session"); err != nil {
			os.Args = append(os.Args, "--frameworks", "arquitecto")
		}
	}
	handleChat()
}

func printUsage() {
	fmt.Print(`remora - Cliente terminal para la red RPC de remora-go

Uso rápido:
  remora                         ← modo pair-programming (arquitecto)
  remora code                    ← idem (alias explícito)
  remora c                       ← idem (alias corto)

Comandos:
  code | c [--frameworks ...] [--session <id>]
    Shortcut para conversar con el framework arquitecto (modelo mental del
    repo + tools read_file/list_dir/grep/find_files/query_symbols).
    Reusa .remora_session si existe, si no crea sesion nueva.

  chat [--frameworks <f1,f2,...>] [--session <id>]
    REPL generico. Crea sesion (si no existe) y conversa con los frameworks
    elegidos. Ellos preguntan, tu respondes. !exit para salir.

  session start --name <n> --frameworks <f1,f2,...>
    Crea una nueva conversacion con los frameworks especificados.

  send "<mensaje>"
    Envia un mensaje a la conversacion activa (no streaming).

  poll
    Pregunta al orquestador cual es la siguiente pregunta pendiente.

  answer --question-id <id> --text "<respuesta>"
    Responde una pregunta del framework activo.

  invoke <framework> --command <cmd> [--args ...]
    Invoca un framework async_trigger (ej: paladin audit) directamente.

  frameworks
    Lista frameworks descubiertos con sus capabilities.

Variables de entorno:
  REMORA_API_URL    URL base de api_rest (default: http://localhost:8084/api/v1)
  REMORA_API_TOKEN  Token de autenticacion (opcional)

Instalacion global:
  bash scripts/install-remora.sh
    Compila el CLI y lo linkea en ~/.local/bin/remora (agrega a PATH si falta).
`)
}

func handleSession() {
	if len(os.Args) < 3 || os.Args[2] != "start" {
		fmt.Fprintln(os.Stderr, "usage: session start --name <name> --frameworks <f1,f2,...>")
		os.Exit(1)
	}
	name := flagValue("--name")
	fwRaw := flagValue("--frameworks")
	if name == "" || fwRaw == "" {
		fmt.Fprintln(os.Stderr, "usage: session start --name <name> --frameworks <f1,f2,...>")
		os.Exit(1)
	}
	frameworks := strings.Split(fwRaw, ",")
	for i := range frameworks {
		frameworks[i] = strings.TrimSpace(frameworks[i])
	}

	c := newClient()
	out, err := c.post("/conversations", map[string]interface{}{
		"title":      name,
		"frameworks": frameworks,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	data, _ := out["data"].(map[string]interface{})
	conv, _ := data["conversation"].(map[string]interface{})
	sessionID = conv["id"].(string)
	os.WriteFile(".remora_session", []byte(sessionID), 0644)
	fmt.Printf("Sesion creada: %s\n", sessionID)
	fmt.Printf("Frameworks: %v\n", conv["frameworks"])
}

func handleSend() {
	sid := requireSession()
	msg := strings.Join(os.Args[2:], " ")
	if msg == "" {
		fmt.Fprintln(os.Stderr, "usage: send \"<mensaje>\"")
		os.Exit(1)
	}
	c := newClient()
	out, err := c.post("/conversations/"+sid+"/messages", map[string]interface{}{
		"content": msg,
		"role":    "user",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
}

func handlePoll() {
	sid := requireSession()
	c := newClient()
	out, err := c.get("/conversations/" + sid + "/poll")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
}

func handleAnswer() {
	sid := requireSession()
	qid := flagValue("--question-id")
	text := flagValue("--text")
	if text == "" && len(os.Args) > 2 {
		// Permitir: answer --question-id q_001 "texto libre"
		for i, a := range os.Args {
			if a == "--text" && i+1 < len(os.Args) {
				text = os.Args[i+1]
				break
			}
		}
	}
	if text == "" {
		fmt.Fprintln(os.Stderr, "usage: answer --question-id <id> --text \"<respuesta>\"")
		os.Exit(1)
	}
	c := newClient()
	out, err := c.post("/conversations/"+sid+"/messages", map[string]interface{}{
		"content":     text,
		"role":        "user",
		"question_id": qid,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
}

func handleInvoke() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: invoke <framework> --command <cmd> [--args ...]")
		os.Exit(1)
	}
	framework := os.Args[2]
	cmd := flagValue("--command")
	if cmd == "" {
		fmt.Fprintln(os.Stderr, "usage: invoke <framework> --command <cmd>")
		os.Exit(1)
	}
	var args []string
	if i := indexOf("--args", os.Args); i >= 0 && i+1 < len(os.Args) {
		args = strings.Split(os.Args[i+1], " ")
	}

	c := newClient()
	out, err := c.post("/frameworks/"+framework+"/invoke", map[string]interface{}{
		"command": cmd,
		"args":    args,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
}

func handleFrameworks() {
	c := newClient()
	out, err := c.get("/frameworks")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
}

func handleChat() {
	fwRaw := flagValue("--frameworks")
	sessionFlag := flagValue("--session")
	c := newClient()

	fmt.Println()
	fmt.Println(dim + "  ╭──────────────────────────────────────╮" + reset)
	fmt.Println(dim + "  │" + reset + bold + "  remora · red de frameworks RPC      " + reset + dim + "│" + reset)
	fmt.Println(dim + "  ╰──────────────────────────────────────╯" + reset)
	fmt.Println()

	if sessionFlag != "" {
		sessionID = sessionFlag
		os.WriteFile(".remora_session", []byte(sessionID), 0644)
	} else if fwRaw != "" {
		sessionID = ""
	} else {
		data, err := os.ReadFile(".remora_session")
		if err != nil {
			fmt.Fprintf(os.Stderr, red+"  ✖ sin sesión. Usa --frameworks"+reset+"\n")
			os.Exit(1)
		}
		sessionID = strings.TrimSpace(string(data))
	}

	if sessionID == "" && fwRaw != "" {
		frameworks := strings.Split(fwRaw, ",")
		for i := range frameworks {
			frameworks[i] = strings.TrimSpace(frameworks[i])
		}
		fmt.Print(gray + "  Creando sesión..." + reset)
		out, err := c.post("/conversations", map[string]interface{}{
			"title": "terminal-chat", "frameworks": frameworks,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n"+red+"  ✖ %v"+reset+"\n", err)
			os.Exit(1)
		}
		data, _ := out["data"].(map[string]interface{})
		conv, _ := data["conversation"].(map[string]interface{})
		sessionID = conv["id"].(string)
		os.WriteFile(".remora_session", []byte(sessionID), 0644)

		fmt.Print("\r" + green + "  ✓ sesión " + reset + dim + sessionID[:20] + "..." + reset + "\n")
		fmt.Print(gray + "  frameworks: " + reset)
		for i, fw := range frameworks {
			if i > 0 {
				fmt.Print(gray + ", " + reset)
			}
			fmt.Print(fwColor(fw) + bold + fw + reset)
		}
		fmt.Println()
	} else if sessionID == "" {
		fmt.Fprintf(os.Stderr, red+"  ✖ sin sesión. Usa --frameworks"+reset+"\n")
		os.Exit(1)
	}

	// Mostrar mensajes iniciales
	msgsResp, _ := c.get("/conversations/" + sessionID + "/messages")
	if data, ok := msgsResp["data"]; ok {
		if msgsArr, ok := data.([]interface{}); ok {
			for _, m := range msgsArr {
				if msg, ok := m.(map[string]interface{}); ok {
					if role, _ := msg["role"].(string); role == "framework" {
						fw, _ := msg["framework"].(string)
						content, _ := msg["content"].(string)
						renderMessage(fw, content, msg)
					}
				}
			}
		}
	}

	fmt.Println(gray + "  Escribe tu intención.  !help  !exit" + reset)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(bold + "▸ " + reset)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if input == "!exit" || input == "!quit" {
			fmt.Println(gray + "  sesión terminada." + reset)
			break
		}
		if input == "!help" {
			fmt.Println(gray + "  !exit / !quit = salir  |  !help = esto" + reset)
			continue
		}

		if err := runStreamingTurn(c, sessionID, input); err != nil {
			fmt.Println(red + "  ✖ " + err.Error() + reset)
		}
	}
}

// runStreamingTurn envía el mensaje del usuario con SSE y va renderizando
// cada evento a medida que llega:
//
//	llm_start        → "pensando..."
//	text_delta       → va imprimiendo texto char-a-char (streaming estilo tau)
//	toolcall_delta   → va mostrando args de la tool a medida que se materializan
//	tool_start       → "  ⚡ tool(args)" (fallback si no hubo toolcall_delta)
//	tool_end         → "    ↳ ok · 120ms"
//	framework_message → bubble final (via renderMessage)
//	done             → termina
func runStreamingTurn(c *Client, sessionID, input string) error {
	fmt.Println()
	printed := false

	// Estado: si estamos streameando texto, mantenemos abierto el "bubble"
	// del framework y vamos agregando. Así el final no duplica todo.
	textStreaming := false
	textIter := -1
	// Tool calls en progreso: index → nombre ya impreso
	toolCallPrinted := map[int]bool{}

	thinking := false
	startThinking := func(label string) {
		if thinking {
			return
		}
		thinking = true
		fmt.Print(dim + "  · " + label + dim + " ... " + reset)
	}
	stopThinking := func() {
		if !thinking {
			return
		}
		thinking = false
		fmt.Print("\r\033[K")
	}
	// Si abrimos un stream de texto, cerramos limpio antes de otro evento.
	closeTextStream := func() {
		if textStreaming {
			fmt.Println()
			textStreaming = false
		}
	}

	err := c.streamChat("/conversations/"+sessionID+"/messages", map[string]interface{}{
		"content": input, "role": "user",
	}, func(evt SSEEvent) bool {
		switch evt.Type {
		case "user_message":
			return true
		case "llm_start":
			// Solo mostramos "pensando" la 1ª vez; después con el streaming
			// de texto/tools ya se ve que está trabajando.
			iterF, _ := evt.Data["iter"].(float64)
			if int(iterF) == 0 {
				startThinking("pensando")
			}
			return true
		case "llm_error":
			stopThinking()
			closeTextStream()
			errStr, _ := evt.Data["error"].(string)
			fmt.Println(red + "  ✖ llm: " + errStr + reset)
			return true
		case "text_delta":
			stopThinking()
			chunk, _ := evt.Data["text"].(string)
			if chunk == "" {
				return true
			}
			iterF, _ := evt.Data["iter"].(float64)
			iter := int(iterF)
			if !textStreaming || iter != textIter {
				// Abrir un bubble nuevo de assistant-text.
				if textStreaming {
					fmt.Println()
				}
				fmt.Println()
				fmt.Println(cyan + bold + "  ── ARQUITECTO ──" + reset)
				fmt.Print("  ")
				textStreaming = true
				textIter = iter
			}
			// Normalizar newlines para mantener la indentación del bubble.
			fmt.Print(strings.ReplaceAll(chunk, "\n", "\n  "))
			return true
		case "toolcall_delta":
			stopThinking()
			closeTextStream()
			idxF, _ := evt.Data["tool_index"].(float64)
			idx := int(idxF)
			tool, _ := evt.Data["tool"].(string)
			args, _ := evt.Data["args"].(string)
			if tool == "" {
				return true
			}
			if !toolCallPrinted[idx] {
				fmt.Printf("%s  ⚡ %s%s%s(", cyan, bold, tool, reset+gray)
				toolCallPrinted[idx] = true
			} else {
				// Re-imprimimos la línea en sitio para mostrar args más recientes.
				fmt.Print("\r\033[K")
				fmt.Printf("%s  ⚡ %s%s%s(", cyan, bold, tool, reset+gray)
			}
			// Sanitizar args para una línea. args aquí es JSON en construcción;
			// mostramos lo que se ha acumulado hasta ahora, recortando el
			// wrapper {"path":"..."} a algo más compacto.
			short := compactArgs(args)
			fmt.Printf("%s%s", short, reset)
			return true
		case "tool_start":
			stopThinking()
			closeTextStream()
			idxF, _ := evt.Data["tool_index"].(float64)
			idx := int(idxF)
			tool, _ := evt.Data["tool"].(string)
			args, _ := evt.Data["args"].(string)
			if toolCallPrinted[idx] {
				// Ya veníamos imprimiendo vía delta; ahora cerramos el paréntesis
				// con el resumen oficial del framework (summarizeToolArgs).
				fmt.Print("\r\033[K")
			}
			fmt.Printf("%s  ⚡ %s%s%s(%s%s%s)\n", cyan, bold, tool, reset+cyan, reset+gray, args, reset)
			toolCallPrinted[idx] = true
			return true
		case "tool_end":
			ok, _ := evt.Data["ok"].(bool)
			durMs, _ := evt.Data["duration_ms"].(float64)
			status, _ := evt.Data["status"].(string)
			mark := green + "✓" + reset
			if !ok {
				mark = red + "✖" + reset
			}
			fmt.Printf("     %s %s%s · %dms%s\n", mark, gray, status, int(durMs), reset)
			return true
		case "assistant_final":
			return true
		case "turn_start", "turn_end":
			return true
		case "framework_message":
			stopThinking()
			// Si ya venimos streameando texto, no re-imprimimos el bubble final
			// (sería duplicado). Solo cerramos con salto de línea.
			if textStreaming {
				closeTextStream()
				fmt.Println()
				// Chips/suggested si vienen
				if chips, ok := evt.Data["suggested_chips"].([]interface{}); ok && len(chips) > 0 {
					fmt.Print(gray + "  ── opciones: " + reset)
					for i, chip := range chips {
						if i > 0 {
							fmt.Print(gray + " │ " + reset)
						}
						if s, ok := chip.(string); ok {
							fmt.Print(dim + s + reset)
						}
					}
					fmt.Println()
				}
				fmt.Println()
				printed = true
				return true
			}
			fw, _ := evt.Data["framework"].(string)
			content, _ := evt.Data["content"].(string)
			renderMessage(fw, content, evt.Data)
			printed = true
			return true
		case "error":
			stopThinking()
			closeTextStream()
			errStr, _ := evt.Data["error"].(string)
			fmt.Println(red + "  ✖ " + errStr + reset)
			return true
		case "done":
			stopThinking()
			closeTextStream()
			if idle, _ := evt.Data["idle"].(bool); idle && !printed {
				fmt.Println(gray + "  ⏼  sin más preguntas. Propone un cambio o !exit." + reset)
				fmt.Println()
			}
			return false
		}
		return true
	})
	stopThinking()
	closeTextStream()
	return err
}

// compactArgs toma una cadena JSON (posiblemente parcial) de argumentos
// y la renderiza compacta para el log en vivo. Ejemplos:
//
//	{"path":"channel/internal/handler.go"}   → channel/internal/handler.go
//	{"path":"foo","offset":10,"limit":50}    → foo:10+50
//	{"pattern":"func main","glob":"*.go"}   → "func main" *.go
func compactArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "..."
	}
	// Intento 1: JSON válido → extraer campos conocidos.
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err == nil {
		if p, ok := m["path"].(string); ok && p != "" {
			extra := ""
			if off, ok := m["offset"].(float64); ok {
				extra += fmt.Sprintf(":%d", int(off))
			}
			if lim, ok := m["limit"].(float64); ok {
				extra += fmt.Sprintf("+%d", int(lim))
			}
			return p + extra
		}
		if pat, ok := m["pattern"].(string); ok {
			scope, _ := m["path"].(string)
			glob, _ := m["glob"].(string)
			out := fmt.Sprintf("%q", pat)
			if scope != "" {
				out += " in " + scope
			}
			if glob != "" {
				out += " " + glob
			}
			return out
		}
		if q, ok := m["query"].(string); ok {
			return q
		}
	}
	// Intento 2: JSON parcial. Devolver como está pero compacto y sin newlines.
	clean := strings.ReplaceAll(raw, "\n", " ")
	clean = strings.ReplaceAll(clean, "\r", "")
	if len(clean) > 80 {
		clean = clean[:80] + "…"
	}
	return clean
}

func renderMessage(fw, content string, msg map[string]interface{}) {
	color := fwColor(fw)
	fmt.Println()
	fmt.Println(color + bold + "  ── " + strings.ToUpper(fw) + " ──" + reset)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, "📖📝⚠️✓✖💭🔍")
		trimmed = strings.TrimSpace(trimmed)

		switch {
		case strings.Contains(line, "📖") || strings.HasPrefix(line, "read:"):
			fmt.Println(gray + "  📖 leyendo  " + reset + trimmed)
		case strings.Contains(line, "📝") || strings.HasPrefix(line, "write:"):
			fmt.Println(gray + "  📝 escribiendo  " + reset + trimmed)
		case strings.Contains(line, "💭") || strings.HasPrefix(line, "think:"):
			fmt.Println(dim + "  💭 " + trimmed + reset)
		case strings.Contains(line, "⚠") || strings.HasPrefix(line, "warn:"):
			fmt.Println(yellow + "  ⚠ " + trimmed + reset)
		case strings.Contains(line, "✓") || strings.HasPrefix(line, "ok:"):
			fmt.Println(green + "  ✓ " + trimmed + reset)
		case strings.Contains(line, "✖") || strings.HasPrefix(line, "err:"):
			fmt.Println(red + "  ✖ " + trimmed + reset)
		case strings.Contains(line, "🔍") || strings.HasPrefix(line, "search:"):
			fmt.Println(gray + "  🔍 buscando  " + reset + trimmed)
		default:
			fmt.Println("  " + line)
		}
	}

	if chips, ok := msg["suggested_chips"].([]interface{}); ok && len(chips) > 0 {
		fmt.Print(gray + "  ── opciones: " + reset)
		for i, chip := range chips {
			if i > 0 {
				fmt.Print(gray + " │ " + reset)
			}
			if s, ok := chip.(string); ok {
				fmt.Print(dim + s + reset)
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

func showSpinner(label string, done chan struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-done:
			return
		default:
			fmt.Printf("\r"+gray+"  %s "+frames[i]+" "+label+reset, "")
			i = (i + 1) % len(frames)
			time.Sleep(80 * time.Millisecond)
		}
	}
}

// ensureBackendRunning detecta si api_rest local (:8084) responde.
// Si no, ejecuta scripts/dev-local.sh para levantar channel + api_rest
// y espera a que estén listos (hasta 15s). Solo funciona en local;
// para prod (REMORA_API_URL seteada a URL remota) no hace nada.
func ensureBackendRunning() {
	// Si el usuario apuntó a un backend remoto, no auto-levantamos nada.
	if os.Getenv("REMORA_API_URL") != "" && !strings.Contains(os.Getenv("REMORA_API_URL"), "localhost") {
		return
	}

	// Chequear si :8084 está abierto (api_rest).
	conn, err := net.DialTimeout("tcp", "localhost:8084", 800*time.Millisecond)
	if err == nil {
		conn.Close()
		return // Ya está corriendo
	}

	// Buscar el repo root (donde está scripts/dev-local.sh).
	// El binario del CLI vive en remora-cli/, y el repo es el padre.
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		fmt.Fprintln(os.Stderr, "⚠  No se encontró remora-go repo root. Backend no auto-levantado.")
		fmt.Fprintln(os.Stderr, "   Ejecutá manualmente: bash scripts/dev-local.sh")
		return
	}

	scriptPath := filepath.Join(repoRoot, "scripts", "dev-local.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  No existe %s. Backend no auto-levantado.\n", scriptPath)
		return
	}

	fmt.Fprintln(os.Stderr, "🚀 Backend no detectado. Levantando channel + api_rest...")
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  Error levantando backend: %v\n", err)
		return
	}

	// Esperar a que :8084 responda (hasta 15s).
	fmt.Fprintln(os.Stderr, "⏳ Esperando que api_rest esté listo...")
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.DialTimeout("tcp", "localhost:8084", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			fmt.Fprintln(os.Stderr, "✅ Backend listo.")
			return
		}
	}
	fmt.Fprintln(os.Stderr, "⚠  El backend no respondió a tiempo. Intentá manualmente.")
}

// findRepoRoot busca hacia arriba hasta encontrar scripts/dev-local.sh.
func findRepoRoot() string {
	// Si el binario está en remora-cli/, repo es el padre.
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for {
			if _, err := os.Stat(filepath.Join(dir, "scripts", "dev-local.sh")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// Fallback: buscar desde cwd.
	wd, _ := os.Getwd()
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "dev-local.sh")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func requireSession() string {
	if sessionID != "" {
		return sessionID
	}
	data, err := os.ReadFile(".remora_session")
	if err != nil {
		fmt.Fprintln(os.Stderr, "No hay sesion activa. Corre: remora session start --name ... --frameworks ...")
		os.Exit(1)
	}
	return strings.TrimSpace(string(data))
}

func flagValue(name string) string {
	for i, a := range os.Args {
		if a == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func indexOf(target string, arr []string) int {
	for i, v := range arr {
		if v == target {
			return i
		}
	}
	return -1
}

// PrettyPrint helpers para mostrar preguntas de forma legible
func init() {
	http.DefaultClient.Timeout = 30 * time.Second
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const stateFile = "temp/arquitecto_state.json"

// State es el modelo persistente del framework.
//
// History guarda la conversación con el LLM. PendingResponse es el último
// mensaje del assistant que aún no fue mostrado al usuario vía next-question.
// Cuando next-question lo consume, queda guardado solo en History.
type State struct {
	SessionID       string    `json:"session_id"`
	RepoPath        string    `json:"repo_path"`
	Initialized     bool      `json:"initialized"`
	Indexed         bool      `json:"indexed"`
	PendingResponse string    `json:"pending_response"`
	History         []ChatMsg `json:"history"`
	Packages        []PkgInfo `json:"packages"`
	Nodes           []Node    `json:"nodes"`
	LastUpdate      string    `json:"last_update"`
}

type PkgInfo struct {
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Files   []string `json:"files"`
	Imports []string `json:"imports"`
}

type Node struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Parent   string `json:"parent,omitempty"`
	Pkg      string `json:"pkg,omitempty"`
	Status   string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`Framework Arquitecto - Asistente conversacional sobre el codebase

Comandos del manifest (usados por el orquestador):
  next-question                            Devuelve el próximo mensaje del LLM
  ingest-answer --question-id X --answer Y Recibe respuesta y consulta al LLM

Comandos directos (debug):
  init --session-id ID --repo-path PATH
  index-repo --scope full|delta
  query-structure --query Q --format json|human
  status
  readiness`)
		os.Exit(0)
	}

	switch os.Args[1] {
	case "init":
		handleInit()
	case "index-repo":
		handleIndexRepo()
	case "query-structure":
		handleQueryStructure()
	case "status":
		handleStatus()
	case "readiness":
		handleReadiness()
	case "next-question":
		handleNextQuestion()
	case "ingest-answer":
		handleIngestAnswer()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// ===== state persistence =====

func loadState() *State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return &State{}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{}
	}
	return &s
}

func saveState(s *State) {
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(stateFile, data, 0644)
}

// ===== flag helpers =====

func flagValue(name string) string {
	for i, a := range os.Args {
		if a == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func flagValueDefault(name, def string) string {
	v := flagValue(name)
	if v == "" {
		return def
	}
	return v
}

// ===== init / index =====

func defaultRepoPath() string {
	if v := os.Getenv("REMORA_ROOT"); v != "" {
		return v
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if parent := filepath.Dir(cwd); parent != "" && parent != cwd {
		return parent
	}
	return cwd
}

func handleInit() {
	sessionID := flagValueDefault("--session-id", "cli-session")
	repoPath := flagValueDefault("--repo-path", defaultRepoPath())
	abs, _ := filepath.Abs(repoPath)
	s := &State{
		SessionID:   sessionID,
		RepoPath:    abs,
		Initialized: true,
		LastUpdate:  time.Now().Format(time.RFC3339),
	}
	saveState(s)
	fmt.Printf(`{"initialized":true,"session_id":%q,"repo_path":%q}`+"\n", sessionID, abs)
}

func handleIndexRepo() {
	s := loadState()
	if !s.Initialized {
		s.RepoPath = defaultRepoPath()
		s.Initialized = true
	}
	scope := flagValueDefault("--scope", "full")
	if scope == "delta" && s.Indexed {
		scanRepo(s)
	} else {
		s.Packages = nil
		s.Nodes = nil
		scanRepo(s)
	}
	s.Indexed = true
	s.LastUpdate = time.Now().Format(time.RFC3339)
	saveState(s)
	fmt.Printf(`{"indexed":true,"packages":%d,"nodes":%d}`+"\n", len(s.Packages), len(s.Nodes))
}

func scanRepo(s *State) {
	rootDir := s.RepoPath
	scanDir(rootDir, rootDir, s)
}

func scanDir(rootDir, dir string, s *State) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	hasGoFiles := false
	var goFiles []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "vendor" || name == "temp" || name == "node_modules" {
			continue
		}
		full := filepath.Join(dir, name)
		if e.IsDir() {
			scanDir(rootDir, full, s)
		} else if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			hasGoFiles = true
			goFiles = append(goFiles, full)
		}
	}
	if !hasGoFiles {
		return
	}
	rel, _ := filepath.Rel(rootDir, dir)
	if rel == "." {
		rel = ""
	}
	pkg := PkgInfo{Path: rel, Files: goFiles}
	parseGoPackage(&pkg, s, rootDir)
	s.Packages = append(s.Packages, pkg)
}

func parseGoPackage(pkg *PkgInfo, s *State, rootDir string) {
	fset := token.NewFileSet()
	pkgName := ""
	imports := map[string]bool{}
	for _, f := range pkg.Files {
		node, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			continue
		}
		if pkgName == "" {
			pkgName = node.Name.Name
		}
		for _, imp := range node.Imports {
			imports[strings.Trim(imp.Path.Value, `"`)] = true
		}
		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				id := fmt.Sprintf("fn_%s_%s", sanitizeID(pkgName), sanitizeID(d.Name.Name))
				recvType := ""
				if d.Recv != nil && len(d.Recv.List) > 0 {
					recvType = exprToString(d.Recv.List[0].Type)
					id = fmt.Sprintf("meth_%s_%s_%s", sanitizeID(pkgName), sanitizeID(recvType), sanitizeID(d.Name.Name))
				}
				if !nodeExists(s, id) {
					n := Node{
						ID: id, Type: "function", Name: d.Name.Name, Pkg: pkgName,
						Status:   "confirmed",
						Evidence: filepath.Join(pkg.Path, filepath.Base(f)),
					}
					if recvType != "" {
						n.Type = "method"
						n.Parent = recvType
					}
					s.Nodes = append(s.Nodes, n)
				}
			case *ast.GenDecl:
				if d.Tok == token.TYPE {
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok {
							id := fmt.Sprintf("type_%s_%s", sanitizeID(pkgName), sanitizeID(ts.Name.Name))
							if nodeExists(s, id) {
								continue
							}
							t := "type"
							switch ts.Type.(type) {
							case *ast.InterfaceType:
								t = "interface"
							case *ast.StructType:
								t = "struct"
							}
							s.Nodes = append(s.Nodes, Node{
								ID: id, Type: t, Name: ts.Name.Name, Pkg: pkgName,
								Status:   "confirmed",
								Evidence: filepath.Join(pkg.Path, filepath.Base(f)),
							})
						}
					}
				}
			}
		}
	}
	pkg.Name = pkgName
	for imp := range imports {
		pkg.Imports = append(pkg.Imports, imp)
	}
}

func nodeExists(s *State, id string) bool {
	for i := range s.Nodes {
		if s.Nodes[i].ID == id {
			return true
		}
	}
	return false
}

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	return strings.Trim(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, s), "_")
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// ===== query =====

func handleQueryStructure() {
	s := loadState()
	if !s.Indexed {
		fmt.Fprintln(os.Stderr, "no indexado")
		os.Exit(1)
	}
	q := strings.ToLower(flagValue("--query"))
	format := flagValueDefault("--format", "json")
	results := queryNodes(s, q)
	if format == "json" {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return
	}
	for _, n := range results {
		fmt.Printf("[%s] %s (%s) pkg=%s — %s\n", n.ID, n.Name, n.Type, n.Pkg, n.Evidence)
	}
}

func queryNodes(s *State, term string) []Node {
	term = strings.TrimSpace(strings.ToLower(term))
	if len(term) < 2 {
		return nil
	}
	tokens := strings.Fields(term)
	var out []Node
	for _, n := range s.Nodes {
		hay := strings.ToLower(n.Name + " " + n.Pkg + " " + n.Evidence)
		matchAll := true
		for _, t := range tokens {
			if !strings.Contains(hay, t) {
				matchAll = false
				break
			}
		}
		if matchAll {
			out = append(out, n)
		}
	}
	return out
}

func handleStatus() {
	s := loadState()
	data, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(data))
}

func handleReadiness() {
	s := loadState()
	action := "ready"
	switch {
	case !s.Initialized:
		action = "needs_init"
	case !s.Indexed:
		action = "needs_index"
	}
	fmt.Printf(`{"ready":%t,"recommended_action":%q,"packages":%d,"nodes":%d}`+"\n",
		action == "ready", action, len(s.Packages), len(s.Nodes))
}

// ===== conversational LLM flow =====

func handleNextQuestion() {
	s := loadState()

	// Si tenemos una respuesta del LLM en cola, devolverla.
	if s.PendingResponse != "" {
		qid := fmt.Sprintf("q_chat_%d", time.Now().UnixNano())
		out := map[string]string{"id": qid, "text": s.PendingResponse, "ask_via": ""}
		// Consumir: el orquestador la mostrará una vez al usuario.
		s.PendingResponse = ""
		saveState(s)
		emit(out)
		return
	}

	// Sin estado: pedir el repo (auto-init en el ingest siguiente).
	if !s.Initialized || !s.Indexed {
		emit(map[string]string{
			"id":      "q_bootstrap",
			"text":    fmt.Sprintf("Hola, soy Arquitecto. Voy a indexar %s para ayudarte con el código. ¿Qué querés mejorar, entender o cambiar?", filepath.Base(defaultRepoPath())),
			"ask_via": "",
		})
		return
	}

	// Indexado y sin pending: invitar a continuar.
	emit(map[string]string{
		"id":      "q_chat_idle",
		"text":    "Decime en qué seguimos trabajando.",
		"ask_via": "",
	})
}

func handleIngestAnswer() {
	s := loadState()
	answer := strings.TrimSpace(flagValue("--answer"))
	if answer == "" {
		fmt.Fprintln(os.Stderr, "usage: ingest-answer --question-id ID --answer TEXT")
		os.Exit(1)
	}

	// Bootstrap: la primera vez, inicializar e indexar.
	if !s.Initialized {
		s.SessionID = "cli-session"
		s.RepoPath = defaultRepoPath()
		abs, _ := filepath.Abs(s.RepoPath)
		s.RepoPath = abs
		s.Initialized = true
	}
	if !s.Indexed {
		scanRepo(s)
		s.Indexed = true
	}

	// Agregar mensaje del usuario al historial.
	s.History = append(s.History, ChatMsg{Role: "user", Content: answer})

	// Ejecutar el agent loop: puede pedir varias tools antes de responder.
	final, toolLog := runAgentLoop(s)

	// Persistir el turno (solo el mensaje final visible al usuario).
	// Los tool calls y resultados también van al historial para contexto en
	// próximos turnos, pero el usuario no los ve.
	// El historial ya fue actualizado por runAgentLoop.
	s.PendingResponse = final
	s.LastUpdate = time.Now().Format(time.RFC3339)

	// Acotar historial a últimos 40 mensajes (~20 turnos con tools).
	if len(s.History) > 40 {
		s.History = s.History[len(s.History)-40:]
	}
	saveState(s)

	fmt.Printf(`{"ingested":true,"action":"chat","reply_chars":%d,"tools_used":%d}`+"\n",
		len(final), toolLog)
}

// TraceEntry registra una tool call ejecutada durante el loop.
type TraceEntry struct {
	Tool     string
	Args     string // resumen human-readable
	Duration time.Duration
	OK       bool
}

// runAgentLoop implementa el loop del agente: pide al LLM una respuesta,
// si incluye tool_calls los ejecuta y re-llama al LLM con los resultados,
// hasta que el LLM emita un mensaje de texto final o se alcance el límite.
// Devuelve el texto final (con trace de tools prefijado) y cuántos tool calls se ejecutaron.
func runAgentLoop(s *State) (string, int) {
	maxIterations := 8
	tools := allToolDefs()
	system := buildSystemPrompt(s)
	trace := []TraceEntry{}

	// Inicializar stream de eventos live (JSONL) scope-ado por conv_id.
	stream := newLiveStream()
	defer stream.close()
	stream.emit(LiveEvent{Type: "turn_start", Time: time.Now().UnixMilli()})

	emptyRetries := 0
	for i := 0; i < maxIterations; i++ {
		stream.emit(LiveEvent{Type: "llm_start", Iter: i, Time: time.Now().UnixMilli()})
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		// tool_choice=auto siempre: permite que el LLM razone en voz alta
		// antes de emitir tool_calls, dando visibilidad al usuario.
		toolChoice := "auto"
		// Callback de streaming: cada delta que llega del LLM se emite al
		// live stream JSONL. El orquestador lo tail-ea y lo reenvía al CLI
		// vía SSE; el CLI renderiza texto char-a-char y args de tools en vivo.
		iter := i
		onDelta := func(d StreamDelta) {
			if d.Content != "" {
				stream.emit(LiveEvent{
					Type: "text_delta",
					Iter: iter,
					Text: d.Content,
					Time: time.Now().UnixMilli(),
				})
			}
			if d.ToolCall != nil {
				stream.emit(LiveEvent{
					Type:      "toolcall_delta",
					Iter:      iter,
					ToolIndex: d.ToolCall.Index,
					ID:        d.ToolCall.ID,
					Tool:      d.ToolCall.Name,
					Args:      d.ToolCall.ArgsSoFar,
					Time:      time.Now().UnixMilli(),
				})
			}
		}
		resp, err := llmCompleteWithTools(ctx, system, s.History, tools, toolChoice, onDelta)
		cancel()
		if err != nil {
			stream.emit(LiveEvent{Type: "llm_error", Error: err.Error(), Time: time.Now().UnixMilli()})
			msg := fmt.Sprintf("(LLM no disponible: %v)\n\nTengo %d packages y %d símbolos indexados.", err, len(s.Packages), len(s.Nodes))
			s.History = append(s.History, ChatMsg{Role: "assistant", Content: msg})
			stream.emit(LiveEvent{Type: "turn_end", Time: time.Now().UnixMilli()})
			return msg, len(trace)
		}

		// Agregar el turno del assistant al historial (con tool_calls si los hay).
		assistantMsg := ChatMsg{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls}
		s.History = append(s.History, assistantMsg)

		// Si no pidió tools, verificamos que sea una conclusión sustancial.
		if len(resp.ToolCalls) == 0 {
			if resp.Content == "" {
				stream.emit(LiveEvent{Type: "turn_end", Time: time.Now().UnixMilli()})
				return "(respuesta vacía del modelo)", len(trace)
			}
			// Si la respuesta es muy corta o una "conclusión vacía",
			// forzamos otra iteración pidiendo que amplíe.
			if isEmptyConclusion(resp.Content) && len(trace) > 0 {
				emptyRetries++
				if emptyRetries >= 3 {
					// El modelo es incapaz de concluir a pesar de los retries.
					// Devolvemos el trace + mensaje de error en vez de basura.
					forcedMsg := fmt.Sprintf("⚠️ El modelo no pudo generar una conclusión sustancial después de %d intentos.\n\nHerramientas ejecutadas (%d):", emptyRetries, len(trace))
					for _, t := range trace {
						forcedMsg += fmt.Sprintf("\n  %s(%s)  [%s]", t.Tool, t.Args, t.Duration.Round(time.Millisecond))
					}
					forcedMsg += "\n\nÚltima respuesta del modelo:\n" + resp.Content
					s.History = append(s.History, ChatMsg{Role: "assistant", Content: forcedMsg})
					stream.emit(LiveEvent{Type: "assistant_final", Text: forcedMsg, Time: time.Now().UnixMilli()})
					stream.emit(LiveEvent{Type: "turn_end", Time: time.Now().UnixMilli()})
					return forcedMsg, len(trace)
				}
				fmt.Fprintf(os.Stderr, "   ↳ conclusión vacía (retry %d/2)...\n", emptyRetries)
				var forceMsg string
				switch emptyRetries {
				case 1:
					forceMsg = "Expandí tu respuesta. Explicá en detalle qué encontraste en la investigación, qué aprendiste del código, y qué proponés como siguiente paso. No uses más tools, solo resumí para el usuario."
				case 2:
					forceMsg = "INSTRUCCIÓN OBLIGATORIA: escribí un análisis detallado de al menos 300 palabras explicando qué encontraste en los archivos que leíste, qué significa cada cosa, y qué proponés hacer. Esto es una ORDEN, no una sugerencia. No uses más tools, solo escribí el análisis."
				}
				s.History = append(s.History, ChatMsg{
					Role:    "user",
					Content: forceMsg,
				})
				// No retornamos: el loop itera de nuevo con esta pregunta.
				continue
			}
			final := prefixWithTrace(resp.Content, trace)
			stream.emit(LiveEvent{Type: "assistant_final", Text: resp.Content, Time: time.Now().UnixMilli()})
			stream.emit(LiveEvent{Type: "turn_end", Time: time.Now().UnixMilli()})
			return final, len(trace)
		}

		// Ejecutar cada tool_call y agregar resultado al historial.
		for _, tc := range resp.ToolCalls {
			argsSummary := summarizeToolArgs(tc.Function.Name, tc.Function.Arguments)
			stream.emit(LiveEvent{
				Type: "tool_start",
				Tool: tc.Function.Name,
				Args: argsSummary,
				ID:   tc.ID,
				Time: time.Now().UnixMilli(),
			})
			fmt.Fprintf(os.Stderr, "⚡ %s(%s)\n", tc.Function.Name, argsSummary)

			start := time.Now()
			result := executeTool(tc.Function.Name, tc.Function.Arguments, s.RepoPath)
			dur := time.Since(start)

			ok := !strings.HasPrefix(result, "error:")
			trace = append(trace, TraceEntry{
				Tool:     tc.Function.Name,
				Args:     argsSummary,
				Duration: dur,
				OK:       ok,
			})

			status := statusLabel(ok, result)
			stream.emit(LiveEvent{
				Type:       "tool_end",
				Tool:       tc.Function.Name,
				Args:       argsSummary,
				ID:         tc.ID,
				OK:         ok,
				Status:     status,
				DurationMs: dur.Milliseconds(),
				Time:       time.Now().UnixMilli(),
			})
			fmt.Fprintf(os.Stderr, "   ↳ %s en %s\n", status, dur.Round(time.Millisecond))

			// Limitar cada resultado para no reventar el context window.
			if len(result) > 12000 {
				result = result[:12000] + "\n(...resultado truncado)"
			}
			s.History = append(s.History, ChatMsg{
				Role:       "tool",
				Content:    result,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	finalMsg := fmt.Sprintf("(alcancé el límite de %d iteraciones de tools). Preguntame algo más acotado o dame más contexto.", maxIterations)
	s.History = append(s.History, ChatMsg{Role: "assistant", Content: finalMsg})
	stream.emit(LiveEvent{Type: "assistant_final", Text: finalMsg, Time: time.Now().UnixMilli()})
	stream.emit(LiveEvent{Type: "turn_end", Time: time.Now().UnixMilli()})
	return prefixWithTrace(finalMsg, trace), len(trace)
}

// LiveEvent es el formato JSONL que arquitecto emite al stream live durante
// un turno del agente. El orquestador lo tail-ea y lo reenvía al CLI vía SSE.
//
// Tipos emitidos:
//   - turn_start / turn_end           marcadores de turno
//   - llm_start / llm_error           inicio / error del LLM
//   - text_delta                      fragmento de texto del assistant (streaming)
//   - toolcall_delta                  args parciales de un tool_call (streaming)
//   - tool_start / tool_end           tool ejecutándose localmente
//   - assistant_final                 mensaje final del assistant (texto completo)
type LiveEvent struct {
	Type       string `json:"type"`
	Time       int64  `json:"time_ms"`
	Iter       int    `json:"iter,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Args       string `json:"args,omitempty"`
	ID         string `json:"id,omitempty"`
	ToolIndex  int    `json:"tool_index,omitempty"` // index dentro del turno (0, 1, ...)
	OK         bool   `json:"ok,omitempty"`
	Status     string `json:"status,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Text       string `json:"text,omitempty"`
	Error      string `json:"error,omitempty"`
}

// liveStream escribe eventos JSONL a temp/live_<conv_id>.jsonl.
// conv_id viene del env REMORA_CONV_ID (propagado por Channel).
// Si no hay conv_id, usa "default" — útil para pruebas CLI directas.
type liveStream struct {
	path string
	f    *os.File
}

func newLiveStream() *liveStream {
	convID := os.Getenv("REMORA_CONV_ID")
	if convID == "" {
		convID = "default"
	}
	path := filepath.Join(filepath.Dir(stateFile), "live_"+sanitizeForFilename(convID)+".jsonl")
	// Limpio el archivo al empezar el turno: es "eventos del turno actual".
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	if err != nil {
		return &liveStream{path: path}
	}
	return &liveStream{path: path, f: f}
}

func (ls *liveStream) emit(e LiveEvent) {
	if ls.f == nil {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = ls.f.Write(append(b, '\n'))
	_ = ls.f.Sync() // flush inmediato para que el tailer lo vea
}

func (ls *liveStream) close() {
	if ls.f != nil {
		_ = ls.f.Close()
	}
}

func sanitizeForFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// prefixWithTrace arma un bloque visible con las tools usadas y lo antepone
// al mensaje del LLM. Formato estilo Tau, compacto, emoji al inicio.
// Cuando estamos ejecutándonos vía orquestador (REMORA_CONV_ID seteado),
// los eventos JSONL ya se están emitiendo en vivo al CLI vía SSE, así
// que omitimos el prefijo para no duplicar la info en el bubble final.
func prefixWithTrace(msg string, trace []TraceEntry) string {
	if len(trace) == 0 {
		return msg
	}
	if os.Getenv("REMORA_CONV_ID") != "" {
		return msg
	}
	var sb strings.Builder
	sb.WriteString("```\n")
	total := time.Duration(0)
	for _, t := range trace {
		mark := "✓"
		if !t.OK {
			mark = "✗"
		}
		fmt.Fprintf(&sb, "%s %s(%s)  [%s]\n", mark, t.Tool, t.Args, t.Duration.Round(time.Millisecond))
		total += t.Duration
	}
	fmt.Fprintf(&sb, "— %d tool calls en %s —\n", len(trace), total.Round(time.Millisecond))
	sb.WriteString("```\n\n")
	sb.WriteString(msg)
	return sb.String()
}

// isEmptyConclusion detecta si el LLM dio una respuesta muy corta o vacía
// después de haber ejecutado tools. Se usa para forzar una conclusión real.
func isEmptyConclusion(s string) bool {
	if len(s) < 250 {
		return true
	}
	lower := strings.ToLower(s)
	emptyPhrases := []string{
		"sin más preguntas",
		"propone un cambio",
		"!exit",
		"no tengo más",
		"¿algo más",
		"¿necesitás algo más",
		"¿querés que",
		"¿te sirvió",
		"¿en qué más",
		"¿en qué puedo ayudarte",
		"¿necesitás ayuda",
		"¿necesitas ayuda",
		"espero que te sirva",
		"espero que esto te ayude",
		"espero que te ayude",
		"quedo a disposición",
		"quedo atento",
		"dime si necesitas",
		"dime si querés",
		"avísame si",
	}
	for _, p := range emptyPhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// summarizeToolArgs devuelve una versión corta de los argumentos para log.
func summarizeToolArgs(name, raw string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return "?"
	}
	switch name {
	case "read_file":
		p, _ := m["path"].(string)
		off := ""
		if v, ok := m["offset"]; ok {
			off = fmt.Sprintf(":%v", v)
		}
		lim := ""
		if v, ok := m["limit"]; ok {
			lim = fmt.Sprintf("+%v", v)
		}
		return p + off + lim
	case "list_dir":
		p, _ := m["path"].(string)
		if p == "" {
			p = "."
		}
		return p
	case "grep":
		pat, _ := m["pattern"].(string)
		scope, _ := m["path"].(string)
		glob, _ := m["glob"].(string)
		out := fmt.Sprintf("%q", pat)
		if scope != "" {
			out += " in " + scope
		}
		if glob != "" {
			out += " *" + glob
		}
		return out
	case "find_files":
		pat, _ := m["pattern"].(string)
		return pat
	case "query_symbols":
		q, _ := m["query"].(string)
		return q
	}
	return raw
}

func statusLabel(ok bool, result string) string {
	if ok {
		lines := strings.Count(result, "\n")
		return fmt.Sprintf("ok (%d líneas)", lines)
	}
	first := result
	if nl := strings.Index(first, "\n"); nl > 0 {
		first = first[:nl]
	}
	if len(first) > 80 {
		first = first[:80] + "…"
	}
	return "error: " + first
}

// lastIsUserMsg indica si el último mensaje del historial es del usuario.
// Se usa para decidir cuándo forzar tool_choice=required (después de un
// mensaje humano, queremos que el modelo explore).
func lastIsUserMsg(history []ChatMsg) bool {
	if len(history) == 0 {
		return false
	}
	return history[len(history)-1].Role == "user"
}

// buildSystemPrompt arma el prompt del rol "Arquitecto del repo".
// Incluye el resumen de paquetes y las instrucciones sobre qué tools usar.
func buildSystemPrompt(s *State) string {
	var sb strings.Builder
	sb.WriteString("Sos el Arquitecto, un asistente experto en este codebase Go ubicado en ")
	sb.WriteString(s.RepoPath)
	sb.WriteString(".\n\n")
	sb.WriteString("Tu trabajo es ayudar al usuario a entender y mejorar el código. ")
	sb.WriteString("Para eso tenés herramientas que te permiten leer archivos, listar directorios, buscar texto y símbolos.\n\n")

	sb.WriteString("REGLA DURA: ANTES de responder CUALQUIER pregunta sobre el código, DEBÉS abrir los archivos relevantes con read_file (o list_frameworks si la pregunta es sobre frameworks). ")
	sb.WriteString("No contestes de memoria. No contestes desde el resumen de paquetes. Abrí los archivos. ")
	sb.WriteString("El usuario prefiere ver tus tool calls — eso le da confianza de que la respuesta es real.\n\n")

	sb.WriteString("RAZONAMIENTO VISIBLE: ")
	sb.WriteString("Antes de usar CADA tool, explicá brevemente en qué estás pensando. ")
	sb.WriteString("Por ejemplo: 'Voy a leer channel/internal/handler.go para entender cómo procesa los mensajes JSON-RPC'. ")
	sb.WriteString("El usuario quiere seguir tu proceso de pensamiento. NO te quedes callado entre tools.\n\n")

	sb.WriteString("CONCLUSIÓN OBLIGATORIA Y DETALLADA: ")
	sb.WriteString("Después de recibir los resultados de TODAS las tools, DEBÉS escribir un análisis detallado de al menos 300 palabras para el usuario. ")
	sb.WriteString("Explicá qué encontraste, qué aprendiste, qué significa cada archivo que leíste, y qué proponés como siguiente paso. ")
	sb.WriteString("NUNCA termines con frases vacías como 'sin más preguntas', 'propone un cambio', '!exit', '¿en qué más te ayudo?', o similares. ")
	sb.WriteString("ES PROHIBIDO terminar sin contenido sustancial. Si no tenés suficiente info, pedí leer MÁS archivos con read_file. ")
	sb.WriteString("Siempre cerrá con valor: un análisis, una recomendación, o una pregunta concreta para avanzar.\n\n")

	sb.WriteString("REGLA ANTI-ALUCINACIÓN: ")
	sb.WriteString("Si un tool no te dio la info que necesitás, llamá OTRO tool. NUNCA inventes descripciones, ")
	sb.WriteString("nombres de funciones, paths, o comportamiento de código que no viste en el output de un tool. ")
	sb.WriteString("Si después de 3-4 tools no tenés suficiente info concreta, respondé 'no encontré evidencia clara de X' y sugerí dónde podría estar.\n\n")

	sb.WriteString("Herramientas disponibles:\n")
	sb.WriteString("- read_file(path, offset?, limit?): contenido real de un archivo\n")
	sb.WriteString("- list_dir(path): archivos de un directorio\n")
	sb.WriteString("- grep(pattern, path?, glob?): texto/regex en el código\n")
	sb.WriteString("- find_files(pattern, path?): archivos por nombre\n")
	sb.WriteString("- query_symbols(query, limit?): SOLO para símbolos Go (funcs/types/structs) en el índice AST. NO sirve para entender frameworks.\n")
	sb.WriteString("- list_frameworks(filter?): metadata semántica de los frameworks desde sus manifest.json. ÚSALO para cualquier pregunta sobre 'qué frameworks hay' o 'qué hace X framework'.\n\n")

	sb.WriteString("WORKFLOWS OBLIGATORIOS:\n\n")

	sb.WriteString("1) 'analizá el repo' / 'cómo está estructurado':\n")
	sb.WriteString("   list_dir(.) → read_file(README.md) → read_file(ARCHITECTURE.md si existe) → list_frameworks() → respuesta.\n\n")

	sb.WriteString("2) 'qué frameworks hay' / 'para qué sirve X framework' / 'cuáles son de programación':\n")
	sb.WriteString("   list_frameworks() PRIMERO (te da descripción + tags + intent_examples reales).\n")
	sb.WriteString("   Si necesitás más detalle de uno específico: read_file(framework-X/INITIAL_PROMPT.md) o read_file(framework-X/README.md).\n")
	sb.WriteString("   NO uses query_symbols para esto. query_symbols devuelve funciones Go cuyo nombre contiene la palabra — irrelevante.\n\n")

	sb.WriteString("3) '¿dónde está definida la función X?' / '¿qué hace runAgentLoop?':\n")
	sb.WriteString("   query_symbols(X) → read_file del archivo+líneas que devolvió → respuesta citando el código real.\n\n")

	sb.WriteString("4) '¿cómo funciona el sistema RPC/canal/orquestador?':\n")
	sb.WriteString("   list_dir(channel/) → read_file(channel/README.md) → read_file(channel/internal/handler.go) → read_file(channel/internal/jsonrpc.go) → respuesta con snippets.\n\n")

	sb.WriteString("5) 'más en detalle' / 'profundizá':\n")
	sb.WriteString("   NO repitas lo ya dicho — abrí MÁS archivos relevantes con read_file, seguí el flujo en el código, cita líneas concretas.\n\n")

	sb.WriteString("PRINCIPIOS:\n")
	sb.WriteString("- Podés encadenar hasta 8 tool calls en un turno. USALAS. Múltiples read_file en paralelo es normal.\n")
	sb.WriteString("- NO pidas permiso para explorar. Explorá y respondé.\n")
	sb.WriteString("- Citá paths completos y números de línea (ej: 'channel/internal/handler.go:42').\n")
	sb.WriteString("- Proponé snippets de código REALES (copiados del read_file), no pseudo-código.\n")
	sb.WriteString("- Sólo devolvé texto puro (sin tool_calls) cuando ya leíste los archivos relevantes y tenés info concreta.\n\n")

	sb.WriteString("Estructura del repo (referencia rápida, NO sustituye leer archivos):\n")
	sb.WriteString(packagesSummary(s, 50))

	sb.WriteString("\nRespondé en español, directo, sin preámbulos ni disclaimers.")
	return sb.String()
}

// packagesSummary devuelve un resumen compacto: pkg → cantidad de nodos.
func packagesSummary(s *State, maxPkgs int) string {
	type stat struct {
		Path  string
		Name  string
		Count int
	}
	pkgIndex := map[string]*stat{}
	for _, p := range s.Packages {
		key := p.Path
		if key == "" {
			key = p.Name
		}
		pkgIndex[key] = &stat{Path: p.Path, Name: p.Name}
	}
	for _, n := range s.Nodes {
		// Match by pkg name; el path más preciso ya lo tiene en evidence.
		for _, st := range pkgIndex {
			if st.Name == n.Pkg {
				st.Count++
				break
			}
		}
	}
	all := make([]*stat, 0, len(pkgIndex))
	for _, v := range pkgIndex {
		all = append(all, v)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
	if len(all) > maxPkgs {
		all = all[:maxPkgs]
	}
	var sb strings.Builder
	for _, st := range all {
		display := st.Path
		if display == "" {
			display = "(root)"
		}
		fmt.Fprintf(&sb, "  - %s: %d símbolos\n", display, st.Count)
	}
	return sb.String()
}

func emit(m map[string]string) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(m)
}

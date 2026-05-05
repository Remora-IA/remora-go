package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tool executor: cada función recibe argumentos JSON ya parseados y devuelve
// un string (lo que el LLM va a ver como resultado) o un error.
type toolExec func(args map[string]any, repoRoot string) (string, error)

var toolRegistry = map[string]toolExec{
	"read_file":       execReadFile,
	"list_dir":        execListDir,
	"grep":            execGrep,
	"find_files":      execFindFiles,
	"query_symbols":   execQuerySymbols,
	"list_frameworks": execListFrameworks,
}

// allToolDefs devuelve las definiciones que se mandan al LLM.
// El LLM decide cuándo llamarlas.
func allToolDefs() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: ToolDefFunction{
				Name:        "read_file",
				Description: "Lee el contenido de un archivo de texto del repo. Usá esto cuando necesités ver código real. Acepta paths relativos al repo o absolutos.",
				Parameters: jsonRaw(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "Path del archivo, relativo al repo o absoluto (ej: 'channel/cmd/orchestrator/main.go')",
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Línea inicial (1-indexed). Opcional.",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Cantidad máxima de líneas a leer. Default 200.",
						},
					},
					"required": []string{"path"},
				}),
			},
		},
		{
			Type: "function",
			Function: ToolDefFunction{
				Name:        "list_dir",
				Description: "Lista archivos y subdirectorios de una carpeta del repo. Útil para explorar la estructura.",
				Parameters: jsonRaw(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "Path del directorio (relativo o absoluto)",
						},
					},
					"required": []string{"path"},
				}),
			},
		},
		{
			Type: "function",
			Function: ToolDefFunction{
				Name:        "grep",
				Description: "Busca un patrón de texto en archivos del repo (usa ripgrep). Devuelve archivo:línea:match para cada coincidencia. Limitado a 100 resultados.",
				Parameters: jsonRaw(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{
							"type":        "string",
							"description": "Patrón regex a buscar",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Subdirectorio donde buscar (opcional, default = todo el repo)",
						},
						"glob": map[string]any{
							"type":        "string",
							"description": "Glob para filtrar archivos (ej: '*.go', '*.md'). Opcional.",
						},
					},
					"required": []string{"pattern"},
				}),
			},
		},
		{
			Type: "function",
			Function: ToolDefFunction{
				Name:        "find_files",
				Description: "Busca archivos por nombre en el repo. Acepta glob patterns.",
				Parameters: jsonRaw(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{
							"type":        "string",
							"description": "Nombre o glob (ej: 'main.go', '*.md')",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Subdirectorio donde buscar (opcional)",
						},
					},
					"required": []string{"pattern"},
				}),
			},
		},
		{
			Type: "function",
			Function: ToolDefFunction{
				Name: "query_symbols",
				Description: "Busca SÍMBOLOS GO (funciones, tipos, structs, interfaces) en el índice AST. " +
					"Devuelve nombre, tipo, paquete y archivo donde están definidos. " +
					"SOLO sirve para preguntas tipo '¿dónde está definida la función X?' o '¿qué structs existen?'. " +
					"NO sirve para entender qué hace un framework ni qué contiene un directorio — " +
					"para eso usá list_frameworks o read_file sobre README.md / manifest.json.",
				Parameters: jsonRaw(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Nombre de símbolo Go a buscar (ej: 'runAgentLoop', 'ChatMsg', 'handler').",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Máximo de resultados. Default 30.",
						},
					},
					"required": []string{"query"},
				}),
			},
		},
		{
			Type: "function",
			Function: ToolDefFunction{
				Name: "list_frameworks",
				Description: "Lista todos los frameworks del repo (directorios 'framework-*') con su metadata semántica " +
					"real: nombre, descripción, tags, intent_examples y produces. Lee de 'framework-*/framework.manifest.json'. " +
					"USÁ ESTE TOOL cuando el usuario pregunte 'qué frameworks hay', 'para qué sirve X framework', " +
					"'qué frameworks se usan para programar', etc. Es authoritative — no inventes descripciones, esto las tiene reales.",
				Parameters: jsonRaw(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"filter": map[string]any{
							"type":        "string",
							"description": "Opcional: substring a matchear contra nombre/description/tags. Vacío = todos.",
						},
					},
				}),
			},
		},
	}
}

func jsonRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// executeTool recibe el nombre y los argumentos serializados como llegan
// del LLM, ejecuta la tool y devuelve el resultado como string.
// Si la tool no existe o los argumentos están mal, devuelve un error string
// que igual se manda al LLM para que pueda corregirse.
func executeTool(name string, rawArgs string, repoRoot string) string {
	fn, ok := toolRegistry[name]
	if !ok {
		return fmt.Sprintf("error: tool %q no existe. Disponibles: %s", name, toolNames())
	}
	args := map[string]any{}
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return fmt.Sprintf("error: argumentos inválidos: %v\nraw: %s", err, rawArgs)
		}
	}
	out, err := fn(args, repoRoot)
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

func toolNames() string {
	names := []string{}
	for k := range toolRegistry {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}

// resolvePath normaliza un path dentro del repo y previene escapes.
func resolvePath(raw, repoRoot string) (string, error) {
	if raw == "" {
		return repoRoot, nil
	}
	var abs string
	if filepath.IsAbs(raw) {
		abs = filepath.Clean(raw)
	} else {
		abs = filepath.Clean(filepath.Join(repoRoot, raw))
	}
	repoRootAbs, _ := filepath.Abs(repoRoot)
	if !strings.HasPrefix(abs, repoRootAbs) {
		return "", fmt.Errorf("path %q está fuera del repo %s", raw, repoRootAbs)
	}
	return abs, nil
}

func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func argInt(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		var n int
		_, _ = fmt.Sscanf(x, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return def
}

// ===== tool implementations =====

func execReadFile(args map[string]any, repoRoot string) (string, error) {
	p := argString(args, "path")
	if p == "" {
		return "", fmt.Errorf("falta 'path'")
	}
	abs, err := resolvePath(p, repoRoot)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("no puedo acceder %s: %v", p, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s es un directorio, usá list_dir", p)
	}
	const maxBytes = 200 * 1024
	if info.Size() > maxBytes {
		return "", fmt.Errorf("archivo muy grande (%d bytes, máx %d). Usá offset/limit o leé un fragmento", info.Size(), maxBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	offset := argInt(args, "offset", 1)
	if offset < 1 {
		offset = 1
	}
	limit := argInt(args, "limit", 200)
	if limit < 1 {
		limit = 200
	}
	start := offset - 1
	if start >= len(lines) {
		return fmt.Sprintf("(archivo tiene %d líneas, offset %d fuera de rango)", len(lines), offset), nil
	}
	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== %s (líneas %d-%d de %d) ===\n", p, offset, end, len(lines))
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "%4d  %s\n", i+1, lines[i])
	}
	if end < len(lines) {
		fmt.Fprintf(&sb, "\n(...%d líneas más. Usá offset=%d para continuar.)\n", len(lines)-end, end+1)
	}
	return sb.String(), nil
}

func execListDir(args map[string]any, repoRoot string) (string, error) {
	p := argString(args, "path")
	abs, err := resolvePath(p, repoRoot)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	displayPath := p
	if displayPath == "" {
		displayPath = "."
	}
	fmt.Fprintf(&sb, "=== %s (%d entradas) ===\n", displayPath, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			fmt.Fprintf(&sb, "  %s/\n", name)
		} else {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			fmt.Fprintf(&sb, "  %s (%d bytes)\n", name, size)
		}
	}
	return sb.String(), nil
}

func execGrep(args map[string]any, repoRoot string) (string, error) {
	pattern := argString(args, "pattern")
	if pattern == "" {
		return "", fmt.Errorf("falta 'pattern'")
	}
	scope := argString(args, "path")
	absScope := repoRoot
	if scope != "" {
		var err error
		absScope, err = resolvePath(scope, repoRoot)
		if err != nil {
			return "", err
		}
	}
	cmdArgs := []string{"--line-number", "--no-heading", "--max-count", "10", "--max-columns", "240"}
	if glob := argString(args, "glob"); glob != "" {
		cmdArgs = append(cmdArgs, "--glob", glob)
	}
	cmdArgs = append(cmdArgs, pattern, absScope)
	out, err := exec.Command("rg", cmdArgs...).Output()
	if err != nil {
		// rg returns exit 1 when no matches; no es error real.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "(sin coincidencias)", nil
		}
		// Fallback: intentar con grep si rg no está.
		out2, err2 := exec.Command("grep", "-rn", "--exclude-dir=.git", "--exclude-dir=node_modules", "--exclude-dir=vendor", pattern, absScope).Output()
		if err2 != nil {
			return "", fmt.Errorf("rg falló (%v) y grep falló (%v)", err, err2)
		}
		out = out2
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) > 100 {
		lines = lines[:100]
		lines = append(lines, fmt.Sprintf("(...truncado a 100 líneas)"))
	}
	// Hacer paths relativos al repo para que sea más legible.
	repoRootAbs, _ := filepath.Abs(repoRoot)
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== grep %q en %s ===\n", pattern, scope)
	for _, ln := range lines {
		ln = strings.TrimPrefix(ln, repoRootAbs+"/")
		fmt.Fprintln(&sb, ln)
	}
	return sb.String(), nil
}

func execFindFiles(args map[string]any, repoRoot string) (string, error) {
	pattern := argString(args, "pattern")
	if pattern == "" {
		return "", fmt.Errorf("falta 'pattern'")
	}
	scope := argString(args, "path")
	absScope := repoRoot
	if scope != "" {
		var err error
		absScope, err = resolvePath(scope, repoRoot)
		if err != nil {
			return "", err
		}
	}
	results := []string{}
	repoRootAbs, _ := filepath.Abs(repoRoot)
	_ = filepath.Walk(absScope, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		base := filepath.Base(path)
		if info.IsDir() {
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "temp" {
				return filepath.SkipDir
			}
			return nil
		}
		matched, _ := filepath.Match(pattern, base)
		if matched || strings.Contains(base, strings.Trim(pattern, "*")) {
			rel := strings.TrimPrefix(path, repoRootAbs+"/")
			results = append(results, rel)
			if len(results) >= 200 {
				return filepath.SkipDir
			}
		}
		return nil
	})
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== find %q (%d resultados) ===\n", pattern, len(results))
	for _, r := range results {
		fmt.Fprintln(&sb, r)
	}
	return sb.String(), nil
}

func execQuerySymbols(args map[string]any, repoRoot string) (string, error) {
	q := argString(args, "query")
	if q == "" {
		return "", fmt.Errorf("falta 'query'")
	}
	limit := argInt(args, "limit", 30)
	s := loadState()
	if !s.Indexed {
		return "(índice no disponible; correr index-repo primero)", nil
	}
	matches := queryNodes(s, q)
	if len(matches) == 0 {
		return fmt.Sprintf("(sin símbolos para %q)", q), nil
	}
	if limit > len(matches) {
		limit = len(matches)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== symbols %q (%d total, mostrando %d) ===\n", q, len(matches), limit)
	for i := 0; i < limit; i++ {
		n := matches[i]
		fmt.Fprintf(&sb, "  %s (%s) en %s @ %s\n", n.Name, n.Type, n.Pkg, n.Evidence)
	}
	if limit < len(matches) {
		fmt.Fprintf(&sb, "(...y %d más)\n", len(matches)-limit)
	}
	return sb.String(), nil
}

// frameworkManifest es la forma mínima que leemos de framework.manifest.json.
// Solo los campos que nos interesan para describir el framework.
type frameworkManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Capabilities struct {
		Tags          []string `json:"tags,omitempty"`
		IntentExamples []string `json:"intent_examples,omitempty"`
		Produces      []string `json:"produces,omitempty"`
		Requires      []string `json:"requires,omitempty"`
	} `json:"capabilities_semantic,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
}

// execListFrameworks escanea repoRoot/framework-* leyendo cada
// framework.manifest.json. Retorna lista con name, description, tags, etc.
// — info authoritative directa del manifest (no inventada).
//
// Esta es la tool correcta para responder: "qué frameworks hay",
// "para qué sirve X framework", "cuáles son de programación", etc.
func execListFrameworks(args map[string]any, repoRoot string) (string, error) {
	filter := strings.ToLower(argString(args, "filter"))
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return "", err
	}
	type row struct {
		Name        string
		DirName     string
		Description string
		Tags        []string
		Examples    []string
		Mode        string
		HasManifest bool
	}
	rows := []row{}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "framework-") {
			continue
		}
		r := row{DirName: e.Name(), Name: strings.TrimPrefix(e.Name(), "framework-")}
		manifestPath := filepath.Join(repoRoot, e.Name(), "framework.manifest.json")
		data, rerr := os.ReadFile(manifestPath)
		if rerr == nil {
			var mf frameworkManifest
			if jerr := json.Unmarshal(data, &mf); jerr == nil {
				if mf.Name != "" {
					r.Name = mf.Name
				}
				r.Description = strings.TrimSpace(mf.Description)
				r.Tags = mf.Capabilities.Tags
				r.Examples = mf.Capabilities.IntentExamples
				r.Mode = mf.ExecutionMode
				r.HasManifest = true
			}
		}
		// Filtro opcional: matchea contra name, description, tags.
		if filter != "" {
			hay := strings.ToLower(r.Name + " " + r.Description + " " + strings.Join(r.Tags, " "))
			if !strings.Contains(hay, filter) {
				continue
			}
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		if filter != "" {
			return fmt.Sprintf("(sin frameworks que matcheen %q)", filter), nil
		}
		return "(sin directorios framework-* en el repo)", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== frameworks en el repo (%d) ===\n", len(rows))
	for _, r := range rows {
		fmt.Fprintf(&sb, "\n--- %s  (dir: %s)\n", r.Name, r.DirName)
		if !r.HasManifest {
			fmt.Fprintln(&sb, "  (sin framework.manifest.json — leé framework-"+r.Name+"/INITIAL_PROMPT.md o framework-"+r.Name+"/README.md para saber qué hace)")
			continue
		}
		if r.Description != "" {
			fmt.Fprintf(&sb, "  desc: %s\n", truncate(r.Description, 200))
		}
		if len(r.Tags) > 0 {
			fmt.Fprintf(&sb, "  tags: %s\n", strings.Join(r.Tags, ", "))
		}
		if r.Mode != "" {
			fmt.Fprintf(&sb, "  mode: %s\n", r.Mode)
		}
		if len(r.Examples) > 0 {
			fmt.Fprintf(&sb, "  ejemplos:\n")
			for _, ex := range r.Examples {
				if len(ex) > 100 {
					ex = ex[:100] + "…"
				}
				fmt.Fprintf(&sb, "    - %s\n", ex)
			}
		}
	}
	return sb.String(), nil
}

// truncate recorta un string a n caracteres agregando '…' si corresponde.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

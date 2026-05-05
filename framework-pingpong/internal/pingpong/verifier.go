package pingpong

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Expectation describe qué estructura del AST se espera para un paso dado.
type Expectation struct {
	Kind     string // "func", "var", "slice", "map", "for", "return"
	Name     string // nombre del símbolo esperado (si aplica)
	StepID   int
	StepText string
}

// VerifyReport es el resultado estructural de verificar un archivo contra el paso actual.
type VerifyReport struct {
	File        string   `json:"file"`
	SyntaxOK    bool     `json:"syntax_ok"`
	SyntaxError string   `json:"syntax_error,omitempty"`
	CompileOK   bool     `json:"compile_ok"`
	CompileLog  string   `json:"compile_log,omitempty"`
	StepID      int      `json:"step_id"`
	StepText    string   `json:"step_text"`
	Passed      bool     `json:"passed"`
	Missing     string   `json:"missing,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	Snippet     []string `json:"snippet,omitempty"`
}

// extractSnippet extracts lines around an error line from source code.
// errMsg must contain "file:LINE:COL: msg" format.
// Returns formatted lines like "  12 |   code" with "→ 14 |   code" for the error line.
func extractSnippet(src []byte, errMsg string, radius int) []string {
	parts := strings.SplitN(errMsg, ":", 4)
	if len(parts) < 3 {
		return nil
	}
	lineNum, err := strconv.Atoi(parts[1])
	if err != nil || lineNum < 1 {
		return nil
	}
	lines := strings.Split(string(src), "\n")
	start := lineNum - radius
	if start < 1 {
		start = 1
	}
	end := lineNum + radius
	if end > len(lines) {
		end = len(lines)
	}

	width := len(fmt.Sprintf("%d", end))
	var snippet []string
	for i := start; i <= end; i++ {
		prefix := "  "
		if i == lineNum {
			prefix = "→ "
		}
		snippet = append(snippet, fmt.Sprintf("%s%*d | %s", prefix, width, i, lines[i-1]))
	}
	return snippet
}

// inferExpectation convierte el texto declarativo de un paso en una expectativa AST.
// Es heurístico por keywords; cualquier paso no reconocido pasa solo si syntax_ok.
func inferExpectation(step Step) Expectation {
	t := strings.ToLower(step.Instruction)
	e := Expectation{StepID: step.ID, StepText: step.Instruction}

	switch {
	case strings.Contains(t, "función main") || strings.Contains(t, "funcion main") || strings.Contains(t, "func main"):
		e.Kind, e.Name = "func", "main"
	case strings.Contains(t, "método") || strings.Contains(t, "metodo") || strings.Contains(t, "receiver"):
		e.Kind = "method"
	case strings.Contains(t, "función ") || strings.Contains(t, "funcion "):
		e.Kind = "func"
		e.Name = extractName(step.Instruction, []string{"función ", "funcion ", "func "})
	case strings.Contains(t, "struct") || strings.Contains(t, "estructura"):
		e.Kind = "struct"
	case strings.Contains(t, "importar") || strings.Contains(t, "import"):
		e.Kind = "import"
	case strings.Contains(t, "array ") || strings.Contains(t, "slice "):
		e.Kind = "slice"
	case strings.Contains(t, "mapa ") || strings.Contains(t, "map "):
		e.Kind = "map"
	case strings.Contains(t, "variable "):
		e.Kind = "var"
	case strings.Contains(t, "retornar") || strings.Contains(t, "return"):
		e.Kind = "return"
	case strings.Contains(t, "búsqueda") || strings.Contains(t, "busqueda") ||
		strings.Contains(t, "loop") || strings.Contains(t, "iterar") ||
		strings.Contains(t, "implementar"):
		e.Kind = "for"
	case strings.Contains(t, "registrar") || strings.Contains(t, "register"):
		e.Kind = "call"
		e.Name = "Register"
	case strings.Contains(t, "listener") || strings.Contains(t, "listen") || strings.Contains(t, "escuche") || strings.Contains(t, "escuchar"):
		e.Kind = "call"
		e.Name = "Listen"
	case strings.Contains(t, "accept") || strings.Contains(t, "aceptar") || strings.Contains(t, "acceptar"):
		e.Kind = "call"
		e.Name = "Accept"
	case strings.Contains(t, "bloquear") || strings.Contains(t, "mantener") || strings.Contains(t, "servidor corriendo"):
		e.Kind = "call"
		e.Name = "Accept"
	default:
		e.Kind = "unknown"
	}
	return e
}

// spanishStopWords son palabras de relleno en español que aparecen en las
// instrucciones declarativas pero no son identificadores Go válidos.
var spanishStopWords = map[string]bool{
	"con": true, "un": true, "una": true, "el": true, "la": true,
	"los": true, "las": true, "de": true, "del": true, "para": true,
	"que": true, "en": true, "al": true, "por": true, "su": true,
	"sus": true, "cada": true, "y": true, "o": true, "a": true,
}

// extractName extrae la primera palabra significativa (no stop-word) que sigue
// a cualquiera de los prefijos. Si solo encuentra stop-words, retorna "" para
// que el verifier matchee cualquier entidad del tipo esperado.
func extractName(text string, prefixes []string) string {
	lower := strings.ToLower(text)
	for _, p := range prefixes {
		idx := strings.Index(lower, p)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(text[idx+len(p):])
		for _, word := range strings.Fields(rest) {
			// primera palabra (alfanumérica)
			var b strings.Builder
			for _, r := range word {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
					(r >= '0' && r <= '9') || r == '_' {
					b.WriteRune(r)
				} else {
					break
				}
			}
			candidate := b.String()
			if candidate == "" {
				continue
			}
			if spanishStopWords[strings.ToLower(candidate)] {
				continue
			}
			return candidate
		}
	}
	return ""
}

// VerifyFileLenient es como VerifyFile pero omite el type-check.
// Se usa en Scan para detectar progreso en archivos parcialmente escritos
// donde hay variables/imports sin usar que bloquearían el type-checker.
// El type-check se aplica después en el flujo normal de verify.
func VerifyFileLenient(path string, currentStep Step) (*VerifyReport, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", abs, err)
	}

	rep := &VerifyReport{
		File:     abs,
		StepID:   currentStep.ID,
		StepText: currentStep.Instruction,
	}

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if perr != nil {
		rep.SyntaxOK = false
		rep.SyntaxError = perr.Error()
		rep.Passed = false
		rep.Missing = firstLine(perr.Error())
		return rep, nil
	}
	rep.SyntaxOK = true
	rep.CompileOK = true // optimista: se verificará en el flujo normal

	exp := inferExpectation(currentStep)
	rep.Passed, rep.Missing, rep.Evidence = evaluate(file, fset, exp)
	return rep, nil
}

// DetectNoiseNames returns names of declarations that don't correspond to any step.
// Uses name matching: a declaration is noise if its name doesn't appear in any step instruction.
func DetectNoiseNames(filePath string, steps []Step) []string {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if perr != nil {
		return nil
	}

	var declNames []string
	for _, d := range file.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == "main" || x.Name.Name == "init" {
				continue
			}
			declNames = append(declNames, x.Name.Name)
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					declNames = append(declNames, ts.Name.Name)
				}
			}
		}
	}

	var noiseNames []string
	for _, name := range declNames {
		nameLower := strings.ToLower(name)
		matched := false
		for _, s := range steps {
			if strings.Contains(strings.ToLower(s.Instruction), nameLower) {
				matched = true
				break
			}
		}
		if !matched {
			noiseNames = append(noiseNames, name)
		}
	}
	return noiseNames
}

// RemoveDeclarations removes named declarations from a Go source file.
// Only deletes lines — never adds or modifies code. Returns names actually removed.
func RemoveDeclarations(filePath string, names []string) ([]string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if perr != nil {
		return nil, fmt.Errorf("parse error: %w", perr)
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	type removal struct {
		name      string
		startLine int
		endLine   int
	}
	var removals []removal

	for _, d := range file.Decls {
		var name string
		switch x := d.(type) {
		case *ast.FuncDecl:
			name = x.Name.Name
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					name = ts.Name.Name
				}
			}
		}
		if name != "" && nameSet[name] {
			startLine := fset.Position(d.Pos()).Line
			endLine := fset.Position(d.End()).Line
			removals = append(removals, removal{name, startLine, endLine})
		}
	}

	if len(removals) == 0 {
		return nil, nil
	}

	// Sort by start line descending — remove from bottom to preserve line numbers.
	sort.Slice(removals, func(i, j int) bool {
		return removals[i].startLine > removals[j].startLine
	})

	lines := strings.Split(string(src), "\n")
	var removed []string
	for _, r := range removals {
		start := r.startLine - 1 // 0-based
		end := r.endLine         // exclusive, 0-based
		if start < 0 || end > len(lines) {
			continue
		}
		// Also remove trailing blank lines after the declaration.
		for end < len(lines) && strings.TrimSpace(lines[end]) == "" {
			end++
		}
		lines = append(lines[:start], lines[end:]...)
		removed = append(removed, r.name)
	}

	result := strings.Join(lines, "\n")
	if err := os.WriteFile(abs, []byte(result), 0644); err != nil {
		return nil, fmt.Errorf("write error: %w", err)
	}
	return removed, nil
}

// ExtractDeclNames returns the names of all top-level declarations
// (functions, methods, type declarations) in the Go source file.
// Used for rewrite detection: comparing declarations across verify calls.
func ExtractDeclNames(filePath string) []string {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if perr != nil {
		return nil
	}

	var names []string
	for _, d := range file.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			names = append(names, x.Name.Name)
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					names = append(names, ts.Name.Name)
				}
			}
		}
	}
	return names
}

// ScanAnalysis runs post-scan analysis on the user's file:
// 1. Type-check (compile): catches field mismatches, undefined symbols, etc.
// 2. Noise detection: finds declarations that don't map to any step.
func ScanAnalysis(filePath string, steps []Step) (compileOK bool, compileLog string, noise []string) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return false, err.Error(), nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return false, err.Error(), nil
	}

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if perr != nil {
		return false, shortenTypeError(firstLine(perr.Error())), nil
	}

	// 1. Type-check
	compileOK = true
	if typeErr := runTypeCheck(fset, file); typeErr != "" {
		compileOK = false
		compileLog = typeErr
	}

	// 2. Noise detection: count declarations vs step expectations.
	// Collect all non-trivial declarations (skip main/init).
	type declInfo struct {
		kind string // "func", "method", "struct"
		name string
	}
	var decls []declInfo
	for _, d := range file.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == "main" || x.Name.Name == "init" {
				continue
			}
			if x.Recv != nil {
				decls = append(decls, declInfo{kind: "method", name: x.Name.Name})
			} else {
				decls = append(decls, declInfo{kind: "func", name: x.Name.Name})
			}
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); ok {
						decls = append(decls, declInfo{kind: "struct", name: ts.Name.Name})
					}
				}
			}
		}
	}

	// Count how many steps expect each declaration kind.
	wantByKind := map[string]int{}
	for _, s := range steps {
		exp := inferExpectation(s)
		switch exp.Kind {
		case "struct", "method", "func":
			wantByKind[exp.Kind]++
		}
	}

	// Group declarations by kind.
	haveByKind := map[string][]string{}
	for _, d := range decls {
		haveByKind[d.kind] = append(haveByKind[d.kind], d.name)
	}

	// Report excess declarations as noise.
	for kind, names := range haveByKind {
		want := wantByKind[kind]
		if len(names) > want {
			excess := names[want:]
			for _, name := range excess {
				label := kind
				if kind == "method" {
					label = "método"
				}
				noise = append(noise, fmt.Sprintf("%s %s — no corresponde a ningún paso del objetivo", label, name))
			}
		}
	}

	return compileOK, compileLog, noise
}

// VerifyFile parsea el archivo Go y evalúa si el paso actual está cumplido.
// No interpreta, no opina — solo inspecciona el AST.
func VerifyFile(path string, currentStep Step) (*VerifyReport, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", abs, err)
	}

	rep := &VerifyReport{
		File:     abs,
		StepID:   currentStep.ID,
		StepText: currentStep.Instruction,
	}

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, src, parser.AllErrors)
	if perr != nil {
		rep.SyntaxOK = false
		rep.SyntaxError = perr.Error()
		rep.Passed = false
		rep.Missing = firstLine(perr.Error())
		rep.Snippet = extractSnippet(src, perr.Error(), 3)
		return rep, nil
	}
	rep.SyntaxOK = true

	// Type-check con go/types. Catch errores de scope, variables no declaradas,
	// tipos incompatibles, retornos inválidos, etc. No requiere compilar ni red.
	if typeErr := runTypeCheck(fset, file); typeErr != "" {
		rep.CompileOK = false
		rep.CompileLog = typeErr
		rep.Passed = false
		rep.Missing = typeErr + sourceLine(typeErr, src)
		rep.Snippet = extractSnippet(src, typeErr, 3)
		return rep, nil
	}
	rep.CompileOK = true

	exp := inferExpectation(currentStep)
	rep.Passed, rep.Missing, rep.Evidence = evaluate(file, fset, exp)
	return rep, nil
}

// runTypeCheck intenta type-check el archivo como un paquete de un solo fichero.
// Si hay errores, devuelve la primera línea (suficientemente informativa).
// Si el archivo está en medio de edición y falta algo, el error indica qué.
func runTypeCheck(fset *token.FileSet, file *ast.File) string {
	var firstErr string
	conf := &types.Config{
		Importer: importer.Default(),
		Error: func(err error) {
			if firstErr == "" {
				firstErr = err.Error()
			}
		},
	}
	_, _ = conf.Check(file.Name.Name, fset, []*ast.File{file}, nil)
	if firstErr == "" {
		return ""
	}
	// Formatear: "/abs/path:line:col: msg" → "file.go:line:col: msg"
	return shortenTypeError(firstErr)
}

// sourceLine extrae el número de línea de un error tipo "file.go:LINE:col: msg"
// y devuelve la línea de código correspondiente, precedida por " | → ".
// Esto permite que la IA vea qué escribió realmente el usuario.
func sourceLine(typeErr string, src []byte) string {
	// Formato: "file.go:LINE:COL: msg"
	parts := strings.SplitN(typeErr, ":", 4)
	if len(parts) < 3 {
		return ""
	}
	lineNum, err := strconv.Atoi(parts[1])
	if err != nil || lineNum < 1 {
		return ""
	}
	lines := strings.Split(string(src), "\n")
	if lineNum > len(lines) {
		return ""
	}
	line := strings.TrimSpace(lines[lineNum-1])
	if line == "" {
		return ""
	}
	return " | → " + line
}

func shortenTypeError(e string) string {
	// types errors vienen como "/abs/path:L:C: msg"; acortamos a basename.
	if idx := strings.Index(e, ":"); idx > 0 {
		if strings.ContainsRune(e[:idx], '/') {
			return filepath.Base(e[:idx]) + e[idx:]
		}
	}
	return e
}

func evaluate(file *ast.File, fset *token.FileSet, exp Expectation) (bool, string, []string) {
	var ev []string
	switch exp.Kind {
	case "func":
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if exp.Name == "" || fd.Name.Name == exp.Name {
				ev = append(ev, fmt.Sprintf("func %s at %s", fd.Name.Name, posOf(fset, fd.Pos())))
				return true, "", ev
			}
		}
		return false, fmt.Sprintf("no se encontró función '%s'", exp.Name), ev

	case "slice":
		if n, pos := findAssignWithKind(file, exp.Name, "slice"); n != "" {
			ev = append(ev, fmt.Sprintf("%s := []T{...} at %s", n, posOf(fset, pos)))
			return true, "", ev
		}
		return false, fmt.Sprintf("no se encontró slice '%s' con valores", exp.Name), ev

	case "map":
		if n, pos := findAssignWithKind(file, exp.Name, "map"); n != "" {
			ev = append(ev, fmt.Sprintf("%s := map[...]... at %s", n, posOf(fset, pos)))
			return true, "", ev
		}
		return false, fmt.Sprintf("no se encontró mapa '%s'", exp.Name), ev

	case "var":
		if n, pos := findAssignWithKind(file, exp.Name, "any"); n != "" {
			ev = append(ev, fmt.Sprintf("%s := ... at %s", n, posOf(fset, pos)))
			return true, "", ev
		}
		return false, fmt.Sprintf("no se encontró variable '%s'", exp.Name), ev

	case "for":
		// Exigimos que el loop tenga cuerpo no vacío. Un loop con body {} no
		// implementa nada — se considera un placeholder estructural.
		var foundEmpty bool
		var foundWithBody bool
		var bodyPos token.Pos
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ForStmt:
				if x.Body != nil && len(x.Body.List) > 0 {
					foundWithBody = true
					bodyPos = x.Pos()
					return false
				}
				foundEmpty = true
			case *ast.RangeStmt:
				if x.Body != nil && len(x.Body.List) > 0 {
					foundWithBody = true
					bodyPos = x.Pos()
					return false
				}
				foundEmpty = true
			}
			return true
		})
		if foundWithBody {
			ev = append(ev, fmt.Sprintf("for con cuerpo no vacío at %s", posOf(fset, bodyPos)))
			return true, "", ev
		}
		if foundEmpty {
			return false, "el loop for tiene cuerpo vacío; agregá al menos una operación dentro", ev
		}
		return false, "no se encontró ningún for/range", ev

	case "return":
		// Exigimos que al menos un return tenga una expresión que referencie
		// un identifier (variable local, parámetro o llamada) y NO sea solo un
		// literal compuesto hardcoded como `return []int{1,2}`.
		var foundTrivial bool
		var foundReal bool
		var realPos token.Pos
		ast.Inspect(file, func(n ast.Node) bool {
			rs, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(rs.Results) == 0 {
				// `return` desnudo — aceptable solo si la función retorna void
				// pero esto no resuelve nada en LeetCode. Lo marcamos trivial.
				foundTrivial = true
				return true
			}
			if returnUsesIdentifier(rs.Results) {
				foundReal = true
				realPos = rs.Pos()
				return false
			}
			foundTrivial = true
			return true
		})
		if foundReal {
			ev = append(ev, fmt.Sprintf("return con expresión no trivial at %s", posOf(fset, realPos)))
			return true, "", ev
		}
		if foundTrivial {
			return false, "el return retorna un valor hardcoded; debe usar variables del contexto (parámetros o variables locales)", ev
		}
		return false, "no se encontró ningún return", ev

	case "struct":
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := ts.Type.(*ast.StructType); ok {
					if exp.Name == "" || strings.EqualFold(ts.Name.Name, exp.Name) {
						ev = append(ev, fmt.Sprintf("struct %s at %s", ts.Name.Name, posOf(fset, ts.Pos())))
						return true, "", ev
					}
				}
			}
		}
		// También buscar structs dentro de funciones (menos ideal pero el usuario puede hacerlo)
		var found bool
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := ts.Type.(*ast.StructType); ok {
				if exp.Name == "" || strings.EqualFold(ts.Name.Name, exp.Name) {
					ev = append(ev, fmt.Sprintf("struct %s at %s", ts.Name.Name, posOf(fset, ts.Pos())))
					found = true
					return false
				}
			}
			return true
		})
		if found {
			return true, "", ev
		}
		if exp.Name != "" {
			return false, fmt.Sprintf("no se encontró struct '%s'", exp.Name), ev
		}
		return false, "no se encontró ningún struct", ev

	case "import":
		if len(file.Imports) > 0 {
			for _, imp := range file.Imports {
				ev = append(ev, fmt.Sprintf("import %s", imp.Path.Value))
			}
			return true, "", ev
		}
		return false, "no se encontró ningún import", ev

	case "method":
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			ev = append(ev, fmt.Sprintf("method %s at %s", fd.Name.Name, posOf(fset, fd.Pos())))
			return true, "", ev
		}
		return false, "no se encontró ningún método con receiver", ev

	case "call":
		var found bool
		ast.Inspect(file, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callName := callExprName(ce)
			if exp.Name == "" || strings.Contains(callName, exp.Name) {
				ev = append(ev, fmt.Sprintf("call %s at %s", callName, posOf(fset, ce.Pos())))
				found = true
				return false
			}
			return true
		})
		if found {
			return true, "", ev
		}
		if exp.Name != "" {
			return false, fmt.Sprintf("no se encontró llamada a '%s'", exp.Name), ev
		}
		return false, "no se encontró la llamada esperada", ev

	default:
		// Paso no reconocido: NO auto-aprobar. Reportar que no se puede verificar.
		return false, fmt.Sprintf("el verifier no sabe cómo verificar este paso: '%s'. Si compiló correctamente, usá 'done --step %d' para marcarlo manualmente", exp.StepText, exp.StepID), ev
	}
}

// findAssignWithKind busca `name := <literal>` y devuelve (name, pos) si el tipo del RHS
// coincide con kind ("slice", "map", "any"). Si name == "" busca cualquier assignment.
func findAssignWithKind(file *ast.File, name string, kind string) (string, token.Pos) {
	var outName string
	var outPos token.Pos

	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) == 0 || len(as.Rhs) == 0 {
			return true
		}
		id, ok := as.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		if name != "" && id.Name != name {
			return true
		}
		if !rhsMatchesKind(as.Rhs[0], kind) {
			return true
		}
		outName = id.Name
		outPos = as.Pos()
		return false
	})
	return outName, outPos
}

// returnUsesIdentifier devuelve true si alguna de las expresiones del return
// referencia un identifier que NO sea constante built-in (true/false/nil/iota)
// ni un tipo primitivo (int, string, bool...). Así `return []int{1,2}`,
// `return 0`, `return nil`, `return true` se rechazan como triviales.
func returnUsesIdentifier(results []ast.Expr) bool {
	for _, r := range results {
		if exprHasMeaningfulIdent(r) {
			return true
		}
	}
	return false
}

// trivialIdent es la lista de identifiers que NO cuentan como "uso real"
// (constantes built-in y tipos primitivos del lenguaje).
var trivialIdent = map[string]bool{
	// Constantes built-in
	"true": true, "false": true, "nil": true, "iota": true,
	// Tipos numéricos
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true,
	"float32": true, "float64": true,
	"complex64": true, "complex128": true,
	// Otros tipos primitivos
	"string": true, "bool": true, "byte": true, "rune": true,
	"error": true, "any": true,
}

// exprHasMeaningfulIdent recorre la expresión saltando posiciones "de tipo"
// (Type de CompositeLit, Elt de ArrayType, Key/Value de MapType) y devuelve
// true si encuentra un identifier que no esté en trivialIdent.
func exprHasMeaningfulIdent(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return !trivialIdent[x.Name]
	case *ast.BasicLit:
		return false
	case *ast.CompositeLit:
		// Saltamos x.Type (es el tipo). Solo miramos los Elts.
		for _, el := range x.Elts {
			if exprHasMeaningfulIdent(el) {
				return true
			}
		}
		return false
	case *ast.KeyValueExpr:
		return exprHasMeaningfulIdent(x.Key) || exprHasMeaningfulIdent(x.Value)
	case *ast.UnaryExpr:
		return exprHasMeaningfulIdent(x.X)
	case *ast.BinaryExpr:
		return exprHasMeaningfulIdent(x.X) || exprHasMeaningfulIdent(x.Y)
	case *ast.ParenExpr:
		return exprHasMeaningfulIdent(x.X)
	case *ast.SelectorExpr:
		// p.ej. obj.Field → cuenta como uso (referencia a obj)
		return exprHasMeaningfulIdent(x.X)
	case *ast.IndexExpr:
		return exprHasMeaningfulIdent(x.X) || exprHasMeaningfulIdent(x.Index)
	case *ast.SliceExpr:
		if exprHasMeaningfulIdent(x.X) {
			return true
		}
		if x.Low != nil && exprHasMeaningfulIdent(x.Low) {
			return true
		}
		if x.High != nil && exprHasMeaningfulIdent(x.High) {
			return true
		}
		if x.Max != nil && exprHasMeaningfulIdent(x.Max) {
			return true
		}
		return false
	case *ast.CallExpr:
		// Una llamada cuenta como uso real (ej: helper(x), len(nums)).
		// Excepción: llamadas tipo conversión `int(x)` ya cubren x via Args.
		// Si el callee es un Ident trivial (ej `int(x)`), igual sus args pueden tener uso.
		if id, ok := x.Fun.(*ast.Ident); ok && trivialIdent[id.Name] {
			// conversión a tipo primitivo: revisar args
			for _, a := range x.Args {
				if exprHasMeaningfulIdent(a) {
					return true
				}
			}
			return false
		}
		return true
	case *ast.StarExpr:
		return exprHasMeaningfulIdent(x.X)
	case *ast.TypeAssertExpr:
		return exprHasMeaningfulIdent(x.X)
	}
	return false
}

func rhsMatchesKind(e ast.Expr, kind string) bool {
	switch kind {
	case "any":
		return true
	case "slice":
		if cl, ok := e.(*ast.CompositeLit); ok {
			if _, ok := cl.Type.(*ast.ArrayType); ok {
				return true
			}
		}
		// make([]T, ...)
		if ce, ok := e.(*ast.CallExpr); ok {
			if id, ok := ce.Fun.(*ast.Ident); ok && id.Name == "make" && len(ce.Args) > 0 {
				if _, ok := ce.Args[0].(*ast.ArrayType); ok {
					return true
				}
			}
		}
	case "map":
		if cl, ok := e.(*ast.CompositeLit); ok {
			if _, ok := cl.Type.(*ast.MapType); ok {
				return true
			}
		}
		if ce, ok := e.(*ast.CallExpr); ok {
			if id, ok := ce.Fun.(*ast.Ident); ok && id.Name == "make" && len(ce.Args) > 0 {
				if _, ok := ce.Args[0].(*ast.MapType); ok {
					return true
				}
			}
		}
	}
	return false
}

// callExprName extrae el nombre de la función/método de un CallExpr.
// Maneja: pkg.Func → "pkg.Func", obj.Method → "obj.Method", plain → "func".
func callExprName(ce *ast.CallExpr) string {
	switch fn := ce.Fun.(type) {
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name + "." + fn.Sel.Name
		}
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	}
	return ""
}

func posOf(fset *token.FileSet, p token.Pos) string {
	pos := fset.Position(p)
	return fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

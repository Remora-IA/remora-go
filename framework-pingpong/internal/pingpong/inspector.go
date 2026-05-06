package pingpong

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type SearchMatch struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Text    string   `json:"text"`
	Snippet []string `json:"snippet,omitempty"`
}

type CodeSymbol struct {
	File     string   `json:"file"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Receiver string   `json:"receiver,omitempty"`
	Fields   []string `json:"fields,omitempty"`
	Params   []string `json:"params,omitempty"`
	Returns  []string `json:"returns,omitempty"`
	Line     int      `json:"line"`
}

type StepInspection struct {
	Step       Step          `json:"step"`
	File       string        `json:"file"`
	Terms      []string      `json:"terms"`
	Evidence   []SearchMatch `json:"evidence"`
	Symbols    []CodeSymbol  `json:"symbols,omitempty"`
	CompileOK  bool          `json:"compile_ok"`
	CompileLog string        `json:"compile_log,omitempty"`
	Snippet    []string      `json:"compile_snippet,omitempty"`
	Guidance   string        `json:"guidance"`
}

func (c *Client) Search(filePath, root, query string, max int) (*Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search requiere --query <texto>")
	}
	if max <= 0 {
		max = 20
	}
	matches, err := searchCode(filePath, root, query, max)
	if err != nil {
		return nil, err
	}
	return &Result{
		Success: true,
		Message: fmt.Sprintf("%d coincidencias para %q", len(matches), query),
		Data: map[string]interface{}{
			"query":   query,
			"matches": matches,
		},
	}, nil
}

func (c *Client) Symbols(filePath, root string) (*Result, error) {
	symbols, err := collectSymbols(filePath, root)
	if err != nil {
		return nil, err
	}
	return &Result{
		Success: true,
		Message: fmt.Sprintf("%d símbolos encontrados", len(symbols)),
		Data: map[string]interface{}{
			"symbols": symbols,
		},
	}, nil
}

func (c *Client) Inspect(fileOverride string) (*Result, error) {
	p, err := c.loadOrCreate()
	if err != nil {
		return nil, err
	}
	if !p.Active {
		return nil, fmt.Errorf("no hay proyecto activo. Usa 'start --goal' primero")
	}
	current, err := currentStepForProgress(p)
	if err != nil {
		return nil, err
	}
	filePath, err := resolveFile(current, fileOverride)
	if err != nil {
		return nil, err
	}
	inspection := inspectStep(filePath, current, c.Lang)
	return &Result{
		Success: true,
		Message: fmt.Sprintf("Evidencia para paso %d: %s", current.ID, current.Instruction),
		Data: map[string]interface{}{
			"inspection":      inspection,
			"currentBatch":    buildBatchInfo(p),
			"overallProgress": overallProgress(p),
			"action_required": "judge_step_from_evidence",
		},
	}, nil
}

func currentStepForProgress(p *Progress) (Step, error) {
	if p.Detour != nil {
		d := p.Detour
		if d.CurrentStep >= 1 && d.CurrentStep <= len(d.Steps) {
			current := d.Steps[d.CurrentStep-1]
			for _, s := range p.Steps {
				if s.ID == d.ParentStepID && current.File == "" {
					current.File = s.File
					break
				}
			}
			return current, nil
		}
	}
	for _, s := range p.Steps {
		if s.ID == p.CurrentStep {
			return s, nil
		}
	}
	return Step{}, fmt.Errorf("no hay paso actual (currentStep=%d)", p.CurrentStep)
}

func inspectStep(filePath string, step Step, lang LangConfig) StepInspection {
	abs, _ := filepath.Abs(filePath)
	terms := extractInspectionTerms(step.Instruction)
	var evidence []SearchMatch
	seen := map[string]bool{}
	for _, term := range terms {
		matches, _ := searchCode(abs, "", term, 6)
		for _, m := range matches {
			key := fmt.Sprintf("%s:%d", m.File, m.Line)
			if !seen[key] {
				seen[key] = true
				evidence = append(evidence, m)
			}
		}
		if len(evidence) >= 10 {
			break
		}
	}
	symbols, _ := collectSymbols(abs, "")
	relevantSymbols := filterRelevantSymbols(symbols, terms)
	report := CompileCheck(abs, lang)
	guidance := "Juzgá el paso con evidence/symbols. compile_ok es diagnóstico separado: un error de compilación no relacionado no prueba que el paso esté incompleto."
	return StepInspection{
		Step:       step,
		File:       abs,
		Terms:      terms,
		Evidence:   evidence,
		Symbols:    relevantSymbols,
		CompileOK:  report.CompileOK,
		CompileLog: report.CompileLog,
		Snippet:    report.Snippet,
		Guidance:   guidance,
	}
}

func searchCode(filePath, root, query string, max int) ([]SearchMatch, error) {
	var files []string
	if filePath != "" {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return nil, err
		}
		files = []string{abs}
	} else {
		if root == "" {
			root = "."
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "temp" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if isSourceFile(path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	needle := strings.ToLower(query)
	var matches []SearchMatch
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), needle) {
				matches = append(matches, SearchMatch{File: file, Line: i + 1, Text: strings.TrimSpace(line), Snippet: formatSnippet(lines, i+1, 2)})
				if len(matches) >= max {
					return matches, nil
				}
			}
		}
	}
	return matches, nil
}

func collectSymbols(filePath, root string) ([]CodeSymbol, error) {
	var files []string
	if filePath != "" {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return nil, err
		}
		files = []string{abs}
	} else {
		if root == "" {
			root = "."
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "temp" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) == ".go" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var symbols []CodeSymbol
	for _, file := range files {
		ss, _ := goSymbols(file)
		symbols = append(symbols, ss...)
	}
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].File == symbols[j].File {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].File < symbols[j].File
	})
	return symbols, nil
}

func goSymbols(filePath string) ([]CodeSymbol, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if file == nil {
		return nil, err
	}
	var symbols []CodeSymbol
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				line := fset.Position(ts.Pos()).Line
				if st, ok := ts.Type.(*ast.StructType); ok {
					symbols = append(symbols, CodeSymbol{File: filePath, Kind: "struct", Name: ts.Name.Name, Fields: fieldListStrings(fset, st.Fields), Line: line})
				} else {
					symbols = append(symbols, CodeSymbol{File: filePath, Kind: "type", Name: ts.Name.Name, Line: line})
				}
			}
		case *ast.FuncDecl:
			line := fset.Position(d.Pos()).Line
			s := CodeSymbol{File: filePath, Kind: "func", Name: d.Name.Name, Params: fieldListStrings(fset, d.Type.Params), Returns: fieldListStrings(fset, d.Type.Results), Line: line}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				s.Kind = "method"
				s.Receiver = exprString(fset, d.Recv.List[0].Type)
			}
			symbols = append(symbols, s)
		}
	}
	return symbols, nil
}

func fieldListStrings(fset *token.FileSet, fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var out []string
	for _, f := range fields.List {
		typ := exprString(fset, f.Type)
		if len(f.Names) == 0 {
			out = append(out, typ)
			continue
		}
		for _, name := range f.Names {
			out = append(out, strings.TrimSpace(name.Name+" "+typ))
		}
	}
	return out
}

func exprString(fset *token.FileSet, x ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, fset, x)
	return b.String()
}

func extractInspectionTerms(instruction string) []string {
	stop := map[string]bool{
		"crear": true, "crea": true, "definir": true, "definí": true, "con": true, "que": true, "del": true, "de": true, "la": true, "el": true, "los": true, "las": true, "un": true, "una": true, "y": true, "o": true, "en": true, "al": true, "tipo": true, "campo": true, "campos": true, "función": true, "funcion": true, "método": true, "metodo": true, "estructura": true, "puntero": true, "reciba": true, "recibir": true, "usando": true, "usar": true, "mostrar": true, "resultado": false,
	}
	seen := map[string]bool{}
	var terms []string
	for _, raw := range strings.FieldsFunc(instruction, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.')
	}) {
		term := strings.TrimSpace(raw)
		if term == "" {
			continue
		}
		lower := strings.ToLower(term)
		if stop[lower] || len([]rune(term)) < 3 {
			continue
		}
		if !seen[lower] {
			seen[lower] = true
			terms = append(terms, term)
		}
	}
	return terms
}

func filterRelevantSymbols(symbols []CodeSymbol, terms []string) []CodeSymbol {
	var relevant []CodeSymbol
	for _, s := range symbols {
		hay := strings.ToLower(s.Name + " " + s.Receiver + " " + strings.Join(s.Fields, " ") + " " + strings.Join(s.Params, " ") + " " + strings.Join(s.Returns, " "))
		for _, term := range terms {
			if strings.Contains(hay, strings.ToLower(term)) {
				relevant = append(relevant, s)
				break
			}
		}
	}
	if len(relevant) > 12 {
		return relevant[:12]
	}
	return relevant
}

func autoAcceptStep(filePath string, step Step, lang LangConfig) (bool, string) {
	if lang.Name != "go" {
		return false, "auto solo disponible para evidencia estructural Go"
	}
	lower := strings.ToLower(step.Instruction)
	terms := extractInspectionTerms(step.Instruction)
	symbols, _ := collectSymbols(filePath, "")
	if strings.Contains(lower, "estructura") || strings.Contains(lower, "struct") {
		name := firstMatchingSymbolName(terms, symbols, "struct")
		if name == "" {
			return false, "no se encontró struct esperada"
		}
		for _, sym := range symbols {
			if sym.Kind != "struct" || !strings.EqualFold(sym.Name, name) {
				continue
			}
			missing := missingFieldTerms(terms, sym)
			if len(missing) > 0 {
				return false, "faltan campos: " + strings.Join(missing, ", ")
			}
			return true, fmt.Sprintf("struct %s existe", sym.Name)
		}
	}
	if strings.Contains(lower, "método") || strings.Contains(lower, "metodo") {
		for _, sym := range symbols {
			if sym.Kind == "method" && termMatches(terms, sym.Name) {
				return true, fmt.Sprintf("método %s existe", sym.Name)
			}
		}
		return false, "no se encontró método esperado"
	}
	if strings.Contains(lower, "función") || strings.Contains(lower, "funcion") {
		for _, sym := range symbols {
			if sym.Kind == "func" && (termMatches(terms, sym.Name) || strings.EqualFold(sym.Name, "main")) {
				return true, fmt.Sprintf("función %s existe", sym.Name)
			}
		}
		return false, "no se encontró función esperada"
	}
	return false, "paso no estructural; requiere juicio del tutor"
}

func firstMatchingSymbolName(terms []string, symbols []CodeSymbol, kind string) string {
	for _, term := range terms {
		for _, sym := range symbols {
			if sym.Kind == kind && strings.EqualFold(sym.Name, term) {
				return sym.Name
			}
		}
	}
	return ""
}

func missingFieldTerms(terms []string, sym CodeSymbol) []string {
	var missing []string
	for _, term := range terms {
		if !looksLikeExportedIdentifier(term) || strings.EqualFold(term, sym.Name) {
			continue
		}
		found := false
		for _, field := range sym.Fields {
			if strings.Contains(strings.ToLower(field), strings.ToLower(term)) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, term)
		}
	}
	return missing
}

func termMatches(terms []string, value string) bool {
	for _, term := range terms {
		if strings.EqualFold(term, value) {
			return true
		}
	}
	return false
}

func looksLikeExportedIdentifier(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)[0]
	return unicode.IsUpper(r)
}

func isSourceFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".java", ".rs", ".c", ".cpp", ".h", ".hpp":
		return true
	default:
		return false
	}
}

func formatSnippet(lines []string, line, radius int) []string {
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	width := len(fmt.Sprintf("%d", end))
	var snippet []string
	for i := start; i <= end; i++ {
		prefix := "  "
		if i == line {
			prefix = "→ "
		}
		snippet = append(snippet, fmt.Sprintf("%s%*d | %s", prefix, width, i, lines[i-1]))
	}
	return snippet
}

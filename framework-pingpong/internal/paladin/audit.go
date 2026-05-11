package paladin

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AuditResult struct {
	Root             string
	FilesScanned     int
	FilesWithPaladin int
	Calls            map[string]int
	Findings         []AuditFinding
}

type AuditFinding struct {
	Level   string
	Code    string
	Message string
}

var semanticCallNames = []string{
	"Actor",
	"Goal",
	"Event",
	"Rule",
	"Check",
	"Expect",
	"Handoff",
	"Violation",
}

var technicalCallNames = []string{
	"NewTrace",
	"NewTraceWithServer",
	"Start",
	"Flush",
	"Child",
	"Var",
	"Decision",
	"Error",
	"ErrorMsg",
}

func AuditRepo(root string) (AuditResult, error) {
	result := AuditResult{
		Root:  root,
		Calls: map[string]int{},
	}
	for _, name := range append(append([]string{}, semanticCallNames...), technicalCallNames...) {
		result.Calls[name] = 0
	}

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "temp", "node_modules", "vendor", "bin":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		result.FilesScanned++
		fileCalls, usesPaladin, err := auditGoFile(path)
		if err != nil {
			result.Findings = append(result.Findings, AuditFinding{
				Level:   "warn",
				Code:    "parse_error",
				Message: fmt.Sprintf("%s: %v", path, err),
			})
			return nil
		}
		if usesPaladin {
			result.FilesWithPaladin++
		}
		for name, count := range fileCalls {
			result.Calls[name] += count
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	result.Findings = append(result.Findings, evaluateAudit(result)...)
	return result, nil
}

func WriteAudit(w io.Writer, result AuditResult) {
	fmt.Fprintf(w, "Paladin Audit\n")
	fmt.Fprintf(w, "Repo: %s\n", result.Root)
	fmt.Fprintf(w, "Go files scanned: %d\n", result.FilesScanned)
	fmt.Fprintf(w, "Files using Paladin: %d\n\n", result.FilesWithPaladin)

	fmt.Fprintln(w, "Semantic Coverage")
	for _, name := range semanticCallNames {
		fmt.Fprintf(w, "  - %-9s %d\n", name+":", result.Calls[name])
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Technical Coverage")
	for _, name := range technicalCallNames {
		fmt.Fprintf(w, "  - %-19s %d\n", name+":", result.Calls[name])
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Findings")
	if len(result.Findings) == 0 {
		fmt.Fprintln(w, "  - ok: Paladin usage matches semantic tracing baseline")
		return
	}
	for _, finding := range result.Findings {
		fmt.Fprintf(w, "  - [%s] %s: %s\n", finding.Level, finding.Code, finding.Message)
	}
}

func auditGoFile(path string) (map[string]int, bool, error) {
	calls := map[string]int{}
	for _, name := range append(append([]string{}, semanticCallNames...), technicalCallNames...) {
		calls[name] = 0
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return calls, false, err
	}

	usesPaladin := false
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(importPath, "framework-paladin/paladin") || importPath == "paladin" {
			usesPaladin = true
			break
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			name := fun.Sel.Name
			if _, ok := calls[name]; ok {
				calls[name]++
				if isPaladinCallName(name) {
					usesPaladin = true
				}
			}
		}
		return true
	})

	return calls, usesPaladin, nil
}

func evaluateAudit(result AuditResult) []AuditFinding {
	var findings []AuditFinding
	totalSemantic := 0
	for _, name := range semanticCallNames {
		totalSemantic += result.Calls[name]
	}

	if result.Calls["NewTrace"]+result.Calls["NewTraceWithServer"] == 0 {
		findings = append(findings, AuditFinding{"fail", "no_trace", "no se encontró creación de traces Paladin"})
	}
	if result.Calls["Child"] == 0 {
		findings = append(findings, AuditFinding{"fail", "no_spans", "no se encontraron spans jerárquicos con Child"})
	}
	if totalSemantic == 0 {
		findings = append(findings, AuditFinding{"fail", "no_semantic_layer", "usa Paladin sin eventos semánticos; no cumple WHY.md"})
	}
	if result.Calls["Rule"] == 0 {
		findings = append(findings, AuditFinding{"warn", "no_rules", "no hay reglas de negocio declaradas con Rule"})
	}
	if result.Calls["Check"] == 0 {
		findings = append(findings, AuditFinding{"warn", "no_checks", "no hay evaluación explícita de reglas con Check"})
	}
	if result.Calls["Expect"] == 0 {
		findings = append(findings, AuditFinding{"warn", "no_expectations", "no hay próximos estados esperados con Expect"})
	}
	if result.Calls["Handoff"] == 0 {
		findings = append(findings, AuditFinding{"warn", "no_handoffs", "no hay transferencias de actor declaradas con Handoff"})
	}
	if result.Calls["Var"] > 0 && totalSemantic > 0 && result.Calls["Var"] > totalSemantic*4 {
		findings = append(findings, AuditFinding{"warn", "var_heavy", "hay muchas variables técnicas respecto a eventos semánticos; el trace puede volverse ruidoso"})
	}
	if result.Calls["Decision"] > 0 && result.Calls["Rule"] == 0 {
		findings = append(findings, AuditFinding{"warn", "decisions_without_rules", "hay decisiones sin reglas declaradas; una IA verá decisiones pero no contrato de negocio"})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		return findingRank(findings[i].Level) < findingRank(findings[j].Level)
	})
	return findings
}

func isPaladinCallName(name string) bool {
	for _, candidate := range semanticCallNames {
		if name == candidate {
			return true
		}
	}
	for _, candidate := range technicalCallNames {
		if name == candidate {
			return true
		}
	}
	return false
}

func findingRank(level string) int {
	switch level {
	case "fail":
		return 0
	case "warn":
		return 1
	default:
		return 2
	}
}

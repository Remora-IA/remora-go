package charlie

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ChangeType string

const (
	TypeFeat     ChangeType = "feat"
	TypeFix      ChangeType = "fix"
	TypeDocs     ChangeType = "docs"
	TypeTest     ChangeType = "test"
	TypeChore    ChangeType = "chore"
	TypeRefactor ChangeType = "refactor"
	TypePerf     ChangeType = "perf"
	TypeCI       ChangeType = "ci"
	TypeBuild    ChangeType = "build"
)

type Change struct {
	FilePath   string
	Type       ChangeType
	IsNew      bool
	IsDeleted  bool
	IsModified bool
}

type CharlieConfig struct {
	Version      string
	Conventional struct {
		Enabled bool `json:"enabled"`
		Types   []string
	}
	Semver struct {
		Enabled     bool
		BumpRules   map[string]string
	}
	Changelog struct {
		Format           string
		Unreleased      bool
		Categories      []string
	}
	Behavior struct {
		AutoClassify   bool
		SingleCommit   bool // ✅ NUEVO: Un solo commit por versión
		CommitFormat   string // ✅ NUEVO: "chore: commit vVERSION - descripcion"
	}
}

var DefaultConfig = CharlieConfig{
	Version: "0.1.0",
	Conventional: struct {
		Enabled bool
		Types   []string
	}{
		Enabled: true,
		Types:   []string{"feat", "fix", "docs", "refactor", "test", "chore", "perf", "ci", "build"},
	},
	Semver: struct {
		Enabled   bool
		BumpRules map[string]string
	}{
		Enabled:   true,
		BumpRules: map[string]string{"feat": "minor", "fix": "patch", "docs": "patch", "test": "patch", "chore": "patch", "refactor": "patch"},
	},
	Changelog: struct {
		Format      string
		Unreleased  bool
		Categories  []string
	}{
		Format:     "keep-a-changelog",
		Unreleased: true,
		Categories: []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"},
	},
	Behavior: struct {
		AutoClassify bool
		SingleCommit bool // ✅ Siempre true
		CommitFormat string
	}{
		AutoClassify: true,
		SingleCommit: true,
		CommitFormat: "chore: commit v{{version}} - {{description}}",
	},
}

// Cambiar directorio de trabajo
func ChangeToRepoRoot() error {
	if err := os.Chdir("/Users/alcless_a1234_cursor/remora-go"); err != nil {
		return fmt.Errorf("no se pudo cambiar al directorio del repo: %v", err)
	}
	return nil
}

// Ejecutar comando git
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Status returns porcelain git status
func Status() (string, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return "", err
	}
	return runGit("status", "--porcelain")
}

// GetCurrentTag returns the latest tag or "no-tags"
func GetCurrentTag() (string, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return "", err
	}
	tag, err := runGit("describe", "--tags", "--abbrev=0")
	if err != nil {
		return "no-tags", nil
	}
	return tag, nil
}

// ClassifyChanges classifies all pending changes
func ClassifyChanges() ([]Change, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}
	
	status, err := runGit("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	
	var changes []Change
	lines := strings.Split(status, "\n")
	
	ignoredPatterns := []string{
		".DS_Store",
		"charlie",      // binario compilado
		"examples/",    // ejemplos
		"temp/",        // temporal
		"cmd/",         // ejecutables
	}
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		// Ignorar archivos de la lista negra
		ignored := false
		for _, pattern := range ignoredPatterns {
			if strings.Contains(line, pattern) {
				ignored = true
				break
			}
		}
		if ignored {
			continue
		}
		
		// Parsear estado
		state := line[:2]
		file := strings.TrimSpace(line[3:])
		
		change := Change{
			FilePath: file,
			IsNew:    strings.Contains(state, "A"),
			IsDeleted: strings.Contains(state, "D"),
			IsModified: strings.Contains(state, "M"),
		}
		
		change.Type = ClassifyFile(file)
		changes = append(changes, change)
	}
	
	return changes, nil
}

// ClassifyFile returns the type of change for a file
func ClassifyFile(filePath string) ChangeType {
	// MD files (except CHANGELOG)
	if strings.HasSuffix(filePath, ".md") && !strings.Contains(filePath, "CHANGELOG") {
		return TypeDocs
	}
	
	// Test files
	if strings.HasSuffix(filePath, "_test.go") {
		return TypeTest
	}
	
	// Go files
	if strings.HasSuffix(filePath, ".go") {
		// Revisar si tiene lógica nueva
		content, err := os.ReadFile(filePath)
		if err == nil {
			hasLogic := hasNewLogic(string(content))
			if hasLogic {
				return TypeFeat
			}
		}
		return TypeRefactor
	}
	
	// Config files
	if strings.HasSuffix(filePath, "go.mod") || 
	   strings.HasSuffix(filePath, "go.sum") ||
	   filePath == ".gitignore" {
		return TypeChore
	}
	
	// GitHub workflows
	if strings.Contains(filePath, ".github/workflows/") {
		return TypeCI
	}
	
	// Default
	return TypeChore
}

// hasNewLogic checks if Go file has new logic (functions, not just imports)
func hasNewLogic(content string) bool {
	newPatterns := []string{"func ", "type ", "const ", "var ", "struct{", "interface{"}
	count := 0
	for _, pattern := range newPatterns {
		if strings.Contains(content, pattern) {
			count++
		}
	}
	return count > 2 // Más de 2 indica lógica nueva
}

// GetScope detects the framework scope from file path
func GetScope(filePath string) string {
	frameworks := map[string]string{
		"framework-alfa/":   "alfa",
		"framework-bravo/":  "bravo",
		"framework-charlie/": "charlie",
		"framework-echo/":  "echo",
		"framework-excel/": "excel",
		"framework-quine/": "quine",
		"framework-paladin/": "paladin",
		"remora-flujo/":     "flujo",
	}
	
	for prefix, scope := range frameworks {
		if strings.HasPrefix(filePath, prefix) {
			return scope
		}
	}
	return ""
}

// GenerateCommitMessage generates the ONE commit message for the version
func GenerateCommitMessage(changes []Change, version string, description string) string {
	if description == "" {
		description = summarizeChanges(changes)
	}
	return fmt.Sprintf("chore: commit v%s - %s", version, description)
}

// summarizeChanges creates a brief description of all changes
func summarizeChanges(changes []Change) string {
	typeGroups := make(map[ChangeType][]string)
	
	for _, c := range changes {
		scope := GetScope(c.FilePath)
		if scope != "" {
			typeGroups[c.Type] = append(typeGroups[c.Type], scope)
		}
	}
	
	var parts []string
	
	// Detectar nuevos frameworks
	newFrameworks := make(map[string]bool)
	for _, c := range changes {
		if c.IsNew && strings.Contains(c.FilePath, "framework-") {
			scope := GetScope(c.FilePath)
			if scope != "" {
				newFrameworks[scope] = true
			}
		}
	}
	if len(newFrameworks) > 0 {
		frameworks := make([]string, 0, len(newFrameworks))
		for f := range newFrameworks {
			frameworks = append(frameworks, f)
		}
		sort.Strings(frameworks)
		parts = append(parts, fmt.Sprintf("nuevos: %s", strings.Join(frameworks, ", ")))
	}
	
	// Detectar expansiones
	expansions := make(map[string]bool)
	for _, c := range changes {
		if c.IsModified || c.IsNew {
			scope := GetScope(c.FilePath)
			if scope != "" && !newFrameworks[scope] {
				expansions[scope] = true
			}
		}
	}
	if len(expansions) > 0 {
		frameworks := make([]string, 0, len(expansions))
		for f := range expansions {
			frameworks = append(frameworks, f)
		}
		sort.Strings(frameworks)
		parts = append(parts, fmt.Sprintf("expandir: %s", strings.Join(frameworks, ", ")))
	}
	
	if len(parts) == 0 {
		return "actualizar proyecto"
	}
	
	return strings.Join(parts, ", ")
}

// ParseVersion parses a version string and returns major, minor, patch
func ParseVersion(version string) (int, int, int) {
	re := regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 4 {
		return 0, 0, 0
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return major, minor, patch
}

// NextVersion calculates the next version based on changes
func NextVersion(currentTag string, changes []Change) string {
	major, minor, patch := ParseVersion(currentTag)
	if currentTag == "no-tags" {
		return "v0.0.1"
	}
	
	// Si hay nuevos frameworks -> minor bump
	hasNewFrameworks := false
	for _, c := range changes {
		if c.IsNew && strings.Contains(c.FilePath, "framework-") {
			hasNewFrameworks = true
			break
		}
	}
	
	if hasNewFrameworks {
		return fmt.Sprintf("v%d.%d.0", major, minor+1)
	}
	
	// Si hay docs significativos -> patch
	hasDocs := false
	for _, c := range changes {
		if c.Type == TypeDocs {
			hasDocs = true
			break
		}
	}
	
	// Default: patch
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch+1)
}

// GenerateChangelogSection generates changelog for a version
func GenerateChangelogSection(changes []Change, version string, date string) string {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	
	var lines []string
	lines = append(lines, fmt.Sprintf("## [%s] - %s", version, date))
	lines = append(lines, "")
	lines = append(lines, "> **Release**: resumen de cambios")
	lines = append(lines, "")
	
	// Agrupar por tipo
	typeGroups := make(map[ChangeType][]string)
	for _, c := range changes {
		typeGroups[c.Type] = append(typeGroups[c.Type], c.FilePath)
	}
	
	// Escribir cada grupo
	for _, changeType := range []ChangeType{TypeFeat, TypeDocs, TypeChore, TypeRefactor} {
		files := typeGroups[changeType]
		if len(files) > 0 {
			lines = append(lines, fmt.Sprintf("### %s", strings.ToUpper(string(changeType))))
			lines = append(lines, "")
			for _, f := range files {
				scope := GetScope(f)
				short := getShortDescription(f, changeType)
				lines = append(lines, fmt.Sprintf("- **%s**: %s", scope, short))
			}
			lines = append(lines, "")
		}
	}
	
	return strings.Join(lines, "\n")
}

// getShortDescription returns a brief description of the change
func getShortDescription(filePath string, changeType ChangeType) string {
	base := strings.TrimSuffix(filePath, ".go")
	base = strings.TrimPrefix(base, "framework-")
	
	// Quitar paths
	if idx := strings.LastIndex(base, "/"); idx > 0 {
		base = base[idx+1:]
	}
	
	descriptions := map[ChangeType]map[string]string{
		TypeFeat: {
			"charlie":     "Framework Charlie de versionado",
			"excel":       "Framework Excel para archivos Excel",
			"quine":       "Framework Quine auto-replicante",
			"paladin":     "Expansión de tracing",
			"echo":        "Agregar cmd y consola de tracing",
		},
		TypeDocs: {
			"README":       "documentación README",
			"SYSTEM":       "prompt del sistema",
			"INITIAL":      "prompt inicial",
		},
	}
	
	if d, ok := descriptions[changeType]; ok {
		for key, val := range d {
			if strings.Contains(base, key) {
				return val
			}
		}
	}
	
	// Default: usar el nombre del archivo
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	
	return base
}

// CheckIfClean returns true if repo has no pending changes
func CheckIfClean() (bool, error) {
	status, err := Status()
	if err != nil {
		return false, err
	}
	
	// Filtrar solo archivos no ignorados
	lines := strings.Split(status, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Ignorar .DS_Store y otros archivos macOS
		if strings.Contains(line, ".DS_Store") {
			continue
		}
		return false
	}
	return true, nil
}

// FormatResponse formats Charlie's response
func FormatResponse(changes []Change, version string, commitMsg string) string {
	var lines []string
	
	lines = append(lines, "=== CHARLIE ===")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("archivos: %d", len(changes)))
	lines = append(lines, fmt.Sprintf("tag: %s", version))
	lines = append(lines, "")
	
	// Agrupar por tipo
	typeGroups := make(map[ChangeType][]string)
	for _, c := range changes {
		typeGroups[c.Type] = append(typeGroups[c.Type], c.FilePath)
	}
	
	// Escribir grupos
	for changeType, files := range typeGroups {
		lines = append(lines, fmt.Sprintf("[%s] %d archivos", changeType, len(files)))
		for _, f := range files {
			lines = append(lines, fmt.Sprintf("  • %s", f))
		}
	}
	
	lines = append(lines, "")
	lines = append(lines, "--- PROPUESTA ---")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("commit: %s", commitMsg))
	lines = append(lines, fmt.Sprintf("versión: %s", version))
	lines = append(lines, "")
	lines = append(lines, "**Recordar**: Actualizar CHANGELOG.md con los cambios")
	
	return strings.Join(lines, "\n")
}
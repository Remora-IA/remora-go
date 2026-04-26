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
	TypeRefactor ChangeType = "refactor"
	TypeTest     ChangeType = "test"
	TypeChore    ChangeType = "chore"
	TypePerf     ChangeType = "perf"
	TypeCI       ChangeType = "ci"
	TypeBuild    ChangeType = "build"
	TypeBreaking ChangeType = "BREAKING"
)

type Change struct {
	FilePath   string
	ChangeType ChangeType
	Scope      string // opcional, ej: "alfa", "echo", "charlie"
	Message    string
	IsBreaking bool
}

type Status struct {
	CurrentVersion  string
	LastTag         string
	PendingChanges  []Change
	ChangelogExists bool
	HasUnreleased   bool
	RepoDirty       bool
	CommitsSinceTag int
}

type ReleaseSuggestion struct {
	BumpType       string // "major", "minor", "patch"
	NewVersion     string
	Reason         string
	ChangesSummary string
}

// ClassifyChanges analiza los archivos modificados y clasifica los cambios
func ClassifyChanges(dir string) ([]Change, error) {
	var changes []Change

	// Obtener archivos modificados
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error ejecutando git status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Formato: XY filename
		if len(line) < 3 {
			continue
		}
		statusFlag := line[:2]
		filePath := strings.TrimSpace(line[3:])

		// Ignorar archivos eliminados o renombrados
		if statusFlag == "D " || strings.HasPrefix(statusFlag, "R") {
			continue
		}

		change := classifyFile(filePath, dir)
		if change.FilePath != "" {
			changes = append(changes, change)
		}
	}

	return changes, nil
}

func classifyFile(filePath, dir string) Change {
	change := Change{FilePath: filePath}

	// Detectar breaking changes
	if isBreakingChange(dir, filePath) {
		change.IsBreaking = true
		change.ChangeType = TypeBreaking
		return change
	}

	// Clasificar por extensión y ubicación
	switch {
	case strings.HasSuffix(filePath, ".md") && !strings.Contains(filePath, "CHANGELOG"):
		change.ChangeType = TypeDocs

	case strings.HasSuffix(filePath, "_test.go"):
		change.ChangeType = TypeTest

	case strings.HasPrefix(filePath, ".github/workflows"):
		change.ChangeType = TypeCI

	case isConfigFile(filePath):
		change.ChangeType = TypeChore

	case strings.HasSuffix(filePath, ".go"):
		change.ChangeType = detectGoChangeType(dir, filePath)

	default:
		change.ChangeType = TypeChore
	}

	// Detectar scope
	change.Scope = detectScope(filePath)

	return change
}

func isBreakingChange(dir, filePath string) bool {
	// Buscar BREAKING CHANGE en el diff
	cmd := exec.Command("git", "diff", "--", filePath)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	text := strings.ToLower(string(output))
	breakIndicators := []string{"breaking change:", "breaking:", "-func", "-type", "-const"}
	for _, indicator := range breakIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}
	return false
}

func isConfigFile(filePath string) bool {
	configFiles := []string{
		".gitignore", ".gitattributes", ".goreleaser.yaml",
		"go.mod", "go.sum", "Makefile", "Dockerfile",
	}
	for _, cf := range configFiles {
		if strings.HasSuffix(filePath, cf) {
			return true
		}
	}
	return false
}

func detectGoChangeType(dir, filePath string) ChangeType {
	// Leer el diff para ver si hay lógica nueva o solo refactor
	cmd := exec.Command("git", "diff", "--", filePath)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return TypeRefactor
	}

	diff := string(output)

	// Si hay tests, es test
	if strings.HasSuffix(filePath, "_test.go") {
		return TypeTest
	}

	// Si hay cambios en lógica (nuevas funciones, nuevos archivos)
	hasNewLogic := strings.Contains(diff, "+func") ||
		strings.Contains(diff, "+type") ||
		strings.Contains(diff, "+const") ||
		strings.Contains(diff, "+package")

	if hasNewLogic {
		return TypeFeat
	}

	return TypeRefactor
}

func detectScope(filePath string) string {
	// Extraer scope de la ruta: framework-name/internal/path
	parts := strings.Split(filePath, "/")
	if len(parts) >= 1 {
		if parts[0] == "framework-alfa" || parts[0] == "framework-bravo" ||
			parts[0] == "framework-echo" || parts[0] == "framework-charlie" {
			return strings.TrimPrefix(parts[0], "framework-")
		}
		if parts[0] == "remora-flujo" {
			return "flujo"
		}
	}
	return ""
}

// GenerateCommitMessage genera un mensaje de commit convencional
func GenerateCommitMessage(changes []Change, includeScope bool) string {
	// Agrupar por tipo
	typeGroups := make(map[ChangeType][]Change)
	for _, c := range changes {
		typeGroups[c.ChangeType] = append(typeGroups[c.ChangeType], c)
	}

	var messages []string

	for _, changeType := range []ChangeType{TypeFeat, TypeFix, TypeDocs, TypeRefactor, TypeTest, TypeChore, TypePerf, TypeCI, TypeBuild} {
		group, exists := typeGroups[changeType]
		if !exists || len(group) == 0 {
			continue
		}

		prefix := string(changeType)
		if includeScope && group[0].Scope != "" {
			prefix = fmt.Sprintf("%s(%s)", changeType, group[0].Scope)
		}

		// Generar descripción basada en archivos
		descriptions := generateDescriptions(group)
		for _, desc := range descriptions {
			messages = append(messages, fmt.Sprintf("%s: %s", prefix, desc))
		}
	}

	// Unir si hay múltiples del mismo tipo, o devolver uno solo
	if len(messages) == 0 {
		return ""
	}
	if len(messages) == 1 {
		return messages[0]
	}

	// Devolver el primero (el más importante) y mencionar que hay más
	return messages[0]
}

func generateDescriptions(changes []Change) []string {
	var descriptions []string

	// Analizar patrones en archivos
	subjects := make(map[string]bool)

	for _, change := range changes {
		file := change.FilePath
		if strings.Contains(file, ".go") && !strings.Contains(file, "_test.go") {
			subjects["código"] = true
		}
		if strings.Contains(file, "_test.go") {
			subjects["tests"] = true
		}
		if strings.HasSuffix(file, ".md") {
			subjects["documentación"] = true
		}
	}

	// Construir descripción
	if len(subjects) > 0 {
		var subjectList []string
		for s := range subjects {
			subjectList = append(subjectList, s)
		}
		descriptions = append(descriptions, fmt.Sprintf("mejorar %s", strings.Join(subjectList, " y ")))
	}

	return descriptions
}

// CalculateNextVersion calcula la siguiente versión según semver
func CalculateNextVersion(currentVersion string, changes []Change) ReleaseSuggestion {
	major := false
	minor := false

	hasBreaking := false
	hasFeat := false

	for _, c := range changes {
		if c.IsBreaking || c.ChangeType == TypeBreaking {
			hasBreaking = true
		}
		if c.ChangeType == TypeFeat {
			hasFeat = true
		}
	}

	if hasBreaking {
		major = true
	} else if hasFeat {
		minor = true
	}

	newVersion := bump(currentVersion, major, minor)

	reason := "patch"
	if major {
		reason = "major (breaking change detectado)"
	} else if minor {
		reason = "minor (nuevas funcionalidades)"
	}

	return ReleaseSuggestion{
		BumpType:       reason,
		NewVersion:     newVersion,
		Reason:         reason,
		ChangesSummary: summarizeChanges(changes),
	}
}

func bump(version string, major, minor bool) string {
	// Parsear version actual
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) < 3 {
		parts = []string{"0", "0", "1"}
	}

	vMaj, _ := strconv.Atoi(parts[0])
	vMin, _ := strconv.Atoi(parts[1])
	vPat, _ := strconv.Atoi(parts[2])

	if major {
		vMaj++
		vMin = 0
		vPat = 0
	} else if minor {
		vMin++
		vPat = 0
	} else {
		vPat++
	}

	return fmt.Sprintf("v%d.%d.%d", vMaj, vMin, vPat)
}

func summarizeChanges(changes []Change) string {
	typeGroups := make(map[ChangeType]int)
	for _, c := range changes {
		typeGroups[c.ChangeType]++
	}

	var parts []string
	for ct, count := range typeGroups {
		parts = append(parts, fmt.Sprintf("%d %s", count, ct))
	}

	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// GetStatus obtiene el estado actual del repositorio
func GetStatus(dir string) (*Status, error) {
	status := &Status{}

	// Verificar si existe changelog
	if _, err := os.Stat(fullPath(dir, "CHANGELOG.md")); err == nil {
		status.ChangelogExists = true
	}

	// Obtener último tag
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		status.LastTag = strings.TrimSpace(string(output))
		status.CurrentVersion = status.LastTag
	}

	// Contar commits desde tag
	cmd = exec.Command("git", "log", "--oneline", fmt.Sprintf("%s..HEAD", status.LastTag))
	cmd.Dir = dir
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 && lines[0] != "" {
			status.CommitsSinceTag = len(lines)
		}
	}

	// Verificar si hay cambios pendientes
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err = cmd.Output()
	if err == nil {
		status.RepoDirty = strings.TrimSpace(string(output)) != ""
	}

	// Verificar si hay sección UNRELEASED
	if status.ChangelogExists {
		status.HasUnreleased = checkHasUnreleased(dir)
	}

	return status, nil
}

func checkHasUnreleased(dir string) bool {
	content, err := os.ReadFile(fullPath(dir, "CHANGELOG.md"))
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "## [UNRELEASED]")
}

func fullPath(dir, file string) string {
	if dir == "" {
		return file
	}
	return dir + "/" + file
}

// FormatStatus formatea el estado para mostrar al usuario
func FormatStatus(s *Status, changes []Change) string {
	var sb strings.Builder

	sb.WriteString("=== CHARLIE STATUS ===\n\n")
	sb.WriteString(fmt.Sprintf("📦 Versión actual: %s\n", s.CurrentVersion))
	sb.WriteString(fmt.Sprintf("🏷️ Último tag: %s\n", s.LastTag))
	sb.WriteString(fmt.Sprintf("📊 Commits desde tag: %d\n", s.CommitsSinceTag))

	if s.RepoDirty {
		sb.WriteString(fmt.Sprintf("⚠️ Cambios pendientes: %d archivos\n\n", len(changes)))
	} else {
		sb.WriteString("✅ Repo limpio\n\n")
	}

	if len(changes) > 0 {
		sb.WriteString("=== CAMBIOS DETECTADOS ===\n")
		typeGroups := groupByType(changes)
		for ct, chgs := range typeGroups {
			sb.WriteString(fmt.Sprintf("  %s: %d archivos\n", ct, len(chgs)))
		}
		sb.WriteString("\n")
	}

	if s.HasUnreleased {
		sb.WriteString("📝 Changelog tiene sección UNRELEASED\n\n")
	}

	return sb.String()
}

func groupByType(changes []Change) map[ChangeType][]Change {
	groups := make(map[ChangeType][]Change)
	for _, c := range changes {
		groups[c.ChangeType] = append(groups[c.ChangeType], c)
	}
	return groups
}

// UpdateChangelog actualiza el CHANGELOG.md
func UpdateChangelog(dir string, changes []Change, version string) error {
	changelogPath := fullPath(dir, "CHANGELOG.md")

	var content string
	if _, err := os.Stat(changelogPath); err == nil {
		data, err := os.ReadFile(changelogPath)
		if err == nil {
			content = string(data)
		}
	}

	// Detectar categoría según el tipo de cambio más importante
	category := detectChangelogCategory(changes)

	// Generar entrada
	date := time.Now().Format("2006-01-02")
	entry := fmt.Sprintf("\n## [%s] - %s\n\n### %s\n", version, date, category)

	// Generar líneas de cambio
	for _, c := range changes {
		prefix := string(c.ChangeType)
		if c.Scope != "" {
			prefix = fmt.Sprintf("%s(%s)", c.ChangeType, c.Scope)
		}
		if c.IsBreaking {
			prefix += " [!]"
		}
		entry += fmt.Sprintf("- %s: %s\n", prefix, c.Message)
	}

	// Insertar después del header o antes de la primera versión
	if strings.Contains(content, "## [UNRELEASED]") {
		// Ya existe unreleased, no hacer nada
		return nil
	}

	// Insertar entrada
	lines := strings.Split(content, "\n")
	var newContent []string

	// Encontrar donde insertar (después del header principal)
	inserted := false
	for _, line := range lines {
		newContent = append(newContent, line)
		if !inserted && strings.HasPrefix(line, "## [") && line != "## [UNRELEASED]" {
			// Insertar antes de esta línea (primera versión)
			newContent = append(newContent, entry)
			inserted = true
		}
	}

	if !inserted {
		newContent = append(newContent, entry)
	}

	return os.WriteFile(changelogPath, []byte(strings.Join(newContent, "\n")), 0644)
}

func detectChangelogCategory(changes []Change) string {
	for _, c := range changes {
		if c.ChangeType == TypeFeat || c.ChangeType == TypeBreaking {
			return "Added"
		}
		if c.ChangeType == TypeFix {
			return "Fixed"
		}
		if c.ChangeType == TypeDocs {
			return "Changed"
		}
	}
	return "Changed"
}

// SuggestRelease verifica si es momento de hacer release
func SuggestRelease(s *Status, changes []Change) string {
	if !s.RepoDirty && s.CommitsSinceTag > 0 {
		return fmt.Sprintf("✅ Listo para release: %d commits desde último tag '%s'",
			s.CommitsSinceTag, s.LastTag)
	}

	if s.RepoDirty && len(changes) > 0 {
		return fmt.Sprintf("⚠️ Hay %d archivos sin commit. Recomendable hacer commit antes de release.",
			len(changes))
	}

	return "📋 No hay cambios que requieran release"
}

// ValidateCommitMessage valida que el mensaje siga conventional commits
func ValidateCommitMessage(message string) (bool, string) {
	// Formato: type(scope): description
	pattern := `^(feat|fix|docs|refactor|test|chore|perf|ci|build)(\([a-z0-9-]+\))?!?: .+`
	matched, _ := regexp.MatchString(pattern, strings.ToLower(message))

	if !matched {
		return false, "El mensaje debe seguir conventional commits: tipo(alcance): descripción"
	}

	// Verificar longitud
	parts := strings.SplitN(message, ":", 2)
	if len(parts) == 2 && len(strings.TrimSpace(parts[1])) > 72 {
		return false, "La descripción debe ser menor a 72 caracteres"
	}

	return true, "✅ Mensaje válido"
}
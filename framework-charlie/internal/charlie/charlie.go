package charlie

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ChangeType string

const (
	TypeFeat     ChangeType = "feat"
	TypeDocs     ChangeType = "docs"
	TypeTest     ChangeType = "test"
	TypeChore    ChangeType = "chore"
	TypeRefactor ChangeType = "refactor"
	TypeCI       ChangeType = "ci"
)

type Change struct {
	FilePath    string
	Type        ChangeType
	Status      string
	IsNew       bool
	IsDeleted   bool
	IsModified  bool
	DiffSummary DiffSummary
}

type DiffSummary struct {
	AddedLines   int
	DeletedLines int
	ChangedFuncs []string
	ChangedTypes []string
	IsBinary     bool
	IsUntracked  bool
	ShortSummary string
	SemanticTags []string // etiquetas semánticas: "api-semantica", "cola-preguntas", "quine-taxonomia", etc.
}

// semanticPatterns detecta qué tipo de cambio significativo ocurrió
func detectSemanticPatterns(filePath string, diff string, isNew bool, content string) []string {
	var tags []string

	// Detectar API semántica de Paladin
	if strings.Contains(filePath, "paladin/") {
		if strings.Contains(diff, "func (c *Context) Actor") || strings.Contains(diff, "Actor(name") {
			tags = append(tags, "api-actor")
		}
		if strings.Contains(diff, "func (c *Context) Goal") || strings.Contains(diff, "Goal(") {
			tags = append(tags, "api-goal")
		}
		if strings.Contains(diff, "func (c *Context) Rule") || strings.Contains(diff, "Rule(") {
			tags = append(tags, "api-rule")
		}
		if strings.Contains(diff, "func (c *Context) Check") || strings.Contains(diff, "Check(") {
			tags = append(tags, "api-check")
		}
		if strings.Contains(diff, "func (c *Context) Handoff") || strings.Contains(diff, "Handoff(") {
			tags = append(tags, "api-handoff")
		}
		if strings.Contains(diff, "func (c *Context) Expect") || strings.Contains(diff, "Expect(") {
			tags = append(tags, "api-expect")
		}
		if strings.Contains(diff, "type SemanticEvent") {
			tags = append(tags, "tipo-semantic-event")
		}
		if strings.Contains(diff, "func AuditRepo") || strings.Contains(diff, "type AuditResult") {
			tags = append(tags, "paladin-audit")
		}
		if strings.Contains(diff, "func BuildExplanation") || strings.Contains(diff, "type Explanation") {
			tags = append(tags, "paladin-explain")
		}
		if strings.Contains(diff, "type TraceClient") || strings.Contains(diff, "func NewTraceClient") {
			tags = append(tags, "paladin-client")
		}
		if strings.Contains(diff, "type Server") && strings.Contains(filePath, "server") {
			tags = append(tags, "paladin-server")
		}
	}

	// Detectar cola de preguntas de Flujo
	if strings.Contains(filePath, "handoff/") {
		if strings.Contains(diff, "type QuestionsQueue") || strings.Contains(diff, "NewQuestionsQueue") {
			tags = append(tags, "cola-preguntas")
		}
		if strings.Contains(diff, "EventEchoUserAnswered") || strings.Contains(diff, "echo_user_answered") {
			tags = append(tags, "evento-echo-user-answered")
		}
		if strings.Contains(diff, "EventAlfaCededToEcho") || strings.Contains(diff, "alfa_ceded_to_echo") {
			tags = append(tags, "evento-alfa-ceded")
		}
		if strings.Contains(diff, "EventAlfaAsksQuestion") || strings.Contains(diff, "alfa_asks_question") {
			tags = append(tags, "evento-alfa-pregunta")
		}
		if strings.Contains(diff, "SpeakerAlfa") || strings.Contains(diff, "GetNextAlfaQuestion") {
			tags = append(tags, "cola-alfa")
		}
	}

	// Detectar taxonomía de comandos de Quine
	if strings.Contains(filePath, "quine/") {
		if strings.Contains(diff, "CommandTaxonomy") || strings.Contains(diff, "AnalyzeCommands") {
			tags = append(tags, "quine-taxonomia-comandos")
		}
		if strings.Contains(diff, "checklist") && strings.Contains(diff, "comandos-ejecutables") {
			tags = append(tags, "quine-checklist-comandos")
		}
		if strings.Contains(diff, "WHY.md") || strings.Contains(diff, "generateWhy") {
			tags = append(tags, "quine-why")
		}
		if strings.Contains(diff, "Register") && strings.Contains(diff, "Connect") && strings.Contains(diff, "integracion") {
			tags = append(tags, "quine-metodos-integracion")
		}
	}

	// Detectar cambios de Charlie
	if strings.Contains(filePath, "charlie/") {
		if strings.Contains(diff, "func ValidateSafeToOperate") {
			tags = append(tags, "charlie-validar-operacion")
		}
		if strings.Contains(diff, "func BuildReport") {
			tags = append(tags, "charlie-report")
		}
		if strings.Contains(diff, "rejectDangerousGit") || strings.Contains(diff, "git reset --hard") {
			tags = append(tags, "charlie-bloqueo-git")
		}
	}

	// Detectar terminal handoff de Flujo
	if strings.Contains(filePath, "nativeagent/agent.go") {
		if strings.Contains(diff, "successfulTerminalHandoff") || strings.Contains(diff, "terminal_handoff") {
			tags = append(tags, "flujo-terminal-handoff")
		}
		if strings.Contains(diff, "groqFailedGenerationResponse") {
			tags = append(tags, "flujo-groq-fallback")
		}
		if strings.Contains(diff, "shellCommandFromTextResponse") {
			tags = append(tags, "flujo-shell-fallback")
		}
	}

	// Detectar ejemplos nuevos
	if strings.Contains(filePath, "examples/03") {
		tags = append(tags, "ejemplo-semantic-flow")
	}

	// Detectar refactor de main.go
	if strings.Contains(filePath, "cmd/") && strings.HasSuffix(filePath, "/main.go") {
		if strings.Contains(diff, "audit") || strings.Contains(diff, "explain") {
			tags = append(tags, "main-multi-modo")
		}
	}

	return tags
}

type Report struct {
	Changes       []Change
	CurrentTag    string
	NextVersion   string
	CommitMessage string
	Changelog     string
}

type PreflightReport struct {
	Branch     string
	BackupPath string
	Ahead      int
	Behind     int
	Changes    []Change
	Blockers   []string
	Warnings   []string
}

type AmendPlan struct {
	Version       string
	Branch        string
	Head          string
	TagCommit     string
	Ahead         int
	Behind        int
	Changes       []Change
	CommitMessage string
	Changelog     string
	Blockers      []string
	Warnings      []string
}

type ReconcilePlan struct {
	State         string
	Decision      string
	Branch        string
	Ahead         int
	Behind        int
	Head          string
	Upstream      string
	HeadMessage   string
	RemoteMessage string
	HeadTags      []string
	RemoteTags    []string
	Changes       []Change
	Stashes       []string
	Blockers      []string
	Warnings      []string
	NextCommands  []string
}

type RepairReleasePlan struct {
	Version       string
	Mode          string
	Branch        string
	BaseRef       string
	BaseCommit    string
	Head          string
	CommitMessage string
	BackupPath    string
	NewHead       string
	Changes       []Change
	Stashes       []string
	Actions       []string
	Blockers      []string
	Warnings      []string
	Applied       bool
}

type PublishDraftPlan struct {
	Mode          string
	Branch        string
	Head          string
	Upstream      string
	RemoteTag     string
	Version       string
	CommitMessage string
	Ahead         int
	Behind        int
	Strategy      string
	TagStrategy   string
	Actions       []string
	Blockers      []string
	Warnings      []string
	Applied       bool
}

type PublishTagPlan struct {
	Mode      string
	Version   string
	LocalTag  string
	RemoteTag string
	Strategy  string
	Actions   []string
	Blockers  []string
	Warnings  []string
	Applied   bool
}

type PublishMainPlan struct {
	Mode          string
	Branch        string
	Head          string
	DraftRemote   string
	MainLocal     string
	MainRemote    string
	Version       string
	CommitMessage string
	Strategy      string
	TagStrategy   string
	Actions       []string
	Blockers      []string
	Warnings      []string
	Applied       bool
}

type fileSnapshot struct {
	Path    string
	Data    []byte
	Mode    os.FileMode
	Deleted bool
}

func ValidateSafeToOperate() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("no se pudo obtener el directorio actual: %v", err)
	}
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		return fmt.Errorf("el directorio actual '%s' no existe", currentDir)
	}
	return nil
}

func ChangeToRepoRoot() error {
	if err := os.Chdir(RepoRoot); err != nil {
		return fmt.Errorf("no se pudo cambiar al directorio del repo: %v", err)
	}
	return nil
}

// ResetToCommit exists only as a guardrail for older callers. Charlie must not
// expose destructive git operations as framework behavior.
func ResetToCommit(commitHash string) (string, error) {
	return "", fmt.Errorf("operacion bloqueada: Charlie no ejecuta git reset --hard; use cherry-pick --no-commit para recuperar cambios")
}

func BackupWorkingTree() (string, error) {
	if err := os.MkdirAll(BackupRoot, 0755); err != nil {
		return "", fmt.Errorf("no se pudo crear directorio de backups: %v", err)
	}

	stamp := time.Now().Format("20060102-150405")
	dest := filepath.Join(BackupRoot, repoName(RepoRoot)+"-"+stamp)
	for suffix := 2; ; suffix++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		dest = filepath.Join(BackupRoot, fmt.Sprintf("%s-%s-%d", repoName(RepoRoot), stamp, suffix))
	}

	if err := copyTree(RepoRoot, dest); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}
	return dest, nil
}

func copyTree(src string, dest string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return os.MkdirAll(dest, 0755)
		}
		if shouldSkipBackup(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info.Mode())
	})
}

func shouldSkipBackup(rel string, entry os.DirEntry) bool {
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	if ShouldIgnore(rel) {
		return true
	}
	return false
}

func copyFile(src string, dest string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func runGit(args ...string) (string, error) {
	if err := rejectDangerousGit(args...); err != nil {
		return "", err
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimRight(string(output), "\n"), nil
}

func runGitControlled(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimRight(string(output), "\n"), nil
}

func rejectDangerousGit(args ...string) error {
	joined := strings.Join(args, " ")
	if len(args) >= 2 && args[0] == "reset" && args[1] == "--hard" {
		return fmt.Errorf("operacion bloqueada: git reset --hard esta prohibido en Charlie")
	}
	if strings.Contains(joined, "push --force") || strings.Contains(joined, "push -f") {
		return fmt.Errorf("operacion bloqueada: git push --force esta prohibido en Charlie")
	}
	return nil
}

func CurrentBranch() (string, error) {
	branch, err := runGit("branch", "--show-current")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(branch) == "" {
		return "", fmt.Errorf("HEAD detached: Charlie requiere branch draft")
	}
	return strings.TrimSpace(branch), nil
}

func HeadCommit() (string, error) {
	return runGit("rev-parse", "--short=12", "HEAD")
}

func FullCommit(ref string) (string, error) {
	return runGit("rev-parse", ref+"^{}")
}

func TagCommit(tag string) (string, error) {
	return runGit("rev-list", "-n", "1", "--abbrev-commit", "--abbrev=12", tag)
}

func RemoteTagCommit(version string) (string, error) {
	output, err := runGit("ls-remote", "--tags", "origin", "refs/tags/"+version)
	if err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return "", nil
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func RemoteBranchCommit(branch string) (string, error) {
	output, err := runGit("ls-remote", "--heads", "origin", branch)
	if err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return "", nil
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func IsAncestor(ancestor string, descendant string) bool {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = RepoRoot
	return cmd.Run() == nil
}

func shortSHA(ref string) string {
	if len(ref) <= 12 {
		return ref
	}
	return ref[:12]
}

func CommitMessage(ref string) (string, error) {
	return runGit("log", "-1", "--format=%s", ref)
}

func TagsAt(ref string) []string {
	output, err := runGit("tag", "--points-at", ref)
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}
	var tags []string
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) != "" {
			tags = append(tags, strings.TrimSpace(line))
		}
	}
	sort.Strings(tags)
	return tags
}

func StashList() []string {
	output, err := runGit("stash", "list")
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}
	var stashes []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			stashes = append(stashes, line)
		}
	}
	return stashes
}

func UpstreamDivergence() (ahead int, behind int, warning string) {
	output, err := runGit("rev-list", "--left-right", "--count", "@{u}...HEAD")
	if err != nil {
		return 0, 0, "sin upstream configurado o no disponible"
	}
	fields := strings.Fields(output)
	if len(fields) != 2 {
		return 0, 0, "no se pudo leer divergencia con upstream"
	}
	left, leftErr := strconv.Atoi(fields[0])
	right, rightErr := strconv.Atoi(fields[1])
	if leftErr != nil || rightErr != nil {
		return 0, 0, "no se pudo parsear divergencia con upstream"
	}
	return right, left, ""
}

func UpstreamRef() string {
	ref, err := runGit("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil || strings.TrimSpace(ref) == "" {
		return "origin/draft"
	}
	return strings.TrimSpace(ref)
}

func UnmergedPaths() ([]string, error) {
	status, err := runGit("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		state := line[:2]
		if strings.ContainsAny(state, "U") || state == "AA" || state == "DD" {
			paths = append(paths, strings.TrimSpace(line[3:]))
		}
	}
	return paths, nil
}

func Preflight() (*PreflightReport, error) {
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}

	backupPath, err := BackupWorkingTree()
	if err != nil {
		return nil, err
	}

	branch, err := CurrentBranch()
	if err != nil {
		return nil, err
	}
	ahead, behind, warning := UpstreamDivergence()
	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}
	unmerged, err := UnmergedPaths()
	if err != nil {
		return nil, err
	}

	report := &PreflightReport{
		Branch:     branch,
		BackupPath: backupPath,
		Ahead:      ahead,
		Behind:     behind,
		Changes:    changes,
	}
	if warning != "" {
		report.Warnings = append(report.Warnings, warning)
	}
	if branch != "draft" {
		report.Blockers = append(report.Blockers, fmt.Sprintf("branch actual %q; Charlie solo versiona en draft", branch))
	}
	if behind > 0 {
		report.Blockers = append(report.Blockers, fmt.Sprintf("draft esta %d commit(s) detras del upstream; reconciliar antes de versionar", behind))
	}
	if len(unmerged) > 0 {
		report.Blockers = append(report.Blockers, "hay conflictos sin resolver: "+strings.Join(unmerged, ", "))
	}
	// Doctor-level health check (v0.1.8+): detects repo corruption that
	// would make the rest of the flow crash with raw git errors. Merged
	// into Preflight blockers so the operator sees one unified view.
	for _, hb := range PreflightHealthBlockers() {
		report.Blockers = append(report.Blockers, "[doctor] "+hb)
	}
	return report, nil
}

func BuildAmendPlan(version string) (*AmendPlan, error) {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}

	branch, err := CurrentBranch()
	if err != nil {
		return nil, err
	}
	head, err := HeadCommit()
	if err != nil {
		return nil, err
	}
	tagCommit, err := TagCommit(version)
	if err != nil {
		return nil, fmt.Errorf("no existe el tag %s: %v", version, err)
	}
	ahead, behind, warning := UpstreamDivergence()
	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}

	plan := &AmendPlan{
		Version:       version,
		Branch:        branch,
		Head:          head,
		TagCommit:     tagCommit,
		Ahead:         ahead,
		Behind:        behind,
		Changes:       changes,
		CommitMessage: GenerateCommitMessage(changes, version),
		Changelog:     GenerateChangelogSection(changes, version, ""),
	}
	if warning != "" {
		plan.Warnings = append(plan.Warnings, warning)
	}
	if branch != "draft" {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("branch actual %q; cambiar a draft fuera de Charlie", branch))
	}
	if behind > 0 {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("draft esta %d commit(s) detras del upstream; no reescribir release hasta reconciliar", behind))
	}
	if head != tagCommit {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("HEAD %s no coincide con %s %s; solo se puede amendar la release si el tag apunta al HEAD", head, version, tagCommit))
	}
	if len(changes) == 0 {
		plan.Warnings = append(plan.Warnings, "no hay cambios pendientes para agregar a la release")
	}
	return plan, nil
}

func BuildReconcileDraftPlan() (*ReconcilePlan, error) {
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}

	branch, err := CurrentBranch()
	if err != nil {
		return nil, err
	}
	head, err := HeadCommit()
	if err != nil {
		return nil, err
	}
	upstreamRef := UpstreamRef()
	upstream, err := runGit("rev-parse", "--short=12", upstreamRef)
	if err != nil {
		upstream = "no-upstream"
	}
	headMessage, _ := CommitMessage("HEAD")
	remoteMessage, _ := CommitMessage(upstreamRef)
	ahead, behind, warning := UpstreamDivergence()
	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}
	unmerged, err := UnmergedPaths()
	if err != nil {
		return nil, err
	}

	plan := &ReconcilePlan{
		Branch:        branch,
		Ahead:         ahead,
		Behind:        behind,
		Head:          head,
		Upstream:      upstream,
		HeadMessage:   headMessage,
		RemoteMessage: remoteMessage,
		HeadTags:      TagsAt("HEAD"),
		RemoteTags:    TagsAt(upstreamRef),
		Changes:       changes,
		Stashes:       StashList(),
	}
	if warning != "" {
		plan.Warnings = append(plan.Warnings, warning)
	}
	if branch != "draft" {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("branch actual %q; reconciliacion solo opera en draft", branch))
	}
	if len(unmerged) > 0 {
		plan.Blockers = append(plan.Blockers, "hay conflictos sin resolver: "+strings.Join(unmerged, ", "))
	}
	if len(plan.Stashes) > 0 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("hay %d stash(es); no usar stash pop/drop automaticamente", len(plan.Stashes)))
	}

	switch {
	case ahead == 0 && behind == 0:
		plan.State = "SINCRONIZADO"
		plan.Decision = "continuar con preflight/status/amend-plan segun el objetivo"
	case ahead > 0 && behind == 0:
		plan.State = "LOCAL_ADELANTE"
		plan.Decision = "no hacer pull; validar release local y publicar solo con comando de publicacion controlada"
		plan.NextCommands = append(plan.NextCommands, "go run ./cmd/charlie amend-plan vVERSION")
	case ahead == 0 && behind > 0:
		plan.State = "REMOTE_ADELANTE"
		plan.Decision = "no usar merge manual; actualizar draft con fast-forward controlado antes de versionar"
		plan.Blockers = append(plan.Blockers, "draft esta detras del upstream; falta comando apply para fast-forward seguro")
	case ahead > 0 && behind > 0:
		plan.State = "DIVERGENCIA"
		plan.Decision = "no hacer force push, pull --rebase, merge commit, reset ni checkout manual"
		if sameReleaseCommitMessage(headMessage, remoteMessage) {
			version := commitMessageVersion(headMessage)
			plan.State = "DIVERGENCIA_RELEASE"
			plan.Decision = "dos commits distintos declaran la misma release; reparar con comando controlado del framework"
			plan.NextCommands = append(plan.NextCommands, fmt.Sprintf("go run ./cmd/charlie repair-release %s --apply", version))
		} else {
			plan.Blockers = append(plan.Blockers, "draft y upstream tienen commits distintos; Charlie no debe escoger force push ni merge manual")
		}
	}

	if len(changes) > 0 && behind > 0 && plan.State != "DIVERGENCIA_RELEASE" {
		plan.Blockers = append(plan.Blockers, "hay cambios locales pendientes mientras draft esta detras/divergido")
	}
	return plan, nil
}

func BuildRepairReleasePlan(version string) (*RepairReleasePlan, error) {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}

	branch, err := CurrentBranch()
	if err != nil {
		return nil, err
	}
	head, err := HeadCommit()
	if err != nil {
		return nil, err
	}
	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}
	unmerged, err := UnmergedPaths()
	if err != nil {
		return nil, err
	}

	upstreamRef := UpstreamRef()
	upstreamCommit, upstreamErr := runGit("rev-parse", "--short=12", upstreamRef)
	upstreamMessage, _ := CommitMessage(upstreamRef)
	headMessage, _ := CommitMessage("HEAD")

	plan := &RepairReleasePlan{
		Version:       version,
		Mode:          "plan",
		Branch:        branch,
		Head:          head,
		Changes:       changes,
		Stashes:       StashList(),
		CommitMessage: upstreamMessage,
		Actions: []string{
			"crear backup liviano",
			"capturar cambios locales actuales sin usar stash",
			"tomar origin/draft como base canonica si declara la misma version",
			"restaurar cambios locales sobre la base canonica",
			"actualizar CHANGELOG.md dentro de la seccion de la version",
			"amendar el commit de la release",
			"mover el tag local de la version al commit reparado",
		},
	}
	if branch != "draft" {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("branch actual %q; repair-release solo opera en draft", branch))
	}
	if len(unmerged) > 0 {
		plan.Blockers = append(plan.Blockers, "hay conflictos sin resolver: "+strings.Join(unmerged, ", "))
	}
	if len(changes) == 0 {
		plan.Blockers = append(plan.Blockers, "no hay cambios locales para incorporar a la release")
	}
	if len(plan.Stashes) > 0 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("hay %d stash(es); repair-release no los aplica ni elimina", len(plan.Stashes)))
	}
	if upstreamErr != nil {
		plan.Blockers = append(plan.Blockers, "no se pudo leer upstream de draft")
		return plan, nil
	}

	if commitMessageVersion(upstreamMessage) == version {
		plan.BaseRef = upstreamRef
		plan.BaseCommit = upstreamCommit
		plan.CommitMessage = upstreamMessage
	} else if commitMessageVersion(headMessage) == version {
		plan.BaseRef = "HEAD"
		plan.BaseCommit = head
		plan.CommitMessage = headMessage
		plan.Warnings = append(plan.Warnings, "upstream no declara la version objetivo; se usara HEAD como base")
	} else {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("ni HEAD ni %s declaran %s", upstreamRef, version))
	}

	if plan.CommitMessage == "" {
		plan.CommitMessage = fmt.Sprintf("chore: commit %s - reparar release", version)
	}
	return plan, nil
}

func ApplyRepairRelease(version string) (*RepairReleasePlan, error) {
	plan, err := BuildRepairReleasePlan(version)
	if err != nil {
		return nil, err
	}
	plan.Mode = "apply"
	if len(plan.Blockers) > 0 {
		return plan, nil
	}

	backupPath, err := BackupWorkingTree()
	if err != nil {
		return nil, err
	}
	plan.BackupPath = backupPath

	snapshots, err := captureSnapshots(plan.Changes)
	if err != nil {
		return nil, err
	}

	if _, err := runGitControlled("reset", "--hard", plan.BaseRef); err != nil {
		return nil, err
	}
	if err := restoreSnapshots(snapshots); err != nil {
		return nil, err
	}

	changesAfterRestore, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}
	plan.Changes = changesAfterRestore

	changelog := GenerateChangelogSection(changesAfterRestore, plan.Version, "")
	if err := mergeChangelogForRelease(plan.Version, changelog); err != nil {
		return nil, err
	}

	paths := changedFilePaths(changesAfterRestore)
	paths = append(paths, "CHANGELOG.md")
	sort.Strings(paths)
	addArgs := append([]string{"add", "--"}, paths...)
	if _, err := runGitControlled(addArgs...); err != nil {
		return nil, err
	}
	if _, err := runGitControlled("commit", "--amend", "-m", plan.CommitMessage); err != nil {
		return nil, err
	}
	if _, err := runGitControlled("tag", "-f", plan.Version); err != nil {
		return nil, err
	}

	newHead, err := HeadCommit()
	if err != nil {
		return nil, err
	}
	plan.NewHead = newHead
	plan.Applied = true
	return plan, nil
}

func BuildPublishDraftPlan() (*PublishDraftPlan, error) {
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}
	branch, err := CurrentBranch()
	if err != nil {
		return nil, err
	}
	head, err := HeadCommit()
	if err != nil {
		return nil, err
	}
	fullHead, err := FullCommit("HEAD")
	if err != nil {
		return nil, err
	}
	upstreamRef := UpstreamRef()
	upstream, _ := runGit("rev-parse", "--short=12", upstreamRef)
	message, _ := CommitMessage("HEAD")
	remoteMessage, _ := CommitMessage(upstreamRef)
	version := commitMessageVersion(message)
	ahead, behind, warning := UpstreamDivergence()
	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}

	plan := &PublishDraftPlan{
		Mode:          "plan",
		Branch:        branch,
		Head:          head,
		Upstream:      upstream,
		Version:       version,
		CommitMessage: message,
		Ahead:         ahead,
		Behind:        behind,
		Actions:       []string{"publicar draft", "publicar tag de la version si apunta al HEAD"},
	}
	if warning != "" {
		plan.Warnings = append(plan.Warnings, warning)
	}
	if branch != "draft" {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("branch actual %q; publish-draft solo opera en draft", branch))
	}
	if len(changes) > 0 {
		plan.Blockers = append(plan.Blockers, "hay cambios locales sin commit; ejecuta repair-release antes de publicar")
	}
	if version == "" {
		plan.Blockers = append(plan.Blockers, "HEAD no es un commit de version Charlie")
	}
	if version != "" {
		tagCommit, err := TagCommit(version)
		if err != nil {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("tag %s no existe localmente", version))
		} else if tagCommit != head {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("tag %s apunta a %s, no al HEAD %s", version, tagCommit, head))
		}
		tagPlan := buildPublishTagPlan(version, fullHead)
		plan.RemoteTag = tagPlan.RemoteTag
		plan.TagStrategy = tagPlan.Strategy
		plan.Warnings = append(plan.Warnings, tagPlan.Warnings...)
		for _, blocker := range tagPlan.Blockers {
			plan.Blockers = append(plan.Blockers, "tag: "+blocker)
		}
	}

	switch {
	case ahead == 0 && behind == 0:
		plan.Strategy = "noop"
		plan.Warnings = append(plan.Warnings, "draft ya esta sincronizado")
	case ahead > 0 && behind == 0:
		plan.Strategy = "push"
	case ahead > 0 && behind > 0 && version != "" && commitMessageVersion(remoteMessage) == version:
		plan.Strategy = "force-with-lease"
		plan.Warnings = append(plan.Warnings, "se publicara una reescritura controlada de la misma version con --force-with-lease")
	default:
		plan.Blockers = append(plan.Blockers, "divergencia no publicable automaticamente")
	}
	return plan, nil
}

func ApplyPublishDraft() (*PublishDraftPlan, error) {
	plan, err := BuildPublishDraftPlan()
	if err != nil {
		return nil, err
	}
	plan.Mode = "apply"
	if len(plan.Blockers) > 0 {
		return plan, nil
	}
	switch plan.Strategy {
	case "noop":
		// Branch already published; still publish/repair the tag below.
	case "push":
		if _, err := runGitControlled("push", "origin", "draft"); err != nil {
			return nil, err
		}
	case "force-with-lease":
		if _, err := runGitControlled("push", "--force-with-lease", "origin", "draft"); err != nil {
			return nil, err
		}
	default:
		plan.Blockers = append(plan.Blockers, "estrategia de publicacion desconocida")
		return plan, nil
	}
	if plan.Version != "" {
		tagPlan, err := ApplyPublishTag(plan.Version)
		if err != nil {
			return nil, err
		}
		plan.TagStrategy = tagPlan.Strategy
		plan.RemoteTag = tagPlan.RemoteTag
		plan.Warnings = append(plan.Warnings, tagPlan.Warnings...)
	}
	plan.Applied = true
	return plan, nil
}

func BuildPublishTagPlan(version string) (*PublishTagPlan, error) {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}
	localTag, err := FullCommit(version)
	if err != nil {
		plan := buildPublishTagPlan(version, "")
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("tag local %s no existe", version))
		return plan, nil
	}
	return buildPublishTagPlan(version, localTag), nil
}

func buildPublishTagPlan(version string, localTag string) *PublishTagPlan {
	remoteTag, err := RemoteTagCommit(version)
	plan := &PublishTagPlan{
		Mode:      "plan",
		Version:   version,
		LocalTag:  localTag,
		RemoteTag: remoteTag,
		Actions:   []string{"publicar tag de version"},
	}
	if err != nil {
		plan.Blockers = append(plan.Blockers, "no se pudo leer tag remoto: "+err.Error())
		return plan
	}
	switch {
	case localTag == "":
		plan.Blockers = append(plan.Blockers, "tag local vacio")
	case remoteTag == "":
		plan.Strategy = "push-tag"
	case remoteTag == localTag:
		plan.Strategy = "noop"
		plan.Warnings = append(plan.Warnings, "tag remoto ya apunta al commit local")
	default:
		plan.Strategy = "force-with-lease-tag"
		plan.Warnings = append(plan.Warnings, "tag remoto existe y se actualizara con --force-with-lease especifico")
	}
	return plan
}

func ApplyPublishTag(version string) (*PublishTagPlan, error) {
	plan, err := BuildPublishTagPlan(version)
	if err != nil {
		return nil, err
	}
	plan.Mode = "apply"
	if len(plan.Blockers) > 0 {
		return plan, nil
	}
	ref := "refs/tags/" + plan.Version
	refspec := ref + ":" + ref
	switch plan.Strategy {
	case "noop":
		plan.Applied = true
		return plan, nil
	case "push-tag":
		if _, err := runGitControlled("push", "origin", refspec); err != nil {
			return nil, err
		}
	case "force-with-lease-tag":
		lease := fmt.Sprintf("--force-with-lease=%s:%s", ref, plan.RemoteTag)
		if _, err := runGitControlled("push", lease, "origin", refspec); err != nil {
			return nil, err
		}
	default:
		plan.Blockers = append(plan.Blockers, "estrategia de tag desconocida")
		return plan, nil
	}
	plan.Applied = true
	return plan, nil
}

func BuildPublishMainPlan() (*PublishMainPlan, error) {
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}
	branch, err := CurrentBranch()
	if err != nil {
		return nil, err
	}
	headFull, err := FullCommit("HEAD")
	if err != nil {
		return nil, err
	}
	headShort := shortSHA(headFull)
	message, _ := CommitMessage("HEAD")
	version := commitMessageVersion(message)
	draftRemote, draftErr := RemoteBranchCommit("draft")
	mainRemote, mainErr := RemoteBranchCommit("main")
	mainLocal, _ := FullCommit("main")
	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}

	plan := &PublishMainPlan{
		Mode:          "plan",
		Branch:        branch,
		Head:          headShort,
		DraftRemote:   draftRemote,
		MainLocal:     mainLocal,
		MainRemote:    mainRemote,
		Version:       version,
		CommitMessage: message,
		Actions: []string{
			"verificar que draft remoto ya apunta al HEAD",
			"mover main local al mismo commit que draft",
			"publicar main remoto desde draft",
			"verificar/publicar tag de la version",
		},
	}
	if branch != "draft" {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("branch actual %q; publish-main solo opera desde draft", branch))
	}
	if len(changes) > 0 {
		plan.Blockers = append(plan.Blockers, "hay cambios locales sin commit; repara/publica draft antes de main")
	}
	if version == "" {
		plan.Blockers = append(plan.Blockers, "HEAD no es un commit de version Charlie")
	}
	if draftErr != nil || draftRemote == "" {
		plan.Blockers = append(plan.Blockers, "no se pudo leer origin/draft")
	} else if draftRemote != headFull {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("origin/draft %s no coincide con HEAD %s; ejecuta publish-draft primero", shortSHA(draftRemote), headShort))
	}
	if mainErr != nil || mainRemote == "" {
		plan.Blockers = append(plan.Blockers, "no se pudo leer origin/main")
	}
	if version != "" {
		tagPlan := buildPublishTagPlan(version, headFull)
		plan.TagStrategy = tagPlan.Strategy
		plan.Warnings = append(plan.Warnings, tagPlan.Warnings...)
		for _, blocker := range tagPlan.Blockers {
			plan.Blockers = append(plan.Blockers, "tag: "+blocker)
		}
	}

	switch {
	case mainRemote == "":
		// Already blocked above.
	case mainRemote == headFull:
		plan.Strategy = "noop"
		plan.Warnings = append(plan.Warnings, "origin/main ya apunta al mismo commit que draft")
	case IsAncestor(mainRemote, headFull):
		plan.Strategy = "push"
	default:
		remoteMessage, err := CommitMessage(mainRemote)
		if err != nil {
			remoteMessage, _ = CommitMessage("origin/main")
		}
		if version != "" && commitMessageVersion(remoteMessage) == version {
			plan.Strategy = "force-with-lease-main"
			plan.Warnings = append(plan.Warnings, "origin/main declara la misma version con otro hash; se actualizara con --force-with-lease")
		} else {
			plan.Blockers = append(plan.Blockers, "origin/main no es ancestro de draft ni declara la misma version")
		}
	}
	return plan, nil
}

func ApplyPublishMain() (*PublishMainPlan, error) {
	plan, err := BuildPublishMainPlan()
	if err != nil {
		return nil, err
	}
	plan.Mode = "apply"
	if len(plan.Blockers) > 0 {
		return plan, nil
	}
	if _, err := runGitControlled("branch", "-f", "main", "HEAD"); err != nil {
		return nil, err
	}
	switch plan.Strategy {
	case "noop":
		// Remote main already points at draft.
	case "push":
		if _, err := runGitControlled("push", "origin", "refs/heads/draft:refs/heads/main"); err != nil {
			return nil, err
		}
	case "force-with-lease-main":
		lease := fmt.Sprintf("--force-with-lease=refs/heads/main:%s", plan.MainRemote)
		if _, err := runGitControlled("push", lease, "origin", "refs/heads/draft:refs/heads/main"); err != nil {
			return nil, err
		}
	default:
		plan.Blockers = append(plan.Blockers, "estrategia de main desconocida")
		return plan, nil
	}
	if plan.Version != "" {
		tagPlan, err := ApplyPublishTag(plan.Version)
		if err != nil {
			return nil, err
		}
		plan.TagStrategy = tagPlan.Strategy
		plan.Warnings = append(plan.Warnings, tagPlan.Warnings...)
	}
	plan.Applied = true
	return plan, nil
}

func captureSnapshots(changes []Change) ([]fileSnapshot, error) {
	var snapshots []fileSnapshot
	seen := map[string]bool{}
	for _, change := range changes {
		if seen[change.FilePath] {
			continue
		}
		seen[change.FilePath] = true
		fullPath := filepath.Join(RepoRoot, change.FilePath)
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) || change.IsDeleted {
			snapshots = append(snapshots, fileSnapshot{Path: change.FilePath, Deleted: true})
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, fileSnapshot{
			Path: change.FilePath,
			Data: data,
			Mode: info.Mode(),
		})
	}
	return snapshots, nil
}

func restoreSnapshots(snapshots []fileSnapshot) error {
	for _, snapshot := range snapshots {
		fullPath := filepath.Join(RepoRoot, snapshot.Path)
		if snapshot.Deleted {
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		mode := snapshot.Mode
		if mode == 0 {
			mode = 0644
		}
		if err := os.WriteFile(fullPath, snapshot.Data, mode); err != nil {
			return err
		}
	}
	return nil
}

func changedFilePaths(changes []Change) []string {
	seen := map[string]bool{}
	var paths []string
	for _, change := range changes {
		if seen[change.FilePath] {
			continue
		}
		seen[change.FilePath] = true
		paths = append(paths, change.FilePath)
	}
	return paths
}

func mergeChangelogForRelease(version string, generated string) error {
	path := filepath.Join(RepoRoot, "CHANGELOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	merged := appendGeneratedBulletsToRelease(string(data), version, generated)
	return os.WriteFile(path, []byte(merged), 0644)
}

func appendGeneratedBulletsToRelease(existing string, version string, generated string) string {
	bullets := extractGeneratedBullets(generated)
	if len(bullets) == 0 {
		return existing
	}
	marker := fmt.Sprintf("## [%s]", strings.TrimPrefix(version, "v"))
	start := strings.Index(existing, marker)
	if start < 0 {
		return strings.TrimRight(existing, "\n") + "\n\n" + generated + "\n"
	}
	next := strings.Index(existing[start+len(marker):], "\n## [")
	end := len(existing)
	if next >= 0 {
		end = start + len(marker) + next
	}
	section := existing[start:end]
	insert := "\n### Charlie\n\n" + strings.Join(bullets, "\n") + "\n"
	if strings.Contains(section, "### Charlie") {
		pos := strings.Index(section, "### Charlie")
		nextScope := strings.Index(section[pos+len("### Charlie"):], "\n### ")
		insertAt := len(section)
		if nextScope >= 0 {
			insertAt = pos + len("### Charlie") + nextScope
		}
		section = section[:insertAt] + "\n" + strings.Join(bullets, "\n") + "\n" + section[insertAt:]
	} else {
		section = strings.TrimRight(section, "\n") + insert
	}
	return existing[:start] + section + existing[end:]
}

func extractGeneratedBullets(generated string) []string {
	var bullets []string
	for _, line := range strings.Split(generated, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			bullets = append(bullets, trimmed)
		}
	}
	return bullets
}

func sameReleaseCommitMessage(a string, b string) bool {
	re := regexp.MustCompile(`chore: commit (v[0-9]+(?:\.[0-9]+)+) - `)
	ma := re.FindStringSubmatch(a)
	mb := re.FindStringSubmatch(b)
	return len(ma) == 2 && len(mb) == 2 && ma[1] == mb[1]
}

func commitMessageVersion(message string) string {
	re := regexp.MustCompile(`chore: commit (v[0-9]+(?:\.[0-9]+)+) - `)
	match := re.FindStringSubmatch(message)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func Status() (string, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return "", err
	}
	return runGit("status", "--porcelain")
}

func CheckIfClean() (bool, error) {
	changes, err := ClassifyChanges()
	if err != nil {
		return false, err
	}
	return len(changes) == 0, nil
}

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

func BuildReport() (*Report, error) {
	if err := ValidateSafeToOperate(); err != nil {
		return nil, err
	}
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}

	changes, err := ClassifyChanges()
	if err != nil {
		return nil, err
	}

	tag, err := GetCurrentTag()
	if err != nil {
		return nil, err
	}

	next := NextVersion(tag)
	report := &Report{
		Changes:       changes,
		CurrentTag:    tag,
		NextVersion:   next,
		CommitMessage: GenerateCommitMessage(changes, next),
	}
	report.Changelog = GenerateChangelogSection(changes, next, "")
	return report, nil
}

func ClassifyChanges() ([]Change, error) {
	if err := ChangeToRepoRoot(); err != nil {
		return nil, err
	}

	status, err := runGit("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	var changes []Change
	for _, line := range strings.Split(status, "\n") {
		if strings.TrimSpace(line) == "" || len(line) < 4 {
			continue
		}
		state := line[:2]
		file := strings.TrimSpace(line[3:])
		if strings.Contains(file, " -> ") {
			parts := strings.Split(file, " -> ")
			file = parts[len(parts)-1]
		}
		expanded, err := expandStatusEntry(file, state)
		if err != nil {
			return nil, err
		}
		changes = append(changes, expanded...)
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Type == changes[j].Type {
			return changes[i].FilePath < changes[j].FilePath
		}
		return changes[i].Type < changes[j].Type
	})
	return changes, nil
}

func expandStatusEntry(filePath string, state string) ([]Change, error) {
	if ShouldIgnore(filePath) {
		return nil, nil
	}

	fullPath := filepath.Join(RepoRoot, filePath)
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		var changes []Change
		err := filepath.WalkDir(fullPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(RepoRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if rel != filePath && ShouldIgnore(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if ShouldIgnore(rel) {
				return nil
			}
			change := newChange(rel, state)
			change.IsNew = true
			change.DiffSummary = SummarizeDiff(change)
			changes = append(changes, change)
			return nil
		})
		return changes, err
	}

	change := newChange(filePath, state)
	change.DiffSummary = SummarizeDiff(change)
	return []Change{change}, nil
}

func newChange(filePath string, state string) Change {
	return Change{
		FilePath:   filePath,
		Type:       ClassifyFile(filePath),
		Status:     state,
		IsNew:      strings.Contains(state, "A") || strings.Contains(state, "?"),
		IsDeleted:  strings.Contains(state, "D"),
		IsModified: strings.Contains(state, "M"),
	}
}

func ShouldIgnore(filePath string) bool {
	clean := filepath.ToSlash(filePath)
	base := filepath.Base(clean)
	if base == ".DS_Store" || strings.HasSuffix(base, ".bak") {
		return true
	}
	if base == "charlie" || base == "quine" || base == "paladin" || base == "flujo" || base == "api_rest" || base == "gmail" {
		return true
	}
	ignoredParts := []string{"/temp/", "/bin/"}
	wrapped := "/" + clean
	for _, part := range ignoredParts {
		if strings.Contains(wrapped, part) {
			return true
		}
	}
	// Fall back to .charlieignore patterns (v0.1.8+).
	return matchesCharlieIgnore(filePath)
}

func ClassifyFile(filePath string) ChangeType {
	switch {
	case strings.Contains(filePath, ".github/workflows/"):
		return TypeCI
	case strings.HasSuffix(filePath, ".md") && !strings.Contains(strings.ToUpper(filePath), "CHANGELOG"):
		return TypeDocs
	case strings.HasSuffix(filePath, "_test.go"):
		return TypeTest
	case strings.HasSuffix(filePath, "go.mod"), strings.HasSuffix(filePath, "go.sum"), filePath == ".gitignore", strings.HasSuffix(filePath, ".json"), strings.HasSuffix(filePath, ".env"):
		return TypeChore
	case strings.HasSuffix(filePath, ".go"):
		if looksLikeNewLogic(filePath) {
			return TypeFeat
		}
		return TypeRefactor
	default:
		return TypeChore
	}
}

func looksLikeNewLogic(filePath string) bool {
	data, err := os.ReadFile(filepath.Join(RepoRoot, filePath))
	if err != nil {
		return false
	}
	content := string(data)
	score := 0
	for _, pattern := range []string{"func ", "type ", "interface {", "struct {", "const (", "var ("} {
		if strings.Contains(content, pattern) {
			score++
		}
	}
	return score >= 2
}

func SummarizeDiff(change Change) DiffSummary {
	var content string
	if change.IsNew && !change.IsModified {
		return summarizeUntracked(change.FilePath)
	}

	diff, err := runGit("diff", "--", change.FilePath)
	if err != nil || strings.TrimSpace(diff) == "" {
		diff, _ = runGit("diff", "--cached", "--", change.FilePath)
	}

	// Leer contenido actual para análisis semántico
	if fullPath := filepath.Join(RepoRoot, change.FilePath); change.Type == "docs" || strings.HasSuffix(change.FilePath, ".md") {
		if data, err := os.ReadFile(fullPath); err == nil {
			content = string(data)
		}
	}

	return summarizeDiffText(diff, change.FilePath, change.IsNew, content)
}

func summarizeUntracked(filePath string) DiffSummary {
	fullPath := filepath.Join(RepoRoot, filePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return DiffSummary{IsUntracked: true, ShortSummary: "archivo nuevo sin detalle disponible"}
	}
	if info.IsDir() {
		// Detectar tipo de directorio
		if strings.Contains(filePath, "examples/") {
			return DiffSummary{IsUntracked: true, SemanticTags: []string{"nuevo-ejemplo"}, ShortSummary: "directorio ejemplo nuevo"}
		}
		if strings.Contains(filePath, "cmd/") {
			return DiffSummary{IsUntracked: true, SemanticTags: []string{"nuevo-cmd"}, ShortSummary: "directorio cmd nuevo"}
		}
		return DiffSummary{IsUntracked: true, ShortSummary: "directorio nuevo detectado"}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return DiffSummary{IsUntracked: true, IsBinary: true, ShortSummary: "archivo nuevo binario"}
	}
	content := string(data)
	tags := detectSemanticPatterns(filePath, "+"+content, true, content)
	summary := summarizeDiffText("+"+content, filePath, true, content)
	summary.IsUntracked = true
	if len(tags) > 0 {
		summary.SemanticTags = tags
		summary.ShortSummary = generateSemanticDescription(filePath, summary, "+dato", true)
	}
	if summary.ShortSummary == "cambio detectado" || summary.ShortSummary == "" {
		// Describir por extensión
		lines := strings.Count(content, "\n")
		if strings.HasSuffix(filePath, ".go") {
			if lines > 100 {
				summary.ShortSummary = fmt.Sprintf("modulo nuevo (~%d lineas)", lines)
			} else {
				summary.ShortSummary = fmt.Sprintf("archivo nuevo (~%d lineas)", lines)
			}
		} else if strings.HasSuffix(filePath, ".md") {
			summary.ShortSummary = fmt.Sprintf("documento nuevo (~%d lineas)", lines)
		} else {
			summary.ShortSummary = "archivo nuevo"
		}
	}
	return summary
}

func asAddedDiff(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		lines = append(lines, "+"+line)
	}
	return strings.Join(lines, "\n")
}

func summarizeDiffText(diff string, filePath string, isNew bool, content string) DiffSummary {
	var summary DiffSummary
	funcRe := regexp.MustCompile(`^\+\s*func\s+([A-Za-z0-9_]+|\([^)]+\)\s*[A-Za-z0-9_]+)`)
	typeRe := regexp.MustCompile(`^\+\s*type\s+([A-Za-z0-9_]+)`)

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "Binary files ") {
			summary.IsBinary = true
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			summary.AddedLines++
			if match := funcRe.FindStringSubmatch(line); len(match) > 1 {
				summary.ChangedFuncs = appendUnique(summary.ChangedFuncs, strings.TrimSpace(match[1]))
			}
			if match := typeRe.FindStringSubmatch(line); len(match) > 1 {
				summary.ChangedTypes = appendUnique(summary.ChangedTypes, strings.TrimSpace(match[1]))
			}
		}
		if strings.HasPrefix(line, "-") {
			summary.DeletedLines++
		}
	}

	// Detectar patrones semánticos
	summary.SemanticTags = detectSemanticPatterns(filePath, diff, isNew, content)

	// Generar descripción significativa
	summary.ShortSummary = generateSemanticDescription(filePath, summary, diff, isNew)

	return summary
}

// generateSemanticDescription crea una descripción comprensible del cambio
func generateSemanticDescription(filePath string, summary DiffSummary, diff string, isNew bool) string {
	// Si hay etiquetas semánticas, usarlas para describir
	if len(summary.SemanticTags) > 0 {
		descParts := []string{}

		// Categorizar etiquetas en grupos significativos
		apiPaladin := []string{}
		flujoEventos := []string{}
		quine := []string{}
		charlie := []string{}
		flujo := []string{}

		for _, tag := range summary.SemanticTags {
			switch {
			case strings.HasPrefix(tag, "api-"):
				apiPaladin = append(apiPaladin, strings.TrimPrefix(tag, "api-"))
			case strings.HasPrefix(tag, "evento-"):
				flujoEventos = append(flujoEventos, strings.TrimPrefix(tag, "evento-"))
			case strings.HasPrefix(tag, "cola-") || tag == "cola-alfa":
				flujoEventos = append(flujoEventos, tag)
			case strings.HasPrefix(tag, "quine-") || tag == "quine-why" || tag == "quine-metodos-integracion":
				quine = append(quine, strings.TrimPrefix(tag, "quine-"))
			case strings.HasPrefix(tag, "charlie-"):
				charlie = append(charlie, strings.TrimPrefix(tag, "charlie-"))
			case strings.HasPrefix(tag, "flujo-") || tag == "paladin-audit" || tag == "paladin-explain" || tag == "paladin-client" || tag == "paladin-server":
				flujo = append(flujo, strings.TrimPrefix(tag, "flujo-"))
			}
		}

		if len(apiPaladin) > 0 {
			descParts = append(descParts, "API semantica: "+strings.Join(apiPaladin, ", "))
		}
		if len(flujoEventos) > 0 {
			descParts = append(descParts, "Eventos: "+strings.Join(flujoEventos, ", "))
		}
		if len(quine) > 0 {
			descParts = append(descParts, "Quine: "+strings.Join(quine, ", "))
		}
		if len(charlie) > 0 {
			descParts = append(descParts, "Charlie: "+strings.Join(charlie, ", "))
		}
		if len(flujo) > 0 {
			descParts = append(descParts, strings.Join(flujo, ", "))
		}

		if len(descParts) > 0 {
			return strings.Join(descParts, " | ")
		}
	}

	// Fallback: descripción basada en funciones y tipos
	if len(summary.ChangedFuncs) > 0 || len(summary.ChangedTypes) > 0 {
		parts := []string{}
		if len(summary.ChangedFuncs) > 0 {
			funcs := limitStrings(summary.ChangedFuncs, 4)
			parts = append(parts, "funciones: "+strings.Join(funcs, ", "))
		}
		if len(summary.ChangedTypes) > 0 {
			types := limitStrings(summary.ChangedTypes, 3)
			parts = append(parts, "tipos: "+strings.Join(types, ", "))
		}
		return strings.Join(parts, " | ")
	}

	// Fallback: descripción genérica basada en líneas
	parts := []string{}
	if summary.AddedLines > 0 {
		parts = append(parts, fmt.Sprintf("+%d", summary.AddedLines))
	}
	if summary.DeletedLines > 0 {
		parts = append(parts, fmt.Sprintf("-%d", summary.DeletedLines))
	}
	if isNew {
		return "archivo nuevo"
	}
	if len(parts) > 0 {
		return strings.Join(parts, " / ")
	}
	return "cambio detectado"
}

func appendUnique(items []string, next string) []string {
	for _, item := range items {
		if item == next {
			return items
		}
	}
	return append(items, next)
}

func limitStrings(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return items[:max]
}

func GetScope(filePath string) string {
	frameworks := map[string]string{
		"framework-alfa/":    "alfa",
		"framework-bravo/":   "bravo",
		"framework-charlie/": "charlie",
		"framework-echo/":    "echo",
		"framework-excel/":   "excel",
		"framework-paladin/": "paladin",
		"framework-quine/":   "quine",
		"framework-gmail/":   "gmail",
		"remora-flujo/":      "flujo",
	}
	for prefix, scope := range frameworks {
		if strings.HasPrefix(filePath, prefix) {
			return scope
		}
	}
	return ""
}

func GenerateCommitMessage(changes []Change, version string) string {
	return fmt.Sprintf("chore: commit %s - %s", version, summarizeChanges(changes))
}

func summarizeChanges(changes []Change) string {
	if len(changes) == 0 {
		return "repo limpio"
	}

	scopes := map[string]bool{}
	for _, c := range changes {
		if scope := GetScope(c.FilePath); scope != "" {
			scopes[scope] = true
		}
	}
	scopeList := make([]string, 0, len(scopes))
	for scope := range scopes {
		scopeList = append(scopeList, scope)
	}
	sort.Strings(scopeList)

	hasGo := false
	hasDocs := false
	for _, c := range changes {
		hasGo = hasGo || c.Type == TypeFeat || c.Type == TypeRefactor || c.Type == TypeTest
		hasDocs = hasDocs || c.Type == TypeDocs
	}

	if len(scopeList) > 0 {
		prefix := "actualizar"
		if hasGo {
			prefix = "expandir"
		} else if hasDocs {
			prefix = "documentar"
		}
		return fmt.Sprintf("%s %s", prefix, strings.Join(scopeList, ", "))
	}
	return "actualizar proyecto"
}

func NextVersion(currentTag string) string {
	if currentTag == "" || currentTag == "no-tags" {
		return "v0.0.1"
	}

	prefix := ""
	raw := currentTag
	if strings.HasPrefix(raw, "v") {
		prefix = "v"
		raw = strings.TrimPrefix(raw, "v")
	}

	parts := strings.Split(raw, ".")
	if len(parts) == 0 {
		return "v0.0.1"
	}
	last, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "v0.0.1"
	}
	parts[len(parts)-1] = strconv.Itoa(last + 1)
	if prefix == "" {
		prefix = "v"
	}
	return prefix + strings.Join(parts, ".")
}

func GenerateChangelogSection(changes []Change, version string, date string) string {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Agrupar cambios por scope y tipo
	scopeGroups := make(map[string][]Change)
	for _, c := range changes {
		scope := GetScope(c.FilePath)
		if scope == "" {
			scope = "repo"
		}
		scopeGroups[scope] = append(scopeGroups[scope], c)
	}

	// Generar changelog con estructura semántica
	lines := []string{
		fmt.Sprintf("## [%s] - %s", strings.TrimPrefix(version, "v"), date),
		"",
		fmt.Sprintf("> **Release**: %s", summarizeChanges(changes)),
		"",
	}

	// Orden de frameworks para output consistente
	scopeOrder := []string{"charlie", "paladin", "quine", "flujo", "echo", "gmail", "alfa", "bravo", "excel", "repo"}

	for _, scope := range scopeOrder {
		items, ok := scopeGroups[scope]
		if !ok || len(items) == 0 {
			continue
		}

		// Agregar header de sección solo si hay cambios significativos
		lines = append(lines, fmt.Sprintf("### %s", capitalize(scope)))
		lines = append(lines, "")

		// Agrupar por SemanticTags para generar descripciones coherentes
		changesByTag := groupChangesBySemanticTags(items)

		for _, tag := range getOrderedTags(changesByTag) {
			group := changesByTag[tag]
			if tag == "default" {
				// Sin tag semántico: listar archivos con descripción genérica
				for _, c := range group {
					desc := generateFileDescription(c)
					lines = append(lines, fmt.Sprintf("- **%s**: %s", shortFileName(c.FilePath), desc))
				}
			} else {
				// Con tag semántico: describir el grupo
				desc := generateTagDescription(tag, group)
				files := make([]string, 0, len(group))
				for _, c := range group {
					files = append(files, shortFileName(c.FilePath))
				}
				lines = append(lines, fmt.Sprintf("- **%s**: %s (%s)", tag, desc, strings.Join(files, ", ")))
			}
		}
		lines = append(lines, "")
	}

	if len(changes) == 0 {
		lines = append(lines, "- Sin cambios pendientes.")
	}
	return strings.Join(lines, "\n")
}

// groupChangesBySemanticTags agrupa cambios por su tag semántico principal
func groupChangesBySemanticTags(changes []Change) map[string][]Change {
	result := make(map[string][]Change)
	for _, c := range changes {
		tag := "default"
		if len(c.DiffSummary.SemanticTags) > 0 {
			tag = c.DiffSummary.SemanticTags[0] // Usar primer tag como identificador de grupo
		}
		result[tag] = append(result[tag], c)
	}
	return result
}

// getOrderedTags devuelve los tags en orden prioritario
func getOrderedTags(groups map[string][]Change) []string {
	order := []string{
		"api-actor", "api-goal", "api-rule", "api-check", "api-handoff", "api-expect",
		"tipo-semantic-event",
		"paladin-audit", "paladin-explain", "paladin-client", "paladin-server",
		"cola-preguntas", "evento-", "cola-alfa",
		"quine-taxonomia-comandos", "quine-checklist-comandos", "quine-why", "quine-metodos-integracion",
		"flujo-terminal-handoff", "flujo-groq-fallback", "flujo-shell-fallback",
		"charlie-validar-operacion", "charlie-report", "charlie-bloqueo-git",
		"nuevo-ejemplo", "nuevo-cmd", "main-multi-modo",
		"ejemplo-semantic-flow",
		"default",
	}
	result := []string{}
	for _, t := range order {
		if _, ok := groups[t]; ok {
			result = append(result, t)
		}
	}
	// Agregar tags no reconocidos
	for t := range groups {
		found := false
		for _, o := range result {
			if o == t {
				found = true
				break
			}
		}
		if !found {
			result = append(result, t)
		}
	}
	return result
}

func shortFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func generateFileDescription(c Change) string {
	summary := c.DiffSummary.ShortSummary
	if summary == "cambio detectado" || summary == "" {
		if c.Type == "docs" {
			return "documentacion actualizada"
		}
		if c.Type == "test" {
			return "tests agregados"
		}
		if c.Type == "chore" {
			return "configuracion"
		}
		return "codigo modificado"
	}
	return summary
}

func generateTagDescription(tag string, changes []Change) string {
	switch {
	// API semántica de Paladin
	case tag == "api-actor":
		return "nueva API para declarar Actor (quien actúa)"
	case tag == "api-goal":
		return "nueva API para declarar Goal (intención del span)"
	case tag == "api-rule":
		return "nueva API para declarar Rule (regla de negocio)"
	case tag == "api-check":
		return "nueva API para evaluar Check (regla evaluada)"
	case tag == "api-handoff":
		return "nueva API para registrar Handoff (transferencia)"
	case tag == "api-expect":
		return "nueva API para declarar Expect (estado esperado)"
	case tag == "tipo-semantic-event":
		return "tipo SemanticEvent para eventos de negocio"

		// Paladin features
	case tag == "paladin-audit":
		return "comando audit para evaluar implementacion de Paladin"
	case tag == "paladin-explain":
		return "comando explain para traducir trace a lenguaje humano"
	case tag == "paladin-client":
		return "cliente TraceClient para enviar traces al servidor"
	case tag == "paladin-server":
		return "servidor HTTP para recibir traces"

		// Cola de preguntas
	case tag == "cola-preguntas":
		return "sistema de cola de preguntas para control de turnos"
	case strings.HasPrefix(tag, "evento-"):
		name := strings.TrimPrefix(tag, "evento-")
		return fmt.Sprintf("nuevo evento %s", name)
	case tag == "cola-alfa":
		return "funciones para manejar preguntas de Alfa"

		// Quine
	case tag == "quine-taxonomia-comandos":
		return "taxonomia semantica de comandos"
	case tag == "quine-checklist-comandos":
		return "checklist para verificar comandos ejecutables"
	case tag == "quine-why":
		return "generacion automatica de WHY.md"
	case tag == "quine-metodos-integracion":
		return "metodos Register, Connect, Validate para frameworks de integracion"

		// Flujo
	case tag == "flujo-terminal-handoff":
		return "terminal handoff para comandos done/ask-echo"
	case tag == "flujo-groq-fallback":
		return "recuperacion de errores failed_generation de Groq"
	case tag == "flujo-shell-fallback":
		return "extraccion de comandos shell de texto y tool calls"

		// Charlie
	case tag == "charlie-validar-operacion":
		return "validacion de directorio antes de operar"
	case tag == "charlie-report":
		return "generacion de reporte con version y changelog"
	case tag == "charlie-bloqueo-git":
		return "bloqueo de operaciones git peligrosas (reset --hard, push --force)"

		// Otros
	case tag == "nuevo-ejemplo":
		return "nuevo ejemplo"
	case tag == "nuevo-cmd":
		return "nuevo comando"
	case tag == "main-multi-modo":
		return "main.go soporta multiples modos (audit, explain, tree)"
	case tag == "ejemplo-semantic-flow":
		return "ejemplo de flujo semantico"
	}
	return tag
}

func FormatStatus(report *Report) string {
	if len(report.Changes) == 0 {
		return "✅ Repo limpio, no hay cambios pendientes"
	}

	lines := []string{
		"=== CHARLIE ===",
		"",
		fmt.Sprintf("archivos: %d", len(report.Changes)),
		fmt.Sprintf("tag actual: %s", report.CurrentTag),
		fmt.Sprintf("siguiente version: %s", report.NextVersion),
		"",
	}

	groups := map[ChangeType][]Change{}
	for _, c := range report.Changes {
		groups[c.Type] = append(groups[c.Type], c)
	}
	for _, t := range []ChangeType{TypeFeat, TypeRefactor, TypeTest, TypeDocs, TypeChore, TypeCI} {
		items := groups[t]
		if len(items) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %d archivos", t, len(items)))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("  - %s", item.FilePath))
		}
		lines = append(lines, "")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatPreflight(report *PreflightReport) string {
	state := "OK"
	if len(report.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE PREFLIGHT ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("branch: %s", report.Branch),
		fmt.Sprintf("upstream: ahead %d / behind %d", report.Ahead, report.Behind),
		fmt.Sprintf("backup: %s", report.BackupPath),
		fmt.Sprintf("cambios detectados: %d", len(report.Changes)),
		"",
	}
	if len(report.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range report.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(report.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range report.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if len(report.Blockers) > 0 {
		lines = append(lines, "No ejecutes git manual para arreglar esto. Reporta el bloqueo al humano.")
	} else {
		lines = append(lines, "Preflight OK. Puedes continuar con status/changelog/propose.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatProposal(report *Report) string {
	if len(report.Changes) == 0 {
		return "✅ Repo limpio, no hay cambios pendientes"
	}

	lines := []string{
		"=== CHARLIE ===",
		"",
		fmt.Sprintf("archivos: %d", len(report.Changes)),
		fmt.Sprintf("tag actual: %s", report.CurrentTag),
		fmt.Sprintf("version: %s", report.NextVersion),
		"",
		"--- CHANGELOG OBLIGATORIO ---",
		"",
		report.Changelog,
		"",
		"--- PROPUESTA ---",
		"",
		fmt.Sprintf("commit: %s", report.CommitMessage),
		"",
		"Charlie solo propone. El equipo decide si ejecuta git add, git commit, tag y push.",
	}
	return strings.Join(lines, "\n")
}

func FormatAmendPlan(plan *AmendPlan) string {
	state := "OK"
	if len(plan.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE AMEND PLAN ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("version objetivo: %s", plan.Version),
		fmt.Sprintf("branch: %s", plan.Branch),
		fmt.Sprintf("HEAD: %s", plan.Head),
		fmt.Sprintf("%s: %s", plan.Version, plan.TagCommit),
		fmt.Sprintf("upstream: ahead %d / behind %d", plan.Ahead, plan.Behind),
		fmt.Sprintf("cambios pendientes: %d", len(plan.Changes)),
		"",
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range plan.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(plan.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range plan.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if len(plan.Blockers) == 0 && len(plan.Changes) > 0 {
		lines = append(lines,
			"--- CHANGELOG PARA LA RELEASE EXISTENTE ---",
			"",
			plan.Changelog,
			"",
			"--- COMMIT OBJETIVO ---",
			"",
			"commit: "+plan.CommitMessage,
			"",
			"Charlie solo entrega el plan. No ejecutes stash/reset/clean/pull para esta operacion.",
			"La aplicacion debe ser atomica: add de archivos permitidos, commit --amend, tag -f local, push de draft/tag solo con aprobacion humana.",
		)
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "No amendar la release hasta resolver los bloqueos.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatReconcilePlan(plan *ReconcilePlan) string {
	state := "OK"
	if len(plan.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE RECONCILE DRAFT ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("diagnostico: %s", plan.State),
		fmt.Sprintf("decision: %s", plan.Decision),
		fmt.Sprintf("branch: %s", plan.Branch),
		fmt.Sprintf("upstream: ahead %d / behind %d", plan.Ahead, plan.Behind),
		fmt.Sprintf("HEAD: %s", plan.Head),
		fmt.Sprintf("remote: %s", plan.Upstream),
		fmt.Sprintf("HEAD msg: %s", plan.HeadMessage),
		fmt.Sprintf("remote msg: %s", plan.RemoteMessage),
		fmt.Sprintf("HEAD tags: %s", formatStringList(plan.HeadTags)),
		fmt.Sprintf("remote tags: %s", formatStringList(plan.RemoteTags)),
		fmt.Sprintf("cambios pendientes: %d", len(plan.Changes)),
		fmt.Sprintf("stashes: %d", len(plan.Stashes)),
		"",
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range plan.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(plan.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range plan.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if len(plan.NextCommands) > 0 {
		lines = append(lines, "SIGUIENTE COMANDO CHARLIE:")
		for _, command := range plan.NextCommands {
			lines = append(lines, "- "+command)
		}
		lines = append(lines, "")
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "No preguntes A/B ni ejecutes git manual. Reporta esta decision y espera un comando de reparacion del framework.")
	} else if len(plan.NextCommands) > 0 {
		lines = append(lines, "Ejecuta el siguiente comando Charlie sin pedirle al humano que haga Git manual.")
	} else {
		lines = append(lines, "Reconciliacion diagnosticada. Sigue la decision indicada sin preguntar opciones redundantes.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatRepairReleasePlan(plan *RepairReleasePlan) string {
	state := "OK"
	if len(plan.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE REPAIR RELEASE ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("modo: %s", plan.Mode),
		fmt.Sprintf("version: %s", plan.Version),
		fmt.Sprintf("branch: %s", plan.Branch),
		fmt.Sprintf("base canonica: %s (%s)", plan.BaseRef, plan.BaseCommit),
		fmt.Sprintf("HEAD inicial: %s", plan.Head),
		fmt.Sprintf("commit: %s", plan.CommitMessage),
		fmt.Sprintf("cambios locales: %d", len(plan.Changes)),
		fmt.Sprintf("stashes preservados: %d", len(plan.Stashes)),
		"",
	}
	if plan.BackupPath != "" {
		lines = append(lines, "backup: "+plan.BackupPath, "")
	}
	if len(plan.Actions) > 0 {
		lines = append(lines, "ACCIONES:")
		for _, action := range plan.Actions {
			lines = append(lines, "- "+action)
		}
		lines = append(lines, "")
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range plan.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(plan.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range plan.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if plan.Applied {
		lines = append(lines,
			"APLICADO:",
			"- draft fue reconstruido sobre la base canonica",
			"- los cambios locales fueron incorporados al commit de la release",
			"- CHANGELOG.md fue actualizado",
			"- el tag local fue movido al commit reparado",
			fmt.Sprintf("- nuevo HEAD: %s", plan.NewHead),
		)
	} else if len(plan.Blockers) == 0 {
		lines = append(lines, fmt.Sprintf("Para aplicar sin preguntar: go run ./cmd/charlie repair-release %s --apply", plan.Version))
	} else {
		lines = append(lines, "No aplicar repair-release hasta resolver los bloqueos.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatPublishDraftPlan(plan *PublishDraftPlan) string {
	state := "OK"
	if len(plan.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE PUBLISH DRAFT ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("modo: %s", plan.Mode),
		fmt.Sprintf("branch: %s", plan.Branch),
		fmt.Sprintf("HEAD: %s", plan.Head),
		fmt.Sprintf("upstream: %s", plan.Upstream),
		fmt.Sprintf("tag remoto: %s", emptyAsDash(plan.RemoteTag)),
		fmt.Sprintf("version: %s", plan.Version),
		fmt.Sprintf("commit: %s", plan.CommitMessage),
		fmt.Sprintf("upstream: ahead %d / behind %d", plan.Ahead, plan.Behind),
		fmt.Sprintf("estrategia: %s", plan.Strategy),
		fmt.Sprintf("estrategia tag: %s", plan.TagStrategy),
		"",
	}
	if len(plan.Actions) > 0 {
		lines = append(lines, "ACCIONES:")
		for _, action := range plan.Actions {
			lines = append(lines, "- "+action)
		}
		lines = append(lines, "")
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range plan.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(plan.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range plan.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if plan.Applied {
		lines = append(lines, "APLICADO: draft fue publicado segun la estrategia indicada.")
	} else if len(plan.Blockers) == 0 {
		lines = append(lines, "Para publicar sin preguntar: go run ./cmd/charlie publish-draft --apply")
	} else {
		lines = append(lines, "No publicar draft hasta resolver los bloqueos.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatPublishTagPlan(plan *PublishTagPlan) string {
	state := "OK"
	if len(plan.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE PUBLISH TAG ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("modo: %s", plan.Mode),
		fmt.Sprintf("version: %s", plan.Version),
		fmt.Sprintf("tag local: %s", emptyAsDash(plan.LocalTag)),
		fmt.Sprintf("tag remoto: %s", emptyAsDash(plan.RemoteTag)),
		fmt.Sprintf("estrategia: %s", plan.Strategy),
		"",
	}
	if len(plan.Actions) > 0 {
		lines = append(lines, "ACCIONES:")
		for _, action := range plan.Actions {
			lines = append(lines, "- "+action)
		}
		lines = append(lines, "")
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range plan.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(plan.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range plan.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if plan.Applied {
		lines = append(lines, "APLICADO: tag remoto publicado segun la estrategia indicada.")
	} else if len(plan.Blockers) == 0 {
		lines = append(lines, fmt.Sprintf("Para publicar sin preguntar: go run ./cmd/charlie publish-tag %s --apply", plan.Version))
	} else {
		lines = append(lines, "No publicar tag hasta resolver los bloqueos.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func FormatPublishMainPlan(plan *PublishMainPlan) string {
	state := "OK"
	if len(plan.Blockers) > 0 {
		state = "BLOQUEADO"
	}
	lines := []string{
		"=== CHARLIE PUBLISH MAIN ===",
		"",
		fmt.Sprintf("estado: %s", state),
		fmt.Sprintf("modo: %s", plan.Mode),
		fmt.Sprintf("branch: %s", plan.Branch),
		fmt.Sprintf("HEAD/draft: %s", plan.Head),
		fmt.Sprintf("origin/draft: %s", shortSHA(emptyAsDash(plan.DraftRemote))),
		fmt.Sprintf("main local: %s", shortSHA(emptyAsDash(plan.MainLocal))),
		fmt.Sprintf("origin/main: %s", shortSHA(emptyAsDash(plan.MainRemote))),
		fmt.Sprintf("version: %s", plan.Version),
		fmt.Sprintf("commit: %s", plan.CommitMessage),
		fmt.Sprintf("estrategia main: %s", plan.Strategy),
		fmt.Sprintf("estrategia tag: %s", plan.TagStrategy),
		"",
	}
	if len(plan.Actions) > 0 {
		lines = append(lines, "ACCIONES:")
		for _, action := range plan.Actions {
			lines = append(lines, "- "+action)
		}
		lines = append(lines, "")
	}
	if len(plan.Blockers) > 0 {
		lines = append(lines, "BLOQUEOS:")
		for _, blocker := range plan.Blockers {
			lines = append(lines, "- "+blocker)
		}
		lines = append(lines, "")
	}
	if len(plan.Warnings) > 0 {
		lines = append(lines, "ADVERTENCIAS:")
		for _, warning := range plan.Warnings {
			lines = append(lines, "- "+warning)
		}
		lines = append(lines, "")
	}
	if plan.Applied {
		lines = append(lines, "APLICADO: main local y remoto quedaron alineados con draft.")
	} else if len(plan.Blockers) == 0 {
		lines = append(lines, "Para actualizar main sin preguntar: go run ./cmd/charlie publish-main --apply")
	} else {
		lines = append(lines, "No actualizar main hasta resolver los bloqueos.")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func emptyAsDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func formatStringList(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func ValidateReport(report *Report) []string {
	var issues []string
	if len(report.Changes) == 0 {
		return issues
	}
	if !regexp.MustCompile(`^chore: commit v[0-9]+(\.[0-9]+)+ - .+`).MatchString(report.CommitMessage) {
		issues = append(issues, "commit invalido: debe usar 'chore: commit vVERSION - descripcion'")
	}

	// Verificar estructura del changelog (nuevo formato con secciones por scope)
	hasValidStructure := false
	validSectionMarkers := []string{"### Charlie", "### Paladin", "### Quine", "### Flujo", "### Echo", "### Gmail", "### Alfa", "### Bravo", "### Repo", "### Limpieza", "### Scripts", "### Foco", "### Sabio", "### Mecanico", "### Mensajero", "### Hosting", "### Indexa", "### Tareas", "### Auditor", "### Excel"}
	for _, marker := range validSectionMarkers {
		if strings.Contains(report.Changelog, marker) {
			hasValidStructure = true
			break
		}
	}
	// También aceptar el formato antiguo
	if strings.Contains(report.Changelog, "### Cambios por archivo") {
		hasValidStructure = true
	}

	if !hasValidStructure {
		issues = append(issues, "changelog invalido: falta estructura de secciones")
	}

	// Verificar que los cambios están documentados (usar nombre corto para comparar)
	coveredFiles := make(map[string]bool)
	for _, c := range report.Changes {
		shortName := shortFileName(c.FilePath)
		// Verificar si el archivo o alguno de sus componentes aparece en el changelog
		if strings.Contains(report.Changelog, shortName) ||
			strings.Contains(report.Changelog, c.FilePath) ||
			strings.Contains(report.Changelog, filepath.Base(c.FilePath)) {
			coveredFiles[c.FilePath] = true
		}
	}

	// Reportar archivos no cubiertos
	for _, c := range report.Changes {
		if !coveredFiles[c.FilePath] {
			issues = append(issues, fmt.Sprintf("changelog incompleto: falta %s", c.FilePath))
		}
	}

	return issues
}

func FormatValidation(report *Report) string {
	issues := ValidateReport(report)
	if len(issues) == 0 {
		return "✅ Charlie validate: OK"
	}
	lines := []string{"❌ Charlie validate: FAIL", ""}
	for _, issue := range issues {
		lines = append(lines, "- "+issue)
	}
	return strings.Join(lines, "\n")
}

func GetFilesInCommit(commitHash string) ([]string, error) {
	output, err := runGit("ls-tree", "-r", "--name-only", commitHash)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar archivos del commit %s: %v", commitHash, err)
	}
	var files []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func HasUncommittedChanges() (bool, error) {
	clean, err := CheckIfClean()
	return !clean, err
}

func SuggestConsolidateCommits() (string, string, error) {
	output, err := runGit("log", "--oneline", "-6", "--no-merges")
	if err != nil {
		return "", "", err
	}

	var individual []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		msg := parts[1]
		if strings.HasPrefix(msg, "feat(") || strings.HasPrefix(msg, "fix(") || strings.HasPrefix(msg, "docs(") || strings.HasPrefix(msg, "test(") {
			individual = append(individual, parts[0])
		}
	}
	if len(individual) <= 2 {
		return "", "", nil
	}

	return individual[0], "ALERTA: Se detectaron commits separados. Para consolidar, recuperar cambios con git cherry-pick --no-commit y crear un unico commit 'chore: commit vNEXT - descripcion'. No usar git reset --hard.", nil
}

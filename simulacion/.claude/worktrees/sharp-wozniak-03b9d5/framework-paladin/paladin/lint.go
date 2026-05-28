package paladin

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type LintResult struct {
	Root          string
	FilesScanned  int
	Manifests     int
	FlowRuleFiles int
	Findings      []LintFinding
}

type LintFinding struct {
	Level   string
	Code    string
	Path    string
	Message string
}

type lintManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Binary  struct {
		Command string `json:"command"`
	} `json:"binary"`
	Commands      map[string]lintCommand `json:"commands"`
	ExecutionMode string                 `json:"execution_mode"`
	UserInput     struct {
		Supported       bool   `json:"supported"`
		NextQuestionCmd string `json:"next_question_cmd"`
		IngestAnswerCmd string `json:"ingest_answer_cmd"`
	} `json:"user_input"`
	Model struct {
		Provider     string   `json:"provider"`
		Capabilities []string `json:"capabilities"`
	} `json:"model"`
	CapabilitiesSemantic struct {
		Tags           []string `json:"tags"`
		IntentExamples []string `json:"intent_examples"`
		Produces       []string `json:"produces"`
		Requires       []string `json:"requires"`
	} `json:"capabilities_semantic"`
	Capabilities []lintCapability `json:"capabilities"`
}

type lintCommand struct {
	Args     []string          `json:"args"`
	Params   []string          `json:"params"`
	Defaults map[string]string `json:"defaults"`
}

type lintCapability struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Inputs      []string `json:"inputs"`
	Outputs     []string `json:"outputs"`
	Requires    []string `json:"requires"`
	Produces    []string `json:"produces"`
	Execution   string   `json:"execution"`
	Policies    []string `json:"policies"`
}

func LintRepo(root string) (LintResult, error) {
	result := LintResult{Root: root}
	manifestNames, err := lintManifests(root, &result)
	if err != nil {
		return result, err
	}
	if err := lintFlowRules(root, manifestNames, &result); err != nil {
		return result, err
	}
	if err := lintGoArchitecture(root, manifestNames, &result); err != nil {
		return result, err
	}
	if err := lintLocalIntegration(root, &result); err != nil {
		return result, err
	}
	sort.SliceStable(result.Findings, func(i, j int) bool {
		li, lj := findingRank(result.Findings[i].Level), findingRank(result.Findings[j].Level)
		if li != lj {
			return li < lj
		}
		if result.Findings[i].Path != result.Findings[j].Path {
			return result.Findings[i].Path < result.Findings[j].Path
		}
		return result.Findings[i].Code < result.Findings[j].Code
	})
	return result, nil
}

func WriteLint(w io.Writer, result LintResult) {
	fmt.Fprintln(w, "Paladin Lint")
	fmt.Fprintf(w, "Repo: %s\n", result.Root)
	fmt.Fprintf(w, "Go files scanned: %d\n", result.FilesScanned)
	fmt.Fprintf(w, "Manifests: %d\n", result.Manifests)
	fmt.Fprintf(w, "Flow rule files: %d\n\n", result.FlowRuleFiles)
	fmt.Fprintln(w, "Findings")
	if len(result.Findings) == 0 {
		fmt.Fprintln(w, "  - ok: la punta Channel/API/flow/manifests respeta las reglas base para agentes")
		return
	}
	for _, finding := range result.Findings {
		path := finding.Path
		if path == "" {
			path = "."
		}
		fmt.Fprintf(w, "  - [%s] %s %s: %s\n", finding.Level, finding.Code, path, finding.Message)
	}
}

func lintManifests(root string, result *LintResult) (map[string]bool, error) {
	manifestNames := map[string]bool{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return manifestNames, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "framework-") {
			continue
		}
		path := filepath.Join(root, entry.Name(), "framework.manifest.json")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			addLint(result, "fail", "manifest_read", path, err.Error())
			continue
		}
		result.Manifests++
		var manifest lintManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			addLint(result, "fail", "manifest_json", path, err.Error())
			continue
		}
		if manifest.Name == "" {
			addLint(result, "fail", "manifest_name", path, "name vacío; una IA no puede mapear framework-dir a capability")
			continue
		}
		manifestNames[manifest.Name] = true
		if manifest.Version == "" {
			addLint(result, "fail", "manifest_version", path, "version vacío")
		}
		if manifest.Binary.Command == "" {
			addLint(result, "fail", "manifest_binary", path, "binary.command vacío")
		}
		mode := manifest.ExecutionMode
		if mode == "" {
			mode = "sync_chain"
		}
		if mode != "sync_chain" && mode != "async_trigger" {
			addLint(result, "fail", "manifest_execution_mode", path, fmt.Sprintf("execution_mode %q no es sync_chain ni async_trigger", mode))
		}
		if mode == "sync_chain" && manifest.UserInput.Supported {
			if manifest.UserInput.NextQuestionCmd == "" || manifest.Commands == nil || !commandExists(manifest.Commands, manifest.UserInput.NextQuestionCmd) {
				addLint(result, "fail", "manifest_next_question", path, "sync_chain con user_input.supported debe declarar next_question_cmd existente")
			}
			if manifest.UserInput.IngestAnswerCmd == "" || manifest.Commands == nil || !commandExists(manifest.Commands, manifest.UserInput.IngestAnswerCmd) {
				addLint(result, "fail", "manifest_ingest_answer", path, "sync_chain con user_input.supported debe declarar ingest_answer_cmd existente")
			}
			if manifest.UserInput.NextQuestionCmd != "" && manifest.Commands != nil {
				if cmd, ok := manifest.Commands[manifest.UserInput.NextQuestionCmd]; ok && !commandAcceptsSession(cmd) {
					addLint(result, "warn", "sync_chain_next_without_session", path, "next-question no declara conv_id/conversation_id; probar frameworks aislados puede mezclar estado")
				}
			}
			if manifest.UserInput.IngestAnswerCmd != "" && manifest.Commands != nil {
				if cmd, ok := manifest.Commands[manifest.UserInput.IngestAnswerCmd]; ok {
					if !commandAcceptsSession(cmd) {
						addLint(result, "warn", "sync_chain_ingest_without_session", path, "ingest-answer no declara conv_id/conversation_id; la sesión aislada no queda trazable en el framework")
					}
					if !commandHasParamName(cmd, "answer") {
						addLint(result, "fail", "sync_chain_ingest_without_answer", path, "ingest-answer debe declarar param answer para recibir input humano por manifest")
					}
					if !commandHasParamName(cmd, "history") {
						addLint(result, "warn", "sync_chain_ingest_without_history", path, "ingest-answer no declara history; la IA del framework pierde contexto conversacional reciente")
					}
				}
			}
			if strings.TrimSpace(strings.ToLower(manifest.Model.Provider)) == "none" {
				addLint(result, "warn", "sync_chain_without_ai_model", path, "sync_chain con input humano declara model.provider=none; no será una sesión IA conversacional real")
			}
			lintSyncChainSource(filepath.Dir(path), result)
		}
		if len(manifest.CapabilitiesSemantic.Tags) == 0 && len(manifest.CapabilitiesSemantic.IntentExamples) == 0 && len(manifest.CapabilitiesSemantic.Produces) == 0 {
			addLint(result, "warn", "manifest_no_capabilities_semantic", path, "sin capabilities_semantic útil; routing capability-based queda ciego")
		}
		lintManifestCapabilities(path, manifest, result)
	}
	return manifestNames, nil
}

func lintManifestCapabilities(path string, manifest lintManifest, result *LintResult) {
	if len(manifest.Capabilities) == 0 {
		if manifest.ExecutionMode == "" || manifest.ExecutionMode == "sync_chain" {
			addLint(result, "warn", "manifest_no_typed_capabilities", path, "sync_chain sin capabilities typed; el futuro protocolo de equipo no puede validar act/wait/need_help por contrato")
		}
		return
	}
	seen := map[string]bool{}
	for _, cap := range manifest.Capabilities {
		id := strings.TrimSpace(cap.ID)
		if id == "" {
			addLint(result, "fail", "capability_id_empty", path, "capability sin id; el router no puede referenciarla")
			continue
		}
		if seen[id] {
			addLint(result, "fail", "capability_duplicate", path, fmt.Sprintf("capability %q duplicada", id))
		}
		seen[id] = true
		if strings.TrimSpace(cap.Description) == "" {
			addLint(result, "fail", "capability_description_empty", path, fmt.Sprintf("capability %q sin description", id))
		}
		if strings.TrimSpace(cap.Command) == "" {
			addLint(result, "fail", "capability_command_empty", path, fmt.Sprintf("capability %q sin command", id))
		} else if !commandExists(manifest.Commands, cap.Command) {
			addLint(result, "fail", "capability_command_missing", path, fmt.Sprintf("capability %q referencia command inexistente %q", id, cap.Command))
		}
		if strings.TrimSpace(cap.Execution) == "" {
			addLint(result, "fail", "capability_execution_empty", path, fmt.Sprintf("capability %q sin execution; no se puede auditar si es determinística, LLM, SQL, etc.", id))
		}
		if len(cap.Outputs) == 0 && len(cap.Produces) == 0 {
			addLint(result, "fail", "capability_no_outputs", path, fmt.Sprintf("capability %q no declara outputs/produces", id))
		}
		if capabilityUsesMultipleEngines(cap) && !hasPolicy(cap.Policies, "no_silent_fallback") && !hasPolicy(cap.Policies, "explicit_fallback_only") {
			addLint(result, "fail", "capability_fallback_policy_missing", path, fmt.Sprintf("capability %q mezcla engines/sources y no declara no_silent_fallback o explicit_fallback_only", id))
		}
		if capabilityLooksGrounded(cap) && !hasPolicy(cap.Policies, "trace_required") {
			addLint(result, "warn", "capability_trace_policy_missing", path, fmt.Sprintf("capability %q produce respuesta grounded pero no exige trace_required", id))
		}
	}
}

func hasPolicy(policies []string, want string) bool {
	for _, policy := range policies {
		if strings.TrimSpace(policy) == want {
			return true
		}
	}
	return false
}

func capabilityUsesMultipleEngines(cap lintCapability) bool {
	text := strings.ToLower(strings.Join(append(append(append([]string{cap.ID, cap.Description, cap.Execution}, cap.Inputs...), cap.Outputs...), append(cap.Requires, cap.Produces...)...), " "))
	engineCount := 0
	if strings.Contains(text, "sql") || strings.Contains(text, "sqlite") {
		engineCount++
	}
	if strings.Contains(text, "bm25") || strings.Contains(text, "semantic") || strings.Contains(text, "vector") || strings.Contains(text, "rag") {
		engineCount++
	}
	if strings.Contains(text, "llm") {
		engineCount++
	}
	return engineCount >= 2
}

func capabilityLooksGrounded(cap lintCapability) bool {
	text := strings.ToLower(strings.Join(append(append([]string{cap.ID, cap.Description}, cap.Outputs...), cap.Produces...), " "))
	return strings.Contains(text, "grounded") || strings.Contains(text, "answer") || strings.Contains(text, "data.")
}

func commandExists(commands map[string]lintCommand, name string) bool {
	_, ok := commands[name]
	return ok
}

func commandAcceptsSession(cmd lintCommand) bool {
	for _, p := range cmd.Params {
		if p == "conv_id" || p == "conversation_id" {
			return true
		}
	}
	for _, arg := range cmd.Args {
		if strings.Contains(arg, "{params.conv_id}") || strings.Contains(arg, "{params.conversation_id}") {
			return true
		}
	}
	return false
}

func commandHasParamName(cmd lintCommand, name string) bool {
	for _, p := range cmd.Params {
		if p == name {
			return true
		}
	}
	return false
}

func lintSyncChainSource(frameworkDir string, result *LintResult) {
	_ = filepath.WalkDir(frameworkDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if strings.Contains(content, "Perfecto. Para organizar tu día necesito identificar tareas") ||
			strings.Contains(content, "Puedo ayudarte a priorizar deudores") {
			addLint(result, "fail", "sync_chain_hardcoded_reply", path, "sync_chain parece responder con fallback fijo; debe materializar estado/artefacto y exponerlo por next-question")
		}
		return nil
	})
}

func lintFlowRules(root string, manifestNames map[string]bool, result *LintResult) error {
	paths := []string{
		filepath.Join(root, "remora-flujo", "cmd", "api_rest", "flow.rules.json"),
		filepath.Join(root, "profiles", "cobranza-chile", "flow.rules.json"),
	}
	allowedWhen := map[string]bool{
		"frameworks_active_all":          true,
		"frameworks_active_any":          true,
		"user_answer_count_min":          true,
		"user_answer_count_max":          true,
		"user_message_has_resource_type": true,
		"user_intent_any":                true,
		"capability_missing":             true,
	}
	allowedThen := map[string]bool{
		"prepend_speaker":             true,
		"prepend_speaker_provider_of": true,
		"preprocess":                  true,
		"delegate_to_provider_of":     true,
		"note":                        true,
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			addLint(result, "fail", "flow_rules_read", path, err.Error())
			continue
		}
		result.FlowRuleFiles++
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			addLint(result, "fail", "flow_rules_json", path, err.Error())
			continue
		}
		rules, ok := doc["rules"].([]any)
		if !ok {
			addLint(result, "fail", "flow_rules_rules", path, "rules debe ser una lista")
			continue
		}
		for idx, rawRule := range rules {
			rule, ok := rawRule.(map[string]any)
			if !ok {
				addLint(result, "fail", "flow_rule_shape", path, fmt.Sprintf("rule #%d no es objeto", idx+1))
				continue
			}
			id, _ := rule["id"].(string)
			if id == "" {
				id = fmt.Sprintf("#%d", idx+1)
			}
			deprecated, _ := rule["deprecated"].(bool)
			if _, old := rule["conditions"]; old {
				addLint(result, "fail", "flow_rules_old_schema", path, fmt.Sprintf("%s usa conditions; la API actual lee when", id))
			}
			if _, old := rule["actions"]; old {
				addLint(result, "fail", "flow_rules_old_schema", path, fmt.Sprintf("%s usa actions; la API actual lee then", id))
			}
			when, _ := rule["when"].(map[string]any)
			then, _ := rule["then"].(map[string]any)
			for key := range when {
				if !allowedWhen[key] {
					addLint(result, "fail", "flow_rules_ignored_when", path, fmt.Sprintf("%s.when.%s no existe en FlowCondition; Go lo ignora", id, key))
				}
			}
			for key := range then {
				if !allowedThen[key] {
					addLint(result, "fail", "flow_rules_ignored_then", path, fmt.Sprintf("%s.then.%s no existe en FlowAction; Go lo ignora", id, key))
				}
			}
			if !deprecated {
				if speaker, _ := then["prepend_speaker"].(string); speaker != "" {
					level := "fail"
					if !manifestNames[speaker] {
						addLint(result, level, "flow_rules_unknown_speaker", path, fmt.Sprintf("%s prepende %q pero no hay manifest con ese name", id, speaker))
					}
					addLint(result, level, "flow_rules_name_based_routing", path, fmt.Sprintf("%s usa prepend_speaker=%q; ARCHITECTURE.md pide capability-based", id, speaker))
				}
			}
		}
	}
	return nil
}

func lintGoArchitecture(root string, manifestNames map[string]bool, result *LintResult) error {
	corePrefixes := []string{
		filepath.Join(root, "remora-flujo", "cmd", "api_rest"),
		filepath.Join(root, "channel", "adapter"),
		filepath.Join(root, "channel", "internal"),
		filepath.Join(root, "channel", "manifest"),
		filepath.Join(root, "channel", "cmd", "channel"),
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "temp", "node_modules", "vendor", "bin", "static":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		result.FilesScanned++
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			if isUnderAny(path, corePrefixes) || isRelevantFrameworkPath(path) {
				addLint(result, "warn", "go_parse", path, err.Error())
			}
			return nil
		}
		lintCrossFrameworkImports(path, file, result)
		if isUnderAny(path, corePrefixes) {
			lintCoreGoFile(path, fset, file, manifestNames, result)
		}
		return nil
	})
}

func lintLocalIntegration(root string, result *LintResult) error {
	frontendPaths := []string{
		filepath.Join(root, "remora-flujo", "frontends", "frontend-chat", "index.html"),
		filepath.Join(root, "remora-flujo", "cmd", "api_rest", "static", "index.html"),
	}
	for _, frontendPath := range frontendPaths {
		data, err := os.ReadFile(frontendPath)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "window.location.hostname") && strings.Contains(content, ":8084/api/v1") {
			addLint(result, "fail", "frontend_api_base_hardcoded_localhost", frontendPath, "API_BASE fuerza localhost:8084 por hostname; servido desde Docker/Cloud Run debe usar window.location.origin")
		}
		if strings.Contains(content, "function showSingleFwModal()") && strings.Contains(content, "discoveredFrameworks.forEach(fw =>") {
			addLint(result, "fail", "frontend_single_lists_all_frameworks", frontendPath, "Probar Framework itera discoveredFrameworks; debe listar testableFrameworks para separar modo aislado de chain")
		}
		if strings.Contains(content, "currentMode = 'command'") && strings.Contains(content, "createFrameworkCommandSession(selectedSingleFramework)") {
			addLint(result, "fail", "frontend_single_bypasses_standard_session", frontendPath, "Probar Framework usa modo comando paralelo; todos los frameworks testeables deben iniciar conversations-single y normalizarse como sesión")
		}
		if functionBodyContains(content, "testableFrameworks", "execution_mode !== 'async_trigger'") {
			addLint(result, "fail", "frontend_single_hides_async_testable", frontendPath, "testableFrameworks excluye async_trigger; si tiene comandos debe poder probarse como sesión aislada estándar")
		}
	}

	apiPath := filepath.Join(root, "remora-flujo", "cmd", "api_rest", "main.go")
	if data, err := os.ReadFile(apiPath); err == nil {
		content := string(data)
		if strings.Contains(content, `envOr("CHANNEL_BASE_DIR", "/workspace")`) {
			addLint(result, "fail", "api_root_defaults_workspace", apiPath, "api_rest defaulta REMORA_ROOT a /workspace; en local discovery queda ciego y solo aparecen drivers hardcodeados")
		}
		if !strings.Contains(content, `apiBase+"/frameworks/testable"`) {
			addLint(result, "fail", "api_missing_testable_frameworks_endpoint", apiPath, "falta /api/v1/frameworks/testable; frontend single no puede separar frameworks testeables de chainables")
		}
		if !strings.Contains(content, `apiBase+"/frameworks/chainable"`) {
			addLint(result, "fail", "api_missing_chainable_frameworks_endpoint", apiPath, "falta /api/v1/frameworks/chainable; flow builder no puede listar solo frameworks encadenables")
		}
		if strings.Contains(content, "driverRegistry[req.Framework]") && !strings.Contains(content, "createUniversalSingleMessage") {
			addLint(result, "fail", "api_single_conversation_uses_driver_registry", apiPath, "conversations-single valida contra driverRegistry sin wrapper universal; frameworks testeables por manifest quedan invisibles")
		}
		wrapperPath := filepath.Join(root, "remora-flujo", "cmd", "api_rest", "single_wrapper.go")
		if _, err := os.Stat(wrapperPath); err != nil {
			addLint(result, "fail", "api_missing_universal_single_wrapper", apiPath, "falta wrapper universal para adaptar comandos de manifest a sesión conversacional estándar")
		}
	}
	driversPath := filepath.Join(root, "remora-flujo", "cmd", "api_rest", "drivers.go")
	if data, err := os.ReadFile(driversPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"/workspace/framework-`) || strings.Contains(content, `"/frameworks/framework`) {
			addLint(result, "fail", "api_driver_hardcoded_workspace", driversPath, "drivers hardcodean /workspace o /frameworks; en dev local Channel no encuentra binarios y single-mode queda idle")
		}
	}

	dockerfiles, err := findDockerfiles(root)
	if err != nil {
		return err
	}
	for _, path := range dockerfiles {
		data, err := os.ReadFile(path)
		if err != nil {
			addLint(result, "fail", "dockerfile_read", path, err.Error())
			continue
		}
		content := string(data)
		lintDockerfileCopies(root, path, content, result)
		lintDockerfileGoVersion(root, path, content, result)
	}
	return nil
}

func functionBodyContains(content, functionName, needle string) bool {
	marker := "function " + functionName
	start := strings.Index(content, marker)
	if start < 0 {
		return false
	}
	open := strings.Index(content[start:], "{")
	if open < 0 {
		return false
	}
	pos := start + open + 1
	depth := 1
	for pos < len(content) {
		switch content[pos] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.Contains(content[start:pos], needle)
			}
		}
		pos++
	}
	return false
}

func findDockerfiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "temp", "node_modules", "vendor", "bin", "static":
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if name == "Dockerfile" || strings.HasPrefix(name, "Dockerfile.") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func lintDockerfileCopies(root, path, content string, result *LintResult) {
	for lineNo, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "COPY ") {
			continue
		}
		fields := strings.Fields(line)
		sources := dockerCopySources(fields)
		for _, source := range sources {
			source = strings.Trim(source, `"'`)
			if source == "" || strings.HasPrefix(source, "$") || strings.Contains(source, "*") || strings.HasPrefix(source, "--") {
				continue
			}
			full := filepath.Join(root, filepath.Clean(source))
			if _, err := os.Stat(full); err != nil {
				addLint(result, "fail", "dockerfile_copy_missing_source", path, fmt.Sprintf("línea %d: COPY referencia %q pero no existe en el repo", lineNo+1, source))
			}
		}
	}
}

func dockerCopySources(fields []string) []string {
	if len(fields) < 3 || fields[0] != "COPY" {
		return nil
	}
	args := fields[1:]
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		args = args[1:]
	}
	if len(args) < 2 {
		return nil
	}
	return args[:len(args)-1]
}

func lintDockerfileGoVersion(root, path, content string, result *LintResult) {
	imageMajor, imageMinor, ok := dockerGolangVersion(content)
	if !ok {
		return
	}
	for lineNo, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "COPY ") || !strings.Contains(line, "go.mod") {
			continue
		}
		for _, source := range dockerCopySources(strings.Fields(line)) {
			source = strings.Trim(source, `"'`)
			if filepath.Base(source) != "go.mod" {
				continue
			}
			modPath := filepath.Join(root, filepath.Clean(source))
			major, minor, err := readGoModVersion(modPath)
			if err != nil {
				continue
			}
			if versionLess(imageMajor, imageMinor, major, minor) {
				addLint(result, "fail", "dockerfile_go_version_too_old", path, fmt.Sprintf("línea %d: imagen golang:%d.%d es menor que %s declara go %d.%d", lineNo+1, imageMajor, imageMinor, source, major, minor))
			}
		}
	}
}

func dockerGolangVersion(content string) (int, int, bool) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "FROM golang:") {
			continue
		}
		tag := strings.TrimPrefix(line, "FROM golang:")
		tag = strings.Fields(tag)[0]
		tag = strings.Split(tag, "-")[0]
		parts := strings.Split(tag, ".")
		if len(parts) < 2 {
			return 0, 0, false
		}
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		return major, minor, err1 == nil && err2 == nil
	}
	return 0, 0, false
}

func readGoModVersion(path string) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "go ") {
			continue
		}
		parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "go ")), ".")
		if len(parts) < 2 {
			return 0, 0, fmt.Errorf("go version inválida en %s", path)
		}
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
		return major, minor, nil
	}
	return 0, 0, fmt.Errorf("go directive no encontrada en %s", path)
}

func versionLess(aMajor, aMinor, bMajor, bMinor int) bool {
	if aMajor != bMajor {
		return aMajor < bMajor
	}
	return aMinor < bMinor
}

func lintCrossFrameworkImports(path string, file *ast.File, result *LintResult) {
	owner := frameworkOwner(path)
	if owner == "" {
		return
	}
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		importOwner := importFrameworkName(importPath)
		if importOwner == "paladin" {
			continue
		}
		if importOwner != "" && importOwner != owner {
			addLint(result, "fail", "framework_cross_import", path, fmt.Sprintf("framework-%s importa framework-%s; frameworks deben comunicarse por JSON/Channel", owner, importOwner))
		}
	}
}

func lintCoreGoFile(path string, fset *token.FileSet, file *ast.File, manifestNames map[string]bool, result *LintResult) {
	paladinCalls := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			if n.Body != nil {
				start := fset.Position(n.Pos()).Line
				end := fset.Position(n.End()).Line
				lines := end - start + 1
				if lines > 90 {
					addLint(result, "warn", "function_too_large", path, fmt.Sprintf("línea %d: %s tiene %d líneas; una IA contexto-cero necesita funciones <90 líneas en core", start, n.Name.Name, lines))
				}
				complexity := simpleComplexity(n.Body)
				if complexity > 18 {
					addLint(result, "warn", "function_complexity", path, fmt.Sprintf("línea %d: %s complejidad simple=%d; extraer reglas/decisiones haría el flujo más navegable", start, n.Name.Name, complexity))
				}
			}
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				paladinCalls[sel.Sel.Name]++
				if isRawPrint(sel.Sel.Name) {
					line := fset.Position(n.Pos()).Line
					addLint(result, "warn", "raw_print_core", path, fmt.Sprintf("línea %d: uso de %s en core; preferir evento/decisión Paladin o logger estructurado", line, sel.Sel.Name))
				}
			}
		case *ast.BinaryExpr:
			if n.Op.String() == "==" && comparesKnownFrameworkName(n, manifestNames) {
				line := fset.Position(n.Pos()).Line
				addLint(result, "warn", "name_based_code", path, fmt.Sprintf("línea %d: comparación directa contra nombre de framework; preferir capability/provider lookup", line))
			}
		}
		return true
	})
	if strings.HasSuffix(path, filepath.Join("api_rest", "orchestrator.go")) {
		semantic := paladinCalls["Actor"] + paladinCalls["Goal"] + paladinCalls["Event"] + paladinCalls["Rule"] + paladinCalls["Check"] + paladinCalls["Expect"] + paladinCalls["Handoff"] + paladinCalls["Violation"]
		if paladinCalls["NewTrace"] > 0 && semantic == 0 {
			addLint(result, "fail", "paladin_trace_without_semantics", path, "runLoop crea trace pero no declara Actor/Goal/Rule/Check/Expect/Handoff; la IA queda leyendo Vars")
		}
		if paladinCalls["Var"] > 0 && semantic > 0 && paladinCalls["Var"] > semantic*4 {
			addLint(result, "warn", "paladin_var_heavy", path, "demasiadas Var vs semántica; el trace puede volverse ruidoso")
		}
	}
}

func simpleComplexity(body *ast.BlockStmt) int {
	complexity := 1
	ast.Inspect(body, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.CaseClause:
			complexity++
		}
		if expr, ok := node.(*ast.BinaryExpr); ok {
			if expr.Op.String() == "&&" || expr.Op.String() == "||" {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func comparesKnownFrameworkName(expr *ast.BinaryExpr, manifestNames map[string]bool) bool {
	return isKnownFrameworkString(expr.X, manifestNames) || isKnownFrameworkString(expr.Y, manifestNames)
}

func isKnownFrameworkString(expr ast.Expr, manifestNames map[string]bool) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	value := strings.Trim(lit.Value, `"`)
	return manifestNames[value]
}

func isRawPrint(name string) bool {
	switch name {
	case "Print", "Printf", "Println":
		return true
	default:
		return false
	}
}

func frameworkOwner(path string) string {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(part, "framework-") {
			return strings.TrimPrefix(part, "framework-")
		}
	}
	return ""
}

func importFrameworkName(importPath string) string {
	parts := strings.Split(importPath, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "framework-framework-") {
			continue
		}
		if strings.HasPrefix(part, "framework-") {
			return strings.TrimPrefix(part, "framework-")
		}
	}
	return ""
}

func isUnderAny(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func isRelevantFrameworkPath(path string) bool {
	owner := frameworkOwner(path)
	switch owner {
	case "foco", "sabio", "mecanico", "mensajero", "tareas", "hosting", "indexa", "bravo", "gmail":
		return true
	default:
		return false
	}
}

func addLint(result *LintResult, level, code, path, message string) {
	result.Findings = append(result.Findings, LintFinding{
		Level:   level,
		Code:    code,
		Path:    path,
		Message: message,
	})
}

package paladin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLintLocalIntegrationCatchesFrontendHardcodedAPIBase(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "frontends", "frontend-chat", "index.html"), `<script>
const API_BASE = (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
  ? `+"`http://${window.location.hostname}:8084/api/v1`"+`
  : `+"`${window.location.origin}/api/v1`"+`;
</script>`)

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "frontend_api_base_hardcoded_localhost")
}

func TestLintLocalIntegrationCatchesFrontendSingleModeListingAllFrameworks(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "static", "index.html"), `<script>
function showSingleFwModal() {
  discoveredFrameworks.forEach(fw => {});
}
function testableFrameworks() {
  return discoveredFrameworks.filter(fw => fw.testable !== false && fw.execution_mode !== 'async_trigger');
}
function start() {
  currentMode = 'command';
  createFrameworkCommandSession(selectedSingleFramework);
}
</script>`)

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "frontend_single_lists_all_frameworks")
	assertLintFindingCode(t, result, "frontend_single_bypasses_standard_session")
	assertLintFindingCode(t, result, "frontend_single_hides_async_testable")
}

func TestLintLocalIntegrationCatchesMissingFrameworkCapabilityEndpoints(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "main.go"), `package main

func main() {
	r.HandleFunc(apiBase+"/frameworks", srv.listFrameworks)
}
`)

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "api_missing_testable_frameworks_endpoint")
	assertLintFindingCode(t, result, "api_missing_chainable_frameworks_endpoint")
	assertLintFindingCode(t, result, "api_missing_universal_single_wrapper")
}

func TestLintLocalIntegrationCatchesWorkspaceRootDefaults(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "main.go"), `package main

func main() {
	rootDir := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "/workspace"))
	r.HandleFunc(apiBase+"/frameworks/testable", srv.listTestableFrameworks)
	r.HandleFunc(apiBase+"/frameworks/chainable", srv.listChainableFrameworks)
	_ = rootDir
	_ = createUniversalSingleMessage
}
`)
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "single_wrapper.go"), `package main`)

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "api_root_defaults_workspace")
}

func TestLintLocalIntegrationCatchesHardcodedWorkspaceDrivers(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "main.go"), `package main

func main() {
	r.HandleFunc(apiBase+"/frameworks/testable", srv.listTestableFrameworks)
	r.HandleFunc(apiBase+"/frameworks/chainable", srv.listChainableFrameworks)
	_ = createUniversalSingleMessage
}
`)
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "single_wrapper.go"), `package main`)
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "drivers.go"), `package main

func run() {
	ch.ExecuteCommand(ctx, "/frameworks/frameworkecho", []string{"next-question"}, "/workspace/framework-echo")
}
`)

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "api_driver_hardcoded_workspace")
}

func TestLintLocalIntegrationCatchesSingleConversationDriverRegistryCoupling(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "cmd", "flujo_api", "main.go"), `package main

func createSingleConversation() {
	if _, ok := driverRegistry[req.Framework]; !ok {
		return
	}
	r.HandleFunc(apiBase+"/frameworks/testable", srv.listTestableFrameworks)
	r.HandleFunc(apiBase+"/frameworks/chainable", srv.listChainableFrameworks)
}
`)

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "api_single_conversation_uses_driver_registry")
	assertLintFindingCode(t, result, "api_missing_universal_single_wrapper")
}

func TestLintLocalIntegrationCatchesMissingDockerfileCopySource(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, "Dockerfile", "FROM golang:1.24-alpine\nCOPY channel/go.mod channel/go.sum /workspace/channel/\n")
	writeLintTestFile(t, root, filepath.Join("channel", "go.mod"), "module channel\n\ngo 1.21\n")

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "dockerfile_copy_missing_source")
}

func TestLintLocalIntegrationCatchesDockerfileGoVersionTooOld(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, "Dockerfile", "FROM golang:1.23-alpine\nCOPY remora-flujo/go.mod /workspace/remora-flujo/\n")
	writeLintTestFile(t, root, filepath.Join("remora-flujo", "go.mod"), "module remora-flujo\n\ngo 1.24.0\n")

	var result LintResult
	if err := lintLocalIntegration(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "dockerfile_go_version_too_old")
}

func TestLintManifestsCatchesSyncChainWithoutSessionParam(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("framework-demo", "framework.manifest.json"), `{
  "name": "demo",
  "version": "1.0.0",
  "binary": {"command": "./demo"},
  "execution_mode": "sync_chain",
  "commands": {
    "next-question": {"args": ["next-question"], "params": []},
    "ingest-answer": {"args": ["ingest-answer", "--answer", "{params.answer}"], "params": ["answer"]}
  },
  "user_input": {
    "supported": true,
    "next_question_cmd": "next-question",
    "ingest_answer_cmd": "ingest-answer"
  }
}`)

	var result LintResult
	if _, err := lintManifests(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "sync_chain_next_without_session")
	assertLintFindingCode(t, result, "sync_chain_ingest_without_session")
	assertLintFindingCode(t, result, "sync_chain_ingest_without_history")
}

func TestLintManifestsCatchesSyncChainWithoutAIModel(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("framework-demo", "framework.manifest.json"), `{
  "name": "demo",
  "version": "1.0.0",
  "binary": {"command": "./demo"},
  "execution_mode": "sync_chain",
  "model": {"provider": "none"},
  "commands": {
    "next-question": {"args": ["next-question", "--conv-id", "{params.conv_id}"], "params": ["conv_id"]},
    "ingest-answer": {"args": ["ingest-answer", "--conv-id", "{params.conv_id}", "--answer", "{params.answer}", "--history", "{params.history}"], "params": ["conv_id", "answer", "history"]}
  },
  "user_input": {
    "supported": true,
    "next_question_cmd": "next-question",
    "ingest_answer_cmd": "ingest-answer"
  }
}`)

	var result LintResult
	if _, err := lintManifests(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "sync_chain_without_ai_model")
}

func TestLintManifestsCatchesSyncChainHardcodedReply(t *testing.T) {
	root := t.TempDir()
	writeLintTestFile(t, root, filepath.Join("framework-demo", "framework.manifest.json"), `{
  "name": "demo",
  "version": "1.0.0",
  "binary": {"command": "./demo"},
  "execution_mode": "sync_chain",
  "commands": {
    "next-question": {"args": ["next-question", "--conv-id", "{params.conv_id}"], "params": ["conv_id"]},
    "ingest-answer": {"args": ["ingest-answer", "--conv-id", "{params.conv_id}", "--answer", "{params.answer}"], "params": ["conv_id", "answer"]}
  },
  "user_input": {
    "supported": true,
    "next_question_cmd": "next-question",
    "ingest_answer_cmd": "ingest-answer"
  }
}`)
	writeLintTestFile(t, root, filepath.Join("framework-demo", "cmd", "demo", "main.go"), `package main

func reply() string {
	return "Perfecto. Para organizar tu día necesito identificar tareas, urgencias y bloqueos."
}
`)

	var result LintResult
	if _, err := lintManifests(root, &result); err != nil {
		t.Fatal(err)
	}
	assertLintFindingCode(t, result, "sync_chain_hardcoded_reply")
}

func writeLintTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertLintFindingCode(t *testing.T, result LintResult, code string) {
	t.Helper()
	for _, finding := range result.Findings {
		if finding.Code == code {
			return
		}
	}
	t.Fatalf("finding %s not found in %#v", code, result.Findings)
}

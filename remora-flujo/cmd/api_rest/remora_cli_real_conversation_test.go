package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"channel/adapter"

	"github.com/gorilla/mux"
)

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

type realConversationAPIServer struct {
	server       *server
	httpServer   *httptest.Server
	rootDir      string
	token        string
	framework    string
	conversation string
}

type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error,omitempty"`
}

type createSingleConversationData struct {
	Conversation  Conversation `json:"conversation"`
	FirstQuestion Message      `json:"first_question"`
}

type conversationData struct {
	Conversation Conversation `json:"conversation"`
	Messages     []Message    `json:"messages"`
}

type realConversationArtifact struct {
	TestName       string    `json:"test_name"`
	Framework      string    `json:"framework"`
	ConversationID string    `json:"conversation_id"`
	Inputs         []string  `json:"inputs"`
	RecordedAt     time.Time `json:"recorded_at"`
	Messages       []Message `json:"messages"`
}

func TestRealChatTranscriptPingpongGoalThenFile(t *testing.T) {
	if testing.Short() {
		t.Skip("real conversation integration test")
	}
	srv := newRealConversationAPIServer(t, "pingpong")
	runRealChatTranscriptTest(t, srv, []string{
		"Quiero aprender Go",
		"main.go",
		"!exit",
	})
}

func TestRealChatTranscriptPingpongSpecificGoal(t *testing.T) {
	if testing.Short() {
		t.Skip("real conversation integration test")
	}
	srv := newRealConversationAPIServer(t, "pingpong")
	runRealChatTranscriptTest(t, srv, []string{
		"Quiero hacer un programa Hola Mundo en Go en main.go",
		"Quiero arrancar con imprimir hola mundo",
		"!exit",
	})
}

func newRealConversationAPIServer(t *testing.T, framework string) *realConversationAPIServer {
	t.Helper()

	root := resolveRemoraRoot()
	if root == "" {
		t.Fatal("no se pudo resolver REMORA_ROOT para test real")
	}
	if _, err := os.Stat(filepath.Join(root, ".env")); err != nil {
		t.Skip("real conversation test requiere root/.env con credenciales LLM")
	}

	authDB := filepath.Join(t.TempDir(), "remora_auth.db")
	t.Setenv("REMORA_ROOT", root)
	t.Setenv("REMORA_AUTH_DB", authDB)
	t.Setenv("CHANNEL_API_KEY", "test-key-001")

	authStore, err := openAuthStore()
	if err != nil {
		t.Fatalf("openAuthStore: %v", err)
	}
	flowStore, err := openFlowStore(authDB)
	if err != nil {
		t.Fatalf("openFlowStore: %v", err)
	}

	channelURL := ensureChannel(envOr("CHANNEL_URL", "http://localhost:8765"), envOr("CHANNEL_API_KEY", "test-key-001"), root)
	oldRegistry := driverRegistry
	driverRegistry = map[string]FrameworkDriver{}
	t.Cleanup(func() {
		driverRegistry = oldRegistry
	})
	loadedManifests, _ := initDriverRegistry(root, log.New(io.Discard, "", 0))
	if _, ok := loadedManifests[framework]; !ok {
		t.Fatalf("framework %q no descubierto en manifests", framework)
	}

	rulesPath := filepath.Join(root, "remora-flujo", "cmd", "api_rest", "flow.rules.json")
	rules, err := loadFlowRules(rulesPath)
	if err != nil {
		t.Fatalf("loadFlowRules: %v", err)
	}
	srv := &server{
		channel:      adapter.New(channelURL, envOr("CHANNEL_API_KEY", "test-key-001")),
		rules:        rules,
		runtimeInfo:  runtimeInfo{Provider: "real-test", Model: framework, Note: "integration transcript"},
		allManifests: loadedManifests,
		rootDir:      root,
		auth:         authStore,
		flows:        flowStore,
	}

	user, err := authStore.createUser(
		fmt.Sprintf("real-chat-%d@example.com", time.Now().UnixNano()),
		"devin-test-pass-123",
		"Real Chat Test",
		"user",
	)
	if err != nil {
		t.Fatalf("createUser: %v", err)
	}
	if _, _, err := authStore.createBusiness(user.ID, "Real Chat Test Biz", "example.com", "AR"); err != nil {
		t.Fatalf("createBusiness: %v", err)
	}
	token, _, err := authStore.createSession(user.ID)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	r := mux.NewRouter()
	apiBase := "/api/v1"
	r.HandleFunc("/health", srv.health).Methods("GET")
	r.HandleFunc(apiBase+"/frameworks", srv.listFrameworks).Methods("GET")
	r.HandleFunc(apiBase+"/conversations", srv.createConversation).Methods("POST")
	r.HandleFunc(apiBase+"/conversations/{id}", srv.getConversation).Methods("GET")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", srv.getMessages).Methods("GET")
	r.HandleFunc(apiBase+"/conversations/{id}/messages", srv.postMessage).Methods("POST")
	r.HandleFunc(apiBase+"/conversations-single", srv.createSingleConversation).Methods("POST")
	r.HandleFunc(apiBase+"/conversations-single/{id}/messages", srv.postSingleMessage).Methods("POST")

	httpSrv := httptest.NewServer(r)
	t.Cleanup(httpSrv.Close)

	return &realConversationAPIServer{
		server:     srv,
		httpServer: httpSrv,
		rootDir:    root,
		token:      token,
		framework:  framework,
	}
}

func runRealChatTranscriptTest(t *testing.T, srv *realConversationAPIServer, inputs []string) {
	t.Helper()

	bin := buildRemoraCLIBinary(t, srv.rootDir)
	workDir := t.TempDir()
	stdin := strings.Join(inputs, "\n") + "\n"
	env := append(os.Environ(),
		"REMORA_API_URL="+srv.httpServer.URL+"/api/v1",
		"REMORA_API_TOKEN="+srv.token,
	)

	cmd := exec.Command(bin, "chat", "--frameworks", srv.framework)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remora chat failed: %v\n%s", err, out)
	}
	rawOutput := string(out)
	sessionID := strings.TrimSpace(readFileOrFatal(t, filepath.Join(workDir, ".remora_session")))
	if sessionID == "" {
		t.Fatal("remora chat no dejó .remora_session")
	}
	srv.conversation = sessionID
	kind := strings.TrimSpace(readFileOrFatal(t, filepath.Join(workDir, ".remora_session_kind")))
	if kind != "single" {
		t.Fatalf("session kind = %q, want single", kind)
	}
	if strings.Contains(rawOutput, "ingest-answer exit 1") {
		t.Fatalf("chat transcript todavía contiene error de ingest-answer:\n%s", rawOutput)
	}
	if !strings.Contains(rawOutput, "── PINGPONG ──") {
		t.Fatalf("chat transcript no contiene mensajes de pingpong:\n%s", rawOutput)
	}

	messages := fetchConversationMessages(t, srv.httpServer.URL+"/api/v1", srv.token, sessionID)
	if len(messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d: %#v", len(messages), messages)
	}
	if messages[0].Role != "framework" {
		t.Fatalf("first message role = %q, want framework", messages[0].Role)
	}
	if messages[len(messages)-1].Role != "framework" {
		t.Fatalf("last message role = %q, want framework", messages[len(messages)-1].Role)
	}

	artifactDir := persistRealConversationArtifacts(t, srv.rootDir, realConversationArtifact{
		TestName:       t.Name(),
		Framework:      srv.framework,
		ConversationID: sessionID,
		Inputs:         append([]string(nil), inputs...),
		RecordedAt:     time.Now().UTC(),
		Messages:       messages,
	}, stdin, rawOutput)
	t.Logf("real transcript saved in %s", artifactDir)
}

func buildRemoraCLIBinary(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "remora")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/remora")
	cmd.Dir = filepath.Join(root, "remora-cli")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build remora-cli failed: %v\n%s", err, out)
	}
	return bin
}

func fetchConversationMessages(t *testing.T, baseURL, token, sessionID string) []Message {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/conversations/"+sessionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read conversation response: %v", err)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("get conversation status=%d body=%s", resp.StatusCode, raw)
	}
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, raw)
	}
	var data conversationData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode conversation data: %v\n%s", err, env.Data)
	}
	return data.Messages
}

func persistRealConversationArtifacts(t *testing.T, root string, artifact realConversationArtifact, stdin, rawOutput string) string {
	t.Helper()

	dir := filepath.Join(root, "remora-cli", "temp", "test_transcripts", sanitizeTestName(t.Name()))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}

	writeArtifactFile(t, filepath.Join(dir, "stdin.txt"), stdin)
	writeArtifactFile(t, filepath.Join(dir, "terminal_output.ansi.txt"), rawOutput)
	writeArtifactFile(t, filepath.Join(dir, "terminal_output.txt"), stripANSI(rawOutput))
	writeArtifactFile(t, filepath.Join(dir, "conversation.txt"), formatConversation(artifact.Messages))

	rawJSON, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal artifact json: %v", err)
	}
	writeArtifactFile(t, filepath.Join(dir, "artifact.json"), string(rawJSON)+"\n")

	msgJSON, err := json.MarshalIndent(artifact.Messages, "", "  ")
	if err != nil {
		t.Fatalf("marshal messages json: %v", err)
	}
	writeArtifactFile(t, filepath.Join(dir, "messages.json"), string(msgJSON)+"\n")

	return dir
}

func writeArtifactFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write artifact %s: %v", path, err)
	}
}

func formatConversation(messages []Message) string {
	var b bytes.Buffer
	for _, msg := range messages {
		label := strings.ToUpper(msg.Role)
		if msg.Role == "framework" && strings.TrimSpace(msg.Framework) != "" {
			label = strings.ToUpper(msg.Framework)
		}
		b.WriteString(label)
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(msg.Content))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func sanitizeTestName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func stripANSI(s string) string {
	return ansiEscapeRE.ReplaceAllString(s, "")
}

func readFileOrFatal(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

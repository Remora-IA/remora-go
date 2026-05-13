package paladin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PingPongFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type PingPongAction struct {
	Label string         `json:"label,omitempty"`
	Files []PingPongFile `json:"files,omitempty"`
	Args  []string       `json:"args,omitempty"`
}

type PingPongScenario struct {
	Name         string           `json:"name"`
	PingPongDir  string           `json:"pingpong_dir"`
	InitialFiles []PingPongFile   `json:"initial_files,omitempty"`
	Actions      []PingPongAction `json:"actions"`
}

type PingPongStepResult struct {
	Label    string         `json:"label,omitempty"`
	Files    []PingPongFile `json:"files,omitempty"`
	Command  []string       `json:"command,omitempty"`
	Raw      string         `json:"raw,omitempty"`
	Parsed   any            `json:"parsed,omitempty"`
	Message  string         `json:"message,omitempty"`
	Success  *bool          `json:"success,omitempty"`
	ExitCode int            `json:"exit_code"`
}

type PingPongRunResult struct {
	Name             string               `json:"name"`
	PingPongDir      string               `json:"pingpong_dir"`
	WorkspaceDir     string               `json:"workspace_dir"`
	BinaryPath       string               `json:"binary_path"`
	ArtifactDir      string               `json:"artifact_dir"`
	ArtifactPath     string               `json:"artifact_path"`
	ConversationPath string               `json:"conversation_path"`
	TranscriptPath   string               `json:"transcript_path"`
	Steps            []PingPongStepResult `json:"steps"`
	Progress         any                  `json:"progress,omitempty"`
}

func RunPingPongScenario(s PingPongScenario) (*PingPongRunResult, error) {
	if strings.TrimSpace(s.Name) == "" {
		return nil, fmt.Errorf("scenario requiere name")
	}
	if strings.TrimSpace(s.PingPongDir) == "" {
		return nil, fmt.Errorf("scenario requiere pingpong_dir")
	}
	if len(s.Actions) == 0 {
		return nil, fmt.Errorf("scenario requiere actions")
	}

	artifactDir := filepath.Join(s.PingPongDir, "..", "framework-paladin", "temp", "pingpong_eval", slugForPath(s.Name)+"_"+time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, err
	}

	workspaceDir := filepath.Join(artifactDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, err
	}

	binaryPath := filepath.Join(artifactDir, "pingpong")
	if err := buildPingPongBinary(s.PingPongDir, binaryPath); err != nil {
		return nil, err
	}

	result := &PingPongRunResult{
		Name:         s.Name,
		PingPongDir:  s.PingPongDir,
		WorkspaceDir: workspaceDir,
		BinaryPath:   binaryPath,
		ArtifactDir:  artifactDir,
	}

	for _, file := range s.InitialFiles {
		if err := writePingPongFile(workspaceDir, file); err != nil {
			return nil, err
		}
		result.Steps = append(result.Steps, PingPongStepResult{
			Label: "initial_file",
			Files: []PingPongFile{file},
		})
	}

	for _, action := range s.Actions {
		step := PingPongStepResult{Label: action.Label}
		for _, file := range action.Files {
			if err := writePingPongFile(workspaceDir, file); err != nil {
				return nil, err
			}
			step.Files = append(step.Files, file)
		}
		if len(action.Args) > 0 {
			step.Command = append([]string{binaryPath}, action.Args...)
			raw, exitCode, err := runPingPongCommand(binaryPath, workspaceDir, action.Args...)
			step.Raw = raw
			step.ExitCode = exitCode
			if err != nil {
				result.Steps = append(result.Steps, step)
				_ = writePingPongArtifacts(result)
				return result, fmt.Errorf("pingpong %s: %w", strings.Join(action.Args, " "), err)
			}
			step.Parsed, step.Message, step.Success = parsePingPongOutput(raw)
		}
		result.Steps = append(result.Steps, step)
	}

	progressPath := filepath.Join(workspaceDir, "pingpong_progress.json")
	if data, err := os.ReadFile(progressPath); err == nil {
		var progress any
		if json.Unmarshal(data, &progress) == nil {
			result.Progress = progress
		}
	}

	if err := writePingPongArtifacts(result); err != nil {
		return nil, err
	}
	return result, nil
}

func buildPingPongBinary(pingPongDir, binaryPath string) error {
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binaryPath, "./cmd/framework-pingpong")
	cmd.Dir = pingPongDir
	cmd.Env = append(os.Environ(), "DISABLE_TRACES=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build pingpong: %w\n%s", err, out.String())
	}
	return nil
}

func writePingPongFile(workspaceDir string, file PingPongFile) error {
	if strings.TrimSpace(file.Path) == "" {
		return fmt.Errorf("file path vacío")
	}
	target := filepath.Join(workspaceDir, file.Path)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(file.Content), 0644)
}

func runPingPongCommand(binaryPath, workspaceDir string, args ...string) (string, int, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = workspaceDir
	cmd.Env = append(os.Environ(), "DISABLE_TRACES=1")
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		return string(out), exitCode, err
	}
	return string(out), exitCode, nil
}

func parsePingPongOutput(raw string) (any, string, *bool) {
	chunk := extractTrailingJSON(raw)
	if chunk == "" {
		return nil, "", nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(chunk), &parsed); err != nil {
		return nil, "", nil
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return parsed, "", nil
	}
	message, _ := obj["message"].(string)
	var successPtr *bool
	if success, ok := obj["success"].(bool); ok {
		successPtr = &success
	}
	return parsed, message, successPtr
}

func extractTrailingJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] != '{' {
			continue
		}
		chunk := trimmed[i:]
		var parsed any
		if json.Unmarshal([]byte(chunk), &parsed) == nil {
			return chunk
		}
	}
	return ""
}

func writePingPongArtifacts(result *PingPongRunResult) error {
	artifactPath := filepath.Join(result.ArtifactDir, "artifact.json")
	conversationPath := filepath.Join(result.ArtifactDir, "conversation.txt")
	transcriptPath := filepath.Join(result.ArtifactDir, "transcript.txt")

	result.ArtifactPath = artifactPath
	result.ConversationPath = conversationPath
	result.TranscriptPath = transcriptPath

	artifact, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(artifactPath, artifact, 0644); err != nil {
		return err
	}

	if err := os.WriteFile(conversationPath, []byte(buildPingPongConversation(result)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(transcriptPath, []byte(buildPingPongTranscript(result)), 0644); err != nil {
		return err
	}
	return nil
}

func buildPingPongConversation(result *PingPongRunResult) string {
	var b strings.Builder
	for _, step := range result.Steps {
		if len(step.Files) > 0 {
			for _, file := range step.Files {
				fmt.Fprintf(&b, "USER_FILE %s\n%s\n\n", file.Path, file.Content)
			}
		}
		if len(step.Command) > 0 {
			fmt.Fprintf(&b, "RUN %s\n", strings.Join(step.Command[1:], " "))
			if step.Message != "" {
				fmt.Fprintf(&b, "PINGPONG %s\n\n", step.Message)
			} else if strings.TrimSpace(step.Raw) != "" {
				fmt.Fprintf(&b, "PINGPONG %s\n\n", strings.TrimSpace(step.Raw))
			}
		}
	}
	return b.String()
}

func buildPingPongTranscript(result *PingPongRunResult) string {
	var b strings.Builder
	for i, step := range result.Steps {
		fmt.Fprintf(&b, "STEP %d", i+1)
		if step.Label != "" {
			fmt.Fprintf(&b, " [%s]", step.Label)
		}
		b.WriteString("\n")
		for _, file := range step.Files {
			fmt.Fprintf(&b, "WRITE %s\n%s\n", file.Path, file.Content)
		}
		if len(step.Command) > 0 {
			fmt.Fprintf(&b, "CMD %s\n", strings.Join(step.Command, " "))
			if strings.TrimSpace(step.Raw) != "" {
				b.WriteString(strings.TrimSpace(step.Raw))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func slugForPath(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('_')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "scenario"
	}
	return out
}

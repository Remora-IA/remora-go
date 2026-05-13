package pingpong

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LangConfig defines how to compile and run code for a given language.
type LangConfig struct {
	Name     string `json:"name"`
	CheckCmd string `json:"check_cmd"`
	BuildCmd string `json:"build_cmd,omitempty"`
	RunCmd   string `json:"run_cmd,omitempty"`
	FileExt  string `json:"file_ext,omitempty"`
}

// DefaultLangConfigs are built-in language configurations.
var DefaultLangConfigs = map[string]LangConfig{
	"go": {
		Name:     "go",
		CheckCmd: "go test ./...",
		BuildCmd: "go build -o program .",
		RunCmd:   "./program",
		FileExt:  ".go",
	},
	"python": {
		Name:     "python",
		CheckCmd: "python3 -m py_compile {file}",
		RunCmd:   "python3 {file}",
		FileExt:  ".py",
	},
	"javascript": {
		Name:     "javascript",
		CheckCmd: "node --check {file}",
		RunCmd:   "node {file}",
		FileExt:  ".js",
	},
}

// VerifyReport es el resultado de verificar un archivo contra el paso actual.
type VerifyReport struct {
	File        string   `json:"file"`
	CompileOK   bool     `json:"compile_ok"`
	CompileLog  string   `json:"compile_log,omitempty"`
	StepID      int      `json:"step_id"`
	StepText    string   `json:"step_text"`
	FileContent string   `json:"file_content,omitempty"`
	FileHash    string   `json:"file_hash,omitempty"`
	Snippet     []string `json:"snippet,omitempty"`
}

// CompileCheck runs the language's check command on the file.
// Returns compile status + file content for AI judgment.
func CompileCheck(filePath string, lang LangConfig) *VerifyReport {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return &VerifyReport{File: filePath, CompileOK: false, CompileLog: err.Error()}
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return &VerifyReport{File: abs, CompileOK: false, CompileLog: fmt.Sprintf("no se pudo leer: %v", err)}
	}

	h := sha256.Sum256(src)
	rep := &VerifyReport{
		File:        abs,
		FileContent: string(src),
		FileHash:    fmt.Sprintf("%x", h[:8]),
	}

	// Run check command
	cmdStr, cleanup := checkCommandForFile(lang, abs)
	defer cleanup()
	if strings.TrimSpace(cmdStr) == "" {
		// No check command configured — assume OK
		rep.CompileOK = true
		return rep
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = filepath.Dir(abs)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		rep.CompileOK = false
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = runErr.Error()
		}
		// Shorten absolute paths to basenames for readability
		errMsg = strings.ReplaceAll(errMsg, filepath.Dir(abs)+"/", "")
		rep.CompileLog = firstLine(errMsg)
		rep.Snippet = extractSnippet(src, errMsg, 3)
		return rep
	}

	rep.CompileOK = true
	return rep
}

// FileHash returns a SHA-256 hex hash of the file content.
// Used for rewrite detection instead of AST-based declaration tracking.
func FileHash(filePath string) string {
	abs, _ := filepath.Abs(filePath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

// ReadFileContent reads a file and returns its content as a string.
func ReadFileContent(filePath string) (string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func commandWithFile(cmd string, filePath string) string {
	if strings.Contains(cmd, "{file}") {
		return strings.ReplaceAll(cmd, "{file}", shellQuote(filePath))
	}
	return cmd
}

func checkCommandForFile(lang LangConfig, filePath string) (string, func()) {
	if lang.Name != "go" {
		return commandWithFile(lang.CheckCmd, filePath), func() {}
	}
	dir := filepath.Dir(filePath)
	if hasGoModInTree(dir) {
		return commandWithFile(lang.CheckCmd, filePath), func() {}
	}
	out := filepath.Join(os.TempDir(), fmt.Sprintf("pingpong-verify-%d", time.Now().UnixNano()))
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		files = []string{filePath}
	}
	var quoted []string
	for _, file := range files {
		quoted = append(quoted, shellQuote(file))
	}
	return fmt.Sprintf("go build -o %s %s", shellQuote(out), strings.Join(quoted, " ")), func() {
		_ = os.Remove(out)
	}
}

func hasGoModInTree(dir string) bool {
	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

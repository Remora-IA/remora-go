package pingpong

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunReport es el resultado de ejecutar un archivo en sandbox.
type RunReport struct {
	File       string `json:"file"`
	CompileOK  bool   `json:"compile_ok"`
	CompileLog string `json:"compile_log,omitempty"`
	RunOK      bool   `json:"run_ok"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr,omitempty"`
	Expected   string `json:"expected,omitempty"`
	Match      bool   `json:"match"`
	TimedOut   bool   `json:"timed_out,omitempty"`
}

// RunFile compila y ejecuta un archivo en un sandbox temporal.
// stdin se pasa como stdin del proceso; expected (si no vacío) se compara con stdout trimmed.
// Timeout de 10 segundos.
func RunFile(filePath string, stdin string, expected string, lang ...LangConfig) (*RunReport, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", abs, err)
	}

	rep := &RunReport{
		File:     abs,
		Expected: expected,
	}

	// Determine language config
	var lc LangConfig
	if len(lang) > 0 {
		lc = lang[0]
	} else {
		lc = DefaultLangConfigs["go"]
	}

	// Crear sandbox temporal
	sandbox, err := os.MkdirTemp("", "pingpong-run-*")
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear sandbox: %w", err)
	}
	defer os.RemoveAll(sandbox)

	// Copy source to sandbox
	ext := lc.FileExt
	if ext == "" {
		ext = filepath.Ext(abs)
	}
	targetName := "main" + ext
	targetFile := filepath.Join(sandbox, targetName)
	if err := os.WriteFile(targetFile, src, 0644); err != nil {
		return nil, fmt.Errorf("no se pudo copiar archivo: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return runConfigured(ctx, sandbox, targetFile, lc, rep, stdin, expected)
}

// runConfigured executes optional build_cmd and required run_cmd from LangConfig.
func runConfigured(ctx context.Context, sandbox, targetFile string, lc LangConfig, rep *RunReport, stdin, expected string) (*RunReport, error) {
	if lc.Name == "go" {
		goMod := "module sandbox\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(sandbox, "go.mod"), []byte(goMod), 0644); err != nil {
			return nil, fmt.Errorf("no se pudo escribir go.mod: %w", err)
		}
	}

	if strings.TrimSpace(lc.BuildCmd) != "" {
		buildCmd := exec.CommandContext(ctx, "sh", "-c", commandWithFile(lc.BuildCmd, targetFile))
		buildCmd.Dir = sandbox
		buildCmd.Env = sandboxEnv(sandbox)
		var buildStderr bytes.Buffer
		buildCmd.Stderr = &buildStderr

		if err := buildCmd.Run(); err != nil {
			rep.CompileOK = false
			rep.CompileLog = strings.TrimSpace(buildStderr.String())
			if rep.CompileLog == "" {
				rep.CompileLog = err.Error()
			}
			rep.CompileLog = strings.ReplaceAll(rep.CompileLog, sandbox+"/", "")
			return rep, nil
		}
	}
	rep.CompileOK = true

	runCmdText := commandWithFile(lc.RunCmd, targetFile)
	if strings.TrimSpace(runCmdText) == "" {
		return nil, fmt.Errorf("lenguaje %q no tiene run_cmd configurado", lc.Name)
	}
	runCmd := exec.CommandContext(ctx, "sh", "-c", runCmdText)
	runCmd.Dir = sandbox
	runCmd.Env = sandboxEnv(sandbox)
	return execRun(runCmd, rep, stdin, expected, ctx)
}

// execRun executes a command and populates the RunReport.
func execRun(cmd *exec.Cmd, rep *RunReport, stdin, expected string, ctx context.Context) (*RunReport, error) {
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var runStdout, runStderr bytes.Buffer
	cmd.Stdout = &runStdout
	cmd.Stderr = &runStderr

	runErr := cmd.Run()
	rep.Stdout = strings.TrimRight(runStdout.String(), "\n\r ")
	rep.Stderr = strings.TrimSpace(runStderr.String())

	if ctx.Err() == context.DeadlineExceeded {
		rep.TimedOut = true
		rep.RunOK = false
		return rep, nil
	}
	if runErr != nil {
		rep.RunOK = false
		if rep.Stderr == "" {
			rep.Stderr = runErr.Error()
		}
		return rep, nil
	}
	rep.RunOK = true

	if expected != "" {
		expTrim := strings.TrimRight(expected, "\n\r ")
		rep.Match = rep.Stdout == expTrim
	} else {
		rep.Match = true
	}

	return rep, nil
}

// sandboxEnv genera variables de entorno para el sandbox Go.
func sandboxEnv(sandbox string) []string {
	env := []string{
		"GOPROXY=off",
		"GOFLAGS=-mod=mod",
		"GOPATH=" + filepath.Join(sandbox, ".gopath"),
		"GOCACHE=" + filepath.Join(sandbox, ".cache"),
	}
	for _, key := range []string{"PATH", "HOME", "GOROOT", "TMPDIR"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return env
}

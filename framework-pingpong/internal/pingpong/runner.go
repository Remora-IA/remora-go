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

// RunReport es el resultado de ejecutar un archivo Go en sandbox.
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

// RunFile compila y ejecuta un archivo Go en un sandbox temporal.
// stdin se pasa como stdin del proceso; expected (si no vacío) se compara con stdout trimmed.
// Timeout de 10 segundos. Sin acceso a red (GOPROXY=off, GOFLAGS=-mod=mod).
func RunFile(filePath string, stdin string, expected string) (*RunReport, error) {
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

	// Crear sandbox temporal
	sandbox, err := os.MkdirTemp("", "pingpong-run-*")
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear sandbox: %w", err)
	}
	defer os.RemoveAll(sandbox)

	// Escribir go.mod mínimo
	goMod := "module sandbox\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(sandbox, "go.mod"), []byte(goMod), 0644); err != nil {
		return nil, fmt.Errorf("no se pudo escribir go.mod: %w", err)
	}

	// Copiar archivo del usuario como main.go
	targetFile := filepath.Join(sandbox, "main.go")
	if err := os.WriteFile(targetFile, src, 0644); err != nil {
		return nil, fmt.Errorf("no se pudo copiar archivo: %w", err)
	}

	// Timeout de 10 segundos para build+run
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Paso 1: go build
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "program", ".")
	buildCmd.Dir = sandbox
	buildCmd.Env = sandboxEnv(sandbox)
	var buildStdout, buildStderr bytes.Buffer
	buildCmd.Stdout = &buildStdout
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		rep.CompileOK = false
		rep.CompileLog = strings.TrimSpace(buildStderr.String())
		if rep.CompileLog == "" {
			rep.CompileLog = err.Error()
		}
		// Reemplazar path de sandbox por nombre legible
		rep.CompileLog = strings.ReplaceAll(rep.CompileLog, sandbox+"/", "")
		return rep, nil
	}
	rep.CompileOK = true

	// Paso 2: ejecutar el binario
	runCmd := exec.CommandContext(ctx, filepath.Join(sandbox, "program"))
	runCmd.Dir = sandbox
	runCmd.Env = sandboxEnv(sandbox)
	if stdin != "" {
		runCmd.Stdin = strings.NewReader(stdin)
	}
	var runStdout, runStderr bytes.Buffer
	runCmd.Stdout = &runStdout
	runCmd.Stderr = &runStderr

	runErr := runCmd.Run()
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

	// Comparar output si expected dado
	if expected != "" {
		expTrim := strings.TrimRight(expected, "\n\r ")
		rep.Match = rep.Stdout == expTrim
	} else {
		rep.Match = true // sin expected, siempre match
	}

	return rep, nil
}

// sandboxEnv genera variables de entorno para el sandbox:
// - GOPROXY=off (sin red)
// - GOPATH en temp
// - HOME y PATH heredados
func sandboxEnv(sandbox string) []string {
	env := []string{
		"GOPROXY=off",
		"GOFLAGS=-mod=mod",
		"GOPATH=" + filepath.Join(sandbox, ".gopath"),
		"GOCACHE=" + filepath.Join(sandbox, ".cache"),
	}
	// Heredar PATH, HOME, GOROOT
	for _, key := range []string{"PATH", "HOME", "GOROOT", "TMPDIR"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return env
}

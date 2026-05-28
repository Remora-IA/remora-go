package deployer

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	Name        string `json:"name"`
	Cwd         string `json:"cwd,omitempty"`
	Command     string `json:"command"`
	OK          bool   `json:"ok"`
	ExitCode    int    `json:"exit_code"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	Skipped     bool   `json:"skipped,omitempty"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
	Diagnostic  string `json:"diagnostic,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

type Runner interface {
	Run(ctx context.Context, cwd, name string, args ...string) CommandResult
	RunShell(ctx context.Context, cwd, script string) CommandResult
}

type RealRunner struct{}

func (RealRunner) Run(ctx context.Context, cwd, name string, args ...string) CommandResult {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return finishResult(start, cwd, shellQuote(append([]string{name}, args...)), err, out.String())
}

func (RealRunner) RunShell(ctx context.Context, cwd, script string) CommandResult {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "bash", "-lc", script)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return finishResult(start, cwd, script, err, out.String())
}

func finishResult(start time.Time, cwd, command string, err error, output string) CommandResult {
	res := CommandResult{
		Cwd:        cwd,
		Command:    command,
		OK:         err == nil,
		DurationMS: time.Since(start).Milliseconds(),
		Output:     strings.TrimSpace(output),
	}
	if err != nil {
		res.Error = err.Error()
		res.ExitCode = 1
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		}
		diag := DiagnoseText(output + "\n" + err.Error())
		res.Diagnostic = diag.Diagnostic
		res.Remediation = diag.Remediation
	}
	return res
}

func skipped(name, cwd, command, reason string) CommandResult {
	return CommandResult{Name: name, Cwd: cwd, Command: command, OK: false, Skipped: true, Error: reason}
}

func shellQuote(parts []string) string {
	var out []string
	for _, p := range parts {
		if p == "" {
			out = append(out, "''")
			continue
		}
		if strings.ContainsAny(p, " \t\n'\"$`\\") {
			out = append(out, "'"+strings.ReplaceAll(p, "'", "'\\''")+"'")
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

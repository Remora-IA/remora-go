package internal

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// ExecuteCommand ejecuta un comando con timeout (Axiomas 4.4, 5, 12).
// Usa exec.Command con argumentos separados - NUNCA sh -c.
// cwd: working directory ya validado/sanitizado por el caller; "" = default.
func ExecuteCommand(cmd string, args []string, cwd string, timeout time.Duration) (int, string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	execCmd := exec.CommandContext(ctx, cmd, args...)
	if cwd != "" {
		execCmd.Dir = cwd
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	execCmd.Stdout = &stdoutBuf
	execCmd.Stderr = &stderrBuf

	err := execCmd.Run()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return -1, stdout, stderr, context.DeadlineExceeded
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), stdout, stderr, nil
		}
		return -1, stdout, stderr, err
	}
	return 0, stdout, stderr, nil
}

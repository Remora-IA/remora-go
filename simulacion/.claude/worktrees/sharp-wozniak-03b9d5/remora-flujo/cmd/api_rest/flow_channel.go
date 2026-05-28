package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ensureChannel tries to reach an external Channel at channelURL. If it is
// not reachable, it builds (if needed) and starts the Channel binary as a
// child process, then returns the URL where it is listening. This way
// api_rest is fully self-contained and never fails because
// "Channel is not running".
func ensureChannel(channelURL, apiKey, baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pingChannel(ctx, channelURL); err == nil {
		fmt.Printf("  Channel externo OK: %s\n", channelURL)
		return channelURL
	}

	// Channel not reachable — start as subprocess.
	// Find or build the channel binary.
	channelBin := findChannelBinary(baseDir)
	if channelBin == "" {
		fmt.Printf("  WARN: Channel no alcanzable en %s y no encontré el binario channel\n", channelURL)
		fmt.Printf("  Construyendo channel...\n")
		channelBin = buildChannelBinary(baseDir)
		if channelBin == "" {
			fmt.Printf("  ERROR: no pude construir channel. Flujos dependerán de Channel externo.\n")
			return channelURL
		}
	}

	// Pick a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("  WARN: no pude obtener puerto libre: %v\n", err)
		return channelURL
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // release so the child can bind

	embeddedURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	addr := fmt.Sprintf(":%d", port)

	cmd := exec.Command(channelBin,
		"-addr", addr,
		"-base-dir", baseDir,
		"-api-keys", apiKey,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Printf("  ERROR: no pude iniciar channel: %v\n", err)
		return channelURL
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for it to be ready. Do not return the dynamic URL unless the
	// child actually answered health; otherwise every flow fails later with a
	// misleading "Channel no está disponible en 127.0.0.1:<random>" message.
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		select {
		case err := <-done:
			fmt.Printf("  ERROR: Channel hijo terminó antes de estar listo en %s (pid=%d): %v\n",
				embeddedURL, cmd.Process.Pid, err)
			return channelURL
		default:
		}
		ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
		if pingChannel(ctx2, embeddedURL) == nil {
			cancel2()
			fmt.Printf("  Channel hijo iniciado en %s (pid=%d, base_dir=%s)\n",
				embeddedURL, cmd.Process.Pid, baseDir)
			return embeddedURL
		}
		cancel2()
	}

	fmt.Printf("  WARN: Channel hijo no respondió a tiempo en %s (pid=%d)\n",
		embeddedURL, cmd.Process.Pid)
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return channelURL
}

// findChannelBinary looks for an already-built channel binary in common
// locations relative to baseDir.
func findChannelBinary(baseDir string) string {
	candidates := []string{}
	if p := strings.TrimSpace(os.Getenv("CHANNEL_BIN")); p != "" {
		candidates = append(candidates, p)
	}
	candidates = append(candidates,
		"/channel", // Docker image location when api_rest is not launched via entrypoint.
		filepath.Join(baseDir, "channel", "channel"),
		filepath.Join(baseDir, "channel", "cmd", "channel", "channel"),
	)
	// Also check PATH
	if p, err := exec.LookPath("channel"); err == nil {
		candidates = append(candidates, p)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// buildChannelBinary runs `go build` on the channel cmd and returns the
// path to the resulting binary, or "" on failure.
func buildChannelBinary(baseDir string) string {
	channelSrc := filepath.Join(baseDir, "channel", "cmd", "channel")
	if _, err := os.Stat(channelSrc); err != nil {
		return ""
	}
	outBin := filepath.Join(baseDir, "channel", "channel")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", outBin, "./cmd/channel")
	cmd.Dir = filepath.Join(baseDir, "channel")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  go build channel falló: %v\n", err)
		return ""
	}
	return outBin
}

func channelUnavailableMessage(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8765"
	}
	return "Channel no está disponible en " + baseURL + ". Normalmente se inicia automáticamente. Verificá que el base_dir del repo sea correcto."
}

func isChannelUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded")
}

func channelHealthURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	u.Path = "/health"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func pingChannel(ctx context.Context, baseURL string) error {
	healthURL := channelHealthURL(baseURL)
	if healthURL == "" {
		return errors.New("channel URL inválida")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(resp.Status)
	}
	return nil
}

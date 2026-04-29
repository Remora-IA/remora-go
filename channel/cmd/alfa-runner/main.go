// alfa-runner simula una "sesión IA Alfa" que usa el adapter contra Channel
// para leer el árbol que dejó Echo, ejecutar frameworkalfa compile, y leer
// el alfa_spec.json resultante.
//
// El handoff Echo→Alfa NO usa eventos: usa el filesystem que Channel expone.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"channel/adapter"
)

const (
	defaultURL    = "http://localhost:8765"
	defaultAPIKey = "test-key-001"

	alfaDir   = "framework-alfa"
	echoJSON  = "/Users/alcless_a1234_cursor/remora-go/framework-echo/frameworkecho.json"
	specOut   = "/Users/alcless_a1234_cursor/remora-go/framework-alfa/temp/alfa_spec.json"
	specRead  = "framework-alfa/temp/alfa_spec.json"
)

func main() {
	url := flag.String("channel", defaultURL, "Channel URL")
	apiKey := flag.String("key", defaultAPIKey, "Channel API key")
	flag.Parse()

	c := adapter.New(*url, *apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	step("1) Leer árbol Echo que dejó la sesión anterior (handoff vía filesystem)")
	resp, err := c.ReadFile(ctx, "framework-echo/frameworkecho.json")
	mustOk(resp, err)
	fmt.Printf("   → %d bytes leídos del árbol Echo\n", len(resp.Stdout))

	step("2) Asegurar que existe framework-alfa/temp/")
	resp, err = c.ExecuteCommand(ctx, "mkdir", []string{"-p", "temp"}, alfaDir)
	mustOk(resp, err)

	step("3) Compilar Alfa (vía Channel.execute_command → go run frameworkalfa compile)")
	resp, err = c.ExecuteCommand(ctx,
		"go",
		[]string{"run", "./cmd/frameworkalfa", "compile",
			"--echo-tree", echoJSON,
			"--out", specOut,
			"--allow-draft=true"},
		alfaDir,
	)
	mustOk(resp, err)
	fmt.Printf("   stdout:\n%s", resp.Stdout)
	if resp.Stderr != "" {
		fmt.Printf("   stderr:\n%s", resp.Stderr)
	}
	fmt.Printf("   exit_code=%d duration_ms=%d\n", resp.ExitCode, resp.DurationMs)

	step("4) Leer alfa_spec.json producido (vía Channel.read_file)")
	resp, err = c.ReadFile(ctx, specRead)
	mustOk(resp, err)
	fmt.Printf("   → %d bytes de alfa_spec.json\n", len(resp.Stdout))

	step("5) Inspect del spec (vía Channel.execute_command → frameworkalfa inspect)")
	resp, err = c.ExecuteCommand(ctx,
		"go",
		[]string{"run", "./cmd/frameworkalfa", "inspect", "--spec", specOut},
		alfaDir,
	)
	mustOk(resp, err)
	fmt.Printf("%s", resp.Stdout)

	fmt.Println("\n✓ alfa-runner terminó. Channel orquestó: read → execute → read → execute, todo remoto.")
}

func step(msg string) {
	fmt.Printf("\n▶ %s\n", msg)
}

func mustOk(resp *adapter.Response, err error) {
	if err != nil {
		log.Fatalf("adapter error: %v", err)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Channel rechazó: %s\n", resp.Error)
		os.Exit(1)
	}
}

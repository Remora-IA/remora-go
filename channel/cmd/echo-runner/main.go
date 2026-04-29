// echo-runner simula una sesión IA Echo: usa el adapter contra Channel para
// construir una cadena AXIOM→THEORY→TASK→PAIN→OPPORTUNITY validada,
// dejándola lista para que alfa-runner la compile.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"channel/adapter"
)

const (
	defaultURL    = "http://localhost:8765"
	defaultAPIKey = "test-key-001"
	echoDir       = "framework-echo"
	echoJSON      = "framework-echo/frameworkecho.json"
)

type tree struct {
	Nodes map[string]struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status string `json:"status"`
	} `json:"nodes"`
}

func main() {
	url := flag.String("channel", defaultURL, "Channel URL")
	apiKey := flag.String("key", defaultAPIKey, "Channel API key")
	flag.Parse()

	c := adapter.New(*url, *apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	step("1) Leer árbol Echo actual (Channel.read_file)")
	resp := mustExec(c.ReadFile(ctx, echoJSON))
	fmt.Printf("   → %d bytes\n", len(resp.Stdout))
	t := parseTree(resp.Stdout)

	axID := firstByType(t, "AXIOM", "VALIDATED")
	if axID == "" {
		log.Fatal("No hay AXIOM validado en el árbol; aborting")
	}
	fmt.Printf("   axiom raíz: %s\n", axID)

	thID := firstByType(t, "THEORY", "")
	if thID == "" {
		step("2) Crear THEORY (no existía)")
		thID = runEcho(ctx, c, "add-theory",
			"--parent", axID,
			"--title", "Demo: WhatsApp → Excel",
			"--evidence", "El usuario quiere transformar movs de WhatsApp en filas de Excel")
	} else {
		fmt.Printf("\n▶ 2) Usar THEORY existente: %s\n", thID)
	}

	step("3) Validar la THEORY")
	runEchoSimple(ctx, c, "validate", thID, "--answer", "Sí, ese es el flujo")

	step("4) Crear TASK")
	tkID := runEcho(ctx, c, "add-task",
		"--parent", thID,
		"--title", "Copiar manualmente cada transacción de WhatsApp a Excel",
		"--evidence", "Usuario lo hace 3 veces al día")
	runEchoSimple(ctx, c, "validate", tkID, "--answer", "Sí, lo hago así")

	step("5) Crear PAIN")
	pnID := runEcho(ctx, c, "add-pain",
		"--parent", tkID,
		"--title", "Pierdo 1 hora al día copiando movimientos",
		"--evidence", "60 minutos diarios duplicando datos")
	runEchoSimple(ctx, c, "validate", pnID, "--answer", "Sí, es lo más doloroso")

	step("6) Crear OPPORTUNITY")
	opID := runEcho(ctx, c, "add-opportunity",
		"--parent", pnID,
		"--title", "Parser automático WhatsApp → Excel",
		"--evidence", "Eliminaría la copia manual diaria")
	runEchoSimple(ctx, c, "validate", opID, "--answer", "Sí, esa es la solución que busco")

	step("7) Releer árbol final")
	resp = mustExec(c.ReadFile(ctx, echoJSON))
	fmt.Printf("   → %d bytes (tree completo: AXIOM→THEORY→TASK→PAIN→OPPORTUNITY validada)\n", len(resp.Stdout))

	fmt.Println("\n✓ echo-runner terminó. alfa-runner ahora tiene una OPPORTUNITY validada para compilar.")
}

func step(msg string) {
	fmt.Printf("\n▶ %s\n", msg)
}

func runEcho(ctx context.Context, c *adapter.Client, args ...string) string {
	cmdArgs := append([]string{"run", "./cmd/frameworkecho"}, args...)
	resp := mustExec(c.ExecuteCommand(ctx, "go", cmdArgs, echoDir))
	if resp.ExitCode != 0 {
		log.Fatalf("frameworkecho %v falló: exit=%d stderr=%s", args, resp.ExitCode, resp.Stderr)
	}
	id := extractID(resp.Stdout)
	fmt.Printf("   → %s (id=%s)\n", strings.TrimSpace(firstLine(resp.Stdout)), id)
	return id
}

func runEchoSimple(ctx context.Context, c *adapter.Client, args ...string) {
	cmdArgs := append([]string{"run", "./cmd/frameworkecho"}, args...)
	resp := mustExec(c.ExecuteCommand(ctx, "go", cmdArgs, echoDir))
	if resp.ExitCode != 0 {
		log.Fatalf("frameworkecho %v falló: exit=%d stderr=%s", args, resp.ExitCode, resp.Stderr)
	}
	fmt.Printf("   → %s\n", strings.TrimSpace(firstLine(resp.Stdout)))
}

// extractID busca patrón "Creado XX_NNN" en el stdout.
func extractID(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "✓ Creado ") {
			continue
		}
		rest := strings.TrimPrefix(ln, "✓ Creado ")
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func parseTree(data string) *tree {
	var t tree
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		log.Fatalf("parse tree: %v", err)
	}
	return &t
}

func firstByType(t *tree, kind, status string) string {
	for id, n := range t.Nodes {
		if n.Type == kind && (status == "" || n.Status == status) {
			return id
		}
	}
	return ""
}

func mustExec(resp *adapter.Response, err error) *adapter.Response {
	if err != nil {
		log.Fatalf("adapter error: %v", err)
	}
	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Channel rechazó: %s\n", resp.Error)
		os.Exit(1)
	}
	return resp
}

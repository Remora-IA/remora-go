// orchestrator es un binario genérico que:
//  1. Descubre frameworks vía framework.manifest.json
//  2. Calcula cadenas válidas (matching de formats output→input)
//  3. Ejecuta cualquier comando de cualquier framework vía Channel
//
// NO sabe nada específico de Echo, Alfa, Bravo, etc. Todo es declarativo.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/adapter"
	"channel/manifest"
)

func main() {
	channelURL := flag.String("channel", "http://localhost:8765", "Channel URL")
	apiKey := flag.String("key", "test-key-001", "Channel API key")
	sessionID := flag.String("session", "", "Session ID (opcional, persiste a sessions/<id>.jsonl)")
	baseDir := flag.String("base-dir", "/Users/alcless_a1234_cursor/remora-go", "Base dir donde viven los frameworks")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	frameworks, err := discover(*baseDir)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}

	c := adapter.New(*channelURL, *apiKey)
	c.SessionID = *sessionID

	switch args[0] {
	case "list":
		cmdList(frameworks)
	case "chains":
		cmdChains(frameworks)
	case "run":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: orchestrator run <framework> <command> [k=v ...]")
			os.Exit(1)
		}
		cmdRun(c, frameworks, args[1], args[2], parseKV(args[3:]))
	case "chain":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: orchestrator chain <fw1>:<cmd1> <fw2>:<cmd2> ...")
			os.Exit(1)
		}
		cmdChain(c, frameworks, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[0])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`orchestrator - genérico, leído por manifests

USO:
  orchestrator list                          Lista frameworks descubiertos
  orchestrator chains                        Muestra cadenas válidas (output.format → input.format)
  orchestrator run <fw> <cmd> [k=v ...]      Ejecuta un comando
  orchestrator chain <fw1>:<cmd1> ...        Ejecuta una cadena de comandos

EJEMPLOS:
  orchestrator list
  orchestrator chains
  orchestrator run echo add-axiom title="Demo" evidence="..."
  orchestrator chain echo:show-tree alfa:compile alfa:inspect

FLAGS:
  --channel URL       URL de Channel (default localhost:8765)
  --key KEY           API key
  --session ID        Session ID para persistir el JSONL
  --base-dir DIR      Donde buscar framework.manifest.json (default /Users/.../remora-go)`)
}

// discover busca framework.manifest.json en cada subdir del baseDir.
func discover(baseDir string) (map[string]*manifest.Manifest, error) {
	out := make(map[string]*manifest.Manifest)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(baseDir, e.Name(), "framework.manifest.json")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		m, err := manifest.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", path, err)
			continue
		}
		out[m.Name] = m
	}
	return out, nil
}

func cmdList(fws map[string]*manifest.Manifest) {
	fmt.Printf("Frameworks descubiertos: %d\n\n", len(fws))
	for _, m := range fws {
		fmt.Printf("• %s v%s — %s\n", m.Name, m.Version, m.Description)
		if len(m.Inputs) > 0 {
			fmt.Printf("    inputs:\n")
			for _, i := range m.Inputs {
				req := ""
				if i.Required {
					req = " (required)"
				}
				fmt.Printf("      - %s : %s%s\n", i.Name, i.Format, req)
			}
		}
		if len(m.Outputs) > 0 {
			fmt.Printf("    outputs:\n")
			for _, o := range m.Outputs {
				fmt.Printf("      - %s : %s → %s\n", o.Name, o.Format, o.Path)
			}
		}
		fmt.Printf("    commands: %s\n", strings.Join(commandNames(m), ", "))
		fmt.Println()
	}
}

func cmdChains(fws map[string]*manifest.Manifest) {
	fmt.Println("Cadenas válidas (output.format → input.format):")
	fmt.Println()
	count := 0
	for _, from := range fws {
		for _, to := range fws {
			if from.Name == to.Name {
				continue
			}
			for _, link := range from.CanChain(to) {
				fmt.Printf("  %s.%s ──[%s]──► %s.%s\n",
					link.From, link.Output, link.Format, link.To, link.Input)
				count++
			}
		}
	}
	if count == 0 {
		fmt.Println("  (ninguna cadena posible: ningún output match con un input)")
	}
}

func cmdRun(c *adapter.Client, fws map[string]*manifest.Manifest, fwName, cmdName string, params map[string]string) {
	m, ok := fws[fwName]
	if !ok {
		log.Fatalf("framework no encontrado: %s", fwName)
	}
	cmd, ok := m.Commands[cmdName]
	if !ok {
		log.Fatalf("comando no encontrado: %s.%s", fwName, cmdName)
	}

	inputs := resolveInputs(m, fws)
	outputs := absPathsFromPorts(m.Outputs)

	resolved, err := cmd.ResolveArgs(params, inputs, outputs)
	if err != nil {
		log.Fatalf("resolve args: %v", err)
	}

	fullArgs := append([]string{}, m.Binary.ArgsPrefix...)
	fullArgs = append(fullArgs, resolved...)

	fmt.Printf("▶ %s.%s\n", fwName, cmdName)
	fmt.Printf("  cmd: %s %s\n", m.Binary.Command, strings.Join(fullArgs, " "))
	fmt.Printf("  cwd: %s\n", m.Cwd)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	resp, err := c.ExecuteCommand(ctx, m.Binary.Command, fullArgs, m.Cwd)
	if err != nil {
		log.Fatalf("adapter error: %v", err)
	}
	if !resp.Success {
		log.Fatalf("Channel rechazó: %s", resp.Error)
	}
	fmt.Printf("  exit_code=%d duration_ms=%d\n", resp.ExitCode, resp.DurationMs)
	if resp.Stdout != "" {
		fmt.Printf("  stdout:\n%s", indent(resp.Stdout, "    "))
	}
	if resp.Stderr != "" {
		fmt.Printf("  stderr:\n%s", indent(resp.Stderr, "    "))
	}
	if resp.ExitCode != 0 {
		os.Exit(resp.ExitCode)
	}
}

func cmdChain(c *adapter.Client, fws map[string]*manifest.Manifest, steps []string) {
	for i, step := range steps {
		parts := strings.SplitN(step, ":", 2)
		if len(parts) != 2 {
			log.Fatalf("paso %d inválido: %q (formato: framework:comando)", i+1, step)
		}
		fmt.Printf("\n=== Paso %d/%d ===\n", i+1, len(steps))
		cmdRun(c, fws, parts[0], parts[1], map[string]string{})
	}
	fmt.Println("\n✓ Cadena completada")
}

// absPathsFromPorts convierte las paths relativas del manifest en absolutas
// dentro del baseDir de Channel (las paths ya son relativas al baseDir).
func absPathsFromPorts(ports []manifest.IOPort) map[string]string {
	out := make(map[string]string)
	for _, p := range ports {
		out[p.Name] = "/Users/alcless_a1234_cursor/remora-go/" + p.Path
	}
	return out
}

// resolveInputs resuelve los paths de los inputs del manifest m buscando
// QUIÉN produce ese formato en el set de frameworks descubiertos. Así el
// consumidor no tiene que conocer dónde vive el archivo del productor.
func resolveInputs(m *manifest.Manifest, all map[string]*manifest.Manifest) map[string]string {
	out := make(map[string]string)
	for _, in := range m.Inputs {
		// Buscar qué framework produce este formato
		for _, producer := range all {
			if producer.Name == m.Name {
				continue
			}
			for _, op := range producer.Outputs {
				if op.Format == in.Format {
					out[in.Name] = "/Users/alcless_a1234_cursor/remora-go/" + op.Path
					break
				}
			}
			if _, ok := out[in.Name]; ok {
				break
			}
		}
		if _, ok := out[in.Name]; !ok && in.Required {
			log.Fatalf("no producer found for required input %s.%s (format=%s)",
				m.Name, in.Name, in.Format)
		}
	}
	return out
}

func commandNames(m *manifest.Manifest) []string {
	out := make([]string, 0, len(m.Commands))
	for name := range m.Commands {
		out = append(out, name)
	}
	return out
}

func parseKV(args []string) map[string]string {
	out := make(map[string]string)
	for _, a := range args {
		if i := strings.Index(a, "="); i > 0 {
			out[a[:i]] = a[i+1:]
		}
	}
	return out
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if ln != "" {
			lines[i] = prefix + ln
		}
	}
	return strings.Join(lines, "\n")
}

// para shutup go vet sobre json import (no lo uso pero lo mantengo por si extiendo)
var _ = json.Marshal

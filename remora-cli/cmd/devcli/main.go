package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"
)

// Version is set at build time via ldflags
var Version = "dev"

func main() {
	app := &cli.App{
		Name:    "remora",
		Usage:   "CLI para desarrollar y depurar flujos en remora-go-lite",
		Version: Version,
		Before: func(c *cli.Context) error {
			client := newClient()
			if err := client.HealthCheck(); err != nil {
				fmt.Fprintf(os.Stderr, "%s[warn]%s backend no reachable: %v\n", yellow, reset, err)
			} else {
				fmt.Fprintf(os.Stderr, "%s[ok]%s backend reachable at %s\n", green, reset, client.BaseURL)
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "dev",
				Usage: "Comandos de desarrollador para inspeccionar, probar y debuggear flujos",
				Subcommands: []*cli.Command{
					inspectCmd,
					providersCmd,
					flowCmd,
					traceCmd,
					artifactsCmd,
					rulesCmd,
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s[error]%s %v\n", red, reset, err)
		os.Exit(1)
	}
}

// inspectCmd lists all frameworks with their capabilities
var inspectCmd = &cli.Command{
	Name:  "inspect",
	Usage: "Lista todos los frameworks con sus capabilities",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "json", Usage: "Output en JSON"},
		&cli.BoolFlag{Name: "verbose", Alias: "v", Usage: "Muestra más detalle"},
	},
	Action: func(c *cli.Context) error {
		client := newClient()
		frameworks, err := client.GetFrameworks()
		if err != nil {
			return fmt.Errorf("failed to get frameworks: %w", err)
		}

		if c.Bool("json") {
			return printJSON(frameworks)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%sFramework%s\t%sCapabilities%s\t%sMode%s\t%sProduces%s\n", bold, reset, bold, reset, bold, reset, bold, reset)
		fmt.Fprintf(w, "%s-----------%s\t%s-------------%s\t%s----%s\t%s--------%s\n", dim, reset, dim, reset, dim, reset, dim, reset)
		
		for _, fw := range frameworks {
			color := fwColor(fw.Name)
			caps := joinStrings(fw.Capabilities, ", ")
			if caps == "" {
				caps = "-"
			}
			prods := joinStrings(fw.Produces, ", ")
			if prods == "" {
				prods = "-"
			}
			fmt.Fprintf(w, "%s%s%s\t%s%s%s\t%s%s%s\t%s%s%s\n",
				color, fw.Name, reset,
				dim, caps, reset,
				fw.ExecutionMode, reset,
				dim, prods, reset,
			)
		}
		return w.Flush()
	},
}

// providersCmd shows capability -> framework routing
var providersCmd = &cli.Command{
	Name:  "providers",
	Usage: "Muestra el mapeo capability → framework",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "json", Usage: "Output en JSON"},
		&cli.StringFlag{Name: "capability", Alias: "c", Usage: "Filtrar por capability"},
	},
	Action: func(c *cli.Context) error {
		client := newClient()
		providers, err := client.GetProviders()
		if err != nil {
			return fmt.Errorf("failed to get providers: %w", err)
		}

		if c.Bool("json") {
			return printJSON(providers)
		}

		filter := c.String("capability")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%sCapability%s\t%sFrameworks%s\n", bold, reset, bold, reset)
		fmt.Fprintf(w, "%s-----------%s\t%s----------%s\n", dim, reset, dim, reset)
		
		for cap, fws := range providers {
			if filter != "" && cap != filter {
				continue
			}
			fwList := make([]string, len(fws))
			for i, fw := range fws {
				fwList[i] = fmt.Sprintf("%s%s%s", fwColor(fw), fw, reset)
			}
			fmt.Fprintf(w, "%s%s%s\t%s\n", cyan, cap, reset, joinStrings(fwList, ", "))
		}
		return w.Flush()
	},
}

// rulesCmd shows and manages flow composition rules
var rulesCmd = &cli.Command{
	Name:  "rules",
	Usage: "Muestra las reglas de composición de flujos",
	Action: func(c *cli.Context) error {
		client := newClient()
		rules, err := client.GetRules()
		if err != nil {
			return fmt.Errorf("failed to get rules: %w", err)
		}
		return printJSON(rules)
	},
}

// flowCmd is the parent for flow-related commands
var flowCmd = &cli.Command{
	Name:  "flow",
	Usage: "Gestión de flujos: listar, crear, simular, ejecutar, debuggear",
	Subcommands: []*cli.Command{
		flowListCmd,
		flowInspectCmd,
		flowCreateCmd,
		flowSimulateCmd,
		flowRunCmd,
		flowDebugCmd,
	},
}

// flowListCmd lists all compiled flows
var flowListCmd = &cli.Command{
	Name:  "list",
	Usage: "Lista todos los flujos compilados",
	Action: func(c *cli.Context) error {
		client := newClient()
		flows, err := client.ListFlows()
		if err != nil {
			return fmt.Errorf("failed to list flows: %w", err)
		}
		return printJSON(flows)
	},
}

// flowInspectCmd shows detailed flow definition
var flowInspectCmd = &cli.Command{
	Name:      "inspect",
	Usage:     "Muestra la definición detallada de un flujo",
	Args:      true,
	ArgsUsage: "<flow_id>",
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("se requiere flow_id")
		}
		client := newClient()
		flow, err := client.GetFlow(c.Args().First())
		if err != nil {
			return fmt.Errorf("failed to get flow: %w", err)
		}
		return printFlowManifest(flow)
	},
}

// flowCreateCmd creates a new flow from a template
var flowCreateCmd = &cli.Command{
	Name:      "create",
	Usage:     "Crea un nuevo flujo desde template (WIP: por ahora usa API directa)",
	Args:      true,
	ArgsUsage: "<business_id> <name>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "template", Alias: "t", Usage: "Template a usar"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 2 {
			return fmt.Errorf("usage: remora dev flow create <business_id> <name> [--template <tmpl>]")
		}
		// TODO: Implement template-based flow creation
		fmt.Fprintf(os.Stderr, "%s[info]%s creación de flujo vía API REST (implementación completa pendiente)\n", yellow, reset)
		fmt.Fprintf(os.Stderr, "  business_id: %s\n  name: %s\n  template: %s\n", 
			c.Args().Get(0), c.Args().Get(1), c.String("template"))
		return nil
	},
}

// flowSimulateCmd performs a dry-run of a flow
var flowSimulateCmd = &cli.Command{
	Name:      "simulate",
	Usage:     "Simula ejecución dry-run de un flujo",
	Args:      true,
	ArgsUsage: "<flow_id|compiled_id>",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{Name: "fixture", Alias: "f", Usage: "Fixtures a usar (archivos CSV/JSON)"},
		&cli.BoolFlag{Name: "verbose", Alias: "v", Usage: "Muestra más detalle"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("se requiere flow_id o compiled_id")
		}
		id := c.Args().First()
		fixtures := c.StringSlice("fixture")
		
		client := newClient()
		result, err := client.SimulateFlow(id, fixtures)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}
		
		return printFlowRunResult(result, c.Bool("verbose"))
	},
}

// flowRunCmd executes a flow for real
var flowRunCmd = &cli.Command{
	Name:      "run",
	Usage:     "Ejecuta un flujo (dry-run por defecto)",
	Args:      true,
	ArgsUsage: "<flow_id|compiled_id>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "Ejecuta realmente (no dry-run)"},
		&cli.StringFlag{Name: "recipient", Usage: "Email de test (para test_mode)"},
		&cli.IntFlag{Name: "max-cycles", Usage: "Máximo de ciclos", Value: 1},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("se requiere flow_id o compiled_id")
		}
		id := c.Args().First()
		live := c.Bool("live")
		
		req := flowRunRequest{
			CompiledID: id,
			DryRun:     !live,
			MaxCycles:  c.Int("max-cycles"),
		}
		if email := c.String("recipient"); email != "" {
			req.TestMode = true
			req.TestRecipient = email
		}
		
		client := newClient()
		result, err := client.RunFlow(req)
		if err != nil {
			return fmt.Errorf("run failed: %w", err)
		}
		
		return printFlowRunResult(result, true)
	},
}

// flowDebugCmd runs a flow in debug mode with step-by-step output
var flowDebugCmd = &cli.Command{
	Name:      "debug",
	Usage:     "Ejecuta un flujo en modo debug paso a paso",
	Args:      true,
	ArgsUsage: "<flow_id|compiled_id>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "step", Usage: "Pausa en cada paso"},
		&cli.StringFlag{Name: "break-on", Usage: "Puntos de ruptura: handoff,needs_input,error", Value: "needs_input"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("se requiere flow_id o compiled_id")
		}
		id := c.Args().First()
		
		fmt.Fprintf(os.Stderr, "%s[debug]%s iniciando debug para %s\n", cyan, reset, id)
		fmt.Fprintf(os.Stderr, "  break-on: %s\n", c.String("break-on"))
		
		// Run with verbose output to see each step
		req := flowRunRequest{
			CompiledID: id,
			DryRun:     true,
		}
		
		client := newClient()
		result, err := client.RunFlow(req)
		if err != nil {
			return fmt.Errorf("debug run failed: %w", err)
		}
		
		// Print step-by-step
		for i, step := range result.Timeline {
			fmt.Fprintf(os.Stdout, "\n%s[%d] %s%s.%s %s(%s)%s\n",
				bold, i+1,
				fwColor(step.Framework), step.Framework, reset,
				step.Node,
				dim, step.Capability, reset)
			
			if step.Role != "" {
				fmt.Fprintf(os.Stdout, "  role: %s%s%s\n", dim, step.Role, reset)
			}
			if len(step.Inputs) > 0 {
				fmt.Fprintf(os.Stdout, "  inputs: %s\n", joinStrings(step.Inputs, ", "))
			}
			if len(step.Outputs) > 0 {
				fmt.Fprintf(os.Stdout, "  outputs: %s\n", joinStrings(step.Outputs, ", "))
			}
			if step.Status != "" {
				statusColor := green
				if step.Status == "failed" || step.Status == "error" {
					statusColor = red
				}
				fmt.Fprintf(os.Stdout, "  status: %s%s%s\n", statusColor, step.Status, reset)
			}
			if step.DurationMs > 0 {
				fmt.Fprintf(os.Stdout, "  duration: %s%dms%s\n", dim, step.DurationMs, reset)
			}
			if step.Error != "" {
				fmt.Fprintf(os.Stdout, "  %serror:%s %s%s%s\n", red, reset, red, step.Error, reset)
			}
			if step.HumanSummary != "" {
				fmt.Fprintf(os.Stdout, "  summary: %s\n", step.HumanSummary)
			}
		}
		
		fmt.Fprintf(os.Stdout, "\n%s%s%s run_id: %s\n", bold, result.Status, reset, result.RunID)
		return nil
	},
}

// traceCmd shows traces for a specific run
var traceCmd = &cli.Command{
	Name:      "trace",
	Usage:     "Muestra trazas de ejecución de un run",
	Args:      true,
	ArgsUsage: "<run_id>",
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("se requiere run_id")
		}
		runID := c.Args().First()
		
		client := newClient()
		result, err := client.GetFlowRun(runID)
		if err != nil {
			return fmt.Errorf("failed to get run: %w", err)
		}
		
		return printFlowRunResult(result, true)
	},
}

// artifactsCmd inspects artifacts from a run
var artifactsCmd = &cli.Command{
	Name:      "artifacts",
	Usage:     "Inspecciona artefactos de un run",
	Args:      true,
	ArgsUsage: "<run_id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "view", Alias: "w", Usage: "Ver contenido de un artefacto específico"},
		&cli.BoolFlag{Name: "list", Alias: "l", Usage: "Lista todos los artefactos"},
	},
	Action: func(c *cli.Context) error {
		if c.Args().Len() < 1 {
			return fmt.Errorf("se requiere run_id")
		}
		runID := c.Args().First()
		
		client := newClient()
		result, err := client.GetFlowRun(runID)
		if err != nil {
			return fmt.Errorf("failed to get run: %w", err)
		}
		
		if view := c.String("view"); view != "" {
			if art, ok := result.Artifacts[view]; ok {
				return printJSON(art)
			}
			return fmt.Errorf("artifact %s not found", view)
		}
		
		// List artifacts
		fmt.Fprintf(os.Stdout, "%sArtefactos del run %s%s\n", bold, runID, reset)
		fmt.Fprintf(os.Stdout, "%s%-40s %s%-15s %s%s%s\n", dim, "TYPE", "SOURCE", "CREATED", reset)
		fmt.Fprintf(os.Stdout, "%s%-40s %s%-15s %s%s%s\n", dim, "----", "------", "-------", reset)
		
		for name, art := range result.Artifacts {
			created := art.CreatedAt
			if created == "" {
				created = "-"
			}
			fmt.Fprintf(os.Stdout, "%s%-40s %s%-15s %s%s%s\n",
				dim, name, reset,
				dim, art.Source, reset,
				gray, created, reset)
		}
		
		return nil
	},
}

// --- Formatting helpers ---

func printJSON(v interface{}) error {
	return printJSONWithEncoder(v, os.Stdout)
}

func printFlowManifest(flow *FlowManifest) error {
	fmt.Fprintf(os.Stdout, "%sFlow: %s%s (%s)\n", bold, flow.ID, reset, flow.BusinessID)
	
	if flow.Intent.Goal != "" {
		fmt.Fprintf(os.Stdout, "\n  %sIntent:%s %s\n", bold, reset, flow.Intent.Goal)
	}
	
	fmt.Fprintf(os.Stdout, "\n  %sNodes (%d):%s\n", bold, len(flow.Nodes), reset)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  %sID%s\t%sFramework%s\t%sCapability%s\t%sRole%s\t%sInputs%s\t%sOutputs%s\n", bold, reset, bold, reset, bold, reset, bold, reset, bold, reset, bold, reset)
	
	for _, n := range flow.Nodes {
		inputs := joinStrings(n.Inputs, ", ")
		outputs := joinStrings(n.Outputs, ", ")
		role := n.Role
		if role == "" {
			role = "-"
		}
		fmt.Fprintf(w, "  %s%s%s\t%s%s%s\t%s%s%s\t%s%s%s\t%s%s%s\t%s%s%s\n",
			fwColor(n.Framework), n.ID, reset,
			fwColor(n.Framework), n.Framework, reset,
			dim, n.Capability, reset,
			dim, role, reset,
			dim, inputs, reset,
			dim, outputs, reset)
	}
	w.Flush()
	
	if len(flow.Edges) > 0 {
		fmt.Fprintf(os.Stdout, "\n  %sEdges (%d):%s\n", bold, len(flow.Edges), reset)
		for _, e := range flow.Edges {
			fmt.Fprintf(os.Stdout, "    %s%s%s → %s%s%s\n",
				dim, e.From, reset,
				dim, e.To, reset)
		}
	}
	
	return nil
}

func printFlowRunResult(result *FlowRunResult, verbose bool) error {
	// Status header
	statusColor := green
	if result.Status == "invalid" || result.Status == "failed" {
		statusColor = red
	} else if result.Status == "pending" {
		statusColor = yellow
	}
	
	fmt.Fprintf(os.Stdout, "\n%s%s══════════════════════════════════════════════════%s\n", bold, dim, reset)
	fmt.Fprintf(os.Stdout, "%s  Run: %s%s\n", bold, result.RunID, reset)
	fmt.Fprintf(os.Stdout, "  Flow: %s%s%s\n", fwColor(result.FlowID), result.FlowID, reset)
	fmt.Fprintf(os.Stdout, "  Status: %s%s%s\n", statusColor, result.Status, reset)
	fmt.Fprintf(os.Stdout, "  Valid: %v | DryRun: %v | TestMode: %v\n", result.Valid, result.DryRun, result.TestMode)
	
	if result.BusinessID != "" {
		fmt.Fprintf(os.Stdout, "  BusinessID: %s%s%s\n", cyan, result.BusinessID, reset)
	}
	
	fmt.Fprintf(os.Stdout, "%s══════════════════════════════════════════════════%s\n", bold, dim, reset)
	
	// Execution timeline
	if len(result.Timeline) > 0 {
		fmt.Fprintf(os.Stdout, "\n%sExecution Timeline (%d steps):%s\n", bold, len(result.Timeline), reset)
		
		for i, step := range result.Timeline {
			prefix := "  "
			if verbose {
				prefix = fmt.Sprintf("  %s%2d.%s ", bold, i+1, reset)
			}
			
			fmt.Fprintf(os.Stdout, "%s%s%s.%s %s%s%s",
				prefix,
				fwColor(step.Framework), step.Framework, reset,
				cyan, step.Node, reset)
			
			if step.Capability != "" {
				fmt.Fprintf(os.Stdout, " %s(%s)%s", dim, step.Capability, reset)
			}
			
			// Status indicator
			statusOk := step.Status == "completed" || step.Status == "success"
			statusIndicator := green
			if !statusOk {
				statusIndicator = red
			}
			fmt.Fprintf(os.Stdout, " %s[%s]%s", statusIndicator, step.Status, reset)
			
			if step.DurationMs > 0 {
				fmt.Fprintf(os.Stdout, " %s%dms%s", dim, step.DurationMs, reset)
			}
			
			fmt.Fprintf(os.Stdout, "\n")
			
			if verbose {
				if len(step.Inputs) > 0 {
					fmt.Fprintf(os.Stdout, "    %sinputs:%s %s\n", dim, reset, joinStrings(step.Inputs, ", "))
				}
				if len(step.Outputs) > 0 {
					fmt.Fprintf(os.Stdout, "    %soutputs:%s %s\n", dim, reset, joinStrings(step.Outputs, ", "))
				}
				if step.Error != "" {
					fmt.Fprintf(os.Stdout, "    %serror:%s %s%s%s\n", red, reset, red, step.Error, reset)
				}
				if step.HumanSummary != "" {
					fmt.Fprintf(os.Stdout, "    %s%s%s\n", dim, step.HumanSummary, reset)
				}
			}
		}
	}
	
	// Artifacts summary
	if len(result.Artifacts) > 0 {
		fmt.Fprintf(os.Stdout, "\n%sArtifacts (%d):%s\n", bold, len(result.Artifacts), reset)
		for name, art := range result.Artifacts {
			fmt.Fprintf(os.Stdout, "  %s• %s%s %s(%s)%s\n",
				dim, reset,
				cyan, name, reset,
				dim, art.Source, reset)
		}
	}
	
	// Handoffs
	if len(result.Handoffs) > 0 {
		fmt.Fprintf(os.Stdout, "\n%sHandoffs (%d):%s\n", bold, len(result.Handoffs), reset)
		for _, h := range result.Handoffs {
			fmt.Fprintf(os.Stdout, "  %s%s%s → %s%s%s [%s]%s\n",
				dim, h.FromNode, reset,
				dim, h.ToNode, reset,
				cyan, h.Artifact, reset)
		}
	}
	
	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Fprintf(os.Stdout, "\n%sWarnings:%s\n", yellow, reset)
		for _, w := range result.Warnings {
			nodeInfo := ""
			if w.Node != "" {
				nodeInfo = fmt.Sprintf(" [%s]", w.Node)
			}
			fmt.Fprintf(os.Stdout, "  %s⚠%s %s%s%s%s\n",
				yellow, reset,
				dim, w.Code, nodeInfo, reset,
				w.Message)
		}
	}
	
	// Needs input
	if len(result.NeedsInput) > 0 {
		fmt.Fprintf(os.Stdout, "\n%sNeeds Input:%s\n", bold, reset)
		for _, ni := range result.NeedsInput {
			fmt.Fprintf(os.Stdout, "  • %s%s%s %s(%s)%s\n",
				fwColor(ni.Node), ni.Node, reset,
				dim, ni.Capability, reset)
			if ni.Question != "" {
				fmt.Fprintf(os.Stdout, "    %s%s%s\n", dim, ni.Question, reset)
			}
		}
	}
	
	// Timestamps
	if result.CreatedAt != "" {
		fmt.Fprintf(os.Stdout, "\n%sCreated:%s %s\n", bold, reset, result.CreatedAt)
	}
	if result.FinishedAt != "" {
		fmt.Fprintf(os.Stdout, "%sFinished:%s %s\n", bold, reset, result.FinishedAt)
	}
	
	fmt.Fprintf(os.Stdout, "\n")
	return nil
}

func joinStrings(arr []string, sep string) string {
	if len(arr) == 0 {
		return ""
	}
	result := arr[0]
	for i := 1; i < len(arr); i++ {
		result += sep + arr[i]
	}
	return result
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", float64(seconds))
	}
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
}

func formatTimestamp(ts string) string {
	if ts == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.Format("15:04:05.000")
}
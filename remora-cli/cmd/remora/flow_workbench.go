package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type cliFlowIntent struct {
	Goal            string   `json:"goal,omitempty"`
	OperatorRole    string   `json:"operator_role,omitempty"`
	SuccessCriteria string   `json:"success_criteria,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Description     string   `json:"description,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	CapabilityHint  string   `json:"capability_hint,omitempty"`
}

type cliFlowLifecycleBinding struct {
	Framework  string `json:"framework,omitempty"`
	Capability string `json:"capability,omitempty"`
}

type cliFlowLifecycle struct {
	Entry  cliFlowLifecycleBinding `json:"entry,omitempty"`
	Tutela cliFlowLifecycleBinding `json:"tutela,omitempty"`
}

type cliFlowNode struct {
	ID         string `json:"id"`
	Framework  string `json:"framework"`
	Capability string `json:"capability,omitempty"`
	Role       string `json:"role,omitempty"`
}

type cliFlowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type cliFlowManifest struct {
	ID         string             `json:"id"`
	BusinessID string             `json:"business_id,omitempty"`
	Intent     cliFlowIntent      `json:"intent,omitempty"`
	Lifecycle  cliFlowLifecycle   `json:"lifecycle,omitempty"`
	Nodes      []cliFlowNode      `json:"nodes"`
	Edges      []cliFlowEdge      `json:"edges,omitempty"`
	Derivation *cliFlowDerivation `json:"derivation,omitempty"`
}

type cliFlowDataGrounding struct {
	DesiredCapability string   `json:"desired_capability,omitempty"`
	BusinessArtifacts []string `json:"business_artifacts,omitempty"`
	MissingArtifacts  []string `json:"missing_artifacts,omitempty"`
	RequiredArtifacts []string `json:"required_artifacts,omitempty"`
	UniversalRoles    []string `json:"universal_roles,omitempty"`
}

type cliFlowAmendment struct {
	Kind    string `json:"kind"`
	NodeID  string `json:"node_id,omitempty"`
	Summary string `json:"summary"`
	Reason  string `json:"reason,omitempty"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
}

type cliFlowDerivedContract struct {
	NodeID         string   `json:"node_id"`
	Framework      string   `json:"framework"`
	Capability     string   `json:"capability,omitempty"`
	Role           string   `json:"role,omitempty"`
	Command        string   `json:"command,omitempty"`
	Inputs         []string `json:"inputs,omitempty"`
	Requires       []string `json:"requires,omitempty"`
	Outputs        []string `json:"outputs,omitempty"`
	Produces       []string `json:"produces,omitempty"`
	Policies       []string `json:"policies,omitempty"`
	ResolutionMode string   `json:"resolution_mode,omitempty"`
}

type cliFlowDerivedHandoff struct {
	FromNode      string   `json:"from_node"`
	ToNode        string   `json:"to_node"`
	FromFramework string   `json:"from_framework"`
	ToFramework   string   `json:"to_framework"`
	Artifacts     []string `json:"artifacts,omitempty"`
	Ownership     string   `json:"ownership,omitempty"`
	Summary       string   `json:"summary"`
}

type cliFlowInstallPreview struct {
	RequiresInstall bool     `json:"requires_install"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

type cliFlowExecutablePlan struct {
	Nodes     []cliFlowNode    `json:"nodes,omitempty"`
	Edges     []cliFlowEdge    `json:"edges,omitempty"`
	Lifecycle cliFlowLifecycle `json:"lifecycle,omitempty"`
}

type cliFlowDerivation struct {
	Grounding  cliFlowDataGrounding     `json:"grounding,omitempty"`
	Amendments []cliFlowAmendment       `json:"amendments,omitempty"`
	Contracts  []cliFlowDerivedContract `json:"contracts,omitempty"`
	Handoffs   []cliFlowDerivedHandoff  `json:"handoffs,omitempty"`
	Install    cliFlowInstallPreview    `json:"install"`
	Executable cliFlowExecutablePlan    `json:"executable,omitempty"`
}

type cliFlowCapabilitySuggestion struct {
	Framework   string  `json:"framework"`
	Capability  string  `json:"capability"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Reason      string  `json:"reason"`
	Category    string  `json:"category"`
	Confidence  float64 `json:"confidence"`
}

type cliFlowSuggestionProposal struct {
	IntentPlan cliFlowSuggestIntentPlan   `json:"intent_plan"`
	Bindings   []cliFlowSuggestionBinding `json:"bindings,omitempty"`
	Manifest   cliFlowManifest            `json:"manifest"`
	Derivation *cliFlowDerivation         `json:"derivation,omitempty"`
}

type cliFlowSuggestIntentPlan struct {
	Goal            string                   `json:"goal,omitempty"`
	OperatorRole    string                   `json:"operator_role,omitempty"`
	SuccessCriteria string                   `json:"success_criteria,omitempty"`
	Description     string                   `json:"description,omitempty"`
	Roles           []cliFlowSuggestRolePlan `json:"roles,omitempty"`
	CapabilityHint  string                   `json:"capability_hint,omitempty"`
}

type cliFlowSuggestRolePlan struct {
	Role      string `json:"role"`
	Objective string `json:"objective,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type cliFlowSuggestionBinding struct {
	Role             string  `json:"role"`
	Objective        string  `json:"objective,omitempty"`
	IntentReason     string  `json:"intent_reason,omitempty"`
	Framework        string  `json:"framework"`
	Capability       string  `json:"capability"`
	Title            string  `json:"title,omitempty"`
	SuggestionReason string  `json:"suggestion_reason,omitempty"`
	Category         string  `json:"category,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
}

type cliBusinessArtifactsResponse struct {
	BusinessID string            `json:"business_id"`
	Artifacts  []string          `json:"artifacts"`
	Sources    map[string]string `json:"sources"`
}

type cliFlowSuggestResponse struct {
	Suggestions []cliFlowCapabilitySuggestion `json:"suggestions"`
	Source      string                        `json:"source"`
	Proposal    *cliFlowSuggestionProposal    `json:"proposal,omitempty"`
}

type flowCreateAnswers struct {
	BusinessID       string
	Name             string
	Description      string
	CapabilityHint   string
	SuccessCriteria  string
	AutonomyMode     string
	EntryFramework   string
	EntryCapability  string
	TutelaFramework  string
	TutelaCapability string
}

type cliInstalledSnapshot struct {
	Installed       bool     `json:"installed"`
	AnalysisPlan    string   `json:"analysis_plan,omitempty"`
	AnalysisSchema  string   `json:"analysis_schema,omitempty"`
	SchemaID        string   `json:"schema_id,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
	ReconfigureHint []string `json:"reconfigure_hint,omitempty"`
}

type cliFlowRecord struct {
	ID          string                `json:"id"`
	BusinessID  string                `json:"business_id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Status      string                `json:"status"`
	Manifest    *cliFlowManifest      `json:"manifest"`
	Installed   *cliInstalledSnapshot `json:"installed,omitempty"`
}

type cliValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cliValidationResult struct {
	Valid    bool                 `json:"valid"`
	Errors   []cliValidationIssue `json:"errors"`
	Warnings []cliValidationIssue `json:"warnings"`
}

type cliSimulationStep struct {
	Node             string   `json:"node"`
	Framework        string   `json:"framework"`
	Capability       string   `json:"capability,omitempty"`
	Status           string   `json:"status"`
	MissingArtifacts []string `json:"missing_artifacts,omitempty"`
	Produces         []string `json:"produces,omitempty"`
}

type cliSimulationResult struct {
	Valid      bool                `json:"valid"`
	Artifacts  []string            `json:"artifacts"`
	Timeline   []cliSimulationStep `json:"timeline"`
	Derivation *cliFlowDerivation  `json:"derivation,omitempty"`
	Validation cliValidationResult `json:"validation"`
}

var flowWorkbenchExecCommand = exec.Command

func handleFlowWorkbench() {
	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	// Handle 'remora debug ...' subcommands
	if len(args) > 0 && args[0] == "debug" {
		if len(args) == 1 {
			printDebugUsage()
			return
		}
		handleDebugCommand(args[1:])
		return
	}

	if err := delegateToCanonicalFlowWorkbench(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printDebugUsage() {
	fmt.Print(`debug

Uso:
  remora debug frameworks                    lista todos los frameworks disponibles
  remora debug manifest <framework>          muestra manifest completo del framework
  remora debug commands <framework>         lista comandos del framework
  remora debug capabilities <framework>      lista capabilities del framework
  remora debug trace <run-id>                muestra timeline de ejecución
  remora debug validate <flow-id>           valida flujo
  remora debug simulate <flow-id>           dry-run con timeline
  remora debug dependencies <flow-id>        muestra grafo de dependencias
`)
}

func handleDebugCommand(args []string) {
	if len(args) == 0 {
		printDebugUsage()
		return
	}

	cmd := args[0]
	switch cmd {
	case "frameworks":
		handleDebugFrameworks(args[1:])
	case "manifest":
		handleDebugManifest(args[1:])
	case "commands":
		handleDebugCommands(args[1:])
	case "capabilities":
		handleDebugCapabilities(args[1:])
	case "trace":
		handleDebugTrace(args[1:])
	case "validate":
		handleDebugValidate(args[1:])
	case "simulate":
		handleDebugSimulate(args[1:])
	case "dependencies":
		handleDebugDependencies(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "comando debug desconocido: %s\n", cmd)
		printDebugUsage()
		os.Exit(1)
	}
}

// =============================================================================
// DEBUG COMMANDS
// =============================================================================

// handleDebugFrameworks implements: remora debug frameworks
func handleDebugFrameworks(args []string) {
	fs := flag.NewFlagSet("debug frameworks", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	c := newClient()
	resp, err := c.get("/frameworks")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "respuesta inesperada del servidor")
		os.Exit(1)
	}

	frameworks := make([]FrameworkInfo, 0, len(data))
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			fw := FrameworkInfo{}
			if name, ok := m["name"].(string); ok {
				fw.Name = name
			}
			if version, ok := m["version"].(string); ok {
				fw.Version = version
			}
			if command, ok := m["command"].(string); ok {
				fw.Command = command
			}
			if mode, ok := m["mode"].(string); ok {
				fw.Mode = mode
			}
			if freshness, ok := m["freshness"].(string); ok {
				fw.Freshness = freshness
			}
			frameworks = append(frameworks, fw)
		}
	}

	if *jsonOut {
		printJSON(frameworks)
		return
	}

	fmt.Printf("%sFrameworks disponibles%s (%d)\n\n", cyan, reset, len(frameworks))
	for _, fw := range frameworks {
		printFrameworkName(fw.Name)
		if fw.Version != "" {
			fmt.Printf(" %s(v%s)%s", dim, fw.Version, reset)
		}
		if fw.Mode != "" {
			fmt.Printf(" [%s]", fw.Mode)
		}
		if fw.Freshness != "" {
			fmt.Printf(" %s%s%s", gray, fw.Freshness, reset)
		}
		fmt.Println()
	}
}

// handleDebugManifest implements: remora debug manifest <framework>
func handleDebugManifest(args []string) {
	fs := flag.NewFlagSet("debug manifest", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug manifest <framework>")
		os.Exit(1)
	}

	framework := args[0]
	c := newClient()
	resp, err := c.get("/frameworks/" + framework + "/manifest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(resp)
		return
	}

	fmt.Printf("%sManifest: %s%s\n\n", green, framework, reset)
	fmt.Print(formatFrameworkManifest(resp))
}

// handleDebugCommands implements: remora debug commands <framework>
func handleDebugCommands(args []string) {
	fs := flag.NewFlagSet("debug commands", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug commands <framework>")
		os.Exit(1)
	}

	framework := args[0]
	c := newClient()
	resp, err := c.get("/frameworks/" + framework + "/commands")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "respuesta inesperada del servidor")
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(data)
		return
	}

	fmt.Printf("%sComandos de %s%s\n\n", cyan, framework, reset)
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			cmd, _ := m["command"].(string)
			desc, _ := m["description"].(string)
			if desc == "" {
				desc = "sin descripción"
			}
			fmt.Printf("  %s%s%s %s\n", bold, cmd, reset, dim+desc+reset)
		}
	}
}

// handleDebugCapabilities implements: remora debug capabilities <framework>
func handleDebugCapabilities(args []string) {
	fs := flag.NewFlagSet("debug capabilities", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug capabilities <framework>")
		os.Exit(1)
	}

	framework := args[0]
	c := newClient()
	resp, err := c.get("/frameworks/" + framework + "/capabilities")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "respuesta inesperada del servidor")
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(data)
		return
	}

	fmt.Printf("%sCapabilities de %s%s\n\n", magenta, framework, reset)
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			cap, _ := m["capability"].(string)
			inputs := []string{}
			if inp, ok := m["inputs"].([]interface{}); ok {
				for _, i := range inp {
					if s, ok := i.(string); ok {
						inputs = append(inputs, s)
					}
				}
			}
			outputs := []string{}
			if out, ok := m["outputs"].([]interface{}); ok {
				for _, o := range out {
					if s, ok := o.(string); ok {
						outputs = append(outputs, s)
					}
				}
			}
			fmt.Printf("  %s%s%s\n", bold, cap, reset)
			if len(inputs) > 0 {
				fmt.Printf("    inputs:  %s\n", strings.Join(inputs, ", "))
			}
			if len(outputs) > 0 {
				fmt.Printf("    outputs: %s\n", strings.Join(outputs, ", "))
			}
		}
	}
}

// handleDebugTrace implements: remora debug trace <run-id>
func handleDebugTrace(args []string) {
	fs := flag.NewFlagSet("debug trace", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	verbose := fs.Bool("v", false, "verbose con stdout/stderr preview")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug trace <run-id>")
		os.Exit(1)
	}

	runID := args[0]
	c := newClient()
	resp, err := c.get("/flows/runs/" + runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var result FlowRunResult
	if err := decodeAPIData(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando run: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(result)
		return
	}

	printRunTrace(result, *verbose)
}

// handleDebugValidate implements: remora debug validate <flow-id>
func handleDebugValidate(args []string) {
	fs := flag.NewFlagSet("debug validate", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug validate <flow-id>")
		os.Exit(1)
	}

	flowID := args[0]
	c := newClient()
	resp, err := c.post("/flows/"+flowID+"/validate", map[string]interface{}{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var result FlowValidationResult
	if err := decodeAPIData(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando validacion: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(result)
		return
	}

	printFlowValidation(flowID, result)
}

// handleDebugSimulate implements: remora debug simulate <flow-id>
func handleDebugSimulate(args []string) {
	fs := flag.NewFlagSet("debug simulate", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	fixtures := fs.String("fixtures", "", "artifacts separados por coma")
	input := fs.String("input", "", "texto de prueba")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug simulate <flow-id>")
		os.Exit(1)
	}

	flowID := args[0]

	// Fetch flow record first
	c := newClient()
	flowResp, err := c.get("/flows/" + flowID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	var record cliFlowRecord
	if err := decodeAPIData(flowResp, &record); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando flow: %v\n", err)
		os.Exit(1)
	}

	// Simulate
	resp, err := c.post("/flows/simulate", map[string]interface{}{
		"flow":              record.Manifest,
		"input":             *input,
		"fixture_artifacts": parseCSVList(*fixtures),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var result SimulateResult
	if err := decodeAPIData(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando simulacion: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(result)
		return
	}

	printSimulateResult(flowID, result)
}

// handleDebugDependencies implements: remora debug dependencies <flow-id>
func handleDebugDependencies(args []string) {
	fs := flag.NewFlagSet("debug dependencies", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	if len(args) == 0 || fs.Parsed() && args[len(args)-1] == "" {
		fmt.Fprintln(os.Stderr, "usage: remora debug dependencies <flow-id>")
		os.Exit(1)
	}

	flowID := args[0]
	c := newClient()

	// Fetch flow with derivation (which contains dependency info)
	resp, err := c.get("/flows/" + flowID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var record cliFlowRecord
	if err := decodeAPIData(resp, &record); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando flow: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		printJSON(record)
		return
	}

	printFlowDependencies(record)
}

// =============================================================================
// FORMATTING HELPERS
// =============================================================================

func formatFrameworkManifest(resp map[string]interface{}) string {
	var buf bytes.Buffer

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if name, ok := data["name"].(string); ok {
			fmt.Fprintf(&buf, "  name:        %s%s%s\n", bold, name, reset)
		}
		if version, ok := data["version"].(string); ok {
			fmt.Fprintf(&buf, "  version:     %s\n", version)
		}
		if command, ok := data["command"].(string); ok {
			fmt.Fprintf(&buf, "  command:     %s\n", command)
		}
		if mode, ok := data["mode"].(string); ok {
			fmt.Fprintf(&buf, "  mode:        %s\n", mode)
		}
	}

	return buf.String()
}

func printRunTrace(result FlowRunResult, verbose bool) {
	fmt.Printf("%sRun Trace%s %s\n", cyan, reset, result.RunID)
	fmt.Printf("  status:      %s", result.Status)
	if result.FinishedAt != "" {
		fmt.Printf(" (finished: %s)", formatTime(result.FinishedAt))
	}
	fmt.Println()
	fmt.Printf("  timeline:    %d steps\n", len(result.Timeline))
	fmt.Printf("  execution:   %s\n", strings.Join(result.ExecutionOrder, " → "))

	if len(result.Handoffs) > 0 {
		fmt.Printf("\n%sHandoffs%s\n", blue, reset)
		for _, h := range result.Handoffs {
			fmt.Printf("  %s → %s (%s)\n", h.FromNode, h.ToNode, strings.Join(h.Artifacts, ", "))
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n%sWarnings%s\n", yellow, reset)
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w.Message)
		}
	}

	fmt.Printf("\n%sTimeline detalhada%s\n", magenta, reset)
	for i, step := range result.Timeline {
		fmt.Printf("\n  %d. ", i+1)
		printFrameworkName(step.Framework)
		if step.Capability != "" {
			fmt.Printf(".%s", step.Capability)
		}
		fmt.Printf(" %s[%s]%s", fwColor(step.Framework), step.Node, reset)
		printStatus(step.Status)

		if step.StartedAt != "" {
			fmt.Printf("%s → %s", formatTime(step.StartedAt), formatTime(step.FinishedAt))
		}
		if step.DurationMs > 0 {
			fmt.Printf(" (%s)", formatDuration(step.DurationMs))
		}
		fmt.Println()

		if verbose && step.Error != "" {
			fmt.Printf("     %sError%s: %s\n", red, reset, step.Error)
		}
		if verbose && step.StdoutPreview != "" {
			lines := strings.Split(step.StdoutPreview, "\n")
			for _, line := range lines {
				if len(line) > 100 {
					line = line[:100] + "..."
				}
				fmt.Printf("     %s%s%s\n", green, line, reset)
			}
		}
		if verbose && step.StderrPreview != "" {
			lines := strings.Split(step.StderrPreview, "\n")
			for _, line := range lines {
				if len(line) > 100 {
					line = line[:100] + "..."
				}
				fmt.Printf("     %s%s%s\n", red, line, reset)
			}
		}
		if len(step.Produces) > 0 {
			fmt.Printf("     produces: %s\n", strings.Join(step.Produces, ", "))
		}
	}

	fmt.Println()
}

func printFlowValidation(flowID string, result FlowValidationResult) {
	if result.Valid {
		fmt.Printf("%s✓ Flow valido%s %s\n", green, reset, flowID)
	} else {
		fmt.Printf("%s✗ Flow invalido%s %s\n", red, reset, flowID)
	}

	if len(result.Violations) > 0 {
		fmt.Printf("\n%sViolaciones (%d)%s\n", red, len(result.Violations), reset)
		for _, v := range result.Violations {
			fmt.Printf("  - %s: %s", v.Kind, v.Message)
			if v.At != "" {
				fmt.Printf(" %sat %s%s", dim, v.At, reset)
			}
			fmt.Println()
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n%sAdvertencias (%d)%s\n", yellow, len(result.Warnings), reset)
		for _, w := range result.Warnings {
			fmt.Printf("  - %s: %s\n", w.Kind, w.Message)
		}
	}
}

func printSimulateResult(flowID string, result SimulateResult) {
	if result.Valid {
		fmt.Printf("%s✓ Simulacion valida%s %s\n", green, reset, flowID)
	} else {
		fmt.Printf("%s✗ Simulacion invalida%s %s\n", red, reset, flowID)
	}

	if len(result.ExecutionPlan) > 0 {
		fmt.Printf("\n%sPlan de ejecucion%s\n", cyan, reset)
		for i, step := range result.ExecutionPlan {
			fmt.Printf("  %d. %s\n", i+1, step)
		}
	}

	if len(result.Capabilities) > 0 {
		fmt.Printf("\n%sCapabilities (%d)%s\n", magenta, len(result.Capabilities), reset)
		fmt.Printf("  %s\n", strings.Join(result.Capabilities, ", "))
	}

	if len(result.Providers) > 0 {
		fmt.Printf("\n%sProviders (%d)%s\n", blue, len(result.Providers), reset)
		for _, p := range result.Providers {
			fmt.Printf("  - %s → %s.%s", p.Source, p.Framework, p.Capability)
			if p.Execution != "" {
				fmt.Printf(" [%s]", p.Execution)
			}
			fmt.Println()
		}
	}

	if len(result.MissingCapabilities) > 0 {
		fmt.Printf("\n%sCapabilities faltantes (%d)%s\n", red, len(result.MissingCapabilities), reset)
		for _, m := range result.MissingCapabilities {
			fmt.Printf("  - %s\n", m)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n%sWarnings%s\n", yellow, reset)
		for _, w := range result.Warnings {
			fmt.Printf("  - %s: %s\n", w.Kind, w.Message)
		}
	}
}

func printFlowDependencies(record cliFlowRecord) {
	fmt.Printf("%sDependencies%s %s (%s)\n\n", cyan, reset, record.Name, record.ID)

	if record.Manifest == nil {
		fmt.Println("  (sin manifest)")
		return
	}

	// Build adjacency map
	deps := make(map[string][]string)
	for _, edge := range record.Manifest.Edges {
		deps[edge.From] = append(deps[edge.From], edge.To)
	}

	fmt.Printf("%sNodes%s\n", green, reset)
	for _, node := range record.Manifest.Nodes {
		fmt.Printf("  %s%s%s: %s.%s", bold, node.ID, reset, fwColor(node.Framework), node.Framework, reset)
		if node.Capability != "" {
			fmt.Printf(".%s", node.Capability)
		}
		fmt.Println()
	}

	fmt.Printf("\n%sEdges (dependencias)%s\n", blue, reset)
	if len(record.Manifest.Edges) == 0 {
		fmt.Printf("  %s(sin aristas)%s\n", dim, reset)
	} else {
		for _, edge := range record.Manifest.Edges {
			fmt.Printf("  %s → %s\n", edge.From, edge.To)
		}
	}

	// Print dependency graph
	fmt.Printf("\n%sGrafo de dependencias%s\n", magenta, reset)
	for _, node := range record.Manifest.Nodes {
		children := deps[node.ID]
		if len(children) == 0 {
			fmt.Printf("  %s (%s.%s) - sin dependientes\n", node.ID, node.Framework, node.Capability)
		} else {
			fmt.Printf("  %s (%s.%s) → %s\n", node.ID, node.Framework, node.Capability, strings.Join(children, ", "))
		}
	}

	// Print derivation contracts if available
	if record.Manifest.Derivation != nil && len(record.Manifest.Derivation.Contracts) > 0 {
		fmt.Printf("\n%sContratos derivados%s\n", yellow, reset)
		for _, c := range record.Manifest.Derivation.Contracts {
			fmt.Printf("  %s:\n", c.NodeID)
			fmt.Printf("    in:  %s\n", strings.Join(c.Requires, ", "))
			fmt.Printf("    out: %s\n", strings.Join(c.Produces, ", "))
		}
	}
}

func delegateToCanonicalFlowWorkbench(args []string) error {
	if len(args) == 0 {
		printFlowWorkbenchUsage()
		return nil
	}
	cmd, err := canonicalFlowWorkbenchCommand(args)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func canonicalFlowWorkbenchCommand(args []string) (*exec.Cmd, error) {
	if repoRoot := findRepoRoot(); repoRoot != "" {
		cmd := flowWorkbenchExecCommand("go", append([]string{"run", "./cmd/flujo", "flow"}, args...)...)
		cmd.Dir = filepath.Join(repoRoot, "remora-flujo")
		return cmd, nil
	}
	if _, err := exec.LookPath("flujo"); err == nil {
		return flowWorkbenchExecCommand("flujo", append([]string{"flow"}, args...)...), nil
	}
	return nil, fmt.Errorf("flow workbench canónico no disponible; usá `flujo flow ...`")
}

func printFlowWorkbenchUsage() {
	fmt.Print(`flow workbench

Uso:
  remora flow create --business <id> [--name <n>] [--description <texto>]
  remora flow draft --business <id> --name <n> --description <texto> [--create]
  remora flow compile --id <flow_id>
  remora flow inspect --id <flow_id>
  remora flow validate --id <flow_id>
  remora flow simulate --id <flow_id> [--fixtures a,b] [--input texto]
  remora flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]
  remora flow install --id <flow_id> [--reconfigure]
  remora flow replay --run <run_id>
  remora flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
`)
}

func flowAutonomySummary(mode string) (label string, constraints []string, summary string) {
	switch strings.TrimSpace(mode) {
	case "advisory":
		return "Solo diagnóstico", []string{"solo_diagnostico", "no_external_side_effect"}, "No ejecutar efectos externos automáticos; solo análisis y propuestas."
	case "approved":
		return "Operación aprobable", []string{"approved_runtime_execution"}, "Puede operar en vivo una vez instalado y aprobado."
	default:
		return "Preparar con aprobación", []string{"approval_required", "human_review_before_apply"}, "Puede preparar acciones, pero requiere aprobación humana antes de aplicar cambios."
	}
}

func buildIntentFirstDescription(in flowCreateAnswers) string {
	_, _, autonomySummary := flowAutonomySummary(in.AutonomyMode)
	parts := []string{
		strings.TrimSpace(in.Description),
	}
	if value := strings.TrimSpace(in.SuccessCriteria); value != "" {
		parts = append(parts, "Éxito esperado: "+value+".")
	}
	if autonomySummary != "" {
		parts = append(parts, autonomySummary)
	}
	return strings.Join(compactStrings(parts), " ")
}

func buildFlowCreateSuggestPayload(in flowCreateAnswers, max int) map[string]interface{} {
	intent := map[string]interface{}{
		"goal":             firstNonEmpty(strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)),
		"operator_role":    "staff",
		"success_criteria": strings.TrimSpace(in.SuccessCriteria),
		"constraints":      flowAutonomyConstraints(in.AutonomyMode),
		"description":      buildIntentFirstDescription(in),
		"roles":            inferFlowCreateRoles(in),
		"capability_hint":  strings.TrimSpace(in.CapabilityHint),
	}
	payload := map[string]interface{}{
		"business_id": in.BusinessID,
		"name":        in.Name,
		"description": buildIntentFirstDescription(in),
		"max":         max,
		"intent":      intent,
	}
	if lifecycle := buildFlowCreateLifecycle(in); !emptyCLILifecycle(lifecycle) {
		payload["lifecycle"] = lifecycle
	}
	return payload
}

func flowAutonomyConstraints(mode string) []string {
	_, constraints, _ := flowAutonomySummary(mode)
	return append([]string(nil), constraints...)
}

func inferFlowCreateRoles(in flowCreateAnswers) []string {
	text := strings.ToLower(strings.Join([]string{in.Name, in.Description, in.SuccessCriteria}, " "))
	seen := map[string]bool{}
	out := []string{}
	add := func(role string) {
		if role == "" || seen[role] {
			return
		}
		seen[role] = true
		out = append(out, role)
	}
	if strings.Contains(text, "analiz") || strings.Contains(text, "revis") || strings.Contains(text, "prioriz") || strings.Contains(text, "cartera") || strings.Contains(text, "mora") || strings.Contains(text, "deud") {
		add("analizar")
	}
	if strings.Contains(text, "redact") || strings.Contains(text, "borrador") || strings.Contains(text, "correo") || strings.Contains(text, "mensaje") || strings.Contains(text, "email") || strings.Contains(text, "prepar") {
		add("redactar")
	}
	if strings.Contains(text, "valid") || strings.Contains(text, "audit") || strings.Contains(text, "verific") {
		add("validar")
	}
	if strings.Contains(text, "enviar") || strings.Contains(text, "aplicar") || strings.Contains(text, "ejecut") {
		add("actuar")
	}
	if len(out) == 0 {
		add("analizar")
	}
	return out
}

func applyFlowCreateIntentModel(manifest *cliFlowManifest, in flowCreateAnswers) {
	if manifest == nil {
		return
	}
	_, constraints, autonomySummary := flowAutonomySummary(in.AutonomyMode)
	intent := manifest.Intent
	intent.Goal = firstNonEmpty(strings.TrimSpace(in.Name), intent.Goal, strings.TrimSpace(in.Description))
	intent.OperatorRole = firstNonEmpty(intent.OperatorRole, "staff")
	intent.SuccessCriteria = firstNonEmpty(strings.TrimSpace(in.SuccessCriteria), intent.SuccessCriteria)
	intent.Constraints = append([]string(nil), constraints...)
	intent.Roles = append([]string(nil), inferFlowCreateRoles(in)...)
	intent.CapabilityHint = firstNonEmpty(strings.TrimSpace(in.CapabilityHint), intent.CapabilityHint)
	intent.Description = firstNonEmpty(
		buildIntentFirstDescription(in),
		intent.Description,
		autonomySummary,
	)
	if strings.TrimSpace(intent.Goal) == "" {
		intent.Goal = strings.TrimSpace(in.Description)
	}
	manifest.Intent = intent
	if lifecycle := buildFlowCreateLifecycle(in); !emptyCLILifecycle(lifecycle) {
		manifest.Lifecycle = lifecycle
	}
}

func buildFlowCreateLifecycle(in flowCreateAnswers) cliFlowLifecycle {
	return cliFlowLifecycle{
		Entry: cliFlowLifecycleBinding{
			Framework:  strings.TrimSpace(in.EntryFramework),
			Capability: strings.TrimSpace(in.EntryCapability),
		},
		Tutela: cliFlowLifecycleBinding{
			Framework:  strings.TrimSpace(in.TutelaFramework),
			Capability: strings.TrimSpace(in.TutelaCapability),
		},
	}
}

func emptyCLILifecycle(lifecycle cliFlowLifecycle) bool {
	return cliLifecycleBindingLabel(lifecycle.Entry) == "" && cliLifecycleBindingLabel(lifecycle.Tutela) == ""
}

func cliLifecycleBindingLabel(binding cliFlowLifecycleBinding) string {
	framework := strings.TrimSpace(binding.Framework)
	capability := strings.TrimSpace(binding.Capability)
	switch {
	case framework != "" && capability != "":
		return framework + "." + capability
	case framework != "":
		return framework
	case capability != "":
		return capability
	default:
		return ""
	}
}

func fetchBusinessArtifacts(c *Client, businessID string) *cliBusinessArtifactsResponse {
	resp, err := c.get("/businesses/" + businessID + "/artifacts")
	if err != nil {
		return nil
	}
	var out cliBusinessArtifactsResponse
	if err := decodeAPIData(resp, &out); err != nil {
		return nil
	}
	return &out
}

func promptFlowCreateAnswers(in flowCreateAnswers, artifacts *cliBusinessArtifactsResponse) flowCreateAnswers {
	reader := bufio.NewReader(os.Stdin)
	if strings.TrimSpace(in.BusinessID) == "" {
		in.BusinessID = promptFlowField(reader, "business_id", in.BusinessID)
	}
	if artifacts != nil && len(artifacts.Artifacts) > 0 {
		fmt.Printf("%sArtifacts detectados%s %s\n", cyan, reset, strings.Join(artifacts.Artifacts, ", "))
	}
	in.Name = promptFlowField(reader, "Nombre corto del flujo", in.Name)
	in.Description = promptFlowField(reader, "Qué quieres automatizar", in.Description)
	in.CapabilityHint = promptFlowField(reader, "Capacidad inicial sugerida (opcional)", in.CapabilityHint)
	in.SuccessCriteria = promptFlowField(reader, "Cómo sabrás que salió bien", in.SuccessCriteria)
	autonomyPrompt := "Autonomía [approval/advisory/approved]"
	if current := strings.TrimSpace(in.AutonomyMode); current != "" {
		autonomyPrompt += " (" + current + ")"
	}
	value := promptFlowField(reader, autonomyPrompt, in.AutonomyMode)
	if value == "" {
		value = "approval"
	}
	in.AutonomyMode = strings.ToLower(strings.TrimSpace(value))
	return in
}

func promptFlowField(reader *bufio.Reader, label, current string) string {
	if strings.TrimSpace(current) != "" {
		fmt.Printf("%s [%s]: ", label, current)
	} else {
		fmt.Printf("%s: ", label)
	}
	raw, _ := reader.ReadString('\n')
	value := strings.TrimSpace(raw)
	if value == "" {
		return strings.TrimSpace(current)
	}
	return value
}

func handleFlowCreate(args []string) {
	fs := flag.NewFlagSet("flow create", flag.ExitOnError)
	businessID := fs.String("business", "", "business_id")
	name := fs.String("name", "", "nombre corto del flow")
	description := fs.String("description", "", "caso de uso en lenguaje natural")
	capability := fs.String("capability", "", "capacidad simple deseada")
	success := fs.String("success", "", "criterio de exito")
	autonomy := fs.String("autonomy", "approval", "approval|advisory|approved")
	entryFramework := fs.String("entry-framework", "", "framework que abre el flow")
	entryCapability := fs.String("entry-capability", "", "capability explícita del entry")
	tutelaFramework := fs.String("tutela-framework", "", "framework que conduce o tutela el caso")
	tutelaCapability := fs.String("tutela-capability", "", "capability explícita de tutela")
	max := fs.Int("max", 6, "cantidad maxima de suggestions")
	interactive := fs.Bool("interactive", false, "hacer preguntas guiadas")
	noCreate := fs.Bool("no-create", false, "no persistir, solo mostrar proposal")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)

	answers := flowCreateAnswers{
		BusinessID:       strings.TrimSpace(*businessID),
		Name:             strings.TrimSpace(*name),
		Description:      strings.TrimSpace(*description),
		CapabilityHint:   strings.TrimSpace(*capability),
		SuccessCriteria:  strings.TrimSpace(*success),
		AutonomyMode:     strings.TrimSpace(*autonomy),
		EntryFramework:   strings.TrimSpace(*entryFramework),
		EntryCapability:  strings.TrimSpace(*entryCapability),
		TutelaFramework:  strings.TrimSpace(*tutelaFramework),
		TutelaCapability: strings.TrimSpace(*tutelaCapability),
	}
	if answers.AutonomyMode == "" {
		answers.AutonomyMode = "approval"
	}

	c := newClient()
	var artifacts *cliBusinessArtifactsResponse
	if answers.BusinessID != "" {
		artifacts = fetchBusinessArtifacts(c, answers.BusinessID)
	}
	if *interactive || answers.BusinessID == "" || answers.Name == "" || answers.Description == "" {
		answers = promptFlowCreateAnswers(answers, artifacts)
		if artifacts == nil && answers.BusinessID != "" {
			artifacts = fetchBusinessArtifacts(c, answers.BusinessID)
		}
	}
	if answers.BusinessID == "" || answers.Name == "" || answers.Description == "" {
		fmt.Fprintln(os.Stderr, "usage: remora flow create --business <id> [--name <n>] [--description <texto>]")
		os.Exit(1)
	}

	resp, err := c.post("/flows/suggest", buildFlowCreateSuggestPayload(answers, *max))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	var suggestion cliFlowSuggestResponse
	if err := decodeAPIData(resp, &suggestion); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando suggestion: %v\n", err)
		os.Exit(1)
	}
	if suggestion.Proposal == nil {
		fmt.Fprintln(os.Stderr, "el backend no devolvió proposal derivada")
		os.Exit(1)
	}
	if suggestion.Proposal.Derivation != nil {
		suggestion.Proposal.Manifest.Derivation = suggestion.Proposal.Derivation
	}
	suggestion.Proposal.Manifest.BusinessID = answers.BusinessID
	applyFlowCreateIntentModel(&suggestion.Proposal.Manifest, answers)

	if *noCreate {
		if *jsonOut {
			printJSON(suggestion)
			return
		}
		printFlowCreatePreview(answers, artifacts, suggestion)
		fmt.Println()
		fmt.Print(formatFlowWorkbench(answers.Name, answers.Description, &suggestion.Proposal.Manifest, suggestion.Suggestions, nil))
		return
	}

	createdResp, err := c.post("/businesses/"+answers.BusinessID+"/flows", map[string]interface{}{
		"name":        answers.Name,
		"description": answers.Description,
		"manifest":    suggestion.Proposal.Manifest,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creando flow: %v\n", err)
		os.Exit(1)
	}
	var created cliFlowRecord
	if err := decodeAPIData(createdResp, &created); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando flow creado: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		printJSON(created)
		return
	}
	printFlowCreatePreview(answers, artifacts, suggestion)
	fmt.Printf("\n%sFlow creado%s %s (%s)\n\n", green, reset, created.Name, created.ID)
	fmt.Print(formatFlowWorkbench(created.Name, created.Description, created.Manifest, nil, created.Installed))
}

func printFlowCreatePreview(in flowCreateAnswers, artifacts *cliBusinessArtifactsResponse, suggestion cliFlowSuggestResponse) {
	label, _, summary := flowAutonomySummary(in.AutonomyMode)
	fmt.Printf("%sFlow create%s %s\n", cyan, reset, in.Name)
	fmt.Printf("  business: %s\n", in.BusinessID)
	if value := strings.TrimSpace(in.CapabilityHint); value != "" {
		fmt.Printf("  capacidad: %s\n", value)
	}
	if value := strings.TrimSpace(in.SuccessCriteria); value != "" {
		fmt.Printf("  exito: %s\n", value)
	}
	fmt.Printf("  autonomia: %s · %s\n", label, summary)
	if artifacts != nil && len(artifacts.Artifacts) > 0 {
		fmt.Printf("  artifacts: %s\n", strings.Join(artifacts.Artifacts, ", "))
	}
	fmt.Printf("  fuente: %s\n", suggestion.Source)
}

func handleFlowDraft(args []string) {
	fs := flag.NewFlagSet("flow draft", flag.ExitOnError)
	businessID := fs.String("business", "", "business_id")
	name := fs.String("name", "", "nombre del flow")
	description := fs.String("description", "", "caso de uso en lenguaje natural")
	max := fs.Int("max", 5, "cantidad maxima de suggestions")
	create := fs.Bool("create", false, "persistir el flow sugerido")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)
	if strings.TrimSpace(*businessID) == "" || strings.TrimSpace(*name) == "" || strings.TrimSpace(*description) == "" {
		fmt.Fprintln(os.Stderr, "usage: remora flow draft --business <id> --name <n> --description <texto> [--create]")
		os.Exit(1)
	}

	c := newClient()
	resp, err := c.post("/flows/suggest", map[string]interface{}{
		"business_id": *businessID,
		"name":        *name,
		"description": *description,
		"max":         *max,
		"intent": map[string]interface{}{
			"goal":          strings.TrimSpace(*name),
			"operator_role": "staff",
			"description":   strings.TrimSpace(*description),
			"roles":         inferFlowCreateRoles(flowCreateAnswers{Name: *name, Description: *description}),
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	var suggestion cliFlowSuggestResponse
	if err := decodeAPIData(resp, &suggestion); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando suggestion: %v\n", err)
		os.Exit(1)
	}
	if suggestion.Proposal == nil {
		fmt.Fprintln(os.Stderr, "el backend no devolvió proposal derivada")
		os.Exit(1)
	}

	if *create {
		createdResp, err := c.post("/businesses/"+*businessID+"/flows", map[string]interface{}{
			"name":        *name,
			"description": *description,
			"manifest":    suggestion.Proposal.Manifest,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creando flow: %v\n", err)
			os.Exit(1)
		}
		var created cliFlowRecord
		if err := decodeAPIData(createdResp, &created); err != nil {
			fmt.Fprintf(os.Stderr, "error decodificando flow creado: %v\n", err)
			os.Exit(1)
		}
		if *jsonOut {
			printJSON(created)
			return
		}
		fmt.Printf("%sFlow creado%s %s (%s)\n\n", green, reset, created.Name, created.ID)
		fmt.Print(formatFlowWorkbench(created.Name, created.Description, created.Manifest, nil, created.Installed))
		return
	}

	if *jsonOut {
		printJSON(suggestion)
		return
	}
	fmt.Printf("%sProposal%s fuente=%s\n", cyan, reset, suggestion.Source)
	for _, item := range suggestion.Suggestions {
		fmt.Printf("  - %s.%s · %s\n", item.Framework, item.Capability, item.Reason)
	}
	fmt.Println()
	fmt.Print(formatFlowWorkbench(*name, *description, &suggestion.Proposal.Manifest, suggestion.Suggestions, nil))
}

func handleFlowInspect(args []string) {
	fs := flag.NewFlagSet("flow inspect", flag.ExitOnError)
	flowID := fs.String("id", "", "flow id")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)
	if strings.TrimSpace(*flowID) == "" {
		fmt.Fprintln(os.Stderr, "usage: remora flow inspect --id <flow_id>")
		os.Exit(1)
	}

	record := mustFetchFlowRecord(*flowID)
	if *jsonOut {
		printJSON(record)
		return
	}
	fmt.Print(formatFlowWorkbench(record.Name, record.Description, record.Manifest, nil, record.Installed))
}

func handleFlowSimulate(args []string) {
	fs := flag.NewFlagSet("flow simulate", flag.ExitOnError)
	flowID := fs.String("id", "", "flow id")
	input := fs.String("input", "", "texto de prueba")
	fixtures := fs.String("fixtures", "", "artifacts separados por coma")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	_ = fs.Parse(args)
	if strings.TrimSpace(*flowID) == "" {
		fmt.Fprintln(os.Stderr, "usage: remora flow simulate --id <flow_id> [--fixtures a,b] [--input texto]")
		os.Exit(1)
	}

	record := mustFetchFlowRecord(*flowID)
	if record.Manifest == nil {
		fmt.Fprintln(os.Stderr, "flow sin manifest")
		os.Exit(1)
	}
	c := newClient()
	resp, err := c.post("/flows/simulate", map[string]interface{}{
		"flow":              record.Manifest,
		"input":             *input,
		"fixture_artifacts": parseCSVList(*fixtures),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error simulando flow: %v\n", err)
		os.Exit(1)
	}
	var result cliSimulationResult
	if err := decodeAPIData(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando simulacion: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		printJSON(result)
		return
	}
	fmt.Print(formatFlowWorkbench(record.Name, record.Description, record.Manifest, nil, record.Installed))
	fmt.Println()
	fmt.Printf("%sSimulacion%s valid=%t artifacts=%d\n", magenta, reset, result.Valid, len(result.Artifacts))
	for _, step := range result.Timeline {
		fmt.Printf("  - [%s] %s.%s (%s)\n", step.Status, step.Framework, step.Capability, step.Node)
		if len(step.MissingArtifacts) > 0 {
			fmt.Printf("      missing: %s\n", strings.Join(step.MissingArtifacts, ", "))
		}
		if len(step.Produces) > 0 {
			fmt.Printf("      produces: %s\n", strings.Join(step.Produces, ", "))
		}
	}
	if len(result.Validation.Errors) > 0 {
		fmt.Println()
		fmt.Printf("%sErrores%s\n", red, reset)
		for _, issue := range result.Validation.Errors {
			fmt.Printf("  - %s: %s\n", issue.Code, issue.Message)
		}
	}
}

func mustFetchFlowRecord(flowID string) cliFlowRecord {
	c := newClient()
	resp, err := c.get("/flows/" + flowID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	var record cliFlowRecord
	if err := decodeAPIData(resp, &record); err != nil {
		fmt.Fprintf(os.Stderr, "error decodificando flow: %v\n", err)
		os.Exit(1)
	}
	return record
}

func decodeAPIData(resp map[string]interface{}, out interface{}) error {
	data, ok := resp["data"]
	if !ok {
		return fmt.Errorf("respuesta sin data")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func formatFlowWorkbench(name, description string, manifest *cliFlowManifest, suggestions []cliFlowCapabilitySuggestion, installed *cliInstalledSnapshot) string {
	if manifest == nil {
		return ""
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%sWorkbench%s %s\n", yellow, reset, name)
	if strings.TrimSpace(description) != "" {
		fmt.Fprintf(&buf, "  caso: %s\n", description)
	}
	fmt.Fprintf(&buf, "  flow: %s  business: %s\n", manifest.ID, manifest.BusinessID)
	if goal := strings.TrimSpace(manifest.Intent.Goal); goal != "" {
		fmt.Fprintf(&buf, "  objetivo: %s\n", goal)
	}
	if len(manifest.Intent.Roles) > 0 {
		fmt.Fprintf(&buf, "  roles objetivo: %s\n", strings.Join(sortedList(manifest.Intent.Roles), ", "))
	}
	if success := strings.TrimSpace(manifest.Intent.SuccessCriteria); success != "" {
		fmt.Fprintf(&buf, "  exito: %s\n", success)
	}
	if len(manifest.Intent.Constraints) > 0 {
		fmt.Fprintf(&buf, "  restricciones: %s\n", strings.Join(sortedList(manifest.Intent.Constraints), ", "))
	}
	if !emptyCLILifecycle(manifest.Lifecycle) {
		fmt.Fprintf(&buf, "  %sLifecycle autoral%s\n", cyan, reset)
		if label := cliLifecycleBindingLabel(manifest.Lifecycle.Entry); label != "" {
			fmt.Fprintf(&buf, "    entry: %s\n", label)
		}
		if label := cliLifecycleBindingLabel(manifest.Lifecycle.Tutela); label != "" {
			fmt.Fprintf(&buf, "    tutela: %s\n", label)
		}
	}
	if len(suggestions) > 0 {
		fmt.Fprintf(&buf, "\n%sIntento%s\n", cyan, reset)
		for _, item := range suggestions {
			fmt.Fprintf(&buf, "  - %s.%s · %s\n", item.Framework, item.Capability, item.Reason)
		}
	}
	fmt.Fprintf(&buf, "\n%sAuthored%s\n", green, reset)
	for _, node := range manifest.Nodes {
		fmt.Fprintf(&buf, "  - %s → %s.%s\n", node.ID, node.Framework, node.Capability)
	}
	if manifest.Derivation == nil {
		return buf.String()
	}
	if goal := firstNonEmpty(manifest.Derivation.Grounding.DesiredCapability, manifest.Intent.Goal, manifest.Intent.Description); goal != "" {
		fmt.Fprintf(&buf, "\n%sGrounding%s %s\n", blue, reset, goal)
	}
	if len(manifest.Derivation.Grounding.BusinessArtifacts) > 0 {
		fmt.Fprintf(&buf, "  business_artifacts: %s\n", strings.Join(manifest.Derivation.Grounding.BusinessArtifacts, ", "))
	}
	if len(manifest.Derivation.Grounding.MissingArtifacts) > 0 {
		fmt.Fprintf(&buf, "  missing: %s\n", strings.Join(manifest.Derivation.Grounding.MissingArtifacts, ", "))
	}
	if len(manifest.Derivation.Grounding.UniversalRoles) > 0 {
		fmt.Fprintf(&buf, "  roles: %s\n", strings.Join(manifest.Derivation.Grounding.UniversalRoles, ", "))
	}
	if !emptyCLILifecycle(manifest.Derivation.Executable.Lifecycle) {
		fmt.Fprintf(&buf, "\n%sLifecycle derivado%s\n", blue, reset)
		if label := cliLifecycleBindingLabel(manifest.Derivation.Executable.Lifecycle.Entry); label != "" {
			fmt.Fprintf(&buf, "  entry: %s\n", label)
		}
		if label := cliLifecycleBindingLabel(manifest.Derivation.Executable.Lifecycle.Tutela); label != "" {
			fmt.Fprintf(&buf, "  tutela: %s\n", label)
		}
	}
	fmt.Fprintf(&buf, "\n%sDerived%s\n", magenta, reset)
	for _, node := range manifest.Derivation.Executable.Nodes {
		role := node.Role
		if role == "" {
			role = "pipeline"
		}
		fmt.Fprintf(&buf, "  - [%s] %s → %s.%s\n", role, node.ID, node.Framework, node.Capability)
	}
	if len(manifest.Derivation.Amendments) > 0 {
		fmt.Fprintf(&buf, "\n%sEnmiendas%s\n", yellow, reset)
		for _, amendment := range manifest.Derivation.Amendments {
			fmt.Fprintf(&buf, "  - %s\n", amendment.Summary)
		}
	}
	if len(manifest.Derivation.Contracts) > 0 {
		fmt.Fprintf(&buf, "\n%sContratos%s\n", cyan, reset)
		for _, contract := range manifest.Derivation.Contracts {
			fmt.Fprintf(&buf, "  - %s → in[%s] out[%s]\n", contract.NodeID, strings.Join(sortedList(contract.Requires), ", "), strings.Join(sortedList(contract.Produces), ", "))
		}
	}
	if len(manifest.Derivation.Handoffs) > 0 {
		fmt.Fprintf(&buf, "\n%sHandoffs%s\n", blue, reset)
		for _, handoff := range manifest.Derivation.Handoffs {
			artifacts := strings.Join(handoff.Artifacts, ", ")
			if artifacts == "" {
				artifacts = "sin artifacto compartido explicito"
			}
			fmt.Fprintf(&buf, "  - %s -> %s (%s)\n", handoff.FromNode, handoff.ToNode, artifacts)
		}
	}
	fmt.Fprintf(&buf, "\n%sInstalacion%s requires_install=%t\n", green, reset, manifest.Derivation.Install.RequiresInstall)
	if len(manifest.Derivation.Install.Capabilities) > 0 {
		fmt.Fprintf(&buf, "  capabilities: %s\n", strings.Join(manifest.Derivation.Install.Capabilities, ", "))
	}
	if installed != nil {
		fmt.Fprintf(&buf, "  installed: %t", installed.Installed)
		if strings.TrimSpace(installed.SchemaID) != "" {
			fmt.Fprintf(&buf, "  schema: %s", installed.SchemaID)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sortedList(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func compactStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func printJSON(v interface{}) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(raw))
}

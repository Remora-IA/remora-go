package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

const flowWorkbenchAPIBaseDefault = "http://localhost:8084/api/v1"

type flowWorkbenchClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type flowWorkbenchSSEEvent struct {
	Type string
	Data map[string]interface{}
}

type flowAPIEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

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
	ID         string   `json:"id"`
	Framework  string   `json:"framework"`
	Capability string   `json:"capability,omitempty"`
	Role       string   `json:"role,omitempty"`
	Policies   []string `json:"policies,omitempty"`
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
	Policies   []string           `json:"policies,omitempty"`
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

type cliFlowCompiledManifest struct {
	ID   string          `json:"id"`
	Flow cliFlowManifest `json:"flow"`
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
	CompiledID  string                `json:"compiled_id,omitempty"`
	Manifest    *cliFlowManifest      `json:"manifest"`
	Installed   *cliInstalledSnapshot `json:"installed,omitempty"`
}

type cliFlowCompileResponse struct {
	Manifest   cliFlowManifest         `json:"manifest"`
	Derivation *cliFlowDerivation      `json:"derivation,omitempty"`
	Compiled   cliFlowCompiledManifest `json:"compiled"`
}

type cliFlowCompiledRecord struct {
	Authored   cliFlowManifest         `json:"authored"`
	Derivation *cliFlowDerivation      `json:"derivation,omitempty"`
	Compiled   cliFlowCompiledManifest `json:"compiled"`
	CreatedAt  string                  `json:"created_at,omitempty"`
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
	Compiled   cliFlowCompiledManifest    `json:"compiled"`
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

type cliFlowSuggestResponse struct {
	Suggestions []cliFlowCapabilitySuggestion `json:"suggestions"`
	Source      string                        `json:"source"`
	Proposal    *cliFlowSuggestionProposal    `json:"proposal,omitempty"`
}

type cliBusinessArtifactsResponse struct {
	BusinessID string            `json:"business_id"`
	Artifacts  []string          `json:"artifacts"`
	Sources    map[string]string `json:"sources"`
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

type cliValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cliValidationResult struct {
	Valid      bool                 `json:"valid"`
	CompiledID string               `json:"compiled_id,omitempty"`
	Errors     []cliValidationIssue `json:"errors"`
	Warnings   []cliValidationIssue `json:"warnings"`
	Derivation *cliFlowDerivation   `json:"derivation,omitempty"`
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
	CompiledID string              `json:"compiled_id,omitempty"`
	Artifacts  []string            `json:"artifacts"`
	Timeline   []cliSimulationStep `json:"timeline"`
	Derivation *cliFlowDerivation  `json:"derivation,omitempty"`
	Validation cliValidationResult `json:"validation"`
}

type cliRunArtifact struct {
	Source string `json:"source,omitempty"`
}

type cliObservedHandoff struct {
	FromNode     string   `json:"from_node"`
	ToNode       string   `json:"to_node"`
	Artifacts    []string `json:"artifacts,omitempty"`
	Status       string   `json:"status,omitempty"`
	SegmentOwner string   `json:"segment_owner,omitempty"`
	Summary      string   `json:"summary"`
}

type cliRunStep struct {
	Node             string   `json:"node"`
	Framework        string   `json:"framework"`
	Capability       string   `json:"capability,omitempty"`
	Status           string   `json:"status"`
	ArtifactTypes    []string `json:"artifact_types,omitempty"`
	MissingArtifacts []string `json:"missing_artifacts,omitempty"`
}

type cliRunResult struct {
	RunID      string                    `json:"run_id"`
	Status     string                    `json:"status"`
	CompiledID string                    `json:"compiled_id,omitempty"`
	Valid      bool                      `json:"valid"`
	Timeline   []cliRunStep              `json:"timeline"`
	Handoffs   []cliObservedHandoff      `json:"handoffs,omitempty"`
	Artifacts  map[string]cliRunArtifact `json:"artifacts"`
	Validation cliValidationResult       `json:"validation"`
	Derivation *cliFlowDerivation        `json:"derivation,omitempty"`
}

type cliInstallationResult struct {
	FlowID       string   `json:"flow_id"`
	Status       string   `json:"status"`
	CompiledID   string   `json:"compiled_id,omitempty"`
	Already      bool     `json:"already_installed,omitempty"`
	ArtifactType string   `json:"artifact_type,omitempty"`
	Artifacts    []string `json:"artifacts,omitempty"`
	Summary      string   `json:"summary,omitempty"`
}

type cliFlowRunRequest struct {
	CompiledID       string                 `json:"compiled_id,omitempty"`
	Flow             *cliFlowManifest       `json:"flow,omitempty"`
	Input            string                 `json:"input,omitempty"`
	DryRun           bool                   `json:"dry_run"`
	Approved         bool                   `json:"approved,omitempty"`
	TestMode         bool                   `json:"test_mode,omitempty"`
	TestRecipient    string                 `json:"test_recipient,omitempty"`
	FixtureArtifacts []string               `json:"fixture_artifacts,omitempty"`
	InitialArtifacts map[string]interface{} `json:"initial_artifacts,omitempty"`
	MaxCycles        int                    `json:"max_cycles,omitempty"`
}

var flowWorkbenchInput io.Reader = os.Stdin

func cmdFlow(args []string) {
	if err := runFlowWorkbench(os.Stdout, args, nil); err != nil {
		fail(err)
	}
}

func runFlowWorkbench(out io.Writer, args []string, client *flowWorkbenchClient) error {
	if client == nil {
		client = newFlowWorkbenchClient()
	}
	if len(args) == 0 {
		printFlowWorkbenchUsage(out)
		return nil
	}
	switch args[0] {
	case "create":
		return runFlowCreate(out, client, args[1:])
	case "draft":
		return runFlowDraft(out, client, args[1:])
	case "compile":
		return runFlowCompile(out, client, args[1:])
	case "inspect":
		return runFlowInspect(out, client, args[1:])
	case "validate":
		return runFlowValidate(out, client, args[1:])
	case "simulate":
		return runFlowSimulate(out, client, args[1:])
	case "run":
		return runFlowRun(out, client, args[1:])
	case "install":
		return runFlowInstall(out, client, args[1:])
	case "replay":
		return runFlowReplay(out, client, args[1:])
	case "debug":
		return runFlowDebug(out, client, args[1:])
	case "help", "-h", "--help":
		printFlowWorkbenchUsage(out)
		return nil
	default:
		return fmt.Errorf("subcomando flow desconocido: %s", args[0])
	}
}

func newFlowWorkbenchClient() *flowWorkbenchClient {
	base := os.Getenv("REMORA_API_URL")
	if strings.TrimSpace(base) == "" {
		base = flowWorkbenchAPIBaseDefault
	}
	return &flowWorkbenchClient{
		BaseURL:    strings.TrimSuffix(base, "/"),
		Token:      os.Getenv("REMORA_API_TOKEN"),
		HTTPClient: http.DefaultClient,
	}
}

func (c *flowWorkbenchClient) get(path string, out interface{}) error {
	return c.doJSON(http.MethodGet, path, nil, out)
}

func (c *flowWorkbenchClient) post(path string, body interface{}, out interface{}) error {
	return c.doJSON(http.MethodPost, path, body, out)
}

func (c *flowWorkbenchClient) stream(path string, body interface{}, onEvent func(flowWorkbenchSSEEvent) bool) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	reader := bufio.NewReader(resp.Body)
	var eventType string
	var dataLines []string
	flush := func() bool {
		if len(dataLines) == 0 {
			eventType = ""
			return true
		}
		var payload map[string]interface{}
		_ = json.Unmarshal([]byte(strings.Join(dataLines, "\n")), &payload)
		dataLines = nil
		evt := flowWorkbenchSSEEvent{Type: eventType, Data: payload}
		eventType = ""
		return onEvent(evt)
	}
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(strings.TrimRight(line, "\n"), "\r")
			switch {
			case line == "":
				if !flush() {
					return nil
				}
			case strings.HasPrefix(line, "event:"):
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if err == io.EOF {
			flush()
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (c *flowWorkbenchClient) doJSON(method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope flowAPIEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 400 {
		if strings.TrimSpace(envelope.Error) != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, envelope.Error)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func printFlowWorkbenchUsage(out io.Writer) {
	fmt.Fprint(out, `flow workbench

Uso:
  flujo flow create --business <business_id> [--name <nombre>] [--description <texto>]
  flujo flow draft --business <business_id> --name <nombre> --description <texto> [--create]
  flujo flow compile --id <flow_id>
  flujo flow inspect --id <flow_id>
  flujo flow validate --id <flow_id>
  flujo flow simulate --id <flow_id> [--fixtures a,b] [--input texto]
  flujo flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]
  flujo flow install --id <flow_id> [--reconfigure]
  flujo flow replay --run <run_id>
  flujo flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on handoff,needs_input,approval]
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

func buildFlowCreateIntent(in flowCreateAnswers) cliFlowIntent {
	_, constraints, _ := flowAutonomySummary(in.AutonomyMode)
	return cliFlowIntent{
		Goal:            firstNonEmpty(strings.TrimSpace(in.Name), strings.TrimSpace(in.Description), strings.TrimSpace(in.CapabilityHint)),
		OperatorRole:    "staff",
		SuccessCriteria: strings.TrimSpace(in.SuccessCriteria),
		Constraints:     append([]string(nil), constraints...),
		Description:     buildIntentFirstDescription(in),
		Roles:           inferCLIIntentRoles(in.Name, in.Description, in.SuccessCriteria),
		CapabilityHint:  strings.TrimSpace(in.CapabilityHint),
	}
}

func buildFlowCreateSuggestPayload(in flowCreateAnswers, max int) map[string]interface{} {
	intent := buildFlowCreateIntent(in)
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

func applyFlowCreateIntentHints(manifest *cliFlowManifest, in flowCreateAnswers) {
	if manifest == nil {
		return
	}
	model := buildFlowCreateIntent(in)
	intent := manifest.Intent
	intent.Goal = firstNonEmpty(model.Goal, intent.Goal, intent.Description)
	intent.OperatorRole = firstNonEmpty(intent.OperatorRole, model.OperatorRole, "staff")
	intent.SuccessCriteria = firstNonEmpty(model.SuccessCriteria, intent.SuccessCriteria)
	intent.Constraints = append([]string(nil), model.Constraints...)
	if len(model.Roles) > 0 {
		intent.Roles = append([]string(nil), model.Roles...)
	} else {
		intent.Roles = append([]string(nil), intent.Roles...)
	}
	intent.CapabilityHint = firstNonEmpty(model.CapabilityHint, intent.CapabilityHint)
	intent.Description = firstNonEmpty(model.Description, intent.Description)
	manifest.Intent = intent
	manifest.BusinessID = strings.TrimSpace(in.BusinessID)
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

func emptyCLILifecycle(lifecycle cliFlowLifecycle) bool {
	return cliLifecycleBindingLabel(lifecycle.Entry) == "" && cliLifecycleBindingLabel(lifecycle.Tutela) == ""
}

func fetchBusinessArtifacts(client *flowWorkbenchClient, businessID string) *cliBusinessArtifactsResponse {
	if strings.TrimSpace(businessID) == "" {
		return nil
	}
	var artifacts cliBusinessArtifactsResponse
	if err := client.get("/businesses/"+businessID+"/artifacts", &artifacts); err != nil {
		return nil
	}
	return &artifacts
}

func promptFlowCreateAnswers(out io.Writer, client *flowWorkbenchClient, in flowCreateAnswers) (flowCreateAnswers, *cliBusinessArtifactsResponse, error) {
	reader := bufio.NewReader(flowWorkbenchInput)
	if strings.TrimSpace(in.BusinessID) == "" {
		in.BusinessID = promptFlowField(out, reader, "business_id", in.BusinessID)
	}
	artifacts := fetchBusinessArtifacts(client, in.BusinessID)
	if artifacts != nil && len(artifacts.Artifacts) > 0 {
		fmt.Fprintf(out, "Artifacts detectados %s\n", strings.Join(sortedList(artifacts.Artifacts), ", "))
	}
	in.Name = promptFlowField(out, reader, "Qué quieres automatizar (nombre corto)", in.Name)
	in.Description = promptFlowField(out, reader, "Qué quieres automatizar", in.Description)
	in.CapabilityHint = promptFlowField(out, reader, "Capacidad inicial", in.CapabilityHint)
	in.SuccessCriteria = promptFlowField(out, reader, "Cómo sabrás que salió bien", in.SuccessCriteria)
	autonomyPrompt := "Autonomía [approval/advisory/approved]"
	if current := strings.TrimSpace(in.AutonomyMode); current != "" {
		autonomyPrompt += " (" + current + ")"
	}
	in.AutonomyMode = strings.ToLower(strings.TrimSpace(promptFlowField(out, reader, autonomyPrompt, in.AutonomyMode)))
	if in.AutonomyMode == "" {
		in.AutonomyMode = "approval"
	}
	return in, artifacts, nil
}

func promptFlowField(out io.Writer, reader *bufio.Reader, label, current string) string {
	if strings.TrimSpace(current) != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, current)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	raw, _ := reader.ReadString('\n')
	value := strings.TrimSpace(raw)
	if value == "" {
		return strings.TrimSpace(current)
	}
	return value
}

func printFlowCreatePreview(out io.Writer, in flowCreateAnswers, artifacts *cliBusinessArtifactsResponse, source string) {
	label, _, summary := flowAutonomySummary(in.AutonomyMode)
	fmt.Fprintf(out, "Flow create %s\n", in.Name)
	fmt.Fprintf(out, "  business: %s\n", in.BusinessID)
	if value := strings.TrimSpace(in.CapabilityHint); value != "" {
		fmt.Fprintf(out, "  capacidad: %s\n", value)
	}
	if value := strings.TrimSpace(in.SuccessCriteria); value != "" {
		fmt.Fprintf(out, "  exito: %s\n", value)
	}
	fmt.Fprintf(out, "  autonomia: %s · %s\n", label, summary)
	if artifacts != nil && len(artifacts.Artifacts) > 0 {
		fmt.Fprintf(out, "  artifacts: %s\n", strings.Join(sortedList(artifacts.Artifacts), ", "))
	}
	if strings.TrimSpace(source) != "" {
		fmt.Fprintf(out, "  fuente: %s\n", source)
	}
}

func runFlowCreate(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow create")
	businessID := fs.String("business", "", "business id")
	name := fs.String("name", "", "nombre corto del flow")
	description := fs.String("description", "", "caso de uso")
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
	if err := fs.Parse(args); err != nil {
		return err
	}

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

	var artifacts *cliBusinessArtifactsResponse
	if *interactive || answers.BusinessID == "" || answers.Name == "" || answers.Description == "" {
		var err error
		answers, artifacts, err = promptFlowCreateAnswers(out, client, answers)
		if err != nil {
			return err
		}
	} else {
		artifacts = fetchBusinessArtifacts(client, answers.BusinessID)
	}
	if strings.TrimSpace(answers.BusinessID) == "" || strings.TrimSpace(answers.Name) == "" || strings.TrimSpace(answers.Description) == "" {
		return fmt.Errorf("usage: flujo flow create --business <business_id> [--name <nombre>] [--description <texto>]")
	}

	var suggestion cliFlowSuggestResponse
	if err := client.post("/flows/suggest", buildFlowCreateSuggestPayload(answers, *max), &suggestion); err != nil {
		return err
	}
	if suggestion.Proposal == nil {
		return fmt.Errorf("el backend no devolvió proposal")
	}
	suggestion.Proposal.Manifest.BusinessID = answers.BusinessID
	applyFlowCreateIntentHints(&suggestion.Proposal.Manifest, answers)

	compile, err := compileFlowRecord(client, suggestion.Proposal.Manifest)
	if err != nil {
		return err
	}
	record := cliFlowRecord{
		ID:          compile.Manifest.ID,
		BusinessID:  answers.BusinessID,
		Name:        answers.Name,
		Description: answers.Description,
		Status:      "proposal",
		Manifest:    &compile.Manifest,
	}
	if *noCreate {
		if *jsonOut {
			return printJSON(out, map[string]interface{}{
				"suggestion": suggestion,
				"compile":    compile,
			})
		}
		printFlowCreatePreview(out, answers, artifacts, suggestion.Source)
		fmt.Fprintln(out)
		fmt.Fprint(out, formatFlowWorkbench(record, compile))
		return nil
	}

	var created cliFlowRecord
	if err := client.post("/businesses/"+answers.BusinessID+"/flows", map[string]interface{}{
		"name":        answers.Name,
		"description": answers.Description,
		"manifest":    compile.Manifest,
	}, &created); err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, created)
	}
	printFlowCreatePreview(out, answers, artifacts, suggestion.Source)
	fmt.Fprintln(out)
	fmt.Fprint(out, formatFlowWorkbench(created, compile))
	return nil
}

func runFlowDraft(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow draft")
	businessID := fs.String("business", "", "business id")
	name := fs.String("name", "", "nombre del flow")
	description := fs.String("description", "", "caso de uso")
	max := fs.Int("max", 5, "cantidad maxima de suggestions")
	create := fs.Bool("create", false, "persistir el flow sugerido")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*businessID) == "" || strings.TrimSpace(*name) == "" || strings.TrimSpace(*description) == "" {
		return fmt.Errorf("usage: flujo flow draft --business <business_id> --name <nombre> --description <texto> [--create]")
	}
	var suggestion cliFlowSuggestResponse
	if err := client.post("/flows/suggest", buildFlowDraftSuggestPayload(*businessID, *name, *description, *max), &suggestion); err != nil {
		return err
	}
	if suggestion.Proposal == nil {
		return fmt.Errorf("el backend no devolvió proposal")
	}
	if *create {
		var created cliFlowRecord
		if err := client.post("/businesses/"+*businessID+"/flows", map[string]interface{}{
			"name":        *name,
			"description": *description,
			"manifest":    suggestion.Proposal.Manifest,
		}, &created); err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(out, created)
		}
		fmt.Fprint(out, formatFlowWorkbench(created, cliFlowCompileResponse{
			Manifest:   suggestion.Proposal.Manifest,
			Derivation: suggestion.Proposal.Derivation,
			Compiled:   suggestion.Proposal.Compiled,
		}))
		return nil
	}
	if *jsonOut {
		return printJSON(out, suggestion)
	}
	record := cliFlowRecord{
		ID:          suggestion.Proposal.Manifest.ID,
		BusinessID:  *businessID,
		Name:        *name,
		Description: *description,
		Status:      "proposal",
		Manifest:    &suggestion.Proposal.Manifest,
	}
	fmt.Fprintf(out, "Proposal fuente=%s\n", suggestion.Source)
	for _, item := range suggestion.Suggestions {
		fmt.Fprintf(out, "  - %s.%s · %s\n", item.Framework, item.Capability, item.Reason)
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, formatFlowWorkbench(record, cliFlowCompileResponse{
		Manifest:   suggestion.Proposal.Manifest,
		Derivation: suggestion.Proposal.Derivation,
		Compiled:   suggestion.Proposal.Compiled,
	}))
	return nil
}

func buildFlowDraftSuggestPayload(businessID, name, description string, max int) map[string]interface{} {
	intent := cliFlowIntent{
		Goal:         strings.TrimSpace(name),
		OperatorRole: "staff",
		Description:  strings.TrimSpace(description),
		Roles:        inferCLIIntentRoles(name, description),
	}
	return map[string]interface{}{
		"business_id": businessID,
		"name":        name,
		"description": description,
		"max":         max,
		"intent":      intent,
	}
}

func inferCLIIntentRoles(values ...string) []string {
	text := strings.ToLower(strings.Join(values, " "))
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

func runFlowCompile(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow compile")
	flowID := fs.String("id", "", "flow id")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow compile --id <flow_id>")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, compile)
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	return nil
}

func runFlowInspect(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow inspect")
	flowID := fs.String("id", "", "flow id")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow inspect --id <flow_id>")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, record)
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	return nil
}

func runFlowValidate(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow validate")
	flowID := fs.String("id", "", "flow id")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow validate --id <flow_id>")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	var result cliValidationResult
	if err := client.post("/flows/validate", map[string]interface{}{"compiled_id": compile.Compiled.ID}, &result); err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	fmt.Fprint(out, formatValidationSummary(result))
	return nil
}

func runFlowSimulate(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow simulate")
	flowID := fs.String("id", "", "flow id")
	input := fs.String("input", "", "texto de prueba")
	fixtures := fs.String("fixtures", "", "artifacts separados por coma")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow simulate --id <flow_id> [--fixtures a,b] [--input texto]")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	var result cliSimulationResult
	if err := client.post("/flows/simulate", map[string]interface{}{
		"compiled_id":       compile.Compiled.ID,
		"input":             *input,
		"fixture_artifacts": parseCSVList(*fixtures),
	}, &result); err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	fmt.Fprint(out, formatSimulationSummary(result))
	return nil
}

func runFlowRun(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow run")
	flowID := fs.String("id", "", "flow id")
	input := fs.String("input", "", "texto de prueba")
	fixtures := fs.String("fixtures", "", "artifacts separados por coma")
	dryRun := fs.Bool("dry-run", false, "correr sin side effects reales")
	approve := fs.Bool("approve", false, "marcar approval como concedido")
	testMode := fs.Bool("test-mode", false, "forzar recipient de prueba")
	testRecipient := fs.String("test-recipient", "", "destinatario de prueba")
	maxCycles := fs.Int("max-cycles", 0, "limite de ciclos")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow run --id <flow_id> [--fixtures a,b] [--input texto] [--dry-run]")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	req := cliFlowRunRequest{
		CompiledID:       compile.Compiled.ID,
		Input:            *input,
		DryRun:           *dryRun,
		Approved:         *approve,
		TestMode:         *testMode,
		TestRecipient:    strings.TrimSpace(*testRecipient),
		FixtureArtifacts: parseCSVList(*fixtures),
		MaxCycles:        *maxCycles,
	}
	var result cliRunResult
	if err := client.post("/flows/run", req, &result); err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	fmt.Fprint(out, formatRunSummary(result))
	return nil
}

func runFlowInstall(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow install")
	flowID := fs.String("id", "", "flow id")
	reconfigure := fs.Bool("reconfigure", false, "forzar reinstalacion")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow install --id <flow_id> [--reconfigure]")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	var result cliInstallationResult
	if err := client.post("/flows/"+*flowID+"/install", map[string]interface{}{"reconfigure": *reconfigure, "compiled_id": compile.Compiled.ID}, &result); err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	fmt.Fprint(out, formatInstallationSummary(result))
	return nil
}

func loadFlowWorkbenchRecord(client *flowWorkbenchClient, flowID string) (cliFlowRecord, cliFlowCompileResponse, error) {
	record, err := fetchFlowRecord(client, flowID)
	if err != nil {
		return cliFlowRecord{}, cliFlowCompileResponse{}, err
	}
	if record.Manifest == nil {
		return cliFlowRecord{}, cliFlowCompileResponse{}, fmt.Errorf("flow sin manifest: %s", flowID)
	}
	if strings.TrimSpace(record.CompiledID) != "" {
		if compiled, err := fetchCompiledFlowRecord(client, record.CompiledID); err == nil {
			return record, cliFlowCompileResponse{
				Manifest:   compiled.Authored,
				Derivation: compiled.Derivation,
				Compiled:   compiled.Compiled,
			}, nil
		}
	}
	compile, err := compileFlowRecord(client, *record.Manifest)
	if err != nil {
		return cliFlowRecord{}, cliFlowCompileResponse{}, err
	}
	return record, compile, nil
}

func runFlowReplay(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow replay")
	runID := fs.String("run", "", "run id")
	jsonOut := fs.Bool("json", false, "imprimir JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*runID) == "" {
		return fmt.Errorf("usage: flujo flow replay --run <run_id>")
	}
	var result cliRunResult
	if err := client.get("/flows/runs/"+*runID, &result); err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "Replay run %s\n", result.RunID)
	fmt.Fprint(out, formatRunSummary(result))
	return nil
}

func runFlowDebug(out io.Writer, client *flowWorkbenchClient, args []string) error {
	fs := newFlowFlagSet("flow debug")
	flowID := fs.String("id", "", "flow id")
	input := fs.String("input", "", "texto de prueba")
	fixtures := fs.String("fixtures", "", "artifacts separados por coma")
	stepMode := fs.Bool("step", false, "pausar en cada step_complete")
	breakOn := fs.String("break-on", "", "handoff,needs_input,approval")
	dryRun := fs.Bool("dry-run", true, "usar dry-run por defecto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*flowID) == "" {
		return fmt.Errorf("usage: flujo flow debug --id <flow_id> [--fixtures a,b] [--input texto] [--step] [--break-on ...]")
	}
	record, compile, err := loadFlowWorkbenchRecord(client, *flowID)
	if err != nil {
		return err
	}
	fmt.Fprint(out, formatFlowWorkbench(record, compile))
	fmt.Fprintln(out, "\nDebug stream")
	breaks := map[string]bool{}
	for _, item := range parseCSVList(*breakOn) {
		breaks[item] = true
	}
	reader := bufio.NewReader(os.Stdin)
	var previousFramework string
	return client.stream("/flows/run/stream", map[string]interface{}{
		"compiled_id":       compile.Compiled.ID,
		"input":             *input,
		"dry_run":           *dryRun,
		"fixture_artifacts": parseCSVList(*fixtures),
	}, func(evt flowWorkbenchSSEEvent) bool {
		if evt.Type == "flow_complete" {
			var result cliRunResult
			if decodeMapInto(evt.Data, &result) == nil {
				fmt.Fprint(out, formatRunSummary(result))
			}
			return false
		}
		if evt.Type != "step_complete" && evt.Type != "step_start" {
			return true
		}
		var payload struct {
			Step cliRunStep `json:"step"`
		}
		if decodeMapInto(evt.Data, &payload) != nil {
			return true
		}
		step := payload.Step
		fmt.Fprintf(out, "  [%s] %s.%s (%s)\n", step.Status, step.Framework, step.Capability, step.Node)
		shouldPause := *stepMode
		if breaks["needs_input"] && step.Status == "needs_input" {
			shouldPause = true
		}
		if breaks["approval"] && step.Status == "awaiting_approval" {
			shouldPause = true
		}
		if breaks["handoff"] && previousFramework != "" && previousFramework != step.Framework {
			shouldPause = true
		}
		if evt.Type == "step_complete" && step.Status == "completed" {
			previousFramework = step.Framework
		}
		if shouldPause {
			fmt.Fprint(out, "    pausa debug — enter para continuar...")
			_, _ = reader.ReadString('\n')
		}
		return true
	})
}

func fetchFlowRecord(client *flowWorkbenchClient, flowID string) (cliFlowRecord, error) {
	var record cliFlowRecord
	err := client.get("/flows/"+flowID, &record)
	return record, err
}

func fetchCompiledFlowRecord(client *flowWorkbenchClient, compiledID string) (cliFlowCompiledRecord, error) {
	var record cliFlowCompiledRecord
	err := client.get("/flows/compiled/"+compiledID, &record)
	return record, err
}

func compileFlowRecord(client *flowWorkbenchClient, manifest cliFlowManifest) (cliFlowCompileResponse, error) {
	var compile cliFlowCompileResponse
	err := client.post("/flows/workbench/compile", map[string]interface{}{"flow": manifest}, &compile)
	return compile, err
}

func formatFlowWorkbench(record cliFlowRecord, compile cliFlowCompileResponse) string {
	manifest := compile.Manifest
	manifest.Derivation = compile.Derivation
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Workbench %s\n", record.Name)
	if strings.TrimSpace(record.Description) != "" {
		fmt.Fprintf(&buf, "  caso: %s\n", record.Description)
	}
	fmt.Fprintf(&buf, "  flow: %s  business: %s  status: %s  compiled: %s\n", record.ID, record.BusinessID, record.Status, firstNonEmpty(compile.Compiled.ID, record.CompiledID))
	if goal := firstNonEmpty(manifest.Intent.Goal, manifest.Intent.Description); goal != "" {
		fmt.Fprintf(&buf, "\nIntent\n  %s\n", goal)
	}
	if strings.TrimSpace(manifest.Intent.CapabilityHint) != "" {
		fmt.Fprintf(&buf, "  capability_hint: %s\n", manifest.Intent.CapabilityHint)
	}
	if strings.TrimSpace(manifest.Intent.SuccessCriteria) != "" {
		fmt.Fprintf(&buf, "  success: %s\n", manifest.Intent.SuccessCriteria)
	}
	if len(manifest.Intent.Roles) > 0 {
		fmt.Fprintf(&buf, "  roles: %s\n", strings.Join(sortedList(manifest.Intent.Roles), ", "))
	}
	if len(manifest.Intent.Constraints) > 0 {
		fmt.Fprintf(&buf, "  constraints: %s\n", strings.Join(sortedList(manifest.Intent.Constraints), ", "))
	}
	if len(manifest.Policies) > 0 {
		fmt.Fprintf(&buf, "  policies: %s\n", strings.Join(sortedList(manifest.Policies), ", "))
	}
	if !emptyCLILifecycle(manifest.Lifecycle) {
		fmt.Fprintf(&buf, "\nLifecycle autoral\n")
		if label := cliLifecycleBindingLabel(manifest.Lifecycle.Entry); label != "" {
			fmt.Fprintf(&buf, "  entry: %s\n", label)
		}
		if label := cliLifecycleBindingLabel(manifest.Lifecycle.Tutela); label != "" {
			fmt.Fprintf(&buf, "  tutela: %s\n", label)
		}
	}
	fmt.Fprintf(&buf, "\nAuthored\n")
	for _, node := range manifest.Nodes {
		fmt.Fprintf(&buf, "  - %s -> %s.%s\n", node.ID, node.Framework, node.Capability)
	}
	if manifest.Derivation == nil {
		return buf.String()
	}
	if goal := firstNonEmpty(manifest.Derivation.Grounding.DesiredCapability); goal != "" {
		fmt.Fprintf(&buf, "\nGrounding\n  goal: %s\n", goal)
	}
	if len(manifest.Derivation.Grounding.BusinessArtifacts) > 0 {
		fmt.Fprintf(&buf, "  business_artifacts: %s\n", strings.Join(sortedList(manifest.Derivation.Grounding.BusinessArtifacts), ", "))
	}
	if len(manifest.Derivation.Grounding.MissingArtifacts) > 0 {
		fmt.Fprintf(&buf, "  missing: %s\n", strings.Join(sortedList(manifest.Derivation.Grounding.MissingArtifacts), ", "))
	}
	if len(manifest.Derivation.Grounding.UniversalRoles) > 0 {
		fmt.Fprintf(&buf, "  roles: %s\n", strings.Join(sortedList(manifest.Derivation.Grounding.UniversalRoles), ", "))
	}
	if !emptyCLILifecycle(manifest.Derivation.Executable.Lifecycle) {
		fmt.Fprintf(&buf, "\nLifecycle derivado\n")
		if label := cliLifecycleBindingLabel(manifest.Derivation.Executable.Lifecycle.Entry); label != "" {
			fmt.Fprintf(&buf, "  entry: %s\n", label)
		}
		if label := cliLifecycleBindingLabel(manifest.Derivation.Executable.Lifecycle.Tutela); label != "" {
			fmt.Fprintf(&buf, "  tutela: %s\n", label)
		}
	}
	fmt.Fprintf(&buf, "\nDerived\n")
	for _, node := range manifest.Derivation.Executable.Nodes {
		role := firstNonEmpty(node.Role, "pipeline")
		fmt.Fprintf(&buf, "  - [%s] %s -> %s.%s\n", role, node.ID, node.Framework, node.Capability)
	}
	fmt.Fprintf(&buf, "\nCompiled\n  id: %s\n", compile.Compiled.ID)
	for _, node := range compile.Compiled.Flow.Nodes {
		role := firstNonEmpty(node.Role, "pipeline")
		fmt.Fprintf(&buf, "  - [%s] %s -> %s.%s\n", role, node.ID, node.Framework, node.Capability)
	}
	if len(manifest.Derivation.Amendments) > 0 {
		fmt.Fprintf(&buf, "\nEnmiendas\n")
		for _, amendment := range manifest.Derivation.Amendments {
			fmt.Fprintf(&buf, "  - %s\n", amendment.Summary)
		}
	}
	if len(manifest.Derivation.Contracts) > 0 {
		fmt.Fprintf(&buf, "\nContratos\n")
		for _, contract := range manifest.Derivation.Contracts {
			inputs := sortedList(append(append([]string{}, contract.Inputs...), contract.Requires...))
			outputs := sortedList(append(append([]string{}, contract.Outputs...), contract.Produces...))
			fmt.Fprintf(&buf, "  - %s cmd=%s in[%s] out[%s]", contract.NodeID, contract.Command, strings.Join(inputs, ", "), strings.Join(outputs, ", "))
			if len(contract.Policies) > 0 {
				fmt.Fprintf(&buf, " policies[%s]", strings.Join(sortedList(contract.Policies), ", "))
			}
			buf.WriteByte('\n')
		}
	}
	if len(manifest.Derivation.Handoffs) > 0 {
		fmt.Fprintf(&buf, "\nHandoffs\n")
		for _, handoff := range manifest.Derivation.Handoffs {
			fmt.Fprintf(&buf, "  - %s -> %s owner=%s artifacts=%s\n", handoff.FromNode, handoff.ToNode, firstNonEmpty(handoff.Ownership, "pipeline"), nonEmptyJoin(sortedList(handoff.Artifacts), ", ", "sin artifacto compartido explicito"))
		}
	}
	fmt.Fprintf(&buf, "\nInstalacion\n  requires_install=%t\n", manifest.Derivation.Install.RequiresInstall)
	if len(manifest.Derivation.Install.Capabilities) > 0 {
		fmt.Fprintf(&buf, "  capabilities: %s\n", strings.Join(sortedList(manifest.Derivation.Install.Capabilities), ", "))
	}
	if record.Installed != nil {
		fmt.Fprintf(&buf, "  installed: %t\n", record.Installed.Installed)
		if strings.TrimSpace(record.Installed.SchemaID) != "" {
			fmt.Fprintf(&buf, "  schema: %s\n", record.Installed.SchemaID)
		}
	}
	return buf.String()
}

func formatValidationSummary(result cliValidationResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nValidacion\n  compiled: %s  valid: %t\n", result.CompiledID, result.Valid)
	for _, issue := range result.Errors {
		fmt.Fprintf(&buf, "  error %s: %s\n", issue.Code, issue.Message)
	}
	for _, issue := range result.Warnings {
		fmt.Fprintf(&buf, "  warning %s: %s\n", issue.Code, issue.Message)
	}
	return buf.String()
}

func formatSimulationSummary(result cliSimulationResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nSimulacion\n  compiled: %s  valid: %t  artifacts: %d\n", result.CompiledID, result.Valid, len(result.Artifacts))
	for _, step := range result.Timeline {
		fmt.Fprintf(&buf, "  - [%s] %s.%s (%s)\n", step.Status, step.Framework, step.Capability, step.Node)
		if len(step.MissingArtifacts) > 0 {
			fmt.Fprintf(&buf, "      missing: %s\n", strings.Join(sortedList(step.MissingArtifacts), ", "))
		}
		if len(step.Produces) > 0 {
			fmt.Fprintf(&buf, "      produces: %s\n", strings.Join(sortedList(step.Produces), ", "))
		}
	}
	for _, issue := range result.Validation.Errors {
		fmt.Fprintf(&buf, "  error %s: %s\n", issue.Code, issue.Message)
	}
	return buf.String()
}

func formatRunSummary(result cliRunResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nRun\n  run_id: %s  status: %s  compiled: %s  valid: %t\n", result.RunID, result.Status, result.CompiledID, result.Valid)
	if keys := sortedArtifactKeys(result.Artifacts); len(keys) > 0 {
		fmt.Fprintf(&buf, "  artifacts: %s\n", strings.Join(keys, ", "))
	}
	if len(result.Timeline) > 0 {
		fmt.Fprintf(&buf, "  timeline:\n")
		for _, step := range result.Timeline {
			fmt.Fprintf(&buf, "    - [%s] %s.%s (%s)\n", step.Status, step.Framework, step.Capability, step.Node)
			if len(step.ArtifactTypes) > 0 {
				fmt.Fprintf(&buf, "        artifacts: %s\n", strings.Join(sortedList(step.ArtifactTypes), ", "))
			}
			if len(step.MissingArtifacts) > 0 {
				fmt.Fprintf(&buf, "        missing: %s\n", strings.Join(sortedList(step.MissingArtifacts), ", "))
			}
		}
	}
	if len(result.Handoffs) > 0 {
		fmt.Fprintf(&buf, "  handoffs:\n")
		for _, handoff := range result.Handoffs {
			fmt.Fprintf(&buf, "    - %s -> %s owner=%s status=%s artifacts=%s\n", handoff.FromNode, handoff.ToNode, firstNonEmpty(handoff.SegmentOwner, "n/a"), firstNonEmpty(handoff.Status, "unknown"), nonEmptyJoin(sortedList(handoff.Artifacts), ", ", "sin artifactos"))
		}
	}
	for _, issue := range result.Validation.Errors {
		fmt.Fprintf(&buf, "  error %s: %s\n", issue.Code, issue.Message)
	}
	return buf.String()
}

func formatInstallationSummary(result cliInstallationResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nInstalacion ejecutada\n  status: %s  compiled: %s\n", result.Status, result.CompiledID)
	if result.Already {
		fmt.Fprintf(&buf, "  already: true\n")
	}
	if strings.TrimSpace(result.ArtifactType) != "" {
		fmt.Fprintf(&buf, "  artifact_type: %s\n", result.ArtifactType)
	}
	if len(result.Artifacts) > 0 {
		fmt.Fprintf(&buf, "  artifacts: %s\n", strings.Join(sortedList(result.Artifacts), ", "))
	}
	if strings.TrimSpace(result.Summary) != "" {
		fmt.Fprintf(&buf, "  summary: %s\n", result.Summary)
	}
	return buf.String()
}

func newFlowFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
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

func sortedArtifactKeys(in map[string]cliRunArtifact) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
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

func nonEmptyJoin(values []string, sep, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return strings.Join(values, sep)
}

func printJSON(out io.Writer, v interface{}) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(raw))
	return err
}

func decodeMapInto(raw map[string]interface{}, out interface{}) error {
	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}

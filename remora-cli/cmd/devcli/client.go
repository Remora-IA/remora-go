package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// ANSI colors for terminal output
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
	white   = "\033[37m"
)

// Framework colors for visual distinction
var frameworkColors = map[string]string{
	"arquitecto": cyan,
	"auditor":    magenta,
	"bravo":      magenta,
	"critico":    magenta,
	"paladin":    yellow,
	"echo":       green,
	"alfa":       blue,
	"sabio":      blue,
	"foco":       yellow,
	"hosting":    cyan,
	"mecanico":   green,
	"mensajero":  cyan,
	"deployer":   red,
	"radar":      yellow,
	"indexa":     green,
}

func fwColor(name string) string {
	if c, ok := frameworkColors[name]; ok {
		return c
	}
	return white
}

// Client communicates with the remora backend API
type Client struct {
	BaseURL string
	Token   string
}

func newClient() *Client {
	base := os.Getenv("REMORA_API_URL")
	if base == "" {
		base = "http://localhost:8084"
	}
	return &Client{
		BaseURL: strings.TrimSuffix(base, "/"),
		Token:   os.Getenv("REMORA_API_TOKEN"),
	}
}

func (c *Client) get(path string) (map[string]interface{}, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) post(path string, body interface{}) (map[string]interface{}, error) {
	url := c.BaseURL + path
	jbody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jbody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// FrameworkInfo represents a framework's capabilities
type FrameworkInfo struct {
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	Capabilities  []string `json:"capabilities"`
	ExecutionMode string   `json:"execution_mode"`
	Produces      []string `json:"produces"`
	Requires      []string `json:"requires"`
	Description   string   `json:"description,omitempty"`
	AskVia        string   `json:"ask_via,omitempty"`
	Modes         []string `json:"modes,omitempty"`
	Testable      bool     `json:"testable"`
	Chainable     bool     `json:"chainable"`
	EnvKey        string   `json:"env_key,omitempty"`
}

// FlowManifest represents a flow definition
type FlowManifest struct {
	ID         string     `json:"id"`
	BusinessID string     `json:"business_id,omitempty"`
	Intent     FlowIntent `json:"intent,omitempty"`
	Nodes      []FlowNode `json:"nodes"`
	Edges      []FlowEdge `json:"edges,omitempty"`
	Policies   []string   `json:"policies,omitempty"`
}

type FlowIntent struct {
	Goal            string   `json:"goal,omitempty"`
	OperatorRole    string   `json:"operator_role,omitempty"`
	SuccessCriteria string   `json:"success_criteria,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Description     string   `json:"description,omitempty"`
}

type FlowNode struct {
	ID         string            `json:"id"`
	Framework  string            `json:"framework"`
	Capability string            `json:"capability,omitempty"`
	Command    string            `json:"command,omitempty"`
	Role       string            `json:"role,omitempty"`
	Inputs     []string          `json:"inputs,omitempty"`
	Outputs    []string          `json:"outputs,omitempty"`
	Requires   []string          `json:"requires,omitempty"`
	Produces   []string          `json:"produces,omitempty"`
	Policies   []string          `json:"policies,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

type FlowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FlowRunResult represents the result of running a flow
type FlowRunResult struct {
	RunID             string              `json:"run_id"`
	FlowID            string              `json:"flow_id,omitempty"`
	Status            string              `json:"status"`
	CompiledID        string              `json:"compiled_id,omitempty"`
	CyclesDone        int                 `json:"cycles_done,omitempty"`
	Valid             bool                `json:"valid"`
	DryRun            bool                `json:"dry_run"`
	TestMode          bool                `json:"test_mode,omitempty"`
	BusinessID        string              `json:"business_id,omitempty"`
	ExecutionOrder    []string            `json:"execution_order"`
	Timeline          []FlowRunStep       `json:"timeline"`
	Artifacts         map[string]Artifact `json:"artifacts"`
	Handoffs          []Handoff           `json:"handoffs,omitempty"`
	Validation        ValidationResult    `json:"validation"`
	Warnings          []ValidationIssue  `json:"warnings,omitempty"`
	NeedsInput        []RequiredInput     `json:"needs_input,omitempty"`
	CreatedAt         string              `json:"created_at"`
	FinishedAt        string              `json:"finished_at,omitempty"`
}

type FlowRunStep struct {
	Node           string           `json:"node"`
	Framework      string           `json:"framework"`
	Capability     string           `json:"capability,omitempty"`
	Command        string           `json:"command,omitempty"`
	Role            string           `json:"role,omitempty"`
	ResolutionMode string           `json:"resolution_mode,omitempty"`
	CycleIndex     int              `json:"cycle_index,omitempty"`
	Status         string           `json:"status"`
	Inputs         []string         `json:"inputs,omitempty"`
	Requires       []string         `json:"requires,omitempty"`
	Outputs        []string         `json:"outputs,omitempty"`
	Produces       []string         `json:"produces,omitempty"`
	StartedAt      string           `json:"started_at,omitempty"`
	FinishedAt     string           `json:"finished_at,omitempty"`
	DurationMs     int64            `json:"duration_ms,omitempty"`
	Error          string           `json:"error,omitempty"`
	HumanSummary   string           `json:"human_summary,omitempty"`
	StdoutPreview  string           `json:"stdout_preview,omitempty"`
	StderrPreview  string           `json:"stderr_preview,omitempty"`
	ExitCode       int              `json:"exit_code,omitempty"`
}

type Artifact struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	CreatedAt string      `json:"created_at"`
	Payload   interface{} `json:"payload,omitempty"`
}

type Handoff struct {
	FromNode string `json:"from_node"`
	ToNode   string `json:"to_node"`
	Artifact string `json:"artifact"`
}

type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []ValidationIssue  `json:"errors,omitempty"`
	Warnings []ValidationIssue  `json:"warnings,omitempty"`
}

type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Node    string `json:"node,omitempty"`
}

type RequiredInput struct {
	Node       string `json:"node"`
	Capability string `json:"capability,omitempty"`
	Question   string `json:"question,omitempty"`
}

// GetFrameworks returns all registered frameworks
func (c *Client) GetFrameworks() ([]FrameworkInfo, error) {
	result, err := c.get("/api/v1/frameworks")
	if err != nil {
		return nil, err
	}
	
	if data, ok := result["data"].([]interface{}); ok {
		frameworks := make([]FrameworkInfo, 0, len(data))
		for _, f := range data {
			if m, ok := f.(map[string]interface{}); ok {
				fi := FrameworkInfo{
					Name:          getString(m, "name"),
					Provider:      getString(m, "provider"),
					Model:         getString(m, "model"),
					ExecutionMode: getString(m, "execution_mode"),
					Description:   getString(m, "description"),
				}
				if caps, ok := m["capabilities"].([]interface{}); ok {
					fi.Capabilities = toStrings(caps)
				}
				if prods, ok := m["produces"].([]interface{}); ok {
					fi.Produces = toStrings(prods)
				}
				if reqs, ok := m["requires"].([]interface{}); ok {
					fi.Requires = toStrings(reqs)
				}
				frameworks = append(frameworks, fi)
			}
		}
		return frameworks, nil
	}
	return nil, fmt.Errorf("unexpected response format")
}

// GetFramework returns details for a specific framework
func (c *Client) GetFramework(name string) (*FrameworkInfo, error) {
	result, err := c.get("/api/v1/frameworks/" + name)
	if err != nil {
		return nil, err
	}
	
	if data, ok := result["data"].(map[string]interface{}); ok {
		fi := &FrameworkInfo{
			Name:          getString(data, "name"),
			Provider:      getString(data, "provider"),
			Model:         getString(data, "model"),
			ExecutionMode: getString(data, "execution_mode"),
			Description:   getString(data, "description"),
			AskVia:        getString(data, "ask_via"),
		}
		if caps, ok := data["capabilities"].([]interface{}); ok {
			fi.Capabilities = toStrings(caps)
		}
		if prods, ok := data["produces"].([]interface{}); ok {
			fi.Produces = toStrings(prods)
		}
		if reqs, ok := data["requires"].([]interface{}); ok {
			fi.Requires = toStrings(reqs)
		}
		if modes, ok := data["modes"].([]interface{}); ok {
			fi.Modes = toStrings(modes)
		}
		fi.Testable = getBool(data, "testable")
		fi.Chainable = getBool(data, "chainable")
		fi.EnvKey = getString(data, "env_key")
		return fi, nil
	}
	return nil, fmt.Errorf("unexpected response format")
}

// ListFlows returns all compiled flows
func (c *Client) ListFlows() ([]FlowManifest, error) {
	result, err := c.get("/api/v1/flows")
	if err != nil {
		return nil, err
	}
	
	flows := []FlowManifest{}
	if data, ok := result["data"].([]interface{}); ok {
		for _, f := range data {
			if m, ok := f.(map[string]interface{}); ok {
				flows = append(flows, FlowManifest{
					ID:         getString(m, "id"),
					BusinessID: getString(m, "business_id"),
				})
			}
		}
	}
	return flows, nil
}

// GetFlow returns a specific flow by ID
func (c *Client) GetFlow(id string) (*FlowManifest, error) {
	result, err := c.get("/api/v1/flows/" + id)
	if err != nil {
		return nil, err
	}
	
	if data, ok := result["data"].(map[string]interface{}); ok {
		return parseFlowManifest(data), nil
	}
	return nil, fmt.Errorf("flow not found")
}

// RunFlow executes a flow and returns detailed results
func (c *Client) RunFlow(req flowRunRequest) (*FlowRunResult, error) {
	result, err := c.post("/api/v1/flows/run", req)
	if err != nil {
		return nil, err
	}
	
	if data, ok := result["data"].(map[string]interface{}); ok {
		return parseFlowRunResult(data), nil
	}
	return nil, fmt.Errorf("unexpected response format")
}

// SimulateFlow performs a dry-run of a flow
func (c *Client) SimulateFlow(id string, fixtures []string) (*FlowRunResult, error) {
	req := flowRunRequest{
		CompiledID: id,
		DryRun:     true,
	}
	if len(fixtures) > 0 {
		req.FixtureArtifacts = fixtures
	}
	return c.RunFlow(req)
}

// GetFlowRun returns a specific run result
func (c *Client) GetFlowRun(runID string) (*FlowRunResult, error) {
	result, err := c.get("/api/v1/flows/runs/" + runID)
	if err != nil {
		return nil, err
	}
	
	if data, ok := result["data"].(map[string]interface{}); ok {
		return parseFlowRunResult(data), nil
	}
	return nil, fmt.Errorf("run not found")
}

// GetRules returns the flow composition rules
func (c *Client) GetRules() (map[string]interface{}, error) {
	result, err := c.get("/api/v1/rules")
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func toStrings(arr []interface{}) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func parseFlowManifest(m map[string]interface{}) *FlowManifest {
	flow := &FlowManifest{
		ID:         getString(m, "id"),
		BusinessID: getString(m, "business_id"),
	}
	
	if intent, ok := m["intent"].(map[string]interface{}); ok {
		flow.Intent = FlowIntent{
			Goal:            getString(intent, "goal"),
			OperatorRole:    getString(intent, "operator_role"),
			SuccessCriteria: getString(intent, "success_criteria"),
			Description:     getString(intent, "description"),
		}
		if constraints, ok := intent["constraints"].([]interface{}); ok {
			flow.Intent.Constraints = toStrings(constraints)
		}
	}
	
	if nodes, ok := m["nodes"].([]interface{}); ok {
		for _, n := range nodes {
			if node, ok := n.(map[string]interface{}); ok {
				fn := FlowNode{
					ID:         getString(node, "id"),
					Framework:  getString(node, "framework"),
					Capability: getString(node, "capability"),
					Command:    getString(node, "command"),
					Role:       getString(node, "role"),
				}
				if inputs, ok := node["inputs"].([]interface{}); ok {
					fn.Inputs = toStrings(inputs)
				}
				if outputs, ok := node["outputs"].([]interface{}); ok {
					fn.Outputs = toStrings(outputs)
				}
				if requires, ok := node["requires"].([]interface{}); ok {
					fn.Requires = toStrings(requires)
				}
				if produces, ok := node["produces"].([]interface{}); ok {
					fn.Produces = toStrings(produces)
				}
				if policies, ok := node["policies"].([]interface{}); ok {
					fn.Policies = toStrings(policies)
				}
				flow.Nodes = append(flow.Nodes, fn)
			}
		}
	}
	
	if edges, ok := m["edges"].([]interface{}); ok {
		for _, e := range edges {
			if edge, ok := e.(map[string]interface{}); ok {
				flow.Edges = append(flow.Edges, FlowEdge{
					From: getString(edge, "from"),
					To:   getString(edge, "to"),
				})
			}
		}
	}
	
	return flow
}

func parseFlowRunResult(m map[string]interface{}) *FlowRunResult {
	result := &FlowRunResult{
		RunID:             getString(m, "run_id"),
		FlowID:            getString(m, "flow_id"),
		Status:            getString(m, "status"),
		CompiledID:        getString(m, "compiled_id"),
		BusinessID:        getString(m, "business_id"),
		CreatedAt:        getString(m, "created_at"),
		FinishedAt:        getString(m, "finished_at"),
		Valid:             getBool(m, "valid"),
		DryRun:            getBool(m, "dry_run"),
		TestMode:          getBool(m, "test_mode"),
	}
	
	if v, ok := m["cycles_done"].(float64); ok {
		result.CyclesDone = int(v)
	}
	
	if order, ok := m["execution_order"].([]interface{}); ok {
		result.ExecutionOrder = toStrings(order)
	}
	
	if timeline, ok := m["timeline"].([]interface{}); ok {
		for _, t := range timeline {
			if step, ok := t.(map[string]interface{}); ok {
				fs := FlowRunStep{
					Node:           getString(step, "node"),
					Framework:      getString(step, "framework"),
					Capability:     getString(step, "capability"),
					Command:        getString(step, "command"),
					Role:           getString(step, "role"),
					ResolutionMode: getString(step, "resolution_mode"),
					Status:         getString(step, "status"),
					Error:          getString(step, "error"),
					HumanSummary:   getString(step, "human_summary"),
					StdoutPreview:  getString(step, "stdout_preview"),
					StderrPreview:  getString(step, "stderr_preview"),
				}
				if inputs, ok := step["inputs"].([]interface{}); ok {
					fs.Inputs = toStrings(inputs)
				}
				if requires, ok := step["requires"].([]interface{}); ok {
					fs.Requires = toStrings(requires)
				}
				if outputs, ok := step["outputs"].([]interface{}); ok {
					fs.Outputs = toStrings(outputs)
				}
				if produces, ok := step["produces"].([]interface{}); ok {
					fs.Produces = toStrings(produces)
				}
				if v, ok := step["cycle_index"].(float64); ok {
					fs.CycleIndex = int(v)
				}
				if v, ok := step["duration_ms"].(float64); ok {
					fs.DurationMs = int64(v)
				}
				if v, ok := step["exit_code"].(float64); ok {
					fs.ExitCode = int(v)
				}
				result.Timeline = append(result.Timeline, fs)
			}
		}
	}
	
	if artifacts, ok := m["artifacts"].(map[string]interface{}); ok {
		result.Artifacts = make(map[string]Artifact)
		for k, v := range artifacts {
			if a, ok := v.(map[string]interface{}); ok {
				result.Artifacts[k] = Artifact{
					Type:      getString(a, "type"),
					Source:    getString(a, "source"),
					CreatedAt: getString(a, "created_at"),
					Payload:   a["payload"],
				}
			}
		}
	}
	
	return result
}

// flowRunRequest matches the API request format
type flowRunRequest struct {
	CompiledID       string                 `json:"compiled_id,omitempty"`
	Flow             FlowManifest           `json:"flow,omitempty"`
	Input            string                 `json:"input,omitempty"`
	DryRun           bool                  `json:"dry_run"`
	Approved         bool                  `json:"approved,omitempty"`
	TestMode         bool                  `json:"test_mode,omitempty"`
	TestRecipient    string                `json:"test_recipient,omitempty"`
	FixtureArtifacts []string              `json:"fixture_artifacts,omitempty"`
	InitialArtifacts map[string]interface{} `json:"initial_artifacts,omitempty"`
	BranchMode       bool                  `json:"branch_mode,omitempty"`
	MaxBranches      int                   `json:"max_branches,omitempty"`
	MaxCycles        int                   `json:"max_cycles,omitempty"`
	SimulateHuman    bool                  `json:"simulate_human,omitempty"`
}

// GetProviders returns capability -> framework mapping
func (c *Client) GetProviders() (map[string][]string, error) {
	frameworks, err := c.GetFrameworks()
	if err != nil {
		return nil, err
	}
	
	providers := make(map[string][]string)
	for _, fw := range frameworks {
		for _, cap := range fw.Capabilities {
			providers[cap] = append(providers[cap], fw.Name)
		}
		for _, prod := range fw.Produces {
			providers[prod] = append(providers[prod], fw.Name)
		}
	}
	
	return providers, nil
}

// HealthCheck verifies the backend is reachable
func (c *Client) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("backend unreachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend returned status %d", resp.StatusCode)
	}
	return nil
}
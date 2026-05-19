package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ANSI colors
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

// Client for API communication
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
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	return out, nil
}

func (c *Client) post(path string, body interface{}) (map[string]interface{}, error) {
	url := c.BaseURL + path
	var b []byte
	if m, ok := body.(map[string]interface{}); ok {
		b, _ = json.Marshal(m)
	} else {
		b, _ = json.Marshal(body)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
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
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return out, nil
}

// FlowManifest represents a flow definition
type FlowManifest struct {
	ID         string       `json:"id"`
	BusinessID string       `json:"business_id,omitempty"`
	Intent     FlowIntent  `json:"intent,omitempty"`
	Nodes      []FlowNode   `json:"nodes"`
	Edges      []FlowEdge   `json:"edges,omitempty"`
	Policies   []string     `json:"policies,omitempty"`
}

type FlowIntent struct {
	Goal            string   `json:"goal,omitempty"`
	OperatorRole   string   `json:"operator_role,omitempty"`
	SuccessCriteria string   `json:"success_criteria,omitempty"`
	Description    string   `json:"description,omitempty"`
}

type FlowNode struct {
	ID         string            `json:"id"`
	Framework  string            `json:"framework"`
	Capability string            `json:"capability,omitempty"`
	Command    string            `json:"command,omitempty"`
	Role       string            `json:"role,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	Inputs     []string          `json:"inputs,omitempty"`
	Outputs    []string          `json:"outputs,omitempty"`
	Requires   []string          `json:"requires,omitempty"`
	Produces   []string          `json:"produces,omitempty"`
	Policies   []string          `json:"policies,omitempty"`
}

type FlowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FlowRunResult represents the result of running a flow
type FlowRunResult struct {
	RunID          string        `json:"run_id"`
	Status         string        `json:"status"`
	CompiledID     string        `json:"compiled_id,omitempty"`
	CyclesDone     int           `json:"cycles_done,omitempty"`
	ExecutionOrder []string      `json:"execution_order"`
	Timeline       []FlowRunStep `json:"timeline"`
	Artifacts     map[string]flowRunArtifact `json:"artifacts,omitempty"`
	Handoffs       []FlowHandoff `json:"handoffs,omitempty"`
	Validation     FlowValidation `json:"validation"`
	Warnings       []FlowIssue   `json:"warnings,omitempty"`
	NeedsInput     []FlowInput   `json:"needs_input,omitempty"`
	DynamicNodes   []FlowNode    `json:"dynamic_nodes,omitempty"`
	CreatedAt      string        `json:"created_at"`
	FinishedAt     string        `json:"finished_at,omitempty"`
}

type FlowRunStep struct {
	Node           string     `json:"node"`
	Framework      string     `json:"framework"`
	Capability     string     `json:"capability,omitempty"`
	Command        string     `json:"command,omitempty"`
	Role           string     `json:"role,omitempty"`
	Status         string     `json:"status"`
	Inputs         []string   `json:"inputs,omitempty"`
	Requires       []string   `json:"requires,omitempty"`
	Outputs        []string   `json:"outputs,omitempty"`
	Produces       []string   `json:"produces,omitempty"`
	Policies       []string   `json:"policies,omitempty"`
	StartedAt      string     `json:"started_at,omitempty"`
	FinishedAt     string     `json:"finished_at,omitempty"`
	DurationMs     int64      `json:"duration_ms,omitempty"`
	Error          string     `json:"error,omitempty"`
	StdoutPreview  string     `json:"stdout_preview,omitempty"`
	StderrPreview  string     `json:"stderr_preview,omitempty"`
	ActionOptions  []map[string]string `json:"action_options,omitempty"`
	SegmentID      string     `json:"segment_id,omitempty"`
	SegmentOwner   string     `json:"segment_owner,omitempty"`
}

type flowRunArtifact struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	Node      string      `json:"node,omitempty"`
	Path      string      `json:"path,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	CreatedAt string      `json:"created_at"`
}

type FlowHandoff struct {
	FromNode      string `json:"from_node"`
	ToNode        string `json:"to_node"`
	FromFramework string `json:"from_framework"`
	ToFramework   string `json:"to_framework"`
	Artifacts     []string `json:"artifacts,omitempty"`
	Summary       string   `json:"summary"`
}

type FlowValidation struct {
	Valid      bool   `json:"valid"`
	Violations []FlowIssue `json:"violations,omitempty"`
}

type FlowIssue struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	At      string `json:"at,omitempty"`
}

type FlowInput struct {
	Artifact   string `json:"artifact"`
	Kind       string `json:"kind"`
	Node       string `json:"node,omitempty"`
	Framework  string `json:"framework,omitempty"`
	Capability string `json:"capability,omitempty"`
}

// FrameworkInfo represents a registered framework
type FrameworkInfo struct {
	Name        string     `json:"name"`
	Version     string     `json:"version,omitempty"`
	Command     string     `json:"command,omitempty"`
	Cwd         string     `json:"cwd,omitempty"`
	Mode        string     `json:"mode,omitempty"`
	Freshness   string     `json:"freshness,omitempty"`
	Capabilities CapabilityInfo `json:"capabilities,omitempty"`
}

type CapabilityInfo struct {
	Semantic  map[string]interface{} `json:"semantic,omitempty"`
	Contracts []ContractSpec         `json:"contracts,omitempty"`
}

type ContractSpec struct {
	ID         string   `json:"id"`
	Command    string   `json:"command,omitempty"`
	Inputs     []string `json:"inputs,omitempty"`
	Outputs    []string `json:"outputs,omitempty"`
	Requires   []string `json:"requires,omitempty"`
	Produces   []string `json:"produces,omitempty"`
	Policies   []string `json:"policies,omitempty"`
	Execution  string   `json:"execution,omitempty"`
}

// CapabilityProvider represents a capability provider
type CapabilityProvider struct {
	Capability  string   `json:"capability"`
	Framework   string   `json:"framework"`
	Command     string   `json:"command,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
	Requires    []string `json:"requires,omitempty"`
	Produces    []string `json:"produces,omitempty"`
	Execution   string   `json:"execution,omitempty"`
	Source      string   `json:"source"`
}

// FlowValidationResult from validate endpoint
type FlowValidationResult struct {
	Valid      bool   `json:"valid"`
	Violations []FlowIssue `json:"violations,omitempty"`
	Warnings   []FlowIssue `json:"warnings,omitempty"`
	Issues     []FlowIssue `json:"issues,omitempty"`
}

// SimulateResult from simulate endpoint
type SimulateResult struct {
	Valid           bool             `json:"valid"`
	ExecutionPlan   []string         `json:"execution_plan,omitempty"`
	Capabilities    []string         `json:"capabilities,omitempty"`
	Providers       []CapabilityProvider `json:"providers,omitempty"`
	MissingCapabilities []string     `json:"missing_capabilities,omitempty"`
	Warnings        []FlowIssue      `json:"warnings,omitempty"`
}

// Helper to print colored framework name
func printFrameworkName(name string) {
	fmt.Printf("%s%s%s", fwColor(name), name, reset)
}

// Helper to print status with color
func printStatus(status string) {
	color := white
	switch status {
	case "done", "completed", "valid":
		color = green
	case "pending", "waiting", "queued":
		color = yellow
	case "error", "failed", "invalid":
		color = red
	case "running", "in_progress":
		color = cyan
	}
	fmt.Printf("%s[%s]%s ", color, status, reset)
}

// Format duration in ms to human readable
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(ms)/60000)
}

// Format timestamp to time only
func formatTime(ts string) string {
	if ts == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	return t.Format("15:04:05.000")
}
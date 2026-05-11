package main

type flowRunRequest struct {
	Flow             flowManifest           `json:"flow"`
	Input            string                 `json:"input,omitempty"`
	DryRun           bool                   `json:"dry_run"`
	Approved         bool                   `json:"approved,omitempty"`
	TestMode         bool                   `json:"test_mode,omitempty"`
	TestRecipient    string                 `json:"test_recipient,omitempty"`
	FixtureArtifacts []string               `json:"fixture_artifacts,omitempty"`
	InitialArtifacts map[string]interface{} `json:"initial_artifacts,omitempty"`
	BranchMode       bool                   `json:"branch_mode,omitempty"`
	MaxBranches      int                    `json:"max_branches,omitempty"`
	MaxCycles        int                    `json:"max_cycles,omitempty"`
	SimulateHuman    bool                   `json:"simulate_human,omitempty"`
}

type flowRunResult struct {
	RunID             string                     `json:"run_id"`
	Status            string                     `json:"status"`
	CyclesDone        int                        `json:"cycles_done,omitempty"`
	Valid             bool                       `json:"valid"`
	DryRun            bool                       `json:"dry_run"`
	Approved          bool                       `json:"approved,omitempty"`
	TestMode          bool                       `json:"test_mode,omitempty"`
	TestRecipient     string                     `json:"test_recipient,omitempty"`
	BusinessID        string                     `json:"business_id,omitempty"`
	BusinessArtifacts []string                   `json:"business_artifacts,omitempty"`
	ExecutionOrder    []string                   `json:"execution_order"`
	Timeline          []flowRunStep              `json:"timeline"`
	Artifacts         map[string]flowRunArtifact `json:"artifacts"`
	Validation        flowValidationResult       `json:"validation"`
	Warnings          []flowValidationIssue      `json:"warnings,omitempty"`
	NeedsInput        []flowRequiredInput        `json:"needs_input,omitempty"`
	Branches          []flowBranchRun            `json:"branches,omitempty"`
	DynamicNodes      []flowNode                 `json:"dynamic_nodes,omitempty"`
	CreatedAt         string                     `json:"created_at"`
	FinishedAt        string                     `json:"finished_at,omitempty"`
}

type flowBranchRun struct {
	BranchID       string              `json:"branch_id"`
	Action         map[string]string   `json:"action"`
	Status         string              `json:"status"`
	Timeline       []flowRunStep       `json:"timeline"`
	Artifacts      []string            `json:"artifacts,omitempty"`
	NeedsInput     []flowRequiredInput `json:"needs_input,omitempty"`
	CycleCompleted bool                `json:"cycle_completed,omitempty"`
}

type flowRunStep struct {
	Node             string              `json:"node"`
	Framework        string              `json:"framework"`
	Capability       string              `json:"capability,omitempty"`
	Command          string              `json:"command,omitempty"`
	Role             string              `json:"role,omitempty"`
	Visibility       string              `json:"visibility,omitempty"`
	TriggeredBy      *flowStepTrigger    `json:"triggered_by,omitempty"`
	ResolutionMode   string              `json:"resolution_mode,omitempty"`
	CycleIndex       int                 `json:"cycle_index,omitempty"`
	Status           string              `json:"status"`
	Inputs           []string            `json:"inputs,omitempty"`
	Requires         []string            `json:"requires,omitempty"`
	Outputs          []string            `json:"outputs,omitempty"`
	Produces         []string            `json:"produces,omitempty"`
	Policies         []string            `json:"policies,omitempty"`
	MissingArtifacts []string            `json:"missing_artifacts,omitempty"`
	ArtifactTypes    []string            `json:"artifact_types,omitempty"`
	StartedAt        string              `json:"started_at,omitempty"`
	FinishedAt       string              `json:"finished_at,omitempty"`
	ExitCode         int                 `json:"exit_code,omitempty"`
	DurationMs       int64               `json:"duration_ms,omitempty"`
	Error            string              `json:"error,omitempty"`
	HumanSummary     string              `json:"human_summary,omitempty"`
	StdoutPreview    string              `json:"stdout_preview,omitempty"`
	StderrPreview    string              `json:"stderr_preview,omitempty"`
	ActionOptions    []map[string]string `json:"action_options,omitempty"`
}

type flowStepTrigger struct {
	Node       string `json:"node,omitempty"`
	Framework  string `json:"framework,omitempty"`
	Capability string `json:"capability,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

const (
	flowStepVisibilityUserFacing     = "user_facing"
	flowStepVisibilityInfrastructure = "infrastructure"
)

type flowRunArtifact struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	Node      string      `json:"node,omitempty"`
	Path      string      `json:"path,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	CreatedAt string      `json:"created_at"`
}

type flowRequiredInput struct {
	Artifact    string            `json:"artifact"`
	Kind        string            `json:"kind"`
	Framework   string            `json:"framework,omitempty"`
	Capability  string            `json:"capability,omitempty"`
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	Fields      []flowInputField  `json:"fields,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
	QuestionID  string            `json:"question_id,omitempty"`
}

type flowInputField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
}

const inlineArtifactArgMaxBytes = 100 * 1024

type flowStepCallback func(event string, step flowRunStep, totalSteps int)

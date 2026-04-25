package alfa

type EchoTree struct {
	ProjectID              string           `json:"project_id"`
	ClientName             string           `json:"client_name"`
	DateStarted            string           `json:"date_started"`
	SelectedOpportunityIDs []string         `json:"selected_opportunity_ids,omitempty"`
	Signals                []SignalEntry    `json:"signals,omitempty"`
	Nodes                  map[string]*Node `json:"nodes"`
}

type SignalEntry struct {
	Type      string `json:"type"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at,omitempty"`
}

type Node struct {
	ID               string   `json:"id"`
	Layer            int      `json:"layer"`
	Type             string   `json:"type"`
	Title            string   `json:"title"`
	Evidence         []string `json:"evidence"`
	Status           string   `json:"status"`
	Confidence       int      `json:"confidence"`
	ParentID         string   `json:"parent_id,omitempty"`
	ChildrenIDs      []string `json:"children_ids,omitempty"`
	QuestionsToAsk   []string `json:"questions_to_ask,omitempty"`
	Perceptions      []string `json:"perceptions,omitempty"`
	ValidationAnswer string   `json:"validation_answer,omitempty"`
}

type AlfaSpec struct {
	Version               string            `json:"version"`
	Generated             string            `json:"generated"`
	SourceTree            string            `json:"source_tree"`
	ProjectID             string            `json:"project_id"`
	ClientName            string            `json:"client_name,omitempty"`
	AutomationIntent      string            `json:"automation_intent"`
	ExportReady           bool              `json:"export_ready"`
	SelectedOpportunities []OpportunitySpec `json:"selected_opportunities"`
	ConfirmedPains        []NodeRef         `json:"confirmed_pains"`
	SupportingTasks       []NodeRef         `json:"supporting_tasks,omitempty"`
	SupportingTheories    []NodeRef         `json:"supporting_theories,omitempty"`
	SupportingAxioms      []NodeRef         `json:"supporting_axioms,omitempty"`
	Perceptions           []string          `json:"perceptions,omitempty"`
	ConversationSignals   []SignalEntry     `json:"conversation_signals,omitempty"`
	IdealSteps            []IdealStep       `json:"ideal_steps"`
	BusinessRules         []BusinessRule    `json:"business_rules"`
	CriticalVariables     []string          `json:"critical_variables"`
	SuccessCriteria       []string          `json:"success_criteria"`
	EdgeCases             []string          `json:"edge_cases,omitempty"`
	OpenQuestions         []OpenQuestion    `json:"open_questions,omitempty"`
}

type OpportunitySpec struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	ParentPainID     string   `json:"parent_pain_id"`
	Evidence         []string `json:"evidence,omitempty"`
	ValidationAnswer string   `json:"validation_answer,omitempty"`
}

type NodeRef struct {
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	Title            string   `json:"title"`
	Evidence         []string `json:"evidence,omitempty"`
	ValidationAnswer string   `json:"validation_answer,omitempty"`
	Status           string   `json:"status,omitempty"`
}

type IdealStep struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Inputs      []string `json:"inputs,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
	SourceNodes []string `json:"source_nodes,omitempty"`
}

type BusinessRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	When        string   `json:"when,omitempty"`
	Then        string   `json:"then"`
	Importance  int      `json:"importance"`
	SourceNodes []string `json:"source_nodes,omitempty"`
}

type OpenQuestion struct {
	ID              string   `json:"id"`
	Reason          string   `json:"reason"`
	QuestionForEcho string   `json:"question_for_echo"`
	NeededFor       string   `json:"needed_for"`
	SourceNodes     []string `json:"source_nodes,omitempty"`
}

type BravoIdealFlow struct {
	TraceID       string      `json:"trace_id"`
	Generated     string      `json:"generated"`
	Description   string      `json:"description"`
	Verbalization string      `json:"verbalization"`
	Intent        string      `json:"intent,omitempty"`
	Rules         []BravoRule `json:"rules,omitempty"`
	CriticalVars  []string    `json:"critical_vars,omitempty"`
	CriticalPath  []string    `json:"critical_path,omitempty"`
}

type BravoRule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	When        string `json:"when,omitempty"`
	Then        string `json:"then"`
	Importance  int    `json:"importance,omitempty"`
}

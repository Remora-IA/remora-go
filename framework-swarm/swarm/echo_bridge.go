package swarm

// echo_bridge.go — bridging Echo discovery trees and Alfa specs into swarm inputs.
//
// These functions consume JSON produced by framework-echo and framework-alfa,
// converting their output into the Zone and IdealFlow types used by framework-swarm.
//
// No cross-module imports are required: both Echo and Alfa emit plain JSON,
// so the bridge reads structs that mirror the upstream shapes.
//
// Typical usage (full tripod pipeline):
//
//	zones, err   := ZonesFromEchoFile("frameworkecho.json")
//	flow,  err   := IdealFlowFromAlfaFile("alfa_spec.json")
//	s,     err   := swarm.New(swarm.Config{Zones: zones, ...})
//	result, err  := s.Run(ctx)
//	score,  err  := ScoreLatestTrace(flow, "traces/", 0.80)

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ─── Echo bridge ──────────────────────────────────────────────────────────────

// echoTree mirrors the JSON schema of framework-echo's frameworkecho.json.
// Only the fields needed for zone extraction are mapped.
type echoTree struct {
	ProjectID  string               `json:"project_id"`
	ClientName string               `json:"client_name"`
	Nodes      map[string]*echoNode `json:"nodes"`
}

type echoNode struct {
	ID         string   `json:"id"`
	Layer      int      `json:"layer"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Evidence   []string `json:"evidence"`
	Status     string   `json:"status"`
	Confidence int      `json:"confidence"`
	ParentID   string   `json:"parent_id,omitempty"`
}

// ZonesFromEchoTree converts a raw Echo tree JSON document into a []Zone.
//
// Strategy:
//   - OPPORTUNITY nodes (layer 4) become zones — they represent concrete automation targets.
//   - If no OPPORTUNITY nodes exist, PAIN nodes (layer 3) are used instead.
//   - PainWeight is derived from node Confidence (0–100) scaled to 0.0–1.0.
//   - Only VALIDATED or PENDING nodes are included; REJECTED nodes are skipped.
//   - Zones are returned sorted by PainWeight descending (highest urgency first).
func ZonesFromEchoTree(data []byte) ([]Zone, error) {
	var tree echoTree
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("echo_bridge: unmarshal echo tree: %w", err)
	}

	const (
		typeOpportunity = "OPPORTUNITY"
		typePain        = "PAIN"
		statusRejected  = "REJECTED"
	)

	// Collect nodes by type, excluding rejected
	var opps, pains []*echoNode
	for _, n := range tree.Nodes {
		if n.Status == statusRejected {
			continue
		}
		switch n.Type {
		case typeOpportunity:
			opps = append(opps, n)
		case typePain:
			pains = append(pains, n)
		}
	}

	// Prefer opportunities; fall back to pains
	candidates := opps
	if len(candidates) == 0 {
		candidates = pains
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("echo_bridge: no OPPORTUNITY or PAIN nodes found in echo tree")
	}

	// Sort by confidence descending (deterministic order)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].ID < candidates[j].ID
	})

	zones := make([]Zone, 0, len(candidates))
	for _, n := range candidates {
		zoneID := normaliseZoneID(n.Title)
		weight := float64(n.Confidence) / 100.0
		if weight > 1.0 {
			weight = 1.0
		}
		if weight <= 0 {
			weight = 0.1 // minimum viable pressure
		}

		desc := ""
		if len(n.Evidence) > 0 {
			desc = n.Evidence[0]
		}

		zones = append(zones, Zone{
			ID:          zoneID,
			Name:        n.Title,
			Description: desc,
			PainWeight:  weight,
			Tags:        []string{strings.ToLower(n.Type), n.ID},
			Meta: map[string]any{
				"echo_node_id": n.ID,
				"echo_type":    n.Type,
				"confidence":   n.Confidence,
			},
		})
	}

	return zones, nil
}

// ZonesFromEchoFile reads a frameworkecho.json file and returns zones.
// It delegates to ZonesFromEchoTree.
func ZonesFromEchoFile(path string) ([]Zone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("echo_bridge: read echo file %q: %w", path, err)
	}
	return ZonesFromEchoTree(data)
}

// normaliseZoneID converts a human title into a snake_case zone ID.
// "Automatizar reconciliación" → "automatizar_reconciliacion"
func normaliseZoneID(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_':
			return '_'
		default:
			// Drop accents and special chars that don't map cleanly
			switch r {
			case 'á', 'à', 'â', 'ä', 'ã':
				return 'a'
			case 'é', 'è', 'ê', 'ë':
				return 'e'
			case 'í', 'ì', 'î', 'ï':
				return 'i'
			case 'ó', 'ò', 'ô', 'ö', 'õ':
				return 'o'
			case 'ú', 'ù', 'û', 'ü':
				return 'u'
			case 'ñ':
				return 'n'
			case 'ç':
				return 'c'
			}
			return '_'
		}
	}, s)
	// Collapse multiple underscores
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}

// ─── Alfa bridge ──────────────────────────────────────────────────────────────

// alfaSpec mirrors the JSON schema of framework-alfa's compiled spec.
// Only the fields needed for IdealFlow construction are mapped.
type alfaSpec struct {
	ProjectID         string         `json:"project_id"`
	AutomationIntent  string         `json:"automation_intent"`
	IdealSteps        []alfaStep     `json:"ideal_steps"`
	BusinessRules     []alfaRule     `json:"business_rules"`
	CriticalVariables []string       `json:"critical_variables"`
	SuccessCriteria   []string       `json:"success_criteria,omitempty"`
}

type alfaStep struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Outputs     []string `json:"outputs,omitempty"`
}

type alfaRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	When        string `json:"when,omitempty"`
	Then        string `json:"then"`
	Importance  int    `json:"importance"`
}

// IdealFlowFromAlfaSpec converts a raw Alfa spec JSON document into an IdealFlow
// ready for use with ScoreLatestTrace.
//
// Mapping:
//   - AutomationIntent → IdealFlow.Intent
//   - IdealSteps (in order) → CriticalPath (normalised to snake_case)
//   - BusinessRules → VerifyRules
//   - CriticalVariables → CriticalVars
func IdealFlowFromAlfaSpec(data []byte) (*IdealFlow, error) {
	var spec alfaSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("echo_bridge: unmarshal alfa spec: %w", err)
	}

	// Build CriticalPath from ordered ideal steps
	criticalPath := make([]string, 0, len(spec.IdealSteps))
	for _, step := range spec.IdealSteps {
		criticalPath = append(criticalPath, normaliseZoneID(step.Name))
	}

	// Build VerifyRules from business rules
	rules := make([]VerifyRule, 0, len(spec.BusinessRules))
	for _, br := range spec.BusinessRules {
		imp := br.Importance
		if imp == 0 {
			imp = 2 // default: important
		}
		rules = append(rules, VerifyRule{
			Name:        br.Name,
			Description: br.Description,
			When:        br.When,
			Then:        br.Then,
			Importance:  imp,
		})
	}

	// Description from first success criterion or intent
	desc := spec.AutomationIntent
	if len(spec.SuccessCriteria) > 0 {
		desc = spec.SuccessCriteria[0]
	}

	return &IdealFlow{
		Description:  desc,
		Intent:       spec.AutomationIntent,
		CriticalPath: criticalPath,
		CriticalVars: spec.CriticalVariables,
		Rules:        rules,
	}, nil
}

// IdealFlowFromAlfaFile reads an alfa_spec.json file and returns an IdealFlow.
// It delegates to IdealFlowFromAlfaSpec.
func IdealFlowFromAlfaFile(path string) (*IdealFlow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("echo_bridge: read alfa file %q: %w", path, err)
	}
	return IdealFlowFromAlfaSpec(data)
}

// ─── BravoIdealFlow bridge ────────────────────────────────────────────────────

// bravoIdealFlow mirrors the JSON schema of framework-alfa's BravoIdealFlow export.
// Use this when you have an ideal_flow.json (the output of alfa ExportBravoFlow).
type bravoIdealFlow struct {
	Description  string      `json:"description"`
	Intent       string      `json:"intent,omitempty"`
	CriticalPath []string    `json:"critical_path"`
	CriticalVars []string    `json:"critical_vars"`
	Rules        []bravoRule `json:"rules,omitempty"`
}

type bravoRule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	When        string `json:"when,omitempty"`
	Then        string `json:"then"`
	Importance  int    `json:"importance,omitempty"`
}

// IdealFlowFromBravoFile reads an ideal_flow.json produced by alfa.ExportBravoFlow
// and returns an IdealFlow. This is the highest-fidelity path: Alfa compiles the
// Echo tree into an ideal flow, exports it, and the swarm loads it directly.
func IdealFlowFromBravoFile(path string) (*IdealFlow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("echo_bridge: read bravo flow file %q: %w", path, err)
	}
	return IdealFlowFromBravoJSON(data)
}

// IdealFlowFromBravoJSON converts raw BravoIdealFlow JSON into an IdealFlow.
func IdealFlowFromBravoJSON(data []byte) (*IdealFlow, error) {
	var bif bravoIdealFlow
	if err := json.Unmarshal(data, &bif); err != nil {
		return nil, fmt.Errorf("echo_bridge: unmarshal bravo ideal_flow: %w", err)
	}

	rules := make([]VerifyRule, 0, len(bif.Rules))
	for _, r := range bif.Rules {
		rules = append(rules, VerifyRule{
			Name:        r.Name,
			Description: r.Description,
			When:        r.When,
			Then:        r.Then,
			Importance:  r.Importance,
		})
	}

	return &IdealFlow{
		Description:  bif.Description,
		Intent:       bif.Intent,
		CriticalPath: bif.CriticalPath,
		CriticalVars: bif.CriticalVars,
		Rules:        rules,
	}, nil
}

// ─── Combining bridge ─────────────────────────────────────────────────────────

// IdealFlowForZones creates a final IdealFlow ready for Bravo scoring by
// combining a base flow (rules + vars from Alfa) with the actual zone IDs
// extracted from the Echo tree.
//
// This resolves the language/naming mismatch between:
//   - Echo opportunity titles → zone IDs (e.g. Spanish: "validar_campos_...")
//   - Alfa ideal step names → CriticalPath items (e.g. English: "validate_invoice_fields")
//
// Since Paladin spans are named "zone.<zoneID>", the Bravo scorer must search
// for zone IDs — not Alfa step names — in the trace.
//
// The zones slice sets the CriticalPath order (highest PainWeight first, as returned
// by ZonesFromEchoTree). Business rules and critical vars come from the base flow.
//
// Typical usage:
//
//	zones,    _ := ZonesFromEchoFile("frameworkecho.json")
//	baseFlow, _ := IdealFlowFromAlfaFile("alfa_spec.json")
//	flow        := IdealFlowForZones(baseFlow, zones)
//	// flow.CriticalPath now contains zone IDs that match Paladin span names
func IdealFlowForZones(base *IdealFlow, zones []Zone) *IdealFlow {
	path := make([]string, len(zones))
	for i, z := range zones {
		path[i] = z.ID
	}
	return &IdealFlow{
		Description:  base.Description,
		Intent:       base.Intent,
		CriticalPath: path,
		CriticalVars: base.CriticalVars,
		Rules:        base.Rules,
	}
}

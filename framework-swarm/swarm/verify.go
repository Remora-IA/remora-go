package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/remora-go/framework-paladin/paladin"
)

// ScoreTrace compares a Paladin trace (JSON bytes) against an IdealFlow heuristically.
// No LLM needed — it uses deterministic coverage metrics:
//
//   - PathCoverage:  what % of CriticalPath step names appear in span names
//   - VarCoverage:   what % of CriticalVars appear in span Vars
//   - RuleCoverage:  what % of Rules are evidenced by semantic events
//   - Violations:    count of "violation" semantic events (penalty)
//
// Final score = mean(path, var, rule) × (1 − violationPenalty)
// Passed = Score ≥ threshold (default 0.80).
func ScoreTrace(flow *IdealFlow, traceJSON []byte, threshold float64) (*VerifyResult, error) {
	var result paladin.TraceResult
	if err := json.Unmarshal(traceJSON, &result); err != nil {
		return nil, fmt.Errorf("parse paladin trace: %w", err)
	}
	if result.Root == nil {
		return &VerifyResult{Details: []string{"empty trace — nothing to score"}}, nil
	}
	if threshold <= 0 {
		threshold = 0.80
	}

	// Flatten trace into searchable sets
	spanNames := collectSpanNames(result.Root)
	varKeys := collectVarKeys(result.Root)
	semanticEvents := collectSemantic(result.Root)

	var details []string

	// ── 1. Path coverage ────────────────────────────────────────────────────
	pathHits := 0
	for _, step := range flow.CriticalPath {
		if fuzzyContains(spanNames, step) {
			pathHits++
			details = append(details, fmt.Sprintf("✅ path: %s", step))
		} else {
			details = append(details, fmt.Sprintf("❌ path faltante: %s", step))
		}
	}
	pathCov := safeRatio(pathHits, len(flow.CriticalPath))

	// ── 2. Var coverage ──────────────────────────────────────────────────────
	varHits := 0
	for _, v := range flow.CriticalVars {
		if fuzzyContains(varKeys, v) {
			varHits++
			details = append(details, fmt.Sprintf("✅ var: %s", v))
		} else {
			details = append(details, fmt.Sprintf("❌ var faltante: %s", v))
		}
	}
	varCov := safeRatio(varHits, len(flow.CriticalVars))

	// ── 3. Rule coverage ─────────────────────────────────────────────────────
	ruleHits := 0
	for _, rule := range flow.Rules {
		if ruleEvidenced(rule, semanticEvents) {
			ruleHits++
			details = append(details, fmt.Sprintf("✅ regla evidenciada: %s", rule.Name))
		} else {
			details = append(details, fmt.Sprintf("⚠️  regla no evidenciada: %s", rule.Name))
		}
	}
	ruleCov := safeRatio(ruleHits, len(flow.Rules))

	// ── 4. Violations ────────────────────────────────────────────────────────
	violations := 0
	for _, ev := range semanticEvents {
		if ev.Kind == "violation" {
			violations++
			details = append(details, fmt.Sprintf("❌ violation: [%s] %s", ev.Subject, ev.Summary))
		}
	}
	violationPenalty := clamp(float64(violations)*0.15, 0, 1)

	// ── Final score ───────────────────────────────────────────────────────────
	raw := (pathCov + varCov + ruleCov) / 3.0
	score := raw * (1.0 - violationPenalty)

	return &VerifyResult{
		Score:        score,
		PathCoverage: pathCov,
		VarCoverage:  varCov,
		RuleCoverage: ruleCov,
		Violations:   violations,
		Passed:       score >= threshold,
		Threshold:    threshold,
		Details:      details,
	}, nil
}

// ScoreTraceFile loads a trace from tracePath and scores it.
func ScoreTraceFile(flow *IdealFlow, tracePath string, threshold float64) (*VerifyResult, error) {
	data, err := os.ReadFile(tracePath)
	if err != nil {
		return nil, fmt.Errorf("read trace file: %w", err)
	}
	return ScoreTrace(flow, data, threshold)
}

// ScoreLatestTrace finds the most recent trace_*.json in traceDir and scores it.
// traceDir is typically "temp/paladin" relative to the working directory.
func ScoreLatestTrace(flow *IdealFlow, traceDir string, threshold float64) (*VerifyResult, error) {
	entries, err := os.ReadDir(traceDir)
	if err != nil {
		return nil, fmt.Errorf("read trace dir %s: %w", traceDir, err)
	}

	latest := ""
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, "trace_") && strings.HasSuffix(name, ".json") {
			candidate := filepath.Join(traceDir, name)
			if candidate > latest {
				latest = candidate
			}
		}
	}
	if latest == "" {
		return nil, fmt.Errorf("no trace files found in %s", traceDir)
	}
	return ScoreTraceFile(flow, latest, threshold)
}

// ─── Trace traversal helpers ──────────────────────────────────────────────────

func collectSpanNames(span *paladin.Span) []string {
	if span == nil {
		return nil
	}
	names := []string{span.Name}
	for _, child := range span.Children {
		names = append(names, collectSpanNames(child)...)
	}
	return names
}

func collectVarKeys(span *paladin.Span) []string {
	if span == nil {
		return nil
	}
	var keys []string
	for k := range span.Vars {
		keys = append(keys, k)
	}
	for _, child := range span.Children {
		keys = append(keys, collectVarKeys(child)...)
	}
	return keys
}

func collectSemantic(span *paladin.Span) []paladin.SemanticEvent {
	if span == nil {
		return nil
	}
	evts := append([]paladin.SemanticEvent{}, span.Semantic...)
	for _, child := range span.Children {
		evts = append(evts, collectSemantic(child)...)
	}
	return evts
}

// fuzzyContains reports whether needle appears in any element of haystack,
// after normalising both sides (lowercase, _ ↔ space).
func fuzzyContains(haystack []string, needle string) bool {
	needle = normalise(needle)
	for _, h := range haystack {
		h = normalise(h)
		if strings.Contains(h, needle) || strings.Contains(needle, h) {
			return true
		}
	}
	return false
}

// ruleEvidenced checks whether any semantic event in the trace references the rule
// by name or subject. It looks for "rule", "check", or "decision" events.
func ruleEvidenced(rule VerifyRule, evts []paladin.SemanticEvent) bool {
	rn := normalise(rule.Name)
	for _, ev := range evts {
		if ev.Kind != "rule" && ev.Kind != "check" && ev.Kind != "decision" {
			continue
		}
		subj := normalise(ev.Subject)
		summ := normalise(ev.Summary)
		if strings.Contains(subj, rn) || strings.Contains(summ, rn) {
			return true
		}
		// Accept partial word matches for multi-word rule names
		for _, word := range strings.Fields(rn) {
			if len(word) > 4 && (strings.Contains(subj, word) || strings.Contains(summ, word)) {
				return true
			}
		}
	}
	return false
}

func normalise(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	return s
}

func safeRatio(hits, total int) float64 {
	if total == 0 {
		return 1.0 // no requirements = fully satisfied
	}
	return float64(hits) / float64(total)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

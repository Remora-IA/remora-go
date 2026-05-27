package alfa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ToBravoIdealFlow wraps ExportBravo with normalised CriticalPath names.
// Step names are snake_cased so they fuzzy-match span names in Paladin traces
// (e.g. "Validate Invoices" → "validate_invoices" matches span "zone.validate_invoices").
func (s *AlfaSpec) ToBravoIdealFlow() *BravoIdealFlow {
	flow := ExportBravo(s, time.Now())
	// Normalise the critical path for trace matching
	for i, step := range flow.CriticalPath {
		flow.CriticalPath[i] = normaliseStepName(step)
	}
	return flow
}

// ExportBravoFlow converts the spec to BravoIdealFlow and saves it to outputPath.
// Creates parent directories as needed. This closes the Alfa → Bravo gap:
// the output file is loadable by bravo.LoadIdealFlow() and the swarm verifier.
func ExportBravoFlow(spec *AlfaSpec, outputPath string) error {
	flow := spec.ToBravoIdealFlow()

	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ideal_flow: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write ideal_flow: %w", err)
	}

	fmt.Printf("[ALFA] ideal_flow exportado → %s (%d reglas, %d pasos, %d vars críticas)\n",
		outputPath, len(flow.Rules), len(flow.CriticalPath), len(flow.CriticalVars))
	return nil
}

// normaliseStepName converts a human step name to snake_case so it
// fuzzy-matches Paladin span names like "zone.validate_invoices".
func normaliseStepName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

package bravo

import (
	"fmt"
	"strings"
)

// VerifierResult es el formato exacto que la IA debe devolver al analizar.
type VerifierResult struct {
	Sufficient      bool            `json:"sufficient"`
	Gaps            []Gap           `json:"gaps"`
	Observations    string          `json:"observations"`
	Recommendations []string        `json:"recommendations"`
	IdealComparison IdealComparison `json:"ideal_comparison"`
}

type Gap struct {
	SpanPath      string   `json:"span_path"`
	MissingFields []string `json:"missing_fields"`
	Rationale     string   `json:"rationale"`
}

type IdealComparison struct {
	Matched     bool   `json:"matched"`
	Differences []Diff `json:"differences"`
	Notes       string `json:"notes"`
}

type Diff struct {
	Criterion string `json:"criterion"`
	Actual    string `json:"actual"`
	Expected  string `json:"expected"`
}

// PrintVerificationInstructions imprime cómo usar el verificador
func PrintVerificationInstructions() {
	sep := strings.Repeat("=", 80)
	fmt.Println("\n" + sep)
	fmt.Println("          FRAMEWORKBRAVO - MODO VERIFICACIÓN")
	fmt.Println(sep)
	fmt.Println("1. Ejecuta tu programa → se generarán trace_*.json + ideal_flow.json")
	fmt.Println("2. Dale a tu IA agentica con terminal los siguientes 3 elementos:")
	fmt.Println("   - El contenido completo de ideal_flow.json")
	fmt.Println("   - El contenido completo de trace_*.json")
	fmt.Println("   - El prompt que está en prompts/VERIFICATION_PROMPT.md")
	fmt.Println("3. La IA hace la comparación ideal vs real; FrameworkBravo solo entrega los artefactos.")
	fmt.Println(sep)
}

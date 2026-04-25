package bravo

import (
	"fmt"
	"strings"

	"framework-paladin/paladin"
)

type Context = paladin.Context
type Span = paladin.Span
type Decision = paladin.Decision

type Trace struct {
	trace *paladin.Trace
	ideal *IdealFlow
}

func NewTrace(appName string) *Trace {
	ideal, _ := LoadIdealFlow()
	if ideal != nil {
		fmt.Printf("[FRAMEWORKBRAVO] IdealFlow disponible: %s (%d reglas)\n", ideal.Description, len(ideal.Rules))
	} else {
		fmt.Printf("[FRAMEWORKBRAVO] IdealFlow no encontrado. Bravo analizara solo cuando exista temp/ideal_flow.json.\n")
	}
	return &Trace{
		trace: paladin.NewTrace(appName),
		ideal: ideal,
	}
}

func (t *Trace) Start() *Context {
	return t.trace.Start()
}

func (t *Trace) Flush() {
	t.trace.Flush()
	t.Analyze()
}

func (t *Trace) SetBottleneckThreshold(ms int64) {
	t.trace.SetBottleneckThreshold(ms)
}

func (t *Trace) GetIdealFlow() *IdealFlow {
	return t.ideal
}

func (t *Trace) ReloadIdealFlow() {
	t.ideal, _ = LoadIdealFlow()
	if t.ideal != nil {
		fmt.Printf("[FRAMEWORKBRAVO] IdealFlow recargado (%d reglas, %d vars criticas)\n",
			len(t.ideal.Rules), len(t.ideal.CriticalVars))
	}
}

func (t *Trace) Analyze() {
	fmt.Print("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf(" FRAMEWORKBRAVO CONTEXT READY - IDEAL + PALADIN TRACE\n")
	fmt.Print(strings.Repeat("=", 60) + "\n\n")

	if t.ideal == nil {
		fmt.Println("No se encontro ideal_flow.json")
		fmt.Println("Paladin guardo el trace real; falta el contrato ideal para comparar ideal vs real.")
		fmt.Println("Sugerencia: crea temp/ideal_flow.json antes de ejecutar el flujo.")
		return
	}

	fmt.Printf("Descripcion: %s\n", t.ideal.Description)
	fmt.Printf("Verbalizacion cargada: %v\n", t.ideal.Verbalization != "")
	fmt.Printf("Reglas definidas: %d\n", len(t.ideal.Rules))
	fmt.Printf("Variables criticas: %d\n\n", len(t.ideal.CriticalVars))

	if len(t.ideal.CriticalVars) > 0 {
		fmt.Println("Variables criticas definidas:")
		for _, v := range t.ideal.CriticalVars {
			fmt.Printf("  - %s\n", v)
		}
	}

	fmt.Println("\nFrameworkBravo no hace la comparacion semantica solo.")
	fmt.Println("Su responsabilidad es juntar IdealFlow + trace Paladin para que una IA agentica compare ideal vs real.")
	fmt.Println("Usa prompts/VERIFICATION_PROMPT.md para la evaluacion.")
}

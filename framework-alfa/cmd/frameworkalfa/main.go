package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"framework-alfa/internal/alfa"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "compile":
		cmdCompile(os.Args[2:])
	case "export-bravo":
		cmdExportBravo(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func cmdCompile(args []string) {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	echoTree := fs.String("echo-tree", "", "path a frameworkecho.json")
	opportunity := fs.String("opportunity", "", "id de OPPORTUNITY validada (opcional; si falta usa todas)")
	out := fs.String("out", "alfa_spec.json", "path de salida para alfa_spec.json")
	allowDraft := fs.Bool("allow-draft", false, "permite compilar un draft temprano desde PAIN/TASK aunque no haya OPPORTUNITY validada")
	_ = fs.Parse(args)

	spec, err := alfa.Compile(alfa.CompileOptions{
		EchoTreePath: *echoTree,
		Opportunity:  *opportunity,
		OutputPath:   *out,
		AllowDraft:   *allowDraft,
	})
	if err != nil {
		fail(err)
	}
	if err := alfa.SaveSpec(spec, *out); err != nil {
		fail(err)
	}

	fmt.Printf("✓ Alfa spec generado: %s\n", *out)
	fmt.Printf("  Intent: %s\n", spec.AutomationIntent)
	fmt.Printf("  Opportunities: %d | Pains: %d | Open questions: %d | Export ready: %v\n",
		len(spec.SelectedOpportunities), len(spec.ConfirmedPains), len(spec.OpenQuestions), spec.ExportReady)
	printQuestions(spec)
}

func cmdExportBravo(args []string) {
	fs := flag.NewFlagSet("export-bravo", flag.ExitOnError)
	specPath := fs.String("spec", "alfa_spec.json", "path a alfa_spec.json")
	out := fs.String("out", "ideal_flow.json", "path de salida para ideal_flow.json compatible con Bravo")
	allowDraft := fs.Bool("allow-draft", true, "exportar aunque existan open_questions")
	_ = fs.Parse(args)

	spec, err := alfa.LoadSpec(*specPath)
	if err != nil {
		fail(err)
	}
	if !spec.ExportReady && !*allowDraft {
		fail(fmt.Errorf("spec tiene %d open_questions; usa --allow-draft=true o resuelve las preguntas", len(spec.OpenQuestions)))
	}

	flow := alfa.ExportBravo(spec, time.Now())
	if err := alfa.SaveBravo(flow, *out); err != nil {
		fail(err)
	}
	fmt.Printf("✓ Bravo ideal flow generado: %s\n", *out)
	fmt.Printf("  Rules: %d | Critical vars: %d | Critical path: %d\n",
		len(flow.Rules), len(flow.CriticalVars), len(flow.CriticalPath))
	if !spec.ExportReady {
		fmt.Printf("  Nota: exportado como draft con %d preguntas abiertas.\n", len(spec.OpenQuestions))
	}
}

func cmdInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	specPath := fs.String("spec", "alfa_spec.json", "path a alfa_spec.json")
	_ = fs.Parse(args)

	spec, err := alfa.LoadSpec(*specPath)
	if err != nil {
		fail(err)
	}
	fmt.Printf("Framework Alfa Spec\n")
	fmt.Printf("Intent: %s\n", spec.AutomationIntent)
	fmt.Printf("Export ready: %v\n", spec.ExportReady)
	fmt.Printf("Opportunities: %s\n", strings.Join(opportunityTitles(spec), ", "))
	fmt.Printf("Pains: %d | Steps: %d | Rules: %d | Open questions: %d\n",
		len(spec.ConfirmedPains), len(spec.IdealSteps), len(spec.BusinessRules), len(spec.OpenQuestions))
	printQuestions(spec)
}

func printQuestions(spec *alfa.AlfaSpec) {
	if len(spec.OpenQuestions) == 0 {
		return
	}
	fmt.Println("  Preguntas para devolver a Echo:")
	for _, question := range spec.OpenQuestions {
		fmt.Printf("    %s: %s\n", question.ID, question.QuestionForEcho)
		fmt.Printf("      razón: %s\n", question.Reason)
	}
}

func opportunityTitles(spec *alfa.AlfaSpec) []string {
	out := make([]string, 0, len(spec.SelectedOpportunities))
	for _, op := range spec.SelectedOpportunities {
		out = append(out, op.ID+" "+op.Title)
	}
	return out
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func usage() {
	fmt.Println(`Framework Alfa - compilador semántico Echo → Bravo

USO:
  frameworkalfa compile --echo-tree <frameworkecho.json> [--opportunity op_001] [--out alfa_spec.json] [--allow-draft=true]
  frameworkalfa export-bravo --spec <alfa_spec.json> [--out ideal_flow.json] [--allow-draft=true]
  frameworkalfa inspect --spec <alfa_spec.json>

IDEA:
  Echo descubre dolores y oportunidades.
  Alfa traduce ese árbol a una especificación de flujo ideal.
  Bravo usa ideal_flow.json para comparar código real vs flujo esperado.

REGLA:
  Si Alfa no tiene información suficiente, no inventa: genera open_questions para Echo.`)
}

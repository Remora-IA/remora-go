package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"framework-alfa/internal/alfa"
	"framework-alfa/internal/llm"
)

// answersSidecarPath devuelve la ruta del archivo donde se persisten las
// respuestas a open_questions, junto al spec. Así sobreviven a recompiles.
func answersSidecarPath(specPath string) string {
	dir := filepath.Dir(specPath)
	base := strings.TrimSuffix(filepath.Base(specPath), ".json")
	return filepath.Join(dir, base+".answers.json")
}

func loadAnswers(specPath string) (map[string]string, error) {
	data, err := os.ReadFile(answersSidecarPath(specPath))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	out := map[string]string{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func saveAnswers(specPath string, answers map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(answers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(answersSidecarPath(specPath), append(data, '\n'), 0644)
}

// cmdNextQuestionAlfa imprime la próxima open_question sin responder como JSON:
//
//	{ "id": "oq_001", "text": "...", "ask_via": "echo" }
//
// Si no hay, imprime {}. El spec debe existir (compilado previamente con
// frameworkalfa compile). Si no existe y se pasa --echo-tree, se compila draft.
func cmdNextQuestionAlfa() {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	specPath := fs.String("spec", "alfa_spec.json", "path al alfa_spec.json")
	echoTree := fs.String("echo-tree", "", "(opcional) compila draft si el spec no existe")
	_ = fs.Parse(os.Args[2:])

	if _, err := os.Stat(*specPath); os.IsNotExist(err) && *echoTree != "" {
		spec, err := alfa.Compile(alfa.CompileOptions{
			EchoTreePath: *echoTree, OutputPath: *specPath, AllowDraft: true,
		})
		if err == nil {
			_ = alfa.SaveSpec(spec, *specPath)
		}
	}

	spec, err := alfa.LoadSpec(*specPath)
	if err != nil {
		// Sin spec disponible: respondemos vacío (no es error fatal).
		fmt.Println("{}")
		return
	}

	answers, _ := loadAnswers(*specPath)
	out := map[string]string{}
	for _, oq := range spec.OpenQuestions {
		if _, answered := answers[oq.ID]; answered {
			continue
		}
		if oq.Answer != "" {
			continue
		}
		out["id"] = oq.ID
		out["text"] = generateAlfaQuestion(oq, spec)
		out["ask_via"] = "echo"
		out["reason"] = oq.Reason
		break
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// cmdIngestAnswerAlfa persiste la respuesta del usuario a una open_question.
// Output: { "ok": true, "question_id": "oq_001" }
func cmdIngestAnswerAlfa() {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	specPath := fs.String("spec", "alfa_spec.json", "path al alfa_spec.json")
	questionID := fs.String("question-id", "", "id de la open_question (oq_NNN)")
	answer := fs.String("answer", "", "respuesta del usuario")
	_ = fs.Parse(os.Args[2:])

	if *questionID == "" || *answer == "" {
		fmt.Fprintln(os.Stderr, "Error: --question-id y --answer son obligatorios")
		os.Exit(1)
	}

	answers, err := loadAnswers(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	answers[*questionID] = *answer
	if err := saveAnswers(*specPath, answers); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Best-effort: actualizar el spec en disco si existe.
	if spec, err := alfa.LoadSpec(*specPath); err == nil {
		for i := range spec.OpenQuestions {
			if spec.OpenQuestions[i].ID == *questionID {
				spec.OpenQuestions[i].Answer = *answer
				break
			}
		}
		_ = alfa.SaveSpec(spec, *specPath)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]interface{}{
		"ok":          true,
		"question_id": *questionID,
	})
}

const alfaSystemPrompt = `Eres Alfa, el compilador semántico de Remora. Tu trabajo es traducir el árbol de descubrimiento de Echo en un flujo ideal verificable por Bravo.

Tu personalidad:
- Eres técnico pero accesible.
- Hablas en español natural.
- Cuando necesitás información, pedís cosas concretas: datos, ejemplos, capturas.
- No inventás reglas de negocio que Echo no validó.
- No ofrecés soluciones vagas.

Cuando generás una pregunta, debe ser directa y orientada a desbloquear la compilación del flujo. No explicás el framework.`

func generateAlfaQuestion(oq alfa.OpenQuestion, spec *alfa.AlfaSpec) string {
	client, err := llm.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error LLM: %v\n", err)
		return oq.QuestionForEcho
	}
	opTitle := spec.AutomationIntent
	if len(spec.SelectedOpportunities) > 0 {
		opTitle = spec.SelectedOpportunities[0].Title
	}
	userPrompt := fmt.Sprintf(
		"Pregunta original del spec: %s\nRazón: %s\nOportunidad compilada: %s\n\nReformulá esta pregunta de forma natural y directa para hacérsela al usuario. Solo la pregunta, sin explicación.",
		oq.QuestionForEcho, oq.Reason, opTitle)
	reply, err := client.Generate(context.Background(), alfaSystemPrompt, userPrompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error LLM: %v\n", err)
		return oq.QuestionForEcho
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return oq.QuestionForEcho
	}
	return reply
}

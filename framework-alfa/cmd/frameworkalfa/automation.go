package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"framework-alfa/internal/alfa"
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
		out["text"] = oq.QuestionForEcho
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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/user/framework-inspector/internal"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: inspector <command> [flags]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "next-question":
		cmdNextQuestion()
	case "ingest-answer":
		cmdIngestAnswer()
	case "test-endpoint":
		cmdTestEndpoint()
	case "search-docs":
		cmdSearchDocs()
	case "reset":
		cmdReset()
	case "status":
		cmdStatus()
	default:
		fmt.Fprintf(os.Stderr, "Comando desconocido: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// ─── next-question ────────────────────────────────────────────────────────────

type Question struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Reasoning string `json:"reasoning,omitempty"`
}

func cmdNextQuestion() {
	s, err := internal.LoadState(internal.StateFile)
	if err != nil {
		outputQuestion(Question{ID: "error", Text: "No pude cargar el estado de la conversación."})
		return
	}

	if s.Done {
		outputJSON(map[string]any{})
		return
	}

	switch s.Step {
	case internal.StepIntro:
		outputQuestion(Question{
			ID:        "api_name",
			Text:      "¿A qué API o sistema querés conectarte? Podés escribir el nombre o una descripción.",
			Reasoning: "Starting connection discovery",
		})

	case internal.StepURL:
		docsContext := ""
		if len(s.ExaDocs) > 0 {
			docsContext = "\n\n" + internal.FormatDocsForQuestion(s.ExaDocs, s.APIName)
		}
		outputQuestion(Question{
			ID:   "base_url",
			Text: fmt.Sprintf("Entendido, \"%s\".%s\n\n¿Cuál es la URL base de la API? (ej: https://api.sistema.com/v2)", s.APIName, docsContext),
		})

	case internal.StepAuth:
		outputQuestion(Question{
			ID:   "auth_token",
			Text: fmt.Sprintf("URL registrada: %s\n\n¿Esta API necesita autenticación? Pegá el token o API key si tenés uno. Si no necesita, decí \"sin token\".", s.BaseURL),
		})

	case internal.StepTesting:
		if s.TestResult == nil {
			outputQuestion(Question{
				ID:   "wait_test",
				Text: "Testeando la conexión…",
			})
			return
		}
		if s.TestResult.Success {
			outputQuestion(Question{
				ID:   "conn_name",
				Text: fmt.Sprintf("✓ Conexión exitosa (%d ms, status %d).\n\n%s\n\n¿Cómo querés llamar a esta conexión para identificarla?",
					s.TestResult.LatencyMS, s.TestResult.StatusCode, s.TestResult.Diagnosis),
			})
		} else {
			// Suggest retry with different approach
			retryMsg := buildRetryMessage(s)
			outputQuestion(Question{
				ID:   "retry_input",
				Text: fmt.Sprintf("%s\n\n%s", s.TestResult.Diagnosis, retryMsg),
			})
		}

	case internal.StepName:
		outputQuestion(Question{
			ID:   "conn_name",
			Text: "¿Cómo querés llamar a esta conexión para identificarla?",
		})

	case internal.StepDone:
		outputJSON(map[string]any{})
	}
}

func buildRetryMessage(s *internal.State) string {
	if s.TestResult == nil {
		return "¿Querés intentar con otra URL o token?"
	}
	switch s.TestResult.StatusCode {
	case 401:
		return "¿Tenés otro token, o el token tiene que ir en un header diferente? Podés escribir: \"header X-API-Key: mitoken\" para especificar el header."
	case 404:
		if len(s.ExaDocs) > 0 {
			return fmt.Sprintf("Busqué en la documentación y la URL base correcta podría ser diferente. ¿Querés probar con otra URL?")
		}
		return "¿La URL es correcta? A veces el endpoint base es /api, /v1, /v2, /rest o similar."
	case 403:
		return "Tu token es válido pero no tiene permisos suficientes. ¿Tenés un token con más permisos o acceso de administrador?"
	default:
		if s.TestResult.ErrorMsg != "" {
			return "¿Querés corregir la URL o el token y volver a intentar? Escribí la corrección o decí \"saltar\" para continuar sin verificar."
		}
		return "¿Querés intentar de nuevo con otro token o URL?"
	}
}

// ─── ingest-answer ────────────────────────────────────────────────────────────

func cmdIngestAnswer() {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	questionID := fs.String("question-id", "", "")
	answer := fs.String("answer", "", "")
	fs.Parse(os.Args[2:])

	s, err := internal.LoadState(internal.StateFile)
	if err != nil {
		s = &internal.State{Step: internal.StepIntro}
	}

	ans := strings.TrimSpace(*answer)
	qid := strings.TrimSpace(*questionID)

	switch {
	case qid == "api_name" || s.Step == internal.StepIntro:
		s.APIName = ans
		s.Step = internal.StepURL
		// Buscar docs en segundo plano (sincrónico para simplicidad)
		docs, err := internal.SearchDocs(ans)
		if err == nil {
			s.ExaDocs = docs
		}

	case qid == "base_url" || s.Step == internal.StepURL:
		url := ans
		if !strings.HasPrefix(url, "http") {
			url = "https://" + url
		}
		s.BaseURL = url
		s.Step = internal.StepAuth

	case qid == "auth_token" || s.Step == internal.StepAuth:
		ansLower := strings.ToLower(ans)
		if ansLower == "sin token" || ansLower == "no" || ansLower == "ninguno" || ansLower == "no necesita" {
			s.AuthToken = ""
		} else if strings.HasPrefix(ans, "header ") {
			// Formato: "header X-API-Key: mitoken"
			rest := strings.TrimPrefix(ans, "header ")
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) == 2 {
				s.AuthHeader = strings.TrimSpace(parts[0])
				s.AuthToken = strings.TrimSpace(parts[1])
			} else {
				s.AuthToken = rest
			}
		} else {
			s.AuthToken = ans
		}
		// Ejecutar test inmediatamente
		s.Step = internal.StepTesting
		result := internal.TestEndpoint(s.BaseURL, s.AuthToken, s.AuthHeader, "", "")
		s.TestResult = result
		if result.Success {
			s.Step = internal.StepName
		}
		// Si falla, quedamos en StepTesting para que next-question ofrezca retry

	case qid == "retry_input" || (s.Step == internal.StepTesting && s.TestResult != nil && !s.TestResult.Success):
		ansLower := strings.ToLower(ans)
		if ansLower == "saltar" || ansLower == "skip" || ansLower == "continuar" {
			s.Step = internal.StepName
		} else if strings.HasPrefix(ans, "http") {
			// Nueva URL
			s.BaseURL = ans
			s.TestResult = nil
			result := internal.TestEndpoint(s.BaseURL, s.AuthToken, s.AuthHeader, "", "")
			s.TestResult = result
			if result.Success {
				s.Step = internal.StepName
			}
		} else if strings.HasPrefix(ans, "header ") {
			// Nuevo header format
			rest := strings.TrimPrefix(ans, "header ")
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) == 2 {
				s.AuthHeader = strings.TrimSpace(parts[0])
				s.AuthToken = strings.TrimSpace(parts[1])
			}
			result := internal.TestEndpoint(s.BaseURL, s.AuthToken, s.AuthHeader, "", "")
			s.TestResult = result
			if result.Success {
				s.Step = internal.StepName
			}
		} else {
			// Asumir que es un nuevo token
			s.AuthToken = ans
			result := internal.TestEndpoint(s.BaseURL, s.AuthToken, s.AuthHeader, "", "")
			s.TestResult = result
			if result.Success {
				s.Step = internal.StepName
			}
		}

	case qid == "conn_name" || s.Step == internal.StepName:
		if ans != "" {
			s.ConnName = ans
		} else {
			s.ConnName = s.APIName
		}
		s.Step = internal.StepDone
		s.Done = true
		// Escribir artifact de conexión
		writeConnectionArtifact(s)
	}

	if err := internal.SaveState(internal.StateFile, s); err != nil {
		fmt.Fprintf(os.Stderr, "Error guardando estado: %v\n", err)
	}

	outputJSON(map[string]string{"status": "ok"})
}

func writeConnectionArtifact(s *internal.State) {
	type ConnectionArtifact struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		AuthType  string `json:"auth_type"`
		AuthToken string `json:"auth_token"`
		AuthHeader string `json:"auth_header"`
		Verified  bool   `json:"verified"`
	}
	authType := "none"
	if s.AuthToken != "" {
		authType = "bearer"
		if s.AuthHeader != "" && s.AuthHeader != "Authorization" {
			authType = "header"
		}
	}
	artifact := ConnectionArtifact{
		Name:       s.ConnName,
		BaseURL:    s.BaseURL,
		AuthType:   authType,
		AuthToken:  s.AuthToken,
		AuthHeader: s.AuthHeader,
		Verified:   s.TestResult != nil && s.TestResult.Success,
	}
	data, _ := json.MarshalIndent(artifact, "", "  ")
	os.WriteFile("inspector_connection.json", data, 0644)
}

// ─── test-endpoint ────────────────────────────────────────────────────────────

func cmdTestEndpoint() {
	fs := flag.NewFlagSet("test-endpoint", flag.ExitOnError)
	url := fs.String("url", "", "URL a testear")
	token := fs.String("token", "", "Token o API key")
	header := fs.String("header", "Authorization", "Nombre del header de auth")
	user := fs.String("user", "", "Usuario para Basic Auth")
	pass := fs.String("pass", "", "Contraseña para Basic Auth")
	method := fs.String("method", "GET", "Método HTTP (GET, POST, OPTIONS)")
	body := fs.String("body", "", "JSON body para POST requests")
	fs.Parse(os.Args[2:])

	if *url == "" {
		fmt.Fprintln(os.Stderr, "Error: --url requerido")
		os.Exit(1)
	}

	tok, hdr := *token, *header
	if *user != "" && *pass != "" {
		tok = internal.BasicAuthToken(*user, *pass)
		hdr = "Authorization"
	}

	result := internal.TestEndpoint(*url, tok, hdr, *method, *body)
	outputJSON(result)
}

// ─── search-docs ──────────────────────────────────────────────────────────────

func cmdSearchDocs() {
	fs := flag.NewFlagSet("search-docs", flag.ExitOnError)
	query := fs.String("query", "", "Consulta de búsqueda")
	fs.Parse(os.Args[2:])

	if *query == "" {
		fmt.Fprintln(os.Stderr, "Error: --query requerido")
		os.Exit(1)
	}

	results, err := internal.SearchDocs(*query)
	if err != nil {
		outputJSON(map[string]string{"error": err.Error()})
		return
	}
	outputJSON(results)
}

// ─── reset ────────────────────────────────────────────────────────────────────

func cmdReset() {
	os.Remove(internal.StateFile)
	os.Remove("inspector_connection.json")
	outputJSON(map[string]string{"status": "reset"})
}

// ─── status ───────────────────────────────────────────────────────────────────

func cmdStatus() {
	s, err := internal.LoadState(internal.StateFile)
	if err != nil {
		outputJSON(map[string]string{"error": err.Error()})
		return
	}
	outputJSON(s)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func outputQuestion(q Question) {
	outputJSON(q)
}

func outputJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializando JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

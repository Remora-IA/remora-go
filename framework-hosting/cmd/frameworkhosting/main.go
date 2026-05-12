// frameworkhosting: framework conversacional que se conecta al panel de
// hosting del usuario (cPanel/Plesk/DirectAdmin) usando solo credenciales
// estándar y opera sobre la API del panel.
//
// POC mínimo: solo cPanel UAPI, comandos `connect` y `list-emails`.
//
// Contrato del orquestador remora-flujo:
//
//	./frameworkhosting next-question
//	    Devuelve {"id","text"} con la próxima cosa que mostrar al usuario,
//	    o {} si no hay nada pendiente.
//
//	./frameworkhosting ingest-answer --question-id <id> --answer <text>
//	    Procesa input del usuario. Por ahora reconoce comandos simples:
//	      "conectar <host> <user> <pass>"  → equivalente a `connect`
//	      "listar correos" / "list emails" → equivalente a `list-emails`
//
//	./frameworkhosting connect --host <h> --user <u> --pass <p> [--conv-id <id>]
//	    Verifica login al panel y persiste credenciales encriptadas.
//
//	./frameworkhosting list-emails [--conv-id <id>]
//	    Lista cuentas de email del dominio (Email/list_pops).
//
//	./frameworkhosting genkey
//	    Genera una clave AES-256 para HOSTING_VAULT_KEY (setup inicial).
package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/credentials"
	"framework-hosting/internal/cpanel"
	"framework-hosting/internal/creds"
)

const (
	defaultStateDir = "temp"
)

type frameworkQuestion struct {
	ID               string `json:"id"`
	Text             string `json:"text"`
	Title            string `json:"title,omitempty"`
	Framework        string `json:"framework,omitempty"`
	Capability       string `json:"capability,omitempty"`
	Kind             string `json:"kind,omitempty"`
	Field            string `json:"field,omitempty"`
	FieldLabel       string `json:"field_label,omitempty"`
	InputType        string `json:"input_type,omitempty"`
	Placeholder      string `json:"placeholder,omitempty"`
	Secret           bool   `json:"secret,omitempty"`
	Step             string `json:"step,omitempty"`
	RequiredArtifact string `json:"required_artifact,omitempty"`
	NextTransition   string `json:"next_transition,omitempty"`
}

// state mantiene el progreso conversacional del framework. Vive en disco,
// separado por conversación.
type state struct {
	PendingQuestion *frameworkQuestion `json:"pending_question,omitempty"`
	SetupStep       string             `json:"setup_step,omitempty"`
	Domain          string             `json:"domain,omitempty"`
	Host            string             `json:"host,omitempty"`
	User            string             `json:"user,omitempty"`
	SMTPEmail       string             `json:"smtp_email,omitempty"`
	SMTPHost        string             `json:"smtp_host,omitempty"`
	SMTPPort        string             `json:"smtp_port,omitempty"`
	LastAt          time.Time          `json:"last_at,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fail("uso: frameworkhosting <comando>. Comandos: next-question, ingest-answer, connect, list-emails, genkey")
	}
	switch os.Args[1] {
	case "next-question":
		cmdNextQuestion(os.Args[2:])
	case "ingest-answer":
		cmdIngestAnswer(os.Args[2:])
	case "connect":
		cmdConnect(os.Args[2:])
	case "list-emails":
		cmdListEmails(os.Args[2:])
	case "provision-smtp":
		cmdProvisionSMTP(os.Args[2:])
	case "import-smtp":
		cmdImportSMTP(os.Args[2:])
	case "verify-smtp":
		cmdVerifySMTP(os.Args[2:])
	case "has-smtp":
		cmdHasSMTP(os.Args[2:])
	case "delete-smtp":
		cmdDeleteSMTP(os.Args[2:])
	case "discover-cpanel":
		cmdDiscoverCPanel(os.Args[2:])
	case "genkey":
		cmdGenKey()
	default:
		fail("comando desconocido: %s", os.Args[1])
	}
}

// cmdNextQuestion devuelve el siguiente paso explícito del wizard de Hosting.
func cmdNextQuestion(args []string) {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	statePath := fs.String("state", "", "path al state (override)")
	_ = fs.Parse(args)

	sp := resolveStatePath(*statePath, *convID)
	s := loadState(sp)

	if s.PendingQuestion != nil {
		out := *s.PendingQuestion
		s.PendingQuestion = nil
		_ = saveState(sp, s)
		printJSON(out)
		return
	}
	if smtpAlreadyConfigured(*convID) {
		printJSON(map[string]string{})
		return
	}
	if q := questionForState(s); q != nil {
		printJSON(q)
		return
	}
	printJSON(map[string]string{}) // nada pendiente
}

// cmdIngestAnswer procesa input del usuario. Reconoce intents simples:
//   - "conectar <host> <user> <pass>"
//   - "listar correos" / "lista emails" / "list emails"
//
// Cualquier otra cosa sigue el wizard secuencial de Hosting.
func cmdIngestAnswer(args []string) {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	questionID := fs.String("question-id", "", "id de la pregunta")
	answer := fs.String("answer", "", "respuesta del usuario")
	convID := fs.String("conv-id", "", "id de la conversación")
	statePath := fs.String("state", "", "path al state")
	_ = fs.Parse(args)
	_ = questionID

	if *answer == "" {
		fail("ingest-answer: --answer requerido")
	}

	sp := resolveStatePath(*statePath, *convID)
	cp := creds.Path(filepath.Dir(sp), *convID)

	s := loadState(sp)
	s.PendingQuestion = dispatchIntent(strings.TrimSpace(*answer), strings.TrimSpace(*questionID), cp, *convID, s)
	s.LastAt = time.Now()
	_ = saveState(sp, s)
}

// dispatchIntent es un router super simple basado en prefijos. Para una v2
// usaríamos LLM o reglas más ricas, pero para POC alcanza.
func dispatchIntent(answer, questionID, credsPath, convID string, s *state) *frameworkQuestion {
	low := strings.ToLower(answer)

	switch {
	case strings.HasPrefix(low, "conectar "), strings.HasPrefix(low, "connect "):
		return questionFromConnectResult(doConnectFromText(answer, credsPath, convID), s)
	case strings.Contains(low, "listar correo"),
		strings.Contains(low, "lista emails"),
		strings.Contains(low, "list emails"),
		strings.Contains(low, "list-emails"):
		return infoQuestion("hosting_list_emails", doListEmails(credsPath), "credentials.cpanel.connect", "email_listing", "hosting_ready")
	default:
		return handleConnectWizard(answer, questionID, credsPath, convID, s)
	}
}

func handleConnectWizard(answer, questionID, credsPath, convID string, s *state) *frameworkQuestion {
	_ = questionID
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return questionForState(s)
	}
	switch s.SetupStep {
	case "", "cpanel_domain":
		s.Domain = normalizeDomain(answer)
		if looksLikeDirectCPanelHost(answer) {
			s.Host = sanitizeHostAnswer(answer)
			s.SetupStep = "cpanel_user"
			return questionForState(s)
		}
		best, found := discoverBestCPanel(s.Domain)
		if found {
			s.Host = best
			s.SetupStep = "cpanel_user"
			return &frameworkQuestion{
				ID:               questionIDForStep("cpanel_user"),
				Text:             fmt.Sprintf("Encontré un cPanel probable en %s. Ahora necesito el usuario de cPanel para continuar con la conexión.", best),
				Title:            "Conectar hosting",
				Framework:        "hosting",
				Capability:       "credentials.cpanel.connect",
				Kind:             "conversational_question",
				Field:            "cpanel_user",
				FieldLabel:       "Usuario de cPanel",
				InputType:        "text",
				Placeholder:      "usuario o usuario@dominio.com",
				Step:             "cpanel_user",
				RequiredArtifact: "credentials.smtp",
				NextTransition:   "request_cpanel_user",
			}
		}
		s.Host = ""
		s.SetupStep = "cpanel_host"
		return &frameworkQuestion{
			ID:               questionIDForStep("cpanel_host"),
			Text:             fmt.Sprintf("Probé descubrir cPanel para %s y no encontré un endpoint confiable. Compartime el host o URL de cPanel para seguir con Hosting.", firstNonEmpty(answer, "tu dominio")),
			Title:            "Conectar hosting",
			Framework:        "hosting",
			Capability:       "credentials.cpanel.connect",
			Kind:             "conversational_question",
			Field:            "cpanel_host",
			FieldLabel:       "Host de cPanel",
			InputType:        "text",
			Placeholder:      "cpanel.tudominio.com",
			Step:             "cpanel_host",
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_cpanel_host",
		}
	case "cpanel_host":
		s.Host = sanitizeHostAnswer(answer)
		s.SetupStep = "cpanel_user"
		return questionForState(s)
	case "cpanel_user":
		s.User = strings.TrimSpace(answer)
		s.SetupStep = "cpanel_pass"
		return questionForState(s)
	case "cpanel_pass":
		return questionFromConnectResult(doConnectWizard(s.Host, s.User, answer, credsPath, convID), s)
	case "smtp_email":
		s.SMTPEmail = strings.TrimSpace(answer)
		if s.SMTPHost == "" {
			s.SMTPHost = defaultSMTPHostForEmail(s.SMTPEmail, s.Domain)
		}
		if s.SMTPPort == "" {
			s.SMTPPort = "587"
		}
		s.SetupStep = "smtp_pass"
		return questionForState(s)
	case "smtp_pass":
		return questionFromSMTPImportResult(importSMTPWizard(s, answer, convID), s)
	case "smtp_host":
		s.SMTPHost = sanitizeHostAnswer(answer)
		if s.SMTPPort == "" {
			s.SMTPPort = "587"
		}
		s.SetupStep = "smtp_port"
		return questionForState(s)
	case "smtp_port":
		port := strings.TrimSpace(answer)
		if port == "" {
			port = "587"
		}
		s.SMTPPort = port
		s.SetupStep = "smtp_pass"
		return &frameworkQuestion{
			ID:               questionIDForStep("smtp_pass"),
			Text:             "Perfecto. Ahora necesito la contraseña de esa casilla para verificar el acceso SMTP real.",
			Title:            "Importar casilla existente",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_pass",
			FieldLabel:       "Contraseña de la casilla",
			InputType:        "password",
			Secret:           true,
			Step:             "smtp_pass",
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "verify_imported_smtp",
		}
	default:
		resetSetupState(s)
		return questionForState(s)
	}
}

func sanitizeHostAnswer(answer string) string {
	answer = strings.TrimSpace(answer)
	answer = strings.TrimPrefix(answer, "https://")
	answer = strings.TrimPrefix(answer, "http://")
	answer = strings.TrimSuffix(answer, "/")
	fields := strings.Fields(answer)
	if len(fields) == 1 {
		return fields[0]
	}
	return answer
}

func questionForState(s *state) *frameworkQuestion {
	step := strings.TrimSpace(s.SetupStep)
	if step == "" {
		step = "cpanel_domain"
		s.SetupStep = step
	}
	switch step {
	case "cpanel_domain":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "Veo que todavía no tengo conectado el hosting del negocio. Primero necesito el dominio principal o el host de cPanel para descubrir el panel y preparar el correo saliente.",
			Title:            "Conectar hosting",
			Framework:        "hosting",
			Capability:       "credentials.cpanel.connect",
			Kind:             "conversational_question",
			Field:            "domain",
			FieldLabel:       "Dominio principal o host de cPanel",
			InputType:        "text",
			Placeholder:      "tudominio.com",
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "discover_cpanel",
		}
	case "cpanel_user":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "Perfecto. Ahora necesito el usuario de cPanel para seguir con la conexión del hosting.",
			Title:            "Conectar hosting",
			Framework:        "hosting",
			Capability:       "credentials.cpanel.connect",
			Kind:             "conversational_question",
			Field:            "cpanel_user",
			FieldLabel:       "Usuario de cPanel",
			InputType:        "text",
			Placeholder:      "usuario o usuario@dominio.com",
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_cpanel_password",
		}
	case "cpanel_pass":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "Listo. Ahora decime la contraseña o API token de cPanel. La usaré solo para conectarme al panel y guardar los secretos cifrados.",
			Title:            "Conectar hosting",
			Framework:        "hosting",
			Capability:       "credentials.cpanel.connect",
			Kind:             "conversational_question",
			Field:            "cpanel_pass",
			FieldLabel:       "Contraseña o token de cPanel",
			InputType:        "password",
			Secret:           true,
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "connect_cpanel",
		}
	case "smtp_email":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "Ya conecté el hosting, pero no pude dejar lista la casilla de envío automáticamente. Si ya tienes una casilla existente, compartime primero el email completo que quieres usar para enviar.",
			Title:            "Importar casilla existente",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_email",
			FieldLabel:       "Email de la casilla",
			InputType:        "email",
			Placeholder:      "cobranza@tudominio.com",
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_smtp_password",
		}
	case "smtp_pass":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "Gracias. Ahora necesito la contraseña de esa casilla para verificar el acceso SMTP real.",
			Title:            "Importar casilla existente",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_pass",
			FieldLabel:       "Contraseña de la casilla",
			InputType:        "password",
			Secret:           true,
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "verify_imported_smtp",
		}
	case "smtp_host":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "No pude verificar la casilla con el host por defecto. Compartime el host SMTP real para probar de nuevo.",
			Title:            "Ajustar SMTP",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_host",
			FieldLabel:       "Host SMTP",
			InputType:        "text",
			Placeholder:      "mail.tudominio.com",
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_smtp_port",
		}
	case "smtp_port":
		return &frameworkQuestion{
			ID:               questionIDForStep(step),
			Text:             "Último paso: ¿qué puerto SMTP quieres usar? Si no estás seguro, normalmente es 587.",
			Title:            "Ajustar SMTP",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_port",
			FieldLabel:       "Puerto SMTP",
			InputType:        "text",
			Placeholder:      "587",
			Step:             step,
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_smtp_password",
		}
	default:
		resetSetupState(s)
		return questionForState(s)
	}
}

func questionIDForStep(step string) string {
	return "hosting_" + strings.TrimSpace(step)
}

func infoQuestion(id, text, capability, step, transition string) *frameworkQuestion {
	return &frameworkQuestion{
		ID:               id,
		Text:             text,
		Title:            "Hosting",
		Framework:        "hosting",
		Capability:       capability,
		Kind:             "conversational_question",
		Field:            "ack",
		FieldLabel:       "Continuar",
		InputType:        "text",
		Step:             step,
		RequiredArtifact: "credentials.smtp",
		NextTransition:   transition,
	}
}

func smtpAlreadyConfigured(convID string) bool {
	_, status := loadSMTPBundle(convID)
	return status.Present && status.Readable && status.Complete
}

func looksLikeDirectCPanelHost(answer string) bool {
	host := strings.ToLower(sanitizeHostAnswer(answer))
	return strings.Contains(host, "cpanel.") || strings.Contains(host, ":2083") || strings.Contains(host, ":2082")
}

func resetSetupState(s *state) {
	s.SetupStep = "cpanel_domain"
	s.Domain = ""
	s.Host = ""
	s.User = ""
	s.SMTPEmail = ""
	s.SMTPHost = ""
	s.SMTPPort = ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultSMTPHostForEmail(emailAddr, fallbackDomain string) string {
	domain := fallbackDomain
	if parts := strings.Split(strings.TrimSpace(emailAddr), "@"); len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		domain = strings.TrimSpace(parts[1])
	}
	if domain == "" {
		return ""
	}
	return "mail." + normalizeDomain(domain)
}

func discoverBestCPanel(domain string) (string, bool) {
	d := normalizeDomain(domain)
	if d == "" {
		return "", false
	}
	candidates := []string{
		"https://cpanel." + d + ":2083",
		"https://" + d + ":2083",
		"http://cpanel." + d + ":2082",
		"http://" + d + ":2082",
	}
	best := ""
	for _, u := range candidates {
		r := probeCPanelURL(u)
		if best == "" && r.Reachable && r.LooksCPanel {
			best = r.URL
		}
	}
	if best == "" {
		for _, u := range candidates {
			r := probeCPanelURL(u)
			if r.Reachable {
				best = r.URL
				break
			}
		}
	}
	return best, best != ""
}

// doConnectFromText parsea "conectar host user pass" y delega a doConnect.
// Tolerante con espacios extras y variantes en el verbo.
func doConnectFromText(text, credsPath, convID string) connectOutcome {
	parts := strings.Fields(text)
	if len(parts) < 4 {
		return connectOutcome{Message: "Faltan datos. Formato: `conectar <host> <usuario> <password>`"}
	}
	host, user, pass := parts[1], parts[2], strings.Join(parts[3:], " ")
	return doConnect(host, user, pass, credsPath, convID)
}

type connectOutcome struct {
	Success            bool
	SMTPReady          bool
	Message            string
	RequiresSMTPImport bool
}

type smtpImportOutcome struct {
	Success      bool
	Message      string
	NeedSMTPHost bool
}

func doConnectWizard(host, user, pass, credsPath, convID string) connectOutcome {
	return doConnect(host, user, pass, credsPath, convID)
}

func questionFromConnectResult(result connectOutcome, s *state) *frameworkQuestion {
	if result.Success && result.SMTPReady {
		resetSetupState(s)
		return infoQuestion("hosting_ready", result.Message, "credentials.cpanel.connect", "hosting_ready", "smtp_ready")
	}
	if result.Success && result.RequiresSMTPImport {
		s.SetupStep = "smtp_email"
		return &frameworkQuestion{
			ID:               questionIDForStep("smtp_email"),
			Text:             result.Message + "\n\nPara terminar, voy a importar una casilla existente. Empecemos por el email completo de esa casilla.",
			Title:            "Importar casilla existente",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_email",
			FieldLabel:       "Email de la casilla",
			InputType:        "email",
			Placeholder:      "cobranza@tudominio.com",
			Step:             "smtp_email",
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_smtp_password",
		}
	}
	s.SetupStep = "cpanel_pass"
	return &frameworkQuestion{
		ID:               questionIDForStep("cpanel_pass"),
		Text:             result.Message + "\n\nProbemos de nuevo con la contraseña o API token de cPanel.",
		Title:            "Conectar hosting",
		Framework:        "hosting",
		Capability:       "credentials.cpanel.connect",
		Kind:             "conversational_question",
		Field:            "cpanel_pass",
		FieldLabel:       "Contraseña o token de cPanel",
		InputType:        "password",
		Secret:           true,
		Step:             "cpanel_pass",
		RequiredArtifact: "credentials.smtp",
		NextTransition:   "connect_cpanel",
	}
}

func importSMTPWizard(s *state, pass, convID string) smtpImportOutcome {
	if s.SMTPEmail == "" {
		s.SetupStep = "smtp_email"
		return smtpImportOutcome{Message: "Primero necesito el email de la casilla que quieres usar."}
	}
	host := firstNonEmpty(s.SMTPHost, defaultSMTPHostForEmail(s.SMTPEmail, s.Domain))
	port := firstNonEmpty(s.SMTPPort, "587")
	if err := saveSMTPBundle(convID, credentials.SMTPBundle{
		Host: host, Port: port, User: s.SMTPEmail, Pass: pass, From: s.SMTPEmail,
	}); err != nil {
		return smtpImportOutcome{Message: "No pude guardar esas credenciales SMTP: " + err.Error()}
	}
	if err := verifySMTPLogin(host, port, s.SMTPEmail, pass); err != nil {
		s.SMTPHost = host
		s.SMTPPort = port
		s.SetupStep = "smtp_host"
		return smtpImportOutcome{
			Message:      "No pude verificar la casilla con el host actual: " + err.Error(),
			NeedSMTPHost: true,
		}
	}
	emailAddr := s.SMTPEmail
	resetSetupState(s)
	return smtpImportOutcome{
		Success: true,
		Message: fmt.Sprintf("Perfecto. Hosting dejó configurada la casilla %s vía %s:%s y el flujo ya puede continuar.", emailAddr, host, port),
	}
}

func questionFromSMTPImportResult(result smtpImportOutcome, s *state) *frameworkQuestion {
	if result.Success {
		return infoQuestion("hosting_smtp_ready", result.Message, "credentials.smtp.import", "smtp_ready", "resume_flow")
	}
	if result.NeedSMTPHost {
		return &frameworkQuestion{
			ID:               questionIDForStep("smtp_host"),
			Text:             result.Message + "\n\nCompartime ahora el host SMTP real para reintentar.",
			Title:            "Ajustar SMTP",
			Framework:        "hosting",
			Capability:       "credentials.smtp.import",
			Kind:             "conversational_question",
			Field:            "smtp_host",
			FieldLabel:       "Host SMTP",
			InputType:        "text",
			Placeholder:      "mail.tudominio.com",
			Step:             "smtp_host",
			RequiredArtifact: "credentials.smtp",
			NextTransition:   "request_smtp_port",
		}
	}
	return questionForState(s)
}

// doConnect prueba auth contra cPanel y, si OK, persiste credenciales.
// Luego descubre automáticamente cuentas de email y auto-configura SMTP.
func doConnect(host, user, pass, credsPath, convID string) connectOutcome {
	host = sanitizeHostAnswer(host)
	if isPlaceholderHost(host) {
		return connectOutcome{Message: "No voy a conectar contra un dominio de ejemplo. Decime el dominio real del negocio o usa el endpoint cPanel descubierto."}
	}
	cli, err := cpanel.New(host, user, pass, true)
	if err != nil {
		return connectOutcome{Message: fmt.Sprintf("Error de configuración: %v", err)}
	}
	if err := cli.Login(); err != nil {
		// Fallback 1: user con @ → probar sin @
		if local := cpanelLocalUserCandidate(user); local != "" {
			retry, retryErr := cpanel.New(host, local, pass, true)
			if retryErr == nil && retry.Login() == nil {
				cli = retry
				user = local
			} else {
				return connectOutcome{Message: cpanelLoginErrorHelp(host, user, err)}
			}
			// Fallback 2: user sin @ → probar con @dominio
		} else if full := cpanelFullUserCandidate(user, host); full != "" && full != user {
			retry, retryErr := cpanel.New(host, full, pass, true)
			if retryErr == nil && retry.Login() == nil {
				cli = retry
				user = full
			} else {
				return connectOutcome{Message: cpanelLoginErrorHelp(host, user, err)}
			}
		} else {
			return connectOutcome{Message: cpanelLoginErrorHelp(host, user, err)}
		}
	}
	if err := cli.Ping(); err != nil {
		return connectOutcome{Message: fmt.Sprintf("No pude conectar al hosting: %v", err)}
	}
	c := &creds.Credentials{
		Panel: "cpanel", Host: host, Port: 2083,
		User: user, Pass: pass, Insecure: true,
	}
	if err := creds.Save(credsPath, c); err != nil {
		return connectOutcome{Message: fmt.Sprintf("Conexión OK pero no pude guardar credenciales: %v.\n"+
			"Verificá que HOSTING_VAULT_KEY esté seteada (corré `frameworkhosting genkey` para generar una).", err)}
	}
	emailAddr, smtpHost, err := autoProvisionSMTP(cli, host, convID)
	if err != nil {
		return connectOutcome{
			Success:            true,
			Message:            fmt.Sprintf("Conecté el hosting en %s, pero no pude preparar el correo saliente automáticamente: %v", host, err),
			RequiresSMTPImport: true,
		}
	}
	return connectOutcome{
		Success:   true,
		SMTPReady: true,
		Message:   fmt.Sprintf("Conectado a %s. Preparé automáticamente el correo saliente con cPanel: %s vía %s:587. Ya puedo enviar correos con aprobación.", host, emailAddr, smtpHost),
	}
}

func cpanelLocalUserCandidate(user string) string {
	user = strings.TrimSpace(user)
	if !strings.Contains(user, "@") {
		return ""
	}
	local := strings.TrimSpace(strings.SplitN(user, "@", 2)[0])
	if local == "" || local == user {
		return ""
	}
	return local
}

func cpanelFullUserCandidate(user, host string) string {
	user = strings.TrimSpace(user)
	if strings.Contains(user, "@") {
		return ""
	}
	domain := hostDomainFromHost(host)
	if domain == "" {
		return ""
	}
	return user + "@" + domain
}

func hostDomainFromHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "cpanel.")
	host = strings.TrimPrefix(host, "mail.")
	host = strings.TrimPrefix(host, "www.")
	return host
}

func cpanelLoginErrorHelp(host, user string, err error) string {
	return fmt.Sprintf("No pude conectar al hosting: %v\n\n"+
		"Probá con el usuario completo incluyendo el dominio (ej: usuario@%s).\n"+
		"Algunos cPanels aceptan solo la contraseña web; otros requieren API token.", err, hostDomainFromHost(host))
}

func isPlaceholderHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "ejemplo.com" || host == "cpanel.ejemplo.com" || host == "example.com" || host == "cpanel.example.com" || strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".ejemplo.com")
}

// doListEmails carga credenciales y llama UAPI list_pops.
func doListEmails(credsPath string) string {
	c, err := creds.Load(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "No hay credenciales guardadas. Conectá primero con `conectar <host> <usuario> <password>`."
		}
		return fmt.Sprintf("No pude leer credenciales: %v", err)
	}
	cli, err := cpanel.New(c.Host, c.User, c.Pass, c.Insecure)
	if err != nil {
		return fmt.Sprintf("Error de configuración: %v", err)
	}
	cli.Port = c.Port
	accounts, err := cli.ListEmailAccounts()
	if err != nil {
		return fmt.Sprintf("Error listando correos: %v", err)
	}
	if len(accounts) == 0 {
		return "No hay cuentas de email en este dominio."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Cuentas de email en %s (%d):\n", c.Host, len(accounts))
	for _, a := range accounts {
		marker := ""
		if a.SuspendedLogin == 1 || a.SuspendedIncoming == 1 {
			marker = " [suspendida]"
		}
		fmt.Fprintf(&b, "• %s%s\n", a.Email, marker)
	}
	return strings.TrimRight(b.String(), "\n")
}

func autoProvisionSMTP(cli *cpanel.Client, fallbackHost, convID string) (string, string, error) {
	domain := fallbackHost
	if d, err := cli.ListDomains(); err == nil && strings.TrimSpace(d.MainDomain) != "" {
		domain = strings.TrimSpace(d.MainDomain)
	}
	domain = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(domain, "cpanel."), "mail."), "www.")
	local := "remora-cobranza"
	password := generatePassword()
	emailAddr, err := cli.AddPop(cpanel.AddPopParams{
		Email: local, Domain: domain, Password: password, QuotaMB: 250,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "exist") {
			local = "remora-cobranza-" + fmt.Sprint(time.Now().Unix())
			emailAddr, err = cli.AddPop(cpanel.AddPopParams{
				Email: local, Domain: domain, Password: password, QuotaMB: 250,
			})
		}
		if err != nil {
			return "", "", err
		}
	}
	smtpHost := "mail." + domain
	bundle := credentials.SMTPBundle{
		Host:      smtpHost,
		Port:      "587",
		User:      emailAddr,
		Pass:      password,
		From:      emailAddr,
		DefaultTo: "",
	}
	if err := saveSMTPBundle(convID, bundle); err != nil {
		return "", "", err
	}
	return emailAddr, smtpHost, nil
}

// cmdConnect: modo CLI directo, equivalente a `connect host user pass` sin
// pasar por el estado conversacional.
func cmdConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	host := fs.String("host", "", "")
	user := fs.String("user", "", "")
	pass := fs.String("pass", "", "")
	convID := fs.String("conv-id", "", "")
	statePath := fs.String("state", "", "")
	_ = fs.Parse(args)
	if *host == "" || *user == "" || *pass == "" {
		fail("connect: --host, --user, --pass son requeridos")
	}
	sp := resolveStatePath(*statePath, *convID)
	cp := creds.Path(filepath.Dir(sp), *convID)
	fmt.Println(doConnect(*host, *user, *pass, cp, *convID).Message)
}

// cmdListEmails: modo CLI directo.
func cmdListEmails(args []string) {
	fs := flag.NewFlagSet("list-emails", flag.ExitOnError)
	convID := fs.String("conv-id", "", "")
	statePath := fs.String("state", "", "")
	_ = fs.Parse(args)
	sp := resolveStatePath(*statePath, *convID)
	cp := creds.Path(filepath.Dir(sp), *convID)
	fmt.Println(doListEmails(cp))
}

// cmdGenKey imprime una clave AES-256 hex lista para pegar en HOSTING_VAULT_KEY.
func cmdGenKey() {
	k, err := creds.GenerateKey()
	if err != nil {
		fail("genkey: %v", err)
	}
	fmt.Printf("HOSTING_VAULT_KEY=%s\n", k)
}

// resolveStatePath calcula el path del state.json. Por defecto:
// temp/state_<convID>.json. Si --state se pasó explícito, lo usa tal cual.
func resolveStatePath(override, convID string) string {
	if override != "" {
		return override
	}
	if convID == "" {
		return filepath.Join(defaultStateDir, "state.json")
	}
	return filepath.Join(defaultStateDir, "state_"+sanitizeID(convID)+".json")
}

func sanitizeID(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= 'A' && ch <= 'Z',
			ch >= '0' && ch <= '9',
			ch == '_', ch == '-':
			out = append(out, ch)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

func loadState(path string) *state {
	b, err := os.ReadFile(path)
	if err != nil {
		return &state{}
	}
	var s state
	if json.Unmarshal(b, &s) != nil {
		return &state{}
	}
	return &s
}

func saveState(path string, s *state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func printJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		fail("json marshal: %v", err)
	}
	fmt.Println(string(b))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "frameworkhosting: "+format+"\n", args...)
	os.Exit(1)
}

// ============================================================
// SMTP provisioning + vault wiring
// ============================================================
//
// Estos comandos producen la capability canónica `credentials.smtp` en el
// vault compartido (channel/bin/vault). El framework `mensajero` (y
// cualquier otro consumer futuro) lee esa capability sin saber que fue
// hosting quien la generó. Mantenemos la separación productor/consumidor
// vía vault, no vía imports cruzados.

// provisionSMTPResp es el JSON que devuelven provision-smtp/import-smtp
// para que el orquestador o el frontend interpreten el resultado.
type provisionSMTPResp struct {
	ArtifactType string   `json:"artifact_type,omitempty"`
	Artifacts    []string `json:"artifacts,omitempty"`
	Success      bool     `json:"success"`
	Email        string   `json:"email,omitempty"`
	SMTPHost     string   `json:"smtp_host,omitempty"`
	SMTPPort     string   `json:"smtp_port,omitempty"`
	Capability   string   `json:"capability,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// cmdProvisionSMTP: crea (o asume existente) un buzón en cPanel y guarda
// las credenciales SMTP resultantes en el vault como `credentials.smtp`.
//
// Flujo:
//  1. Carga credenciales cPanel desde el vault del hosting (las que el user
//     ya conectó con `connect`).
//  2. Llama UAPI Email/add_pop para crear el buzón.
//  3. Construye {host, port, user, pass, from, default_to} y los serializa
//     a JSON.
//  4. Shells out a `vault set` para persistir bajo `credentials.smtp`.
func cmdProvisionSMTP(args []string) {
	fs := flag.NewFlagSet("provision-smtp", flag.ExitOnError)
	mailbox := fs.String("mailbox", "", "parte local del email (ej 'cobranza')")
	domain := fs.String("domain", "", "dominio del buzón (ej 'patriciastocker.com')")
	password := fs.String("password", "", "password del buzón (vacío = autogenerada)")
	defaultTo := fs.String("default-to", "", "destinatario por defecto si el caller no especifica --to")
	convID := fs.String("conv-id", "", "id de la conversación")
	statePath := fs.String("state", "", "path al state.json")
	_ = fs.Parse(args)

	if *mailbox == "" || *domain == "" {
		emitJSONErr("provision-smtp: --mailbox y --domain requeridos")
		os.Exit(2)
	}
	if *password == "" {
		*password = generatePassword()
	}

	sp := resolveStatePath(*statePath, *convID)
	cp := creds.Path(filepath.Dir(sp), *convID)
	c, err := creds.Load(cp)
	if err != nil {
		emitJSONErr(fmt.Sprintf("provision-smtp: no hay credenciales cPanel guardadas (corré 'connect' primero): %v", err))
		os.Exit(2)
	}

	cli, err := cpanel.New(c.Host, c.User, c.Pass, c.Insecure)
	if err != nil {
		emitJSONErr("provision-smtp: " + err.Error())
		os.Exit(1)
	}
	cli.Port = c.Port
	if err := cli.Login(); err != nil {
		emitJSONErr("provision-smtp: login cPanel: " + err.Error())
		os.Exit(1)
	}
	emailAddr, err := cli.AddPop(cpanel.AddPopParams{
		Email: *mailbox, Domain: *domain, Password: *password, QuotaMB: 250,
	})
	if err != nil {
		// Caso común: el buzón ya existe. Reportamos pero seguimos con
		// los datos que tenemos para guardar en vault de todas formas
		// (asumiendo que el password pasado es el correcto).
		if !strings.Contains(strings.ToLower(err.Error()), "exist") {
			emitJSONErr("provision-smtp: add_pop: " + err.Error())
			os.Exit(1)
		}
		emailAddr = *mailbox + "@" + *domain
	}

	smtpHost := "mail." + *domain
	smtpPort := "587"
	bundle := credentials.SMTPBundle{
		Host:      smtpHost,
		Port:      smtpPort,
		User:      emailAddr,
		Pass:      *password,
		From:      emailAddr,
		DefaultTo: *defaultTo,
	}
	if err := saveSMTPBundle(*convID, bundle); err != nil {
		emitJSONErr("provision-smtp: vault set: " + err.Error())
		os.Exit(1)
	}
	emitJSON(provisionSMTPResp{
		ArtifactType: "credentials.smtp",
		Artifacts:    []string{"credentials.smtp"},
		Success:      true, Email: emailAddr,
		SMTPHost: smtpHost, SMTPPort: smtpPort,
		Capability: "credentials.smtp",
	})
}

// cmdImportSMTP: shortcut para cuando el usuario ya tiene un buzón creado
// (manualmente o vía otro panel) y solo quiere guardar las credenciales
// SMTP en el vault. No habla con cPanel.
//
// Útil para migrar el setup actual (cobranza@patriciastocker.com creado a
// mano) sin re-provisionar.
func cmdImportSMTP(args []string) {
	fs := flag.NewFlagSet("import-smtp", flag.ExitOnError)
	host := fs.String("host", "", "host SMTP (ej 'mail.patriciastocker.com')")
	port := fs.String("port", "587", "puerto SMTP")
	user := fs.String("user", "", "usuario SMTP (suele ser email completo)")
	pass := fs.String("pass", "", "password SMTP")
	from := fs.String("from", "", "from header (default: --user)")
	defaultTo := fs.String("default-to", "", "destinatario por defecto opcional")
	convID := fs.String("conv-id", "", "id de la conversación")
	_ = fs.Parse(args)

	if *host == "" || *user == "" || *pass == "" {
		emitJSONErr("import-smtp: --host, --user y --pass requeridos")
		os.Exit(2)
	}
	if *from == "" {
		*from = *user
	}
	bundle := credentials.SMTPBundle{
		Host:      *host,
		Port:      *port,
		User:      *user,
		Pass:      *pass,
		From:      *from,
		DefaultTo: *defaultTo,
	}
	if err := saveSMTPBundle(*convID, bundle); err != nil {
		emitJSONErr("import-smtp: vault set: " + err.Error())
		os.Exit(1)
	}
	emitJSON(provisionSMTPResp{
		ArtifactType: "credentials.smtp",
		Artifacts:    []string{"credentials.smtp"},
		Success:      true, Email: *user, SMTPHost: *host, SMTPPort: *port,
		Capability: "credentials.smtp",
	})
}

// cmdVerifySMTP lee las credenciales SMTP del vault y hace un login real
// contra el servidor SMTP para verificar que funcionan. Devuelve
// {"verified": bool, "error": "..."} sin enviar ningún email.
func cmdVerifySMTP(args []string) {
	fs := flag.NewFlagSet("verify-smtp", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	_ = fs.Parse(args)
	bundle, status := loadSMTPBundle(*convID)
	if !status.Present || !status.Readable || !status.Complete {
		emitJSON(map[string]interface{}{
			"artifact_type":  "credentials.smtp.verification.v1",
			"verified":       false,
			"capability":     status.Capability,
			"present":        status.Present,
			"readable":       status.Readable,
			"complete":       status.Complete,
			"ready":          false,
			"missing_fields": status.MissingFields,
			"scope":          status.Scope,
			"error":          status.Error,
		})
		return
	}
	verifyErr := verifySMTPLogin(bundle.Host, bundle.Port, bundle.User, bundle.Pass)
	if verifyErr != nil {
		emitJSON(map[string]interface{}{
			"artifact_type": "credentials.smtp.verification.v1",
			"verified":      false,
			"capability":    status.Capability,
			"present":       true,
			"readable":      true,
			"complete":      true,
			"ready":         false,
			"scope":         status.Scope,
			"error":         verifyErr.Error(),
			"host":          bundle.Host,
			"port":          bundle.Port,
			"user":          bundle.User,
		})
		return
	}
	emitJSON(map[string]interface{}{
		"artifact_type": "credentials.smtp.verification.v1",
		"verified":      true,
		"capability":    status.Capability,
		"present":       true,
		"readable":      true,
		"complete":      true,
		"ready":         true,
		"scope":         status.Scope,
		"host":          bundle.Host,
		"port":          bundle.Port,
		"user":          bundle.User,
	})
}

// verifySMTPLogin intenta hacer login SMTP real (STARTTLS o TLS directo)
// sin enviar ningún email. Timeout de 10 segundos.
func verifySMTPLogin(host, port, user, pass string) error {
	addr := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tlsCfg := &tls.Config{ServerName: host}

	var c *smtp.Client
	if port == "465" {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("no pude conectar a %s (TLS): %v", addr, err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("handshake SMTP falló en %s: %v", addr, err)
		}
		c = client
	} else {
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("no pude conectar a %s: %v", addr, err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("handshake SMTP falló en %s: %v", addr, err)
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				client.Close()
				return fmt.Errorf("STARTTLS falló en %s: %v", addr, err)
			}
		}
		c = client
	}
	defer c.Close()
	auth := smtp.PlainAuth("", user, pass, host)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("login SMTP falló (%s@%s): %v", user, host, err)
	}
	return nil
}

// cmdHasSMTP valida si credentials.smtp está realmente lista para usar:
// existe en vault, puede desencriptarse, está completa y permite login SMTP
// real con el mismo scope que luego usará Mensajero.
func cmdHasSMTP(args []string) {
	fs := flag.NewFlagSet("has-smtp", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	_ = fs.Parse(args)
	bundle, status := loadSMTPBundle(*convID)
	if !status.Present || !status.Readable || !status.Complete {
		emitJSON(map[string]interface{}{
			"artifact_type":  "credentials.status.v1",
			"available":      false,
			"present":        status.Present,
			"readable":       status.Readable,
			"complete":       status.Complete,
			"ready":          false,
			"verified":       false,
			"capability":     status.Capability,
			"missing_fields": status.MissingFields,
			"scope":          status.Scope,
			"error":          status.Error,
		})
		return
	}
	if verifyErr := verifySMTPLogin(bundle.Host, bundle.Port, bundle.User, bundle.Pass); verifyErr != nil {
		emitJSON(map[string]interface{}{
			"artifact_type": "credentials.status.v1",
			"available":     false,
			"present":       true,
			"readable":      true,
			"complete":      true,
			"ready":         false,
			"verified":      false,
			"capability":    status.Capability,
			"scope":         status.Scope,
			"error":         verifyErr.Error(),
			"host":          bundle.Host,
			"port":          bundle.Port,
			"user":          bundle.User,
		})
		return
	}
	emitJSON(map[string]interface{}{
		"artifact_type": "credentials.status.v1",
		"available":     true,
		"present":       true,
		"readable":      true,
		"complete":      true,
		"ready":         true,
		"verified":      true,
		"capability":    status.Capability,
		"scope":         status.Scope,
		"host":          bundle.Host,
		"port":          bundle.Port,
		"user":          bundle.User,
	})
}

// cmdDeleteSMTP elimina credentials.smtp del vault.
func cmdDeleteSMTP(args []string) {
	fs := flag.NewFlagSet("delete-smtp", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	_ = fs.Parse(args)
	err := credentials.DeleteSMTP("", *convID)
	deleted := err == nil
	emitJSON(map[string]interface{}{
		"artifact_type": "credentials.deleted.v1",
		"deleted":       deleted,
		"capability":    "credentials.smtp",
	})
}

type cpanelDiscoveryResp struct {
	ArtifactType string                  `json:"artifact_type"`
	Domain       string                  `json:"domain"`
	Found        bool                    `json:"found"`
	Best         string                  `json:"best,omitempty"`
	Candidates   []cpanelCandidateResult `json:"candidates"`
}

type cpanelCandidateResult struct {
	URL         string `json:"url"`
	Reachable   bool   `json:"reachable"`
	StatusCode  int    `json:"status_code,omitempty"`
	LooksCPanel bool   `json:"looks_cpanel"`
	Error       string `json:"error,omitempty"`
}

func cmdDiscoverCPanel(args []string) {
	fs := flag.NewFlagSet("discover-cpanel", flag.ExitOnError)
	domain := fs.String("domain", "", "dominio principal del negocio")
	_ = fs.Parse(args)
	d := normalizeDomain(*domain)
	if d == "" {
		emitJSONErr("discover-cpanel: --domain requerido")
		os.Exit(2)
	}
	candidates := []string{
		"https://cpanel." + d + ":2083",
		"https://" + d + ":2083",
		"http://cpanel." + d + ":2082",
		"http://" + d + ":2082",
	}
	results := make([]cpanelCandidateResult, 0, len(candidates))
	best := ""
	for _, u := range candidates {
		r := probeCPanelURL(u)
		results = append(results, r)
		if best == "" && r.Reachable && r.LooksCPanel {
			best = r.URL
		}
	}
	if best == "" {
		for _, r := range results {
			if r.Reachable {
				best = r.URL
				break
			}
		}
	}
	emitJSON(cpanelDiscoveryResp{
		ArtifactType: "hosting.cpanel.discovery.v1",
		Domain:       d,
		Found:        best != "",
		Best:         best,
		Candidates:   results,
	})
}

func normalizeDomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "cpanel.")
	s = strings.TrimPrefix(s, "www.")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		s = s[:idx]
	}
	return strings.ToLower(strings.TrimSpace(s))
}

func probeCPanelURL(u string) cpanelCandidateResult {
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest(http.MethodGet, u+"/login/?login_only=1", nil)
	if err != nil {
		return cpanelCandidateResult{URL: u, Error: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return cpanelCandidateResult{URL: u, Error: err.Error()}
	}
	defer resp.Body.Close()
	looks := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden
	return cpanelCandidateResult{URL: u, Reachable: true, StatusCode: resp.StatusCode, LooksCPanel: looks}
}

func saveSMTPBundle(convID string, bundle credentials.SMTPBundle) error {
	return credentials.SaveSMTP("", convID, bundle)
}

func loadSMTPBundle(convID string) (credentials.SMTPBundle, credentials.SMTPStatus) {
	return credentials.LoadSMTP("", convID)
}

// emitJSON / emitJSONErr son wrappers para mantener formato consistente
// en stdout (parseable por orchestrator) y stderr (legible para humanos).
func emitJSON(v interface{}) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

func emitJSONErr(msg string) {
	emitJSON(provisionSMTPResp{Success: false, Error: msg})
}

// generatePassword produce un password aleatorio de 24 chars base64-url
// válido para cPanel (sin caracteres que UAPI rechace).
func generatePassword() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return "Cb-" + base64.RawURLEncoding.EncodeToString(b)
}

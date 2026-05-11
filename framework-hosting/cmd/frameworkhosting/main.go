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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"framework-hosting/internal/cpanel"
	"framework-hosting/internal/creds"
)

const (
	defaultStateDir = "temp"
	greetingText    = "Soy tu asistente de hosting. Voy a conectar tu cPanel y preparar automáticamente el correo de envío para Remora. Primero decime el host de cPanel o el dominio principal (por ejemplo: cpanel.tudominio.com o tudominio.com)."
)

// state mantiene el progreso conversacional del framework (un saludo + una
// respuesta pendiente). Vive en disco, separado por conversación.
type state struct {
	GreetingAsked bool      `json:"greeting_asked"`
	PendingAnswer string    `json:"pending_answer,omitempty"`
	PendingID     string    `json:"pending_id,omitempty"`
	SetupStep     string    `json:"setup_step,omitempty"`
	Host          string    `json:"host,omitempty"`
	User          string    `json:"user,omitempty"`
	LastAt        time.Time `json:"last_at,omitempty"`
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

// cmdNextQuestion devuelve el saludo inicial UNA vez, luego la respuesta
// pendiente si hay (resultado de un ingest-answer reciente), o {}.
func cmdNextQuestion(args []string) {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	statePath := fs.String("state", "", "path al state (override)")
	_ = fs.Parse(args)

	sp := resolveStatePath(*statePath, *convID)
	s := loadState(sp)

	if s.PendingAnswer != "" {
		out := map[string]string{
			"id":   s.PendingID,
			"text": s.PendingAnswer,
		}
		s.PendingAnswer = ""
		s.PendingID = ""
		_ = saveState(sp, s)
		printJSON(out)
		return
	}

	if !s.GreetingAsked {
		s.GreetingAsked = true
		s.LastAt = time.Now()
		_ = saveState(sp, s)
		printJSON(map[string]string{
			"id":   fmt.Sprintf("hosting_greet_%d", time.Now().Unix()),
			"text": greetingText,
		})
		return
	}

	printJSON(map[string]string{}) // nada pendiente
}

// cmdIngestAnswer procesa input del usuario. Reconoce intents simples:
//   - "conectar <host> <user> <pass>"
//   - "listar correos" / "lista emails" / "list emails"
//
// Cualquier otra cosa devuelve un mensaje de ayuda.
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
	respText := dispatchIntent(strings.TrimSpace(*answer), cp, *convID, s)
	s.PendingAnswer = respText
	s.PendingID = fmt.Sprintf("hosting_resp_%d", time.Now().Unix())
	s.LastAt = time.Now()
	_ = saveState(sp, s)
}

// dispatchIntent es un router super simple basado en prefijos. Para una v2
// usaríamos LLM o reglas más ricas, pero para POC alcanza.
func dispatchIntent(answer, credsPath, convID string, s *state) string {
	low := strings.ToLower(answer)

	switch {
	case strings.HasPrefix(low, "conectar "), strings.HasPrefix(low, "connect "):
		return doConnectFromText(answer, credsPath, convID)
	case strings.Contains(low, "listar correo"),
		strings.Contains(low, "lista emails"),
		strings.Contains(low, "list emails"),
		strings.Contains(low, "list-emails"):
		return doListEmails(credsPath)
	default:
		return handleConnectWizard(answer, credsPath, convID, s)
	}
}

func handleConnectWizard(answer, credsPath, convID string, s *state) string {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "No recibí texto. Decime el host de cPanel o el dominio principal."
	}
	switch s.SetupStep {
	case "":
		s.Host = sanitizeHostAnswer(answer)
		s.SetupStep = "user"
		return "Perfecto. ¿Cuál es el usuario de cPanel?"
	case "user":
		s.User = strings.TrimSpace(answer)
		s.SetupStep = "pass"
		return "Listo. Ahora decime la contraseña de cPanel. No la voy a mostrar ni registrar; la usaré solo para conectar por API y guardar secretos cifrados."
	case "pass":
		pass := answer
		host, user := s.Host, s.User
		s.SetupStep = ""
		s.Host = ""
		s.User = ""
		return doConnect(host, user, pass, credsPath, convID)
	default:
		s.SetupStep = ""
		return "Reinicié el asistente de hosting. Decime el host de cPanel o dominio principal."
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

// doConnectFromText parsea "conectar host user pass" y delega a doConnect.
// Tolerante con espacios extras y variantes en el verbo.
func doConnectFromText(text, credsPath, convID string) string {
	parts := strings.Fields(text)
	if len(parts) < 4 {
		return "Faltan datos. Formato: `conectar <host> <usuario> <password>`"
	}
	host, user, pass := parts[1], parts[2], strings.Join(parts[3:], " ")
	return doConnect(host, user, pass, credsPath, convID)
}

// doConnect prueba auth contra cPanel y, si OK, persiste credenciales.
// Luego descubre automáticamente cuentas de email y auto-configura SMTP.
func doConnect(host, user, pass, credsPath, convID string) string {
	host = sanitizeHostAnswer(host)
	if isPlaceholderHost(host) {
		return "No voy a conectar contra un dominio de ejemplo. Decime el dominio real del negocio o usa el endpoint cPanel descubierto."
	}
	cli, err := cpanel.New(host, user, pass, true)
	if err != nil {
		return fmt.Sprintf("Error de configuración: %v", err)
	}
	if err := cli.Login(); err != nil {
		// Fallback 1: user con @ → probar sin @
		if local := cpanelLocalUserCandidate(user); local != "" {
			retry, retryErr := cpanel.New(host, local, pass, true)
			if retryErr == nil && retry.Login() == nil {
				cli = retry
				user = local
			} else {
				return cpanelLoginErrorHelp(host, user, err)
			}
			// Fallback 2: user sin @ → probar con @dominio
		} else if full := cpanelFullUserCandidate(user, host); full != "" && full != user {
			retry, retryErr := cpanel.New(host, full, pass, true)
			if retryErr == nil && retry.Login() == nil {
				cli = retry
				user = full
			} else {
				return cpanelLoginErrorHelp(host, user, err)
			}
		} else {
			return cpanelLoginErrorHelp(host, user, err)
		}
	}
	if err := cli.Ping(); err != nil {
		return fmt.Sprintf("No pude conectar al hosting: %v", err)
	}
	c := &creds.Credentials{
		Panel: "cpanel", Host: host, Port: 2083,
		User: user, Pass: pass, Insecure: true,
	}
	if err := creds.Save(credsPath, c); err != nil {
		return fmt.Sprintf("Conexión OK pero no pude guardar credenciales: %v.\n"+
			"Verificá que HOSTING_VAULT_KEY esté seteada (corré `frameworkhosting genkey` para generar una).", err)
	}
	emailAddr, smtpHost, err := autoProvisionSMTP(cli, host, convID)
	if err != nil {
		return fmt.Sprintf("Conectado a %s, pero no pude preparar el correo saliente automáticamente: %v", host, err)
	}
	return fmt.Sprintf("Conectado a %s. Preparé automáticamente el correo saliente con cPanel: %s vía %s:587. Ya puedo enviar correos con aprobación.", host, emailAddr, smtpHost)
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
	bundle := map[string]string{
		"host":       smtpHost,
		"port":       "587",
		"user":       emailAddr,
		"pass":       password,
		"from":       emailAddr,
		"default_to": "",
		"source":     "cpanel_auto_provision",
	}
	if err := vaultSet(convID, "credentials.smtp", bundle); err != nil {
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
	fmt.Println(doConnect(*host, *user, *pass, cp, *convID))
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
	bundle := map[string]string{
		"host":       smtpHost,
		"port":       smtpPort,
		"user":       emailAddr,
		"pass":       *password,
		"from":       emailAddr,
		"default_to": *defaultTo,
	}
	if err := vaultSet(*convID, "credentials.smtp", bundle); err != nil {
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
	bundle := map[string]string{
		"host":       *host,
		"port":       *port,
		"user":       *user,
		"pass":       *pass,
		"from":       *from,
		"default_to": *defaultTo,
	}
	if err := vaultSet(*convID, "credentials.smtp", bundle); err != nil {
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

// cmdHasSMTP imprime {"available": bool} consultando el vault.
// Permite al orquestador chequear sin desencriptar.
func cmdHasSMTP(args []string) {
	fs := flag.NewFlagSet("has-smtp", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	_ = fs.Parse(args)
	available := vaultHas(*convID, "credentials.smtp")
	emitJSON(map[string]interface{}{
		"artifact_type": "credentials.status.v1",
		"available":     available,
		"capability":    "credentials.smtp",
	})
}

// cmdDeleteSMTP elimina credentials.smtp del vault.
func cmdDeleteSMTP(args []string) {
	fs := flag.NewFlagSet("delete-smtp", flag.ExitOnError)
	convID := fs.String("conv-id", "", "id de la conversación")
	_ = fs.Parse(args)
	err := vaultDelete(*convID, "credentials.smtp")
	deleted := err == nil
	emitJSON(map[string]interface{}{
		"artifact_type": "credentials.deleted.v1",
		"deleted":       deleted,
		"capability":      "credentials.smtp",
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

// ============================================================
// vault client (shells out to channel/bin/vault)
// ============================================================

func vaultBin() string {
	if v := os.Getenv("REMORA_VAULT_BIN"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	return "../channel/bin/vault"
}

func vaultSet(convID, key string, value interface{}) error {
	conv := strings.TrimSpace(convID)
	if conv == "" {
		conv = "default"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cmd := exec.Command(vaultBin(), "set", "--conv", conv, "--key", key, "--stdin")
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(string(data))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (output=%s)", err, string(out))
	}
	return nil
}

func vaultHas(convID, key string) bool {
	conv := strings.TrimSpace(convID)
	if conv == "" {
		conv = "default"
	}
	cmd := exec.Command(vaultBin(), "has", "--conv", conv, "--key", key)
	cmd.Env = os.Environ()
	return cmd.Run() == nil
}

func vaultDelete(convID, key string) error {
	conv := strings.TrimSpace(convID)
	if conv == "" {
		conv = "default"
	}
	cmd := exec.Command(vaultBin(), "delete", "--conv", conv, "--key", key)
	cmd.Env = os.Environ()
	return cmd.Run()
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

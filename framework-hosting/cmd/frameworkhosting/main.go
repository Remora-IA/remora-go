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
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
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
	greetingText    = "Soy tu asistente de hosting. Conectame con `conectar <host> <usuario> <password>` (ej: `conectar patriciastocker.com tomashigh@patriciastocker.com PASS`) y después podés pedirme `listar correos`."
)

// state mantiene el progreso conversacional del framework (un saludo + una
// respuesta pendiente). Vive en disco, separado por conversación.
type state struct {
	GreetingAsked bool      `json:"greeting_asked"`
	PendingAnswer string    `json:"pending_answer,omitempty"`
	PendingID     string    `json:"pending_id,omitempty"`
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

	respText := dispatchIntent(strings.TrimSpace(*answer), cp)

	s := loadState(sp)
	s.PendingAnswer = respText
	s.PendingID = fmt.Sprintf("hosting_resp_%d", time.Now().Unix())
	s.LastAt = time.Now()
	_ = saveState(sp, s)
}

// dispatchIntent es un router super simple basado en prefijos. Para una v2
// usaríamos LLM o reglas más ricas, pero para POC alcanza.
func dispatchIntent(answer, credsPath string) string {
	low := strings.ToLower(answer)

	switch {
	case strings.HasPrefix(low, "conectar "), strings.HasPrefix(low, "connect "):
		return doConnectFromText(answer, credsPath)
	case strings.Contains(low, "listar correo"),
		strings.Contains(low, "lista emails"),
		strings.Contains(low, "list emails"),
		strings.Contains(low, "list-emails"):
		return doListEmails(credsPath)
	default:
		return "No entendí. Decime:\n" +
			"• `conectar <host> <usuario> <password>` para conectar al panel\n" +
			"• `listar correos` para ver las cuentas de email del dominio"
	}
}

// doConnectFromText parsea "conectar host user pass" y delega a doConnect.
// Tolerante con espacios extras y variantes en el verbo.
func doConnectFromText(text, credsPath string) string {
	parts := strings.Fields(text)
	if len(parts) < 4 {
		return "Faltan datos. Formato: `conectar <host> <usuario> <password>`"
	}
	host, user, pass := parts[1], parts[2], strings.Join(parts[3:], " ")
	return doConnect(host, user, pass, credsPath)
}

// doConnect prueba auth contra cPanel y, si OK, persiste credenciales.
func doConnect(host, user, pass, credsPath string) string {
	cli, err := cpanel.New(host, user, pass, true)
	if err != nil {
		return fmt.Sprintf("Error de configuración: %v", err)
	}
	if err := cli.Login(); err != nil {
		return fmt.Sprintf("No pude conectar al hosting: %v", err)
	}
	if err := cli.Ping(); err != nil {
		return fmt.Sprintf("Login OK pero la sesión no respondió a Ping: %v", err)
	}
	c := &creds.Credentials{
		Panel: "cpanel", Host: host, Port: 2083,
		User: user, Pass: pass, Insecure: true,
	}
	if err := creds.Save(credsPath, c); err != nil {
		return fmt.Sprintf("Conexión OK pero no pude guardar credenciales: %v.\n" +
			"Verificá que HOSTING_VAULT_KEY esté seteada (corré `frameworkhosting genkey` para generar una).", err)
	}
	return fmt.Sprintf("Conectado a %s como %s. Credenciales guardadas. Pedime `listar correos` para ver las cuentas del dominio.", host, user)
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
	fmt.Println(doConnect(*host, *user, *pass, cp))
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
	Success    bool   `json:"success"`
	Email      string `json:"email,omitempty"`
	SMTPHost   string `json:"smtp_host,omitempty"`
	SMTPPort   string `json:"smtp_port,omitempty"`
	Capability string `json:"capability,omitempty"`
	Error      string `json:"error,omitempty"`
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
		Success: true, Email: emailAddr,
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
		Success: true, Email: *user, SMTPHost: *host, SMTPPort: *port,
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
		"available":  available,
		"capability": "credentials.smtp",
	})
}

// ============================================================
// vault client (shells out to channel/bin/vault)
// ============================================================

func vaultBin() string {
	if v := os.Getenv("REMORA_VAULT_BIN"); v != "" {
		return v
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

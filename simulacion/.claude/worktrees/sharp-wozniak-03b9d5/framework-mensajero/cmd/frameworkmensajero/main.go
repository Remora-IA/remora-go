// frameworkmensajero: framework genérico de envío de mensajes salientes.
//
// Filosofía: este binario no sabe nada del negocio. Recibe un draft
// (subject, body, to) y un canal (email/sms/whatsapp/...) y ejecuta el
// envío usando las credenciales canónicas del vault compartido
// (capability "credentials.<channel>"). Si no hay credenciales, devuelve
// un error estructurado para que el orquestador delegue el provisioning.
//
// Estandarización:
//   - El cuerpo del mensaje se pasa como base64 (--body-b64) para evitar
//     problemas con newlines y caracteres especiales en argumentos shell
//     (channel rechaza newlines en args por seguridad — axioma 4.3).
//   - El framework NUNCA loguea credenciales ni el body completo.
//   - Todos los outputs son JSON parseables en stdout.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/credentials"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "next-question":
		cmdNextQuestion(os.Args[2:])
	case "ingest-answer":
		cmdIngestAnswer(os.Args[2:])
	case "can-send":
		cmdCanSend(os.Args[2:])
	case "send":
		cmdSend(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "mensajero: comando desconocido: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`frameworkmensajero — envío genérico de mensajes salientes

Comandos:
  next-question  --conv-id <id>
  ingest-answer  --question-id <id> --answer <text> --conv-id <id>
  can-send       --channel <email|sms|...> --conv-id <id>
  send           --channel <c> --to <addr> --subject <s> --body-b64 <b64> --conv-id <id>

Canales soportados:
  email — usa credentials.smtp (host, port, user, pass, from)

Env:
  REMORA_VAULT_BIN   path al binario vault (default: ../channel/bin/vault)
  REMORA_VAULT_KEY   clave maestra del vault (heredada al binario vault)
  REMORA_VAULT_DIR   override del directorio del vault`)
}

// ============================================================
// Conversational contract (next-question / ingest-answer)
// ============================================================

func cmdNextQuestion(args []string) {
	// Mensajero no inicia conversación. Idle por defecto.
	// Si en el futuro queremos que confirme envíos pendientes acá lo metemos.
	fmt.Println("{}")
}

func cmdIngestAnswer(args []string) {
	// Mensajero no procesa respuestas humanas directamente. Las acciones
	// vienen como comandos `send` invocados por el orquestador.
	// No-op silencioso para no romper el chain.
}

// ============================================================
// can-send: discovery de credenciales
// ============================================================

type canSendResp struct {
	ArtifactType      string `json:"artifact_type,omitempty"`
	Available         bool   `json:"available"`
	Channel           string `json:"channel"`
	Capability        string `json:"capability,omitempty"`
	Present           bool   `json:"present,omitempty"`
	Readable          bool   `json:"readable,omitempty"`
	Complete          bool   `json:"complete,omitempty"`
	MissingCapability string `json:"missing_capability,omitempty"`
	ProviderHint      string `json:"provider_hint,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Scope             any    `json:"scope,omitempty"`
}

func cmdCanSend(args []string) {
	fs := flag.NewFlagSet("can-send", flag.ExitOnError)
	channel := fs.String("channel", "email", "")
	convID := fs.String("conv-id", "", "")
	_ = fs.Parse(args)

	cap := capabilityForChannel(*channel)
	if cap == "" {
		emitJSON(canSendResp{
			ArtifactType: "message.send_readiness.v1",
			Available:    false,
			Channel:      *channel,
			Reason:       fmt.Sprintf("canal desconocido: %s", *channel),
		})
		return
	}
	if cap == "credentials.smtp" {
		_, status := credentials.LoadSMTP("", *convID)
		resp := canSendResp{
			ArtifactType: "message.send_readiness.v1",
			Available:    status.Ready,
			Channel:      *channel,
			Capability:   status.Capability,
			Present:      status.Present,
			Readable:     status.Readable,
			Complete:     status.Complete,
			ProviderHint: providerHintForCapability(cap),
			Reason:       status.Error,
			Scope:        status.Scope,
		}
		if !status.Present {
			resp.MissingCapability = cap
		}
		emitJSON(resp)
		return
	}
	emitJSON(canSendResp{
		ArtifactType:      "message.send_readiness.v1",
		Available:         false,
		Channel:           *channel,
		MissingCapability: cap,
		ProviderHint:      providerHintForCapability(cap),
		Reason:            fmt.Sprintf("falta %s en el vault", cap),
	})
}

// capabilityForChannel mapea canal → capability canónica del vault.
// Tabla extensible: cada canal nuevo agrega una línea acá.
func capabilityForChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "email", "smtp":
		return "credentials.smtp"
	case "sms", "twilio":
		return "credentials.twilio"
	case "whatsapp", "wa":
		return "credentials.whatsapp"
	default:
		return ""
	}
}

// providerHintForCapability sugiere quién suele producir esa capability.
// El orquestador puede usar esto como pista, pero la verdad está en los
// manifests (campo capabilities_semantic.produces).
func providerHintForCapability(cap string) string {
	switch cap {
	case "credentials.smtp", "credentials.imap":
		return "hosting"
	default:
		return ""
	}
}

// ============================================================
// send: ejecuta el envío
// ============================================================

type sendResp struct {
	ArtifactType      string   `json:"artifact_type,omitempty"`
	Artifacts         []string `json:"artifacts,omitempty"`
	Success           bool     `json:"success"`
	Channel           string   `json:"channel"`
	To                string   `json:"to,omitempty"`
	MessageID         string   `json:"message_id,omitempty"`
	Error             string   `json:"error,omitempty"`
	MissingCapability string   `json:"missing_capability,omitempty"`
	ProviderHint      string   `json:"provider_hint,omitempty"`
}

func cmdSend(args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	channel := fs.String("channel", "email", "")
	to := fs.String("to", "", "")
	subject := fs.String("subject", "", "")
	bodyB64 := fs.String("body-b64", "", "body codificado en base64")
	bodyPlain := fs.String("body", "", "body en texto plano (alternativa a --body-b64)")
	convID := fs.String("conv-id", "", "")
	_ = fs.Parse(args)

	body := *bodyPlain
	if *bodyB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(*bodyB64)
		if err != nil {
			// Probar URL-safe como fallback (channel pasa newlines así).
			decoded, err = base64.RawURLEncoding.DecodeString(*bodyB64)
			if err != nil {
				emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: "body-b64 inválido: " + err.Error()})
				os.Exit(1)
			}
		}
		body = string(decoded)
	}
	if body == "" {
		emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: "body vacío"})
		os.Exit(2)
	}

	cap := capabilityForChannel(*channel)
	if cap == "" {
		emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: "canal no soportado: " + *channel})
		os.Exit(2)
	}
	if cap == "credentials.smtp" {
		bundle, status := credentials.LoadSMTP("", *convID)
		if !status.Present {
			emitJSON(sendResp{
				ArtifactType:      "message.sent.v1",
				Channel:           *channel,
				Error:             "credenciales faltantes",
				MissingCapability: cap,
				ProviderHint:      providerHintForCapability(cap),
			})
			os.Exit(3) // exit code reservado: capability missing
		}
		if !status.Readable || !status.Complete {
			emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: status.Error})
			os.Exit(1)
		}
		c := smtpCredsFromBundle(bundle)
		dest := *to
		if dest == "" {
			dest = c.DefaultTo
		}
		if dest == "" {
			emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: "destinatario vacío y no hay default_to en credentials.smtp"})
			os.Exit(2)
		}
		if !strings.Contains(dest, "@") || strings.Contains(dest, "sin destinatario") {
			emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: "destinatario de email faltante o inválido", MissingCapability: "contact.destination.v1", ProviderHint: "sabio"})
			os.Exit(3)
		}
		realDest := dest
		sendSubject := *subject
		if override := strings.TrimSpace(os.Getenv("REMORA_DEV_EMAIL_OVERRIDE")); override != "" {
			dest = override
			sendSubject = subjectWithDevPrefix(sendSubject, realDest)
		}
		msgID := fmt.Sprintf("mensajero-%d@%s", time.Now().UnixNano(), c.Host)
		if err := smtpSend(c, dest, sendSubject, body, msgID); err != nil {
			emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, To: dest, Error: err.Error()})
			os.Exit(1)
		}
		_ = appendSentLog(*convID, *channel, dest, sendSubject, msgID)
		emitJSON(sendResp{ArtifactType: "message.sent.v1", Artifacts: []string{"message.sent.v1", "message.sent"}, Success: true, Channel: *channel, To: dest, MessageID: msgID})
		return
	}
	emitJSON(sendResp{ArtifactType: "message.sent.v1", Channel: *channel, Error: "canal aún no implementado: " + *channel})
	os.Exit(2)
}

func subjectWithDevPrefix(subject, realDest string) string {
	if strings.HasPrefix(subject, "[DEV]") {
		return subject
	}
	if strings.TrimSpace(subject) == "" {
		subject = "(sin asunto)"
	}
	return "[DEV → " + realDest + "] " + subject
}

// ============================================================
// SMTP send
// ============================================================

type smtpCreds struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	User      string `json:"user"`
	Pass      string `json:"pass"`
	From      string `json:"from,omitempty"`
	DefaultTo string `json:"default_to,omitempty"`
}

func smtpCredsFromBundle(bundle credentials.SMTPBundle) smtpCreds {
	return smtpCreds{
		Host:      bundle.Host,
		Port:      bundle.Port,
		User:      bundle.User,
		Pass:      bundle.Pass,
		From:      bundle.From,
		DefaultTo: bundle.DefaultTo,
	}
}

func (c *smtpCreds) applyDefaults() {
	if c.Port == "" {
		c.Port = "587"
	}
	if c.From == "" {
		c.From = c.User
	}
}

func smtpSend(c smtpCreds, to, subject, body, msgID string) error {
	if c.Host == "" || c.User == "" || c.Pass == "" {
		return fmt.Errorf("credentials.smtp incompletas (host/user/pass)")
	}
	addr := net.JoinHostPort(c.Host, c.Port)
	auth := smtp.PlainAuth("", c.User, c.Pass, c.Host)
	msg := []byte(
		"From: " + c.From + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Message-ID: <" + msgID + ">\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
			body,
	)
	return smtp.SendMail(addr, auth, c.From, []string{to}, msg)
}

// appendSentLog deja un audit-trail JSONL por conversación.
func appendSentLog(convID, channel, to, subject, msgID string) error {
	dir := "temp"
	_ = os.MkdirAll(dir, 0700)
	safe := sanitize(convID)
	if safe == "" {
		safe = "default"
	}
	path := filepath.Join(dir, "sent_"+safe+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	rec := map[string]string{
		"channel":    channel,
		"to":         to,
		"subject":    subject,
		"message_id": msgID,
		"sent_at":    time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.Marshal(rec)
	_, err = f.Write(append(b, '\n'))
	return err
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= 'A' && ch <= 'Z',
			ch >= '0' && ch <= '9',
			ch == '_', ch == '-':
			out = append(out, ch)
		}
	}
	return string(out)
}

// ============================================================
// helpers
// ============================================================

func emitJSON(v interface{}) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

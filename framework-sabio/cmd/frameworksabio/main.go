// frameworksabio: experto en datos declarados por framework-indexa.
// Responde preguntas del usuario usando SQLite como fuente de verdad.
//
// Comandos (contrato del orquestador api_rest):
//
//	./frameworksabio next-question
//	    Devuelve {"id":"...","text":"..."} con la próxima cosa que mostrar
//	    al usuario, o {} si no hay nada pendiente.
//
//	./frameworksabio ingest-answer --question-id <id> --answer <text>
//	    Toma `answer` como pregunta del usuario, consulta SQLite, y
//	    guarda la respuesta para que la próxima next-question la entregue.
//
//	./frameworksabio query --question <text>
//	    Modo CLI directo (sin estado, para testing). Imprime la respuesta.
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"channel/profile"
	"framework-sabio/internal/llm"
	"framework-sabio/internal/sqlqa"
	_ "modernc.org/sqlite"
)

// HistoryTurn es un turno de la conversación pasado por el orquestador.
// Llega serializado como JSON dentro de --history (base64-url-safe para
// pasar el axioma de seguridad del Channel que rechaza newlines y
// metacaracteres de shell).
type HistoryTurn struct {
	Role    string `json:"role"`    // "user" | "framework"
	Content string `json:"content"` // texto del turno
}

type sabioAnswer struct {
	Text  string     `json:"text"`
	Trace sabioTrace `json:"trace"`
}

type sabioTrace struct {
	Capability          string   `json:"capability"`
	Source              string   `json:"source"`
	SQL                 string   `json:"sql,omitempty"`
	Tables              []string `json:"tables,omitempty"`
	RowCount            int      `json:"row_count,omitempty"`
	FallbackUsed        bool     `json:"fallback_used"`
	MissingCapabilities []string `json:"missing_capabilities,omitempty"`
	Error               string   `json:"error,omitempty"`
}

const (
	defaultStorePath = "../framework-indexa/data/store.json"
	defaultDBPath    = ""
	defaultStatePath = "temp/state.json"
	defaultCatalog   = "semantic/catalog.json"
	defaultViewsSQL  = "semantic/views.sql"
	defaultBusiness  = ""
	greetingText     = "Soy el experto en tus datos indexados. Decime qué querés saber (ej: \"cuántos clientes tengo\", \"detalle de proyectos activos\", \"pagos pendientes\")."
)

type businessPack struct {
	Version            int                         `json:"version"`
	BusinessID         string                      `json:"business_id"`
	Name               string                      `json:"name"`
	Domain             string                      `json:"domain"`
	Why                string                      `json:"why"`
	DefaultAudience    string                      `json:"default_audience"`
	DefaultMode        string                      `json:"default_mode"`
	DataSource         map[string]string           `json:"data_source"`
	Audiences          map[string]businessAudience `json:"audiences"`
	PrimaryEntities    map[string]businessEntity   `json:"primary_entities"`
	JobsToBeDone       []string                    `json:"jobs_to_be_done"`
	SuggestedQuestions map[string][]string         `json:"suggested_questions"`
	AnswerPolicies     map[string]string           `json:"answer_policies"`
	ForbiddenClaims    []string                    `json:"forbidden_claims"`
	CanonicalViews     []string                    `json:"canonical_views"`
	ScopePolicies      businessScopePolicies       `json:"scope_policies"`
}

type businessAudience struct {
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	AllowedModes []string `json:"allowed_modes"`
	CanSeeSQL    bool     `json:"can_see_sql"`
	AnswerStyle  string   `json:"answer_style"`
}

type runtimeContext struct {
	BusinessID   string         `json:"business_id"`
	RemoraUserID string         `json:"remora_user_id"`
	Audience     string         `json:"audience"`
	Scope        map[string]any `json:"scope"`
	ActiveEntity map[string]any `json:"active_entity"`
}

type businessEntity struct {
	Table         string `json:"table"`
	Label         string `json:"label"`
	ScopeKey      string `json:"scope_key"`
	ScopeColumn   string `json:"scope_column"`
	DisplayColumn string `json:"display_column"`
}

type businessScopePolicies struct {
	ScopeEntity string                        `json:"scope_entity"`
	Tables      map[string]businessTableScope `json:"tables"`
}

type businessTableScope struct {
	ScopeColumn string `json:"scope_column"`
	JoinToScope string `json:"join_to_scope"`
}

type dbProfile struct {
	Source        string                    `json:"source"`
	TableCount    int                       `json:"table_count"`
	Tables        map[string]dbProfileTable `json:"tables"`
	Relationships []dbRelationship          `json:"relationships,omitempty"`
}

type dbProfileTable struct {
	RowCount   int               `json:"row_count"`
	Columns    []dbProfileColumn `json:"columns"`
	SampleRows []map[string]any  `json:"sample_rows,omitempty"`
}

type dbProfileColumn struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	NotNull bool   `json:"notnull"`
	PK      bool   `json:"pk"`
}

type dbRelationship struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Confidence string `json:"confidence"`
	Basis      string `json:"basis"`
}

// profileLoader carga el perfil activo (Forma Genérica o Forma <nombre>)
// Usa env REMORA_PROFILE_PATH para sobreescribir la ubicación (útil en Docker)
var profileLoader = profile.NewLoader(envOr("REMORA_PROFILE_PATH", "../profiles"))
var activeProfile *profile.Profile

// getProfile carga y cachea el perfil activo
func getProfile() *profile.Profile {
	if activeProfile == nil {
		var err error
		activeProfile, err = profileLoader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sabio] profile load error: %v\n", err)
			activeProfile = &profile.Profile{Name: "generic", Overlays: map[string]string{}}
		}
	}
	return activeProfile
}

// systemPromptWithOverlay aplica el overlay del perfil al system prompt base
func systemPromptWithOverlay(baseSystem, framework string) string {
	return getProfile().SystemPromptWithOverlay(baseSystem, framework)
}

// state mantiene la conversación pendiente de Sabio entre invocaciones CLI.
// Como el orquestador llama al binario una vez por paso, el estado vive en
// disco. Para MVP usamos un único state global (un solo "expert chat").
type state struct {
	GreetingAsked bool      `json:"greeting_asked"`
	PendingAnswer string    `json:"pending_answer"` // respuesta lista para entregar al user
	PendingID     string    `json:"pending_id"`     // id sintético del próximo "next-question"
	LastQuestion  string    `json:"last_question"`  // última pregunta del user (debug)
	LastAnswerAt  time.Time `json:"last_answer_at"`
}

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
	case "query":
		cmdQuery(os.Args[2:])
	case "explain-capabilities":
		cmdExplainCapabilities(os.Args[2:])
	case "inspect-source":
		cmdInspectSource(os.Args[2:])
	case "validate-business-config":
		cmdValidateBusinessConfig(os.Args[2:])
	case "contact-lookup", "lookup":
		cmdContactLookup(os.Args[2:])
	case "contact-store", "store":
		cmdContactStore(os.Args[2:])
	case "contact-list-missing", "list-missing":
		cmdContactListMissing(os.Args[2:])
	case "contact-import-csv", "import-csv":
		cmdContactImportCSV(os.Args[2:])
	case "contact-init", "init":
		cmdContactInit(os.Args[2:])
	case "evaluate":
		cmdEvaluate(os.Args[2:])
	case "dataset-export":
		cmdDatasetExport(os.Args[2:])
	case "reset":
		cmdReset(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`frameworksabio: Q&A sobre datos indexados por framework-indexa

Uso:
  frameworksabio next-question
  frameworksabio ingest-answer --question-id <id> --answer <text>
  frameworksabio query --question <text> [--business-id <id>] [--context-b64 <json-b64>]
  frameworksabio explain-capabilities [--business-id <id>] [--context-b64 <json-b64>]
  frameworksabio inspect-source [--db <path>] [--out <path>]
  frameworksabio validate-business-config [--business-id <id>]
  frameworksabio contact-lookup --entity-type <type> --entity-ref <ref> [--channel email]
  frameworksabio contact-store --entity-type <type> --entity-ref <ref> --value <value> [--channel email]
  frameworksabio contact-list-missing [--entity-type client] [--channel email]
  frameworksabio contact-import-csv --file <path>
  frameworksabio contact-init
  frameworksabio evaluate [--fixture <path>]
  frameworksabio dataset-export --db <path> [--semantic-pack <path>] [--out <path>]
  frameworksabio reset

Variables de entorno:
  GROQ_API_KEY        requerida si la pregunta necesita Text-to-SQL con LLM
  SABIO_DB            override del path de SQLite
  SABIO_STATE         override del path del state
  SABIO_SEMANTIC_CATALOG  override del catálogo semántico JSON
  SABIO_SEMANTIC_VIEWS    override de las vistas semánticas SQL
  SABIO_BUSINESS_ID        negocio activo
  SABIO_CONTEXT_B64        contexto runtime de sesión en JSON base64-url-safe
  SABIO_CONTACTS_DB_PATH   override del SQLite de contactos
`)
}

func resolveContactDBPath(profileName string) string {
	if v := envOr("SABIO_CONTACTS_DB_PATH", ""); v != "" {
		return v
	}
	if v := envOr("CONTACTS_DB_PATH", ""); v != "" {
		return v
	}
	if profileName == "" {
		profileName = envOr("REMORA_PROFILE", "default")
	}
	if base := envOr("REMORA_PROFILE_PATH", ""); base != "" {
		return filepath.Join(base, profileName, "contacts.db")
	}
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "."))
	return filepath.Join(root, "profiles", profileName, "contacts.db")
}

const contactsSchemaSQL = `
CREATE TABLE IF NOT EXISTS contacts (
  entity_type  TEXT NOT NULL,
  entity_ref   TEXT NOT NULL,
  channel      TEXT NOT NULL,
  value        TEXT NOT NULL,
  source       TEXT NOT NULL DEFAULT 'manual',
  verified_at  TEXT,
  created_at   TEXT NOT NULL,
  PRIMARY KEY (entity_type, entity_ref, channel, value)
);
CREATE INDEX IF NOT EXISTS contacts_lookup
  ON contacts(entity_type, entity_ref, channel);
`

func openContactDB(profileName string) (*sql.DB, error) {
	path := resolveContactDBPath(profileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir contacts dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(contactsSchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply contacts schema: %w", err)
	}
	return db, nil
}

func cmdContactInit(args []string) {
	fs := flag.NewFlagSet("contact-init", flag.ExitOnError)
	profileName := fs.String("profile", "", "perfil")
	_ = fs.Parse(args)
	db, err := openContactDB(*profileName)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()
	emitJSON(map[string]interface{}{"success": true, "db_path": resolveContactDBPath(*profileName)})
}

func cmdContactLookup(args []string) {
	fs := flag.NewFlagSet("contact-lookup", flag.ExitOnError)
	profileName := fs.String("profile", "", "perfil")
	entityType := fs.String("entity-type", "", "ej: client, provider")
	entityRef := fs.String("entity-ref", "", "id o code del ERP")
	channel := fs.String("channel", "email", "email|phone|whatsapp|...")
	_ = fs.Parse(args)
	if *entityType == "" || *entityRef == "" {
		emitJSON(map[string]interface{}{
			"artifact_type":      "contact.lookup.v1",
			"found":              false,
			"missing_capability": "contact." + *channel,
			"provider_hint":      "sabio",
			"error":              "entity_type y entity_ref son requeridos",
		})
		return
	}
	db, err := openContactDB(*profileName)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	row := db.QueryRow(`
		SELECT value, source, COALESCE(verified_at, '') FROM contacts
		WHERE entity_type = ? AND entity_ref = ? AND channel = ?
		ORDER BY (verified_at IS NULL), verified_at DESC, created_at DESC
		LIMIT 1`, *entityType, *entityRef, *channel)
	var value, source, verifiedAt string
	if err := row.Scan(&value, &source, &verifiedAt); err != nil {
		if err == sql.ErrNoRows {
			emitJSON(map[string]interface{}{
				"artifact_type":      "contact.lookup.v1",
				"found":              false,
				"missing_capability": "contact." + *channel,
				"provider_hint":      "sabio",
				"entity_type":        *entityType,
				"entity_ref":         *entityRef,
				"channel":            *channel,
			})
			return
		}
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	emitJSON(map[string]interface{}{
		"artifact_type": "contact.destination.v1",
		"artifacts":     []string{"contact.lookup.v1", "contact.destination.v1"},
		"found":         true,
		"value":         value,
		"destination":   value,
		"source":        source,
		"verified_at":   verifiedAt,
		"entity_type":   *entityType,
		"entity_ref":    *entityRef,
		"channel":       *channel,
	})
}

func cmdContactStore(args []string) {
	fs := flag.NewFlagSet("contact-store", flag.ExitOnError)
	profileName := fs.String("profile", "", "perfil")
	entityType := fs.String("entity-type", "", "")
	entityRef := fs.String("entity-ref", "", "")
	channel := fs.String("channel", "email", "")
	value := fs.String("value", "", "valor (ej. email)")
	source := fs.String("source", "manual", "manual|csv|erp|scraped")
	verified := fs.Bool("verified", false, "marcar como verificado ahora")
	_ = fs.Parse(args)
	if *entityType == "" || *entityRef == "" || *value == "" {
		emitJSON(map[string]interface{}{"success": false, "error": "entity_type, entity_ref y value son requeridos"})
		os.Exit(1)
	}
	db, err := openContactDB(*profileName)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	verifiedAt := ""
	if *verified {
		verifiedAt = now
	}
	_, err = db.Exec(`
		INSERT INTO contacts(entity_type, entity_ref, channel, value, source, verified_at, created_at)
		VALUES(?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		ON CONFLICT(entity_type, entity_ref, channel, value) DO UPDATE SET
			source       = excluded.source,
			verified_at  = COALESCE(excluded.verified_at, contacts.verified_at)
	`, *entityType, *entityRef, *channel, *value, *source, verifiedAt, now)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	emitJSON(map[string]interface{}{
		"artifact_type": "contact.record.v1",
		"success":       true,
		"entity_type":   *entityType,
		"entity_ref":    *entityRef,
		"channel":       *channel,
		"value":         *value,
		"source":        *source,
	})
}

func cmdContactListMissing(args []string) {
	fs := flag.NewFlagSet("contact-list-missing", flag.ExitOnError)
	profileName := fs.String("profile", "", "perfil")
	entityType := fs.String("entity-type", "client", "")
	channel := fs.String("channel", "email", "")
	_ = fs.Parse(args)

	entityDB := envOr("SABIO_CONTACTS_ENTITY_DB", envOr("CONTACTS_ENTITY_DB", ""))
	entityQuery := envOr("SABIO_CONTACTS_ENTITY_QUERY", envOr("CONTACTS_ENTITY_QUERY", ""))
	if entityDB == "" {
		emitJSON(map[string]interface{}{
			"artifact_type": "contact.missing_report.v1",
			"missing":       []interface{}{},
			"warning":       "no se configuró SABIO_CONTACTS_ENTITY_DB",
		})
		return
	}
	if entityQuery == "" {
		entityQuery = fmt.Sprintf("SELECT id, name FROM %ss ORDER BY name LIMIT 500", *entityType)
	}

	edb, err := sql.Open("sqlite", "file:"+entityDB+"?mode=ro&_pragma=query_only(true)")
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": "open entity db: " + err.Error()})
		os.Exit(1)
	}
	defer edb.Close()
	rows, err := edb.Query(entityQuery)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": "entity query: " + err.Error()})
		os.Exit(1)
	}
	defer rows.Close()

	type ent struct {
		Ref  string `json:"entity_ref"`
		Name string `json:"name"`
	}
	all := []ent{}
	for rows.Next() {
		var id, name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		all = append(all, ent{Ref: id.String, Name: name.String})
	}

	cdb, err := openContactDB(*profileName)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer cdb.Close()

	have := map[string]bool{}
	hr, err := cdb.Query(`SELECT DISTINCT entity_ref FROM contacts WHERE entity_type = ? AND channel = ?`, *entityType, *channel)
	if err == nil {
		for hr.Next() {
			var r string
			if hr.Scan(&r) == nil {
				have[r] = true
			}
		}
		hr.Close()
	}

	missing := []ent{}
	for _, e := range all {
		if !have[e.Ref] {
			missing = append(missing, e)
		}
	}
	emitJSON(map[string]interface{}{
		"artifact_type": "contact.missing_report.v1",
		"entity_type":   *entityType,
		"channel":       *channel,
		"total":         len(all),
		"with":          len(all) - len(missing),
		"missing":       missing,
	})
}

func cmdContactImportCSV(args []string) {
	fs := flag.NewFlagSet("contact-import-csv", flag.ExitOnError)
	profileName := fs.String("profile", "", "perfil")
	file := fs.String("file", "", "ruta al CSV con headers entity_type,entity_ref,channel,value[,source]")
	_ = fs.Parse(args)
	if *file == "" {
		emitJSON(map[string]interface{}{"success": false, "error": "--file requerido"})
		os.Exit(1)
	}
	f, err := os.Open(*file)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": "csv vacío o inválido: " + err.Error()})
		os.Exit(1)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, k := range []string{"entity_type", "entity_ref", "channel", "value"} {
		if _, ok := col[k]; !ok {
			emitJSON(map[string]interface{}{"success": false, "error": "CSV falta columna: " + k})
			os.Exit(1)
		}
	}

	db, err := openContactDB(*profileName)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	imported, skipped := 0, 0
	errs := []string{}
	now := time.Now().UTC().Format(time.RFC3339)
	stmt, err := db.Prepare(`
		INSERT INTO contacts(entity_type, entity_ref, channel, value, source, created_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_ref, channel, value) DO UPDATE SET source = excluded.source
	`)
	if err != nil {
		emitJSON(map[string]interface{}{"success": false, "error": err.Error()})
		os.Exit(1)
	}
	defer stmt.Close()

	rowNum := 1
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		rowNum++
		get := func(k string) string {
			i, ok := col[k]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		et, er, ch, val := get("entity_type"), get("entity_ref"), get("channel"), get("value")
		src := get("source")
		if src == "" {
			src = "csv"
		}
		if et == "" || er == "" || ch == "" || val == "" {
			skipped++
			continue
		}
		if _, err := stmt.Exec(et, er, ch, val, src, now); err != nil {
			errs = append(errs, fmt.Sprintf("fila %d: %v", rowNum, err))
			skipped++
			continue
		}
		imported++
	}
	emitJSON(map[string]interface{}{
		"artifact_type": "contact.import_report.v1",
		"success":       true,
		"imported":      imported,
		"skipped":       skipped,
		"errors":        errs,
	})
}

func resolvePath(flagVal, envKey, def string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return def
}

func loadState(path string) *state {
	s := &state{}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	return s
}

func saveState(path string, s *state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// cmdNextQuestion: devuelve la próxima cosa que mostrar al user.
//   - Si hay PendingAnswer (acabamos de generar respuesta) → la mostramos
//     como si fuera "pregunta" del bot y la limpiamos.
//   - Si no, y aún no saludamos → saludo inicial.
//   - Si no, {} (nada pendiente).
func cmdNextQuestion(args []string) {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	statePath := fs.String("state", "", "path del state")
	fs.Parse(args)
	sp := resolvePath(*statePath, "SABIO_STATE", defaultStatePath)

	s := loadState(sp)

	if s.PendingAnswer != "" {
		out := map[string]string{
			"id":   s.PendingID,
			"text": s.PendingAnswer,
		}
		// Limpiamos: ya entregamos esta respuesta.
		s.PendingAnswer = ""
		s.PendingID = ""
		_ = saveState(sp, s)
		emitJSON(out)
		return
	}

	if !s.GreetingAsked {
		id := fmt.Sprintf("sabio_greeting_%d", time.Now().Unix())
		s.GreetingAsked = true
		_ = saveState(sp, s)
		emitJSON(map[string]string{"id": id, "text": greetingText})
		return
	}

	// Nada pendiente.
	fmt.Println("{}")
}

// cmdIngestAnswer: toma el answer como query y deja la respuesta en el state
// para que la próxima next-question la entregue.
func cmdIngestAnswer(args []string) {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	questionID := fs.String("question-id", "", "id de la pregunta")
	answer := fs.String("answer", "", "respuesta del usuario (=su pregunta para Sabio)")
	statePath := fs.String("state", "", "path del state")
	storePath := fs.String("store", "", "path del store")
	historyB64 := fs.String("history", "", "historial conversacional (base64-url-safe de JSON [{role,content},...])")
	businessID := fs.String("business-id", "", "negocio activo")
	contextB64 := fs.String("context-b64", "", "contexto runtime de sesión (base64-url-safe JSON)")
	fs.Parse(args)

	if *answer == "" {
		fail("ingest-answer: --answer requerido")
	}
	sp := resolvePath(*statePath, "SABIO_STATE", defaultStatePath)
	stp := resolvePath(*storePath, "SABIO_STORE", defaultStorePath)

	history := decodeHistory(*historyB64)
	rt := loadRuntimeContext(*businessID, *contextB64)
	respText, err := answerQuestionWithRuntime(*answer, stp, history, rt)
	if err != nil {
		// No queremos romper el chain por un error de Sabio. Dejamos un
		// mensaje legible como "respuesta".
		respText = fmt.Sprintf("Sabio no pudo responder esta vez: %v", err)
	}

	s := loadState(sp)
	s.LastQuestion = *answer
	s.PendingAnswer = respText
	s.PendingID = fmt.Sprintf("sabio_answer_%d", time.Now().Unix())
	s.LastAnswerAt = time.Now()
	if err := saveState(sp, s); err != nil {
		fail("save state: %v", err)
	}
	_ = *questionID
}

// cmdQuery: modo directo CLI (sin state). Útil para tests.
func cmdQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	question := fs.String("question", "", "pregunta")
	storePath := fs.String("store", "", "path del store")
	dbPath := fs.String("db", "", "path de SQLite")
	businessID := fs.String("business-id", "", "negocio activo")
	contextB64 := fs.String("context-b64", "", "contexto runtime de sesión (base64-url-safe JSON)")
	capability := fs.String("capability", "", "capability/artifact esperado")
	semanticCapability := fs.String("semantic-capability", "", "capability semántica original pedida por el owner")
	entityRef := fs.String("entity-ref", "", "entidad activa")
	entityType := fs.String("entity-type", "", "tipo de entidad activa")
	analysisIntent := fs.String("analysis-intent", "", "intención analítica solicitada por Radar")
	metricsCSV := fs.String("metrics", "", "métricas solicitadas por Radar, separadas por coma")
	peerStrategy := fs.String("peer-strategy", "", "estrategia de cohorte/pares")
	fs.Parse(args)
	if *question == "" {
		fail("query: --question requerido")
	}
	stp := resolvePath(*storePath, "SABIO_STORE", defaultStorePath)
	if *dbPath != "" {
		_ = os.Setenv("SABIO_DB", *dbPath)
	}
	rt := loadRuntimeContext(*businessID, *contextB64)
	if strings.TrimSpace(*entityRef) != "" {
		rt.ActiveEntity = map[string]any{
			"id":   strings.TrimSpace(*entityRef),
			"type": canonicalEntityType(*entityType),
		}
	}
	if strings.TrimSpace(*capability) == "data.entity_360" {
		emitJSON(runEntity360Artifact(resolvePath(*dbPath, "SABIO_DB", defaultDBPath), rt, *question, *entityType, *entityRef, *analysisIntent))
		return
	}
	if analytical := runAnalyticalQueryArtifact(resolvePath(*dbPath, "SABIO_DB", defaultDBPath), rt, *question, *analysisIntent, *semanticCapability, *entityType, *entityRef, splitCSV(*metricsCSV), *peerStrategy); analytical != nil {
		emitJSON(analytical)
		return
	}
	resp, err := answerQuestionWithRuntime(*question, stp, nil, rt)
	if err != nil {
		fail("query: %v", err)
	}
	emitJSON(sabioQueryArtifact(*question, resp, rt, *capability))
}

func cmdExplainCapabilities(args []string) {
	fs := flag.NewFlagSet("explain-capabilities", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	contextB64 := fs.String("context-b64", "", "contexto runtime de sesión (base64-url-safe JSON)")
	fs.Parse(args)

	rt := loadRuntimeContext(*businessID, *contextB64)
	pack, err := loadBusinessPack(rt.BusinessID)
	if err != nil {
		fail("explain-capabilities: %v", err)
	}
	emitJSON(buildCapabilitiesExplanation(pack, rt))
}

func cmdInspectSource(args []string) {
	fs := flag.NewFlagSet("inspect-source", flag.ExitOnError)
	dbPathFlag := fs.String("db", "", "path de SQLite")
	outPathFlag := fs.String("out", "", "path de profile.json")
	fs.Parse(args)

	dbPath := resolvePath(*dbPathFlag, "SABIO_DB", defaultDBPath)
	outPath := *outPathFlag
	if outPath == "" {
		outPath = "semantic/profile.json"
	}
	profile, err := buildDBProfile(dbPath)
	if err != nil {
		fail("inspect-source: %v", err)
	}
	if err := writeJSONFile(outPath, profile); err != nil {
		fail("inspect-source write: %v", err)
	}
	emitJSON(map[string]any{
		"ok":            true,
		"profile_path":  outPath,
		"table_count":   profile.TableCount,
		"relationships": len(profile.Relationships),
	})
}

func cmdValidateBusinessConfig(args []string) {
	fs := flag.NewFlagSet("validate-business-config", flag.ExitOnError)
	businessID := fs.String("business-id", "", "negocio activo")
	dbPathFlag := fs.String("db", "", "path de SQLite")
	fs.Parse(args)

	bid := *businessID
	if bid == "" {
		bid = envOr("SABIO_BUSINESS_ID", defaultBusiness)
	}
	dbPath := resolvePath(*dbPathFlag, "SABIO_DB", defaultDBPath)
	result := validateBusinessConfig(bid, dbPath)
	emitJSON(result)
	if ok, _ := result["ok"].(bool); !ok {
		os.Exit(1)
	}
}

// ---------- evaluate ----------

const defaultFixturePath = "../../data/qa_cobranza_chile_ideal.json"

type evalFixture struct {
	Version       int        `json:"version"`
	Framework     string     `json:"framework"`
	Profile       string     `json:"profile"`
	DB            string     `json:"db"`
	Conversations []evalConv `json:"conversations"`
}

type evalConv struct {
	ID    string     `json:"id"`
	Title string     `json:"title"`
	Turns []evalTurn `json:"turns"`
}

type evalTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type evalResult struct {
	ConversationID string           `json:"conversation_id"`
	Title          string           `json:"title"`
	Turns          []evalTurnResult `json:"turns"`
	AvgScore       int              `json:"avg_score"`
}

type evalTurnResult struct {
	UserQuestion   string `json:"user_question"`
	IdealResponse  string `json:"ideal_response"`
	ActualResponse string `json:"actual_response"`
	Score          int    `json:"score"`
	Reason         string `json:"reason"`
}

type evalSummary struct {
	Timestamp     string       `json:"timestamp"`
	FixturePath   string       `json:"fixture_path"`
	Conversations []evalResult `json:"conversations"`
	OverallScore  int          `json:"overall_score"`
}

func cmdEvaluate(args []string) {
	fs := flag.NewFlagSet("evaluate", flag.ExitOnError)
	fixturePath := fs.String("fixture", "", "path al JSON de conversaciones ideales")
	storePath := fs.String("store", "", "path del store (ignorado, para compat)")
	fs.Parse(args)

	fp := *fixturePath
	if fp == "" {
		fp = defaultFixturePath
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		fail("evaluate: no pude leer fixture %s: %v", fp, err)
	}
	var fixture evalFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		fail("evaluate: fixture inválido: %v", err)
	}
	if len(fixture.Conversations) == 0 {
		fail("evaluate: fixture sin conversaciones")
	}

	llmCli, err := llm.NewClient()
	if err != nil {
		fail("evaluate: LLM client requerido para scoring: %v", err)
	}

	_ = *storePath
	stp := resolvePath("", "SABIO_STORE", defaultStorePath)

	fmt.Fprintf(os.Stderr, "[evaluate] %d conversaciones en fixture\n", len(fixture.Conversations))

	var results []evalResult
	totalScore := 0
	totalTurns := 0

	for _, conv := range fixture.Conversations {
		fmt.Fprintf(os.Stderr, "[evaluate] === %s: %s ===\n", conv.ID, conv.Title)

		var history []HistoryTurn
		var turnResults []evalTurnResult
		convScore := 0
		convTurns := 0

		for i := 0; i < len(conv.Turns)-1; i += 2 {
			userTurn := conv.Turns[i]
			idealTurn := conv.Turns[i+1]

			if userTurn.Role != "user" || idealTurn.Role != "ideal" {
				fmt.Fprintf(os.Stderr, "[evaluate] skip: par inesperado roles=%s/%s\n", userTurn.Role, idealTurn.Role)
				continue
			}

			fmt.Fprintf(os.Stderr, "[evaluate]   Q: %s\n", userTurn.Content)

			actual, err := answerQuestion(userTurn.Content, stp, history)
			if err != nil {
				actual = fmt.Sprintf("[error] %v", err)
			}

			// Strip the Evidencia trace block for cleaner comparison
			actualClean := stripEvidencia(actual)

			score, reason := scoreResponse(llmCli, userTurn.Content, idealTurn.Content, actualClean)
			fmt.Fprintf(os.Stderr, "[evaluate]   Score: %d — %s\n", score, reason)

			turnResults = append(turnResults, evalTurnResult{
				UserQuestion:   userTurn.Content,
				IdealResponse:  idealTurn.Content,
				ActualResponse: actualClean,
				Score:          score,
				Reason:         reason,
			})

			convScore += score
			convTurns++

			// Build history for multi-turn conversations
			history = append(history,
				HistoryTurn{Role: "user", Content: userTurn.Content},
				HistoryTurn{Role: "framework", Content: actualClean},
			)
		}

		avg := 0
		if convTurns > 0 {
			avg = convScore / convTurns
		}
		results = append(results, evalResult{
			ConversationID: conv.ID,
			Title:          conv.Title,
			Turns:          turnResults,
			AvgScore:       avg,
		})
		totalScore += convScore
		totalTurns += convTurns
	}

	overall := 0
	if totalTurns > 0 {
		overall = totalScore / totalTurns
	}

	summary := evalSummary{
		Timestamp:     time.Now().Format(time.RFC3339),
		FixturePath:   fp,
		Conversations: results,
		OverallScore:  overall,
	}

	out, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(out))

	fmt.Fprintf(os.Stderr, "\n[evaluate] OVERALL SCORE: %d/100 (%d turnos evaluados)\n", overall, totalTurns)
}

func scoreResponse(c *llm.Client, question, ideal, actual string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	system := `Sos un evaluador de calidad de respuestas. Comparás una respuesta real contra una respuesta ideal para la misma pregunta.

Tu trabajo:
1. Dar un score de 0 a 100 indicando qué tan bien la respuesta real cumple con lo que pide la ideal.
2. Dar una razón corta (1-2 frases) del score.

Criterios:
- 90-100: Los datos clave coinciden (números, nombres, hechos). Puede variar el estilo o formato.
- 70-89: La mayoría de datos clave están, pero faltan algunos o hay imprecisiones menores.
- 50-69: Tiene la idea general correcta pero le faltan datos importantes o incluye información incorrecta.
- 30-49: Parcialmente relevante pero con errores importantes o datos inventados.
- 0-29: Incorrecto, irrelevante, o fallback/error.

Importante:
- No penalices diferencias de formato (tabla vs lista vs texto corrido).
- No penalices diferencias de estilo o tono.
- SÍ penalizá datos numéricos incorrectos, nombres equivocados, o información inventada.
- SÍ penalizá si la respuesta real da consejos operativos no solicitados cuando la ideal no los incluye.
- SÍ penalizá si la respuesta real usa fallback o dice que no puede responder cuando la ideal sí responde.

Respondé SOLO en este formato JSON exacto (sin markdown, sin backticks):
{"score": N, "reason": "..."}

Nada más.`

	prompt := fmt.Sprintf("Pregunta del usuario:\n%s\n\nRespuesta IDEAL:\n%s\n\nRespuesta REAL:\n%s", question, ideal, actual)

	resp, err := c.Generate(ctx, system, prompt)
	if err != nil {
		return 0, fmt.Sprintf("llm error: %v", err)
	}

	resp = strings.TrimSpace(resp)
	// Clean potential markdown fences
	if strings.HasPrefix(resp, "```") {
		if idx := strings.Index(resp, "\n"); idx >= 0 {
			resp = resp[idx+1:]
		}
		resp = strings.TrimSuffix(strings.TrimSpace(resp), "```")
		resp = strings.TrimSpace(resp)
	}

	var result struct {
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return 0, fmt.Sprintf("parse error: %v (raw: %s)", err, resp)
	}
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}
	return result.Score, result.Reason
}

func stripEvidencia(s string) string {
	idx := strings.Index(s, "\n\nEvidencia:\n")
	if idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func cmdReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	statePath := fs.String("state", "", "path del state")
	fs.Parse(args)
	sp := resolvePath(*statePath, "SABIO_STATE", defaultStatePath)
	_ = os.Remove(sp)
	fmt.Println("state limpiado")
}

func cmdDatasetExport(args []string) {
	fs := flag.NewFlagSet("dataset-export", flag.ExitOnError)
	dbPath := fs.String("db", "", "path SQLite del negocio")
	semanticPath := fs.String("semantic-pack", "", "path semantic pack (opcional, para filtrar tablas)")
	outPath := fs.String("out", "", "path de salida (opcional)")
	fs.Parse(args)

	resolvedDB := resolvePath(*dbPath, "SABIO_DB", defaultDBPath)
	if resolvedDB == "" {
		emitJSON(map[string]interface{}{
			"artifact_type": "dataset.raw.v1",
			"error":         "falta --db o SABIO_DB",
		})
		return
	}

	db, err := sql.Open("sqlite", "file:"+resolvedDB+"?mode=ro&_pragma=query_only(true)")
	if err != nil {
		emitJSON(map[string]interface{}{
			"artifact_type": "dataset.raw.v1",
			"error":         err.Error(),
		})
		return
	}
	defer db.Close()

	tables, err := datasetExportTables(db, *semanticPath)
	if err != nil {
		emitJSON(map[string]interface{}{
			"artifact_type": "dataset.raw.v1",
			"error":         err.Error(),
		})
		return
	}

	result := map[string]interface{}{
		"artifact_type":   "dataset.raw.v1",
		"artifacts":       []string{"dataset.raw.v1", "external.api.dump.v1"},
		"source_db":       resolvedDB,
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"tables":          tables,
		"table_count":     len(tables),
		"total_row_count": 0,
	}

	totalRows := 0
	for _, rows := range tables {
		totalRows += len(rows)
	}
	result["total_row_count"] = totalRows

	if *outPath != "" {
		_ = os.MkdirAll(filepath.Dir(*outPath), 0755)
		data, _ := json.MarshalIndent(result, "", "  ")
		_ = os.WriteFile(*outPath, data, 0644)
	}

	emitJSON(result)
}

func datasetExportTables(db *sql.DB, semanticPath string) (map[string][]map[string]interface{}, error) {
	// Collect table names.
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type IN ('table','view') ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[string][]map[string]interface{}, len(tableNames))
	for _, name := range tableNames {
		// Skip internal SQLite tables.
		if strings.HasPrefix(name, "sqlite_") {
			continue
		}
		cols, err := datasetExportColumns(db, name)
		if err != nil {
			continue
		}
		if len(cols) == 0 {
			continue
		}
		data, err := datasetExportRows(db, name, cols)
		if err != nil {
			continue
		}
		out[name] = data
	}
	return out, nil
}

func datasetExportColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query("PRAGMA table_info(" + quoteIdent(table) + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func datasetExportRows(db *sql.DB, table string, cols []string) ([]map[string]interface{}, error) {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(c)
	}
	query := "SELECT " + strings.Join(quoted, ", ") + " FROM " + quoteIdent(table) + " LIMIT 10000"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		for i := range vals {
			vals[i] = new(interface{})
		}
		if err := rows.Scan(vals...); err != nil {
			continue
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			v := *(vals[i].(*interface{}))
			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func quoteIdent(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
}

// answerQuestion responde usando SQLite como fuente declarada. Si SQLite
// no está disponible o no puede responder verificablemente, devuelve un
// diagnóstico controlado en vez de usar otro store.
func answerQuestion(question, storePath string, history []HistoryTurn) (string, error) {
	return answerQuestionWithRuntime(question, storePath, history, loadRuntimeContext("", ""))
}

func answerQuestionWithRuntime(question, storePath string, history []HistoryTurn, rt runtimeContext) (string, error) {
	_ = storePath
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	dbPath := envOr("SABIO_DB", defaultDBPath)
	if _, statErr := os.Stat(dbPath); statErr != nil {
		ans := sabioAnswer{
			Text: "No puedo responder con la fuente declarada porque no encuentro la base SQLite `data_sqlite_db`. No voy a usar otro store ni fallback.",
			Trace: sabioTrace{
				Capability:   classifyCapability(question),
				Source:       "sqlite",
				FallbackUsed: false,
				Error:        statErr.Error(),
			},
		}
		return formatSabioAnswer(ans), nil
	}

	if ans, ok, err := answerDeterministic(ctx, dbPath, question, history, rt); ok || err != nil {
		if err != nil {
			controlled := sabioAnswer{
				Text: "No pude responder con SQLite de forma verificable. No voy a usar otro engine ni fallback silencioso.",
				Trace: sabioTrace{
					Capability:   classifyCapability(question),
					Source:       "sqlite",
					FallbackUsed: false,
					Error:        err.Error(),
				},
			}
			return formatSabioAnswer(controlled), nil
		}
		return formatSabioAnswer(ans), nil
	}

	llmCli, err := llm.NewClient()
	if err != nil {
		ans := sabioAnswer{
			Text: "No puedo generar la consulta SQL porque falta el cliente LLM configurado. No voy a usar otro store ni fallback.",
			Trace: sabioTrace{
				Capability:   "data.query.sql",
				Source:       "sqlite",
				FallbackUsed: false,
				Error:        err.Error(),
			},
		}
		return formatSabioAnswer(ans), nil
	}

	ans, sqlErr := answerWithSQL(ctx, llmCli, dbPath, question, history, rt)
	if sqlErr != nil {
		controlled := sabioAnswer{
			Text: "No pude responder con SQLite de forma verificable. No voy a usar otro engine ni fallback silencioso.",
			Trace: sabioTrace{
				Capability:   "data.query.sql",
				Source:       "sqlite",
				FallbackUsed: false,
				Error:        sqlErr.Error(),
			},
		}
		return formatSabioAnswer(controlled), nil
	}
	return formatSabioAnswer(ans), nil
}

// answerWithSQL implementa el path Text-to-SQL:
//
//  1. abre la DB read-only
//  2. pide al LLM una SQL SELECT a partir de schema + pregunta
//  3. ejecuta y trae filas
//  4. pide al LLM phrasing natural en español rioplatense, sin tecnicismos
//
// Si la DB no responde o la SQL es inválida, retorna error para que el caller
// devuelva un diagnóstico controlado.
func answerWithSQL(ctx context.Context, c *llm.Client, dbPath, question string, history []HistoryTurn, rt runtimeContext) (sabioAnswer, error) {
	eng, err := sqlqa.Open(dbPath)
	if err != nil {
		return sabioAnswer{}, fmt.Errorf("open db: %w", err)
	}
	defer eng.Close()

	// Reintento corto: si el LLM se equivoca con SQL inválido, le devolvemos
	// el error para que corrija. 2 intentos máx.
	var (
		lastSQL  string
		lastErr  error
		queryRes *sqlqa.QueryResult
	)
	for attempt := 0; attempt < 3; attempt++ {
		sqlText, err := generateSQL(ctx, c, eng.Schema(), question, history, lastSQL, lastErr, rt)
		if err != nil {
			return sabioAnswer{}, fmt.Errorf("llm sql gen: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[sabio sql attempt %d] %s\n", attempt+1, sqlText)
		lastSQL = sqlText
		if scopeErr := validateSQLScope(sqlText, rt); scopeErr != nil {
			fmt.Fprintf(os.Stderr, "[sabio sql attempt %d SCOPE FAILED] %v\n", attempt+1, scopeErr)
			lastErr = scopeErr
			continue
		}
		res, runErr := eng.Run(ctx, sqlText)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "[sabio sql attempt %d FAILED] %v\n", attempt+1, runErr)
			lastErr = runErr
			continue
		}
		queryRes = res
		break
	}
	if queryRes == nil {
		return sabioAnswer{}, fmt.Errorf("sql falló tras reintentos: %v", lastErr)
	}

	// Phrasing natural con las filas.
	answer, err := phraseSQLAnswer(ctx, c, question, queryRes, history, rt)
	if err != nil {
		return sabioAnswer{}, fmt.Errorf("phrasing: %w", err)
	}
	return sabioAnswer{
		Text: strings.TrimSpace(answer),
		Trace: sabioTrace{
			Capability:   "data.query.sql",
			Source:       "sqlite",
			SQL:          queryRes.SQL,
			Tables:       extractSQLTables(queryRes.SQL),
			RowCount:     queryRes.RowCount,
			FallbackUsed: false,
		},
	}, nil
}

func answerDeterministic(ctx context.Context, dbPath, question string, history []HistoryTurn, rt runtimeContext) (sabioAnswer, bool, error) {
	_ = history
	q := normalizeQuestion(question)
	if lawFirmNameInQuestion(q) != "" {
		return runLawFirmDetail(ctx, dbPath, question), true, nil
	}
	if mentions(q, "estudio", "jurid") || mentions(q, "law", "firm") {
		if name := lawFirmNameInQuestion(q); name != "" || mentions(q, "mas informacion") || mentions(q, "más información") || mentions(q, "detalle") {
			return runLawFirmDetail(ctx, dbPath, question), true, nil
		}
		return runLawFirms(ctx, dbPath), true, nil
	}
	if mentions(q, "que datos") || mentions(q, "qué datos") || mentions(q, "tablas") || mentions(q, "inventario") {
		return runInventory(ctx, dbPath), true, nil
	}
	if mentions(q, "que clientes") || mentions(q, "qué clientes") {
		if hasClientScope(rt) {
			return runClientListScoped(ctx, dbPath, rt), true, nil
		}
		return runClientList(ctx, dbPath), true, nil
	}
	if requestsExternalAction(q) {
		return actionNotSupportedAnswer(), true, nil
	}
	return sabioAnswer{}, false, nil
}

func lawFirmNameInQuestion(q string) string {
	for _, name := range []string{"bartoletti", "zemlak", "zieme", "ledner"} {
		if strings.Contains(q, name) {
			return name
		}
	}
	return ""
}

func runLawFirmDetail(ctx context.Context, dbPath, question string) sabioAnswer {
	q := normalizeQuestion(question)
	filter := "%"
	if strings.Contains(q, "bartoletti") || strings.Contains(q, "zemlak") {
		filter = "%Bartoletti%"
	}
	if strings.Contains(q, "zieme") || strings.Contains(q, "ledner") {
		filter = "%Zieme%"
	}
	sqlText := `SELECT "id", "name", "has_metadata" FROM "law_firms" WHERE "name" LIKE '` + strings.ReplaceAll(filter, `'`, `''`) + `' ORDER BY "name"`
	ans, err := runFixedSQL(ctx, dbPath, sqlText, "data.entity.detail", []string{"law_firms"})
	if err != nil {
		return controlledSQLiteError("data.entity.detail", err)
	}
	rows := ansRows(ctx, dbPath, sqlText)
	if len(rows) == 0 {
		ans.Text = "No encontré ese estudio jurídico en la tabla law_firms."
		return ans
	}
	var sb strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&sb, "Esto es todo lo que la base tiene para %s:\n- ID interno: %v\n- Tiene metadata adicional: %s", row["name"], row["id"], yesNo(row["has_metadata"]))
	}
	sb.WriteString("\n\nNo hay columnas de dirección, teléfono, fecha de registro ni relación visible con clientes/proyectos en la DB actual para este estudio jurídico.")
	ans.Text = sb.String()
	return ans
}

func yesNo(v any) string {
	if fmt.Sprintf("%v", v) == "1" {
		return "sí"
	}
	return "no"
}

func runLawFirms(ctx context.Context, dbPath string) sabioAnswer {
	sqlText := `SELECT "name" AS estudio_juridico FROM "law_firms" ORDER BY "name"`
	ans, err := runFixedSQL(ctx, dbPath, sqlText, "data.entity.list", []string{"law_firms"})
	if err != nil {
		return controlledSQLiteError("data.entity.list", err)
	}
	names := []string{}
	for _, row := range ansRows(ctx, dbPath, sqlText) {
		if v, ok := row["estudio_juridico"]; ok {
			names = append(names, fmt.Sprintf("%v", v))
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tengo %d estudios jurídicos en la base:", len(names))
	for _, name := range names {
		fmt.Fprintf(&sb, "\n- %s", name)
	}
	sb.WriteString("\n\nNo estoy listando clientes ni deudores: respondí sobre la tabla de estudios jurídicos.")
	ans.Text = sb.String()
	return ans
}

func runInventory(ctx context.Context, dbPath string) sabioAnswer {
	sqlText := `SELECT
	(SELECT COUNT(*) FROM "clients") AS clients_count,
	(SELECT COUNT(*) FROM "clients" WHERE "active"='1') AS active_clients_count,
	(SELECT COUNT(*) FROM "clients" WHERE "active"='0') AS inactive_clients_count,
	(SELECT COUNT(*) FROM "law_firms") AS law_firms_count,
	(SELECT COUNT(*) FROM "projects") AS projects_count,
	(SELECT COUNT(*) FROM "charges") AS charges_count,
	(SELECT COUNT(*) FROM "milestones") AS milestones_count,
	(SELECT COUNT(*) FROM "payments") AS payments_count,
	(SELECT COUNT(*) FROM "billing_documents") AS billing_documents_count,
	(SELECT COUNT(*) FROM "users") AS users_count`
	ans, err := runFixedSQL(ctx, dbPath, sqlText, "data.inventory", []string{"clients", "law_firms", "projects", "charges", "milestones", "payments", "billing_documents", "users"})
	if err != nil {
		return controlledSQLiteError("data.inventory", err)
	}
	rows := ansRows(ctx, dbPath, sqlText)
	if len(rows) == 0 {
		return ans
	}
	r := rows[0]
	ans.Text = fmt.Sprintf("Puedo responder desde SQLite sobre estos datos declarados:\n- Clientes: %v (%v activos, %v inactivos)\n- Estudios jurídicos: %v\n- Proyectos: %v\n- Cargos: %v\n- Hitos: %v\n- Pagos: %v\n- Documentos de facturación: %v\n- Usuarios internos: %v\n\nLímite importante: la tabla clients no trae emails de deudores/clientes; los emails visibles están en users y corresponden a usuarios internos.",
		r["clients_count"], r["active_clients_count"], r["inactive_clients_count"], r["law_firms_count"], r["projects_count"], r["charges_count"], r["milestones_count"], r["payments_count"], r["billing_documents_count"], r["users_count"])
	return ans
}

func runClientList(ctx context.Context, dbPath string) sabioAnswer {
	sqlText := `SELECT "name" AS cliente, "code" AS codigo, "active" AS activo FROM "clients" ORDER BY "name" LIMIT 20`
	ans, err := runFixedSQL(ctx, dbPath, sqlText, "data.entity.list", []string{"clients"})
	if err != nil {
		return controlledSQLiteError("data.entity.list", err)
	}
	rows := ansRows(ctx, dbPath, sqlText)
	total := scalarCount(ctx, dbPath, `SELECT COUNT(*) AS clients_count FROM "clients"`)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tengo %d clientes en la base. Estos son los primeros 20 por nombre:", total)
	for _, row := range rows {
		status := "inactivo"
		if fmt.Sprintf("%v", row["activo"]) == "1" {
			status = "activo"
		}
		fmt.Fprintf(&sb, "\n- %s (%s, %s)", row["cliente"], row["codigo"], status)
	}
	ans.Text = sb.String()
	return ans
}

func runClientListScoped(ctx context.Context, dbPath string, rt runtimeContext) sabioAnswer {
	ids := allowedClientIDs(rt)
	if active := activeClientID(rt); active != "" {
		ids = []string{active}
	}
	if len(ids) == 0 {
		return runClientList(ctx, dbPath)
	}
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		quoted = append(quoted, `'`+strings.ReplaceAll(id, `'`, `''`)+`'`)
	}
	sqlText := `SELECT "name" AS cliente, "code" AS codigo, "active" AS activo FROM "clients" WHERE "id" IN (` + strings.Join(quoted, ",") + `) ORDER BY "name" LIMIT 100`
	ans, err := runFixedSQL(ctx, dbPath, sqlText, "data.entity.list", []string{"clients"})
	if err != nil {
		return controlledSQLiteError("data.entity.list", err)
	}
	rows := ansRows(ctx, dbPath, sqlText)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Dentro del scope de esta sesión tengo %d clientes visibles:", len(rows))
	for _, row := range rows {
		status := "inactivo"
		if fmt.Sprintf("%v", row["activo"]) == "1" {
			status = "activo"
		}
		fmt.Fprintf(&sb, "\n- %s (%s, %s)", row["cliente"], row["codigo"], status)
	}
	ans.Text = sb.String()
	return ans
}

func hasClientScope(rt runtimeContext) bool {
	return len(allowedClientIDs(rt)) > 0 || activeClientID(rt) != ""
}

func runFixedSQL(ctx context.Context, dbPath, sqlText, capability string, tables []string) (sabioAnswer, error) {
	eng, err := sqlqa.Open(dbPath)
	if err != nil {
		return sabioAnswer{}, err
	}
	defer eng.Close()
	res, err := eng.Run(ctx, sqlText)
	if err != nil {
		return sabioAnswer{}, err
	}
	return sabioAnswer{
		Trace: sabioTrace{
			Capability:   capability,
			Source:       "sqlite",
			SQL:          res.SQL,
			Tables:       tables,
			RowCount:     res.RowCount,
			FallbackUsed: false,
		},
	}, nil
}

func ansRows(ctx context.Context, dbPath, sqlText string) []map[string]any {
	eng, err := sqlqa.Open(dbPath)
	if err != nil {
		return nil
	}
	defer eng.Close()
	res, err := eng.Run(ctx, sqlText)
	if err != nil {
		return nil
	}
	return res.Rows
}

func scalarCount(ctx context.Context, dbPath, sqlText string) int {
	rows := ansRows(ctx, dbPath, sqlText)
	if len(rows) == 0 {
		return 0
	}
	for _, v := range rows[0] {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		case string:
			var out int
			if _, err := fmt.Sscanf(n, "%d", &out); err == nil {
				return out
			}
		}
	}
	return 0
}

func controlledSQLiteError(capability string, err error) sabioAnswer {
	return sabioAnswer{
		Text: "No pude responder con SQLite de forma verificable. No voy a usar otro engine ni fallback silencioso.",
		Trace: sabioTrace{
			Capability:   capability,
			Source:       "sqlite",
			FallbackUsed: false,
			Error:        err.Error(),
		},
	}
}

func runEntity360Artifact(dbPath string, rt runtimeContext, question, entityType, entityRef, analysisIntent string) map[string]any {
	entityType = canonicalEntityType(entityType)
	entityRef = strings.TrimSpace(entityRef)
	if entityRef == "" {
		entityRef = activeClientID(rt)
	}
	if entityType != "client" || entityRef == "" {
		return map[string]any{
			"artifact_type": "entity_360.v1",
			"artifacts":     []string{"entity_360.v1", "answer.grounded.v1"},
			"business_id":   rt.BusinessID,
			"audience":      rt.Audience,
			"question":      question,
			"text":          "No pude construir la vista 360 de forma verificable porque falta una entidad cliente activa.",
			"summary":       "Falta entity_ref/entity_type para construir entity_360.",
			"verified":      false,
			"trace": map[string]any{
				"capability":    "data.entity_360",
				"source":        "sqlite",
				"fallback_used": false,
				"error":         "entity_ref ausente o entity_type no soportado",
			},
			"generated_at": time.Now().UTC().Format(time.RFC3339),
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := dbPath + "?_pragma=query_only(true)&mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return entity360ErrorArtifact(rt, question, err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return entity360ErrorArtifact(rt, question, err)
	}

	baseSQL := `SELECT
		c.id,
		c.code,
		c.name,
		c.active,
		COUNT(DISTINCT ch.id) AS charges_count,
		COUNT(DISTINCT CASE WHEN ch.state != 'PAGADO' THEN ch.id END) AS open_charges_count,
		COUNT(DISTINCT bd.id) AS documents_count,
		COUNT(DISTINCT CASE WHEN ch.state != 'PAGADO' THEN bd.id END) AS open_documents_count,
		COALESCE(SUM(CASE WHEN ch.state != 'PAGADO' THEN CAST(m.amount AS REAL) ELSE 0 END), 0) AS open_amount,
		COALESCE(SUM(CAST(m.amount AS REAL)), 0) AS total_historical_milestones,
		MIN(CASE WHEN ch.state != 'PAGADO' THEN m.date END) AS oldest_open_milestone_date,
		MIN(m.date) AS oldest_any_milestone_date,
		CAST(julianday('now') - julianday(MIN(CASE WHEN ch.state != 'PAGADO' THEN m.date END)) AS INT) AS oldest_open_debt_days,
		CAST(julianday('now') - julianday(MIN(m.date)) AS INT) AS oldest_any_milestone_days,
		COALESCE((SELECT COUNT(*) FROM payments p WHERE p.client_id = c.id), 0) AS payments_count,
		COALESCE((SELECT SUM(CAST(p.amount AS REAL)) FROM payments p WHERE p.client_id = c.id), 0) AS payments_total,
		COALESCE((SELECT SUM(CAST(p.residue AS REAL)) FROM payments p WHERE p.client_id = c.id), 0) AS payment_residue_total
	FROM clients c
	LEFT JOIN charges ch ON ch.client_id = c.id
	LEFT JOIN milestones m ON m.charge_id = ch.id
	LEFT JOIN billing_documents bd ON bd.charge_id = ch.id
	WHERE c.id = ?
	GROUP BY c.id, c.code, c.name, c.active`
	var (
		clientID                  string
		clientCode                sql.NullString
		clientName                sql.NullString
		active                    sql.NullString
		chargesCount              sql.NullInt64
		openChargesCount          sql.NullInt64
		documentsCount            sql.NullInt64
		openDocumentsCount        sql.NullInt64
		openAmount                sql.NullFloat64
		totalHistoricalMilestones sql.NullFloat64
		oldestOpenMilestoneDate   sql.NullString
		oldestAnyMilestoneDate    sql.NullString
		oldestOpenDebtDays        sql.NullInt64
		oldestAnyMilestoneDays    sql.NullInt64
		paymentsCount             sql.NullInt64
		paymentsTotal             sql.NullFloat64
		paymentResidueTotal       sql.NullFloat64
	)
	if err := db.QueryRowContext(ctx, baseSQL, entityRef).Scan(
		&clientID,
		&clientCode,
		&clientName,
		&active,
		&chargesCount,
		&openChargesCount,
		&documentsCount,
		&openDocumentsCount,
		&openAmount,
		&totalHistoricalMilestones,
		&oldestOpenMilestoneDate,
		&oldestAnyMilestoneDate,
		&oldestOpenDebtDays,
		&oldestAnyMilestoneDays,
		&paymentsCount,
		&paymentsTotal,
		&paymentResidueTotal,
	); err != nil {
		return entity360ErrorArtifact(rt, question, err)
	}

	statesSQL := `SELECT state, COUNT(*) AS count FROM charges WHERE client_id = ? GROUP BY state ORDER BY state`
	stateRows, err := db.QueryContext(ctx, statesSQL, entityRef)
	if err != nil {
		return entity360ErrorArtifact(rt, question, err)
	}
	defer stateRows.Close()
	chargeStates := map[string]int{}
	for stateRows.Next() {
		var state sql.NullString
		var count sql.NullInt64
		if err := stateRows.Scan(&state, &count); err != nil {
			return entity360ErrorArtifact(rt, question, err)
		}
		chargeStates[state.String] = int(count.Int64)
	}

	openDocSQL := `SELECT bd.id, bd.number, bd.date, ch.state
		FROM billing_documents bd
		JOIN charges ch ON ch.id = bd.charge_id
		WHERE ch.client_id = ? AND ch.state != 'PAGADO'
		ORDER BY bd.date`
	openDocRows, err := db.QueryContext(ctx, openDocSQL, entityRef)
	if err != nil {
		return entity360ErrorArtifact(rt, question, err)
	}
	defer openDocRows.Close()
	var openDocuments []map[string]any
	openInvoiceNumber := ""
	for openDocRows.Next() {
		var docID, number, date, state sql.NullString
		if err := openDocRows.Scan(&docID, &number, &date, &state); err != nil {
			return entity360ErrorArtifact(rt, question, err)
		}
		row := map[string]any{
			"id":     docID.String,
			"number": number.String,
			"date":   date.String,
			"state":  state.String,
		}
		openDocuments = append(openDocuments, row)
		if openInvoiceNumber == "" {
			openInvoiceNumber = number.String
		}
	}

	activeBool := active.String == "1"
	dataGaps := []string{"contact email missing"}
	if chargeStates["PAGADO"] > 0 {
		dataGaps = append(dataGaps, "mixed charge states require active-debt filtering")
		dataGaps = append(dataGaps, "historical paid milestones should not be treated as open debt")
	}
	if openDocumentsCount.Int64 == 0 {
		dataGaps = append(dataGaps, "no open billing document found for non-PAGADO charges")
	}

	text := fmt.Sprintf(
		"%s (%s) tiene saldo abierto %.2f, mora abierta aproximada de %d días, %d pagos históricos por %.2f, %d cargos totales (%d abiertos) y %d documentos (%d abiertos). El cliente está %s. Hay estados mezclados %s, por lo que conviene separar deuda abierta de histórico pagado.",
		clientName.String,
		clientCode.String,
		nullFloat(openAmount),
		nullInt(oldestOpenDebtDays),
		nullInt(paymentsCount),
		nullFloat(paymentsTotal),
		nullInt(chargesCount),
		nullInt(openChargesCount),
		nullInt(documentsCount),
		nullInt(openDocumentsCount),
		map[bool]string{true: "activo", false: "inactivo"}[activeBool],
		joinChargeStates(chargeStates),
	)

	structured := map[string]any{
		"name":               clientName.String,
		"code":               clientCode.String,
		"active":             activeBool,
		"amount":             nullFloat(openAmount),
		"saldo_total":        nullFloat(openAmount),
		"days_past_due":      nullInt(oldestOpenDebtDays),
		"invoice_number":     openInvoiceNumber,
		"document_number":    openInvoiceNumber,
		"payment_count":      nullInt(paymentsCount),
		"payment_total":      nullFloat(paymentsTotal),
		"charge_states":      chargeStates,
		"open_documents":     openDocuments,
		"missing_fields":     []string{"contact.destination.v1"},
		"analysis_intent":    analysisIntent,
		"entity_type":        entityType,
		"entity_ref":         entityRef,
		"has_financial_data": true,
		"has_mora_data":      oldestOpenDebtDays.Valid,
		"has_documents":      documentsCount.Int64 > 0,
		"has_email":          false,
	}

	return map[string]any{
		"artifact_type": "entity_360.v1",
		"artifacts":     []string{"entity_360.v1", "answer.grounded.v1"},
		"business_id":   rt.BusinessID,
		"audience":      rt.Audience,
		"question":      question,
		"text":          text,
		"answer":        text,
		"summary":       text,
		"verified":      true,
		"entity": map[string]any{
			"id":     clientID,
			"code":   clientCode.String,
			"name":   clientName.String,
			"active": activeBool,
			"type":   entityType,
		},
		"financial_position": map[string]any{
			"open_amount":                 nullFloat(openAmount),
			"total_historical_milestones": nullFloat(totalHistoricalMilestones),
			"payments_total":              nullFloat(paymentsTotal),
			"payment_residue_total":       nullFloat(paymentResidueTotal),
		},
		"aging": map[string]any{
			"oldest_open_milestone_date": oldestOpenMilestoneDate.String,
			"oldest_open_debt_days":      nullInt(oldestOpenDebtDays),
			"oldest_any_milestone_date":  oldestAnyMilestoneDate.String,
			"oldest_any_milestone_days":  nullInt(oldestAnyMilestoneDays),
		},
		"history": map[string]any{
			"payments_count":        nullInt(paymentsCount),
			"payments_total":        nullFloat(paymentsTotal),
			"payment_residue_total": nullFloat(paymentResidueTotal),
		},
		"documents": map[string]any{
			"count":               nullInt(documentsCount),
			"open_count":          nullInt(openDocumentsCount),
			"open_documents":      openDocuments,
			"open_invoice_number": openInvoiceNumber,
		},
		"charges_count":      nullInt(chargesCount),
		"open_charges_count": nullInt(openChargesCount),
		"charges_by_state":   chargeStates,
		"data_gaps":          dataGaps,
		"structured":         structured,
		"evidence": map[string]any{
			"source": "sqlite",
			"sql": []string{
				baseSQL,
				statesSQL,
				openDocSQL,
			},
		},
		"trace": map[string]any{
			"capability":    "data.entity_360",
			"source":        "sqlite",
			"tables":        []string{"clients", "charges", "milestones", "billing_documents", "payments"},
			"fallback_used": false,
		},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func entity360ErrorArtifact(rt runtimeContext, question string, err error) map[string]any {
	return map[string]any{
		"artifact_type": "entity_360.v1",
		"artifacts":     []string{"entity_360.v1", "answer.grounded.v1"},
		"business_id":   rt.BusinessID,
		"audience":      rt.Audience,
		"question":      question,
		"text":          "No pude construir la vista 360 con SQLite de forma verificable.",
		"summary":       "No pude construir la vista 360 con SQLite de forma verificable.",
		"verified":      false,
		"trace": map[string]any{
			"capability":    "data.entity_360",
			"source":        "sqlite",
			"fallback_used": false,
			"error":         err.Error(),
		},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func runAnalyticalQueryArtifact(dbPath string, rt runtimeContext, question, analysisIntent, semanticCapability, entityType, entityRef string, metrics []string, peerStrategy string) map[string]any {
	switch canonicalAnalyticalIntent(question, analysisIntent, semanticCapability, entityRef, metrics, peerStrategy) {
	case "portfolio_comparison":
		return runPortfolioComparisonArtifact(dbPath, rt, question, entityType, entityRef, metrics, peerStrategy)
	case "score_sensitivity":
		return runScoreSensitivityArtifact(dbPath, rt, question, entityType, entityRef, metrics)
	case "counterfactual_scenario":
		return runCounterfactualArtifact(dbPath, rt, question, entityType, entityRef)
	case "payment_behavior_summary":
		return runPaymentBehaviorSummaryArtifact(dbPath, rt, question, entityType, entityRef)
	default:
		return nil
	}
}

func canonicalAnalyticalIntent(question, analysisIntent, semanticCapability, entityRef string, metrics []string, peerStrategy string) string {
	switch strings.TrimSpace(strings.ToLower(semanticCapability)) {
	case "evidence.portfolio_comparison":
		return "portfolio_comparison"
	case "evidence.payment_behavior_summary":
		return "payment_behavior_summary"
	case "evidence.score_sensitivity":
		return "score_sensitivity"
	case "evidence.counterfactual":
		return "counterfactual_scenario"
	}
	intent := strings.TrimSpace(strings.ToLower(analysisIntent))
	switch intent {
	case "portfolio_comparison", "payment_behavior_summary", "score_sensitivity", "counterfactual_scenario":
		return intent
	}
	hasEntity := strings.TrimSpace(entityRef) != ""
	if strings.Contains(intent, "compar") || strings.Contains(intent, "similar") || strings.TrimSpace(peerStrategy) != "" {
		return "portfolio_comparison"
	}
	// "cartera" sin entity_ref = pregunta general de cartera → dejar que el flujo SQL
	// general la resuelva (genera SQL agregado sobre la tabla real del negocio).
	// Solo rutear a portfolio_comparison si hay una entidad específica para comparar.
	if strings.Contains(intent, "cartera") && hasEntity {
		return "portfolio_comparison"
	}
	questionLower := strings.ToLower(strings.TrimSpace(question))
	if strings.Contains(questionLower, "compar") || strings.Contains(questionLower, "similar") {
		return "portfolio_comparison"
	}
	if strings.Contains(questionLower, "cartera") && hasEntity {
		return "portfolio_comparison"
	}
	if strings.Contains(intent, "payment_behavior") || strings.Contains(intent, "comportamiento de pago") {
		return "payment_behavior_summary"
	}
	if strings.Contains(questionLower, "comportamiento de pago") || strings.Contains(questionLower, "historial de pago") {
		return "payment_behavior_summary"
	}
	for _, metric := range metrics {
		if strings.EqualFold(strings.TrimSpace(metric), "payment_behavior") {
			return "payment_behavior_summary"
		}
	}
	if strings.Contains(intent, "sensibilidad") || strings.Contains(intent, "score_sensitivity") {
		return "score_sensitivity"
	}
	if strings.Contains(intent, "contrafactual") || strings.Contains(questionLower, "contrafactual") {
		return "counterfactual_scenario"
	}
	return ""
}

type portfolioRow struct {
	ID            string
	Name          string
	OpenAmount    float64
	DaysPastDue   int
	PaymentCount  int
	PaymentsTotal float64
}

func runPortfolioComparisonArtifact(dbPath string, rt runtimeContext, question, entityType, entityRef string, metrics []string, peerStrategy string) map[string]any {
	base := runEntity360Artifact(dbPath, rt, question, entityType, entityRef, "case_baseline")
	if verified, _ := base["verified"].(bool); !verified {
		return analyticalErrorArtifact(rt, question, "portfolio_comparison", "No pude verificar la línea base del caso antes de comparar cartera.", delegationTraceFromArtifact(base))
	}
	rows, err := loadPortfolioRows(dbPath)
	if err != nil {
		return analyticalErrorArtifact(rt, question, "portfolio_comparison", "No pude calcular la comparación de cartera con SQLite de forma verificable.", map[string]any{"error": err.Error()})
	}
	targetID := firstNonEmpty(entityRef, activeClientID(rt))
	var target portfolioRow
	found := false
	for _, row := range rows {
		if row.ID == targetID {
			target = row
			found = true
			break
		}
	}
	if !found {
		return analyticalErrorArtifact(rt, question, "portfolio_comparison", "No pude ubicar el caso dentro de la cartera comparable.", nil)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].OpenAmount == rows[j].OpenAmount {
			return rows[i].DaysPastDue > rows[j].DaysPastDue
		}
		return rows[i].OpenAmount > rows[j].OpenAmount
	})
	rankByAmount := 0
	rankByMora := 1
	for i, row := range rows {
		if row.ID == targetID {
			rankByAmount = i + 1
			break
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].DaysPastDue == rows[j].DaysPastDue {
			return rows[i].OpenAmount > rows[j].OpenAmount
		}
		return rows[i].DaysPastDue > rows[j].DaysPastDue
	})
	for i, row := range rows {
		if row.ID == targetID {
			rankByMora = i + 1
			break
		}
	}
	peers := closestPeers(rows, targetID, 4)
	openPct := percentileFromRank(rankByAmount, len(rows))
	moraPct := percentileFromRank(rankByMora, len(rows))
	text := fmt.Sprintf("%s no destaca por mora pura sino por materialidad: tiene saldo abierto %.2f, mora %d días, %d pagos históricos y queda percentil %d por saldo contra percentil %d por mora. Clientes comparables: %s.",
		target.Name,
		target.OpenAmount,
		target.DaysPastDue,
		target.PaymentCount,
		openPct,
		moraPct,
		describePeers(peers),
	)
	return map[string]any{
		"artifact_type":   "answer.grounded.v1",
		"artifacts":       []string{"answer.grounded.v1"},
		"business_id":     rt.BusinessID,
		"audience":        rt.Audience,
		"question":        question,
		"text":            text,
		"answer":          text,
		"summary":         text,
		"verified":        true,
		"analysis_intent": "portfolio_comparison",
		"structured": map[string]any{
			"entity_ref":               target.ID,
			"entity_name":              target.Name,
			"metrics":                  metrics,
			"peer_strategy":            firstNonEmpty(peerStrategy, "similar_clients"),
			"open_amount":              target.OpenAmount,
			"days_past_due":            target.DaysPastDue,
			"payment_count":            target.PaymentCount,
			"payment_total":            target.PaymentsTotal,
			"open_amount_percentile":   openPct,
			"days_past_due_percentile": moraPct,
			"peers":                    peers,
		},
		"trace": map[string]any{
			"capability":    "data.query.sql",
			"source":        "sqlite",
			"tables":        []string{"clients", "charges", "milestones", "payments"},
			"fallback_used": false,
		},
		"evidence": map[string]any{
			"source":  "sqlite",
			"metrics": metrics,
		},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func runScoreSensitivityArtifact(dbPath string, rt runtimeContext, question, entityType, entityRef string, metrics []string) map[string]any {
	rows, err := loadPortfolioRows(dbPath)
	if err != nil {
		return analyticalErrorArtifact(rt, question, "score_sensitivity", "No pude calcular sensibilidad del score con SQLite de forma verificable.", map[string]any{"error": err.Error()})
	}
	targetID := firstNonEmpty(strings.TrimSpace(entityRef), activeClientID(rt))
	target, others, ok := locatePortfolioTarget(rows, targetID)
	if !ok {
		return analyticalErrorArtifact(rt, question, "score_sensitivity", "No pude ubicar el caso objetivo para el análisis de sensibilidad.", nil)
	}
	maxOpen, maxDays := portfolioMaxima(rows)
	currentScore := weightedPortfolioScore(target, maxOpen, maxDays)
	secondByAmount := highestOpenAmount(others)
	reducedMateriality := target
	reducedMateriality.OpenAmount = secondByAmount
	materialityScore := weightedPortfolioScore(reducedMateriality, maxOpen, maxDays)
	reducedMora := target
	reducedMora.DaysPastDue = target.DaysPastDue / 2
	moraScore := weightedPortfolioScore(reducedMora, maxOpen, maxDays)
	driver := "materialidad"
	if absFloat(float64(currentScore-moraScore)) > absFloat(float64(currentScore-materialityScore)) {
		driver = "mora/riesgo"
	}
	text := fmt.Sprintf("La variable más sensible del score de %s es %s. Score base aproximado %.0f/100; si el saldo bajara hacia %.2f caería a %.0f, mientras que si la mora se redujera a %d días caería a %.0f.",
		target.Name,
		driver,
		currentScore,
		secondByAmount,
		materialityScore,
		reducedMora.DaysPastDue,
		moraScore,
	)
	return analyticalSuccessArtifact(rt, question, "score_sensitivity", text, map[string]any{
		"entity_ref":                   target.ID,
		"entity_name":                  target.Name,
		"metrics":                      metrics,
		"current_score":                currentScore,
		"score_if_materiality_reduced": materialityScore,
		"score_if_mora_reduced":        moraScore,
		"primary_driver":               driver,
	})
}

func runCounterfactualArtifact(dbPath string, rt runtimeContext, question, entityType, entityRef string) map[string]any {
	rows, err := loadPortfolioRows(dbPath)
	if err != nil {
		return analyticalErrorArtifact(rt, question, "counterfactual", "No pude calcular el contrafactual con SQLite de forma verificable.", map[string]any{"error": err.Error()})
	}
	targetID := firstNonEmpty(strings.TrimSpace(entityRef), activeClientID(rt))
	target, others, ok := locatePortfolioTarget(rows, targetID)
	if !ok {
		return analyticalErrorArtifact(rt, question, "counterfactual", "No pude ubicar el caso objetivo para el contrafactual.", nil)
	}
	maxOpen, maxDays := portfolioMaxima(rows)
	baseScore := weightedPortfolioScore(target, maxOpen, maxDays)
	otherTopScore := 0.0
	for _, row := range others {
		if score := weightedPortfolioScore(row, maxOpen, maxDays); score > otherTopScore {
			otherTopScore = score
		}
	}
	cf := target
	cf.OpenAmount = highestOpenAmount(others)
	cf.DaysPastDue = target.DaysPastDue / 2
	cfScore := weightedPortfolioScore(cf, maxOpen, maxDays)
	stillFirst := cfScore >= otherTopScore
	text := fmt.Sprintf("En un contrafactual simple, si %s bajara su saldo hacia %.2f y además la mora fuera más reciente (%d días), su score aproximado quedaría en %.0f y %sseguiría primero frente al resto.",
		target.Name,
		cf.OpenAmount,
		cf.DaysPastDue,
		cfScore,
		map[bool]string{true: "", false: "no "}[stillFirst],
	)
	return analyticalSuccessArtifact(rt, question, "counterfactual", text, map[string]any{
		"entity_ref":           target.ID,
		"entity_name":          target.Name,
		"base_score":           baseScore,
		"counterfactual_score": cfScore,
		"still_first":          stillFirst,
	})
}

func runPaymentBehaviorSummaryArtifact(dbPath string, rt runtimeContext, question, entityType, entityRef string) map[string]any {
	base := runEntity360Artifact(dbPath, rt, question, entityType, entityRef, "payment_behavior_summary")
	if verified, _ := base["verified"].(bool); !verified {
		return analyticalErrorArtifact(rt, question, "payment_behavior_summary", "No pude verificar el comportamiento de pagos del caso.", delegationTraceFromArtifact(base))
	}
	history, _ := base["history"].(map[string]any)
	entity, _ := base["entity"].(map[string]any)
	text := fmt.Sprintf("%s registra %s pagos históricos por %s y residuo total %s. Eso sugiere historial de pago registrado, aunque no alcanza por sí solo para concluir recuperabilidad.",
		firstNonEmpty(fmt.Sprintf("%v", entity["name"]), "El caso"),
		formatAny(history["payments_count"]),
		formatAny(history["payments_total"]),
		formatAny(history["payment_residue_total"]),
	)
	return analyticalSuccessArtifact(rt, question, "payment_behavior_summary", text, history)
}

func analyticalSuccessArtifact(rt runtimeContext, question, analysisIntent, text string, structured map[string]any) map[string]any {
	return map[string]any{
		"artifact_type":   "answer.grounded.v1",
		"artifacts":       []string{"answer.grounded.v1"},
		"business_id":     rt.BusinessID,
		"audience":        rt.Audience,
		"question":        question,
		"text":            text,
		"answer":          text,
		"summary":         text,
		"verified":        true,
		"analysis_intent": analysisIntent,
		"structured":      structured,
		"trace": map[string]any{
			"capability":    "data.query.sql",
			"source":        "sqlite",
			"fallback_used": false,
		},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func analyticalErrorArtifact(rt runtimeContext, question, analysisIntent, text string, trace map[string]any) map[string]any {
	if trace == nil {
		trace = map[string]any{}
	}
	trace["capability"] = "data.query.sql"
	trace["source"] = "sqlite"
	trace["fallback_used"] = false
	return map[string]any{
		"artifact_type":   "answer.grounded.v1",
		"artifacts":       []string{"answer.grounded.v1"},
		"business_id":     rt.BusinessID,
		"audience":        rt.Audience,
		"question":        question,
		"text":            text,
		"answer":          text,
		"summary":         text,
		"verified":        false,
		"analysis_intent": analysisIntent,
		"trace":           trace,
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
	}
}

func loadPortfolioRows(dbPath string) ([]portfolioRow, error) {
	dsn := dbPath + "?_pragma=query_only(true)&mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	sqlText := `SELECT
		c.id,
		c.name,
		COALESCE(SUM(CASE WHEN ch.state != 'PAGADO' THEN CAST(m.amount AS REAL) ELSE 0 END), 0) AS open_amount,
		CAST(julianday('now') - julianday(MIN(CASE WHEN ch.state != 'PAGADO' THEN m.date END)) AS INT) AS days_past_due,
		COALESCE((SELECT COUNT(*) FROM payments p WHERE p.client_id = c.id), 0) AS payment_count,
		COALESCE((SELECT SUM(CAST(p.amount AS REAL)) FROM payments p WHERE p.client_id = c.id), 0) AS payments_total
	FROM clients c
	LEFT JOIN charges ch ON ch.client_id = c.id
	LEFT JOIN milestones m ON m.charge_id = ch.id
	GROUP BY c.id, c.name
	HAVING COALESCE(SUM(CASE WHEN ch.state != 'PAGADO' THEN CAST(m.amount AS REAL) ELSE 0 END), 0) > 0`
	rows, err := db.Query(sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []portfolioRow
	for rows.Next() {
		var row portfolioRow
		if err := rows.Scan(&row.ID, &row.Name, &row.OpenAmount, &row.DaysPastDue, &row.PaymentCount, &row.PaymentsTotal); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func locatePortfolioTarget(rows []portfolioRow, targetID string) (portfolioRow, []portfolioRow, bool) {
	var target portfolioRow
	var others []portfolioRow
	found := false
	for _, row := range rows {
		if row.ID == targetID {
			target = row
			found = true
			continue
		}
		others = append(others, row)
	}
	return target, others, found
}

func portfolioMaxima(rows []portfolioRow) (float64, int) {
	maxOpen := 0.0
	maxDays := 0
	for _, row := range rows {
		if row.OpenAmount > maxOpen {
			maxOpen = row.OpenAmount
		}
		if row.DaysPastDue > maxDays {
			maxDays = row.DaysPastDue
		}
	}
	if maxOpen == 0 {
		maxOpen = 1
	}
	if maxDays == 0 {
		maxDays = 1
	}
	return maxOpen, maxDays
}

func weightedPortfolioScore(row portfolioRow, maxOpen float64, maxDays int) float64 {
	materiality := (row.OpenAmount / maxOpen) * 40
	risk := (float64(row.DaysPastDue) / float64(maxDays)) * 30
	behavior := 0.0
	if row.PaymentCount > 0 {
		behavior = 30
	}
	return materiality + risk + behavior
}

func highestOpenAmount(rows []portfolioRow) float64 {
	best := 0.0
	for _, row := range rows {
		if row.OpenAmount > best {
			best = row.OpenAmount
		}
	}
	return best
}

func percentileFromRank(rank, total int) int {
	if total <= 1 {
		return 100
	}
	return int(100 - float64(rank-1)*100/float64(total-1))
}

func closestPeers(rows []portfolioRow, targetID string, limit int) []map[string]any {
	var target portfolioRow
	found := false
	for _, row := range rows {
		if row.ID == targetID {
			target = row
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return absFloat(rows[i].OpenAmount-target.OpenAmount) < absFloat(rows[j].OpenAmount-target.OpenAmount)
	})
	out := []map[string]any{}
	for _, row := range rows {
		if row.ID == targetID {
			continue
		}
		out = append(out, map[string]any{
			"id":            row.ID,
			"name":          row.Name,
			"open_amount":   row.OpenAmount,
			"days_past_due": row.DaysPastDue,
			"payment_count": row.PaymentCount,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func describePeers(peers []map[string]any) string {
	if len(peers) == 0 {
		return "sin cohorte comparable verificable"
	}
	parts := make([]string, 0, len(peers))
	for _, peer := range peers {
		parts = append(parts, fmt.Sprintf("%v (saldo %s, mora %s días)", peer["name"], formatAny(peer["open_amount"]), formatAny(peer["days_past_due"])))
	}
	return strings.Join(parts, "; ")
}

func delegationTraceFromArtifact(artifact map[string]any) map[string]any {
	if trace, ok := artifact["trace"].(map[string]any); ok {
		return trace
	}
	if trace, ok := artifact["trace"].(map[string]interface{}); ok {
		out := map[string]any{}
		for k, v := range trace {
			out[k] = v
		}
		return out
	}
	return nil
}

func formatAny(v interface{}) string {
	switch n := v.(type) {
	case float64:
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%.2f", n)
	case int:
		return fmt.Sprintf("%d", n)
	case int64:
		return fmt.Sprintf("%d", n)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func nullInt(v sql.NullInt64) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int64)
}

func nullFloat(v sql.NullFloat64) float64 {
	if !v.Valid {
		return 0
	}
	return v.Float64
}

func joinChargeStates(states map[string]int) string {
	if len(states) == 0 {
		return "sin estados"
	}
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, states[k]))
	}
	return strings.Join(parts, ", ")
}

func actionNotSupportedAnswer() sabioAnswer {
	return sabioAnswer{
		Text: "No puedo ejecutar esa acción desde Sabio. Puedo consultar datos en SQLite, pero para redactar, enviar mensajes o registrar eventos falta delegar a capabilities específicas.",
		Trace: sabioTrace{
			Capability:          "data.query.sql",
			Source:              "sqlite",
			FallbackUsed:        false,
			MissingCapabilities: []string{"message.draft", "message.send", "contact.lookup", "task.event"},
		},
	}
}

func formatSabioAnswer(ans sabioAnswer) string {
	ans.Text = strings.TrimSpace(ans.Text)
	if ans.Trace.Source == "" {
		ans.Trace.Source = "sqlite"
	}
	trace, err := json.MarshalIndent(ans.Trace, "", "  ")
	if err != nil {
		return ans.Text
	}
	return fmt.Sprintf("%s\n\nEvidencia:\n```json\n%s\n```", ans.Text, trace)
}

func sabioQueryArtifact(question, answer string, rt runtimeContext, capability string) map[string]any {
	artifactType := "answer.grounded.v1"
	if capability == "data.entity_360" {
		artifactType = "entity_360.v1"
	}
	text, trace := splitFormattedSabioAnswer(answer)
	verified := true
	if errText, ok := trace["error"].(string); ok && strings.TrimSpace(errText) != "" {
		verified = false
	}
	result := map[string]any{
		"artifact_type": artifactType,
		"artifacts":     []string{artifactType, "answer.grounded.v1"},
		"business_id":   rt.BusinessID,
		"audience":      rt.Audience,
		"question":      question,
		"text":          text,
		"answer":        text,
		"summary":       text,
		"verified":      verified,
		"trace":         trace,
		"evidence": map[string]any{
			"source": "sqlite",
		},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
	// For entity_360, extract structured fields from the answer trace
	// and identify what's missing for downstream consumers (mecanico, mensajero).
	if capability == "data.entity_360" {
		extracted := extractStructuredEntityFields(answer)
		if extracted != nil {
			result["structured"] = extracted
			if email, ok := extracted["email"].(string); ok && email != "" {
				result["email"] = email
			}
		}
	}
	return result
}

func splitFormattedSabioAnswer(answer string) (string, map[string]any) {
	text := strings.TrimSpace(answer)
	trace := map[string]any{}
	marker := "\n\nEvidencia:\n```json\n"
	idx := strings.Index(answer, marker)
	if idx < 0 {
		return text, trace
	}
	text = strings.TrimSpace(answer[:idx])
	rawTrace := strings.TrimSpace(strings.TrimSuffix(answer[idx+len(marker):], "\n```"))
	if rawTrace != "" {
		_ = json.Unmarshal([]byte(rawTrace), &trace)
	}
	return text, trace
}

// extractStructuredEntityFields parses the entity_360 answer to pull out
// structured data fields (name, saldo, mora, email, etc.) for consumption
// by downstream frameworks. It also identifies what fields are MISSING.
func extractStructuredEntityFields(answer string) map[string]any {
	fields := map[string]any{}
	missing := []string{}

	// Name detection: sabio typically starts with "La cartera de <name>"
	// or "<name> tiene..."
	lower := strings.ToLower(answer)
	email := extractEmail(answer)
	hasEmail := email != ""
	hasPhone := strings.Contains(lower, "telefono") || strings.Contains(lower, "teléfono") || strings.Contains(lower, "celular")
	hasSaldo := strings.Contains(lower, "saldo") || strings.Contains(lower, "adeuda") || strings.Contains(lower, "monto") || strings.Contains(lower, "$")
	hasMora := strings.Contains(lower, "mora") || strings.Contains(lower, "día") || strings.Contains(lower, "dia")
	hasFacturas := strings.Contains(lower, "factura") || strings.Contains(lower, "documento")

	if hasSaldo {
		fields["has_financial_data"] = true
	} else {
		missing = append(missing, "financial_data")
	}
	if hasMora {
		fields["has_mora_data"] = true
	} else {
		missing = append(missing, "mora_data")
	}
	if hasFacturas {
		fields["has_documents"] = true
	} else {
		missing = append(missing, "documents")
	}
	if hasEmail {
		fields["has_email"] = true
		fields["email"] = email
	} else {
		missing = append(missing, "contact.destination.v1")
	}
	if hasPhone {
		fields["has_phone"] = true
	}

	if len(missing) > 0 {
		fields["missing_fields"] = missing
	}
	return fields
}

func extractEmail(s string) string {
	re := regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	for _, m := range re.FindAllString(s, -1) {
		if !strings.Contains(strings.ToLower(m), "@ejemplo.") {
			return m
		}
	}
	return ""
}

func classifyCapability(question string) string {
	q := normalizeQuestion(question)
	if mentions(q, "que datos") || mentions(q, "tablas") || mentions(q, "inventario") {
		return "data.inventory"
	}
	if mentions(q, "estudio", "jurid") || mentions(q, "law", "firm") || mentions(q, "que clientes") {
		return "data.entity.list"
	}
	return "data.query.sql"
}

func normalizeQuestion(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"á", "a",
		"é", "e",
		"í", "i",
		"ó", "o",
		"ú", "u",
		"ü", "u",
		"ñ", "n",
	)
	return replacer.Replace(s)
}

func mentions(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, normalizeQuestion(part)) {
			return false
		}
	}
	return true
}

func requestsExternalAction(q string) bool {
	actionWords := []string{"manda", "mandar", "envia", "enviar", "redacta", "redactar", "email", "correo", "recordatorio", "whatsapp", "mensaje", "agenda", "crear tarea", "registrar"}
	for _, word := range actionWords {
		if strings.Contains(q, word) {
			return true
		}
	}
	return false
}

func extractSQLTables(sqlText string) []string {
	cleaned := strings.NewReplacer("\n", " ", "\t", " ", ",", " ", "(", " ", ")", " ").Replace(sqlText)
	fields := strings.Fields(cleaned)
	seen := map[string]bool{}
	out := []string{}
	for i, f := range fields {
		token := strings.ToLower(strings.Trim(f, `"'`))
		if token != "from" && token != "join" {
			continue
		}
		if i+1 >= len(fields) {
			continue
		}
		table := strings.Trim(fields[i+1], `"'`)
		table = strings.TrimSpace(table)
		if table == "" || seen[table] {
			continue
		}
		seen[table] = true
		out = append(out, table)
	}
	return out
}

func validateSQLScope(sqlText string, rt runtimeContext) error {
	allowed := allowedClientIDs(rt)
	activeID := activeClientID(rt)
	if len(allowed) == 0 && activeID == "" {
		return nil
	}
	pack, err := loadBusinessPack(rt.BusinessID)
	if err != nil {
		return err
	}
	tables := extractSQLTables(sqlText)
	if len(tables) == 0 {
		return nil
	}
	required := []string{}
	for _, table := range tables {
		base := strings.Trim(table, `"'`)
		if _, ok := pack.ScopePolicies.Tables[base]; ok {
			required = append(required, base)
		}
	}
	if len(required) == 0 {
		return nil
	}
	low := strings.ToLower(sqlText)
	if activeID != "" && strings.Contains(low, "'"+strings.ToLower(activeID)+"'") {
		return nil
	}
	for _, id := range allowed {
		if strings.Contains(low, "'"+strings.ToLower(id)+"'") {
			return nil
		}
	}
	return fmt.Errorf("scope requerido para tablas %s: la SQL no contiene ningún client_id permitido ni active_entity.id", strings.Join(required, ", "))
}

func allowedClientIDs(rt runtimeContext) []string {
	return stringSliceFromAny(rt.Scope["allowed_client_ids"])
}

func activeClientID(rt runtimeContext) string {
	if fmt.Sprintf("%v", rt.ActiveEntity["type"]) != "client" {
		return ""
	}
	if v, ok := rt.ActiveEntity["id"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func canonicalEntityType(value string) string {
	switch normalizeQuestion(value) {
	case "", "client", "cliente", "customer", "deudor", "debtor", "portfolio_client":
		return "client"
	default:
		return strings.TrimSpace(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringSliceFromAny(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := []string{}
		for _, item := range x {
			s := fmt.Sprintf("%v", item)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// generateSQL le pide al LLM la SQL SELECT.
// Si previousSQL/previousErr no son vacíos, los incluye como feedback.
// history aporta contexto conversacional para resolver referencias como
// "los 2 primeros", "y los inactivos?", etc.
func generateSQL(ctx context.Context, c *llm.Client, schema, question string, history []HistoryTurn, previousSQL string, previousErr error, rt runtimeContext) (string, error) {
	semanticContext := semanticContextForPrompt(rt)
	businessContext := businessContextForPrompt(rt)
	baseSystem := `Sos un experto en SQLite. Te dan el schema de una base con datos de un sistema de timebilling para estudios jurídicos, y una pregunta del usuario en español.

Tu tarea: generar UNA sola sentencia SELECT (o WITH ... SELECT) en sintaxis SQLite que responda la pregunta usando los datos reales de la base.

REGLAS ESTRICTAS:
- Devolvé SOLO la SQL, sin markdown, sin backticks, sin explicaciones.
- UNA sola sentencia. Sin punto y coma final. Sin múltiples queries.
- Solo SELECT. Nunca INSERT/UPDATE/DELETE/DROP/ALTER/PRAGMA/ATTACH.
- Quoteá tablas y columnas con dobles comillas: "clients", "billing_documents".
- Todos los valores son TEXT en SQLite. Para comparar números: CAST("col" AS REAL) o CAST("col" AS INTEGER).
- Para fechas: las columnas son strings tipo "2024-01-15" o "2024-01-15 14:30:00". Usá comparación de strings o date()/datetime().
- Para preguntas de cantidad usá COUNT(*). Para sumas SUM(CAST(...)). Para top-N usá ORDER BY ... LIMIT.
- Si la pregunta requiere joins, usá JOIN explícito con ON y SOLO con relaciones reales de esta DB.
- Antes de inferir una relación desde nombres de columnas, consultá el CATÁLOGO SEMÁNTICO. Las relaciones confirmadas y limitaciones del catálogo mandan sobre cualquier inferencia.
- Si existe en el Schema una vista vw_* que responde la pregunta, preferila por sobre tablas raw. Si una vista solo aparece en las DEFINICIONES DE VISTAS SEMÁNTICAS pero no en el Schema, NO la consultes por nombre: usá su SQL equivalente con tablas raw.
- Relaciones reales principales:
    * "clients"."id" = "projects"."client_id"
    * "clients"."id" = "agreements"."client_id"
    * "clients"."id" = "charges"."client_id"
    * "clients"."id" = "billing_documents"."client_id"
    * "clients"."id" = "payments"."client_id"
    * "clients"."id" = "expenses"."client_id"
    * "agreements"."id" = "projects"."agreement_id"
    * "agreements"."id" = "charges"."agreement_id"
    * "agreements"."id" = "milestones"."agreement_id"
    * "charges"."id" = "billing_documents"."charge_id"
    * "charges"."id" = "milestones"."charge_id"
    * "projects"."id" = "expenses"."project_id"
    * "projects"."code" = "time_entries"."project_code"
    * "users"."id" = "time_entries"."user_id"
    * "currencies"."id" = "payments"."currency_id"
    * "currencies"."code" = "projects"."currency_code"
    * "project_areas"."id" = "projects"."project_area_id"
    * "project_types"."id" = "projects"."project_type_id"
    * "user_areas"."id" = "users"."user_area_id"
    * "user_categories"."id" = "users"."user_category_id"
- Relaciones inexistentes que NO debes usar:
    * NO existe "billing_documents"."project_id"
    * NO existe "payments"."billing_document_id"
    * NO existe relación visible desde "law_firms" hacia clientes/proyectos/casos; "law_firms" solo tiene id, name, has_metadata.
- Para "activos" usá ' "active" = "1" '. Para "dados de baja"/"inactivos" usá ' "active" = "0" '.
- Si la pregunta es ambigua respecto a qué entidad, elegí la interpretación más útil y completa.
- LIMIT defensivo: si la query devuelve filas (no agregados), poné LIMIT 100.
- Si la pregunta tiene VARIAS sub-preguntas independientes ("cuántos X y cuántos Y"), NO uses JOIN. Usá subqueries escalares en una sola sentencia:
    SELECT
      (SELECT COUNT(*) FROM "clients" WHERE "active" = '1') AS clientes_activos,
      (SELECT COUNT(*) FROM "payments") AS total_pagos
- En JOINs, SIEMPRE prefijá las columnas con el alias o nombre de tabla (ej "c"."id", "p"."id") para evitar errores de "ambiguous column name".
- Antes de hacer un JOIN entre dos tablas, mirá el schema: solo usá la columna FK que existe (ej si projects tiene "client_id", entonces JOIN ON "projects"."client_id" = "clients"."id").
- BÚSQUEDA POR NOMBRE: cuando el usuario menciona un nombre (cliente, proyecto, persona, etc.), NUNCA uses '=' exacto. Usá SIEMPRE LIKE con wildcards y comparación case-insensitive: lower("name") LIKE lower('%texto%'). Los usuarios suelen escribir solo parte del nombre o con acentos distintos.
- Si la pregunta menciona un valor (ej "cliente Tillman"), no asumas que el match es exacto — el nombre real puede ser "Tillman-Crona" o similar.
- Si el usuario pide "más información" de una entidad, NO inventes campos como dirección/teléfono/fecha si esa tabla no los tiene. Primero consulta todas las columnas reales de esa entidad y, si no hay relaciones reales, dilo claramente.
- HISTORIAL: si el usuario hace una pregunta de seguimiento ("y los inactivos?", "cuáles son?", "los 2 primeros", "y en 2023?"), MIRÁ el historial conversacional para entender a qué se refiere y reformulá la query SOBRE ESA MISMA ENTIDAD/FILTRO. Ejemplo: si antes preguntaste "cuántos clientes activos" y ahora dice "cuáles son los 2 primeros", devolvé SELECT "name" FROM "clients" WHERE "active"='1' ORDER BY "name" LIMIT 2. NUNCA repitas el COUNT anterior.
- ALIAS SEMÁNTICOS OBLIGATORIOS: nombrá las columnas devueltas con sufijos que indiquen su unidad para que el lenguaje natural no se confunda:
    * Conteos: usá sufijo _count o _total_n. Ej: COUNT(*) AS clientes_count, COUNT(*) AS pagos_total_n.
    * Sumas de dinero: sufijo _amount con la moneda si se conoce. Ej: SUM(CAST("amount" AS REAL)) AS pagos_amount.
    * Días de mora: sufijo _dias. Ej: CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias.
    * Sumas de horas: sufijo _hours. Ej: SUM(CAST("duration" AS REAL)) AS trabajo_hours.
    * Listados de nombres: alias claro como nombre, cliente, proyecto.
  Esto es CRÍTICO: el phraser lee estos nombres y decide si poner $ o no.

CÁLCULO DE DÍAS DE MORA — REGLA CRÍTICA:
- Para calcular días de mora SIEMPRE usá aritmética de fechas de SQLite:
    CAST((julianday('now') - julianday("date")) AS INTEGER)
- "date" debe ser una columna de tipo fecha ('YYYY-MM-DD'), NUNCA "amount" ni ningún campo numérico monetario.
- NUNCA uses strftime('%s',...) para calcular diferencia de días (produce segundos, no días).
- NUNCA uses CAST("date" AS INTEGER) (produce solo el año, ej: 2015).
- Ejemplo correcto: CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias

CÁLCULO DE SALDO PENDIENTE DE COBRANZA — REGLA CRÍTICA:
- El saldo pendiente real de un cliente se calcula sumando milestones de sus charges en estado impago:
    SELECT c."name",
           COALESCE(SUM(CAST(m."amount" AS REAL)), 0) AS saldo_amount,
           CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias,
           COUNT(DISTINCT ch."id") AS facturas_count
    FROM "clients" c
    JOIN "charges" ch ON ch."client_id" = c."id"
    LEFT JOIN "milestones" m ON m."charge_id" = ch."id"
    WHERE ch."state" IN ('FACTURADO','EMITIDO','PAGO PARCIAL','ENVIADO AL CLIENTE','EN REVISION')
      AND m."amount" IS NOT NULL AND m."amount" != ''
      AND lower(c."name") LIKE lower('%nombre%')
    GROUP BY c."id"
- NUNCA confundas saldo_amount con mora_dias. Son columnas distintas con significado distinto.
- mora_dias es un número de días (ej: 400), saldo_amount es un monto monetario (ej: 22611.60).

EJEMPLOS:
- "cuántos clientes" → SELECT COUNT(*) AS total FROM "clients"
- "cuántos clientes activos y cuántos proyectos" → SELECT (SELECT COUNT(*) FROM "clients" WHERE "active"='1') AS activos, (SELECT COUNT(*) FROM "projects") AS proyectos
- "top 5 clientes por proyectos" → SELECT c."name", COUNT(p."id") AS n FROM "clients" c JOIN "projects" p ON p."client_id" = c."id" GROUP BY c."id" ORDER BY n DESC LIMIT 5
- "cuántas facturas tiene el cliente Tillman" → SELECT COUNT(*) FROM "billing_documents" bd JOIN "clients" c ON bd."client_id" = c."id" WHERE lower(c."name") LIKE lower('%Tillman%')
- "pagos del año 2024" → SELECT COUNT(*) FROM "payments" WHERE "date" LIKE '2024-%'
- "monto total de pagos en 2022" → SELECT SUM(CAST("amount" AS REAL)) FROM "payments" WHERE "date" LIKE '2022-%'
- "análisis 360° de Thiel-Effertz" → usar el patrón de CÁLCULO DE SALDO PENDIENTE con nombre '%Thiel%'
- "generar email de cobranza" o "email para [cliente]" → (1) extraé el nombre del cliente del historial conversacional, (2) aplicá el patrón de CÁLCULO DE SALDO PENDIENTE exacto para ese cliente (charges+milestones, NO billing_documents.amount que es distinto), (3) devolvé: nombre, saldo_amount, mora_dias, facturas_count.
- NUNCA uses "billing_documents"."amount" como saldo de cobranza — ese campo es el monto facturado, no el saldo pendiente de milestones. El saldo real siempre viene de SUM(milestones.amount) filtrado por charges.state impago.

CATÁLOGO SEMÁNTICO Y VISTAS CANÓNICAS:
` + semanticContext + `

CONTEXTO DE NEGOCIO, AUDIENCIA Y SCOPE:
` + businessContext + `

REGLA HARD DE SCOPE:
- Si el contexto runtime trae active_entity.type="client", toda consulta de cartera debe limitarse a ese cliente.
- Si el contexto runtime trae scope.allowed_client_ids, toda consulta de cartera debe limitarse a esos clients.id/client_id.
- Para tablas con client_id ("charges", "payments", "billing_documents", "projects", "agreements", "expenses") agrega WHERE/AND "<alias>"."client_id" IN (...).
- Para "clients" agrega WHERE/AND "<alias>"."id" IN (...).
- Para "milestones" une con "charges" y filtra charges.client_id.
- Para "time_entries" une con "projects" y filtra projects.client_id.
- Si no puedes aplicar el scope, devuelve una SELECT vacía con alias error_scope para que el sistema no exponga datos fuera de scope.

Schema:
` + schema

	system := systemPromptWithOverlay(baseSystem, "sabio")

	historyBlock := formatHistoryForPrompt(history)
	userPrompt := question
	if historyBlock != "" {
		userPrompt = fmt.Sprintf("Historial conversacional reciente (último primero abajo):\n%s\n\nNueva pregunta del usuario: %s", historyBlock, question)
	}
	if previousSQL != "" && previousErr != nil {
		userPrompt = fmt.Sprintf(`%s

Tu intento anterior:
%s

Falló con este error:
%v

Corregí la SQL y devolvé solo la nueva sentencia.`, userPrompt, previousSQL, previousErr)
	}

	resp, err := c.Generate(ctx, system, userPrompt)
	if err != nil {
		return "", err
	}
	return cleanSQLResponse(resp), nil
}

// cleanSQLResponse remueve markdown fences, prefijos y sufijos comunes
// que algunos modelos agregan a pesar de las reglas.
func cleanSQLResponse(s string) string {
	s = strings.TrimSpace(s)
	// Quitar fences ```sql ... ``` o ```...```
	if strings.HasPrefix(s, "```") {
		// Quitar primera línea (con o sin lenguaje)
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	// Algunos modelos prefijan con "SQL:" o similar.
	for _, p := range []string{"sql:", "SQL:", "Query:", "query:"} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

func semanticContextForPrompt(rt runtimeContext) string {
	var parts []string
	pack, _ := loadBusinessPack(rt.BusinessID)
	catalogPath := defaultCatalog
	viewsPath := defaultViewsSQL
	if pack != nil {
		if v, ok := packDataSourceString(pack, "semantic_catalog"); ok {
			catalogPath = v
		}
		if v, ok := packDataSourceString(pack, "semantic_views"); ok {
			viewsPath = v
		}
	}

	if text := readOptionalPromptFile(envOr("SABIO_SEMANTIC_CATALOG", catalogPath)); text != "" {
		parts = append(parts, "CATÁLOGO SEMÁNTICO CURADO (JSON):\n"+text)
	}
	if text := readOptionalPromptFile(envOr("SABIO_SEMANTIC_VIEWS", viewsPath)); text != "" {
		parts = append(parts, "DEFINICIONES DE VISTAS SEMÁNTICAS (SQL CANÓNICO):\n"+text)
	}
	if len(parts) == 0 {
		return "(sin catálogo semántico externo cargado)"
	}
	return strings.Join(parts, "\n\n")
}

func packDataSourceString(pack *businessPack, key string) (string, bool) {
	data, err := os.ReadFile(resolveSabioPath(filepath.Join("businesses", pack.BusinessID, "sabio.business.json")))
	if err != nil {
		return "", false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}
	ds, ok := raw["data_source"].(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := ds[key].(string)
	return v, ok && v != ""
}

func readOptionalPromptFile(path string) string {
	data, err := os.ReadFile(resolveSabioPath(path))
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if len(text) > 12000 {
		text = text[:12000] + "\n... (truncado)"
	}
	return text
}

func loadRuntimeContext(flagBusinessID, contextB64 string) runtimeContext {
	rt := runtimeContext{
		BusinessID: envOr("SABIO_BUSINESS_ID", defaultBusiness),
		Audience:   "",
	}
	if flagBusinessID != "" {
		rt.BusinessID = flagBusinessID
	}
	if contextB64 == "" {
		contextB64 = os.Getenv("SABIO_CONTEXT_B64")
	}
	if contextB64 != "" {
		if data, err := base64.RawURLEncoding.DecodeString(contextB64); err == nil {
			_ = json.Unmarshal(data, &rt)
		}
	}
	if rt.BusinessID == "" {
		rt.BusinessID = envOr("SABIO_BUSINESS_ID", defaultBusiness)
	}
	if flagBusinessID != "" {
		rt.BusinessID = flagBusinessID
	}
	if rt.Audience == "" {
		if pack, err := loadBusinessPack(rt.BusinessID); err == nil && pack.DefaultAudience != "" {
			rt.Audience = pack.DefaultAudience
		}
	}
	return rt
}

func loadBusinessPack(businessID string) (*businessPack, error) {
	if businessID == "" {
		businessID = defaultBusiness
	}
	path := resolveSabioPath(filepath.Join("businesses", businessID, "sabio.business.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leer business pack %s: %w", path, err)
	}
	var pack businessPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return nil, fmt.Errorf("parse business pack %s: %w", path, err)
	}
	if pack.BusinessID == "" {
		pack.BusinessID = businessID
	}
	return &pack, nil
}

func resolveSabioPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	for _, prefix := range []string{"..", "../.."} {
		candidate := filepath.Join(prefix, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return path
	}
	candidate := filepath.Join(filepath.Dir(exe), path)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return path
}

func businessContextForPrompt(rt runtimeContext) string {
	pack, err := loadBusinessPack(rt.BusinessID)
	if err != nil {
		return fmt.Sprintf("(sin business pack cargado: %v)", err)
	}
	audience := rt.Audience
	if audience == "" {
		audience = pack.DefaultAudience
	}
	aud := pack.Audiences[audience]

	var sb strings.Builder
	fmt.Fprintf(&sb, "Negocio: %s (%s)\n", pack.Name, pack.BusinessID)
	fmt.Fprintf(&sb, "Dominio: %s\n", pack.Domain)
	fmt.Fprintf(&sb, "WHY de Sabio para este negocio: %s\n", pack.Why)
	fmt.Fprintf(&sb, "Audiencia actual: %s - %s\n", audience, aud.Label)
	if aud.Description != "" {
		fmt.Fprintf(&sb, "Receptor: %s\n", aud.Description)
	}
	if aud.AnswerStyle != "" {
		fmt.Fprintf(&sb, "Estilo de respuesta: %s\n", aud.AnswerStyle)
	}
	if len(aud.AllowedModes) > 0 {
		fmt.Fprintf(&sb, "Modos permitidos: %s\n", strings.Join(aud.AllowedModes, ", "))
	}
	if len(pack.JobsToBeDone) > 0 {
		sb.WriteString("Trabajos que Sabio debe ayudar a hacer:\n")
		for _, job := range pack.JobsToBeDone {
			fmt.Fprintf(&sb, "- %s\n", job)
		}
	}
	if len(pack.AnswerPolicies) > 0 {
		sb.WriteString("Políticas de respuesta:\n")
		keys := sortedMapKeys(pack.AnswerPolicies)
		for _, k := range keys {
			fmt.Fprintf(&sb, "- %s: %s\n", k, pack.AnswerPolicies[k])
		}
	}
	if len(pack.ForbiddenClaims) > 0 {
		sb.WriteString("Afirmaciones prohibidas:\n")
		for _, claim := range pack.ForbiddenClaims {
			fmt.Fprintf(&sb, "- %s\n", claim)
		}
	}
	if len(rt.Scope) > 0 || len(rt.ActiveEntity) > 0 {
		payload, _ := json.MarshalIndent(map[string]any{
			"scope":         rt.Scope,
			"active_entity": rt.ActiveEntity,
		}, "", "  ")
		sb.WriteString("Contexto runtime de sesión (debe respetarse):\n")
		sb.Write(payload)
		sb.WriteString("\n")
	}
	if !aud.CanSeeSQL {
		sb.WriteString("La audiencia actual NO debe ver SQL ni nombres técnicos salvo en evidencia interna; responder en lenguaje de negocio.\n")
	}
	return sb.String()
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildCapabilitiesExplanation(pack *businessPack, rt runtimeContext) map[string]any {
	audience := rt.Audience
	if audience == "" {
		audience = pack.DefaultAudience
	}
	aud := pack.Audiences[audience]
	questions := []string{}
	for _, mode := range aud.AllowedModes {
		questions = append(questions, pack.SuggestedQuestions[mode]...)
	}
	if len(questions) == 0 {
		for _, qs := range pack.SuggestedQuestions {
			questions = append(questions, qs...)
		}
	}
	return map[string]any{
		"business_id":         pack.BusinessID,
		"name":                pack.Name,
		"domain":              pack.Domain,
		"why":                 pack.Why,
		"audience":            audience,
		"audience_label":      aud.Label,
		"allowed_modes":       aud.AllowedModes,
		"can_see_sql":         aud.CanSeeSQL,
		"jobs_to_be_done":     pack.JobsToBeDone,
		"suggested_questions": questions,
		"canonical_views":     pack.CanonicalViews,
		"forbidden_claims":    pack.ForbiddenClaims,
		"scope":               rt.Scope,
		"active_entity":       rt.ActiveEntity,
	}
}

func buildDBProfile(dbPath string) (*dbProfile, error) {
	dsn := dbPath + "?_pragma=query_only(true)&mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profile := &dbProfile{Source: dbPath, Tables: map[string]dbProfileTable{}}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	for _, t := range tables {
		table, err := profileTable(db, t)
		if err != nil {
			return nil, err
		}
		profile.Tables[t] = table
	}
	profile.TableCount = len(profile.Tables)
	profile.Relationships = inferProfileRelationships(profile)
	return profile, nil
}

func profileTable(db *sql.DB, table string) (dbProfileTable, error) {
	out := dbProfileTable{}
	_ = db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table)).Scan(&out.RowCount)
	colRows, err := db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		return out, err
	}
	for colRows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := colRows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			colRows.Close()
			return out, err
		}
		if typ == "" {
			typ = "TEXT"
		}
		out.Columns = append(out.Columns, dbProfileColumn{Name: name, Type: typ, NotNull: notnull != 0, PK: pk != 0})
	}
	colRows.Close()

	rows, err := db.Query(fmt.Sprintf(`SELECT * FROM "%s" LIMIT 3`, table))
	if err != nil {
		return out, nil
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if rows.Scan(ptrs...) != nil {
			continue
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = normalizeDBValue(vals[i])
		}
		out.SampleRows = append(out.SampleRows, row)
	}
	return out, nil
}

func normalizeDBValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func inferProfileRelationships(profile *dbProfile) []dbRelationship {
	tableExists := func(t string) bool {
		_, ok := profile.Tables[t]
		return ok
	}
	columnExists := func(t, c string) bool {
		table, ok := profile.Tables[t]
		if !ok {
			return false
		}
		for _, col := range table.Columns {
			if col.Name == c {
				return true
			}
		}
		return false
	}
	candidates := []dbRelationship{
		{"advances.client_id", "clients.id", "high", "column name + entity table exists"},
		{"advances.project_id", "projects.id", "high", "column name + entity table exists"},
		{"agreements.client_id", "clients.id", "high", "known API semantics"},
		{"bank_accounts.bank_id", "banks.id", "high", "column name + entity table exists"},
		{"bank_accounts.currency_id", "currencies.id", "high", "column name + entity table exists"},
		{"billing_documents.charge_id", "charges.id", "high", "known API semantics"},
		{"billing_documents.client_id", "clients.id", "high", "known API semantics"},
		{"clients.group_id", "client_groups.id", "medium", "column name; client_groups also has client_id"},
		{"client_groups.client_id", "clients.id", "medium", "column name"},
		{"client_groups.country_id", "countries.id", "high", "column name + entity table exists"},
		{"charges.agreement_id", "agreements.id", "high", "known API semantics"},
		{"charges.client_id", "clients.id", "high", "known API semantics"},
		{"expenses.client_id", "clients.id", "high", "known API semantics"},
		{"expenses.project_id", "projects.id", "high", "known API semantics"},
		{"milestones.agreement_id", "agreements.id", "high", "known API semantics"},
		{"milestones.charge_id", "charges.id", "high", "known API semantics"},
		{"payments.client_id", "clients.id", "high", "known API semantics"},
		{"payments.currency_id", "currencies.id", "high", "known API semantics"},
		{"projects.agreement_id", "agreements.id", "high", "known API semantics"},
		{"projects.client_id", "clients.id", "high", "known API semantics"},
		{"projects.currency_code", "currencies.code", "high", "known API semantics"},
		{"projects.language_code", "languages.code", "medium", "column name + language code table"},
		{"projects.project_area_id", "project_areas.id", "high", "known API semantics"},
		{"projects.project_type_id", "project_types.id", "high", "known API semantics"},
		{"time_entries.project_code", "projects.code", "high", "known API semantics"},
		{"time_entries.user_id", "users.id", "high", "known API semantics"},
		{"user_areas.related_id", "areas.id", "low", "ambiguous related_id"},
		{"users.user_area_id", "user_areas.id", "high", "known API semantics"},
		{"users.user_category_id", "user_categories.id", "high", "known API semantics"},
	}
	out := []dbRelationship{}
	for _, rel := range candidates {
		ft, fc, ok1 := splitRef(rel.From)
		tt, tc, ok2 := splitRef(rel.To)
		if ok1 && ok2 && tableExists(ft) && tableExists(tt) && columnExists(ft, fc) && columnExists(tt, tc) {
			out = append(out, rel)
		}
	}
	return out
}

func splitRef(ref string) (string, string, bool) {
	parts := strings.Split(ref, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func validateBusinessConfig(businessID, dbPath string) map[string]any {
	result := map[string]any{
		"business_id": businessID,
		"ok":          true,
		"errors":      []string{},
		"warnings":    []string{},
	}
	addErr := func(format string, args ...any) {
		result["ok"] = false
		result["errors"] = append(result["errors"].([]string), fmt.Sprintf(format, args...))
	}
	addWarn := func(format string, args ...any) {
		result["warnings"] = append(result["warnings"].([]string), fmt.Sprintf(format, args...))
	}
	pack, err := loadBusinessPack(businessID)
	if err != nil {
		addErr("%v", err)
		return result
	}
	profile, err := buildDBProfile(dbPath)
	if err != nil {
		addErr("profile db: %v", err)
		return result
	}
	if pack.Why == "" {
		addErr("why vacío")
	}
	if pack.DefaultAudience == "" {
		addErr("default_audience vacío")
	} else if _, ok := pack.Audiences[pack.DefaultAudience]; !ok {
		addErr("default_audience %q no existe en audiences", pack.DefaultAudience)
	}
	for name, entity := range pack.PrimaryEntities {
		if _, ok := profile.Tables[entity.Table]; !ok {
			addErr("primary_entities.%s referencia tabla inexistente %q", name, entity.Table)
			continue
		}
		if entity.ScopeKey != "" && !profileHasColumn(profile, entity.Table, entity.ScopeKey) {
			addErr("primary_entities.%s scope_key inexistente: %s.%s", name, entity.Table, entity.ScopeKey)
		}
		if entity.ScopeColumn != "" && !profileHasColumn(profile, entity.Table, entity.ScopeColumn) {
			addErr("primary_entities.%s scope_column inexistente: %s.%s", name, entity.Table, entity.ScopeColumn)
		}
	}
	for table, policy := range pack.ScopePolicies.Tables {
		if _, ok := profile.Tables[table]; !ok {
			addErr("scope_policies.tables.%s no existe en DB", table)
			continue
		}
		if policy.ScopeColumn != "" && !profileHasColumn(profile, table, policy.ScopeColumn) {
			addErr("scope_policies.tables.%s scope_column inexistente: %s", table, policy.ScopeColumn)
		}
		if policy.ScopeColumn == "" && policy.JoinToScope == "" {
			addWarn("scope_policies.tables.%s no define scope_column ni join_to_scope", table)
		}
	}
	for _, view := range pack.CanonicalViews {
		if !strings.HasPrefix(view, "vw_") {
			addWarn("canonical view %q no empieza con vw_", view)
		}
	}
	result["table_count"] = profile.TableCount
	result["relationship_count"] = len(profile.Relationships)
	return result
}

func profileHasColumn(profile *dbProfile, table, column string) bool {
	t, ok := profile.Tables[table]
	if !ok {
		return false
	}
	for _, col := range t.Columns {
		if col.Name == column {
			return true
		}
	}
	return false
}

// phraseSQLAnswer pide al LLM que arme la respuesta natural a partir
// de las filas devueltas por la SQL. history aporta el contexto previo
// para que el phraser pueda continuar el hilo (ej: el usuario dijo "los 2
// primeros" → la respuesta debe nombrarlos, no decir "hay 2").
func phraseSQLAnswer(ctx context.Context, c *llm.Client, question string, qr *sqlqa.QueryResult, history []HistoryTurn, rt runtimeContext) (string, error) {
	businessContext := businessContextForPrompt(rt)
	baseSystem := `Sos Sabio, un asistente que responde en español rioplatense natural sobre los datos de negocio del usuario (estudio jurídico, sistema de timebilling).

El usuario NO es técnico (gerente, abogado, contador). NO conoce JSON, IDs, ni nombres de tablas.

PROHIBIDO:
- Nombres de tablas o columnas en inglés ("billing_documents", "client_id", "active", "id", "code").
- Citas técnicas o IDs internos sueltos. Solo se permiten códigos formateados de negocio (ej "000239-0001").
- Mostrar la SQL al usuario. Solo el resultado en lenguaje humano.
- INVENTAR UNIDADES O MONEDAS. Si los datos no traen una moneda explícita, NO pongas "$", "USD", "€", "ARS", etc.

INTERPRETACIÓN DE COLUMNAS (CRÍTICO PARA NO ALUCINAR UNIDADES):
- Columna terminada en _count, _total_n, _qty, _n, o que es un COUNT(*) → CANTIDAD ENTERA. NUNCA agregues símbolo de moneda. Ej: "pagos_count": 1451 → "hay 1451 pagos" (NO "$1451").
- Columna terminada en _amount, _monto, _total_amount, _sum → MONTO DE DINERO. Solo agregá moneda si en los datos hay una columna como currency, currency_code o si la pregunta lo deja claro. Si no, decí el número sin símbolo.
- Columna terminada en _dias, _days → DÍAS ENTEROS. Decí "X días". NUNCA lo interpretes como monto de dinero.
- Columna terminada en _hours, _horas → HORAS. Decí "X horas".
- Columna terminada en _date, _at o que se ve como fecha → FECHA en DD/MM/AAAA.
- Si la columna se llama solo "name", "id", "code", o un alias semántico claro ("cliente", "proyecto") → es un identificador o nombre, decílo como tal.
- Si la SQL tiene COUNT(*) sin alias, igual es CANTIDAD, nunca dinero.

OBLIGATORIO:
- Traducí los conceptos: facturas, pagos, anticipos, gastos, horas, tarifas, contratos, clientes, proyectos, áreas.
- Estados: "active=0" → "dado de baja". "active=1" → "activo". "residue=0" → "saldado".
- Fechas en DD/MM/AAAA.
- Frases cortas, directas. Sin preámbulos. Si hay 3+ items comparables usá viñetas.
- No uses formato operativo ("Para hoy", "Hacé ahora", instrucciones de acción) salvo que el usuario pida explícitamente priorizar o decidir una acción.
- Si la respuesta es un único número (COUNT, SUM), decílo en una frase clara con la unidad correcta según el alias.
- Si las filas están vacías, decí honestamente "no encontré ningún registro que coincida".
- USÁ EL HISTORIAL: si el usuario hizo una pregunta de seguimiento ("cuáles son?", "y los 2 primeros?"), respondé concretamente con los datos pedidos, NO repitas el conteo o respuesta del turno anterior.
- Respetá el CONTEXTO DE NEGOCIO, AUDIENCIA Y SCOPE. Si la audiencia no puede ver SQL, no menciones SQL ni tablas crudas en la respuesta natural.

Vas a recibir:
1. El historial de la conversación reciente (puede estar vacío).
2. La pregunta actual del usuario.
3. La SQL ejecutada (solo para que entiendas qué representa cada columna; NO la muestres al usuario).
4. Las filas devueltas.

CONTEXTO DE NEGOCIO, AUDIENCIA Y SCOPE:
` + businessContext + `

Tu salida: solo la respuesta natural al usuario. Nada más.`

	system := systemPromptWithOverlay(baseSystem, "sabio")

	historyBlock := formatHistoryForPrompt(history)
	if historyBlock == "" {
		historyBlock = "(sin historial previo)"
	}

	userPrompt := fmt.Sprintf(`Historial conversacional reciente:
%s

Pregunta actual del usuario:
%s

SQL ejecutada (uso interno, NO mostrar al usuario, sirve para entender alias/unidades):
%s

Resultado (%d filas%s):
%s

Respondé en lenguaje natural al usuario aplicando las reglas de unidades.`,
		historyBlock,
		question,
		qr.SQL,
		qr.RowCount,
		func() string {
			if qr.Truncated {
				return ", se muestran los primeros"
			}
			return ""
		}(),
		sqlqa.FormatRowsForPrompt(qr),
	)

	return c.Generate(ctx, system, userPrompt)
}

// formatHistoryForPrompt arma el bloque de historial conversacional
// para inyectar en los prompts. Tomamos hasta 6 turnos recientes y los
// presentamos en orden cronológico.
func formatHistoryForPrompt(history []HistoryTurn) string {
	if len(history) == 0 {
		return ""
	}
	const maxTurns = 6
	start := 0
	if len(history) > maxTurns {
		start = len(history) - maxTurns
	}
	var sb strings.Builder
	for _, t := range history[start:] {
		role := t.Role
		switch role {
		case "user":
			role = "Usuario"
		case "framework", "sabio", "assistant":
			role = "Sabio"
		}
		content := strings.TrimSpace(t.Content)
		if len(content) > 800 {
			content = content[:800] + "…"
		}
		fmt.Fprintf(&sb, "%s: %s\n", role, content)
	}
	return strings.TrimSpace(sb.String())
}

// decodeHistory toma el flag --history (base64-url-safe de un JSON
// [{role,content}, ...]) y lo decodifica de forma defensiva: si está
// vacío o falla el parse, devuelve nil sin romper el flujo.
func decodeHistory(s string) []HistoryTurn {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		// fallback: probar base64 estándar por si el orquestador no usó url-safe
		if raw2, err2 := base64.StdEncoding.DecodeString(s); err2 == nil {
			raw = raw2
		} else {
			fmt.Fprintf(os.Stderr, "[sabio] decodeHistory: base64 decode falló: %v\n", err)
			return nil
		}
	}
	var out []HistoryTurn
	if err := json.Unmarshal(raw, &out); err != nil {
		fmt.Fprintf(os.Stderr, "[sabio] decodeHistory: json unmarshal falló: %v\n", err)
		return nil
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func emitJSON(v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

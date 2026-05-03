// frameworkcontactos: gestor canónico de contactos por entidad y canal.
//
// Mantiene una tabla estándar SQLite por perfil:
//   profiles/<profile>/contacts.db
//   contacts(entity_type, entity_ref, channel, value, source, verified_at, created_at)
//
// Comandos: lookup, store, list-missing, import-csv (+ next-question/ingest-answer
// no-op para satisfacer el contrato conversacional del Channel).
//
// Output siempre JSON en stdout. Errores fatales en stderr con exit 1.
package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: frameworkcontactos <comando> [flags]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "next-question":
		fmt.Println(`{}`)
	case "ingest-answer":
		fmt.Println(`{"ok":true}`)
	case "lookup":
		cmdLookup(args)
	case "store":
		cmdStore(args)
	case "list-missing":
		cmdListMissing(args)
	case "import-csv":
		cmdImportCSV(args)
	case "init":
		cmdInit(args)
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", cmd)
		os.Exit(2)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// resolveDBPath devuelve la ruta del SQLite de contactos para el perfil.
// Por defecto: <REMORA_ROOT>/profiles/<profile>/contacts.db
// Se puede sobreescribir con CONTACTS_DB_PATH (test/dev).
func resolveDBPath(profile string) string {
	if v := os.Getenv("CONTACTS_DB_PATH"); v != "" {
		return v
	}
	if profile == "" {
		profile = envOr("REMORA_PROFILE", "default")
	}
	root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "."))
	return filepath.Join(root, "profiles", profile, "contacts.db")
}

func envOr(k, fb string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fb
}

// openDB abre (creando si hace falta) la SQLite del perfil y aplica el schema.
func openDB(profile string) (*sql.DB, error) {
	path := resolveDBPath(profile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir contacts dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

const schemaSQL = `
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

func writeJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(v)
}

func writeErr(msg string) {
	writeJSON(map[string]interface{}{"success": false, "error": msg})
}

// ─── Commands ─────────────────────────────────────────────────────────────

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	profile := fs.String("profile", "", "perfil")
	_ = fs.Parse(args)
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	writeJSON(map[string]interface{}{
		"success": true,
		"db_path": resolveDBPath(*profile),
	})
}

func cmdLookup(args []string) {
	fs := flag.NewFlagSet("lookup", flag.ExitOnError)
	profile := fs.String("profile", "", "perfil")
	entityType := fs.String("entity-type", "", "ej: client, provider")
	entityRef := fs.String("entity-ref", "", "id o code del ERP")
	channel := fs.String("channel", "email", "email|phone|whatsapp|...")
	_ = fs.Parse(args)
	if *entityType == "" || *entityRef == "" {
		writeJSON(map[string]interface{}{
			"found":               false,
			"missing_capability":  "contact." + *channel,
			"provider_hint":       "contactos",
			"error":               "entity_type y entity_ref son requeridos",
		})
		return
	}
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Preferimos verified_at más reciente; si no hay verified, el más nuevo por created_at.
	row := db.QueryRow(`
		SELECT value, source, COALESCE(verified_at, '') FROM contacts
		WHERE entity_type = ? AND entity_ref = ? AND channel = ?
		ORDER BY (verified_at IS NULL), verified_at DESC, created_at DESC
		LIMIT 1`,
		*entityType, *entityRef, *channel)
	var value, source, verifiedAt string
	if err := row.Scan(&value, &source, &verifiedAt); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(map[string]interface{}{
				"found":              false,
				"missing_capability": "contact." + *channel,
				"provider_hint":      "contactos",
				"entity_type":        *entityType,
				"entity_ref":         *entityRef,
				"channel":            *channel,
			})
			return
		}
		writeErr(err.Error())
		os.Exit(1)
	}
	writeJSON(map[string]interface{}{
		"found":       true,
		"value":       value,
		"source":      source,
		"verified_at": verifiedAt,
		"entity_type": *entityType,
		"entity_ref":  *entityRef,
		"channel":     *channel,
	})
}

func cmdStore(args []string) {
	fs := flag.NewFlagSet("store", flag.ExitOnError)
	profile := fs.String("profile", "", "perfil")
	entityType := fs.String("entity-type", "", "")
	entityRef := fs.String("entity-ref", "", "")
	channel := fs.String("channel", "email", "")
	value := fs.String("value", "", "valor (ej. email)")
	source := fs.String("source", "manual", "manual|csv|erp|scraped")
	verified := fs.Bool("verified", false, "marcar como verificado ahora")
	_ = fs.Parse(args)
	if *entityType == "" || *entityRef == "" || *value == "" {
		writeErr("entity_type, entity_ref y value son requeridos")
		os.Exit(1)
	}
	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
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
		writeErr(err.Error())
		os.Exit(1)
	}
	writeJSON(map[string]interface{}{
		"success":     true,
		"entity_type": *entityType,
		"entity_ref":  *entityRef,
		"channel":     *channel,
		"value":       *value,
		"source":      *source,
	})
}

// cmdListMissing lista entidades sin contacto. Para ser genérico sin acoplarse
// al schema del ERP, leemos el set de entidades desde la base de datos del
// perfil (data.db) si está disponible; si no, devolvemos las que están en
// contacts.db pero faltarían en algún canal cruzado (no aplica a este caso).
//
// Implementación pragmática: necesita env CONTACTS_ENTITY_DB y CONTACTS_ENTITY_QUERY
// (configurables por perfil) o usa heurística por entity_type → tabla.
func cmdListMissing(args []string) {
	fs := flag.NewFlagSet("list-missing", flag.ExitOnError)
	profile := fs.String("profile", "", "perfil")
	entityType := fs.String("entity-type", "client", "")
	channel := fs.String("channel", "email", "")
	_ = fs.Parse(args)

	entityDB := envOr("CONTACTS_ENTITY_DB", "")
	entityQuery := envOr("CONTACTS_ENTITY_QUERY", "")
	if entityDB == "" {
		// Fallback heurístico para Panalbit: data.db + tabla = entity_type+'s'
		root := envOr("REMORA_ROOT", envOr("CHANNEL_BASE_DIR", "."))
		guess := filepath.Join(root, "framework-indexa", "data", "panalbit.db")
		if _, err := os.Stat(guess); err == nil {
			entityDB = guess
		}
	}
	if entityDB == "" {
		writeJSON(map[string]interface{}{
			"missing":  []interface{}{},
			"warning":  "no se configuró CONTACTS_ENTITY_DB ni se encontró data.db",
		})
		return
	}
	if entityQuery == "" {
		// Heurística simple: tabla = entity_type + 's' (clients, providers, ...)
		entityQuery = fmt.Sprintf("SELECT id, name FROM %ss ORDER BY name LIMIT 500", *entityType)
	}

	edb, err := sql.Open("sqlite", entityDB)
	if err != nil {
		writeErr("open entity db: " + err.Error())
		os.Exit(1)
	}
	defer edb.Close()
	rows, err := edb.Query(entityQuery)
	if err != nil {
		writeErr("entity query: " + err.Error())
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

	cdb, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer cdb.Close()

	// set de refs que SÍ tienen contacto en este canal
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
	writeJSON(map[string]interface{}{
		"entity_type": *entityType,
		"channel":     *channel,
		"total":       len(all),
		"with":        len(all) - len(missing),
		"missing":     missing,
	})
}

func cmdImportCSV(args []string) {
	fs := flag.NewFlagSet("import-csv", flag.ExitOnError)
	profile := fs.String("profile", "", "perfil")
	file := fs.String("file", "", "ruta al CSV con headers entity_type,entity_ref,channel,value[,source]")
	_ = fs.Parse(args)
	if *file == "" {
		writeErr("--file requerido")
		os.Exit(1)
	}
	f, err := os.Open(*file)
	if err != nil {
		writeErr(err.Error())
		os.Exit(1)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		writeErr("csv vacío o inválido: " + err.Error())
		os.Exit(1)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"entity_type", "entity_ref", "channel", "value"}
	for _, k := range required {
		if _, ok := col[k]; !ok {
			writeErr("CSV falta columna: " + k)
			os.Exit(1)
		}
	}

	db, err := openDB(*profile)
	if err != nil {
		writeErr(err.Error())
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
		writeErr(err.Error())
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
	writeJSON(map[string]interface{}{
		"success":  true,
		"imported": imported,
		"skipped":  skipped,
		"errors":   errs,
	})
}

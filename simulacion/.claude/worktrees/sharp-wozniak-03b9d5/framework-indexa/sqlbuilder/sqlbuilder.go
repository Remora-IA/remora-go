// Package sqlbuilder convierte un dump JSON multi-endpoint a una
// base SQLite con una tabla por endpoint, columnas inferidas y valores
// JSON-encodeados cuando son anidados. Permite que Sabio responda
// preguntas agregadas / con joins generando SQL.
package sqlbuilder

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// BuildOptions controla la generación.
type BuildOptions struct {
	// DryRun no escribe la DB, solo reporta.
	DryRun bool
}

// BuildResult resume lo construido.
type BuildResult struct {
	Path        string
	Tables      []TableStats
	TotalRows   int
}

type TableStats struct {
	Name    string
	Rows    int
	Columns int
}

// BuildFromEndpoints crea (o sobrescribe) una DB SQLite en `dbPath`
// con una tabla por endpoint. Cada record se inserta como una fila;
// columnas son la unión de keys top-level. Valores complejos se
// guardan como TEXT JSON.
//
// `endpointsMap` es el shape ya parseado:  map[endpointName] -> []record
func BuildFromEndpoints(dbPath string, endpointsMap map[string][]map[string]any, opts BuildOptions) (*BuildResult, error) {
	if !opts.DryRun {
		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("mkdir: %w", err)
			}
		}
		// Sobrescribir limpio para evitar drift de schema entre runs.
		_ = os.Remove(dbPath)
	}

	var db *sql.DB
	if !opts.DryRun {
		var err error
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, fmt.Errorf("open db: %w", err)
		}
		defer db.Close()
		// Pragmas para velocidad de carga.
		for _, p := range []string{
			"PRAGMA journal_mode = MEMORY",
			"PRAGMA synchronous = OFF",
			"PRAGMA temp_store = MEMORY",
		} {
			if _, err := db.Exec(p); err != nil {
				return nil, fmt.Errorf("pragma %s: %w", p, err)
			}
		}
	}

	res := &BuildResult{Path: dbPath}

	// Procesamos en orden alfabético para deterministicidad.
	endpointNames := make([]string, 0, len(endpointsMap))
	for n := range endpointsMap {
		endpointNames = append(endpointNames, n)
	}
	sort.Strings(endpointNames)

	for _, ep := range endpointNames {
		records := endpointsMap[ep]
		if len(records) == 0 {
			continue
		}
		stats, err := buildOneTable(db, ep, records, opts.DryRun)
		if err != nil {
			return nil, fmt.Errorf("table %s: %w", ep, err)
		}
		res.Tables = append(res.Tables, stats)
		res.TotalRows += stats.Rows
	}

	return res, nil
}

// sanitizeIdent garantiza un identificador SQL válido (a-z, 0-9, _).
// SQLite acepta más cosas con quoting, pero nos limitamos para que el
// LLM no se confunda generando queries.
func sanitizeIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := b.String()
	if out == "" {
		out = "col"
	}
	// SQLite no permite identificadores que son solo dígitos al inicio;
	// ya cubrimos eso con (i>0 && digit). Si el primer char es _ está OK.
	return strings.ToLower(out)
}

func buildOneTable(db *sql.DB, endpoint string, records []map[string]any, dryRun bool) (TableStats, error) {
	tableName := sanitizeIdent(endpoint)

	// Inferir columnas: unión de keys de todos los records.
	colSet := map[string]struct{}{}
	for _, rec := range records {
		for k := range rec {
			colSet[k] = struct{}{}
		}
	}
	cols := make([]string, 0, len(colSet))
	for c := range colSet {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	// Map original_key -> sql_column_name. Conservamos orden alfabético.
	type colDef struct {
		Original string
		SQLName  string
	}
	colDefs := make([]colDef, 0, len(cols))
	usedNames := map[string]int{}
	for _, c := range cols {
		s := sanitizeIdent(c)
		// Resolución de colisiones por sanitización.
		if usedNames[s] > 0 {
			s = fmt.Sprintf("%s_%d", s, usedNames[s])
		}
		usedNames[sanitizeIdent(c)]++
		colDefs = append(colDefs, colDef{Original: c, SQLName: s})
	}

	stats := TableStats{Name: tableName, Columns: len(colDefs), Rows: 0}

	if dryRun {
		stats.Rows = len(records)
		return stats, nil
	}

	// CREATE TABLE
	var createSB strings.Builder
	createSB.WriteString(fmt.Sprintf(`CREATE TABLE "%s" (`, tableName))
	for i, cd := range colDefs {
		if i > 0 {
			createSB.WriteString(", ")
		}
		// Todo como TEXT para tolerar tipos heterogéneos en la API.
		// SQLite tiene tipado dinámico, así que comparaciones numéricas
		// con CAST funcionan igual. Quoteamos para evitar choque con
		// reserved words (group, order, etc).
		createSB.WriteString(fmt.Sprintf(`"%s" TEXT`, cd.SQLName))
	}
	createSB.WriteString(")")

	if _, err := db.Exec(createSB.String()); err != nil {
		return stats, fmt.Errorf("create table: %w", err)
	}

	// Bulk insert dentro de una transacción.
	tx, err := db.Begin()
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}

	colNames := make([]string, len(colDefs))
	placeholders := make([]string, len(colDefs))
	for i, cd := range colDefs {
		colNames[i] = `"` + cd.SQLName + `"`
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
		tableName,
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "),
	)

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return stats, fmt.Errorf("prepare: %w", err)
	}

	args := make([]any, len(colDefs))
	for _, rec := range records {
		for i, cd := range colDefs {
			args[i] = encodeValue(rec[cd.Original])
		}
		if _, err := stmt.Exec(args...); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return stats, fmt.Errorf("insert: %w", err)
		}
		stats.Rows++
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return stats, fmt.Errorf("close stmt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}

	return stats, nil
}

// encodeValue convierte un valor de JSON a algo que SQLite acepta.
// Strings/ints/floats/bools van como están. Maps/slices se serializan
// a JSON para que la columna preserve la información (consultable con
// json_extract si hace falta).
func encodeValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string, float64, bool, int, int64:
		return x
	case []any, map[string]any:
		b, err := json.Marshal(x)
		if err != nil {
			return nil
		}
		return string(b)
	default:
		// Defensivo: cualquier otro tipo lo serializamos a JSON.
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(b)
	}
}

// SchemaSummary genera una representación compacta del schema para
// pasar al LLM. Devuelve un string como:
//
//	clients(id, code, name, group_id, ...) — 269 rows
//	  example: id="1" name="Gislason Ltd" active="0"
//	projects(id, name, client_id, ...) — 521 rows
//	  example: ...
//
// Limita el número de columnas mostradas y los ejemplos por tabla.
func SchemaSummary(dbPath string) (string, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	// Listar tablas.
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return "", fmt.Errorf("list tables: %w", err)
	}
	tableNames := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return "", err
		}
		tableNames = append(tableNames, n)
	}
	rows.Close()

	var sb strings.Builder
	for _, t := range tableNames {
		// Count
		var count int
		_ = db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, t)).Scan(&count)

		// Columns
		colRows, err := db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, t))
		if err != nil {
			continue
		}
		cols := []string{}
		for colRows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := colRows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				continue
			}
			cols = append(cols, name)
		}
		colRows.Close()

		fmt.Fprintf(&sb, "TABLE %s — %d rows\n", t, count)
		fmt.Fprintf(&sb, "  columns: %s\n", strings.Join(cols, ", "))

		// Una fila de ejemplo.
		exampleRows, err := db.Query(fmt.Sprintf(`SELECT * FROM "%s" LIMIT 1`, t))
		if err == nil {
			cols2, _ := exampleRows.Columns()
			if exampleRows.Next() {
				vals := make([]any, len(cols2))
				ptrs := make([]any, len(cols2))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := exampleRows.Scan(ptrs...); err == nil {
					fmt.Fprintf(&sb, "  example: ")
					parts := []string{}
					for i, c := range cols2 {
						if i >= 8 {
							parts = append(parts, "...")
							break
						}
						v := vals[i]
						if v == nil {
							continue
						}
						s := fmt.Sprintf("%v", v)
						if len(s) > 40 {
							s = s[:40] + "…"
						}
						parts = append(parts, fmt.Sprintf("%s=%q", c, s))
					}
					sb.WriteString(strings.Join(parts, " "))
					sb.WriteString("\n")
				}
			}
			exampleRows.Close()
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

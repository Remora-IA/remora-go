// Package sqlqa implementa el path Text-to-SQL de Sabio:
//
//  1. recibe pregunta del usuario
//  2. con el schema de la DB y la pregunta, le pide al LLM que genere
//     una sentencia SQLite SELECT
//  3. valida que sea solo SELECT (read-only)
//  4. ejecuta con timeout y LIMIT defensivo
//  5. devuelve filas + SQL para que el caller le pida al LLM phrasing natural
//
// Esto resuelve agregaciones, joins y filtros precisos, donde BM25 falla.
package sqlqa

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Engine encapsula la conexión a la DB y el schema cacheado.
type Engine struct {
	db     *sql.DB
	schema string
}

// Open abre la DB en read-only mode. SQLite read-only es seguro y
// permite que sigamos rebuildeando la DB en otro proceso.
func Open(dbPath string) (*Engine, error) {
	// `?_pragma=query_only(true)` para forzar read-only a nivel de connection.
	dsn := dbPath + "?_pragma=query_only(true)&mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	e := &Engine{db: db}
	schema, err := e.buildSchema()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	e.schema = schema
	return e, nil
}

// Close libera la conexión.
func (e *Engine) Close() error {
	if e == nil || e.db == nil {
		return nil
	}
	return e.db.Close()
}

// Schema devuelve el resumen del schema usado en los prompts.
func (e *Engine) Schema() string { return e.schema }

// QueryResult es lo que devolvemos al caller para que arme el prompt
// de phrasing.
type QueryResult struct {
	SQL          string           // SQL ejecutada
	Columns      []string         // nombres de columnas devueltos
	Rows         []map[string]any // hasta MaxRows filas como JSON-friendly
	RowCount     int              // total filas devueltas (puede ser > len(Rows) si truncamos)
	Truncated    bool             // true si tuvimos que limitar
	ExecMillis   int64            // tiempo de ejecución
}

// Defaults conservadores para evitar OOMs en el contenedor.
const (
	MaxRows       = 200
	QueryTimeout  = 10 * time.Second
)

// Run ejecuta la SQL provista (después de validarla) y retorna las filas.
// Aplica LIMIT defensivo si la query no lo trae.
func (e *Engine) Run(ctx context.Context, rawSQL string) (*QueryResult, error) {
	q, err := SanitizeSelect(rawSQL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()

	start := time.Now()
	rows, err := e.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	out := &QueryResult{SQL: q, Columns: cols}

	for rows.Next() {
		out.RowCount++
		if len(out.Rows) >= MaxRows {
			out.Truncated = true
			continue
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = normalizeScanValue(vals[i])
		}
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	out.ExecMillis = time.Since(start).Milliseconds()
	return out, nil
}

// SanitizeSelect garantiza que rawSQL es UNA sola sentencia SELECT
// (o CTE WITH ... SELECT). Rechaza DML/DDL y multi-sentencia.
//
// Si la query no tiene LIMIT, le agrega LIMIT MaxRows*2 para defensa
// extra (sobre la lógica de Run que ya trunca a MaxRows en lectura).
func SanitizeSelect(rawSQL string) (string, error) {
	q := strings.TrimSpace(rawSQL)
	if q == "" {
		return "", errors.New("sql vacía")
	}
	// Quitar trailing semicolon y comprobar que no haya otro statement.
	q = strings.TrimSuffix(q, ";")
	if strings.Contains(q, ";") {
		return "", errors.New("solo se permite una sola sentencia (sin ';' interno)")
	}
	low := strings.ToLower(q)
	// Solo SELECT o WITH ... SELECT.
	if !(strings.HasPrefix(low, "select") || strings.HasPrefix(low, "with ")) {
		return "", errors.New("solo se permiten queries SELECT (o WITH ... SELECT)")
	}
	// Blacklist de palabras peligrosas. SQLite es read-only por la
	// connection string, pero defendemos en profundidad.
	forbidden := []string{
		"insert ", "update ", "delete ", "drop ", "alter ", "create ",
		"attach ", "detach ", "replace ", "truncate ", "vacuum ",
		"reindex ", "pragma ",
	}
	for _, f := range forbidden {
		if strings.Contains(low+" ", f) {
			return "", fmt.Errorf("palabra prohibida en SQL: %q", strings.TrimSpace(f))
		}
	}
	// Si no tiene LIMIT, le añadimos uno defensivo.
	if !regexp.MustCompile(`(?i)\blimit\b`).MatchString(low) {
		q += fmt.Sprintf(" LIMIT %d", MaxRows*2)
	}
	return q, nil
}

// buildSchema lee la DB e imprime un resumen compacto que se puede
// inyectar al prompt del LLM. Incluye: tablas, columnas, conteo, y
// 1 fila de ejemplo por tabla.
func (e *Engine) buildSchema() (string, error) {
	rows, err := e.db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return "", err
	}
	tables := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return "", err
		}
		tables = append(tables, n)
	}
	rows.Close()

	var sb strings.Builder
	for _, t := range tables {
		var n int
		_ = e.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, t)).Scan(&n)

		colRows, err := e.db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, t))
		if err != nil {
			continue
		}
		cols := []string{}
		for colRows.Next() {
			var cid int
			var cname, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := colRows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
				continue
			}
			cols = append(cols, cname)
		}
		colRows.Close()

		fmt.Fprintf(&sb, `TABLE "%s" (%d rows): %s`, t, n, strings.Join(cols, ", "))
		sb.WriteString("\n")

		// Muestra de una fila para que el LLM vea formatos reales.
		exampleRow, err := e.db.Query(fmt.Sprintf(`SELECT * FROM "%s" LIMIT 1`, t))
		if err == nil {
			ec, _ := exampleRow.Columns()
			if exampleRow.Next() {
				v := make([]any, len(ec))
				p := make([]any, len(ec))
				for i := range v {
					p[i] = &v[i]
				}
				if exampleRow.Scan(p...) == nil {
					parts := []string{}
					for i, c := range ec {
						if i >= 8 {
							parts = append(parts, "…")
							break
						}
						val := normalizeScanValue(v[i])
						if val == nil {
							continue
						}
						s := fmt.Sprintf("%v", val)
						if len(s) > 30 {
							s = s[:30] + "…"
						}
						parts = append(parts, fmt.Sprintf("%s=%q", c, s))
					}
					fmt.Fprintf(&sb, "  example: %s\n", strings.Join(parts, " "))
				}
			}
			exampleRow.Close()
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// normalizeScanValue convierte []byte a string para JSON encoding limpio.
func normalizeScanValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

// FormatRowsForPrompt arma una representación compacta y limitada
// para inyectar en el prompt de phrasing.
func FormatRowsForPrompt(qr *QueryResult) string {
	if qr == nil || len(qr.Rows) == 0 {
		return "(sin filas)"
	}
	b, _ := json.MarshalIndent(qr.Rows, "", "  ")
	out := string(b)
	const maxLen = 8000
	if len(out) > maxLen {
		out = out[:maxLen] + "\n... (truncado)"
	}
	if qr.Truncated {
		out += fmt.Sprintf("\n\n[Solo se muestran %d de %d filas — usá COUNT/SUM/etc en el SQL si querés totales.]", len(qr.Rows), qr.RowCount)
	}
	return out
}

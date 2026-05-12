// Package checks — SQLite loader.
//
// LoadDatasetFromSQLite reads all user tables from a SQLite database and
// converts them into the same Dataset structure that the JSON-based checks
// already understand. This allows every existing rule (FK orphans, empty
// required, missing contact, etc.) to work unchanged on panalbit.db data.
//
// Additionally, CheckSchemaContactCapability inspects the schema itself
// (not data) and emits a structural finding when entity tables lack any
// email-like column — a prerequisite for the cobranza flow.
package checks

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// RuleSchemaContactGap is emitted when the schema of a contact-entity table
// has no column that could hold an email address, making the cobranza flow
// structurally impossible to complete.
const RuleSchemaContactGap = "schema_contact_gap"

// LoadDatasetFromSQLite opens a SQLite database read-only and converts every
// user table into the Dataset.Endpoints map. Column values are kept as
// strings (matching the JSON loader behaviour where everything is interface{}).
func LoadDatasetFromSQLite(dbPath string) (*Dataset, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	defer db.Close()

	tables, err := listUserTables(db)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	endpoints := map[string][]map[string]interface{}{}
	raw := map[string]interface{}{}
	for _, table := range tables {
		records, err := readTable(db, table)
		if err != nil {
			return nil, fmt.Errorf("read table %s: %w", table, err)
		}
		endpoints[table] = records
		// Also populate Raw so SaveDataset works if needed.
		arr := make([]interface{}, len(records))
		for i, r := range records {
			arr[i] = r
		}
		raw[table] = arr
	}

	return &Dataset{
		Endpoints: endpoints,
		Raw:       raw,
	}, nil
}

// listUserTables returns all non-system table names.
func listUserTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// readTable reads all rows from a table into []map[string]interface{}.
func readTable(db *sql.DB, table string) ([]map[string]interface{}, error) {
	// Sanitise table name (no user input, but defence in depth).
	safe := sanitiseIdent(table)
	rows, err := db.Query(`SELECT * FROM "` + safe + `"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := map[string]interface{}{}
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				rec[col] = string(v)
			default:
				rec[col] = v
			}
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// sanitiseIdent removes characters that are not safe in a SQL identifier.
func sanitiseIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// emailLikeColumns are column names that, if present, could hold an email.
var emailLikeColumns = []string{
	"email", "contact_email", "mail", "correo", "e_mail",
	"to", "destination", "email_address",
}

// CheckSchemaContactCapability inspects the schema of contact-entity tables
// (clients, customers, debtors, etc.) and emits a finding if NONE of the
// email-like columns exist. This is a structural gap: no amount of data
// quality fixes can solve it — the schema itself is missing the field.
//
// This check receives a column-map built from the DB schema, not from the
// data values.
func CheckSchemaContactCapability(d *Dataset, tableColumns map[string][]string) []Finding {
	var findings []Finding
	for endpoint := range d.Endpoints {
		if !isContactEntityEndpoint(endpoint) {
			continue
		}
		cols := tableColumns[endpoint]
		if hasEmailColumn(cols) {
			continue
		}
		findings = append(findings, Finding{
			Rule:          RuleSchemaContactGap,
			Severity:      SeverityCritical,
			Endpoint:      endpoint,
			Field:         "email",
			Message:       fmt.Sprintf("La tabla %q no tiene columna de email/contacto. El flujo de cobranza requiere un destinatario para enviar mensajes, pero el schema no contempla este dato.", endpoint),
			Evidence: map[string]interface{}{
				"existing_columns": cols,
				"checked_for":      emailLikeColumns,
			},
			Suggestion:    "Enriquecer la fuente de datos con emails de contacto o proveerlos via Sabio (contact-store/contact-import-csv).",
			AutoFixable:   false,
			Actionability: ActionabilityStructural,
			FixHint: map[string]interface{}{
				"strategy":          "external_enrichment",
				"required_field":    "email",
				"provider_hint":     "sabio",
				"required_artifact": "contact.destination.v1",
			},
		})
	}
	return findings
}

// TableColumnsFromDB reads column names per table from a SQLite DB.
func TableColumnsFromDB(dbPath string) (map[string][]string, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tables, err := listUserTables(db)
	if err != nil {
		return nil, err
	}

	result := map[string][]string{}
	for _, table := range tables {
		cols, err := tableColumnNames(db, table)
		if err != nil {
			return nil, fmt.Errorf("columns for %s: %w", table, err)
		}
		result[table] = cols
	}
	return result, nil
}

// tableColumnNames returns column names for a given table.
func tableColumnNames(db *sql.DB, table string) ([]string, error) {
	safe := sanitiseIdent(table)
	rows, err := db.Query(`PRAGMA table_info("` + safe + `")`)
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
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// hasEmailColumn returns true if any column in the list looks like an email field.
func hasEmailColumn(cols []string) bool {
	for _, col := range cols {
		lower := strings.ToLower(col)
		for _, candidate := range emailLikeColumns {
			if lower == candidate {
				return true
			}
		}
	}
	return false
}

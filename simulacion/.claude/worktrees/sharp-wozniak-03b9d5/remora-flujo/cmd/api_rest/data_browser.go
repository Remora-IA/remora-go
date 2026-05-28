package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/extrame/xls"
	"github.com/gorilla/mux"
	"github.com/xuri/excelize/v2"
	_ "modernc.org/sqlite"
)

type dataTableInfo struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Columns []string `json:"columns"`
}

type dataRowsResponse struct {
	Table   string              `json:"table"`
	Count   int                 `json:"count"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
	Columns []string            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
}

type uploadSheet struct {
	Name string
	Rows [][]string
}

func (s *server) handleDataTables(w http.ResponseWriter, r *http.Request) {
	businessID, _, ok := s.requireMembershipContext(w, r, r.URL.Query().Get("business_id"), nil)
	if !ok {
		return
	}
	db, err := s.openDataDB(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()

	tables, err := listDataTables(db)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, tables)
}

func (s *server) handleDataTableRows(w http.ResponseWriter, r *http.Request) {
	businessID, _, ok := s.requireMembershipContext(w, r, r.URL.Query().Get("business_id"), nil)
	if !ok {
		return
	}
	table := mux.Vars(r)["table"]
	limit := clampInt(queryInt(r, "limit", 100), 1, 500)
	offset := maxInt(queryInt(r, "offset", 0), 0)

	db, err := s.openDataDB(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()

	if ok, err := dataTableExists(db, table); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	} else if !ok {
		writeErr(w, http.StatusNotFound, "tabla no encontrada")
		return
	}

	columns, err := dataTableColumns(db, table)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	count, err := dataTableCount(db, table)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := dataTableRows(db, table, columns, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeOK(w, dataRowsResponse{
		Table:   table,
		Count:   count,
		Limit:   limit,
		Offset:  offset,
		Columns: columns,
		Rows:    rows,
	})
}

func (s *server) handleBusinessDataTables(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	db, err := s.openDataDB(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()
	tables, err := listDataTables(db)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, tables)
}

func (s *server) handleBusinessDataTableRows(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	table := mux.Vars(r)["table"]
	limit := clampInt(queryInt(r, "limit", 100), 1, 500)
	offset := maxInt(queryInt(r, "offset", 0), 0)
	db, err := s.openDataDB(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()
	writeDataTableRows(w, db, table, limit, offset)
}

func (s *server) handleBusinessDataUpload(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "archivo inválido")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file requerido")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 64<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sheets, err := parseUploadedTables(header.Filename, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	db, err := s.openWritableDataDB(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer db.Close()
	imported := []dataTableInfo{}
	for _, sheet := range sheets {
		info, err := importSheet(db, header.Filename, sheet)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		imported = append(imported, info)
	}
	writeOK(w, map[string]any{"tables": imported})
}

func writeDataTableRows(w http.ResponseWriter, db *sql.DB, table string, limit, offset int) {
	if ok, err := dataTableExists(db, table); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	} else if !ok {
		writeErr(w, http.StatusNotFound, "tabla no encontrada")
		return
	}

	columns, err := dataTableColumns(db, table)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	count, err := dataTableCount(db, table)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := dataTableRows(db, table, columns, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeOK(w, dataRowsResponse{
		Table:   table,
		Count:   count,
		Limit:   limit,
		Offset:  offset,
		Columns: columns,
		Rows:    rows,
	})
}

func (s *server) openDataDB(businessID string) (*sql.DB, error) {
	path := os.Getenv("SABIO_DB")
	if path == "" {
		path = s.businessSQLitePath(businessID)
		if path == "" {
			path = businessDataDBPath(s.rootDir, businessID)
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.rootDir, path)
	}
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

func (s *server) openWritableDataDB(businessID string) (*sql.DB, error) {
	path := businessDataDBPath(s.rootDir, businessID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

func businessDataDBPath(rootDir, businessID string) string {
	if businessID == "" {
		return ""
	}
	return filepath.Join(rootDir, "temp", "business_data", safeIdent(businessID)+".db")
}

func parseUploadedTables(filename string, data []byte) ([]uploadSheet, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv", ".txt":
		return parseDelimited(filename, data, ',')
	case ".tsv", ".tab":
		return parseDelimited(filename, data, '\t')
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return parseExcelXML(data)
	case ".xls", ".xlt":
		return parseExcelLegacy(data)
	case ".ods":
		return parseODS(data)
	default:
		return nil, fmt.Errorf("formato no soportado: %s", ext)
	}
}

func parseDelimited(filename string, data []byte, comma rune) ([]uploadSheet, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return []uploadSheet{{Name: strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)), Rows: rows}}, nil
}

func parseExcelXML(data []byte) ([]uploadSheet, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []uploadSheet{}
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name)
		if err != nil {
			return nil, err
		}
		if len(nonEmptyRows(rows)) > 0 {
			out = append(out, uploadSheet{Name: name, Rows: rows})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("archivo sin hojas con datos")
	}
	return out, nil
}

func parseExcelLegacy(data []byte) ([]uploadSheet, error) {
	wb, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return nil, err
	}
	out := []uploadSheet{}
	for i := 0; i < wb.NumSheets(); i++ {
		sheet := wb.GetSheet(i)
		if sheet == nil {
			continue
		}
		rows := [][]string{}
		maxCols := 0
		for r := 0; r <= int(sheet.MaxRow); r++ {
			row := sheet.Row(r)
			if row == nil {
				rows = append(rows, nil)
				continue
			}
			vals := []string{}
			for c := 0; c <= row.LastCol(); c++ {
				vals = append(vals, row.Col(c))
			}
			if len(vals) > maxCols {
				maxCols = len(vals)
			}
			rows = append(rows, vals)
		}
		if maxCols > 0 && len(nonEmptyRows(rows)) > 0 {
			out = append(out, uploadSheet{Name: sheet.Name, Rows: rows})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("archivo sin hojas con datos")
	}
	return out, nil
}

type odsTextP struct {
	Text string `xml:",chardata"`
}

type odsCell struct {
	Repeat string     `xml:"number-columns-repeated,attr"`
	Value  string     `xml:"value,attr"`
	TextP  []odsTextP `xml:"p"`
}

type odsRow struct {
	Repeat string    `xml:"number-rows-repeated,attr"`
	Cells  []odsCell `xml:"table-cell"`
}

type odsTable struct {
	Name string   `xml:"name,attr"`
	Rows []odsRow `xml:"table-row"`
}

type odsContent struct {
	Tables []odsTable `xml:"body>spreadsheet>table"`
}

func parseODS(data []byte) ([]uploadSheet, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var content []byte
	for _, f := range zr.File {
		if f.Name != "content.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err = io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		break
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("ODS inválido: falta content.xml")
	}
	var parsed odsContent
	if err := xml.Unmarshal(content, &parsed); err != nil {
		return nil, err
	}
	out := []uploadSheet{}
	for _, table := range parsed.Tables {
		rows := [][]string{}
		for _, row := range table.Rows {
			repeatRows := clampInt(atoiDefault(row.Repeat, 1), 1, 1000)
			vals := []string{}
			for _, cell := range row.Cells {
				value := cell.Value
				if len(cell.TextP) > 0 {
					parts := []string{}
					for _, p := range cell.TextP {
						parts = append(parts, p.Text)
					}
					value = strings.Join(parts, "\n")
				}
				repeatCols := clampInt(atoiDefault(cell.Repeat, 1), 1, 1000)
				for i := 0; i < repeatCols; i++ {
					vals = append(vals, value)
				}
			}
			for i := 0; i < repeatRows; i++ {
				rows = append(rows, vals)
			}
		}
		if len(nonEmptyRows(rows)) > 0 {
			out = append(out, uploadSheet{Name: table.Name, Rows: rows})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("archivo sin hojas con datos")
	}
	return out, nil
}

func importSheet(db *sql.DB, filename string, sheet uploadSheet) (dataTableInfo, error) {
	rows := nonEmptyRows(sheet.Rows)
	if len(rows) == 0 {
		return dataTableInfo{}, fmt.Errorf("hoja sin datos: %s", sheet.Name)
	}
	headers := normalizeHeaders(rows[0])
	maxCols := len(headers)
	for _, row := range rows[1:] {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	for len(headers) < maxCols {
		headers = append(headers, fmt.Sprintf("col_%d", len(headers)+1))
	}
	table := uniqueTableName(db, safeIdent(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))+"_"+sheet.Name))
	defs := []string{}
	for _, col := range headers {
		defs = append(defs, quoteIdent(col)+" TEXT")
	}
	if _, err := db.Exec(`CREATE TABLE ` + quoteIdent(table) + ` (` + strings.Join(defs, ", ") + `)`); err != nil {
		return dataTableInfo{}, err
	}
	if len(rows) > 1 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(headers)), ",")
		stmt, err := db.Prepare(`INSERT INTO ` + quoteIdent(table) + ` (` + quoteIdentList(headers) + `) VALUES (` + placeholders + `)`)
		if err != nil {
			return dataTableInfo{}, err
		}
		defer stmt.Close()
		for _, row := range rows[1:] {
			vals := make([]any, len(headers))
			for i := range headers {
				if i < len(row) {
					vals[i] = row[i]
				} else {
					vals[i] = ""
				}
			}
			if _, err := stmt.Exec(vals...); err != nil {
				return dataTableInfo{}, err
			}
		}
	}
	count, _ := dataTableCount(db, table)
	return dataTableInfo{Name: table, Count: count, Columns: headers}, nil
}

func nonEmptyRows(rows [][]string) [][]string {
	out := [][]string{}
	for _, row := range rows {
		empty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				empty = false
				break
			}
		}
		if !empty {
			out = append(out, row)
		}
	}
	return out
}

func normalizeHeaders(row []string) []string {
	headers := []string{}
	seen := map[string]int{}
	for i, raw := range row {
		name := safeIdent(raw)
		if name == "" || name == "table" {
			name = fmt.Sprintf("col_%d", i+1)
		}
		seen[name]++
		if seen[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, seen[name])
		}
		headers = append(headers, name)
	}
	if len(headers) == 0 {
		headers = []string{"col_1"}
	}
	return headers
}

func uniqueTableName(db *sql.DB, base string) string {
	if base == "" {
		base = "dataset"
	}
	name := base
	for i := 2; ; i++ {
		ok, err := dataTableExists(db, name)
		if err != nil || !ok {
			return name
		}
		name = fmt.Sprintf("%s_%d", base, i)
	}
}

var unsafeIdentChars = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func safeIdent(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = unsafeIdentChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "t_" + s
	}
	return s
}

func atoiDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func listDataTables(db *sql.DB) ([]dataTableInfo, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dataTableInfo{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns, err := dataTableColumns(db, name)
		if err != nil {
			return nil, err
		}
		count, err := dataTableCount(db, name)
		if err != nil {
			return nil, err
		}
		out = append(out, dataTableInfo{Name: name, Count: count, Columns: columns})
	}
	return out, rows.Err()
}

func dataTableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ? AND name NOT LIKE 'sqlite_%'`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func dataTableColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(`PRAGMA table_info(` + quoteIdent(table) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func dataTableCount(db *sql.DB, table string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM ` + quoteIdent(table)).Scan(&count)
	return count, err
}

func dataTableRows(db *sql.DB, table string, columns []string, limit, offset int) ([]map[string]string, error) {
	query := `SELECT ` + quoteIdentList(columns) + ` FROM ` + quoteIdent(table) + ` ORDER BY rowid DESC LIMIT ? OFFSET ?`
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		query = `SELECT ` + quoteIdentList(columns) + ` FROM ` + quoteIdent(table) + ` LIMIT ? OFFSET ?`
		rows, err = db.Query(query, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]string{}
	raw := make([]sql.NullString, len(columns))
	dest := make([]any, len(columns))
	for i := range raw {
		dest[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		item := map[string]string{}
		for i, col := range columns {
			if raw[i].Valid {
				item[col] = raw[i].String
			} else {
				item[col] = ""
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteIdentList(cols []string) string {
	quoted := make([]string, 0, len(cols))
	for _, col := range cols {
		quoted = append(quoted, quoteIdent(col))
	}
	return strings.Join(quoted, ", ")
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(v, minV int) int {
	if v < minV {
		return minV
	}
	return v
}

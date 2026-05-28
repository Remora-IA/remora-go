package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type apiConnectionResource struct {
	Name             string         `json:"name"`
	Method           string         `json:"method"`
	Path             string         `json:"path"`
	TableName        string         `json:"table_name"`
	RecordsPath      string         `json:"records_path"`
	PrimaryKey       string         `json:"primary_key"`
	Pagination       map[string]any `json:"pagination"`
	Incremental      map[string]any `json:"incremental,omitempty"`
	ResponseMappings map[string]any `json:"response_mapping,omitempty"`
}

type apiConnection struct {
	ID            string                  `json:"id"`
	BusinessID    string                  `json:"business_id"`
	Name          string                  `json:"name"`
	BaseURL       string                  `json:"base_url"`
	AuthType      string                  `json:"auth_type"`
	Resources     []apiConnectionResource `json:"resources"`
	SyncFrequency string                  `json:"sync_frequency"`
	Status        string                  `json:"status"`
	LastSyncAt    string                  `json:"last_sync_at,omitempty"`
	LastError     string                  `json:"last_error,omitempty"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
}

type createAPIConnectionRequest struct {
	Name          string                  `json:"name"`
	BaseURL       string                  `json:"base_url"`
	AuthType      string                  `json:"auth_type"`
	Credentials   map[string]string       `json:"credentials"`
	Resources     []apiConnectionResource `json:"resources"`
	SyncFrequency string                  `json:"sync_frequency"`
}

type planAPIConnectionRequest struct {
	BaseURL string `json:"base_url"`
	Docs    string `json:"docs"`
}

type apiSyncRun struct {
	ID             string   `json:"id"`
	ConnectionID   string   `json:"connection_id"`
	BusinessID     string   `json:"business_id"`
	Status         string   `json:"status"`
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at,omitempty"`
	RecordsRead    int      `json:"records_read"`
	RecordsWritten int      `json:"records_written"`
	TablesUpdated  int      `json:"tables_updated"`
	Error          string   `json:"error,omitempty"`
	Logs           []string `json:"logs"`
}

func (s *server) handleAPIConnectionsList(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	conns, err := s.auth.listAPIConnections(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, conns)
}

func (s *server) handleAPIConnectionCreate(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	membership, err := s.auth.membership(user.ID, businessID)
	if err != nil {
		writeErr(w, http.StatusForbidden, "usuario sin acceso al negocio: "+businessID)
		return
	}
	if !canManageBusiness(membership.Role) {
		writeErr(w, http.StatusForbidden, "rol sin permiso para conectar APIs")
		return
	}
	var req createAPIConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	conn, err := s.auth.createAPIConnection(businessID, user.ID, req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, conn)
}

func (s *server) handleAPIConnectionPlan(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	var req planAPIConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	if strings.TrimSpace(req.Docs) == "" {
		writeErr(w, http.StatusBadRequest, "docs requerido")
		return
	}
	dir := filepath.Join(s.rootDir, "temp", "api_plans", businessID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id := "plan_" + randomToken(10)
	docsPath := filepath.Join(dir, id+".docs.txt")
	outPath := filepath.Join(dir, id+".json")
	if err := os.WriteFile(docsPath, []byte(req.Docs), 0600); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	ctx := r.Context()
	resp, err := s.scoped(id).ExecuteCommand(ctx, "./frameworkindexa", []string{"api-plan", "--docs-file", docsPath, "--base-url", req.BaseURL, "--out", outPath}, filepath.Join(s.rootDir, "framework-indexa"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !resp.Success || resp.ExitCode != 0 {
		msg := strings.TrimSpace(resp.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(resp.Stdout)
		}
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	raw := []byte(resp.Stdout)
	if fromFile, err := os.ReadFile(outPath); err == nil && len(fromFile) > 0 {
		raw = fromFile
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		writeErr(w, http.StatusInternalServerError, "planner devolvió JSON inválido")
		return
	}
	writeOK(w, spec)
}

func (s *server) handleAPIConnectionSync(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	connID := mux.Vars(r)["connection_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	conn, creds, err := s.auth.apiConnectionWithCredentials(businessID, connID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "conexión no encontrada")
		return
	}
	run := s.auth.startAPISyncRun(conn.ID, businessID)
	result := s.syncAPIConnection(r.Context(), conn, creds, run)
	_ = s.auth.finishAPISyncRun(result)
	writeOK(w, result)
}

func (s *authStore) createAPIConnection(businessID, createdBy string, req createAPIConnectionRequest) (apiConnection, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if req.Name == "" {
		return apiConnection{}, errors.New("nombre requerido")
	}
	if req.BaseURL == "" {
		return apiConnection{}, errors.New("base_url requerido")
	}
	if _, err := url.ParseRequestURI(req.BaseURL); err != nil {
		return apiConnection{}, errors.New("base_url inválido")
	}
	if len(req.Resources) == 0 {
		return apiConnection{}, errors.New("al menos un recurso requerido")
	}
	authType := normalizeAPIAuthType(req.AuthType)
	if req.SyncFrequency == "" {
		req.SyncFrequency = "manual"
	}
	resources := normalizeAPIResources(req.Resources)
	resourcesRaw, _ := json.Marshal(resources)
	credsRaw, _ := json.Marshal(req.Credentials)
	cipherText, err := encryptLocalSecret(credsRaw)
	if err != nil {
		return apiConnection{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	conn := apiConnection{
		ID:            "apic_" + randomToken(12),
		BusinessID:    businessID,
		Name:          req.Name,
		BaseURL:       req.BaseURL,
		AuthType:      authType,
		Resources:     resources,
		SyncFrequency: req.SyncFrequency,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err = s.db.Exec(`INSERT INTO business_api_connections (id, business_id, name, base_url, auth_type, credentials_cipher, resources_json, sync_frequency, status, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.BusinessID, conn.Name, conn.BaseURL, conn.AuthType, cipherText, string(resourcesRaw), conn.SyncFrequency, conn.Status, createdBy, now, now)
	return conn, err
}

func (s *authStore) listAPIConnections(businessID string) ([]apiConnection, error) {
	rows, err := s.db.Query(`SELECT id, business_id, name, base_url, auth_type, resources_json, sync_frequency, status, COALESCE(last_sync_at,''), COALESCE(last_error,''), created_at, updated_at FROM business_api_connections WHERE business_id=? ORDER BY created_at DESC`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []apiConnection{}
	for rows.Next() {
		var conn apiConnection
		var resourcesRaw string
		if err := rows.Scan(&conn.ID, &conn.BusinessID, &conn.Name, &conn.BaseURL, &conn.AuthType, &resourcesRaw, &conn.SyncFrequency, &conn.Status, &conn.LastSyncAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(resourcesRaw), &conn.Resources)
		out = append(out, conn)
	}
	return out, rows.Err()
}

func (s *authStore) apiConnectionWithCredentials(businessID, connID string) (apiConnection, map[string]string, error) {
	row := s.db.QueryRow(`SELECT id, business_id, name, base_url, auth_type, credentials_cipher, resources_json, sync_frequency, status, COALESCE(last_sync_at,''), COALESCE(last_error,''), created_at, updated_at FROM business_api_connections WHERE business_id=? AND id=?`, businessID, connID)
	var conn apiConnection
	var resourcesRaw, cipherText string
	if err := row.Scan(&conn.ID, &conn.BusinessID, &conn.Name, &conn.BaseURL, &conn.AuthType, &cipherText, &resourcesRaw, &conn.SyncFrequency, &conn.Status, &conn.LastSyncAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt); err != nil {
		return apiConnection{}, nil, err
	}
	_ = json.Unmarshal([]byte(resourcesRaw), &conn.Resources)
	raw, err := decryptLocalSecret(cipherText)
	if err != nil {
		return apiConnection{}, nil, err
	}
	creds := map[string]string{}
	_ = json.Unmarshal(raw, &creds)
	return conn, creds, nil
}

func (s *authStore) startAPISyncRun(connectionID, businessID string) apiSyncRun {
	now := time.Now().UTC().Format(time.RFC3339)
	run := apiSyncRun{ID: "apisr_" + randomToken(12), ConnectionID: connectionID, BusinessID: businessID, Status: "running", StartedAt: now, Logs: []string{}}
	_, _ = s.db.Exec(`INSERT INTO business_api_sync_runs (id, connection_id, business_id, status, started_at) VALUES (?, ?, ?, ?, ?)`, run.ID, run.ConnectionID, run.BusinessID, run.Status, run.StartedAt)
	return run
}

func (s *authStore) finishAPISyncRun(run apiSyncRun) error {
	rawLogs, _ := json.Marshal(run.Logs)
	_, err := s.db.Exec(`UPDATE business_api_sync_runs SET status=?, finished_at=?, records_read=?, records_written=?, tables_updated=?, error=?, log_json=? WHERE id=?`,
		run.Status, run.FinishedAt, run.RecordsRead, run.RecordsWritten, run.TablesUpdated, run.Error, string(rawLogs), run.ID)
	if err != nil {
		return err
	}
	if run.Status == "success" {
		_, err = s.db.Exec(`UPDATE business_api_connections SET last_sync_at=?, last_error='', updated_at=? WHERE id=?`, run.FinishedAt, run.FinishedAt, run.ConnectionID)
	} else {
		_, err = s.db.Exec(`UPDATE business_api_connections SET last_error=?, updated_at=? WHERE id=?`, run.Error, run.FinishedAt, run.ConnectionID)
	}
	return err
}

func (s *server) syncAPIConnection(ctx any, conn apiConnection, creds map[string]string, run apiSyncRun) apiSyncRun {
	run.Logs = append(run.Logs, "sync iniciado")
	defer func() {
		run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	}()
	db, err := s.openWritableDataDB(conn.BusinessID)
	if err != nil {
		run.Status, run.Error = "error", err.Error()
		return run
	}
	defer db.Close()
	client := &http.Client{Timeout: 60 * time.Second}
	for _, res := range conn.Resources {
		records, err := fetchAPIResource(client, conn, creds, res)
		if err != nil {
			run.Status, run.Error = "error", fmt.Sprintf("%s: %v", res.Name, err)
			run.Logs = append(run.Logs, run.Error)
			return run
		}
		run.RecordsRead += len(records)
		written, err := upsertAPIRecords(db, res, records)
		if err != nil {
			run.Status, run.Error = "error", fmt.Sprintf("%s: %v", res.Name, err)
			run.Logs = append(run.Logs, run.Error)
			return run
		}
		run.RecordsWritten += written
		run.TablesUpdated++
		run.Logs = append(run.Logs, fmt.Sprintf("%s: %d registros", res.Name, written))
	}
	run.Status = "success"
	run.Logs = append(run.Logs, "sync completado")
	return run
}

func fetchAPIResource(client *http.Client, conn apiConnection, creds map[string]string, res apiConnectionResource) ([]map[string]any, error) {
	method := strings.ToUpper(res.Method)
	if method == "" {
		method = "GET"
	}
	if method != "GET" {
		return nil, errors.New("MVP solo soporta GET")
	}
	pType := stringFromMap(res.Pagination, "type", "none")
	pageSize := intFromMap(res.Pagination, "page_size", 100)
	maxPages := intFromMap(res.Pagination, "max_pages", 100)
	out := []map[string]any{}
	for page := 1; page <= maxPages; page++ {
		u, err := buildResourceURL(conn.BaseURL, res.Path)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		switch pType {
		case "page":
			q.Set(stringFromMap(res.Pagination, "page_param", "page"), strconv.Itoa(page))
			q.Set(stringFromMap(res.Pagination, "page_size_param", "limit"), strconv.Itoa(pageSize))
		case "offset":
			q.Set(stringFromMap(res.Pagination, "offset_param", "offset"), strconv.Itoa((page-1)*pageSize))
			q.Set(stringFromMap(res.Pagination, "limit_param", "limit"), strconv.Itoa(pageSize))
		case "none", "":
		default:
			return nil, fmt.Errorf("paginación no soportada: %s", pType)
		}
		u.RawQuery = q.Encode()
		req, _ := http.NewRequest("GET", u.String(), nil)
		applyAPIAuth(req, conn.AuthType, creds)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet(raw, 240))
		}
		batch, err := extractAPIRecords(raw, res.RecordsPath)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if pType == "none" || len(batch) < pageSize {
			break
		}
	}
	return out, nil
}

func upsertAPIRecords(db *sql.DB, res apiConnectionResource, records []map[string]any) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}
	table := safeIdent(res.TableName)
	if table == "" {
		table = safeIdent(res.Name)
	}
	colsMap := map[string]bool{"_raw_json": true, "_synced_at": true}
	for _, rec := range records {
		for k := range flattenMap(rec, "") {
			colsMap[safeIdent(k)] = true
		}
	}
	cols := []string{}
	for col := range colsMap {
		cols = append(cols, col)
	}
	pk := safeIdent(res.PrimaryKey)
	defs := []string{}
	for _, col := range cols {
		def := quoteIdent(col) + " TEXT"
		if pk != "" && col == pk {
			def += " PRIMARY KEY"
		}
		defs = append(defs, def)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ` + quoteIdent(table) + ` (` + strings.Join(defs, ", ") + `)`); err != nil {
		return 0, err
	}
	existing, _ := dataTableColumns(db, table)
	existingSet := map[string]bool{}
	for _, col := range existing {
		existingSet[col] = true
	}
	for _, col := range cols {
		if !existingSet[col] {
			if _, err := db.Exec(`ALTER TABLE ` + quoteIdent(table) + ` ADD COLUMN ` + quoteIdent(col) + ` TEXT`); err != nil {
				return 0, err
			}
		}
	}
	insertCols := append(existing, missingCols(cols, existingSet)...)
	placeholders := strings.TrimRight(strings.Repeat("?,", len(insertCols)), ",")
	query := `INSERT OR REPLACE INTO ` + quoteIdent(table) + ` (` + quoteIdentList(insertCols) + `) VALUES (` + placeholders + `)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, rec := range records {
		flat := flattenMap(rec, "")
		raw, _ := json.Marshal(rec)
		vals := make([]any, len(insertCols))
		for i, col := range insertCols {
			switch col {
			case "_raw_json":
				vals[i] = string(raw)
			case "_synced_at":
				vals[i] = now
			default:
				vals[i] = fmt.Sprint(flat[col])
			}
		}
		if _, err := stmt.Exec(vals...); err != nil {
			return 0, err
		}
	}
	return len(records), nil
}

func normalizeAPIAuthType(v string) string {
	key := strings.ToLower(strings.TrimSpace(v))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	switch key {
	case "bearer", "bearer_token", "token":
		return "bearer"
	case "api_key", "apikey", "x_api_key":
		return "api_key"
	case "basic", "basic_auth":
		return "basic"
	case "none", "no_auth":
		return "none"
	default:
		return "none"
	}
}

func normalizeAPIResources(resources []apiConnectionResource) []apiConnectionResource {
	out := []apiConnectionResource{}
	for _, r := range resources {
		r.Name = strings.TrimSpace(r.Name)
		r.Path = strings.TrimSpace(r.Path)
		if r.Name == "" && r.TableName != "" {
			r.Name = r.TableName
		}
		if r.TableName == "" {
			r.TableName = r.Name
		}
		if r.Method == "" {
			r.Method = "GET"
		}
		if r.RecordsPath == "" {
			r.RecordsPath = "$"
		}
		if r.Pagination == nil {
			r.Pagination = map[string]any{"type": "none"}
		}
		if r.Name != "" && r.Path != "" {
			out = append(out, r)
		}
	}
	return out
}

func buildResourceURL(baseURL, path string) (*url.URL, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return url.Parse(path)
	}
	return url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/"))
}

func applyAPIAuth(req *http.Request, authType string, creds map[string]string) {
	switch authType {
	case "bearer":
		if token := creds["token"]; token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "api_key":
		name := creds["header"]
		if name == "" {
			name = "X-API-Key"
		}
		req.Header.Set(name, creds["key"])
	case "basic":
		req.SetBasicAuth(creds["user"], creds["password"])
	}
	if creds["key"] != "" && authType != "api_key" {
		name := creds["header"]
		if name == "" {
			name = "X-API-Key"
		}
		req.Header.Set(name, creds["key"])
	}
}

func extractAPIRecords(raw []byte, path string) ([]map[string]any, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	target := jsonPath(v, path)
	arr, ok := target.([]any)
	if !ok {
		if m, ok := target.(map[string]any); ok {
			arr = []any{m}
		} else {
			return nil, fmt.Errorf("records_path no apunta a una lista")
		}
	}
	out := []map[string]any{}
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func jsonPath(v any, path string) any {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return v
	}
	cur := v
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

func flattenMap(m map[string]any, prefix string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		key := safeIdent(k)
		if prefix != "" {
			key = prefix + "_" + key
		}
		if nested, ok := v.(map[string]any); ok {
			for nk, nv := range flattenMap(nested, key) {
				out[nk] = nv
			}
			continue
		}
		if arr, ok := v.([]any); ok {
			raw, _ := json.Marshal(arr)
			out[key] = string(raw)
			continue
		}
		out[key] = v
	}
	return out
}

func missingCols(cols []string, existing map[string]bool) []string {
	out := []string{}
	for _, col := range cols {
		if !existing[col] {
			out = append(out, col)
		}
	}
	return out
}

func stringFromMap(m map[string]any, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func intFromMap(m map[string]any, key string, fallback int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func encryptLocalSecret(raw []byte) (string, error) {
	key, err := localSecretKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, raw, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func decryptLocalSecret(cipherText string) ([]byte, error) {
	if cipherText == "" {
		return []byte("{}"), nil
	}
	key, err := localSecretKey()
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("secreto corrupto")
	}
	return gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
}

func localSecretKey() ([]byte, error) {
	if v := os.Getenv("REMORA_SECRET_KEY"); v != "" {
		sum := sha256.Sum256([]byte(v))
		return sum[:], nil
	}
	path := filepath.Join("temp", "remora_secret.key")
	if raw, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(raw)) > 0 {
		decoded, err := base64.RawURLEncoding.DecodeString(string(bytes.TrimSpace(raw)))
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
		sum := sha256.Sum256(bytes.TrimSpace(raw))
		return sum[:], nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.RawURLEncoding.EncodeToString(key)), 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func snippet(raw []byte, n int) string {
	s := string(raw)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

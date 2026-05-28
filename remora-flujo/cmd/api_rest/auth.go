package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const authCookieName = "remora_session"

type authStore struct {
	db *sql.DB
}

type authUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type authMembership struct {
	BusinessID   string         `json:"business_id"`
	BusinessName string         `json:"business_name,omitempty"`
	Role         string         `json:"role"`
	Audience     string         `json:"audience"`
	Scope        map[string]any `json:"scope"`
}

type authBusiness struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Domain           string    `json:"domain,omitempty"`
	Country          string    `json:"country,omitempty"`
	DefaultFramework string    `json:"default_framework,omitempty"`
	OwnerUserID      string    `json:"owner_user_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type authSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type authBusinessMember struct {
	UserID    string         `json:"user_id"`
	Email     string         `json:"email"`
	Name      string         `json:"name"`
	Role      string         `json:"role"`
	Audience  string         `json:"audience"`
	Scope     map[string]any `json:"scope"`
	CreatedAt string         `json:"created_at"`
}

type authBusinessInvite struct {
	ID           string         `json:"id"`
	BusinessID   string         `json:"business_id"`
	BusinessName string         `json:"business_name,omitempty"`
	Token        string         `json:"token"`
	Role         string         `json:"role"`
	Audience     string         `json:"audience"`
	Scope        map[string]any `json:"scope"`
	CreatedBy    string         `json:"created_by"`
	ExpiresAt    time.Time      `json:"expires_at"`
	AcceptedBy   string         `json:"accepted_by,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type remoraUserOverview struct {
	ID              string           `json:"id"`
	Email           string           `json:"email"`
	Name            string           `json:"name"`
	Role            string           `json:"role"`
	CreatedAt       string           `json:"created_at"`
	MembershipCount int              `json:"membership_count"`
	Memberships     []authMembership `json:"memberships"`
}

type remoraTeamInvite struct {
	ID         string    `json:"id"`
	Token      string    `json:"token"`
	Role       string    `json:"role"`
	CreatedBy  string    `json:"created_by"`
	AcceptedBy string    `json:"accepted_by,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func openAuthStore() (*authStore, error) {
	path := envOr("REMORA_AUTH_DB", defaultAuthDBPath())
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &authStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seedDefaults(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func defaultAuthDBPath() string {
	root := resolveRemoraRoot()
	if root == "" || root == "/workspace" {
		return "profiles/_system/remora_auth.db"
	}
	return filepath.Join(root, "profiles", "_system", "remora_auth.db")
}

func (s *authStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			name TEXT,
			role TEXT NOT NULL DEFAULT 'user',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS businesses (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			domain TEXT,
			country TEXT,
			default_framework TEXT,
			owner_user_id TEXT,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS business_memberships (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			business_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'staff',
			audience TEXT NOT NULL,
			scope_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			UNIQUE(user_id, business_id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS business_invites (
			id TEXT PRIMARY KEY,
			business_id TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			code_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'staff',
			audience TEXT NOT NULL DEFAULT 'staff',
			scope_json TEXT NOT NULL DEFAULT '{}',
			created_by TEXT NOT NULL,
			accepted_by TEXT,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS remora_team_invites (
			id TEXT PRIMARY KEY,
			token TEXT NOT NULL UNIQUE,
			code_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'remora_staff',
			created_by TEXT NOT NULL,
			accepted_by TEXT,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS business_api_connections (
			id TEXT PRIMARY KEY,
			business_id TEXT NOT NULL,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			auth_type TEXT NOT NULL DEFAULT 'none',
			credentials_cipher TEXT NOT NULL DEFAULT '',
			resources_json TEXT NOT NULL DEFAULT '[]',
			sync_frequency TEXT NOT NULL DEFAULT 'manual',
			status TEXT NOT NULL DEFAULT 'active',
			last_sync_at TEXT,
			last_error TEXT,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS business_api_sync_runs (
			id TEXT PRIMARY KEY,
			connection_id TEXT NOT NULL,
			business_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			records_read INTEGER NOT NULL DEFAULT 0,
			records_written INTEGER NOT NULL DEFAULT 0,
			tables_updated INTEGER NOT NULL DEFAULT 0,
			error TEXT,
			log_json TEXT NOT NULL DEFAULT '[]'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	alterStmts := []string{
		`ALTER TABLE businesses ADD COLUMN domain TEXT`,
		`ALTER TABLE businesses ADD COLUMN country TEXT`,
		`ALTER TABLE businesses ADD COLUMN owner_user_id TEXT`,
		`ALTER TABLE business_memberships ADD COLUMN role TEXT NOT NULL DEFAULT 'staff'`,
		// Auth0: columna para guardar el sub de Auth0 y hacer upsert de usuarios
		`ALTER TABLE users ADD COLUMN auth0_sub TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_auth0_sub ON users (auth0_sub) WHERE auth0_sub IS NOT NULL`,
	}
	for _, stmt := range alterStmts {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	return nil
}

func (s *authStore) seedDefaults() error {
	email := strings.TrimSpace(os.Getenv("REMORA_BOOTSTRAP_EMAIL"))
	pass := os.Getenv("REMORA_BOOTSTRAP_PASSWORD")
	if email == "" || pass == "" {
		return nil
	}
	var exists int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE lower(email)=lower(?)`, email).Scan(&exists)
	if exists > 0 {
		return nil
	}
	_, err := s.createUser(email, pass, "Admin", "admin")
	return err
}

func (s *authStore) createUser(email, password, name, role string) (*authUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("email inválido")
	}
	if len(password) < 8 {
		return nil, errors.New("password debe tener al menos 8 caracteres")
	}
	if role == "" {
		role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	user := &authUser{
		ID:        "usr_" + randomToken(18),
		Email:     email,
		Name:      strings.TrimSpace(name),
		Role:      role,
		CreatedAt: now,
	}
	_, err = s.db.Exec(`INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, string(hash), user.Name, user.Role, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, errors.New("email ya registrado")
		}
		return nil, err
	}
	return user, nil
}

func (s *authStore) authenticate(email, password string) (*authUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	row := s.db.QueryRow(`SELECT id, email, password_hash, name, role, created_at FROM users WHERE lower(email)=lower(?)`, email)
	var user authUser
	var passwordHash, created string
	if err := row.Scan(&user.ID, &user.Email, &passwordHash, &user.Name, &user.Role, &created); err != nil {
		return nil, errors.New("credenciales inválidas")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, errors.New("credenciales inválidas")
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &user, nil
}

func (s *authStore) createSession(userID string) (string, authSession, error) {
	token := randomToken(32)
	hash := hashToken(token)
	now := time.Now().UTC()
	sess := authSession{
		ID:        "sess_" + randomToken(18),
		UserID:    userID,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	_, err := s.db.Exec(`INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, hash, sess.ExpiresAt.Format(time.RFC3339), now.Format(time.RFC3339))
	return token, sess, err
}

func (s *authStore) userByToken(token string) (*authUser, *authSession, error) {
	if token == "" {
		return nil, nil, errors.New("sin sesión")
	}
	hash := hashToken(token)
	row := s.db.QueryRow(`SELECT s.id, s.user_id, s.expires_at, u.email, u.name, u.role, u.created_at
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=?`, hash)
	var sess authSession
	var user authUser
	var expires, created string
	if err := row.Scan(&sess.ID, &sess.UserID, &expires, &user.Email, &user.Name, &user.Role, &created); err != nil {
		return nil, nil, errors.New("sesión inválida")
	}
	sess.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.deleteSessionToken(token)
		return nil, nil, errors.New("sesión expirada")
	}
	user.ID = sess.UserID
	user.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &user, &sess, nil
}

func (s *authStore) deleteSessionToken(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, hashToken(token))
	return err
}

func (s *authStore) memberships(userID string) ([]authMembership, error) {
	rows, err := s.db.Query(`SELECT bm.business_id, COALESCE(b.name, bm.business_id), COALESCE(bm.role, 'staff'), bm.audience, bm.scope_json
		FROM business_memberships bm LEFT JOIN businesses b ON b.id=bm.business_id
		WHERE bm.user_id=? ORDER BY b.name, bm.business_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []authMembership{}
	for rows.Next() {
		var m authMembership
		var raw string
		if err := rows.Scan(&m.BusinessID, &m.BusinessName, &m.Role, &m.Audience, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &m.Scope)
		if m.Scope == nil {
			m.Scope = map[string]any{}
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *authStore) membership(userID, businessID string) (authMembership, error) {
	row := s.db.QueryRow(`SELECT bm.business_id, COALESCE(b.name, bm.business_id), COALESCE(bm.role, 'staff'), bm.audience, bm.scope_json
		FROM business_memberships bm LEFT JOIN businesses b ON b.id=bm.business_id
		WHERE bm.user_id=? AND bm.business_id=?`, userID, businessID)
	var m authMembership
	var raw string
	if err := row.Scan(&m.BusinessID, &m.BusinessName, &m.Role, &m.Audience, &raw); err != nil {
		return authMembership{}, err
	}
	_ = json.Unmarshal([]byte(raw), &m.Scope)
	if m.Scope == nil {
		m.Scope = map[string]any{}
	}
	return m, nil
}

func (s *authStore) upsertMembership(userID, businessID, role, audience string, scope map[string]any) error {
	var existing string
	err := s.db.QueryRow(`SELECT business_id FROM business_memberships WHERE user_id=? LIMIT 1`, userID).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existing != "" && existing != businessID {
		return errors.New("usuario ya pertenece a otro negocio")
	}
	if role == "" {
		role = "staff"
	}
	if audience == "" {
		audience = "user"
	}
	if scope == nil {
		scope = map[string]any{}
	}
	raw, _ := json.Marshal(scope)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`INSERT INTO business_memberships (id, user_id, business_id, role, audience, scope_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, business_id) DO UPDATE SET role=excluded.role, audience=excluded.audience, scope_json=excluded.scope_json`,
		"mem_"+randomToken(18), userID, businessID, role, audience, string(raw), now)
	return err
}

func (s *authStore) createBusiness(ownerUserID, name, domain, country string) (*authBusiness, authMembership, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, authMembership{}, errors.New("nombre de negocio requerido")
	}
	var existing string
	err := s.db.QueryRow(`SELECT business_id FROM business_memberships WHERE user_id=? LIMIT 1`, ownerUserID).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return nil, authMembership{}, err
	}
	if existing != "" {
		return nil, authMembership{}, errors.New("usuario ya pertenece a otro negocio")
	}
	now := time.Now().UTC()
	business := &authBusiness{
		ID:               "biz_" + randomToken(12),
		Name:             name,
		Domain:           strings.TrimSpace(domain),
		Country:          strings.TrimSpace(country),
		DefaultFramework: "sabio",
		OwnerUserID:      ownerUserID,
		CreatedAt:        now,
	}
	_, err = s.db.Exec(`INSERT INTO businesses (id, name, domain, country, default_framework, owner_user_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		business.ID, business.Name, business.Domain, business.Country, business.DefaultFramework, business.OwnerUserID, business.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, authMembership{}, err
	}
	if err := s.upsertMembership(ownerUserID, business.ID, "owner", "admin", map[string]any{"mode": "setup"}); err != nil {
		return nil, authMembership{}, err
	}
	membership, err := s.membership(ownerUserID, business.ID)
	if err != nil {
		return nil, authMembership{}, err
	}
	return business, membership, nil
}

func (s *authStore) business(businessID string) (*authBusiness, error) {
	businessID = strings.TrimSpace(businessID)
	if businessID == "" {
		return nil, sql.ErrNoRows
	}
	var business authBusiness
	var createdAt string
	err := s.db.QueryRow(`SELECT id, name, domain, country, default_framework, owner_user_id, created_at FROM businesses WHERE id = ?`, businessID).
		Scan(&business.ID, &business.Name, &business.Domain, &business.Country, &business.DefaultFramework, &business.OwnerUserID, &createdAt)
	if err != nil {
		return nil, err
	}
	business.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &business, nil
}

func (s *authStore) listBusinesses() ([]authBusiness, error) {
	rows, err := s.db.Query(`SELECT id, name, domain, country, default_framework, owner_user_id, created_at FROM businesses ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var businesses []authBusiness
	for rows.Next() {
		var business authBusiness
		var createdAt string
		if err := rows.Scan(&business.ID, &business.Name, &business.Domain, &business.Country, &business.DefaultFramework, &business.OwnerUserID, &createdAt); err != nil {
			return nil, err
		}
		business.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		businesses = append(businesses, business)
	}
	if businesses == nil {
		businesses = []authBusiness{}
	}
	return businesses, rows.Err()
}

func (s *authStore) listUsersOverview() ([]remoraUserOverview, error) {
	rows, err := s.db.Query(`SELECT id, email, name, role, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []remoraUserOverview{}
	for rows.Next() {
		var u remoraUserOverview
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		memberships, err := s.memberships(u.ID)
		if err != nil {
			return nil, err
		}
		u.Memberships = memberships
		u.MembershipCount = len(memberships)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *authStore) listRemoraTeam() ([]remoraUserOverview, error) {
	users, err := s.listUsersOverview()
	if err != nil {
		return nil, err
	}
	out := []remoraUserOverview{}
	for _, u := range users {
		switch u.Role {
		case "remora_staff", "remora_admin", "developer":
			out = append(out, u)
		}
	}
	return out, nil
}

func (s *authStore) businessMembers(businessID string) ([]authBusinessMember, error) {
	rows, err := s.db.Query(`SELECT u.id, u.email, u.name, bm.role, bm.audience, bm.scope_json, bm.created_at
		FROM business_memberships bm JOIN users u ON u.id=bm.user_id
		WHERE bm.business_id=? ORDER BY bm.created_at`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []authBusinessMember{}
	for rows.Next() {
		var m authBusinessMember
		var raw string
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.Audience, &raw, &m.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &m.Scope)
		if m.Scope == nil {
			m.Scope = map[string]any{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func canManageBusiness(role string) bool {
	switch role {
	case "owner", "admin", "staff":
		return true
	default:
		return false
	}
}

func (s *authStore) createInvite(businessID, createdBy, role, audience string, scope map[string]any) (*authBusinessInvite, string, error) {
	if role == "" {
		role = "staff"
	}
	if audience == "" {
		audience = "staff"
	}
	if scope == nil {
		scope = map[string]any{"mode": "setup"}
	}
	code := strings.ToUpper(randomToken(8))[:8]
	raw, _ := json.Marshal(scope)
	now := time.Now().UTC()
	inv := &authBusinessInvite{ID: "inv_" + randomToken(12), BusinessID: businessID, Token: "ivt_" + randomToken(18), Role: role, Audience: audience, Scope: scope, CreatedBy: createdBy, ExpiresAt: now.Add(7 * 24 * time.Hour), CreatedAt: now}
	_, err := s.db.Exec(`INSERT INTO business_invites (id, business_id, token, code_hash, role, audience, scope_json, created_by, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, inv.ID, inv.BusinessID, inv.Token, hashToken(code), inv.Role, inv.Audience, string(raw), inv.CreatedBy, inv.ExpiresAt.Format(time.RFC3339), inv.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, "", err
	}
	_ = s.db.QueryRow(`SELECT name FROM businesses WHERE id=?`, businessID).Scan(&inv.BusinessName)
	return inv, code, nil
}

func (s *authStore) inviteByToken(token string) (*authBusinessInvite, error) {
	row := s.db.QueryRow(`SELECT i.id, i.business_id, COALESCE(b.name, i.business_id), i.token, i.role, i.audience, i.scope_json, i.created_by, COALESCE(i.accepted_by,''), i.expires_at, i.created_at
		FROM business_invites i LEFT JOIN businesses b ON b.id=i.business_id WHERE i.token=?`, token)
	var inv authBusinessInvite
	var raw, expires, created string
	if err := row.Scan(&inv.ID, &inv.BusinessID, &inv.BusinessName, &inv.Token, &inv.Role, &inv.Audience, &raw, &inv.CreatedBy, &inv.AcceptedBy, &expires, &created); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(raw), &inv.Scope)
	if inv.Scope == nil {
		inv.Scope = map[string]any{}
	}
	inv.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	inv.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &inv, nil
}

func (s *authStore) acceptInvite(userID, token, code string) (authMembership, error) {
	inv, err := s.inviteByToken(token)
	if err != nil {
		return authMembership{}, errors.New("invitación inválida")
	}
	if inv.AcceptedBy != "" {
		return authMembership{}, errors.New("invitación ya utilizada")
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return authMembership{}, errors.New("invitación expirada")
	}
	var stored string
	if err := s.db.QueryRow(`SELECT code_hash FROM business_invites WHERE token=?`, token).Scan(&stored); err != nil {
		return authMembership{}, err
	}
	if stored != hashToken(strings.TrimSpace(code)) {
		return authMembership{}, errors.New("código inválido")
	}
	if err := s.upsertMembership(userID, inv.BusinessID, inv.Role, inv.Audience, inv.Scope); err != nil {
		return authMembership{}, err
	}
	_, _ = s.db.Exec(`UPDATE business_invites SET accepted_by=? WHERE token=?`, userID, token)
	return s.membership(userID, inv.BusinessID)
}

func normalizeRemoraRole(role string) (string, error) {
	switch role {
	case "", "remora_staff":
		return "remora_staff", nil
	case "remora_admin":
		return "remora_admin", nil
	default:
		return "", errors.New("rol Remora inválido")
	}
}

func (s *authStore) createRemoraTeamInvite(createdBy, role string) (*remoraTeamInvite, string, error) {
	role, err := normalizeRemoraRole(role)
	if err != nil {
		return nil, "", err
	}
	code := strings.ToUpper(randomToken(8))[:8]
	now := time.Now().UTC()
	inv := &remoraTeamInvite{ID: "rtinv_" + randomToken(12), Token: "rtv_" + randomToken(18), Role: role, CreatedBy: createdBy, ExpiresAt: now.Add(7 * 24 * time.Hour), CreatedAt: now}
	_, err = s.db.Exec(`INSERT INTO remora_team_invites (id, token, code_hash, role, created_by, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, inv.ID, inv.Token, hashToken(code), inv.Role, inv.CreatedBy, inv.ExpiresAt.Format(time.RFC3339), inv.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, "", err
	}
	return inv, code, nil
}

func (s *authStore) remoraTeamInviteByToken(token string) (*remoraTeamInvite, error) {
	row := s.db.QueryRow(`SELECT id, token, role, created_by, COALESCE(accepted_by,''), expires_at, created_at FROM remora_team_invites WHERE token=?`, token)
	var inv remoraTeamInvite
	var expires, created string
	if err := row.Scan(&inv.ID, &inv.Token, &inv.Role, &inv.CreatedBy, &inv.AcceptedBy, &expires, &created); err != nil {
		return nil, err
	}
	inv.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	inv.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &inv, nil
}

func (s *authStore) acceptRemoraTeamInvite(userID, token, code string) (*authUser, error) {
	inv, err := s.remoraTeamInviteByToken(token)
	if err != nil {
		return nil, errors.New("invitación Remora inválida")
	}
	if inv.AcceptedBy != "" {
		return nil, errors.New("invitación Remora ya utilizada")
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return nil, errors.New("invitación Remora expirada")
	}
	var stored string
	if err := s.db.QueryRow(`SELECT code_hash FROM remora_team_invites WHERE token=?`, token).Scan(&stored); err != nil {
		return nil, err
	}
	if stored != hashToken(strings.TrimSpace(code)) {
		return nil, errors.New("código Remora inválido")
	}
	if _, err := s.db.Exec(`UPDATE users SET role=?, updated_at=? WHERE id=?`, inv.Role, time.Now().UTC().Format(time.RFC3339), userID); err != nil {
		return nil, err
	}
	_, _ = s.db.Exec(`UPDATE remora_team_invites SET accepted_by=? WHERE token=?`, userID, token)
	row := s.db.QueryRow(`SELECT id, email, name, role, created_at FROM users WHERE id=?`, userID)
	var user authUser
	var created string
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &created); err != nil {
		return nil, err
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &user, nil
}

// upsertAuth0User crea o actualiza el usuario local a partir de un JWT de Auth0.
// El campo auth0_sub es la clave de deduplicación; el email se usa como fallback para
// encontrar usuarios existentes que aún no tienen auth0_sub (migraciones).
func (s *authStore) upsertAuth0User(auth0Sub, email, name string) (*authUser, error) {
	if auth0Sub == "" {
		return nil, errors.New("auth0_sub requerido")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" {
		// nombre por defecto: parte local del email
		if idx := strings.Index(email, "@"); idx > 0 {
			name = email[:idx]
		} else {
			name = email
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// 1. Buscar por auth0_sub
	var user authUser
	var created string
	err := s.db.QueryRow(`SELECT id, email, name, role, created_at FROM users WHERE auth0_sub=?`, auth0Sub).
		Scan(&user.ID, &user.Email, &user.Name, &user.Role, &created)
	if err == nil {
		// Encontrado — actualizar nombre/email si cambió
		_, _ = s.db.Exec(`UPDATE users SET email=?, name=?, updated_at=? WHERE auth0_sub=?`,
			email, name, now, auth0Sub)
		user.CreatedAt, _ = time.Parse(time.RFC3339, created)
		return &user, nil
	}

	// 2. Buscar por email (usuario previo sin auth0_sub — migración)
	if email != "" {
		err2 := s.db.QueryRow(`SELECT id, email, name, role, created_at FROM users WHERE lower(email)=lower(?) AND (auth0_sub IS NULL OR auth0_sub='')`, email).
			Scan(&user.ID, &user.Email, &user.Name, &user.Role, &created)
		if err2 == nil {
			// Vincular auth0_sub al usuario existente
			_, _ = s.db.Exec(`UPDATE users SET auth0_sub=?, name=?, updated_at=? WHERE id=?`,
				auth0Sub, name, now, user.ID)
			user.CreatedAt, _ = time.Parse(time.RFC3339, created)
			return &user, nil
		}
	}

	// 3. Crear nuevo usuario Auth0 (password_hash vacío — no usa password local)
	newUser := &authUser{
		ID:    "usr_" + randomToken(18),
		Email: email,
		Name:  name,
		Role:  "user",
	}
	newUser.CreatedAt = time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO users (id, email, password_hash, name, role, auth0_sub, created_at, updated_at) VALUES (?, ?, '', ?, ?, ?, ?, ?)`,
		newUser.ID, newUser.Email, newUser.Name, newUser.Role, auth0Sub, now, now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			// Raza de condición — reintento por sub
			return s.upsertAuth0User(auth0Sub, email, name)
		}
		return nil, err
	}
	return newUser, nil
}

func authTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(authCookieName); err == nil {
		return c.Value
	}
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}

func setAuthCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   os.Getenv("REMORA_AUTH_SECURE_COOKIE") == "true",
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   os.Getenv("REMORA_AUTH_SECURE_COOKIE") == "true",
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

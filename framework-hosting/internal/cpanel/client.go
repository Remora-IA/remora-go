// Package cpanel implementa un cliente mínimo para la cPanel UAPI vía HTTPS.
//
// La autenticación con cPanel usa "session-based auth" + "security_token":
//
//  1. POST /login/?login_only=1 con {user,pass} → devuelve JSON con
//     security_token (formato "/cpsessXXXXX") y guarda cookie "cpsession".
//  2. Toda llamada UAPI: GET https://host:2083{security_token}/execute/<Module>/<Function>
//     incluyendo el cookie cpsession.
//
// La sesión expira por inactividad y queda atada a la IP que hizo el login,
// así que el cliente reauth automáticamente al detectar 401/403/HTML.
//
// Doc oficial UAPI: https://api.docs.cpanel.net/openapi/cpanel/operation/list_pops/
package cpanel

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client habla cPanel UAPI sobre HTTPS.
//
// Credenciales:
//   - Host: dominio o IP del servidor cPanel (ej: "hosting.empresa.cl").
//     NO incluir puerto ni esquema. Por defecto usa :2083 (cPanel admin).
//   - User: nombre de usuario cPanel. Algunos hostings aceptan el email
//     como alias (ej: "tomashigh@patriciastocker.com").
//   - Pass: password o API token (ver Authorization).
//
// Concurrencia: el zero-value NO es seguro para uso concurrente. Cada
// goroutine debería tener su propio Client, o sincronizar las llamadas
// porque la sesión + token mutan tras Login() y reauth.
type Client struct {
	Host     string // ej: "patriciastocker.com" (sin esquema, sin puerto)
	Port     int    // default 2083
	User     string
	Pass     string
	Insecure bool // true = saltar verificación TLS (self-signed). Default false.

	mu       sync.Mutex
	http     *http.Client
	jar      *cookiejar.Jar
	token    string    // "/cpsessXXXXX" tras login
	loggedAt time.Time // para detectar sesiones obsoletas
}

// LoginResponse es el JSON que cPanel devuelve en /login/?login_only=1.
type loginResponse struct {
	Status        int      `json:"status"` // 1 = éxito, 0 = falló
	SecurityToken string   `json:"security_token"`
	Redirect      string   `json:"redirect"`
	Notices       []string `json:"notices"`
}

// UAPIResponse es el envoltorio estándar de cualquier llamada UAPI.
// Ver https://api.docs.cpanel.net/guides/uapi/uapi-output-format/
type UAPIResponse struct {
	Status   int             `json:"status"` // 1 = éxito
	Errors   []string        `json:"errors"`
	Messages []string        `json:"messages"`
	Data     json.RawMessage `json:"data"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// New construye un Client. No realiza I/O — el primer Login() abre la sesión.
//
// Si insecure=true, el cliente acepta certificados self-signed (común en
// hostings con Let's Encrypt mal configurado o IPs directas). Default: false.
func New(host, user, pass string, insecure bool) (*Client, error) {
	host = normalizeHost(host)
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("cpanel: host/user/pass son requeridos")
	}
	if isPlaceholderHost(host) {
		return nil, fmt.Errorf("cpanel: host de ejemplo no permitido (%s); usa el dominio real del negocio", host)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cpanel: cookiejar: %w", err)
	}
	return &Client{
		Host:     host,
		Port:     2083,
		User:     user,
		Pass:     pass,
		Insecure: insecure,
		jar:      jar,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
			},
		},
	}, nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil && u.Host != "" {
			host = u.Host
		}
	}
	host = strings.TrimPrefix(host, "https//")
	host = strings.TrimPrefix(host, "http//")
	host = strings.Trim(host, "/")
	if h, _, err := strings.Cut(host, ":"); err && h != "" {
		host = h
	}
	return strings.TrimSpace(host)
}

func isPlaceholderHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "ejemplo.com" || host == "cpanel.ejemplo.com" || host == "example.com" || host == "cpanel.example.com" || strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".ejemplo.com")
}

// baseURL devuelve "https://<host>:<port>" sin slash final.
func (c *Client) baseURL() string {
	port := c.Port
	if port == 0 {
		port = 2083
	}
	return fmt.Sprintf("https://%s:%d", c.Host, port)
}

// Login autentica y guarda el security_token + cookies. Si ya hay sesión
// válida (<5min) reutiliza. Llamar antes de Call(); Call() también lo invoca
// transparentemente si detecta sesión expirada.
func (c *Client) Login() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loginLocked()
}

func (c *Client) loginLocked() error {
	form := url.Values{}
	form.Set("user", c.User)
	form.Set("pass", c.Pass)

	loginURL := c.baseURL() + "/login/?login_only=1"
	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("cpanel login: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cpanel login: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 401 = credenciales malas. 200 con HTML = redirect a form (también credenciales malas).
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("cpanel login: credenciales rechazadas (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cpanel login: HTTP %d: %s", resp.StatusCode, snippet(body))
	}
	if !looksLikeJSON(body) {
		return fmt.Errorf("cpanel login: respuesta no-JSON (HTML?). Posible login form: %s", snippet(body))
	}

	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return fmt.Errorf("cpanel login: parse JSON: %w (body=%s)", err, snippet(body))
	}
	if lr.Status != 1 || lr.SecurityToken == "" {
		return fmt.Errorf("cpanel login: status=%d notices=%v", lr.Status, lr.Notices)
	}

	c.token = lr.SecurityToken
	c.loggedAt = time.Now()
	return nil
}

// Call invoca un endpoint UAPI: <Module>/<Function>(params).
//
// params se serializa como query string (UAPI acepta GET con query). Para
// argumentos con caracteres especiales url.Values se encarga del encoding.
//
// Si la sesión expiró, Call hace reauth y reintenta UNA vez.
func (c *Client) Call(module, function string, params url.Values) (*UAPIResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token == "" {
		if err := c.loginLocked(); err != nil {
			return nil, err
		}
	}

	resp, body, err := c.callLocked(module, function, params)
	if err != nil {
		return nil, err
	}

	// Detectar sesión expirada: HTML en lugar de JSON, o 401/403.
	if resp.StatusCode == 401 || resp.StatusCode == 403 || !looksLikeJSON(body) {
		// reauth + retry una vez
		if err := c.loginLocked(); err != nil {
			return nil, fmt.Errorf("cpanel call: sesión expirada y reauth falló: %w", err)
		}
		resp, body, err = c.callLocked(module, function, params)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cpanel call %s/%s: HTTP %d: %s", module, function, resp.StatusCode, snippet(body))
	}

	var ur UAPIResponse
	if err := json.Unmarshal(body, &ur); err != nil {
		return nil, fmt.Errorf("cpanel call %s/%s: parse JSON: %w (body=%s)", module, function, err, snippet(body))
	}
	if ur.Status != 1 {
		return &ur, fmt.Errorf("cpanel %s/%s: status=%d errors=%v", module, function, ur.Status, ur.Errors)
	}
	return &ur, nil
}

func (c *Client) callLocked(module, function string, params url.Values) (*http.Response, []byte, error) {
	endpoint := fmt.Sprintf("%s%s/execute/%s/%s", c.baseURL(), c.token, module, function)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("cpanel build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("cpanel http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body, nil
}

// Logout invalida la sesión en el servidor (best-effort).
func (c *Client) Logout() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" {
		return
	}
	logoutURL := c.baseURL() + c.token + "/logout"
	req, _ := http.NewRequest(http.MethodGet, logoutURL, nil)
	if req != nil {
		_, _ = c.http.Do(req) // ignoramos errores
	}
	c.token = ""
}

// looksLikeJSON detecta heurísticamente si un cuerpo HTTP es JSON válido.
func looksLikeJSON(b []byte) bool {
	for _, c := range b {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		return c == '{' || c == '['
	}
	return false
}

// snippet devuelve los primeros N bytes de un cuerpo HTTP para mensajes de
// error legibles, sin volcar HTML completo a logs.
func snippet(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

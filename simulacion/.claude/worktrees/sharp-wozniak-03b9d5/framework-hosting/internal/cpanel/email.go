package cpanel

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// EmailAccount es una cuenta de correo (cuenta POP/IMAP) listada por
// Email/list_pops. Solo incluye los campos básicos que usa el POC.
type EmailAccount struct {
	Email             string `json:"email"`
	Login             string `json:"login"`
	SuspendedLogin    int    `json:"suspended_login"`
	SuspendedIncoming int    `json:"suspended_incoming"`
}

// ListEmailAccounts devuelve las cuentas de correo del dominio principal de
// la cuenta cPanel. Sin parámetros, lista TODAS las cuentas accesibles al
// usuario logueado.
//
// Endpoint: UAPI Email/list_pops
// Doc: https://api.docs.cpanel.net/openapi/cpanel/operation/list_pops/
func (c *Client) ListEmailAccounts() ([]EmailAccount, error) {
	resp, err := c.Call("Email", "list_pops", url.Values{})
	if err != nil {
		return nil, err
	}
	var accounts []EmailAccount
	if err := json.Unmarshal(resp.Data, &accounts); err != nil {
		return nil, fmt.Errorf("parse list_pops data: %w", err)
	}
	return accounts, nil
}

// Ping verifica que la sesión funciona haciendo una llamada barata.
// StatsBar/get_stats es liviano y siempre disponible para usuarios cPanel
// (devuelve estadísticas generales como uso de disco).
func (c *Client) Ping() error {
	_, err := c.Call("StatsBar", "get_stats", url.Values{})
	return err
}

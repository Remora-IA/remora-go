package cpanel

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// AddPopParams son los parámetros mínimos para crear un buzón POP/IMAP
// vía UAPI Email/add_pop. cPanel además expone Email/add_email pero
// add_pop es estable en todas las versiones soportadas.
//
// Doc: https://api.docs.cpanel.net/openapi/cpanel/operation/add_pop/
type AddPopParams struct {
	// Email es la parte local (antes de la @), ej: "cobranza".
	Email string
	// Domain es el dominio del buzón, ej: "patriciastocker.com".
	Domain string
	// Password del buzón. Debe cumplir la política de seguridad del cPanel
	// (cPanel rechaza con error 1 si la fuerza es insuficiente).
	Password string
	// Quota en MB. 0 = ilimitada. Default sugerido: 250.
	QuotaMB int
}

// AddPop crea un buzón de email en el servidor cPanel. Devuelve la dirección
// completa creada (local@domain) y el host SMTP/IMAP a usar (mail.<domain>).
// Si el buzón ya existe, cPanel devuelve un error que reportamos sin tocar
// nada (idempotencia se maneja arriba: el caller debería chequear primero
// con ListEmailAccounts si quiere evitar el error).
func (c *Client) AddPop(p AddPopParams) (string, error) {
	if p.Email == "" || p.Domain == "" || p.Password == "" {
		return "", fmt.Errorf("cpanel add_pop: email, domain y password son requeridos")
	}
	q := p.QuotaMB
	if q <= 0 {
		q = 250
	}
	form := url.Values{}
	form.Set("email", p.Email)
	form.Set("domain", p.Domain)
	form.Set("password", p.Password)
	form.Set("quota", fmt.Sprintf("%d", q))

	resp, err := c.Call("Email", "add_pop", form)
	if err != nil {
		return "", err
	}
	// Algunos cPanel devuelven data:null en éxito. No es un error.
	_ = json.Unmarshal(resp.Data, new(interface{}))
	return p.Email + "@" + p.Domain, nil
}

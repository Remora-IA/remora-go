// Package vault es el almacén de secretos compartido entre frameworks.
//
// Contrato:
//   - Cada secreto se identifica por (conv_id, capability) donde capability
//     es un nombre canónico documentado en docs/CAPABILITIES.md
//     (ej "credentials.smtp", "credentials.twilio").
//   - Los valores se serializan a JSON y se cifran con AES-256-GCM usando la
//     clave maestra de la env var REMORA_VAULT_KEY (con fallback a
//     HOSTING_VAULT_KEY para back-compat con instalaciones existentes).
//   - El vault NUNCA escribe plaintext. Si la clave maestra falta, falla
//     cerrado (refuse to operate).
//   - Layout en disco: <baseDir>/<conv_id>/<capability>.enc
//     baseDir por defecto: el directorio definido por env REMORA_VAULT_DIR,
//     o "channel/vault_data" relativo al cwd como último recurso.
//
// Este paquete es la fuente única de verdad. Frameworks no deben
// implementar su propio cifrado: deben shellear al binario `vault` o
// importar este paquete.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound se devuelve cuando la capability no está guardada para la
// conversación pedida. Permite que callers distingan "no hay credenciales"
// de un error real (clave incorrecta, disco corrupto, etc.).
var ErrNotFound = errors.New("vault: capability not found")

// ErrNoMasterKey indica que la env var REMORA_VAULT_KEY (o HOSTING_VAULT_KEY)
// no está configurada. Operación rechazada por seguridad.
var ErrNoMasterKey = errors.New("vault: REMORA_VAULT_KEY not set (refuse to operate)")

// DefaultBaseDir devuelve el directorio raíz donde se guardan los secretos.
// Resolución: REMORA_VAULT_DIR env > "channel/vault_data".
func DefaultBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("REMORA_VAULT_DIR")); v != "" {
		return v
	}
	return filepath.Join("channel", "vault_data")
}

// Path arma el path absoluto donde se guarda <conv_id>/<capability>.enc.
// Sanitiza ambos componentes para evitar path traversal y caracteres raros
// que el filesystem no acepte.
func Path(baseDir, convID, capability string) string {
	if baseDir == "" {
		baseDir = DefaultBaseDir()
	}
	if convID == "" {
		convID = "default"
	}
	return filepath.Join(baseDir, sanitize(convID), sanitize(capability)+".enc")
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= 'A' && ch <= 'Z',
			ch >= '0' && ch <= '9',
			ch == '_', ch == '-', ch == '.':
			out = append(out, ch)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "_"
	}
	return string(out)
}

// masterKey lee y decodifica REMORA_VAULT_KEY (o HOSTING_VAULT_KEY).
// Acepta hex (64 chars) o base64 (44 chars std/url-safe).
func masterKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("REMORA_VAULT_KEY"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("HOSTING_VAULT_KEY"))
	}
	if raw == "" {
		return nil, ErrNoMasterKey
	}
	if len(raw) == 64 {
		if k, err := hex.DecodeString(raw); err == nil && len(k) == 32 {
			return k, nil
		}
	}
	if k, err := base64.StdEncoding.DecodeString(raw); err == nil && len(k) == 32 {
		return k, nil
	}
	if k, err := base64.URLEncoding.DecodeString(raw); err == nil && len(k) == 32 {
		return k, nil
	}
	return nil, fmt.Errorf("vault: master key debe ser 32 bytes (hex o base64), got len=%d", len(raw))
}

// Set guarda value (JSON ya serializado o cualquier blob) cifrado bajo
// (convID, capability). Sobrescribe si ya existía.
func Set(baseDir, convID, capability string, value []byte) error {
	key, err := masterKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("vault: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("vault: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("vault: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, value, nil)

	p := Path(baseDir, convID, capability)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return fmt.Errorf("vault: mkdir: %w", err)
	}
	if err := os.WriteFile(p, sealed, 0600); err != nil {
		return fmt.Errorf("vault: write: %w", err)
	}
	return nil
}

// Get lee y desencripta el valor bajo (convID, capability).
// Si no existe, devuelve ErrNotFound.
func Get(baseDir, convID, capability string) ([]byte, error) {
	p := Path(baseDir, convID, capability)
	sealed, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("vault: read: %w", err)
	}
	key, err := masterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault: gcm: %w", err)
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("vault: ciphertext truncated")
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt (clave incorrecta?): %w", err)
	}
	return plain, nil
}

// Has devuelve true si la capability está guardada para la conversación.
// No descifra: es un check barato que solo mira si el archivo existe.
func Has(baseDir, convID, capability string) bool {
	p := Path(baseDir, convID, capability)
	_, err := os.Stat(p)
	return err == nil
}

// List devuelve la lista de capabilities guardadas para una conversación.
// Útil para debugging y para que el orquestador sepa qué creds existen
// antes de ejecutar una acción.
func List(baseDir, convID string) ([]string, error) {
	if baseDir == "" {
		baseDir = DefaultBaseDir()
	}
	if convID == "" {
		convID = "default"
	}
	dir := filepath.Join(baseDir, sanitize(convID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("vault: readdir: %w", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".enc") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".enc"))
	}
	return out, nil
}

// Delete elimina la capability del vault. Idempotente.
func Delete(baseDir, convID, capability string) error {
	p := Path(baseDir, convID, capability)
	err := os.Remove(p)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("vault: remove: %w", err)
	}
	return nil
}

// GenerateKey produce una clave maestra AES-256 en hex (64 chars).
func GenerateKey() (string, error) {
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return "", err
	}
	return hex.EncodeToString(k), nil
}

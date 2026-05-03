// Package creds maneja persistencia encriptada de credenciales de hosting.
//
// Las credenciales son sensibles (acceso completo al hosting del cliente).
// Reglas no negociables:
//
//   - NUNCA en JSON plano.
//   - Encriptación AES-256-GCM con clave maestra de env var HOSTING_VAULT_KEY
//     (32 bytes hex o base64). Si no está seteada, refuse to save (fail-closed).
//   - El binario JAMÁS loguea password/token, solo host y user.
//   - Cada conversación tiene su propio archivo: temp/creds_<convID>.enc
package creds

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Credentials es el bundle que guardamos por conversación.
type Credentials struct {
	Panel    string `json:"panel"` // "cpanel" | "plesk" | "directadmin" (futuro)
	Host     string `json:"host"`  // ej "patriciastocker.com"
	Port     int    `json:"port"`  // ej 2083
	User     string `json:"user"`
	Pass     string `json:"pass"`     // password o token (sensible)
	Insecure bool   `json:"insecure"` // saltar verificación TLS
}

// Path arma el path del archivo encriptado para una conversación.
// Si baseDir está vacío, usa "temp/" relativo al cwd del binario.
func Path(baseDir, convID string) string {
	if baseDir == "" {
		baseDir = "temp"
	}
	if convID == "" {
		convID = "default"
	}
	// Sanitizar convID por las dudas: solo [a-zA-Z0-9_-]
	safe := make([]byte, 0, len(convID))
	for i := 0; i < len(convID); i++ {
		ch := convID[i]
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= 'A' && ch <= 'Z',
			ch >= '0' && ch <= '9',
			ch == '_', ch == '-':
			safe = append(safe, ch)
		default:
			safe = append(safe, '_')
		}
	}
	return filepath.Join(baseDir, "creds_"+string(safe)+".enc")
}

// vaultKey lee y decodifica la clave maestra de env. Acepta hex (64 chars)
// o base64 (44 chars). Devuelve exactamente 32 bytes (AES-256).
func vaultKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("HOSTING_VAULT_KEY"))
	if raw == "" {
		return nil, fmt.Errorf("HOSTING_VAULT_KEY no seteada (refuse to save plaintext credentials)")
	}
	// Probar hex primero
	if len(raw) == 64 {
		k, err := hex.DecodeString(raw)
		if err == nil && len(k) == 32 {
			return k, nil
		}
	}
	// Base64
	k, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && len(k) == 32 {
		return k, nil
	}
	// URL-safe base64
	k, err = base64.URLEncoding.DecodeString(raw)
	if err == nil && len(k) == 32 {
		return k, nil
	}
	return nil, fmt.Errorf("HOSTING_VAULT_KEY debe ser 32 bytes en hex (64 chars) o base64 (44 chars), got len=%d", len(raw))
}

// Save encripta y persiste credenciales en disco.
func Save(path string, c *Credentials) error {
	key, err := vaultKey()
	if err != nil {
		return err
	}
	plain, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal creds: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	// 0600: solo owner lee/escribe
	if err := os.WriteFile(path, sealed, 0600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// Load desencripta y devuelve las credenciales de disco.
// Devuelve (nil, os.ErrNotExist) si no hay archivo aún.
func Load(path string) (*Credentials, error) {
	sealed, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := vaultKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("sealed data too short")
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt (clave maestra incorrecta?): %w", err)
	}
	var c Credentials
	if err := json.Unmarshal(plain, &c); err != nil {
		return nil, fmt.Errorf("unmarshal creds: %w", err)
	}
	return &c, nil
}

// GenerateKey genera una clave AES-256 aleatoria en hex (64 chars).
// Útil para el setup inicial: el usuario puede correr `frameworkhosting genkey`
// para obtener una clave que pega en HOSTING_VAULT_KEY.
func GenerateKey() (string, error) {
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return "", err
	}
	return hex.EncodeToString(k), nil
}

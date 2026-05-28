package main

// auth0_jwt.go — validación manual de JWT RS256 emitidos por Auth0.
// No requiere librería externa; usa solo crypto/rsa + crypto/x509 de la stdlib.
//
// Flujo:
//   1. Parse del JWT (header.payload.signature)
//   2. Fetch de JWKS desde Auth0 (cacheado 24h en memoria)
//   3. Verificación de firma RSA-SHA256
//   4. Verificación de exp, iss, aud
//   5. Extracción de sub + email

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ── JWKS cache ──────────────────────────────────────────────────────────────

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	// x5c contiene el certificado X.509 en base64 (alternativa al N/E)
	X5c []string `json:"x5c"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type jwksCache struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
	ttl     time.Duration
}

var globalJWKSCache = &jwksCache{ttl: 24 * time.Hour}

func (c *jwksCache) getKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if c.keys != nil && time.Since(c.fetched) < c.ttl {
		k := c.keys[kid]
		c.mu.RUnlock()
		if k != nil {
			return k, nil
		}
		return nil, fmt.Errorf("kid %q no encontrado en JWKS (caché)", kid)
	}
	c.mu.RUnlock()

	// Refrescar
	domain := auth0Domain()
	url := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read JWKS body: %w", err)
	}
	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("parse JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k)
		if err != nil {
			continue
		}
		newKeys[k.Kid] = pub
	}

	c.mu.Lock()
	c.keys = newKeys
	c.fetched = time.Now()
	c.mu.Unlock()

	if pub, ok := newKeys[kid]; ok {
		return pub, nil
	}
	return nil, fmt.Errorf("kid %q no encontrado en JWKS", kid)
}

func rsaPublicKeyFromJWK(k jwksKey) (*rsa.PublicKey, error) {
	// Preferir x5c si está disponible
	if len(k.X5c) > 0 {
		certBytes, err := base64.StdEncoding.DecodeString(k.X5c[0])
		if err == nil {
			cert, err := x509.ParseCertificate(certBytes)
			if err == nil {
				if pub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
					return pub, nil
				}
			}
		}
	}

	// Derivar de N + E
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode E: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// ── JWT parsing ─────────────────────────────────────────────────────────────

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type jwtClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Iss   string `json:"iss"`
	Aud   any    `json:"aud"` // string o []string
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
}

func (c *jwtClaims) audiences() []string {
	switch v := c.Aud.(type) {
	case string:
		return []string{v}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// validateAuth0JWT valida un JWT RS256 de Auth0.
// Devuelve (sub, email, error).
func validateAuth0JWT(tokenString string) (sub string, email string, err error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", "", errors.New("JWT malformado: se esperan 3 partes")
	}

	// Decodificar header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("decode header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", "", fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "RS256" {
		return "", "", fmt.Errorf("algoritmo no soportado: %s", header.Alg)
	}

	// Obtener clave pública
	pubKey, err := globalJWKSCache.getKey(header.Kid)
	if err != nil {
		return "", "", fmt.Errorf("JWKS: %w", err)
	}

	// Verificar firma
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", fmt.Errorf("decode signature: %w", err)
	}
	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sigBytes); err != nil {
		return "", "", errors.New("firma JWT inválida")
	}

	// Decodificar claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("decode payload: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", "", fmt.Errorf("parse claims: %w", err)
	}

	// Verificar expiración
	if time.Now().Unix() > claims.Exp {
		return "", "", errors.New("token expirado")
	}

	// Verificar issuer
	expectedIss := fmt.Sprintf("https://%s/", auth0Domain())
	if claims.Iss != expectedIss {
		return "", "", fmt.Errorf("iss inválido: %s", claims.Iss)
	}

	if claims.Sub == "" {
		return "", "", errors.New("sub vacío en token")
	}

	return claims.Sub, claims.Email, nil
}

func auth0Domain() string {
	return envOr("AUTH0_DOMAIN", "remora-ia.us.auth0.com")
}

// ── PEM export (no usado en producción, útil para tests) ────────────────────

func rsaPublicKeyToPEM(pub *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// _ es para evitar "imported and not used" en caso de que rand no se use
var _ = rand.Reader

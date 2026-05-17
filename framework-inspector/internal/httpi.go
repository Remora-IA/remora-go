package internal

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// TestEndpoint hace un GET al URL con las credenciales dadas e inspecciona la respuesta completa.
func TestEndpoint(rawURL, token, authHeader string) *TestResult {
	if authHeader == "" {
		authHeader = "Authorization"
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return &TestResult{
			Success:   false,
			ErrorMsg:  fmt.Sprintf("URL inválida: %v", err),
			Diagnosis: DiagnoseError("invalid_url", 0, "", err.Error()),
		}
	}

	req.Header.Set("Accept", "application/json, */*")
	req.Header.Set("User-Agent", "Remora-Inspector/0.1")
	if token != "" {
		val := token
		if authHeader == "Authorization" && !strings.HasPrefix(strings.ToLower(token), "bearer ") && !strings.HasPrefix(strings.ToLower(token), "basic ") {
			val = "Bearer " + token
		}
		req.Header.Set(authHeader, val)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &TestResult{
			Success:   false,
			LatencyMS: latency,
			ErrorMsg:  err.Error(),
			Diagnosis: DiagnoseError("network", 0, "", err.Error()),
		}
	}
	defer resp.Body.Close()

	// Dump completo de la respuesta
	dump, _ := httputil.DumpResponse(resp, true)
	dumpStr := string(dump)
	if len(dumpStr) > 2000 {
		dumpStr = dumpStr[:2000] + "\n[... truncado ...]"
	}

	// Body snippet
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodySnippet := string(bodyBytes)
	if len(bodySnippet) > 500 {
		bodySnippet = bodySnippet[:500] + "…"
	}

	// Headers relevantes
	headers := map[string]string{}
	for _, h := range []string{"Content-Type", "WWW-Authenticate", "X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After", "Server"} {
		if v := resp.Header.Get(h); v != "" {
			headers[h] = v
		}
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	diagnosis := DiagnoseHTTP(resp.StatusCode, headers, bodySnippet)

	return &TestResult{
		StatusCode:  resp.StatusCode,
		Headers:     headers,
		BodySnippet: bodySnippet,
		FullDump:    dumpStr,
		Success:     success,
		LatencyMS:   latency,
		Diagnosis:   diagnosis,
	}
}

// DiagnoseHTTP interpreta un status HTTP y genera un mensaje humano con causa probable y siguiente paso.
func DiagnoseHTTP(status int, headers map[string]string, body string) string {
	bodyLower := strings.ToLower(body)

	switch {
	case status >= 200 && status < 300:
		return fmt.Sprintf("✓ Respuesta exitosa (%d). La conexión funciona correctamente.", status)

	case status == 301 || status == 302 || status == 307 || status == 308:
		return fmt.Sprintf("↪ Redirección (%d). La URL base puede haber cambiado. Revisá si hay una URL más nueva en la documentación.", status)

	case status == 400:
		if strings.Contains(bodyLower, "required") || strings.Contains(bodyLower, "missing") {
			return "✗ 400 Bad Request: El endpoint requiere parámetros adicionales que no estamos mandando en el test inicial. Es normal para un GET básico."
		}
		return "✗ 400 Bad Request: La solicitud tiene un formato incorrecto. Revisá la documentación del endpoint base."

	case status == 401:
		hint := ""
		if wwwAuth := headers["WWW-Authenticate"]; wwwAuth != "" {
			hint = fmt.Sprintf(" El servidor pide: %s.", wwwAuth)
		}
		return fmt.Sprintf("✗ 401 Unauthorized: El token de autenticación es incorrecto o falta.%s Necesitamos el token correcto.", hint)

	case status == 403:
		return "✗ 403 Forbidden: Autenticación válida pero sin permisos para este endpoint. El token puede no tener los scopes necesarios."

	case status == 404:
		return "✗ 404 Not Found: La URL base no existe. Revisá que la ruta sea correcta — a veces tiene /api/v1, /v2, /rest, etc."

	case status == 405:
		return "✗ 405 Method Not Allowed: Este endpoint no acepta GET. Puede ser un endpoint solo de POST. Es normal para el URL base."

	case status == 422:
		return "✗ 422 Unprocessable: El servidor entendió la request pero le faltan datos. La autenticación parece funcionar."

	case status == 429:
		retry := headers["Retry-After"]
		if retry != "" {
			return fmt.Sprintf("✗ 429 Rate Limited: Demasiadas requests. Espera %s segundos antes de reintentar.", retry)
		}
		return "✗ 429 Rate Limited: Límite de requests excedido. Esperá unos segundos y volvé a intentar."

	case status >= 500:
		return fmt.Sprintf("✗ %d Error del servidor: La API tiene un problema interno. No es un error de nuestra conexión — la API está respondiendo, simplemente con error propio.", status)

	case status == 0:
		return "✗ Sin respuesta: No pude conectarme al servidor. Posibles causas: URL incorrecta, servidor caído, o problema de red/firewall."

	default:
		return fmt.Sprintf("Status %d: Respuesta no estándar. Revisá la documentación para entender qué significa.", status)
	}
}

func DiagnoseError(kind string, status int, headers, errMsg string) string {
	errLower := strings.ToLower(errMsg)
	switch kind {
	case "invalid_url":
		return "✗ URL inválida: Verificá que empiece con https:// o http:// y que el formato sea correcto."
	case "network":
		if strings.Contains(errLower, "no such host") || strings.Contains(errLower, "dns") {
			return "✗ DNS no resuelve: El dominio no existe o no es accesible. Verificá el nombre del host."
		}
		if strings.Contains(errLower, "timeout") {
			return "✗ Timeout: El servidor no respondió en 15 segundos. Puede estar caído o bloqueando conexiones."
		}
		if strings.Contains(errLower, "connection refused") {
			return "✗ Conexión rechazada: El servidor está activo pero rechazó la conexión. Verificá el puerto y protocolo."
		}
		if strings.Contains(errLower, "certificate") || strings.Contains(errLower, "tls") {
			return "✗ Error SSL/TLS: Problema con el certificado del servidor. La API puede usar HTTP en lugar de HTTPS."
		}
		return fmt.Sprintf("✗ Error de red: %s", errMsg)
	}
	return errMsg
}

package internal

import (
	"log"
)

// LogRequest registra información del request (Axioma 10)
// NUNCA loguea información sensible (API key completa, passwords, paths externos)
func LogRequest(method string, apiKeyObfuscated string, command string, args []string, baseDir string, exitCode int, durationMs int64, success bool) {
	// API Key solo muestra últimos 4 caracteres (Axioma 10)
	log.Printf(
		"REQUEST method=%s api_key=***%s cmd=%s args=%v base_dir=%s exit_code=%d duration_ms=%d success=%v",
		method,
		apiKeyObfuscated,
		command,
		args,
		baseDir,
		exitCode,
		durationMs,
		success,
	)
}

// LogSecurityReject registra rechazos de seguridad (Axioma 10)
func LogSecurityReject(method string, apiKeyObfuscated string, reason string) {
	log.Printf(
		"SECURITY_REJECT method=%s api_key=***%s reason=%s",
		method,
		apiKeyObfuscated,
		reason,
	)
}

// LogJSONRPCError registra errores de parseo JSON-RPC (Axioma 6, 10)
func LogJSONRPCError(reason string) {
	log.Printf("JSONRPC_ERROR reason=%s", reason)
}

// ObfuscateAPIKey devuelve los últimos 4 caracteres de la API key (Axioma 10)
func ObfuscateAPIKey(apiKey string) string {
	if len(apiKey) <= 4 {
		return "****"
	}
	return apiKey[len(apiKey)-4:]
}

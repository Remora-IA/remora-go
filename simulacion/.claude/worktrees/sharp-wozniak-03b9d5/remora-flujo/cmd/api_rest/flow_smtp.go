package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// handleSMTPCheck calls hosting has-smtp via Channel to check whether the
// business/conversation has SMTP credentials in the vault.
//
//	GET /api/v1/businesses/{business_id}/smtp/check
func (s *server) handleSMTPCheck(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	bid := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, bid, nil); !ok {
		return
	}
	m := s.allManifests["hosting"]
	if m == nil {
		writeErr(w, http.StatusServiceUnavailable, "framework hosting no disponible")
		return
	}
	cmd, ok := m.Commands["has-smtp"]
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "hosting no soporta has-smtp")
		return
	}
	convID := businessVaultConvID(bid)
	params := map[string]string{"conv_id": convID}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error resolviendo args: "+err.Error())
		return
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := s.scoped(convID).ExecuteCommand(ctx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		writeOK(w, map[string]interface{}{
			"available":  false,
			"capability": "credentials.smtp",
			"error":      "error ejecutando has-smtp: " + err.Error(),
		})
		return
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &result); err != nil {
		result = map[string]interface{}{"available": false, "error": "parse error", "raw": resp.Stdout}
	}
	writeOK(w, result)
}

// smtpImportRequest is the payload for importing SMTP credentials.
// Passwords are never logged or persisted in plaintext outside the vault.
type smtpImportRequest struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	User      string `json:"user"`
	Pass      string `json:"pass"`
	From      string `json:"from"`
	DefaultTo string `json:"default_to"`
}

type hostingConnectRequest struct {
	Host string `json:"host"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

func (s *server) handleHostingConnect(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	bid := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, bid, nil); !ok {
		return
	}
	var req hostingConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.User) == "" || strings.TrimSpace(req.Pass) == "" {
		writeErr(w, http.StatusBadRequest, "host, user y pass son requeridos")
		return
	}
	m := s.allManifests["hosting"]
	if m == nil {
		writeErr(w, http.StatusServiceUnavailable, "framework hosting no disponible")
		return
	}
	cmd, ok := m.Commands["connect"]
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "hosting no soporta connect")
		return
	}
	convID := businessVaultConvID(bid)
	params := map[string]string{
		"host":    strings.TrimSpace(req.Host),
		"user":    strings.TrimSpace(req.User),
		"pass":    strings.TrimSpace(req.Pass),
		"conv_id": convID,
	}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error resolviendo args: "+err.Error())
		return
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	fmt.Printf("[hosting-connect] business=%s user=%s host=%s\n", bid, req.User, req.Host)
	resp, err := s.scoped(convID).ExecuteCommand(ctx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error ejecutando hosting connect: "+err.Error())
		return
	}
	if resp.ExitCode != 0 {
		writeErr(w, http.StatusBadRequest, strings.TrimSpace(resp.Stderr))
		return
	}
	writeOK(w, map[string]interface{}{
		"stdout":  strings.TrimSpace(resp.Stdout),
		"message": strings.TrimSpace(resp.Stdout),
	})
}

// handleSMTPImport calls hosting import-smtp to store SMTP credentials in
// the vault for the given business.
//
//	POST /api/v1/businesses/{business_id}/smtp/import
//
// The request body must contain host, user, pass at minimum. The password
// is forwarded to hosting import-smtp via Channel and stored encrypted in
// the vault. It is NEVER logged by the API.
func (s *server) handleSMTPImport(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	bid := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, bid, nil); !ok {
		return
	}
	var req smtpImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.User) == "" || strings.TrimSpace(req.Pass) == "" {
		writeErr(w, http.StatusBadRequest, "host, user y pass son requeridos")
		return
	}
	m := s.allManifests["hosting"]
	if m == nil {
		writeErr(w, http.StatusServiceUnavailable, "framework hosting no disponible")
		return
	}
	cmd, ok := m.Commands["import-smtp"]
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "hosting no soporta import-smtp")
		return
	}
	convID := businessVaultConvID(bid)
	port := strings.TrimSpace(req.Port)
	if port == "" {
		port = "587"
	}
	fromAddr := strings.TrimSpace(req.From)
	if fromAddr == "" {
		fromAddr = strings.TrimSpace(req.User)
	}
	params := map[string]string{
		"host":       strings.TrimSpace(req.Host),
		"port":       port,
		"user":       strings.TrimSpace(req.User),
		"pass":       strings.TrimSpace(req.Pass),
		"from":       fromAddr,
		"default_to": strings.TrimSpace(req.DefaultTo),
		"conv_id":    convID,
	}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error resolviendo args: "+err.Error())
		return
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	// Log import attempt without credentials
	fmt.Printf("[smtp-import] business=%s user=%s host=%s port=%s\n", bid, req.User, req.Host, port)
	resp, err := s.scoped(convID).ExecuteCommand(ctx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error ejecutando import-smtp: "+err.Error())
		return
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &result); err != nil {
		result = map[string]interface{}{"success": false, "error": "parse error", "raw": resp.Stdout}
	}
	if resp.ExitCode != 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Error: fmt.Sprintf("import-smtp failed (exit %d)", resp.ExitCode), Data: result})
		return
	}
	writeOK(w, result)
}

// handleSMTPDelete elimina credentials.smtp del vault del negocio.
//
//	DELETE /api/v1/businesses/{business_id}/smtp
func (s *server) handleSMTPDelete(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	bid := muxVar(r, "business_id")
	if _, _, ok := s.requireMembershipContext(w, r, bid, nil); !ok {
		return
	}
	m := s.allManifests["hosting"]
	if m == nil {
		writeErr(w, http.StatusServiceUnavailable, "framework hosting no disponible")
		return
	}
	cmd, ok := m.Commands["delete-smtp"]
	if !ok {
		writeErr(w, http.StatusServiceUnavailable, "hosting no soporta delete-smtp")
		return
	}
	convID := businessVaultConvID(bid)
	params := map[string]string{"conv_id": convID}
	args, err := cmd.ResolveArgs(params, nil, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error resolviendo args: "+err.Error())
		return
	}
	runtime := resolveManifestRuntime(s.rootDir, m)
	fullArgs := runtime.FullArgs(args, m)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := s.scoped(convID).ExecuteCommand(ctx, runtime.Command, fullArgs, runtime.Cwd)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error ejecutando delete-smtp: "+err.Error())
		return
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &result); err != nil {
		result = map[string]interface{}{"deleted": false, "error": "parse error", "raw": resp.Stdout}
	}
	writeOK(w, result)
}

func businessVaultConvID(businessID string) string {
	id := strings.TrimSpace(businessID)
	if strings.HasPrefix(id, "biz_") {
		return id
	}
	return "biz_" + id
}

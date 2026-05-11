package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type authRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type authLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createBusinessRequest struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	Country string `json:"country"`
}

type createInviteRequest struct {
	Role     string         `json:"role"`
	Audience string         `json:"audience"`
	Scope    map[string]any `json:"scope"`
}

type acceptInviteRequest struct {
	Token string `json:"token"`
	Code  string `json:"code"`
}

type createRemoraInviteRequest struct {
	Role string `json:"role"`
}

func (s *server) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	var req authRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	user, err := s.auth.createUser(req.Email, req.Password, req.Name, "user")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	token, sess, err := s.auth.createSession(user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	setAuthCookie(w, token, sess.ExpiresAt)
	memberships, _ := s.auth.memberships(user.ID)
	writeOK(w, map[string]any{
		"user":        user,
		"session":     sess,
		"token":       token,
		"memberships": memberships,
	})
}

func (s *server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req authLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	user, err := s.auth.authenticate(req.Email, req.Password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	token, sess, err := s.auth.createSession(user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	setAuthCookie(w, token, sess.ExpiresAt)
	memberships, _ := s.auth.memberships(user.ID)
	writeOK(w, map[string]any{
		"user":        user,
		"session":     sess,
		"token":       token,
		"memberships": memberships,
	})
}

func (s *server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := authTokenFromRequest(r)
	if token != "" {
		_ = s.auth.deleteSessionToken(token)
	}
	clearAuthCookie(w)
	writeOK(w, map[string]any{"logged_out": true})
}

func (s *server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user, sess, err := s.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	memberships, _ := s.auth.memberships(user.ID)
	writeOK(w, map[string]any{
		"user":        user,
		"session":     sess,
		"memberships": memberships,
	})
}

func (s *server) handleBusinesses(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	memberships, err := s.auth.memberships(user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, memberships)
}

func (s *server) handleBusinessCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req createBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	business, membership, err := s.auth.createBusiness(user.ID, req.Name, req.Domain, req.Country)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	memberships, _ := s.auth.memberships(user.ID)
	writeOK(w, map[string]any{
		"business":    business,
		"membership":  membership,
		"memberships": memberships,
	})
}

func (s *server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	users, err := s.auth.listUsersOverview()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, users)
}

func (s *server) handleAdminTeam(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRemoraStaff(w, r); !ok {
		return
	}
	team, err := s.auth.listRemoraTeam()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, team)
}

func (s *server) handleAdminRemoraInviteCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireRemoraStaff(w, r)
	if !ok {
		return
	}
	var req createRemoraInviteRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Role == "remora_admin" && !isRemoraAdmin(user) {
		writeErr(w, http.StatusForbidden, "requiere admin de Remora")
		return
	}
	inv, code, err := s.auth.createRemoraTeamInvite(user.ID, req.Role)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	link := appOrigin(r) + "/?remora_invite=" + inv.Token
	writeOK(w, map[string]any{"invite": inv, "code": code, "link": link})
}

func (s *server) handleRemoraInviteLookup(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	inv, err := s.auth.remoraTeamInviteByToken(token)
	if err != nil {
		writeErr(w, http.StatusNotFound, "invitación Remora inválida")
		return
	}
	writeOK(w, map[string]any{"invite": inv})
}

func (s *server) handleRemoraInviteAccept(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	updated, err := s.auth.acceptRemoraTeamInvite(user.ID, req.Token, req.Code)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	memberships, _ := s.auth.memberships(user.ID)
	writeOK(w, map[string]any{"user": updated, "memberships": memberships})
}

func (s *server) handleBusinessMembers(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	if _, _, ok := s.requireMembershipContext(w, r, businessID, nil); !ok {
		return
	}
	members, err := s.auth.businessMembers(businessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, members)
}

func (s *server) handleBusinessInviteCreate(w http.ResponseWriter, r *http.Request) {
	businessID := mux.Vars(r)["business_id"]
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	membership, err := s.auth.membership(user.ID, businessID)
	if err != nil {
		writeErr(w, http.StatusForbidden, "usuario sin acceso al negocio: "+businessID)
		return
	}
	if !canManageBusiness(membership.Role) {
		writeErr(w, http.StatusForbidden, "rol sin permiso para invitar")
		return
	}
	var req createInviteRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	inv, code, err := s.auth.createInvite(businessID, user.ID, req.Role, req.Audience, req.Scope)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	link := appOrigin(r) + "/?invite=" + inv.Token
	writeOK(w, map[string]any{"invite": inv, "code": code, "link": link})
}

func (s *server) handleInviteLookup(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	inv, err := s.auth.inviteByToken(token)
	if err != nil {
		writeErr(w, http.StatusNotFound, "invitación inválida")
		return
	}
	writeOK(w, map[string]any{"invite": inv})
}

func (s *server) handleInviteAccept(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body inválido")
		return
	}
	membership, err := s.auth.acceptInvite(user.ID, req.Token, req.Code)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	memberships, _ := s.auth.memberships(user.ID)
	writeOK(w, map[string]any{"membership": membership, "memberships": memberships})
}

func appOrigin(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func isRemoraStaff(user *authUser) bool {
	if user == nil {
		return false
	}
	switch user.Role {
	case "remora_staff", "remora_admin", "developer":
		return true
	default:
		return false
	}
}

func isRemoraAdmin(user *authUser) bool {
	if user == nil {
		return false
	}
	switch user.Role {
	case "remora_admin", "developer":
		return true
	default:
		return false
	}
}

func (s *server) requireRemoraStaff(w http.ResponseWriter, r *http.Request) (*authUser, bool) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return nil, false
	}
	if !isRemoraStaff(user) {
		writeErr(w, http.StatusForbidden, "requiere staff de Remora")
		return nil, false
	}
	return user, true
}

func (s *server) requireCurrentUser(w http.ResponseWriter, r *http.Request) (*authUser, *authSession, bool) {
	user, sess, err := s.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return nil, nil, false
	}
	return user, sess, true
}

func (s *server) requireMembershipContext(w http.ResponseWriter, r *http.Request, businessID string, provided map[string]any) (string, map[string]any, bool) {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return "", nil, false
	}
	if businessID == "" {
		memberships, err := s.auth.memberships(user.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return "", nil, false
		}
		if len(memberships) == 0 {
			writeErr(w, http.StatusForbidden, "usuario sin negocio asociado")
			return "", nil, false
		}
		if len(memberships) > 1 {
			writeErr(w, http.StatusBadRequest, "business_id requerido: usuario tiene múltiples negocios")
			return "", nil, false
		}
		businessID = memberships[0].BusinessID
	}
	membership, err := s.auth.membership(user.ID, businessID)
	if err != nil {
		writeErr(w, http.StatusForbidden, "usuario sin acceso al negocio: "+businessID)
		return "", nil, false
	}
	return businessID, contextFromMembership(user, membership, provided), true
}

func (s *server) requireConversationAccess(w http.ResponseWriter, r *http.Request, conv *Conversation) bool {
	user, _, ok := s.requireCurrentUser(w, r)
	if !ok {
		return false
	}
	if conv == nil || conv.BusinessID == "" || conv.UserID == "" {
		writeErr(w, http.StatusForbidden, "conversación sin usuario o negocio asociado")
		return false
	}
	if conv.UserID != user.ID {
		writeErr(w, http.StatusForbidden, "conversación pertenece a otro usuario")
		return false
	}
	if _, err := s.auth.membership(user.ID, conv.BusinessID); err != nil {
		writeErr(w, http.StatusForbidden, "usuario sin acceso al negocio: "+conv.BusinessID)
		return false
	}
	return true
}

func (s *server) currentUser(r *http.Request) (*authUser, *authSession, error) {
	return s.auth.userByToken(authTokenFromRequest(r))
}

func authRequired() bool {
	return envOr("REMORA_AUTH_REQUIRED", "false") == "true"
}

func contextFromMembership(user *authUser, membership authMembership, provided map[string]any) map[string]any {
	ctx := map[string]any{}
	for k, v := range provided {
		ctx[k] = v
	}
	ctx["business_id"] = membership.BusinessID
	ctx["remora_user_id"] = user.ID
	ctx["audience"] = membership.Audience
	if _, ok := ctx["scope"]; !ok {
		ctx["scope"] = membership.Scope
	}
	return ctx
}

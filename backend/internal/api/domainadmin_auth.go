package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// registerDomainAdminAuthRoutes wires Mailfold's own login for domain
// admins — a real authentication tier distinct from both the singleton
// super-admin and webmail mailbox users, so a domain admin can sign into
// Mailfold itself (not just mailcow's SOGo/admin UI) and configure things
// scoped to the domain(s) they manage, such as SSO providers (see
// domainadmin_sso.go).
func (s *Server) registerDomainAdminAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/domain-admin/login", s.handleDomainAdminLogin)
	mux.HandleFunc("POST /api/auth/domain-admin/logout", s.requireDomainAdmin(s.handleDomainAdminLogout))
	mux.HandleFunc("GET /api/auth/domain-admin/me", s.requireDomainAdmin(s.handleDomainAdminMe))
}

type domainAdminCtxKey struct{}

// requireDomainAdmin authenticates a request from its bearer token against
// the domain-admin session store and attaches the identity to the request
// context, mirroring requireAuth/requireWebmail for the other two session
// kinds.
func (s *Server) requireDomainAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.domainAdminSessions.Get(bearerToken(r))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if actor, _ := r.Context().Value(auditActorCtxKey).(*auditActor); actor != nil {
			actor.actorType, actor.actor = "domain_admin", id.Username
		}
		next(w, r.WithContext(context.WithValue(r.Context(), domainAdminCtxKey{}, id)))
	}
}

// domainAdminIdentity reads the authenticated domain admin's identity
// (username and the domains mailcow reported them scoped to at login time)
// from the request context, populated by requireDomainAdmin.
func domainAdminIdentity(r *http.Request) *domainadmin.Identity {
	id, _ := r.Context().Value(domainAdminCtxKey{}).(*domainadmin.Identity)
	return id
}

type domainAdminLoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// handleDomainAdminLogin checks the given credentials against Mailfold's own
// domain-admin login store (captured whenever a super-admin sets/changes a
// domain admin's password — see domainadmins.go), then re-confirms the
// account is still active and fetches its CURRENT domain scope directly from
// mailcow (never cached), so a domain reassignment or deactivation made in
// mailcow takes effect on the domain admin's very next sign-in rather than
// being stale from whenever their password was last set.
func (s *Server) handleDomainAdminLogin(w http.ResponseWriter, r *http.Request) {
	if s.domainAdminStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "domain admin login is not configured"})
		return
	}
	if allowed, retry := s.loginLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, slow down"})
		return
	}
	var req domainAdminLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	hash, ok, err := s.domainAdminStore.GetLoginPassword(req.User)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		s.recordAudit("domain_admin", req.User, "login_failed", http.StatusUnauthorized, clientIP(r))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	domains, active, err := s.currentDomainAdminScope(r.Context(), req.User)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if !active {
		s.recordAudit("domain_admin", req.User, "login_failed", http.StatusUnauthorized, clientIP(r))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, exp, err := s.domainAdminSessions.Create(req.User, domains)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordAudit("domain_admin", req.User, "login", http.StatusOK, clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"user":       req.User,
		"domains":    domains,
		"expires_at": exp,
	})
}

// currentDomainAdminScope fetches username's domains and active flag straight
// from mailcow's domain-admin list — the source of truth for scope, never
// Mailfold's own login store (which only ever holds a password hash).
func (s *Server) currentDomainAdminScope(ctx context.Context, username string) (domains []string, active bool, err error) {
	raw, err := s.mc.DomainAdmins(ctx)
	if err != nil {
		return nil, false, err
	}
	// mailcow returns {} rather than [] when there are no domain admins at all.
	if trimmed := strings.TrimSpace(string(raw)); trimmed == "" || trimmed == "{}" {
		return nil, false, nil
	}
	var list []struct {
		Username        string            `json:"username"`
		Active          mailcow.FlexInt64 `json:"active"`
		SelectedDomains []string          `json:"selected_domains"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, false, err
	}
	for _, da := range list {
		if !strings.EqualFold(da.Username, username) {
			continue
		}
		return da.SelectedDomains, da.Active != 0, nil
	}
	return nil, false, nil
}

func (s *Server) handleDomainAdminLogout(w http.ResponseWriter, r *http.Request) {
	if id := domainAdminIdentity(r); id != nil {
		s.recordAudit("domain_admin", id.Username, "logout", http.StatusOK, clientIP(r))
	}
	s.domainAdminSessions.Delete(bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDomainAdminMe(w http.ResponseWriter, r *http.Request) {
	id := domainAdminIdentity(r)
	writeJSON(w, http.StatusOK, map[string]any{"user": id.Username, "domains": id.Domains})
}

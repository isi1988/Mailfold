package api

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// registerDomainAdminRoutes attaches the "/api/domain-admins" CRUD endpoints
// to mux (all require super-admin auth via requireAuth).
//
// A domain administrator is a scoped mailcow account permitted to manage a
// subset of domains without full super-admin rights. mailcow itself only ever
// lets them log into its own SOGo/admin UI, never Mailfold — so whenever a
// password is set or changed here, it is also bcrypt-hashed and stored in
// Mailfold's own domain-admin store (see internal/domainadmin), which is what
// domain_admin_login.go's login endpoint checks against. That capture is the
// one thing the generic registerCRUD helper can't do, so this resource is
// wired with its own handlers instead of that helper (unlike most other
// "advanced" resources).
func (s *Server) registerDomainAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/domain-admins", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveRaw(w, r, s.mc.DomainAdmins)
	}))
	mux.HandleFunc("POST /api/domain-admins", s.requireAuth(s.handleCreateDomainAdmin))
	mux.HandleFunc("PUT /api/domain-admins", s.requireAuth(s.handleEditDomainAdmin))
	mux.HandleFunc("DELETE /api/domain-admins", s.requireAuth(s.handleDeleteDomainAdmin))
}

// captureDomainAdminPassword bcrypt-hashes and stores password for username in
// the local domain-admin store, when both a store is configured and a
// non-empty password was actually provided (an edit that only changes,
// say, the domain list — "leave blank to keep the current password" — leaves
// the stored hash untouched). Any failure here is logged but does not fail
// the request: the mailcow-side change (the operation the caller actually
// asked for) already succeeded, and Mailfold's own login for this domain
// admin is a bonus capability, not the primary one.
func (s *Server) captureDomainAdminPassword(username, password string) {
	if s.domainAdminStore == nil || password == "" {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash domain admin password; Mailfold login not updated", "error", err, "username", username)
		return
	}
	if err := s.domainAdminStore.SetLoginPassword(username, string(hash), time.Now()); err != nil {
		s.logger.Error("failed to store domain admin login; Mailfold login not updated", "error", err, "username", username)
	}
}

func (s *Server) handleCreateDomainAdmin(w http.ResponseWriter, r *http.Request) {
	var attr map[string]any
	if err := decodeJSON(r, &attr); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	results, err := s.mc.AddDomainAdmin(r.Context(), attr)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if ok, _ := mailcow.ResultsOK(results); ok {
		username, _ := attr["username"].(string)
		password, _ := attr["password"].(string)
		s.captureDomainAdminPassword(username, password)
	}
	s.writeMailcowResults(w, results)
}

func (s *Server) handleEditDomainAdmin(w http.ResponseWriter, r *http.Request) {
	var req editRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	results, err := s.mc.EditDomainAdmin(r.Context(), req.Items, req.Attr)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if ok, _ := mailcow.ResultsOK(results); ok && len(req.Items) > 0 {
		password, _ := req.Attr["password"].(string)
		s.captureDomainAdminPassword(req.Items[0], password)
	}
	s.writeMailcowResults(w, results)
}

func (s *Server) handleDeleteDomainAdmin(w http.ResponseWriter, r *http.Request) {
	var req itemsRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	results, err := s.mc.DeleteDomainAdmin(r.Context(), req.Items)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	if ok, _ := mailcow.ResultsOK(results); ok && s.domainAdminStore != nil {
		for _, username := range req.Items {
			if err := s.domainAdminStore.DeleteLogin(username); err != nil {
				s.logger.Error("failed to delete domain admin login", "error", err, "username", username)
			}
		}
	}
	s.writeMailcowResults(w, results)
}

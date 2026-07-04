package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
)

// registerSSOProviderRoutes wires the super-admin's SSO provider management
// (requireAuth): the full list across every domain, and creating/editing/
// deleting a provider that can be shared across all domains or scoped to a
// specific set. A domain admin's narrower view of the same table — the
// providers available to their own domain(s), plus managing their own custom
// ones — is registered separately in domainadmin_sso.go.
func (s *Server) registerSSOProviderRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sso-providers", s.requireAuth(s.handleListSSOProviders))
	mux.HandleFunc("POST /api/sso-providers", s.requireAuth(s.handleCreateSSOProvider))
	mux.HandleFunc("PUT /api/sso-providers", s.requireAuth(s.handleEditSSOProvider))
	mux.HandleFunc("DELETE /api/sso-providers", s.requireAuth(s.handleDeleteSSOProvider))
}

// requireSSOStore reports 501 when SSO provider management is unavailable (no
// database, or no MAILFOLD_ADMIN_ENC_KEY to encrypt client secrets with).
func (s *Server) requireSSOStore(w http.ResponseWriter) bool {
	if s.domainAdminStore == nil || s.adminCipher == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and MAILFOLD_ADMIN_ENC_KEY to enable SSO"})
		return false
	}
	return true
}

// ssoProviderView is the wire shape for a provider — the client secret is
// never returned, only whether one is configured, matching how every other
// stored secret in this codebase (app-passwords, notify-sender) is exposed.
type ssoProviderView struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Issuer      string   `json:"issuer"`
	ClientID    string   `json:"client_id"`
	Configured  bool     `json:"configured"`
	RedirectURL string   `json:"redirect_url"`
	AllDomains  bool     `json:"all_domains"`
	Domains     []string `json:"domains"`
	CreatedBy   string   `json:"created_by"`
	Active      bool     `json:"active"`
}

func toSSOProviderView(p domainadmin.Provider) ssoProviderView {
	return ssoProviderView{
		ID: p.ID, Name: p.Name, Issuer: p.Issuer, ClientID: p.ClientID,
		Configured: len(p.ClientSecretEnc) > 0, RedirectURL: p.RedirectURL,
		AllDomains: p.AllDomains, Domains: p.Domains, CreatedBy: p.CreatedBy, Active: p.Active,
	}
}

func (s *Server) handleListSSOProviders(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	providers, err := s.domainAdminStore.ListProviders()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]ssoProviderView, len(providers))
	for i, p := range providers {
		out[i] = toSSOProviderView(p)
	}
	writeJSON(w, http.StatusOK, out)
}

// ssoProviderRequest is the wire shape for create/edit. ClientSecret is
// optional on edit — leaving it blank keeps the currently stored secret,
// matching the "leave blank to keep current password" convention used
// elsewhere in this codebase.
type ssoProviderRequest struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	AllDomains   bool     `json:"all_domains"`
	Domains      []string `json:"domains"`
	Active       bool     `json:"active"`
}

func (s *Server) handleCreateSSOProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	var req ssoProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" || req.Issuer == "" || req.ClientID == "" || req.ClientSecret == "" || req.RedirectURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, issuer, client_id, client_secret, and redirect_url are required"})
		return
	}
	enc, nonce, err := s.adminCipher.Seal([]byte(req.ClientSecret))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	id, err := s.domainAdminStore.CreateProvider(domainadmin.Provider{
		Name: req.Name, Issuer: req.Issuer, ClientID: req.ClientID,
		ClientSecretEnc: enc, ClientSecretNonce: nonce, RedirectURL: req.RedirectURL,
		AllDomains: req.AllDomains, Domains: req.Domains, Active: true,
	}, time.Now())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	p, _, err := s.domainAdminStore.GetProvider(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toSSOProviderView(p))
}

func (s *Server) handleEditSSOProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	var req ssoProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	existing, ok, err := s.domainAdminStore.GetProvider(req.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}
	enc, nonce := existing.ClientSecretEnc, existing.ClientSecretNonce
	if req.ClientSecret != "" {
		enc, nonce, err = s.adminCipher.Seal([]byte(req.ClientSecret))
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	updated := domainadmin.Provider{
		ID: existing.ID, Name: req.Name, Issuer: req.Issuer, ClientID: req.ClientID,
		ClientSecretEnc: enc, ClientSecretNonce: nonce, RedirectURL: req.RedirectURL,
		AllDomains: req.AllDomains, Domains: req.Domains, CreatedBy: existing.CreatedBy, Active: req.Active,
	}
	if err := s.domainAdminStore.UpdateProvider(updated, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	// Discard any cached OIDC discovery/runtime for this provider so the next
	// login attempt picks up the new configuration rather than a stale one.
	if s.sso != nil {
		s.sso.invalidate(existing.ID)
	}
	p, _, err := s.domainAdminStore.GetProvider(existing.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toSSOProviderView(p))
}

func (s *Server) handleDeleteSSOProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.domainAdminStore.DeleteProvider(id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if s.sso != nil {
		s.sso.invalidate(id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

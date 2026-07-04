package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/domainadmin"
)

// registerDomainAdminSSORoutes wires a domain admin's own view of SSO
// providers (requireDomainAdmin): the shared providers a super-admin made
// available to their domain(s) (read-only to them) plus any custom provider
// they added themselves, scoped to only the domains they manage. This is
// deliberately a narrower surface than registerSSOProviderRoutes — a domain
// admin can never see, edit, or delete a provider they didn't create, and can
// never scope a provider they create to a domain outside what mailcow itself
// says they manage.
func (s *Server) registerDomainAdminSSORoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/domain-admin/sso-providers", s.requireDomainAdmin(s.handleListDomainAdminSSOProviders))
	mux.HandleFunc("POST /api/domain-admin/sso-providers", s.requireDomainAdmin(s.handleCreateDomainAdminSSOProvider))
	mux.HandleFunc("PUT /api/domain-admin/sso-providers", s.requireDomainAdmin(s.handleEditDomainAdminSSOProvider))
	mux.HandleFunc("DELETE /api/domain-admin/sso-providers", s.requireDomainAdmin(s.handleDeleteDomainAdminSSOProvider))
}

// providersForDomains returns the union of every provider available to any of
// domains, de-duplicated by id.
func (s *Server) providersForDomains(domains []string) ([]domainadmin.Provider, error) {
	seen := make(map[int64]domainadmin.Provider)
	for _, d := range domains {
		list, err := s.domainAdminStore.ProvidersForDomain(d)
		if err != nil {
			return nil, err
		}
		for _, p := range list {
			seen[p.ID] = p
		}
	}
	out := make([]domainadmin.Provider, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	return out, nil
}

// domainAdminProviderView adds "editable" (true only for a provider this
// domain admin created themselves) to the super-admin's provider view.
type domainAdminProviderView struct {
	ssoProviderView
	Editable bool `json:"editable"`
}

func (s *Server) handleListDomainAdminSSOProviders(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	id := domainAdminIdentity(r)
	providers, err := s.providersForDomains(id.Domains)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]domainAdminProviderView, len(providers))
	for i, p := range providers {
		out[i] = domainAdminProviderView{ssoProviderView: toSSOProviderView(p), Editable: p.CreatedBy == id.Username}
	}
	writeJSON(w, http.StatusOK, out)
}

// domainSubset reports whether every entry of want is present in have,
// case-insensitively — used to keep a domain admin from scoping their own
// provider to a domain mailcow doesn't say they manage.
func domainSubset(want, have []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.EqualFold(w, h) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *Server) handleCreateDomainAdminSSOProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	id := domainAdminIdentity(r)
	var req ssoProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" || req.Issuer == "" || req.ClientID == "" || req.ClientSecret == "" || req.RedirectURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, issuer, client_id, client_secret, and redirect_url are required"})
		return
	}
	domains := req.Domains
	if len(domains) == 0 {
		// Default to every domain this admin manages when none was picked.
		domains = id.Domains
	}
	if !domainSubset(domains, id.Domains) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "you can only scope a provider to domains you manage"})
		return
	}
	enc, nonce, err := s.adminCipher.Seal([]byte(req.ClientSecret))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	newID, err := s.domainAdminStore.CreateProvider(domainadmin.Provider{
		Name: req.Name, Issuer: req.Issuer, ClientID: req.ClientID,
		ClientSecretEnc: enc, ClientSecretNonce: nonce, RedirectURL: req.RedirectURL,
		AllDomains: false, Domains: domains, CreatedBy: id.Username, Active: true,
	}, time.Now())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	p, _, err := s.domainAdminStore.GetProvider(newID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, domainAdminProviderView{ssoProviderView: toSSOProviderView(p), Editable: true})
}

// requireOwnedSSOProvider fetches provider id and verifies the caller created
// it, so a domain admin can only ever edit or delete their own custom
// providers — never a super-admin's shared one, and never another domain
// admin's.
func (s *Server) requireOwnedSSOProvider(w http.ResponseWriter, id int64, username string) (domainadmin.Provider, bool) {
	p, ok, err := s.domainAdminStore.GetProvider(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return domainadmin.Provider{}, false
	}
	if !ok || p.CreatedBy != username {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return domainadmin.Provider{}, false
	}
	return p, true
}

func (s *Server) handleEditDomainAdminSSOProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	id := domainAdminIdentity(r)
	var req ssoProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	existing, ok := s.requireOwnedSSOProvider(w, req.ID, id.Username)
	if !ok {
		return
	}
	domains := req.Domains
	if len(domains) == 0 {
		domains = id.Domains
	}
	if !domainSubset(domains, id.Domains) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "you can only scope a provider to domains you manage"})
		return
	}
	enc, nonce := existing.ClientSecretEnc, existing.ClientSecretNonce
	if req.ClientSecret != "" {
		var err error
		enc, nonce, err = s.adminCipher.Seal([]byte(req.ClientSecret))
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	updated := domainadmin.Provider{
		ID: existing.ID, Name: req.Name, Issuer: req.Issuer, ClientID: req.ClientID,
		ClientSecretEnc: enc, ClientSecretNonce: nonce, RedirectURL: req.RedirectURL,
		AllDomains: false, Domains: domains, CreatedBy: id.Username, Active: req.Active,
	}
	if err := s.domainAdminStore.UpdateProvider(updated, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if s.sso != nil {
		s.sso.invalidate(existing.ID)
	}
	p, _, err := s.domainAdminStore.GetProvider(existing.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, domainAdminProviderView{ssoProviderView: toSSOProviderView(p), Editable: true})
}

func (s *Server) handleDeleteDomainAdminSSOProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireSSOStore(w) {
		return
	}
	id := domainAdminIdentity(r)
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	pid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if _, ok := s.requireOwnedSSOProvider(w, pid, id.Username); !ok {
		return
	}
	if err := s.domainAdminStore.DeleteProvider(pid); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if s.sso != nil {
		s.sso.invalidate(pid)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

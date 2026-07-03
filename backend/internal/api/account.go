package api

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/isi1988/Mailfold/backend/internal/admin"
)

// registerAccountRoutes wires the admin-account settings endpoints: profile,
// password change, and session/device management. All of them require the
// admin store (a configured MAILFOLD_DB_PATH); when it is absent they report
// 501 so the frontend can show "not available" instead of a confusing 500.
func (s *Server) registerAccountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/account/profile", s.requireAuth(s.handleProfileGet))
	mux.HandleFunc("PUT /api/account/profile", s.requireAuth(s.handleProfilePut))
	mux.HandleFunc("POST /api/account/password", s.requireAuth(s.handlePasswordChange))
	mux.HandleFunc("GET /api/account/sessions", s.requireAuth(s.handleSessionsList))
	mux.HandleFunc("POST /api/account/sessions/{id}/revoke", s.requireAuth(s.handleSessionRevoke))
	mux.HandleFunc("POST /api/account/sessions/revoke-all", s.requireAuth(s.handleSessionsRevokeAll))
}

// requireAdminStore reports 501 and returns false when the admin store is not
// configured, so every handler in this file can open with one guard line.
func (s *Server) requireAdminStore(w http.ResponseWriter) bool {
	if s.adminStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH to enable account settings"})
		return false
	}
	return true
}

// profileResponse is the account-settings profile shape. It deliberately omits
// every secret field on the stored Account (password hash, TOTP/notify
// ciphertext) — those are managed by their own dedicated endpoints.
type profileResponse struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Timezone    string `json:"timezone"`
	AvatarURL   string `json:"avatar_url"`
	TOTPEnabled bool   `json:"totp_enabled"`
}

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminStore(w) {
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, profileResponse{
		Username:    s.cfg.AdminUser,
		DisplayName: acct.DisplayName,
		Email:       acct.Email,
		Timezone:    acct.Timezone,
		AvatarURL:   acct.AvatarURL,
		TOTPEnabled: acct.TOTPEnabled,
	})
}

func (s *Server) handleProfilePut(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminStore(w) {
		return
	}
	var req admin.ProfileUpdate
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.adminStore.SetProfile(s.cfg.AdminUser, req, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// passwordChangeRequest requires the current password so a hijacked session
// token alone cannot lock the real admin out by silently swapping in a new one.
type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminStore(w) {
		return
	}
	var req passwordChangeRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the new password must be at least 8 characters"})
		return
	}
	if !s.auth.CheckPassword(s.cfg.AdminUser, req.CurrentPassword) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.adminStore.SetPasswordHash(s.cfg.AdminUser, string(hash), time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	// Take effect immediately, in this same process, without a restart.
	s.auth.SetPasswordHash(string(hash))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.auth.ListSessions(bearerToken(r)))
}

func (s *Server) handleSessionRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, si := range s.auth.ListSessions(bearerToken(r)) {
		if si.Current && si.ID == id {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sign out this device from Settings instead of revoking it here"})
			return
		}
	}
	if !s.auth.RevokeByID(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSessionsRevokeAll(w http.ResponseWriter, r *http.Request) {
	n := s.auth.RevokeAllExcept(bearerToken(r))
	writeJSON(w, http.StatusOK, map[string]int{"revoked": n})
}

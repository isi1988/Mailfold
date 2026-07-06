package api

import (
	"encoding/base64"
	"net/http"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/isi1988/Mailfold/backend/internal/admin"
)

// registerTOTPRoutes wires two-factor (TOTP) enrollment. All routes require the
// admin store AND the admin cipher (MAILFOLD_ADMIN_ENC_KEY); enroll reports 501
// when either is missing, so the frontend can hide the whole Security section
// rather than offer a control that would fail.
func (s *Server) registerTOTPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/account/2fa/enroll", s.requireAuth(s.handleTOTPEnroll))
	mux.HandleFunc("POST /api/account/2fa/confirm", s.requireAuth(s.handleTOTPConfirm))
	mux.HandleFunc("POST /api/account/2fa/disable", s.requireAuth(s.handleTOTPDisable))
	mux.HandleFunc("POST /api/account/2fa/recovery-codes", s.requireAuth(s.handleTOTPRecoveryRegenerate))
}

// requireAdminCipher reports 501 and returns false when two-factor auth is not
// available (no DB, or no MAILFOLD_ADMIN_ENC_KEY configured).
func (s *Server) requireAdminCipher(w http.ResponseWriter) bool {
	if s.adminStore == nil || s.adminCipher == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and MAILFOLD_ADMIN_ENC_KEY to enable two-factor auth"})
		return false
	}
	return true
}

// totpEnrollRequest carries the current password: starting (or restarting)
// enrollment is gated on it so a hijacked session token cannot silently take
// over the second factor.
type totpEnrollRequest struct {
	CurrentPassword string `json:"current_password"`
}

// handleTOTPEnroll generates a fresh secret, stores it encrypted (but not yet
// marked enabled — handleTOTPConfirm flips that once the admin proves they can
// generate a matching code), and returns everything needed to add it to an
// authenticator app: the otpauth:// URI, a scannable QR code as a PNG data URI,
// and the raw secret for manual entry.
func (s *Server) handleTOTPEnroll(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminCipher(w) {
		return
	}
	var req totpEnrollRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.auth.CheckPassword(s.cfg.AdminUser, req.CurrentPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password is incorrect"})
		return
	}

	secret, err := admin.NewTOTPSecret()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	enc, nonce, err := s.adminCipher.Seal([]byte(secret))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.adminStore.SetTOTP(s.cfg.AdminUser, false, enc, nonce, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	uri := admin.TOTPURI("Mailfold", s.cfg.AdminUser, secret)
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	var qrDataURI string
	if err == nil {
		qrDataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"secret":      secret,
		"otpauth_uri": uri,
		"qr_data_uri": qrDataURI,
	})
}

type totpConfirmRequest struct {
	Code string `json:"code"`
}

// handleTOTPConfirm verifies the first code against the secret stored by
// handleTOTPEnroll, and — only on success — marks two-factor enabled and mints
// the one-time recovery codes. If the admin never completes this step, the
// stored secret stays inert (TOTPEnabled remains false, so totpEnabled() and
// login never consult it).
func (s *Server) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminCipher(w) {
		return
	}
	var req totpConfirmRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(acct.TOTPSecretEnc) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start enrollment first"})
		return
	}
	secretBytes, err := s.adminCipher.Open(acct.TOTPSecretEnc, acct.TOTPSecretNonce)
	if err != nil || !admin.VerifyTOTP(string(secretBytes), req.Code) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid code"})
		return
	}

	if err := s.adminStore.SetTOTP(s.cfg.AdminUser, true, acct.TOTPSecretEnc, acct.TOTPSecretNonce, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	codes, err := s.mintRecoveryCodes()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "recovery_codes": codes})
}

type totpDisableRequest struct {
	CurrentPassword string `json:"current_password"`
}

// handleTOTPDisable turns two-factor off and wipes the stored secret, gated on
// the current password for the same reason enrollment is.
func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminStore(w) {
		return
	}
	var req totpDisableRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.auth.CheckPassword(s.cfg.AdminUser, req.CurrentPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password is incorrect"})
		return
	}
	if err := s.adminStore.SetTOTP(s.cfg.AdminUser, false, nil, nil, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.adminStore.ReplaceRecoveryCodes(s.cfg.AdminUser, nil); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleTOTPRecoveryRegenerate invalidates every existing recovery code and
// mints a fresh set, for when the admin has used most of them up or suspects
// the old ones leaked. Requires 2FA to already be enabled.
func (s *Server) handleTOTPRecoveryRegenerate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminStore(w) {
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !acct.TOTPEnabled {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "two-factor auth is not enabled"})
		return
	}
	codes, err := s.mintRecoveryCodes()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// mintRecoveryCodes generates, hashes, and persists a fresh set of recovery
// codes, returning the plaintext codes so the caller can show them exactly
// once — only the hashes are ever stored.
func (s *Server) mintRecoveryCodes() ([]string, error) {
	codes, err := admin.NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = admin.HashRecoveryCode(c)
	}
	if err := s.adminStore.ReplaceRecoveryCodes(s.cfg.AdminUser, hashes); err != nil {
		return nil, err
	}
	return codes, nil
}

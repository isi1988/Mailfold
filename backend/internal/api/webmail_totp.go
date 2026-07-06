package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/isi1988/Mailfold/backend/internal/admin"
)

// registerWebmailTOTPRoutes wires optional two-factor auth for a webmail
// mailbox user, mirroring registerTOTPRoutes (the single admin's 2FA) but
// keyed by the caller's own mailbox address instead of the configured admin
// username, and verified against the mailbox's real IMAP password instead of
// a Mailfold-managed one. verify is registered publicly (like
// /api/auth/2fa/verify) because at that point the caller only has the
// short-lived pending token from handleWebmailLogin, not a session yet.
func (s *Server) registerWebmailTOTPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webmail/2fa/status", s.requireWebmail(s.handleWebmailTOTPStatus))
	mux.HandleFunc("POST /api/webmail/2fa/enroll", s.requireWebmail(s.handleWebmailTOTPEnroll))
	mux.HandleFunc("POST /api/webmail/2fa/confirm", s.requireWebmail(s.handleWebmailTOTPConfirm))
	mux.HandleFunc("POST /api/webmail/2fa/disable", s.requireWebmail(s.handleWebmailTOTPDisable))
	mux.HandleFunc("POST /api/webmail/2fa/recovery-codes", s.requireWebmail(s.handleWebmailTOTPRecoveryRegenerate))
	mux.HandleFunc("POST /api/webmail/2fa/verify", s.handleWebmailTOTPVerify)
}

// requireWebmailCipher reports 501 when two-factor auth is not available for
// webmail (no DB, or no MAILFOLD_ADMIN_ENC_KEY — the same cipher the admin's
// own 2FA and notify-sender secrets use, reused here rather than requiring a
// second master key for what is still just AES-256-GCM at rest).
func (s *Server) requireWebmailCipher(w http.ResponseWriter) bool {
	if s.webmailUsers == nil || s.adminCipher == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and MAILFOLD_ADMIN_ENC_KEY to enable two-factor auth"})
		return false
	}
	return true
}

func (s *Server) handleWebmailTOTPStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailUserStore(w) {
		return
	}
	acct, err := s.webmailUsers.GetAccount(webmailCreds(r).Email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": acct.TOTPEnabled})
}

type webmailTOTPEnrollRequest struct {
	CurrentPassword string `json:"current_password"`
}

// handleWebmailTOTPEnroll generates a fresh secret and stores it (not yet
// enabled — handleWebmailTOTPConfirm flips that once the user proves they can
// generate a matching code). The current password is verified live against
// IMAP rather than a stored hash, since a webmail user's real password is
// their mailbox password.
func (s *Server) handleWebmailTOTPEnroll(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailCipher(w) {
		return
	}
	var req webmailTOTPEnrollRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	email := webmailCreds(r).Email
	if err := s.webmail.Verify(email, req.CurrentPassword); err != nil {
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
	if err := s.webmailUsers.SetTOTP(email, false, enc, nonce, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	uri := admin.TOTPURI("Mailfold", email, secret)
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

type webmailTOTPConfirmRequest struct {
	Code string `json:"code"`
}

// handleWebmailTOTPConfirm verifies the first code against the secret stored
// by handleWebmailTOTPEnroll, and only on success marks 2FA enabled and mints
// the one-time recovery codes.
func (s *Server) handleWebmailTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailCipher(w) {
		return
	}
	var req webmailTOTPConfirmRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	email := webmailCreds(r).Email
	acct, err := s.webmailUsers.GetAccount(email)
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

	if err := s.webmailUsers.SetTOTP(email, true, acct.TOTPSecretEnc, acct.TOTPSecretNonce, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	codes, err := s.mintWebmailRecoveryCodes(email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "recovery_codes": codes})
}

type webmailTOTPDisableRequest struct {
	CurrentPassword string `json:"current_password"`
}

func (s *Server) handleWebmailTOTPDisable(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailUserStore(w) {
		return
	}
	var req webmailTOTPDisableRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	email := webmailCreds(r).Email
	if err := s.webmail.Verify(email, req.CurrentPassword); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password is incorrect"})
		return
	}
	if err := s.webmailUsers.SetTOTP(email, false, nil, nil, time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.webmailUsers.ReplaceRecoveryCodes(email, nil); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWebmailTOTPRecoveryRegenerate(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebmailUserStore(w) {
		return
	}
	email := webmailCreds(r).Email
	acct, err := s.webmailUsers.GetAccount(email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !acct.TOTPEnabled {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "two-factor auth is not enabled"})
		return
	}
	codes, err := s.mintWebmailRecoveryCodes(email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (s *Server) mintWebmailRecoveryCodes(email string) ([]string, error) {
	codes, err := admin.NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = admin.HashRecoveryCode(c)
	}
	if err := s.webmailUsers.ReplaceRecoveryCodes(email, hashes); err != nil {
		return nil, err
	}
	return codes, nil
}

// webmailTOTPEnabled reports whether email currently has two-factor auth
// turned on. Like totpEnabled, it fails closed to "disabled" on a store
// error — a database hiccup should not lock every mailbox out of webmail.
func (s *Server) webmailTOTPEnabled(email string) bool {
	if s.webmailUsers == nil {
		return false
	}
	acct, err := s.webmailUsers.GetAccount(email)
	if err != nil {
		s.logger.Error("failed to read webmail user account for 2FA check", "error", err)
		return false
	}
	return acct.TOTPEnabled
}

// verifyWebmailTOTPOrRecovery checks code against email's live TOTP secret
// first, then — only if that fails — against their unused recovery codes.
func (s *Server) verifyWebmailTOTPOrRecovery(email, code string) bool {
	if s.webmailUsers == nil || s.adminCipher == nil {
		return false
	}
	acct, err := s.webmailUsers.GetAccount(email)
	if err != nil || !acct.TOTPEnabled || len(acct.TOTPSecretEnc) == 0 {
		return false
	}
	secretBytes, err := s.adminCipher.Open(acct.TOTPSecretEnc, acct.TOTPSecretNonce)
	if err == nil && admin.VerifyTOTP(string(secretBytes), code) {
		return true
	}
	ok, err := s.webmailUsers.ConsumeRecoveryCode(email, admin.HashRecoveryCode(code), time.Now())
	return err == nil && ok
}

type webmailTOTPVerifyRequest struct {
	PendingToken string `json:"pending_token"`
	Code         string `json:"code"`
}

// handleWebmailTOTPVerify redeems a pending webmail login (issued by
// handleWebmailLogin once the password check succeeded but a second factor
// was still required) together with a TOTP or recovery code, and mints the
// real webmail session on success. A wrong code does not invalidate the
// pending token — Peek only counts the attempt — so a typo can be retried a
// bounded number of times instead of permanently stranding the caller; the
// token is only consumed once a code actually verifies.
func (s *Server) handleWebmailTOTPVerify(w http.ResponseWriter, r *http.Request) {
	if allowed, retry := s.loginLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, slow down"})
		return
	}
	var req webmailTOTPVerifyRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	cred, ok := s.webmailPending.Peek(req.PendingToken)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": errSignInAgain})
		return
	}
	if !s.verifyWebmailTOTPOrRecovery(cred.Email, req.Code) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid code"})
		return
	}
	s.webmailPending.Delete(req.PendingToken)
	token, exp, err := s.webmailSessions.Create(cred.Email, cred.Password)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "email": cred.Email, "expires_at": exp})
}

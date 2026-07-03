package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// resetTokenTTL is how long a password-reset link stays valid after it is
// emailed.
const resetTokenTTL = time.Hour

// registerPasswordResetRoutes wires the public (unauthenticated) forgot/reset
// password flow. Both routes are rate-limited per client IP via s.resetLimiter,
// since an attacker with no credentials could otherwise hammer them.
func (s *Server) registerPasswordResetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/forgot-password", s.handleForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", s.handleResetPassword)
}

// handleForgotPassword always responds 200 regardless of whether an email was
// actually sent: the admin account, its profile email, and its notification
// sender may or may not be configured, and none of that should be observable to
// an unauthenticated caller. Every failure is logged server-side instead.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if allowed, retry := s.resetLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests, slow down"})
		return
	}
	s.tryStartPasswordReset(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// tryStartPasswordReset does the real work behind handleForgotPassword. It is
// split out so that handler can keep its single, unconditional 200 response
// while every early-exit here is a silent no-op from the caller's perspective.
func (s *Server) tryStartPasswordReset(r *http.Request) {
	if s.adminStore == nil || s.adminCipher == nil {
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.logger.Error("forgot-password: read account", "error", err)
		return
	}
	if acct.Email == "" || acct.NotifyMailbox == "" || len(acct.NotifyPasswordEnc) == 0 {
		return
	}
	token, err := randomResetToken()
	if err != nil {
		s.logger.Error("forgot-password: generate token", "error", err)
		return
	}
	if err := s.adminStore.CreateResetToken(hashResetToken(token), s.cfg.AdminUser, time.Now().Add(resetTokenTTL)); err != nil {
		s.logger.Error("forgot-password: create token", "error", err)
		return
	}
	pw, err := s.adminCipher.Open(acct.NotifyPasswordEnc, acct.NotifyPasswordNonce)
	if err != nil {
		s.logger.Error("forgot-password: decrypt sender password", "error", err)
		return
	}
	link := publicOrigin(r) + "/reset?token=" + token
	msg := &webmail.OutgoingMessage{
		To:      []string{acct.Email},
		Subject: "Reset your Mailfold password",
		Text:    "Follow this link to reset your Mailfold admin password:\n\n" + link + "\n\nThis link expires in one hour. If you did not request this, you can ignore this email.",
	}
	if err := s.webmail.Send(acct.NotifyMailbox, string(pw), msg); err != nil {
		s.logger.Error("forgot-password: send email", "error", err)
	}
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// handleResetPassword redeems a password-reset token minted by
// tryStartPasswordReset. On success it also revokes every existing session (the
// admin is presumably resetting because they lost access, so any live session
// should not be trusted to still belong to them).
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if allowed, retry := s.resetLimiter.Allow(clientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, slow down"})
		return
	}
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the new password must be at least 8 characters"})
		return
	}
	if s.adminStore == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired token"})
		return
	}
	username, ok, err := s.adminStore.ConsumeResetToken(hashResetToken(req.Token), time.Now())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired token"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.adminStore.SetPasswordHash(username, string(hash), time.Now()); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.auth.SetPasswordHash(string(hash))
	s.auth.RevokeAllExcept("")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// randomResetToken mints a 256-bit random token, hex-encoded for use in a URL.
func randomResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashResetToken returns the token's SHA-256 hex digest, the only form
// persisted — the raw token exists solely in the emailed link.
func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// publicOrigin resolves the browser-facing origin to build a reset link
// against. It prefers the request's Origin header (which browsers send on
// same-origin fetch/XHR POSTs too, not just cross-origin ones), falling back to
// the Host header with a best-effort scheme when Origin is absent (e.g. a
// same-origin form submission from an older client).
func publicOrigin(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" {
		return o
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

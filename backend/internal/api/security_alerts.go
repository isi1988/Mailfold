package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
)

// loginFailureAlertThreshold is how many consecutive failed admin logins in a
// row trigger exactly one alert email — high enough that an honest typo never
// fires it, low enough to catch a real brute-force attempt quickly, and low
// enough to still fire well before the login rate limiter itself would start
// blocking the attempts outright.
const loginFailureAlertThreshold = 3

// loginFailureTracker counts consecutive failed login attempts per username,
// in memory, so an alert fires once per burst rather than once per guess
// (which would just spam the inbox during a brute-force attempt) or never.
// Like the session stores, it is process-local and forgets in-flight bursts
// across a restart — that's an acceptable trade for not needing a database
// table for what is genuinely transient state.
type loginFailureTracker struct {
	mu      sync.Mutex
	streaks map[string]int
}

func newLoginFailureTracker() *loginFailureTracker {
	return &loginFailureTracker{streaks: make(map[string]int)}
}

// Fail records one failed attempt for username and reports whether this
// attempt is exactly the one that crossed loginFailureAlertThreshold.
func (t *loginFailureTracker) Fail(username string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.streaks[username]++
	return t.streaks[username] == loginFailureAlertThreshold
}

// Reset clears username's streak; called on a successful login so the count
// for the next burst starts fresh.
func (t *loginFailureTracker) Reset(username string) {
	t.mu.Lock()
	delete(t.streaks, username)
	t.mu.Unlock()
}

// deviceFingerprint derives a stable, non-reversible identifier for the
// client that made r, from its IP and User-Agent. It is intentionally coarse
// (no cookies, no client-side state needed): the same browser on the same
// network reliably repeats it, which is all "have we seen this device
// before?" needs.
func deviceFingerprint(r *http.Request) string {
	sum := sha256.Sum256([]byte(clientIP(r) + "|" + r.UserAgent()))
	return hex.EncodeToString(sum[:])
}

// sendSecurityAlert emails the admin's profile address, via the configured
// notification sender, about a security-relevant event on their account. It
// is deliberately best-effort — matching tryStartPasswordReset — since a
// failure to send a heads-up email must never affect the login itself, and a
// caller with no notify-sender or profile email configured yet simply gets no
// alert rather than an error.
func (s *Server) sendSecurityAlert(subject, body string) {
	if s.adminStore == nil || s.adminCipher == nil {
		return
	}
	acct, err := s.adminStore.GetAccount(s.cfg.AdminUser)
	if err != nil {
		s.logger.Error("security alert: read account", "error", err)
		return
	}
	if acct.Email == "" || acct.NotifyMailbox == "" || len(acct.NotifyPasswordEnc) == 0 {
		return
	}
	pw, err := s.adminCipher.Open(acct.NotifyPasswordEnc, acct.NotifyPasswordNonce)
	if err != nil {
		s.logger.Error("security alert: decrypt sender password", "error", err)
		return
	}
	msg := &webmail.OutgoingMessage{To: []string{acct.Email}, Subject: subject, Text: body}
	if err := s.webmail.Send(acct.NotifyMailbox, string(pw), msg); err != nil {
		s.logger.Error("security alert: send email", "error", err)
	}
}

// alertOnFailedLogin bumps username's failure streak and, the moment it
// crosses loginFailureAlertThreshold, emails a single alert for the burst.
func (s *Server) alertOnFailedLogin(username string, r *http.Request) {
	if s.loginFailures.Fail(username) {
		s.sendSecurityAlert(
			"Multiple failed sign-in attempts on your Mailfold account",
			fmt.Sprintf(
				"There have been %d failed sign-in attempts in a row for your Mailfold admin account.\n\nMost recent attempt:\nTime: %s\nIP: %s\nDevice: %s\n\nIf this wasn't you, consider changing your password from Settings.",
				loginFailureAlertThreshold, time.Now().UTC().Format(time.RFC1123), clientIP(r), r.UserAgent(),
			),
		)
	}
}

// alertOnNewDevice records the device fingerprint behind r for username and,
// only the first time that exact fingerprint is seen for this account, emails
// a "new sign-in" alert. A device already on file is refreshed silently.
func (s *Server) alertOnNewDevice(username string, r *http.Request) {
	if s.adminStore == nil {
		return
	}
	fp := deviceFingerprint(r)
	known, err := s.adminStore.IsKnownDevice(username, fp)
	if err != nil {
		s.logger.Error("new-device check failed", "error", err)
		return
	}
	now := time.Now().UTC()
	if err := s.adminStore.RecordDevice(username, fp, now); err != nil {
		s.logger.Error("record device failed", "error", err)
	}
	if known {
		return
	}
	s.sendSecurityAlert(
		"New sign-in to your Mailfold account",
		fmt.Sprintf(
			"A sign-in to your Mailfold admin account was just recorded from a device we haven't seen before.\n\nTime: %s\nIP: %s\nDevice: %s\n\nIf this wasn't you, change your password immediately from Settings.",
			now.Format(time.RFC1123), clientIP(r), r.UserAgent(),
		),
	)
}

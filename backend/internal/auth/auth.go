// Package auth provides Mailfold's own authentication layer, which sits in
// front of the mailcow API. Rather than exposing the powerful mailcow API key
// to browsers, Mailfold authenticates a single configured administrator with a
// username and password and then issues short-lived bearer tokens. Those tokens
// are tracked in an in-process, concurrency-safe session store and expire after
// a configured lifetime. The store is deliberately in-memory: Mailfold is a
// single-node admin tool, so sessions do not need to survive a restart, and
// this keeps the design free of any external session database.
//
// The admin's password itself may be overridden at runtime (SetPasswordHash),
// so a change made through the account-settings API takes effect immediately
// without a restart; the bcrypt hash lives in the admin store, but the
// authenticator only ever sees the derived hash, never the plaintext. Optional
// two-factor enrollment does not live here — it gates login one layer up, in
// the HTTP handler — but the authenticator supports issuing and redeeming a
// short-lived "pending" token for that intermediate state.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned by CheckPassword when the supplied username
// or password does not match the configured administrator credential. It is a
// single sentinel error, deliberately not distinguishing "wrong user" from
// "wrong password", so that callers cannot leak which half was incorrect to an
// attacker probing the login endpoint.
var ErrInvalidCredentials = errors.New("invalid credentials")

// pendingTTL is how long a "password verified, awaiting the 2FA code" token
// stays redeemable. Short enough that an abandoned login attempt cannot be
// resumed much later, long enough that a user is never rushed entering a code.
const pendingTTL = 5 * time.Minute

// Session represents one authenticated login: a bearer token bound to a user
// and an expiry time. It is what Login mints and what Validate hands back to the
// rest of the backend so request handlers can learn who is calling.
type Session struct {
	// Token is the opaque bearer token the client presents on each request. It
	// is tagged json:"-" so it is never serialized into API responses; the
	// token is delivered to the client through a dedicated channel and must not
	// leak back out through, for example, a session-listing endpoint.
	Token string `json:"-"`
	// User is the username this session was issued for. It is surfaced to the
	// frontend so the UI can display who is currently logged in.
	User string `json:"user"`
	// CreatedAt is when this session was minted.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is the instant after which the session is no longer valid.
	// Validate rejects and evicts sessions once the current time passes it.
	ExpiresAt time.Time `json:"expires_at"`
	// IP and UserAgent describe the client that logged in, captured at login
	// time from the request, so the session list can read like a device list.
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// SessionInfo is what ListSessions exposes: everything about a Session except
// the raw bearer token, plus a stable, non-reversible ID a caller can use to
// target Revoke without ever learning (or being able to reconstruct) the token.
type SessionInfo struct {
	ID        string    `json:"id"`
	User      string    `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Current   bool      `json:"current"`
}

// pending is a half-authenticated login: the password matched, but a second
// factor is still required before a real Session is minted.
type pending struct {
	user      string
	expiresAt time.Time
}

// Authenticator validates login credentials against a single configured admin
// account and manages the lifecycle of the sessions it issues. One
// Authenticator is shared across all HTTP requests, so its session map is
// guarded by a mutex to make concurrent logins, validations, and logouts safe.
type Authenticator struct {
	// user is the configured administrator username that Login compares against.
	user string
	// password is the configured administrator password that Login compares
	// against when no bcrypt override has been set. It is held in plaintext
	// because it originates from configuration and is only ever compared, never
	// displayed or transmitted.
	password string
	// ttl is how long each newly minted session remains valid; it is added to
	// the current time to compute a session's ExpiresAt.
	ttl time.Duration

	// mu guards every field below against concurrent access from the many
	// goroutines that serve HTTP requests.
	mu sync.Mutex
	// passwordHash, once set via SetPasswordHash, overrides the plaintext
	// configured password: CheckPassword then verifies with bcrypt instead of a
	// constant-time byte compare. Empty means "no override yet".
	passwordHash string
	// sessions maps an active bearer token to its Session. It is the entire
	// source of truth for which tokens are currently accepted.
	sessions map[string]*Session
	// pendings maps a short-lived pending token (issued after a correct
	// password when 2FA is required) to the user it was issued for.
	pendings map[string]*pending
}

// New creates an Authenticator for the single administrator credential provided
// by configuration, with the given session lifetime. It pre-allocates the empty
// session map so the returned Authenticator is immediately ready for use.
func New(user, password string, ttl time.Duration) *Authenticator {
	return &Authenticator{
		user:     user,
		password: password,
		ttl:      ttl,
		sessions: make(map[string]*Session),
		pendings: make(map[string]*pending),
	}
}

// SetPasswordHash installs (or clears, given an empty hash) a bcrypt override
// for the configured plaintext password. It is called once at startup (from
// the stored admin account, if the password has ever been changed) and again
// every time the admin changes their password, so this single in-process
// Authenticator is always the source of truth without a restart.
func (a *Authenticator) SetPasswordHash(hash string) {
	a.mu.Lock()
	a.passwordHash = hash
	a.mu.Unlock()
}

// CheckPassword reports whether user/password match the administrator
// credential, without minting a session. Callers that also need to enforce a
// second factor call this first, then either Login (2FA off) or IssuePending +
// a later ConsumePending (2FA on). The comparisons run in constant time so
// neither which field was wrong, nor whether an override hash is in effect, is
// observable from timing.
func (a *Authenticator) CheckPassword(user, password string) bool {
	a.mu.Lock()
	hash := a.passwordHash
	a.mu.Unlock()

	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.user)) == 1
	var passOK bool
	if hash != "" {
		passOK = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	} else {
		passOK = subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
	}
	return userOK && passOK
}

// Login verifies the supplied credentials and, on success, creates and stores a
// new session, returning it to the caller. It returns ErrInvalidCredentials on
// a mismatch. meta carries the optional IP/User-Agent to record on the session.
func (a *Authenticator) Login(user, password string, meta SessionMeta) (*Session, error) {
	if !a.CheckPassword(user, password) {
		return nil, ErrInvalidCredentials
	}
	return a.MintSession(meta)
}

// SessionMeta is the client-describing context captured at login time.
type SessionMeta struct {
	IP        string
	UserAgent string
}

// MintSession issues a full session for the configured admin user directly,
// bypassing password verification. It exists for the two-factor login path,
// where CheckPassword (and, before it, ConsumePending) have already verified
// the caller's identity.
func (a *Authenticator) MintSession(meta SessionMeta) (*Session, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &Session{Token: token, User: a.user, CreatedAt: now, ExpiresAt: now.Add(a.ttl), IP: meta.IP, UserAgent: meta.UserAgent}

	a.mu.Lock()
	a.sessions[token] = sess
	a.mu.Unlock()
	return sess, nil
}

// IssuePending records that the password step succeeded and a second factor is
// still required, returning a short-lived token the client exchanges (with the
// TOTP/recovery code) for a real session via ConsumePending.
func (a *Authenticator) IssuePending() (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.pendings[token] = &pending{user: a.user, expiresAt: time.Now().Add(pendingTTL)}
	a.mu.Unlock()
	return token, nil
}

// ConsumePending redeems (and invalidates) a pending token, reporting the user
// it was issued for. It fails for an unknown, already-used, or expired token.
func (a *Authenticator) ConsumePending(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.pendings[token]
	if !ok {
		return "", false
	}
	delete(a.pendings, token)
	if time.Now().After(p.expiresAt) {
		return "", false
	}
	return p.user, true
}

// Validate looks up the session for a bearer token and returns it together with
// true when the token is known and still within its lifetime. It returns false
// for an empty, unknown, or expired token. As a convenient side effect it
// evicts a token that has expired, so lazy validation doubles as opportunistic
// cleanup even without the periodic GC sweep.
func (a *Authenticator) Validate(token string) (*Session, bool) {
	// Reject the empty token immediately: it can never be a real session and
	// avoids a needless map lookup and lock acquisition.
	if token == "" {
		return nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	sess, ok := a.sessions[token]
	if !ok {
		return nil, false
	}
	// A token past its expiry is treated as invalid and removed so it cannot be
	// reused and so the map does not accumulate dead entries.
	if time.Now().After(sess.ExpiresAt) {
		delete(a.sessions, token)
		return nil, false
	}
	return sess, true
}

// Logout invalidates a bearer token by removing its session, so any subsequent
// Validate of the same token fails. Deleting a token that is not present is a
// harmless no-op, which makes logout safe to call unconditionally.
func (a *Authenticator) Logout(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

// ListSessions returns every active session as a token-free SessionInfo, marking
// the one matching currentToken. Ordering is not guaranteed.
func (a *Authenticator) ListSessions(currentToken string) []SessionInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]SessionInfo, 0, len(a.sessions))
	for tok, sess := range a.sessions {
		out = append(out, SessionInfo{
			ID:        sessionID(tok),
			User:      sess.User,
			CreatedAt: sess.CreatedAt,
			ExpiresAt: sess.ExpiresAt,
			IP:        sess.IP,
			UserAgent: sess.UserAgent,
			Current:   tok == currentToken,
		})
	}
	return out
}

// RevokeByID logs out the single session whose SessionInfo.ID matches id. It
// reports whether a matching session was found.
func (a *Authenticator) RevokeByID(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for tok := range a.sessions {
		if sessionID(tok) == id {
			delete(a.sessions, tok)
			return true
		}
	}
	return false
}

// RevokeAllExcept logs out every session except the one presenting
// currentToken, returning how many were revoked. It is the backend for a
// "sign out all other devices" action.
func (a *Authenticator) RevokeAllExcept(currentToken string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for tok := range a.sessions {
		if tok != currentToken {
			delete(a.sessions, tok)
			n++
		}
	}
	return n
}

// GC removes every session and pending token whose expiry has already passed.
// Because Validate/ConsumePending only evict entries that happen to be
// presented again, abandoned ones would otherwise linger forever; GC bounds the
// memory used. It is safe to call periodically from a background goroutine.
func (a *Authenticator) GC() {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for token, sess := range a.sessions {
		if now.After(sess.ExpiresAt) {
			delete(a.sessions, token)
		}
	}
	for token, p := range a.pendings {
		if now.After(p.expiresAt) {
			delete(a.pendings, token)
		}
	}
}

// randomToken returns a 256-bit token drawn from the operating system's
// cryptographically secure random source, encoded as a 64-character hexadecimal
// string. The width makes tokens infeasible to guess, and hex encoding keeps
// them safe to place in HTTP headers. It returns an error only if the system
// randomness source fails, in which case no token should be issued.
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// sessionID derives a stable, non-reversible identifier for a bearer token, so
// ListSessions/RevokeByID never have to expose (or accept back) the token
// itself over the API.
func sessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

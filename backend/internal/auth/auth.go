// Package auth provides Mailfold's own authentication layer, which sits in
// front of the mailcow API. Rather than exposing the powerful mailcow API key
// to browsers, Mailfold authenticates a single configured administrator with a
// username and password and then issues short-lived bearer tokens. Those tokens
// are tracked in an in-process, concurrency-safe session store and expire after
// a configured lifetime. The store is deliberately in-memory: Mailfold is a
// single-node admin tool, so sessions do not need to survive a restart, and
// this keeps the design free of any external session database.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ErrInvalidCredentials is returned by Login when the supplied username or
// password does not match the configured administrator credential. It is a
// single sentinel error, deliberately not distinguishing "wrong user" from
// "wrong password", so that callers cannot leak which half was incorrect to an
// attacker probing the login endpoint.
var ErrInvalidCredentials = errors.New("invalid credentials")

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
	// ExpiresAt is the instant after which the session is no longer valid.
	// Validate rejects and evicts sessions once the current time passes it.
	ExpiresAt time.Time `json:"expires_at"`
}

// Authenticator validates login credentials against a single configured admin
// account and manages the lifecycle of the sessions it issues. One
// Authenticator is shared across all HTTP requests, so its session map is
// guarded by a mutex to make concurrent logins, validations, and logouts safe.
type Authenticator struct {
	// user is the configured administrator username that Login compares against.
	user string
	// password is the configured administrator password that Login compares
	// against. It is held in plaintext because it originates from configuration
	// and is only ever compared, never displayed or transmitted.
	password string
	// ttl is how long each newly minted session remains valid; it is added to
	// the current time to compute a session's ExpiresAt.
	ttl time.Duration

	// mu guards the sessions map against concurrent access from the many
	// goroutines that serve HTTP requests.
	mu sync.Mutex
	// sessions maps an active bearer token to its Session. It is the entire
	// source of truth for which tokens are currently accepted.
	sessions map[string]*Session
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
	}
}

// Login verifies the supplied credentials and, on success, creates and stores a
// new session, returning it to the caller. It returns ErrInvalidCredentials if
// either the username or password is wrong. The comparisons use
// subtle.ConstantTimeCompare so that the time taken to reject a guess does not
// depend on how many leading characters were correct, defeating timing attacks
// that would otherwise let an attacker recover the credential byte by byte.
func (a *Authenticator) Login(user, password string) (*Session, error) {
	// Compare both fields in constant time. Both comparisons are always
	// performed (rather than short-circuiting on the first mismatch) so that
	// the overall timing reveals nothing about which field failed.
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.user)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
	if !userOK || !passOK {
		return nil, ErrInvalidCredentials
	}

	// Generate a cryptographically random token; a failure of the system
	// randomness source is fatal to this login and is propagated to the caller.
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	// Record the configured username (a.user) rather than the client-supplied
	// user string. They are equal after a successful compare, but using the
	// canonical value avoids storing any attacker-influenced input.
	sess := &Session{Token: token, User: a.user, ExpiresAt: time.Now().Add(a.ttl)}

	a.mu.Lock()
	a.sessions[token] = sess
	a.mu.Unlock()
	return sess, nil
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

// GC removes every session whose expiry has already passed. Because Validate
// only evicts tokens that happen to be presented again, tokens that are simply
// abandoned would otherwise linger forever; GC bounds the memory used by the
// session map. It is safe to call periodically from a background goroutine and
// takes the same lock as the other operations to stay concurrency-safe.
func (a *Authenticator) GC() {
	// Snapshot "now" once so every session in this sweep is judged against the
	// same instant.
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for token, sess := range a.sessions {
		if now.After(sess.ExpiresAt) {
			delete(a.sessions, token)
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

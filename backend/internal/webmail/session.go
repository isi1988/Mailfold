package webmail

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Credentials are a webmail user's mailbox login, held for the lifetime of a
// session so subsequent IMAP/SMTP calls can act on the user's behalf.
//
// Storing the password in memory is inherent to a stateless webmail proxy:
// IMAP/SMTP require the password on every connection. It never leaves the
// process and is discarded on logout or expiry.
type Credentials struct {
	Email     string
	Password  string
	ExpiresAt time.Time
}

// Sessions is an in-memory store mapping bearer tokens to webmail credentials.
// It is safe for concurrent use.
type Sessions struct {
	ttl time.Duration
	now func() time.Time

	mu sync.Mutex
	m  map[string]*Credentials
}

// NewSessions creates a session store whose sessions live for ttl.
func NewSessions(ttl time.Duration) *Sessions {
	return &Sessions{ttl: ttl, now: time.Now, m: make(map[string]*Credentials)}
}

// Create stores the credentials under a fresh random token and returns it.
func (s *Sessions) Create(email, password string) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(buf)
	exp := s.now().Add(s.ttl)

	s.mu.Lock()
	s.m[token] = &Credentials{Email: email, Password: password, ExpiresAt: exp}
	s.mu.Unlock()
	return token, exp, nil
}

// Get returns the credentials for a token if present and unexpired.
func (s *Sessions) Get(token string) (*Credentials, bool) {
	if token == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cred, ok := s.m[token]
	if !ok {
		return nil, false
	}
	if s.now().After(cred.ExpiresAt) {
		delete(s.m, token)
		return nil, false
	}
	return cred, true
}

// Delete removes a session (logout).
func (s *Sessions) Delete(token string) {
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}

// Take atomically returns and removes a session, so a caller can use it as a
// single-use pending token (e.g. the password step of a two-factor login):
// two concurrent redemptions of the same token cannot both succeed.
func (s *Sessions) Take(token string) (*Credentials, bool) {
	if token == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cred, ok := s.m[token]
	if !ok {
		return nil, false
	}
	delete(s.m, token)
	if s.now().After(cred.ExpiresAt) {
		return nil, false
	}
	return cred, true
}

// GC evicts expired sessions.
func (s *Sessions) GC() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, cred := range s.m {
		if now.After(cred.ExpiresAt) {
			delete(s.m, token)
		}
	}
}

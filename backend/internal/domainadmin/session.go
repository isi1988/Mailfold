package domainadmin

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Identity is a domain admin's authenticated session state: their username
// and the domains mailcow currently reports them as scoped to (refreshed at
// login time, not cached indefinitely, so a domain reassignment takes effect
// on the admin's next sign-in).
type Identity struct {
	Username  string
	Domains   []string
	ExpiresAt time.Time
}

// Sessions is an in-memory store mapping bearer tokens to domain-admin
// identities, mirroring internal/webmail.Sessions. Safe for concurrent use.
type Sessions struct {
	ttl time.Duration
	now func() time.Time

	mu sync.Mutex
	m  map[string]*Identity
}

// NewSessions creates a session store whose sessions live for ttl.
func NewSessions(ttl time.Duration) *Sessions {
	return &Sessions{ttl: ttl, now: time.Now, m: make(map[string]*Identity)}
}

// Create stores the identity under a fresh random token and returns it.
func (s *Sessions) Create(username string, domains []string) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(buf)
	exp := s.now().Add(s.ttl)

	s.mu.Lock()
	s.m[token] = &Identity{Username: username, Domains: domains, ExpiresAt: exp}
	s.mu.Unlock()
	return token, exp, nil
}

// Get returns the identity for a token if present and unexpired.
func (s *Sessions) Get(token string) (*Identity, bool) {
	if token == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.m[token]
	if !ok {
		return nil, false
	}
	if s.now().After(id.ExpiresAt) {
		delete(s.m, token)
		return nil, false
	}
	return id, true
}

// Delete removes a session (logout).
func (s *Sessions) Delete(token string) {
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}

// GC evicts expired sessions.
func (s *Sessions) GC() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, id := range s.m {
		if now.After(id.ExpiresAt) {
			delete(s.m, token)
		}
	}
}

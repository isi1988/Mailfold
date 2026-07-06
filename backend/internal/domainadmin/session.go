package domainadmin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/sessionstore"
)

// sessionKind scopes this package's rows in the shared sessionstore table so
// they never collide with the admin's or webmail's own session rows in the
// same database. There is only ever one Sessions instance in this package
// (unlike webmail, which also has a separate pending-2FA instance), so a
// single fixed kind is enough.
const sessionKind = "domainadmin_session"

// Identity is a domain admin's authenticated session state: their username
// and the domains mailcow currently reports them as scoped to (refreshed at
// login time, not cached indefinitely, so a domain reassignment takes effect
// on the admin's next sign-in).
type Identity struct {
	Username  string
	Domains   []string
	ExpiresAt time.Time
}

// identityMeta is the JSON shape persisted alongside a session in the shared
// store; the store itself only knows about opaque tokens, subjects
// (Username here), and a metadata blob.
type identityMeta struct {
	Domains []string `json:"domains,omitempty"`
}

// Sessions is a store mapping bearer tokens to domain-admin identities,
// mirroring internal/webmail.Sessions. Safe for concurrent use, and falls
// back to an in-memory map whenever no durable store is attached, exactly
// like every other optional Mailfold feature.
type Sessions struct {
	ttl time.Duration
	now func() time.Time

	mu sync.Mutex
	m  map[string]*Identity

	// store, once attached via SetStore, persists sessions to the shared
	// database instead of m above. Unlike webmail's sessions, no cipher is
	// needed: a domain admin's scoped-domains list isn't a secret.
	store *sessionstore.Store
}

// NewSessions creates a session store whose sessions live for ttl.
func NewSessions(ttl time.Duration) *Sessions {
	return &Sessions{ttl: ttl, now: time.Now, m: make(map[string]*Identity)}
}

// SetStore attaches a durable session store, so domain-admin sessions persist
// across a restart and are visible to every backend instance sharing the
// database rather than only the one that created them.
func (s *Sessions) SetStore(store *sessionstore.Store) {
	s.mu.Lock()
	s.store = store
	s.mu.Unlock()
}

func (s *Sessions) currentStore() *sessionstore.Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store
}

// Create stores the identity under a fresh random token and returns it.
func (s *Sessions) Create(username string, domains []string) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(buf)
	now := s.now()
	exp := now.Add(s.ttl)

	if store := s.currentStore(); store != nil {
		metaJSON, _ := json.Marshal(identityMeta{Domains: domains})
		p := sessionstore.PutParams{Token: token, Kind: sessionKind, Subject: username, Meta: string(metaJSON), Now: now, ExpiresAt: exp}
		if err := store.Put(p); err != nil {
			return "", time.Time{}, err
		}
		return token, exp, nil
	}

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

	if store := s.currentStore(); store != nil {
		row, ok, err := store.Get(token, sessionKind, s.now())
		if err != nil || !ok {
			return nil, false
		}
		var m identityMeta
		_ = json.Unmarshal([]byte(row.Meta), &m)
		return &Identity{Username: row.Subject, Domains: m.Domains, ExpiresAt: row.ExpiresAt}, true
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
	if store := s.currentStore(); store != nil {
		_ = store.Delete(token, sessionKind)
		return
	}
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}

// GC evicts expired sessions.
func (s *Sessions) GC() {
	if store := s.currentStore(); store != nil {
		_ = store.GC(s.now())
		return
	}

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, id := range s.m {
		if now.After(id.ExpiresAt) {
			delete(s.m, token)
		}
	}
}

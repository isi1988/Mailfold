package webmail

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/sessionstore"
)

// Credentials are a webmail user's mailbox login, held for the lifetime of a
// session so subsequent IMAP/SMTP calls can act on the user's behalf.
//
// Storing the password is inherent to a stateless webmail proxy: IMAP/SMTP
// require the password on every connection. It never leaves the process
// unencrypted: kept in memory in the clear for the in-process fallback, or
// AES-256-GCM-encrypted at rest once a Sessions store is attached (see
// SetStore) — never both persisted and plaintext.
type Credentials struct {
	Email     string
	Password  string
	ExpiresAt time.Time
	// Attempts counts verification tries against a PENDING (not yet
	// consumed) credential, e.g. the second-factor step of a login. Real,
	// already-established sessions never touch it.
	Attempts int
	// ActingAs is set only for a session minted on behalf of a shared
	// mailbox (see internal/api/webmail_shared.go): the email of the real
	// webmail user operating it, used to attribute message assignments and
	// notes to a person rather than to the shared mailbox itself. Empty for
	// an ordinary session.
	ActingAs string
}

// sessionMeta is the JSON shape stored in a durable session row's Meta
// column (see sessionstore.Row). It currently holds only ActingAs; kept as
// a struct (rather than a bare string) so a future field can be added
// without another storage-format migration.
type sessionMeta struct {
	ActingAs string `json:"acting_as,omitempty"`
}

// maxPendingAttempts bounds how many codes can be tried against one pending
// login (see Peek) before it's invalidated — enough to absorb an honest
// typo, not enough to make guessing a 6-digit TOTP code or a recovery code
// practical.
const maxPendingAttempts = 5

// Sessions is a store mapping bearer tokens to webmail credentials. Each
// instance is scoped to one "kind" (e.g. real sessions vs. pending
// second-factor logins), since a single Mailfold process constructs more
// than one Sessions instance and, once store-backed, they all share one
// physical table — the kind keeps their tokens from colliding. It is safe
// for concurrent use, and falls back to an in-memory map whenever no durable
// store is attached, exactly like every other optional Mailfold feature.
type Sessions struct {
	ttl  time.Duration
	kind string
	now  func() time.Time

	mu sync.Mutex
	m  map[string]*Credentials

	// store/cipher, once attached via SetStore, persist sessions to the
	// shared database instead of m above. Both are required together: this
	// type always holds a plaintext mailbox password, which must never be
	// written to disk unencrypted, so a store attached without a cipher (or
	// vice versa) leaves sessions in memory rather than persisting a secret
	// in the clear.
	store  *sessionstore.Store
	cipher *sessionstore.Cipher
}

// NewSessions creates a session store whose sessions live for ttl, scoped to
// kind (see the Sessions doc comment).
func NewSessions(ttl time.Duration, kind string) *Sessions {
	return &Sessions{ttl: ttl, kind: kind, now: time.Now, m: make(map[string]*Credentials)}
}

// SetStore attaches durable, encrypted backing for these sessions, so they
// persist across a restart and are visible to every backend instance sharing
// the database rather than only the one that created them. See the Sessions
// doc comment for why both store and cipher are required together.
func (s *Sessions) SetStore(store *sessionstore.Store, cipher *sessionstore.Cipher) {
	s.mu.Lock()
	s.store, s.cipher = store, cipher
	s.mu.Unlock()
}

// currentBackend reads the attached store/cipher, if any, under the same
// mutex SetStore writes them under.
func (s *Sessions) currentBackend() (*sessionstore.Store, *sessionstore.Cipher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store, s.cipher
}

// Create stores the credentials under a fresh random token and returns it.
func (s *Sessions) Create(email, password string) (string, time.Time, error) {
	return s.mintSession(email, password, "")
}

// CreateActingAs is Create, but additionally records actingAs — the real
// webmail user operating this session — for a session minted on behalf of a
// shared mailbox (see internal/api/webmail_shared.go), so message
// assignments and notes can be attributed to a person rather than to the
// shared mailbox itself.
func (s *Sessions) CreateActingAs(email, password, actingAs string) (string, time.Time, error) {
	return s.mintSession(email, password, actingAs)
}

func (s *Sessions) mintSession(email, password, actingAs string) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(buf)
	now := s.now()
	exp := now.Add(s.ttl)

	if store, cipher := s.currentBackend(); store != nil && cipher != nil {
		enc, nonce, err := cipher.Seal([]byte(password))
		if err != nil {
			return "", time.Time{}, err
		}
		meta, err := json.Marshal(sessionMeta{ActingAs: actingAs})
		if err != nil {
			return "", time.Time{}, err
		}
		p := sessionstore.PutParams{Token: token, Kind: s.kind, Subject: email, Secret: enc, SecretNonce: nonce, Meta: string(meta), Now: now, ExpiresAt: exp}
		if err := store.Put(p); err != nil {
			return "", time.Time{}, err
		}
		return token, exp, nil
	}

	s.mu.Lock()
	s.m[token] = &Credentials{Email: email, Password: password, ExpiresAt: exp, ActingAs: actingAs}
	s.mu.Unlock()
	return token, exp, nil
}

// Get returns the credentials for a token if present and unexpired.
func (s *Sessions) Get(token string) (*Credentials, bool) {
	if token == "" {
		return nil, false
	}

	if store, cipher := s.currentBackend(); store != nil && cipher != nil {
		row, ok, err := store.Get(token, s.kind, s.now())
		if err != nil || !ok {
			return nil, false
		}
		return decryptRow(row, cipher)
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
	if store, cipher := s.currentBackend(); store != nil && cipher != nil {
		_ = store.Delete(token, s.kind)
		return
	}
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}

// Peek validates a pending token (existence, expiry, attempt budget) and
// returns its credentials WITHOUT consuming it, so a wrong second-factor code
// can be retried instead of permanently stranding the caller — unlike Take.
// Each call counts as one attempt; exceeding maxPendingAttempts invalidates
// the token exactly like expiry does. The caller should explicitly Delete
// the token once its code has actually verified, to finalize the login.
func (s *Sessions) Peek(token string) (*Credentials, bool) {
	if token == "" {
		return nil, false
	}

	if store, cipher := s.currentBackend(); store != nil && cipher != nil {
		return s.peekStoreBacked(store, cipher, token)
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
	cred.Attempts++
	if cred.Attempts > maxPendingAttempts {
		delete(s.m, token)
		return nil, false
	}
	return cred, true
}

// peekStoreBacked is Peek's database path: split out so Peek itself stays
// simple enough to read at a glance.
func (s *Sessions) peekStoreBacked(store *sessionstore.Store, cipher *sessionstore.Cipher, token string) (*Credentials, bool) {
	now := s.now()
	attempts, _, ok, err := store.IncrementAttempts(token, s.kind, now)
	if err != nil || !ok {
		return nil, false
	}
	if attempts > maxPendingAttempts {
		_ = store.Delete(token, s.kind)
		return nil, false
	}
	row, ok, err := store.Get(token, s.kind, now)
	if err != nil || !ok {
		return nil, false
	}
	cred, ok := decryptRow(row, cipher)
	if !ok {
		return nil, false
	}
	cred.Attempts = attempts
	return cred, true
}

// Take atomically returns and removes a session, so a caller can use it as a
// single-use pending token (e.g. the password step of a two-factor login):
// two concurrent redemptions of the same token cannot both succeed.
func (s *Sessions) Take(token string) (*Credentials, bool) {
	if token == "" {
		return nil, false
	}

	if store, cipher := s.currentBackend(); store != nil && cipher != nil {
		row, ok, err := store.Get(token, s.kind, s.now())
		if err != nil {
			return nil, false
		}
		if !ok {
			// Get already deleted the row if it merely expired.
			return nil, false
		}
		_ = store.Delete(token, s.kind)
		return decryptRow(row, cipher)
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
	if store, cipher := s.currentBackend(); store != nil && cipher != nil {
		_ = store.GC(s.now())
		return
	}

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, cred := range s.m {
		if now.After(cred.ExpiresAt) {
			delete(s.m, token)
		}
	}
}

// decryptRow turns a raw sessionstore row back into Credentials, decrypting
// its stored password. A decryption failure (a corrupt row, or a cipher key
// that no longer matches what encrypted it) is treated as "not found" rather
// than surfaced as an error, since the caller can't do anything about a
// broken row except treat the session as gone.
func decryptRow(row sessionstore.Row, cipher *sessionstore.Cipher) (*Credentials, bool) {
	plain, err := cipher.Open(row.Secret, row.SecretNonce)
	if err != nil {
		return nil, false
	}
	cred := &Credentials{Email: row.Subject, Password: string(plain), ExpiresAt: row.ExpiresAt}
	if row.Meta != "" {
		var m sessionMeta
		if err := json.Unmarshal([]byte(row.Meta), &m); err == nil {
			cred.ActingAs = m.ActingAs
		}
	}
	return cred, true
}

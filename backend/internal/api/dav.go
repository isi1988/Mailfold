package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/dav"
)

// davVerifier authenticates DAV requests with HTTP Basic credentials, caching
// successful checks so a full IMAP login is not required on every request (DAV
// clients are chatty).
type davVerifier struct {
	verify func(user, password string) error
	ttl    time.Duration

	mu    sync.Mutex
	cache map[string]davCacheEntry
}

type davCacheEntry struct {
	hash [32]byte
	exp  time.Time
}

func newDavVerifier(verify func(string, string) error, ttl time.Duration) *davVerifier {
	return &davVerifier{verify: verify, ttl: ttl, cache: make(map[string]davCacheEntry)}
}

// ok reports whether the credentials are valid, consulting the cache first.
func (v *davVerifier) ok(user, password string) bool {
	hash := sha256.Sum256([]byte(password))

	v.mu.Lock()
	if e, found := v.cache[user]; found && time.Now().Before(e.exp) &&
		subtle.ConstantTimeCompare(e.hash[:], hash[:]) == 1 {
		v.mu.Unlock()
		return true
	}
	v.mu.Unlock()

	if v.verify(user, password) != nil {
		return false
	}
	v.mu.Lock()
	v.cache[user] = davCacheEntry{hash: hash, exp: time.Now().Add(v.ttl)}
	v.mu.Unlock()
	return true
}

// registerDAV mounts the CardDAV/CalDAV endpoints when a store is configured.
func (s *Server) registerDAV(mux *http.ServeMux) {
	if s.davStore == nil {
		return
	}
	card := dav.NewCardDAVHandler(s.davStore, "/dav/carddav")
	mux.Handle("/dav/carddav/", s.davBasicAuth(card))
	mux.HandleFunc("GET /.well-known/carddav", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dav/carddav/", http.StatusMovedPermanently)
	})
}

// davBasicAuth guards a DAV handler with Basic authentication and attaches the
// authenticated user to the request context.
func (s *Server) davBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || !s.davAuth.ok(user, password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Mailfold DAV"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(dav.WithUser(r.Context(), user)))
	})
}

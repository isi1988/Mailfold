package domainadmin

import (
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/sessionstore"
)

// newStoreBackedSessions builds a Sessions with a real, temporary
// sessionstore.Store attached, so store-backed tests exercise the database
// path instead of the in-memory fallback the rest of this file covers.
func newStoreBackedSessions(t *testing.T, ttl time.Duration) *Sessions {
	t.Helper()
	store, err := sessionstore.Open("sqlite", t.TempDir()+"/session.db")
	if err != nil {
		t.Fatalf("sessionstore.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	s := NewSessions(ttl)
	s.SetStore(store)
	return s
}

func TestSessionsCreateGetDelete(t *testing.T) {
	s := NewSessions(time.Hour)
	token, _, err := s.Create("da1", []string{"a.com", "b.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	id, ok := s.Get(token)
	if !ok || id.Username != "da1" || len(id.Domains) != 2 {
		t.Fatalf("Get failed: %v %+v", ok, id)
	}
	s.Delete(token)
	if _, ok := s.Get(token); ok {
		t.Error("session should be gone after Delete")
	}
	if _, ok := s.Get(""); ok {
		t.Error("empty token must be invalid")
	}
}

func TestSessionsExpiryAndGC(t *testing.T) {
	s := NewSessions(-time.Second) // expire immediately
	token, _, _ := s.Create("da1", nil)
	if _, ok := s.Get(token); ok {
		t.Error("expired session should be invalid")
	}
	token2, _, _ := s.Create("da2", nil)
	s.GC()
	if _, ok := s.Get(token2); ok {
		t.Error("GC should have removed the expired session")
	}
}

// TestStoreBackedSessionsCreateGetDelete mirrors TestSessionsCreateGetDelete
// against a store-backed Sessions.
func TestStoreBackedSessionsCreateGetDelete(t *testing.T) {
	s := newStoreBackedSessions(t, time.Hour)
	token, _, err := s.Create("da1", []string{"a.com", "b.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	id, ok := s.Get(token)
	if !ok || id.Username != "da1" || len(id.Domains) != 2 {
		t.Fatalf("Get failed: %v %+v", ok, id)
	}
	s.Delete(token)
	if _, ok := s.Get(token); ok {
		t.Error("session should be gone after Delete")
	}
}

// TestStoreBackedSessionsExpiryAndGC mirrors TestSessionsExpiryAndGC against
// a store-backed Sessions.
func TestStoreBackedSessionsExpiryAndGC(t *testing.T) {
	s := newStoreBackedSessions(t, -time.Second) // expire immediately
	token, _, _ := s.Create("da1", nil)
	if _, ok := s.Get(token); ok {
		t.Error("expired session should be invalid")
	}
	token2, _, _ := s.Create("da2", nil)
	s.GC()
	if _, ok := s.Get(token2); ok {
		t.Error("GC should have removed the expired session")
	}
}

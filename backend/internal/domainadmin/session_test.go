package domainadmin

import (
	"testing"
	"time"
)

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

package sessionstore

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/session.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestPutGetDelete(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()

	err := st.Put(PutParams{Token: "tok-1", Kind: "admin_session", Subject: "admin", Meta: `{"ip":"1.2.3.4"}`, Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	row, ok, err := st.Get("tok-1", "admin_session", now)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("want row to be found")
	}
	if row.Subject != "admin" || row.Meta != `{"ip":"1.2.3.4"}` {
		t.Errorf("unexpected row: %+v", row)
	}

	// A different kind with the same token is a completely separate row.
	_, ok, err = st.Get("tok-1", "webmail_session", now)
	if err != nil {
		t.Fatalf("Get (other kind): %v", err)
	}
	if ok {
		t.Error("same token under a different kind must not be found")
	}

	if err := st.Delete("tok-1", "admin_session"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, ok, err = st.Get("tok-1", "admin_session", now)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if ok {
		t.Error("row should be gone after Delete")
	}
}

func TestGetExpired(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	err := st.Put(PutParams{Token: "tok-2", Kind: "admin_session", Subject: "admin", Now: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	_, ok, err := st.Get("tok-2", "admin_session", now)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("an expired row must not be returned")
	}
	// It should also have been deleted as a side effect.
	row, ok2, err := st.Get("tok-2", "admin_session", now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("Get (pretend not expired): %v", err)
	}
	if ok2 {
		t.Errorf("expired row should have been deleted, got %+v", row)
	}
}

func TestPutReplacesExistingRow(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	err := st.Put(PutParams{Token: "tok-3", Kind: "admin_pending", Subject: "admin", Meta: "v1", Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, _, _, err := st.IncrementAttempts("tok-3", "admin_pending", now); err != nil {
		t.Fatalf("IncrementAttempts: %v", err)
	}
	// Re-Put (e.g. IssuePending minting a fresh pending token that happens to
	// collide, astronomically unlikely but the semantics matter) resets state.
	err = st.Put(PutParams{Token: "tok-3", Kind: "admin_pending", Subject: "admin", Meta: "v2", Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put (replace): %v", err)
	}
	row, ok, err := st.Get("tok-3", "admin_pending", now)
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if row.Meta != "v2" || row.Attempts != 0 {
		t.Errorf("want reset row after replace, got %+v", row)
	}
}

func TestIncrementAttempts(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	err := st.Put(PutParams{Token: "tok-4", Kind: "admin_pending", Subject: "admin", Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	for i := 1; i <= 3; i++ {
		n, subject, ok, err := st.IncrementAttempts("tok-4", "admin_pending", now)
		if err != nil || !ok {
			t.Fatalf("IncrementAttempts #%d: ok=%v err=%v", i, ok, err)
		}
		if n != i {
			t.Errorf("attempt #%d: want count %d, got %d", i, i, n)
		}
		if subject != "admin" {
			t.Errorf("attempt #%d: want subject %q, got %q", i, "admin", subject)
		}
	}

	// An unknown token reports ok=false rather than erroring.
	_, _, ok, err := st.IncrementAttempts("no-such-token", "admin_pending", now)
	if err != nil {
		t.Fatalf("IncrementAttempts (unknown): %v", err)
	}
	if ok {
		t.Error("incrementing an unknown token should report ok=false")
	}
}

func TestIncrementAttemptsExpired(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	err := st.Put(PutParams{Token: "tok-5", Kind: "admin_pending", Subject: "admin", Now: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	_, _, ok, err := st.IncrementAttempts("tok-5", "admin_pending", now)
	if err != nil {
		t.Fatalf("IncrementAttempts: %v", err)
	}
	if ok {
		t.Error("incrementing an expired token should report ok=false")
	}
}

func TestSecretRoundTrip(t *testing.T) {
	st := openTestStore(t)
	cipher, err := NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	enc, nonce, err := cipher.Seal([]byte("hunter2"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	now := time.Now()
	err = st.Put(PutParams{Token: "tok-6", Kind: "webmail_session", Subject: "user@example.com", Secret: enc, SecretNonce: nonce, Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	row, ok, err := st.Get("tok-6", "webmail_session", now)
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	plain, err := cipher.Open(row.Secret, row.SecretNonce)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(plain) != "hunter2" {
		t.Errorf("want decrypted secret %q, got %q", "hunter2", plain)
	}
}

func TestListBySubjectAndDeleteByHash(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	err := st.Put(PutParams{Token: "tok-7", Kind: "admin_session", Subject: "admin", Meta: `{"ip":"1"}`, Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	err = st.Put(PutParams{Token: "tok-8", Kind: "admin_session", Subject: "admin", Meta: `{"ip":"2"}`, Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// A different subject must not show up in admin's list.
	err = st.Put(PutParams{Token: "tok-9", Kind: "admin_session", Subject: "someone-else", Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	list, err := st.ListBySubject("admin_session", "admin", now)
	if err != nil {
		t.Fatalf("ListBySubject: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 sessions for admin, got %d", len(list))
	}

	deleted, err := st.DeleteByHash(list[0].TokenHash, "admin_session")
	if err != nil {
		t.Fatalf("DeleteByHash: %v", err)
	}
	if !deleted {
		t.Error("want DeleteByHash to report a row was removed")
	}
	list, err = st.ListBySubject("admin_session", "admin", now)
	if err != nil {
		t.Fatalf("ListBySubject: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 session remaining for admin, got %d", len(list))
	}

	// Deleting an already-gone hash is a harmless no-op.
	deleted, err = st.DeleteByHash(list[0].TokenHash+"stale", "admin_session")
	if err != nil {
		t.Fatalf("DeleteByHash (unknown): %v", err)
	}
	if deleted {
		t.Error("DeleteByHash of an unknown hash should report false")
	}
}

func TestDeleteBySubjectExcept(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	for _, tok := range []string{"tok-a", "tok-b", "tok-c"} {
		err := st.Put(PutParams{Token: tok, Kind: "admin_session", Subject: "admin", Now: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatalf("Put(%s): %v", tok, err)
		}
	}
	n, err := st.DeleteBySubjectExcept("admin_session", "admin", "tok-b")
	if err != nil {
		t.Fatalf("DeleteBySubjectExcept: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 deleted, got %d", n)
	}
	if _, ok, _ := st.Get("tok-b", "admin_session", now); !ok {
		t.Error("the kept token should survive")
	}
	if _, ok, _ := st.Get("tok-a", "admin_session", now); ok {
		t.Error("tok-a should have been deleted")
	}
}

func TestGC(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	err := st.Put(PutParams{Token: "expired", Kind: "admin_session", Subject: "admin", Now: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	err = st.Put(PutParams{Token: "live", Kind: "admin_session", Subject: "admin", Now: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	var before int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM session_token`).Scan(&before); err != nil {
		t.Fatalf("count before GC: %v", err)
	}
	if before != 2 {
		t.Fatalf("want 2 rows before GC, got %d", before)
	}

	if err := st.GC(now); err != nil {
		t.Fatalf("GC: %v", err)
	}

	var after int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM session_token`).Scan(&after); err != nil {
		t.Fatalf("count after GC: %v", err)
	}
	if after != 1 {
		t.Fatalf("want 1 row surviving GC, got %d", after)
	}
	if _, ok, _ := st.Get("live", "admin_session", now); !ok {
		t.Error("the live row should survive GC")
	}
}

package webpush

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/push.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestGetOrCreateVAPIDKeysGeneratesOnce(t *testing.T) {
	st := openTestStore(t)
	calls := 0
	gen := func() (string, string, error) {
		calls++
		return "priv", "pub", nil
	}
	pub, priv, err := st.GetOrCreateVAPIDKeys(gen)
	if err != nil {
		t.Fatalf("GetOrCreateVAPIDKeys: %v", err)
	}
	if pub != "pub" || priv != "priv" {
		t.Errorf("unexpected keys: pub=%q priv=%q", pub, priv)
	}
	if calls != 1 {
		t.Fatalf("want generate called once, got %d", calls)
	}

	// A second call must return the SAME persisted pair, not generate again.
	pub2, priv2, err := st.GetOrCreateVAPIDKeys(gen)
	if err != nil {
		t.Fatalf("GetOrCreateVAPIDKeys (again): %v", err)
	}
	if pub2 != pub || priv2 != priv {
		t.Errorf("want stable keys across calls, got %q/%q then %q/%q", pub, priv, pub2, priv2)
	}
	if calls != 1 {
		t.Errorf("generate should not be called again once a key pair exists, called %d times", calls)
	}
}

func TestAddListRemoveSubscription(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()

	if err := st.AddSubscription("user@example.com", "https://push.example/ep1", "p256dh-1", "auth-1", []byte("enc"), []byte("nonce"), now); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if err := st.AddSubscription("user@example.com", "https://push.example/ep2", "p256dh-2", "auth-2", []byte("enc"), []byte("nonce"), now); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	// A different mailbox's subscription must not show up in user@example.com's list.
	if err := st.AddSubscription("other@example.com", "https://push.example/ep3", "p256dh-3", "auth-3", []byte("enc"), []byte("nonce"), now); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}

	list, err := st.ListByEmail("user@example.com")
	if err != nil {
		t.Fatalf("ListByEmail: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 subscriptions for user@example.com, got %d", len(list))
	}

	all, err := st.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 subscriptions total, got %d", len(all))
	}

	if err := st.RemoveSubscription("https://push.example/ep1"); err != nil {
		t.Fatalf("RemoveSubscription: %v", err)
	}
	list, err = st.ListByEmail("user@example.com")
	if err != nil {
		t.Fatalf("ListByEmail: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 subscription remaining, got %d", len(list))
	}
}

func TestAddSubscriptionUpsertsSameEndpoint(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	if err := st.AddSubscription("user@example.com", "https://push.example/ep1", "old-key", "old-auth", []byte("enc"), []byte("nonce"), now); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if err := st.UpdateLastUID("https://push.example/ep1", 42); err != nil {
		t.Fatalf("UpdateLastUID: %v", err)
	}
	// Re-subscribing with the same endpoint (browser re-registered) must
	// reset last_uid so mail that arrived before the resubscribe isn't
	// silently skipped.
	if err := st.AddSubscription("user@example.com", "https://push.example/ep1", "new-key", "new-auth", []byte("enc"), []byte("nonce"), now); err != nil {
		t.Fatalf("AddSubscription (upsert): %v", err)
	}
	list, err := st.ListByEmail("user@example.com")
	if err != nil {
		t.Fatalf("ListByEmail: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 subscription (upserted, not duplicated), got %d", len(list))
	}
	if list[0].P256dh != "new-key" || list[0].LastUID != 0 {
		t.Errorf("want upserted key and reset last_uid, got %+v", list[0])
	}
}

func TestUpdateLastUID(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	if err := st.AddSubscription("user@example.com", "https://push.example/ep1", "k", "a", []byte("enc"), []byte("nonce"), now); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if err := st.UpdateLastUID("https://push.example/ep1", 7); err != nil {
		t.Fatalf("UpdateLastUID: %v", err)
	}
	list, err := st.ListByEmail("user@example.com")
	if err != nil {
		t.Fatalf("ListByEmail: %v", err)
	}
	if list[0].LastUID != 7 {
		t.Errorf("want last_uid 7, got %d", list[0].LastUID)
	}
}

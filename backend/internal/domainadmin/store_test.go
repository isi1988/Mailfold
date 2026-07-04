package domainadmin

import (
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open("sqlite", t.TempDir()+"/domainadmin.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestLoginPasswordRoundTrip(t *testing.T) {
	st := openTestStore(t)
	if _, ok, err := st.GetLoginPassword("alice"); err != nil || ok {
		t.Fatalf("unset login should not be found: ok=%v err=%v", ok, err)
	}
	if err := st.SetLoginPassword("alice", "hash1", time.Now()); err != nil {
		t.Fatalf("SetLoginPassword: %v", err)
	}
	hash, ok, err := st.GetLoginPassword("alice")
	if err != nil || !ok || hash != "hash1" {
		t.Fatalf("GetLoginPassword = %q, %v, %v, want hash1, true, nil", hash, ok, err)
	}
	// Setting again replaces rather than erroring on the existing row.
	if err := st.SetLoginPassword("alice", "hash2", time.Now()); err != nil {
		t.Fatalf("SetLoginPassword (replace): %v", err)
	}
	hash, _, _ = st.GetLoginPassword("alice")
	if hash != "hash2" {
		t.Errorf("hash after replace = %q, want hash2", hash)
	}

	if err := st.DeleteLogin("alice"); err != nil {
		t.Fatalf("DeleteLogin: %v", err)
	}
	if _, ok, _ := st.GetLoginPassword("alice"); ok {
		t.Error("login should be gone after DeleteLogin")
	}
}

func TestCreateAndGetProviderAllDomains(t *testing.T) {
	st := openTestStore(t)
	id, err := st.CreateProvider(Provider{
		Name: "Corp SSO", Issuer: "https://idp.example.com", ClientID: "cid",
		ClientSecretEnc: []byte("enc"), ClientSecretNonce: []byte("nonce"),
		RedirectURL: "https://mailfold.example.com/api/auth/sso/callback",
		AllDomains:  true, Active: true,
	}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	p, ok, err := st.GetProvider(id)
	if err != nil || !ok {
		t.Fatalf("GetProvider: ok=%v err=%v", ok, err)
	}
	if p.Name != "Corp SSO" || !p.AllDomains || len(p.Domains) != 0 {
		t.Errorf("unexpected provider: %+v", p)
	}
}

func TestCreateAndGetProviderScoped(t *testing.T) {
	st := openTestStore(t)
	id, err := st.CreateProvider(Provider{
		Name: "Domain SSO", Issuer: "https://idp.example.com", ClientID: "cid",
		AllDomains: false, Domains: []string{"a.com", "b.com"}, CreatedBy: "domainadmin1", Active: true,
	}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	p, _, err := st.GetProvider(id)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if p.AllDomains || p.CreatedBy != "domainadmin1" || len(p.Domains) != 2 {
		t.Errorf("unexpected scoped provider: %+v", p)
	}
}

func TestProvidersForDomain(t *testing.T) {
	st := openTestStore(t)
	now := time.Now()
	globalID, _ := st.CreateProvider(Provider{Name: "Global", AllDomains: true, Active: true}, now)
	scopedAID, _ := st.CreateProvider(Provider{Name: "ScopedA", Domains: []string{"a.com"}, Active: true}, now)
	_, _ = st.CreateProvider(Provider{Name: "ScopedB", Domains: []string{"b.com"}, Active: true}, now)
	inactiveID, _ := st.CreateProvider(Provider{Name: "Inactive", AllDomains: true, Active: false}, now)

	list, err := st.ProvidersForDomain("a.com")
	if err != nil {
		t.Fatalf("ProvidersForDomain: %v", err)
	}
	ids := map[int64]bool{}
	for _, p := range list {
		ids[p.ID] = true
	}
	if !ids[globalID] || !ids[scopedAID] {
		t.Errorf("expected global and scopedA providers for a.com, got %+v", list)
	}
	if ids[inactiveID] {
		t.Error("inactive provider should not be returned")
	}
	if len(list) != 2 {
		t.Errorf("expected exactly 2 providers for a.com, got %d: %+v", len(list), list)
	}

	// b.com sees global + ScopedB, not ScopedA.
	listB, err := st.ProvidersForDomain("b.com")
	if err != nil {
		t.Fatalf("ProvidersForDomain(b.com): %v", err)
	}
	if len(listB) != 2 {
		t.Errorf("expected exactly 2 providers for b.com, got %d: %+v", len(listB), listB)
	}

	// A domain with no scoped provider only sees the global one.
	listC, err := st.ProvidersForDomain("c.com")
	if err != nil {
		t.Fatalf("ProvidersForDomain(c.com): %v", err)
	}
	if len(listC) != 1 || listC[0].ID != globalID {
		t.Errorf("expected only the global provider for c.com, got %+v", listC)
	}
}

func TestUpdateProvider(t *testing.T) {
	st := openTestStore(t)
	id, err := st.CreateProvider(Provider{Name: "Original", Domains: []string{"a.com"}, Active: true}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	p, _, _ := st.GetProvider(id)
	p.Name = "Renamed"
	p.Domains = []string{"b.com", "c.com"}
	if err := st.UpdateProvider(p, time.Now()); err != nil {
		t.Fatalf("UpdateProvider: %v", err)
	}
	got, _, _ := st.GetProvider(id)
	if got.Name != "Renamed" || len(got.Domains) != 2 {
		t.Errorf("update did not persist: %+v", got)
	}

	// Switching to AllDomains clears the scoped domain rows.
	got.AllDomains = true
	if err := st.UpdateProvider(got, time.Now()); err != nil {
		t.Fatalf("UpdateProvider (all domains): %v", err)
	}
	final, _, _ := st.GetProvider(id)
	if !final.AllDomains || len(final.Domains) != 0 {
		t.Errorf("switching to all-domains should clear scoped domains: %+v", final)
	}
}

func TestDeleteProvider(t *testing.T) {
	st := openTestStore(t)
	id, err := st.CreateProvider(Provider{Name: "ToDelete", Domains: []string{"a.com"}, Active: true}, time.Now())
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if err := st.DeleteProvider(id); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	if _, ok, _ := st.GetProvider(id); ok {
		t.Error("provider should be gone after delete")
	}
	// Its domain scope rows should not leak into another provider's lookup.
	list, err := st.ProvidersForDomain("a.com")
	if err != nil {
		t.Fatalf("ProvidersForDomain: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected no providers for a.com after delete, got %+v", list)
	}
}

func TestListProviders(t *testing.T) {
	st := openTestStore(t)
	_, _ = st.CreateProvider(Provider{Name: "One", AllDomains: true, Active: true}, time.Now())
	_, _ = st.CreateProvider(Provider{Name: "Two", AllDomains: true, Active: true}, time.Now())
	list, err := st.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(list))
	}
}

func TestMailboxCredential(t *testing.T) {
	st := openTestStore(t)
	if _, ok, err := st.GetMailboxCredential("a@example.com"); err != nil || ok {
		t.Fatalf("unset credential should not be found: ok=%v err=%v", ok, err)
	}
	if err := st.SetMailboxCredential("a@example.com", "pwid1", []byte("enc"), []byte("nonce"), time.Now()); err != nil {
		t.Fatalf("SetMailboxCredential: %v", err)
	}
	c, ok, err := st.GetMailboxCredential("a@example.com")
	if err != nil || !ok || c.AppPasswdID != "pwid1" || string(c.AppPasswdEnc) != "enc" {
		t.Fatalf("GetMailboxCredential = %+v, %v, %v", c, ok, err)
	}
	if err := st.DeleteMailboxCredential("a@example.com"); err != nil {
		t.Fatalf("DeleteMailboxCredential: %v", err)
	}
	if _, ok, _ := st.GetMailboxCredential("a@example.com"); ok {
		t.Error("credential should be gone after delete")
	}
}

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/admin"
)

func TestLoginFailureTrackerFiresOncePerBurst(t *testing.T) {
	tr := newLoginFailureTracker()

	for i := 1; i < loginFailureAlertThreshold; i++ {
		if tr.Fail("admin") {
			t.Fatalf("Fail should not fire before the threshold (attempt %d)", i)
		}
	}
	if !tr.Fail("admin") {
		t.Fatal("Fail should fire exactly on the threshold-th attempt")
	}
	// Further failures in the same burst must not fire again.
	if tr.Fail("admin") {
		t.Fatal("Fail should not fire again within the same burst")
	}
}

func TestLoginFailureTrackerResetStartsFreshBurst(t *testing.T) {
	tr := newLoginFailureTracker()

	for i := 0; i < loginFailureAlertThreshold; i++ {
		tr.Fail("admin")
	}
	tr.Reset("admin")

	for i := 1; i < loginFailureAlertThreshold; i++ {
		if tr.Fail("admin") {
			t.Fatalf("Fail should not fire before the threshold after Reset (attempt %d)", i)
		}
	}
	if !tr.Fail("admin") {
		t.Fatal("Fail should fire again once a fresh burst reaches the threshold")
	}
}

func TestLoginFailureTrackerIsPerUsername(t *testing.T) {
	tr := newLoginFailureTracker()

	for i := 0; i < loginFailureAlertThreshold-1; i++ {
		tr.Fail("alice")
	}
	if tr.Fail("bob") {
		t.Fatal("a different username should have its own independent streak")
	}
}

func TestDeviceFingerprintStableAndDistinct(t *testing.T) {
	r1 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r1.Header.Set("User-Agent", "ua-1")
	r1.RemoteAddr = "10.0.0.1:1234"

	r2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r2.Header.Set("User-Agent", "ua-1")
	r2.RemoteAddr = "10.0.0.1:5678" // different port, same IP

	if deviceFingerprint(r1) != deviceFingerprint(r2) {
		t.Error("fingerprint should be stable across requests from the same IP+UA")
	}

	r3 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r3.Header.Set("User-Agent", "ua-2")
	r3.RemoteAddr = "10.0.0.1:1234"
	if deviceFingerprint(r1) == deviceFingerprint(r3) {
		t.Error("a different User-Agent should yield a different fingerprint")
	}
}

// TestFailedLoginBurstSendsAlertEmail exercises the alert end-to-end through
// the real login handler: three wrong passwords in a row must queue exactly
// one alert email via the notify sender, and the login attempts themselves
// must still behave exactly as before (401, audit-logged).
func TestFailedLoginBurstSendsAlertEmail(t *testing.T) {
	h, srv, smtpBE := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	seedNotifySender(t, srv, "notify@example.com", "app-password")
	if err := srv.adminStore.SetProfile(srv.cfg.AdminUser, admin.ProfileUpdate{Email: "owner@example.com"}, time.Now()); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	for i := 0; i < loginFailureAlertThreshold; i++ {
		rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"wrong"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i+1, rec.Code)
		}
	}

	count, body := smtpBE.sent()
	if count != 1 {
		t.Fatalf("want exactly 1 alert email after a %d-attempt burst, got %d", loginFailureAlertThreshold, count)
	}
	if !strings.Contains(body, "failed sign-in attempts") {
		t.Errorf("alert body should mention the failed sign-in attempts, got: %s", body)
	}
}

// TestNewDeviceLoginSendsAlertEmailOnce exercises the new-device alert
// end-to-end: the first successful login from a given IP+User-Agent should
// queue one alert, and a second login from the same "device" should not
// queue another.
func TestNewDeviceLoginSendsAlertEmailOnce(t *testing.T) {
	h, srv, smtpBE := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	seedNotifySender(t, srv, "notify@example.com", "app-password")
	if err := srv.adminStore.SetProfile(srv.cfg.AdminUser, admin.ProfileUpdate{Email: "owner@example.com"}, time.Now()); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	rec := do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("first login: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	count, body := smtpBE.sent()
	if count != 1 {
		t.Fatalf("want exactly 1 new-device alert after the first login, got %d", count)
	}
	if !strings.Contains(body, "New sign-in") {
		t.Errorf("alert body should mention the new sign-in, got: %s", body)
	}

	rec = do(h, http.MethodPost, "/api/auth/login", "", `{"user":"admin","password":"pw"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("second login: want 200, got %d", rec.Code)
	}
	count, _ = smtpBE.sent()
	if count != 1 {
		t.Fatalf("a repeat login from the same device should not send another alert, got %d sent", count)
	}
}

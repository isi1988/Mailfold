package api

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"

	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// newPushTestServer builds a server with a database, admin encryption key,
// and a real (fake) IMAP backend all configured — everything Web Push needs
// to be fully enabled — and returns both the HTTP handler and the
// underlying *Server so a test can call PollWebPush directly.
func newPushTestServer(t *testing.T, mcURL, imapAddr string) (http.Handler, *Server) {
	t.Helper()
	cfg := &config.Config{
		MailcowBaseURL:    mcURL,
		MailcowAPIKey:     "k",
		AdminUser:         "admin",
		AdminPassword:     "pw",
		SessionTTL:        time.Hour,
		CORSOrigins:       []string{"*"},
		LoginRateMax:      100,
		LoginRateWindow:   time.Minute,
		MaxBodyBytes:      1 << 20,
		IMAPAddr:          imapAddr,
		MailUseTLS:        false,
		WebmailSessionTTL: time.Hour,
		DBDriver:          "sqlite",
		DBPath:            t.TempDir() + "/push.db",
		AdminEncKey:       bytes.Repeat([]byte{7}, 32),
	}
	mc := mailcow.NewClient(mcURL, "k", false)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	limiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(cfg, mc, authn, limiter, logger)
	return srv.Handler(), srv
}

// fakePushKeys generates a syntactically valid (but unclaimed) P-256 key pair
// and random auth secret, in the exact base64url shapes a real
// PushSubscription.toJSON() would produce. webpush-go performs real RFC 8291
// payload encryption against these before ever making an HTTP call, so a
// placeholder string like "key1" would fail before reaching the network —
// these must be real curve points.
func fakePushKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdh key: %v", err)
	}
	p256dh = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	authBytes := make([]byte, 16)
	if _, err := rand.Read(authBytes); err != nil {
		t.Fatalf("random auth secret: %v", err)
	}
	auth = base64.RawURLEncoding.EncodeToString(authBytes)
	return p256dh, auth
}

// appendTestMessage appends one more message directly to INBOX on the fake
// IMAP server, so a poll after this point has genuinely new mail to detect
// (rather than colliding with the sinceUID==0 "first sweep" baseline that
// TestCheckSince's since-base-1 trick would hit if the mailbox only ever has
// its single seed message).
func appendTestMessage(t *testing.T, imapAddr, subject string) {
	t.Helper()
	c, err := imapclient.Dial(imapAddr)
	if err != nil {
		t.Fatalf("dial imap: %v", err)
	}
	defer func() { _ = c.Logout() }()
	if err := c.Login("username", "password"); err != nil {
		t.Fatalf("imap login: %v", err)
	}
	raw := "From: sender@example.com\r\nTo: username@example.com\r\nSubject: " + subject + "\r\n\r\nBody.\r\n"
	if err := c.Append("INBOX", []string{imap.SeenFlag}, time.Now(), bytes.NewReader([]byte(raw))); err != nil {
		t.Fatalf("append: %v", err)
	}
}

func TestWebPushDisabledWithoutEncKey(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withIMAP: true}) // no enc key
	tok := webmailToken(t, h)
	if rec := do(h, http.MethodGet, "/api/webmail/push/vapid-public-key", tok, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("vapid-public-key without cipher = %d, want 501", rec.Code)
	}
}

func TestWebPushSubscribeListUnsubscribe(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	tok := webmailToken(t, h)

	rec := do(h, http.MethodGet, "/api/webmail/push/vapid-public-key", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("vapid-public-key: %d %s", rec.Code, rec.Body.String())
	}
	var vapid struct {
		PublicKey string `json:"public_key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &vapid)
	if vapid.PublicKey == "" {
		t.Fatal("want a non-empty VAPID public key")
	}

	p256dh, auth := fakePushKeys(t)
	subBody := fmt.Sprintf(`{"endpoint":"https://push.example/ep1","keys":{"p256dh":%q,"auth":%q}}`, p256dh, auth)
	if rec := do(h, http.MethodPost, "/api/webmail/push/subscribe", tok, subBody); rec.Code != http.StatusOK {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}

	// Missing fields are rejected.
	if rec := do(h, http.MethodPost, "/api/webmail/push/subscribe", tok, `{"endpoint":""}`); rec.Code != http.StatusBadRequest {
		t.Errorf("subscribe with no endpoint: want 400, got %d", rec.Code)
	}

	rec = do(h, http.MethodGet, "/api/webmail/push/subscriptions", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("subscriptions: %d %s", rec.Code, rec.Body.String())
	}
	var list []pushSubscriptionSummary
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Endpoint != "https://push.example/ep1" {
		t.Fatalf("unexpected subscription list: %+v", list)
	}

	if rec := do(h, http.MethodDelete, "/api/webmail/push/subscribe", tok, `{"endpoint":"https://push.example/ep1"}`); rec.Code != http.StatusOK {
		t.Fatalf("unsubscribe: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodGet, "/api/webmail/push/subscriptions", tok, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("want 0 subscriptions after unsubscribe, got %d", len(list))
	}
}

func TestWebPushUnauthorized(t *testing.T) {
	h, _, _ := newAccountTestServer(t, accountTestOpts{withDB: true, withEncKey: true, withIMAP: true})
	if rec := do(h, http.MethodGet, "/api/webmail/push/vapid-public-key", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 with no token, got %d", rec.Code)
	}
}

// TestWebPushPollerSendsNotification exercises the full background-poll
// path end to end: subscribe, poll once to establish the baseline (must NOT
// notify), append a genuinely new message to the fake IMAP INBOX, poll
// again, and verify a real, VAPID-signed HTTP POST reaches the "push
// service" (a local httptest.Server standing in for one).
func TestWebPushPollerSendsNotification(t *testing.T) {
	var mu sync.Mutex
	var requests int
	var lastAuth string
	pushSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		lastAuth = r.Header.Get("Authorization")
		mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("push request body should not be empty")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer pushSvc.Close()

	imapAddr := startMemIMAP(t)
	h, srv := newPushTestServer(t, mockMailcow(t, 0, "").URL, imapAddr)
	tok := webmailToken(t, h)

	p256dh, auth := fakePushKeys(t)
	subBody := fmt.Sprintf(`{"endpoint":%q,"keys":{"p256dh":%q,"auth":%q}}`, pushSvc.URL, p256dh, auth)
	if rec := do(h, http.MethodPost, "/api/webmail/push/subscribe", tok, subBody); rec.Code != http.StatusOK {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}

	// First sweep only establishes the baseline; it must not notify about
	// mail that already existed when the subscription was created.
	srv.PollWebPush()
	mu.Lock()
	if requests != 0 {
		t.Fatalf("baseline sweep should not send a notification, got %d requests", requests)
	}
	mu.Unlock()

	appendTestMessage(t, imapAddr, "Hello from the poller test")

	srv.PollWebPush()
	mu.Lock()
	defer mu.Unlock()
	if requests != 1 {
		t.Fatalf("want exactly 1 notification after new mail arrived, got %d", requests)
	}
	if !strings.Contains(strings.ToLower(lastAuth), "vapid") {
		t.Errorf("want a VAPID Authorization header, got %q", lastAuth)
	}
}

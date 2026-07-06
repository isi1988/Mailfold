package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	webpushgo "github.com/SherClockHolmes/webpush-go"

	"github.com/isi1988/Mailfold/backend/internal/webmail"
	"github.com/isi1988/Mailfold/backend/internal/webpush"
)

// pushPollInterval is how often the background poller (PollWebPush) checks
// every subscribed mailbox for new mail, independent of whether any browser
// tab happens to be open anywhere — the entire point of a push notification
// versus the existing in-tab SSE stream (handleWebmailEvents).
const pushPollInterval = 60 * time.Second

// PushPollInterval exposes pushPollInterval to package app, which owns the
// actual background ticker that drives PollWebPush.
func PushPollInterval() time.Duration { return pushPollInterval }

// registerWebmailPushRoutes wires Web Push subscription management. All
// routes run behind requireWebmail: a subscription always belongs to the
// currently-authenticated mailbox, never one named by the caller.
func (s *Server) registerWebmailPushRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/webmail/push/vapid-public-key", s.requireWebmail(s.handleWebPushVAPIDKey))
	mux.HandleFunc("GET /api/webmail/push/subscriptions", s.requireWebmail(s.handleWebPushList))
	mux.HandleFunc("POST /api/webmail/push/subscribe", s.requireWebmail(s.handleWebPushSubscribe))
	mux.HandleFunc("DELETE /api/webmail/push/subscribe", s.requireWebmail(s.handleWebPushUnsubscribe))
}

// requireWebPush reports 501 and returns false unless push notifications are
// available: it needs both a database (to persist subscriptions) and the
// admin encryption key (subscriptions always carry the mailbox's own IMAP
// password, encrypted, so the background poller can check for new mail with
// no browser tab open at all — that password must never be stored in the
// clear, so the feature simply stays off without a cipher rather than
// degrade to a weaker, tab-required mode).
func (s *Server) requireWebPush(w http.ResponseWriter) bool {
	if s.webpushStore == nil || s.webpushCipher == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "set MAILFOLD_DB_PATH and MAILFOLD_ADMIN_ENC_KEY to enable push notifications"})
		return false
	}
	return true
}

// handleWebPushVAPIDKey returns the server's public VAPID key, which the
// browser needs to call pushManager.subscribe().
func (s *Server) handleWebPushVAPIDKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebPush(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": s.vapidPublicKey})
}

// webPushSubscribeRequest is the browser's PushSubscription, serialized via
// its own toJSON() method — endpoint plus the two base64url-encoded keys.
type webPushSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// handleWebPushSubscribe stores a new push subscription for the
// currently-authenticated mailbox. The mailbox's IMAP password is taken from
// the caller's own session (never re-requested from the browser) and stored
// encrypted, so the background poller can check this mailbox even once the
// browser tab that subscribed is long closed.
func (s *Server) handleWebPushSubscribe(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebPush(w) {
		return
	}
	var req webPushSubscribeRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint and keys are required"})
		return
	}
	cred := webmailCreds(r)
	enc, nonce, err := s.webpushCipher.Seal([]byte(cred.Password))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	err = s.webpushStore.AddSubscription(cred.Email, req.Endpoint, req.Keys.P256dh, req.Keys.Auth, enc, nonce, time.Now())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// webPushUnsubscribeRequest names the subscription to remove by its
// endpoint — the same opaque URL the browser's PushSubscription carries,
// unique per device.
type webPushUnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// handleWebPushUnsubscribe removes a subscription. Unlike most delete
// endpoints in this codebase it is NOT scoped to the caller's own mailbox by
// query, only by the fact that only that mailbox's own client would ever
// hold this exact endpoint string — matching the browser's own
// pushManager.getSubscription().unsubscribe() flow, which likewise has no
// broader authorization concept than "know your own subscription".
func (s *Server) handleWebPushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebPush(w) {
		return
	}
	var req webPushUnsubscribeRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.webpushStore.RemoveSubscription(req.Endpoint); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// pushSubscriptionSummary is what Settings shows for an enrolled device —
// never the raw endpoint's push keys, which stay server-side.
type pushSubscriptionSummary struct {
	ID        int64     `json:"id"`
	Endpoint  string    `json:"endpoint"`
	CreatedAt time.Time `json:"created_at"`
}

// handleWebPushList returns every device subscribed for the
// currently-authenticated mailbox, so Settings can show "2 devices" and let
// the user clear one that's no longer in use.
func (s *Server) handleWebPushList(w http.ResponseWriter, r *http.Request) {
	if !s.requireWebPush(w) {
		return
	}
	cred := webmailCreds(r)
	subs, err := s.webpushStore.ListByEmail(cred.Email)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]pushSubscriptionSummary, len(subs))
	for i, sub := range subs {
		out[i] = pushSubscriptionSummary{ID: sub.ID, Endpoint: sub.Endpoint, CreatedAt: sub.CreatedAt}
	}
	writeJSON(w, http.StatusOK, out)
}

// PollWebPush sweeps every push subscription, checking each mailbox for new
// INBOX mail since the last sweep and sending a Web Push notification when
// there is any. It is a no-op when push notifications are unavailable, and
// is intended to be called periodically from a background goroutine — see
// pushPollInterval.
func (s *Server) PollWebPush() {
	if s.webpushStore == nil || s.webpushCipher == nil {
		return
	}
	subs, err := s.webpushStore.ListAll()
	if err != nil {
		s.logger.Error("webpush: list subscriptions failed", "error", err)
		return
	}
	for _, sub := range subs {
		s.pollOneWebPushSubscription(sub)
	}
}

// pollOneWebPushSubscription checks one subscription's mailbox for mail
// newer than its last recorded UID, notifying and advancing the UID when
// there is any. A subscription whose stored password no longer works (the
// mailbox password changed, or the IMAP server rejects it for any other
// reason) is removed rather than retried forever against credentials that
// will never succeed again.
func (s *Server) pollOneWebPushSubscription(sub webpush.Subscription) {
	password, err := s.webpushCipher.Open(sub.PasswordEnc, sub.PasswordNonce)
	if err != nil {
		s.logger.Error("webpush: decrypt password failed, removing subscription", "email", sub.Email, "error", err)
		_ = s.webpushStore.RemoveSubscription(sub.Endpoint)
		return
	}
	msgs, newUID, err := s.webmail.CheckSince(sub.Email, string(password), sub.LastUID)
	if err != nil {
		s.logger.Warn("webpush: IMAP check failed, removing subscription", "email", sub.Email, "error", err)
		_ = s.webpushStore.RemoveSubscription(sub.Endpoint)
		return
	}
	firstSweep := sub.LastUID == 0
	if err := s.webpushStore.UpdateLastUID(sub.Endpoint, newUID); err != nil {
		s.logger.Error("webpush: update last uid failed", "error", err)
	}
	// A first sweep only establishes the baseline — exactly like
	// handleWebmailEvents' own baseline CheckSince(..., 0) call — so
	// subscribing never triggers a notification about mail that already
	// existed.
	if firstSweep || len(msgs) == 0 {
		return
	}
	s.sendWebPushNotification(sub, msgs)
}

func (s *Server) sendWebPushNotification(sub webpush.Subscription, msgs []webmail.MessageHeader) {
	payload, _ := json.Marshal(map[string]string{
		"title": "New mail",
		"body":  webPushBody(msgs),
	})
	subscriber := s.cfg.VAPIDContactEmail
	if !strings.HasPrefix(subscriber, "mailto:") {
		subscriber = "mailto:" + subscriber
	}
	target := &webpushgo.Subscription{Endpoint: sub.Endpoint, Keys: webpushgo.Keys{P256dh: sub.P256dh, Auth: sub.Auth}}
	resp, err := webpushgo.SendNotification(payload, target, &webpushgo.Options{
		Subscriber:      subscriber,
		VAPIDPublicKey:  s.vapidPublicKey,
		VAPIDPrivateKey: s.vapidPrivateKey,
		TTL:             int(pushPollInterval.Seconds()),
	})
	if err != nil {
		s.logger.Warn("webpush: send failed", "endpoint", sub.Endpoint, "error", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	// A 404/410 means the push service considers this endpoint gone — the
	// browser unsubscribed, uninstalled, or the endpoint simply expired —
	// so there is no point sending to it again.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		_ = s.webpushStore.RemoveSubscription(sub.Endpoint)
	}
}

// webPushBody summarises one sweep's new messages into a single line: the
// sender's name for exactly one message, or a count for several.
func webPushBody(msgs []webmail.MessageHeader) string {
	if len(msgs) == 1 {
		if from := msgs[0].From; len(from) > 0 {
			if from[0].Name != "" {
				return "From " + from[0].Name
			}
			return "From " + from[0].Email
		}
		return "You have a new message"
	}
	return fmt.Sprintf("%d new messages", len(msgs))
}

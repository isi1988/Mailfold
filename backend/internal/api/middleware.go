package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/isi1988/Mailfold/backend/internal/auth"
)

// ctxKey is a private type used for keys stored in a request's context.Context.
// Using a dedicated unexported type instead of a bare string prevents key
// collisions with values placed in the context by other packages, which is the
// idiomatic Go safeguard for context storage.
type ctxKey string

// sessionCtxKey is the context key under which the authenticated auth.Session is
// stored by requireAuth so that downstream handlers can retrieve it via
// sessionFrom.
const sessionCtxKey ctxKey = "session"

// requestIDCtxKey is the context key under which the per-request id is stored so
// downstream handlers can include it in their own logs when useful.
const requestIDCtxKey ctxKey = "request_id"

// requestIDHeader is the header used both to accept a caller-supplied request id
// and to echo the effective id back on the response.
const requestIDHeader = "X-Request-Id"

// requireAuth wraps a handler so that only authenticated requests reach it. It
// validates the bearer token on the incoming request; if validation fails the
// request is rejected with 401 Unauthorized and the wrapped handler never runs.
// On success the resolved session is attached to the request context so
// downstream code can identify the caller without re-parsing the token. This is
// the single gate applied to every protected route in the package.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.auth.Validate(bearerToken(r))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		// Store the validated session on the context so handlers (and
		// sessionFrom) can access the caller identity without re-validating.
		ctx := context.WithValue(r.Context(), sessionCtxKey, sess)
		next(w, r.WithContext(ctx))
	}
}

// bearerToken extracts the token from the request's Authorization header. It
// recognises the standard "Bearer " scheme and returns the trimmed token that
// follows it, or an empty string when the header is missing or uses a different
// scheme. Returning empty on absence lets callers treat "no token" and "invalid
// token" uniformly as unauthenticated.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// sessionFrom retrieves the authenticated session that requireAuth stored on the
// request context. It returns nil when no session is present, which happens only
// on routes that were not wrapped in requireAuth; handlers behind requireAuth
// can rely on a non-nil result. Centralising the type assertion here keeps the
// context key private to this file.
func sessionFrom(r *http.Request) *auth.Session {
	sess, _ := r.Context().Value(sessionCtxKey).(*auth.Session)
	return sess
}

// withCommon applies the cross-cutting concerns that every request needs,
// regardless of route: a request id, security headers, CORS, preflight
// handling, a request-body size limit, panic recovery, and request logging. It
// wraps the whole mux so these behaviours are guaranteed and never have to be
// repeated per handler.
func (s *Server) withCommon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assign (or accept) a request id so every log line and the client can
		// correlate a single request end to end.
		reqID := requestID(r)
		w.Header().Set(requestIDHeader, reqID)

		setSecurityHeaders(w)
		s.applyCORS(w, r)

		// A CORS preflight (OPTIONS) is answered immediately with 204 and no
		// body; the headers set above are all the browser needs, so the request
		// must not fall through to the real handler.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Bound the request body to guard against oversized or malicious
		// payloads exhausting memory.
		if s.cfg.MaxBodyBytes > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
		}

		// Recover from any panic in a downstream handler so a single failing
		// request cannot crash the server; the client receives a generic 500
		// while the details are logged for operators.
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered", "id", reqID, "error", rec, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()

		s.logger.Info("request", "id", reqID, "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDCtxKey, reqID)))
	})
}

// requestID returns the incoming request id when the client supplied one via the
// X-Request-Id header, otherwise it generates a fresh random hex id. Reusing a
// caller-provided id lets a reverse proxy or the frontend correlate a request
// across systems.
func requestID(r *http.Request) string {
	if id := r.Header.Get(requestIDHeader); id != "" {
		return id
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail; if it somehow does, fall back to a
		// constant so the request still proceeds (correlation is best-effort).
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// setSecurityHeaders applies a conservative set of security response headers to
// every response: forbid content-type sniffing, forbid framing, withhold the
// Referer header, and isolate the browsing context. Together they reduce the
// attack surface of any UI served alongside the API.
func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
}

// clientIP extracts the client's IP address from RemoteAddr, dropping the port.
// It is the key used to rate-limit login attempts per source address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// applyCORS sets the Access-Control response headers according to the server's
// configured list of allowed origins. It grants access only when the request's
// Origin matches an entry in the allow-list, or when the list contains the
// wildcard "*", and otherwise leaves the headers unset so the browser blocks the
// cross-origin request. This gives the deployment explicit control over which
// front-ends may call the API.
func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	allowed := ""
	// Walk the configured origins to decide what to echo back. A wildcard wins
	// outright; otherwise the request's own Origin is echoed only on an exact,
	// non-empty match so that credentials-bearing requests target a single
	// concrete origin rather than "*".
	for _, o := range s.cfg.CORSOrigins {
		if o == "*" {
			allowed = "*"
			break
		}
		if o == origin && origin != "" {
			allowed = origin
			break
		}
	}
	// No match means the origin is not permitted: emit no CORS headers and let
	// the browser enforce the block.
	if allowed == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", allowed)
	// Vary: Origin tells caches that the response depends on the request Origin,
	// preventing a response allowed for one origin from being served to another.
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
}

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/audit"
)

// auditLogPageSize bounds how many entries a single page request returns, so
// a caller cannot force an unbounded read of the whole table.
const auditLogPageSize = 50

// registerAuditLogRoutes wires the read-only audit-log endpoint. It is
// admin-only (not exposed to domain admins, even for their own actions) since
// it is a whole-instance security record, not a per-domain resource.
func (s *Server) registerAuditLogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audit-log", s.requireAuth(s.handleAuditLogList))
}

// handleAuditLogList returns one page of audit-log entries, newest first.
// ?page= is 1-based; an invalid or missing value defaults to page 1.
func (s *Server) handleAuditLogList(w http.ResponseWriter, r *http.Request) {
	if s.auditStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []audit.Entry{}, "total": 0, "page": 1, "page_size": auditLogPageSize})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	entries, total, err := s.auditStore.List(auditLogPageSize, (page-1)*auditLogPageSize)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":   entries,
		"total":     total,
		"page":      page,
		"page_size": auditLogPageSize,
	})
}

// recordAudit appends one audit-log entry. It is deliberately best-effort: a
// failed write is logged for operators but never fails the request it
// describes, and it is a silent no-op when no database is configured.
func (s *Server) recordAudit(actorType, actor, action string, status int, ip string) {
	if s.auditStore == nil {
		return
	}
	err := s.auditStore.Record(audit.Entry{
		At:        time.Now().UTC(),
		Actor:     actor,
		ActorType: actorType,
		Action:    action,
		Status:    status,
		IP:        ip,
	})
	if err != nil {
		s.logger.Error("failed to write audit log entry", "error", err)
	}
}

// isMutatingMethod reports whether method is one of the state-changing verbs
// recordMutatingAction logs. GETs are excluded: they never change state, and
// logging every read alongside every write would bury the signal an audit
// trail exists for.
func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

// explicitlyAuditedPaths are routes whose handlers already call recordAudit
// themselves with a more specific action label ("login"/"login_failed"/
// "logout") than the generic "METHOD /path" this hook would otherwise use.
// Login itself is never wrapped in requireAuth/requireDomainAdmin (there is no
// session yet), so it never reaches this hook regardless; logout runs behind
// both, so without this exclusion every logout would be recorded twice.
var explicitlyAuditedPaths = map[string]bool{
	"/api/auth/logout":              true,
	"/api/auth/domain-admin/logout": true,
}

// recordMutatingAction is withCommon's generic audit hook: once a request has
// run its full handler chain, if it was a state-changing verb AND some
// downstream auth middleware (requireAuth or requireDomainAdmin) populated
// actor, log it as one entry keyed by "METHOD /path" — unless the route
// already records its own, more specific entry (see explicitlyAuditedPaths).
func (s *Server) recordMutatingAction(r *http.Request, status int, actor *auditActor) {
	if actor == nil || actor.actor == "" || !isMutatingMethod(r.Method) || explicitlyAuditedPaths[r.URL.Path] {
		return
	}
	s.recordAudit(actor.actorType, actor.actor, r.Method+" "+r.URL.Path, status, clientIP(r))
}

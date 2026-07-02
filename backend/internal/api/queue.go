package api

import "net/http"

// registerQueueRoutes wires the HTTP routes that expose mailcow's mail queue.
// It exists so that the queue feature can register its endpoints independently
// from the rest of the server, keeping route wiring modular and discoverable.
//
// Two capabilities are exposed:
//   - Standard CRUD (here only a raw GET passthrough) on "/api/queue", which
//     returns the current queue contents straight from mailcow.
//   - A dedicated "POST /api/queue/flush" action that asks mailcow to flush
//     (attempt immediate redelivery of) every message currently held in the
//     queue.
//
// All routes are guarded by requireAuth so only authenticated callers can read
// or manipulate the queue.
func (s *Server) registerQueueRoutes(mux *http.ServeMux) {
	// Register a read-only endpoint that streams the raw mail queue JSON from
	// mailcow. Only the raw GET is provided because the queue is not created,
	// edited, or deleted through this API; it is only inspected and flushed.
	s.registerCRUD(mux, "/api/queue", crud{raw: s.mc.MailQueue})
	// Register the flush action separately because it is a verb-style operation
	// (trigger a side effect) rather than a resource CRUD verb, and therefore
	// does not fit the registerCRUD create/edit/delete model. runAction invokes
	// FlushQueue and reports the per-item mailcow ActionResults back to the
	// caller.
	mux.HandleFunc("POST /api/queue/flush", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.runAction(w, r, s.mc.FlushQueue)
	}))
}

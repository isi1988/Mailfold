package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// registerDKIMRoutes attaches the DKIM key-management endpoints to mux.
//
// DKIM (DomainKeys Identified Mail) keys are the cryptographic signing keys mailcow
// uses to prove that outgoing mail genuinely originates from a domain. Unlike the
// other resources in this package, DKIM does not offer a full CRUD surface: keys are
// created and deleted but never edited in place, and reading a key is scoped to a
// single domain rather than a flat list. This method therefore registers a reduced
// CRUD set plus one extra handcrafted route:
//   - the crud value only wires create and del (there is no list or edit);
//   - a dedicated "GET /api/dkim/{domain}" route fetches the DKIM record for one
//     domain and streams mailcow's response through verbatim.
func (s *Server) registerDKIMRoutes(mux *http.ServeMux) {
	// Register only the create and delete verbs; DKIM keys are immutable once
	// generated, so no list/edit handlers are wired here.
	s.registerCRUD(mux, "/api/dkim", crud{create: s.mc.AddDKIM, del: s.mc.DeleteDKIM})

	// GET /api/dkim/{domain} returns the DKIM record for a single domain. It is
	// guarded by requireAuth like every other route, and uses serveRaw so mailcow's
	// JSON is passed back to the client untouched (the domain is taken from the URL
	// path segment). serveRaw is used instead of the generic helper because the
	// response is a per-domain record rather than a collection.
	mux.HandleFunc("GET /api/dkim/{domain}", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveRaw(w, r, func(ctx context.Context) (json.RawMessage, error) { return s.mc.DKIM(ctx, r.PathValue("domain")) })
	}))
}

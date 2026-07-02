package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// registerAppPasswordRoutes attaches the application-password endpoints to mux.
//
// An application password is a per-mailbox credential that lets a mail client
// authenticate without the account's primary password, which is how Mailfold
// supports app-specific access that can be revoked in isolation. Because these
// credentials are scoped to a mailbox, the read endpoint is keyed by a mailbox
// path parameter and cannot use the collection-level GET that registerCRUD
// wires; it is registered manually below via serveRaw with r.PathValue.
//
// registerCRUD handles only the collection verbs: create adds a new app
// password and del removes one or more by id. No GET (list/raw), edit, or PUT is
// supplied, so registerCRUD registers just POST and DELETE on the collection
// path. Every handler is protected by requireAuth (applied inside registerCRUD
// and wrapped explicitly around the manual route).
func (s *Server) registerAppPasswordRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/app-passwords", crud{
		create: s.mc.AddAppPassword,    // POST creates a new application password.
		del:    s.mc.DeleteAppPassword, // DELETE removes one or more app passwords.
	})
	// GET is keyed by mailbox, so it is registered outside registerCRUD. The
	// closure adapts s.mc.AppPasswords (which needs the mailbox) to the rawFunc
	// signature serveRaw expects by reading the {mailbox} path segment.
	mux.HandleFunc("GET /api/app-passwords/{mailbox}", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveRaw(w, r, func(ctx context.Context) (json.RawMessage, error) {
			return s.mc.AppPasswords(ctx, r.PathValue("mailbox"))
		})
	}))
}

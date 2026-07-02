package api

import (
	"context"
	"net/http"
)

// registerMailboxRoutes attaches the "/api/mailboxes" CRUD endpoints to mux.
//
// A mailbox is an individual mail account (a real login with a password and a
// physical maildir) that belongs to a domain. This is the resource end users care
// about most, so it gets its own route group registered at server startup. Like the
// other resources in this package, it defines the operations declaratively through a
// crud value whose fields map each HTTP verb to a mailcow client call, keeping the
// permitted mailbox operations obvious and auditable in one spot.
func (s *Server) registerMailboxRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/mailboxes", crud{
		// list returns every mailbox visible to the caller. The closure adapts
		// s.mc.Mailboxes (which takes only a context) to the helper's expected
		// func(context.Context) (any, error) signature.
		list:   func(ctx context.Context) (any, error) { return s.mc.Mailboxes(ctx) },
		create: s.mc.AddMailbox,    // POST provisions a new mailbox.
		edit:   s.mc.EditMailbox,   // PUT/PATCH updates an existing mailbox (quota, password, etc.).
		del:    s.mc.DeleteMailbox, // DELETE removes a mailbox and its stored mail.
	})
}

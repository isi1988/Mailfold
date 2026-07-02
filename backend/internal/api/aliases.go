package api

import (
	"context"
	"net/http"
)

// registerAliasRoutes attaches the "/api/aliases" CRUD endpoints to mux.
//
// An alias is a virtual address that forwards to one or more real mailboxes (or to
// external recipients) without holding any mail of its own. Aliases are managed
// independently of mailboxes, so they get a dedicated route group registered at
// server startup. As with the other resources in this package, the permitted
// operations are declared through a crud value that binds each HTTP verb to a mailcow
// client call, so the alias surface area is visible at a glance.
func (s *Server) registerAliasRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/aliases", crud{
		// list returns every alias visible to the caller. The closure adapts
		// s.mc.Aliases (context-only) to the helper's expected
		// func(context.Context) (any, error) signature.
		list:   func(ctx context.Context) (any, error) { return s.mc.Aliases(ctx) },
		create: s.mc.AddAlias,    // POST creates a new alias/forwarding rule.
		edit:   s.mc.EditAlias,   // PUT/PATCH updates an existing alias's targets or state.
		del:    s.mc.DeleteAlias, // DELETE removes an alias.
	})
}

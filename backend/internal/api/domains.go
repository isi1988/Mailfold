// Package api wires the Mailfold HTTP layer to the underlying mailcow client.
//
// Each resource that Mailfold exposes (domains, mailboxes, aliases, DKIM keys and
// sync jobs) lives in its own file in this package and contributes a single
// route-registration method to the shared *Server. Every one of those methods
// delegates the heavy lifting to the generic CRUD helper (registerCRUD) and to the
// mailcow client (s.mc), so the files here stay intentionally thin: they only
// describe which URL paths map to which mailcow operations. Keeping the mapping in
// one obvious place makes it easy to audit exactly which mailcow endpoints the
// backend is willing to call on behalf of an authenticated user.
package api

import (
	"context"
	"net/http"
)

// registerDomainRoutes attaches the "/api/domains" CRUD endpoints to mux.
//
// Domains are the top-level tenant in mailcow: mailboxes, aliases and DKIM keys all
// hang off a domain, so this route group is registered alongside the others when the
// server boots. It exists as a dedicated method (rather than an inline block in the
// server setup) so the full set of operations permitted on a domain is visible in a
// single place. Each field of the crud value binds one HTTP verb to the matching
// mailcow client call:
//   - list wraps s.mc.Domains so it satisfies the helper's context-only signature;
//   - create/edit/del forward straight to the corresponding mailcow mutations.
func (s *Server) registerDomainRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/domains", crud{
		// list returns every domain visible to the caller. It is wrapped in a
		// closure because s.mc.Domains only takes a context, while the helper
		// expects a func(context.Context) (any, error).
		list:   func(ctx context.Context) (any, error) { return s.mc.Domains(ctx) },
		create: s.mc.AddDomain,    // POST creates a new domain in mailcow.
		edit:   s.mc.EditDomain,   // PUT/PATCH updates attributes of an existing domain.
		del:    s.mc.DeleteDomain, // DELETE removes a domain (and its dependents) from mailcow.
	})
}

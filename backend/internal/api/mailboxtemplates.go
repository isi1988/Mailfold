package api

import "net/http"

// registerMailboxTemplateRoutes attaches the "/api/mailbox-templates" CRUD
// endpoints to mux.
//
// A mailbox template captures default mailbox settings (quota, protocols, spam
// and Sieve options, and similar) that mailcow applies when provisioning a new
// mailbox from it, which lets Mailfold offer consistent presets instead of
// configuring each mailbox individually. The listing uses the raw field because
// s.mc.MailboxTemplates already returns pre-encoded JSON, so it is streamed
// straight through instead of being re-marshalled.
func (s *Server) registerMailboxTemplateRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/mailbox-templates", crud{
		raw:    s.mc.MailboxTemplates,      // GET lists templates; already JSON, streamed verbatim.
		create: s.mc.AddMailboxTemplate,    // POST creates a new mailbox template.
		edit:   s.mc.EditMailboxTemplate,   // PUT updates an existing template.
		del:    s.mc.DeleteMailboxTemplate, // DELETE removes a mailbox template.
	})
}

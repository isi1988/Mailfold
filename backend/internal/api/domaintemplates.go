package api

import "net/http"

// registerDomainTemplateRoutes attaches the "/api/domain-templates" CRUD
// endpoints to mux.
//
// A domain template is a mailcow-managed set of default limits and feature flags
// applied when a new domain is provisioned from it. Its listing uses the raw
// field rather than list because s.mc.DomainTemplates already returns pre-encoded
// JSON, so it is handed to the helper as-is and streamed straight through instead
// of being re-marshalled. The create/edit/del fields bind the remaining verbs to
// the matching mailcow calls.
func (s *Server) registerDomainTemplateRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/domain-templates", crud{
		raw:    s.mc.DomainTemplates,      // GET lists domain templates; already JSON, streamed verbatim.
		create: s.mc.AddDomainTemplate,    // POST creates a new domain template.
		edit:   s.mc.EditDomainTemplate,   // PUT/PATCH updates an existing domain template.
		del:    s.mc.DeleteDomainTemplate, // DELETE removes a domain template.
	})
}

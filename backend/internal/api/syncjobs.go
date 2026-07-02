package api

import "net/http"

// registerSyncJobRoutes attaches the "/api/syncjobs" CRUD endpoints to mux.
//
// A sync job is a mailcow-managed task that periodically pulls mail from a remote
// IMAP account into a local mailbox, which is how Mailfold supports migrating or
// mirroring external accounts. Sync jobs get their own route group registered at
// server startup. This is the one resource whose listing uses the raw field rather
// than list: s.mc.SyncJobs already returns pre-encoded JSON, so it is handed to the
// helper as-is (via raw) and streamed straight through instead of being re-marshalled.
// The create/edit/del fields bind the remaining verbs to the matching mailcow calls.
func (s *Server) registerSyncJobRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/syncjobs", crud{
		raw:    s.mc.SyncJobs,      // GET lists sync jobs; already JSON, streamed verbatim.
		create: s.mc.AddSyncJob,    // POST creates a new IMAP sync job.
		edit:   s.mc.EditSyncJob,   // PUT/PATCH updates an existing sync job's settings.
		del:    s.mc.DeleteSyncJob, // DELETE removes a sync job.
	})
}

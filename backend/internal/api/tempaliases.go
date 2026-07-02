package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// registerTempAliasRoutes wires the time-limited alias endpoints. Listing is
// scoped to a mailbox (path parameter); creation is a collection POST.
func (s *Server) registerTempAliasRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/temp-aliases", crud{create: s.mc.AddTempAlias})
	mux.HandleFunc("GET /api/temp-aliases/{mailbox}", s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		s.serveRaw(w, r, func(ctx context.Context) (json.RawMessage, error) {
			return s.mc.TempAliases(ctx, r.PathValue("mailbox"))
		})
	}))
}

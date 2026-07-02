package api

import "net/http"

// registerRspamdSettingRoutes attaches the "/api/rspamd-settings" CRUD endpoints
// to mux.
//
// An Rspamd setting is a mailcow-managed rule block that overrides the spam
// filter's behaviour for a matched set of messages, which lets Mailfold tune spam
// scoring and whitelisting. The listing uses the raw field because
// s.mc.RspamdSettings already returns pre-encoded JSON, so it is streamed straight
// through instead of being re-marshalled. Rspamd settings have no edit verb in
// mailcow, so only create and del are bound.
func (s *Server) registerRspamdSettingRoutes(mux *http.ServeMux) {
	s.registerCRUD(mux, "/api/rspamd-settings", crud{
		raw:    s.mc.RspamdSettings,      // GET lists Rspamd settings; already JSON, streamed verbatim.
		create: s.mc.AddRspamdSetting,    // POST creates a new Rspamd setting.
		del:    s.mc.DeleteRspamdSetting, // DELETE removes an Rspamd setting.
	})
}

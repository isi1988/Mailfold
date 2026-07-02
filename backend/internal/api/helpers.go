package api

// This file holds the generic request-handling machinery of the api package.
// It turns the small set of mailcow client method shapes (list, raw fetch,
// create, edit, delete) into ready-made http.HandlerFunc plumbing so that each
// concrete resource only has to supply the client callbacks rather than repeat
// request decoding and error handling. The authoritative package-level
// documentation lives alongside the Server type in server.go.

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/isi1988/Mailfold/backend/internal/mailcow"
)

// The following function types describe the exact signatures that the generic
// handlers in this file accept. They deliberately mirror the method signatures
// of the mailcow client so that those methods can be passed directly as
// callbacks without any adapter code. Grouping them in a single type block
// keeps the handler contract in one place and documents how each REST verb maps
// onto a mailcow operation.
type (
	// listFunc fetches a typed value (already unmarshalled into a Go value)
	// from mailcow. It backs GET endpoints that return structured JSON the
	// backend understands and re-encodes.
	listFunc func(context.Context) (any, error)
	// rawFunc fetches an opaque JSON payload from mailcow. It backs GET
	// endpoints that pass the upstream body through verbatim, which avoids
	// having to model every field the backend does not care about.
	rawFunc func(context.Context) (json.RawMessage, error)
	// createFunc performs an "add" operation from a decoded attribute map and
	// returns the mailcow action results. It backs POST endpoints.
	createFunc func(context.Context, any) ([]mailcow.ActionResult, error)
	// editFunc performs an "edit" operation on a set of item identifiers using
	// a decoded attribute map, returning the mailcow action results. It backs
	// PUT endpoints.
	editFunc func(context.Context, []string, any) ([]mailcow.ActionResult, error)
	// deleteFunc performs a "delete" operation on a set of item identifiers and
	// returns the mailcow action results. It backs DELETE endpoints.
	deleteFunc func(context.Context, []string) ([]mailcow.ActionResult, error)
)

// serveJSON runs a typed fetch against mailcow and writes the resulting value
// as a JSON response. It exists so that read handlers returning structured data
// share one code path for upstream error mapping and success encoding. Any
// upstream failure is reported as 502 Bad Gateway because the fault lies with
// the mailcow dependency rather than the client's request.
func (s *Server) serveJSON(w http.ResponseWriter, r *http.Request, fetch listFunc) {
	v, err := fetch(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// serveRaw runs a raw-JSON fetch against mailcow and streams the upstream body
// straight to the client. It exists for endpoints where re-modelling the
// mailcow payload would add no value, so the untouched bytes are forwarded. As
// with serveJSON, an upstream error becomes a 502 Bad Gateway.
func (s *Server) serveRaw(w http.ResponseWriter, r *http.Request, fetch rawFunc) {
	raw, err := fetch(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	writeUpstream(w, raw)
}

// serveCreate decodes the request body into a free-form attribute map and runs
// the supplied add operation. The attribute map is intentionally untyped so a
// single helper can service every resource: mailcow accepts arbitrary
// attributes, and validation happens upstream. A malformed body is a client
// error (400), whereas failures from the add operation are handled by runAction.
func (s *Server) serveCreate(w http.ResponseWriter, r *http.Request, add createFunc) {
	var attr map[string]any
	if err := decodeJSON(r, &attr); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.runAction(w, r, func(ctx context.Context) ([]mailcow.ActionResult, error) { return add(ctx, attr) })
}

// serveEdit decodes an editRequest body of the form {items, attr} and runs the
// supplied edit operation, applying the attribute changes to every listed item.
// A malformed body is reported as a 400; operation failures flow through
// runAction. Keeping the {items, attr} shape here means every editable resource
// shares one wire contract.
func (s *Server) serveEdit(w http.ResponseWriter, r *http.Request, edit editFunc) {
	var req editRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.runAction(w, r, func(ctx context.Context) ([]mailcow.ActionResult, error) { return edit(ctx, req.Items, req.Attr) })
}

// serveDelete decodes an itemsRequest body of the form {items} and runs the
// supplied delete operation against those identifiers. A malformed body is a
// 400; operation failures flow through runAction. The shared {items} shape lets
// every deletable resource reuse the same contract.
func (s *Server) serveDelete(w http.ResponseWriter, r *http.Request, del deleteFunc) {
	var req itemsRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.runAction(w, r, func(ctx context.Context) ([]mailcow.ActionResult, error) { return del(ctx, req.Items) })
}

// runAction executes a mailcow mutation and writes its action-result array to
// the response. It centralises the mutation path shared by create, edit, and
// delete: a transport-level error (the call never reached a verdict) becomes a
// 502 Bad Gateway, while a successfully returned result array is handed to
// writeMailcowResults, which decides success versus per-item failure.
func (s *Server) runAction(w http.ResponseWriter, r *http.Request, action func(context.Context) ([]mailcow.ActionResult, error)) {
	results, err := action(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeMailcowResults(w, results)
}

// crud describes the optional REST operations available for a single resource
// path. Each field is a callback for one verb, and a nil field means that verb
// is simply not registered. For GET, a resource supplies either list (a typed
// value the backend re-encodes) or raw (an upstream passthrough), never both in
// a way that matters — registerCRUD prefers list when both are present. Bundling
// the operations in one struct lets registerCRUD wire a whole resource in a
// single call and keeps route registration declarative.
type crud struct {
	// list serves GET as a typed, re-encoded value.
	list listFunc
	// raw serves GET as a verbatim upstream passthrough, used only when list
	// is nil.
	raw rawFunc
	// create serves POST as an add operation.
	create createFunc
	// edit serves PUT as an edit operation.
	edit editFunc
	// del serves DELETE as a delete operation.
	del deleteFunc
}

// registerCRUD wires the standard REST verbs for a resource path onto the mux,
// registering only the verbs whose callbacks are present in c. Every registered
// handler is wrapped in requireAuth so the entire CRUD surface is
// authenticated. It exists so a resource can be exposed with one declarative
// call instead of repeating identical HandleFunc boilerplate per verb.
func (s *Server) registerCRUD(mux *http.ServeMux, path string, c crud) {
	// GET is registered from whichever read callback is supplied. list takes
	// precedence over raw so a typed representation wins when both exist; if
	// neither is set, no GET handler is registered for this path.
	switch {
	case c.list != nil:
		mux.HandleFunc("GET "+path, s.requireAuth(func(w http.ResponseWriter, r *http.Request) { s.serveJSON(w, r, c.list) }))
	case c.raw != nil:
		mux.HandleFunc("GET "+path, s.requireAuth(func(w http.ResponseWriter, r *http.Request) { s.serveRaw(w, r, c.raw) }))
	}
	// Each mutating verb is registered independently so a resource can be, for
	// example, creatable but not deletable simply by leaving a callback nil.
	if c.create != nil {
		mux.HandleFunc("POST "+path, s.requireAuth(func(w http.ResponseWriter, r *http.Request) { s.serveCreate(w, r, c.create) }))
	}
	if c.edit != nil {
		mux.HandleFunc("PUT "+path, s.requireAuth(func(w http.ResponseWriter, r *http.Request) { s.serveEdit(w, r, c.edit) }))
	}
	if c.del != nil {
		mux.HandleFunc("DELETE "+path, s.requireAuth(func(w http.ResponseWriter, r *http.Request) { s.serveDelete(w, r, c.del) }))
	}
}

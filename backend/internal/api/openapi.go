package api

import (
	_ "embed"
	"net/http"
)

// openAPISpec is the OpenAPI 3 description of the Mailfold API, embedded at build
// time so the binary can serve its own documentation with no external files.
//
//go:embed openapi.yaml
var openAPISpec []byte

// swaggerHTML is a minimal Swagger UI page that renders the embedded spec. The
// Swagger UI assets are loaded from a public CDN to keep the binary small; the
// page itself only points the viewer at /api/openapi.yaml.
const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Mailfold API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: '/api/openapi.yaml', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`

// registerDocsRoutes wires the API documentation endpoints. They are public (no
// authentication) so the API contract can be inspected without credentials; the
// spec describes only the shape of the API, not any secret data.
func (s *Server) registerDocsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/openapi.yaml", s.handleOpenAPISpec)
	mux.HandleFunc("GET /api/docs", s.handleDocs)
}

// handleOpenAPISpec serves the raw embedded OpenAPI document.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(openAPISpec)
}

// handleDocs serves the Swagger UI page that renders the spec.
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTML))
}

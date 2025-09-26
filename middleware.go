package httpx

import "net/http"

// Middleware defines the interface for HTTP middleware compatible with ServeMux.
type Middleware interface {
	Handler(http.Handler) http.Handler
}

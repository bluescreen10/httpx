package httpx

import "net/http"

// Middleware defines the interface for HTTP middleware compatible with ServeMux.
type Middleware func(http.Handler) http.Handler

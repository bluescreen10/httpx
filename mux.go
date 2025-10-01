package httpx

import (
	"net/http"
	"strings"
)

// ServeMux is a wrapper around http.ServeMux that adds support for
// route grouping and applying middlewares with syntax sugar.
//
// Usage:
//
//	mux := httpx.NewServeMux()
//
//	// Global middleware for all routes
//	mux.Use(httpx.Logger())
//
//	// Route group with prefix "/api" and additional middleware
//	api := mux.Group("/api", authMiddleware)
//
//	// Register handlers on the group
//	api.HandleFunc("/users", usersHandler)
//	api.HandleFunc("/posts", postsHandler)
//
//	http.ListenAndServe(":8080", mux)
type ServeMux struct {
	*http.ServeMux
	handler http.Handler
}

// NewServeMux creates a new ServeMux instance.
func NewServeMux() *ServeMux {
	mux := new(ServeMux)
	mux.ServeMux = http.NewServeMux()
	mux.handler = mux.ServeMux
	return mux
}

// Group creates a sub-router with the given prefix and optional middlewares.
// The returned sub-router can register its own handlers, which will inherit
// the parent middlewares automatically.
func (mux *ServeMux) Group(prefix string, middlewares ...Middleware) *ServeMux {
	prefix = strings.TrimSuffix(prefix, "/")
	subMux := NewServeMux()

	var wrapped http.Handler = subMux

	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}

	mux.Handle(prefix+"/", http.StripPrefix(prefix, wrapped))
	return subMux
}

// Use adds a global middleware to the ServeMux. These middlewares are applied
// to all routes registered on this mux.
func (mux *ServeMux) Use(middleware Middleware) {
	mux.handler = middleware(mux.handler)
}

// ServeHTTP implements http.Handler and applies global middlewares
// before dispatching to the underlying http.ServeMux.
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux.handler.ServeHTTP(w, r)
}

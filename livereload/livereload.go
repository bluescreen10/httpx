// Package livereload provides an HTTP middleware that enables live reloading
// of web pages by injecting a small JavaScript snippet into HTML responses.
// When the server restarts, connected clients automatically refresh their page.
//
// This is particularly useful during development, as it avoids manual
// browser refreshes when making code or template changes.
//
// Usage:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("<html><body><h1>Hello, world!</h1></body></html>"))
//	})
//
//	// Create a new LiveReload middleware (optional custom path)
//	lr := livereload.New() // or livereload.New(livereload.WithPath("/custom_reload"))
//
//	// Wrap the mux with the middleware
//	handler := lr.Handler(mux)
//
//	http.ListenAndServe(":8080", handler)
//
// Only responses with "Content-Type: text/html" and a closing </body>
// tag will be modified to inject the script. Non-HTML responses pass
// through unmodified. Client communication is done via Server-Sent Events (SSE).
package livereload

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type responseWriter struct {
	w          http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseWriter) Header() http.Header {
	return r.w.Header()
}

func (r *responseWriter) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// script contains the embedded JavaScript code that will be injected
// into HTML pages to enable live reload functionality.
//
//go:embed reload.js
var script []byte

// LiveReload is a middleware that injects a reload script into HTML pages
// and serves an SSE (Server-Sent Events) endpoint used by the client-side
// script to detect when a reload is needed.
type LiveReload struct {
	path string
}

const defaultPath = "/_livereload"

type config func(*LiveReload)

// WithPath customizes the reload endpoint path (default "/_livereload").
// Useful to avoid collisions with existing routes or when integrating
// with a custom routing scheme.
func WithPath(path string) config {
	return config(func(l *LiveReload) {
		l.path = path
	})
}

// Handler wraps the given http.Handler with the LiveReload middleware.
// It injects the reload JavaScript snippet into HTML responses that contain
// a closing </body> tag and serves the EventSource endpoint at the configured path.
func (lr *LiveReload) Handler(next http.Handler) http.Handler {
	js := bytes.ReplaceAll(script, []byte(defaultPath), []byte(lr.path))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == lr.path {
			handleClientConn(w, r)
			return
		}

		rw := responseWriter{
			w:    w,
			body: &bytes.Buffer{},
		}

		next.ServeHTTP(&rw, r)

		contentType := rw.Header().Get("Content-Type")
		body := rw.body.Bytes()
		closingBodyAt := bytes.Index(body, []byte("</body>"))

		if strings.Contains(contentType, "text/html") && closingBodyAt != -1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)+len(js)))
			if rw.statusCode != 0 {
				w.WriteHeader(rw.statusCode)
			}
			w.Write(body[:closingBodyAt])
			w.Write(js)
			w.Write(body[closingBodyAt:])
		} else {
			if rw.statusCode != 0 {
				w.WriteHeader(rw.statusCode)
			}
			w.Write(body)
		}
	})
}

func handleClientConn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	fmt.Fprintf(w, "data: ts=%d\n\n", time.Now().UnixNano())
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	notify := r.Context().Done()

	// keep connection open until client disconnects
	<-notify
}

// New creates a new LiveReload middleware instance.
// By default, it serves the reload endpoint at "/_livereload",
// but this can be customized by passing configuration options such as WithPath.
func New(cfgs ...config) *LiveReload {
	lr := &LiveReload{path: defaultPath}

	for _, cfg := range cfgs {
		cfg(lr)
	}

	return lr
}

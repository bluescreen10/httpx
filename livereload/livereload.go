// Package livereload provides HTTP middleware for automatically reloading web pages
// during development. It injects a client-side script into HTML responses that
// connects to a Server-Sent Events (SSE) endpoint for real-time reload notifications.
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

// DefaultConfig provides a default configuration for the livereload middleware
// with the SSE endpoint set to "/_livereload".
var DefaultConfig = Config{
	Path: "/_livereload",
}

// Config holds the configuration options for the livereload middleware.
type Config struct {
	// Path specifies the URL path for the Server-Sent Events endpoint
	// that clients will connect to for reload notifications.
	Path string
}

// responseWriter is a wrapper around http.ResponseWriter that captures
// the response body and status code for modification before sending
// to the client.
type responseWriter struct {
	w          http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// Header returns the header map that will be sent by WriteHeader.
// It delegates to the underlying http.ResponseWriter.
func (r *responseWriter) Header() http.Header {
	return r.w.Header()
}

// Write writes the data to the internal buffer instead of directly
// to the client, allowing the middleware to modify the response.
func (r *responseWriter) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

// WriteHeader captures the status code instead of writing it immediately,
// allowing the middleware to modify headers before sending the response.
func (r *responseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// script contains the embedded JavaScript code that will be injected
// into HTML pages to enable live reload functionality.
//
//go:embed reload.js
var script []byte

// Wrap wraps an http.Handler with livereload functionality. It intercepts
// HTML responses and injects a client-side script that connects to the
// configured SSE endpoint for reload notifications.
//
// The middleware performs the following actions:
//   - Routes requests to the configured SSE path to handleClientConn
//   - For HTML responses, injects the reload script before the closing </body> tag
//   - Updates the Content-Length header to account for the injected script
//   - Passes through non-HTML responses unchanged
//
// Example usage:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", homeHandler)
//
//	// Wrap with livereload middleware
//	server := livereload.Wrap(mux, livereload.DefaultConfig)
//	http.ListenAndServe(":8080", server)
func Wrap(next http.Handler, config Config) http.Handler {
	js := bytes.ReplaceAll(script, []byte("/_livereload"), []byte(config.Path))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == config.Path {
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
			contentLength := len(body) + len(js)
			rw.Header().Set("Content-Length", strconv.Itoa(contentLength))
			rw.w.WriteHeader(rw.statusCode)
			rw.w.Write(body[:closingBodyAt])
			rw.w.Write(js)
			rw.w.Write(body[closingBodyAt:])
		} else {
			rw.w.WriteHeader(rw.statusCode)
			rw.w.Write(body)
		}

	})
}

// handleClientConn handles Server-Sent Events connections from clients.
// It establishes an SSE connection by setting appropriate headers and
// sends an initial timestamp message. The connection remains open until
// the client disconnects or the request context is cancelled.
//
// The function sets the following SSE headers:
//   - Content-Type: text/event-stream
//   - Cache-Control: no-cache
//   - Connection: keep-alive
//   - Access-Control-Allow-Origin: * (for CORS support)
//   - Access-Control-Allow-Headers: Cache-Control
//
// An initial message with the current timestamp is sent in the format:
// "data: ts=<nanosecond_timestamp>\n\n"
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
	<-notify
}

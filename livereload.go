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
//	lr := httpx.LiveReload() // or httpx.LiveReloadWithConfig(LiveReloadConfig{...})
//
//	// Wrap the mux with the middleware
//
//	http.ListenAndServe(":8080", lr(mux))
//
// Only responses with "Content-Type: text/html" and a closing </body>
// tag will be modified to inject the script. Non-HTML responses pass
// through unmodified. Client communication is done via Server-Sent Events (SSE).
package httpx

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// script contains the embedded JavaScript code that will be injected
// into HTML pages to enable live reload functionality.
//
//go:embed reload.js
var script []byte

const defaultLiveReloadPath = "/_livereload"

// LiveReloadConfig is the optional configuration for live reload
type LiveReloadConfig struct {
	// Path sets the path to be used for SSE
	Path string

	// Reload can be used to force the client to reload. This can
	// be used to force the client to reload when assets change
	Reload <-chan struct{}
}

var DefaultLiveReloadConfig = LiveReloadConfig{
	Path: defaultLiveReloadPath,
}

// LiveReload retuns a middleware that will inject a small script on the
// page. This script will automatically reload the page if the server sends
// an event, or if it gets restarted.
func LiveReload() Middleware {
	return LiveReloadWithConfig(DefaultLiveReloadConfig)
}

// LiveReloadWithConfig returns a LiveReload middleware with the specified
// configuration.
func LiveReloadWithConfig(cfg LiveReloadConfig) Middleware {
	js := bytes.ReplaceAll(script, []byte(defaultLiveReloadPath), []byte(cfg.Path))

	if cfg.Reload == nil {
		cfg.Reload = make(chan struct{})
	}

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == cfg.Path {
				handleClientConn(w, r, cfg.Reload)
				return
			}

			buf := &bytes.Buffer{}
			rw := newResponseWriter(buf, w.Header(), nil)
			next.ServeHTTP(rw, r)

			body := buf.Bytes()
			closingBodyAt := bytes.Index(body, []byte("</body>"))
			contentType := rw.Header().Get("Content-Type")
			validContentType := contentType == "" || strings.Contains(contentType, "text/html")

			if validContentType && closingBodyAt != -1 {
				w.Header().Set("Content-Length", strconv.Itoa(len(body)+len(js)))
				if rw.status != 0 {
					w.WriteHeader(rw.status)
				}
				w.Write(body[:closingBodyAt])
				w.Write(js)
				w.Write(body[closingBodyAt:])
			} else {
				if rw.status != 0 {
					w.WriteHeader(rw.status)
				}
				w.Write(body)
			}
		})
	}
}

func handleClientConn(w http.ResponseWriter, r *http.Request, reload <-chan struct{}) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// send timestamp, the client reloads when the timestamp
	// changes. The first time the client does not do a
	// reload
	sendReload(w)

	select {
	// client closed conection
	case <-r.Context().Done():
		return

	// send new timestamp
	case <-reload:
		sendReload(w)
	}
}

func sendReload(w http.ResponseWriter) {
	fmt.Fprintf(w, "data: ts=%d\n\n", time.Now().UnixNano())
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

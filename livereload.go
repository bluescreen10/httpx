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
//	lr := httpx.NewLiveReload()
//
//	// Wrap the mux with the middleware
//
//	http.ListenAndServe(":8080", lr.Handler(mux))
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
	"sync"
	"time"
)

// script contains the embedded JavaScript code that will be injected
// into HTML pages to enable live reload functionality.
//
//go:embed reload.js
var script []byte

const defaultLiveReloadPath = "/_livereload"

// LiveReloadConfig is the optional configuration for live reload
type LiveReload struct {
	// Path sets the path to be used for SSE
	path string

	subscribers []chan (struct{})
	mu          sync.RWMutex
}

// LiveReload retuns a middleware that will inject a small script on the
// page. This script will automatically reload the page if the server sends
// an event, or if it gets restarted.
func NewLiveReload() *LiveReload {
	return &LiveReload{path: "/_livereload"}
}

// SetPath allows changing the path used for the javascript library to receive
// Server-Side Events (default: "/_livereload").
func (lr *LiveReload) SetPath(path string) {
	lr.path = path
}

// LiveReloadWithConfig returns a LiveReload middleware with the specified
// configuration.
func (lr *LiveReload) Handler(next http.Handler) http.Handler {
	js := bytes.ReplaceAll(script, []byte(defaultLiveReloadPath), []byte(lr.path))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == lr.path {
			lr.handleClientConn(w, r)
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

// Reload will trigger a reload of the current page in the browser.
// This can be used in combination with file watcher to force a page
// reload.
func (lr *LiveReload) Reload() {
	//notify subscribers
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	for _, ch := range lr.subscribers {
		ch <- struct{}{}
	}
}

func (lr *LiveReload) subscribe() chan (struct{}) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	ch := make(chan (struct{}))
	lr.subscribers = append(lr.subscribers, ch)
	return ch
}

func (lr *LiveReload) unsubscribe(ch chan (struct{})) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	defer close(ch)

	for i, subscriber := range lr.subscribers {
		if subscriber == ch {
			lr.subscribers = append(lr.subscribers[:i], lr.subscribers[i+1:]...)
			break
		}
	}
}

func (lr *LiveReload) handleClientConn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	reloadCh := lr.subscribe()
	defer lr.unsubscribe(reloadCh)

	// send timestamp, the client reloads when the timestamp
	// changes. The first time the client does not do a
	// reload
	sendReload(w)

	select {
	// client closed conection
	case <-r.Context().Done():
		return

	// send new timestamp
	case <-reloadCh:
		sendReload(w)
	}
}

func sendReload(w http.ResponseWriter) {
	fmt.Fprintf(w, "data: ts=%d\n\n", time.Now().UnixNano())
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

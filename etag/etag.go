// Package etag provides an HTTP middleware that calculates and sets
// ETag headers for GET requests. It can optionally use a cache to
// avoid recalculating ETags and supports weak ETags.
//
// This middleware allows clients to make conditional requests using
// the If-None-Match header. When the content has not changed, the
// middleware responds with HTTP 304 Not Modified, saving bandwidth
// and improving performance.
//
// Usage:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("Hello, world!"))
//	})
//
//	// Create ETag middleware (weak ETags and caching enabled)
//	et := etag.New(etag.WithWeak(true), etag.WithCache(true))
//
//	// Wrap the mux with the middleware
//	handler := et.Handler(mux)
//
//	http.ListenAndServe(":8080", handler)
//
// Only GET requests are supported. Responses for other HTTP methods
// are passed through unmodified.
package etag

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"net/http"
	"sync"
)

// responseWriter is an internal type that captures the response body
// and calculates the CRC64 checksum used to generate the ETag.
type responseWriter struct {
	http.ResponseWriter
	buffer     *bytes.Buffer
	checksum   uint64
	table      *crc64.Table
	statusCode int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.checksum = crc64.Update(w.checksum, w.table, b)
	return w.buffer.Write(b)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// ETag is a middleware that calculates ETag headers for GET requests.
// It optionally caches ETags and supports weak ETags.
type ETag struct {
	cache    sync.Map
	useCache bool
	isWeak   bool
}

type config func(*ETag)

// WithWeak configures whether the ETag should be weak (prefixed with W/).
func WithWeak(isWeak bool) config {
	return config(func(e *ETag) {
		e.isWeak = isWeak
	})
}

// WithCache enables or disables caching of ETags.
func WithCache(useCache bool) config {
	return config(func(e *ETag) {
		e.useCache = useCache
	})
}

// Handler wraps the given http.Handler with ETag functionality.
// For GET requests, it calculates an ETag based on the response body
// and sets the ETag header. If the client sends If-None-Match matching
// the ETag, a 304 Not Modified is returned.
func (e *ETag) Handler(next http.Handler) http.HandlerFunc {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For now only GET supported
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		uri := r.URL.RequestURI()
		cachedEtag, ok := e.cache.Load(uri)
		clientEtag := r.Header.Get("If-None-Match")

		if e.useCache && ok && clientEtag == cachedEtag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		rw := responseWriter{w, &bytes.Buffer{}, 0, crc64.MakeTable(crc64.ECMA), 0}
		next.ServeHTTP(&rw, r)

		var etag string
		if e.isWeak {
			etag = fmt.Sprintf("W/%x", rw.checksum)
		} else {
			etag = fmt.Sprintf("%x", rw.checksum)
		}

		responseEtag := rw.ResponseWriter.Header().Get("Etag")

		if (rw.statusCode == 0 || rw.statusCode == http.StatusOK) && responseEtag == "" {
			if clientEtag == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			if e.useCache {
				e.cache.Store(uri, etag)
			}

			w.Header().Set("Etag", etag)
		}

		if rw.statusCode != 0 {
			w.WriteHeader(rw.statusCode)
		}

		w.Write(rw.buffer.Bytes())
	})
}

// New creates a new ETag middleware instance, optionally applying
// configuration options such as WithWeak or WithCache.
func New(cfgs ...config) *ETag {
	etag := &ETag{}

	for _, cfg := range cfgs {
		cfg(etag)
	}

	return etag
}

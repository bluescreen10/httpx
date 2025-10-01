// ETag provides an HTTP middleware that calculates and sets
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
//	etag := httpx.Etag()
//
//	http.ListenAndServe(":8080", etag(handler))
//
// Only GET requests are supported. Responses for other HTTP methods
// are passed through unmodified.
package httpx

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"net/http"
	"sync"
)

// ETag Configuration
type ETagConfig struct {
	// Uses a cache to store ETag values for a given URL. This
	// prevents recomputing the ETag for every request.
	UseCache bool

	// Uses the prefix "W/" in the ETag header
	IsWeak bool
}

var DefaultETagConfig = ETagConfig{}

// ETag returs a middleware with the default configuration that set and checks
// ETags headers. For GET requests, it calculates an ETag based on the response
// body and sets the ETag header. If the client sends If-None-Match matching
// the ETag, a 304 Not Modified is returned.
func ETag() Middleware {
	return ETagWithConfig(DefaultETagConfig)
}

// ETagWithConfig returs am ETag middleware with the specified configuration.
func ETagWithConfig(cfg ETagConfig) Middleware {
	return func(next http.Handler) http.Handler {
		var cache sync.Map

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For now only GET supported
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			uri := r.URL.RequestURI()
			cachedEtag, ok := cache.Load(uri)
			clientEtag := r.Header.Get("If-None-Match")

			if cfg.UseCache && ok && clientEtag == cachedEtag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			buf := &bytes.Buffer{}
			table := crc64.MakeTable(crc64.ECMA)
			header := w.Header()
			rw := newResponseWriter(buf, header, nil)
			next.ServeHTTP(rw, r)

			checksum := crc64.Update(0, table, buf.Bytes())

			var etag string
			if cfg.IsWeak {
				etag = fmt.Sprintf("W/%x", checksum)
			} else {
				etag = fmt.Sprintf("%x", checksum)
			}

			responseEtag := header.Get("Etag")

			if (rw.status == 0 || rw.status == http.StatusOK) && responseEtag == "" {
				if clientEtag == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}

				if cfg.UseCache {
					cache.Store(uri, etag)
				}

				w.Header().Set("Etag", etag)
			}

			if rw.status != 0 {
				w.WriteHeader(rw.status)
			}

			w.Write(buf.Bytes())
		})
	}
}

package httpx

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"net/http"
	"sync"
)

// ETagConfig configures the behavior of the ETag middleware.
type ETagConfig struct {
	// Weak indicates whether to generate weak ETags (prefixed with W/).
	// Weak ETags are sufficient for cache validation but not for byte-for-byte equality.
	Weak bool

	// Cache enables in-memory caching of generated ETags by request URI.
	// If disabled, ETags are computed on every request but not cached.
	Cache bool
}

// DefaultETagConfig provides strong ETags and caching enabled.
var DefaultETagConfig = ETagConfig{
	Weak:  false,
	Cache: true,
}

// etagResponseWriter intercepts writes in order to compute a CRC64 checksum
// over the response body, which is later used to generate the ETag header.
type etagResponseWriter struct {
	http.ResponseWriter
	buffer     *bytes.Buffer
	checksum   uint64
	table      *crc64.Table
	statusCode int
}

func (w *etagResponseWriter) Write(b []byte) (int, error) {
	w.checksum = crc64.Update(w.checksum, w.table, b)
	return w.buffer.Write(b)
}

func (w *etagResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// ETag is a middleware that automatically generates and manages ETag headers
// for GET responses. It computes a CRC64 checksum of the response body and
// uses it as the ETag value.
//
// If the client sends an If-None-Match header that matches the computed ETag,
// the middleware responds with 304 Not Modified and suppresses the response body.
//
// Behavior is configurable via ETagConfig:
//   - Weak: if true, generates weak ETags (W/"...").
//   - Cache: if true, caches ETags in memory by request URI.
//
// Example:
//
//	mux := http.NewServeMux()
//	mux.Handle("/data", httpx.ETag(http.HandlerFunc(dataHandler), httpx.DefaultETagConfig))
func ETag(next http.Handler, config ETagConfig) http.HandlerFunc {
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

		if config.Cache && ok && clientEtag == cachedEtag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		rw := etagResponseWriter{w, &bytes.Buffer{}, 0, crc64.MakeTable(crc64.ECMA), 0}
		next.ServeHTTP(&rw, r)

		var etag string
		if config.Weak {
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

			if config.Cache {
				cache.Store(uri, etag)
			}

			w.Header().Set("Etag", etag)
		}

		if rw.statusCode != 0 {
			w.WriteHeader(rw.statusCode)
		}

		w.Write(rw.buffer.Bytes())
	})
}

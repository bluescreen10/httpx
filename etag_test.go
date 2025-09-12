package httpx_test

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluescreen10/httpx"
)

func TestGenerateETag(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	expectedEtag := fmt.Sprintf("%x", crc)

	helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	handler := httpx.ETag(helloHandler, httpx.DefaultETagConfig)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if etag := w.Header().Get("ETag"); etag != expectedEtag {
		t.Fatalf("ETag expected '%s' header but got '%s'", expectedEtag, etag)
	}
}

func TestNotModified(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	etag := fmt.Sprintf("%x", crc)

	helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	handler := httpx.ETag(helloHandler, httpx.DefaultETagConfig)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	r.Header.Set("If-None-Match", etag)

	handler.ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusNotModified {
		t.Fatalf("expected status 304 Not modifed but got %d", w.Result().StatusCode)
	}
}

func TestEtagCache(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	etag := fmt.Sprintf("%x", crc)

	count := 0
	helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count < 1 {
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		} else {
			t.Fatalf("this should not be called")
		}
	})

	handler := httpx.ETag(helloHandler, httpx.DefaultETagConfig)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})

	// this calls hello handler and generates the cache
	handler.ServeHTTP(w, r)

	// this should come from the cache and not invoke the hello handler
	r.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(w, r)
}

func TestGenerateWeakETag(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	expectedEtag := fmt.Sprintf("W/%x", crc)

	helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	handler := httpx.ETag(helloHandler, httpx.ETagConfig{Weak: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if etag := w.Header().Get("ETag"); etag != expectedEtag {
		t.Fatalf("ETag expected '%s' header but got '%s'", expectedEtag, etag)
	}
}

func TestGenerateSkipETag(t *testing.T) {
	body := []byte("hello world")

	helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write(body)
	})

	handler := httpx.ETag(helloHandler, httpx.DefaultETagConfig)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if etag := w.Header().Get("ETag"); etag != "" {
		t.Fatalf("ETag expected '' header but got '%s'", etag)
	}
}

func TestGenerateSkipETagOnMethod(t *testing.T) {
	body := []byte("hello world")

	helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	handler := httpx.ETag(helloHandler, httpx.DefaultETagConfig)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if etag := w.Header().Get("ETag"); etag != "" {
		t.Fatalf("ETag expected '' header but got '%s'", etag)
	}
}

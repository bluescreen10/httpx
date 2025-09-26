package etag_test

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluescreen10/httpx/etag"
)

func TestGenerateETag(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	expectedEtag := fmt.Sprintf("%x", crc)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	mw := etag.New()
	handler := mw.Handler(h)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if got := w.Header().Get("ETag"); got != expectedEtag {
		t.Fatalf("ETag expected '%s' header but got '%s'", expectedEtag, got)
	}
}

func TestNotModified(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	reqEtag := fmt.Sprintf("%x", crc)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	mw := etag.New()
	handler := mw.Handler(h)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	r.Header.Set("If-None-Match", reqEtag)

	handler.ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusNotModified {
		t.Fatalf("expected status 304 Not modifed but got %d", w.Result().StatusCode)
	}
}

func TestEtagCache(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	reqEtag := fmt.Sprintf("%x", crc)

	count := 0
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count < 1 {
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		} else {
			t.Fatalf("this should not be called")
		}
	})

	mw := etag.New()
	handler := mw.Handler(h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})

	// this calls hello handler and generates the cache
	handler.ServeHTTP(w, r)

	// this should come from the cache and not invoke the hello handler
	r.Header.Set("If-None-Match", reqEtag)
	handler.ServeHTTP(w, r)
}

func TestGenerateWeakETag(t *testing.T) {
	body := []byte("hello world")
	crc := crc64.Checksum(body, crc64.MakeTable(crc64.ECMA))
	expectedEtag := fmt.Sprintf("W/%x", crc)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	mw := etag.New(etag.WithWeak(true))
	handler := mw.Handler(h)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if etag := w.Header().Get("ETag"); etag != expectedEtag {
		t.Fatalf("ETag expected '%s' header but got '%s'", expectedEtag, etag)
	}
}

func TestGenerateSkipETag(t *testing.T) {
	body := []byte("hello world")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write(body)
	})

	mw := etag.New(etag.WithWeak(true))
	handler := mw.Handler(h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if got := w.Header().Get("ETag"); got != "" {
		t.Fatalf("ETag expected '' header but got '%s'", got)
	}
}

func TestGenerateSkipETagOnMethod(t *testing.T) {
	body := []byte("hello world")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	mw := etag.New(etag.WithWeak(true))
	handler := mw.Handler(h)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", &bytes.Buffer{})
	handler.ServeHTTP(w, r)

	if got := w.Header().Get("ETag"); got != "" {
		t.Fatalf("ETag expected '' header but got '%s'", got)
	}
}

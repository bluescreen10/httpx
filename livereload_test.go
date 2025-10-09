package httpx_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluescreen10/httpx"
)

func TestLiveReloadInjection(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprintf(w, "<html><body><h1>Hello</h1></body></html>")
	})
	lr := httpx.NewLiveReload()
	ts := httptest.NewServer(lr.Handler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "<script>") {
		t.Fatal("injection not present")
	}
}

func TestLiveReloadInjectionWithoutWriteHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><h1>Hello</h1></body></html>")
	})
	lr := httpx.NewLiveReload()
	ts := httptest.NewServer(lr.Handler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "<script>") {
		t.Fatal("injection not present")
	}
}

func TestLiveReloadSkipInjection(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, "{\"dummy\": \"\"}")
	})

	lr := httpx.NewLiveReload()
	ts := httptest.NewServer(lr.Handler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(body), "<script>") {
		t.Fatal("injection present for application/json")
	}
}

func TestLiveReloadSkipInjectionPartial(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprintf(w, "<h1>Hello</h1>")
	})

	lr := httpx.NewLiveReload()
	ts := httptest.NewServer(lr.Handler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(body), "<script>") {
		t.Fatal("injection present for application/json")
	}
}

func TestLiveReloadInjectionWithConfig(t *testing.T) {
	path := "/my-live-reload"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		fmt.Fprintf(w, "<html><body><h1>Hello</h1></body></html>")
	})
	lr := httpx.NewLiveReload()
	lr.SetPath(path)
	ts := httptest.NewServer(lr.Handler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "<script>") && !strings.Contains(string(body), path) {
		t.Fatal("injection not present")
	}
}

func TestLiveReloadSSE(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	lr := httpx.NewLiveReload()
	s := lr.Handler(dummyHandler)

	ctx, cancel := context.WithCancel(context.Background())

	r := httptest.NewRequest("GET", "/_livereload", &bytes.Buffer{}).WithContext(ctx)
	w := httptest.NewRecorder()
	cancel()
	s.ServeHTTP(w, r)

	res := w.Result()

	if res.Header.Get("Content-Type") != "text/event-stream" &&
		res.Header.Get("CaChe-Control") != "no-cache" &&
		res.Header.Get("Connection") != "keep-alive" {
		t.Fatal("headers not set correctly")
	}

	if !strings.Contains((w.Body.String()), "data: ts=") {
		t.Fatal("missing event data")
	}
}

func TestLiveReloadStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	lr := httpx.NewLiveReload()
	ts := httptest.NewServer(lr.Handler(handler))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusNotModified {
		t.Fatalf("expecetd status code '%d' got '%d'", http.StatusNotModified, res.StatusCode)
	}
}

func TestLiveReloadSSEReload(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	lr := httpx.NewLiveReload()
	s := lr.Handler(dummyHandler)

	ctx, cancel := context.WithCancel(context.Background())

	r := httptest.NewRequest("GET", "/_livereload", &bytes.Buffer{}).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		s.ServeHTTP(w, r)
		<-done
	}()

	defer cancel()
	defer func() { done <- struct{}{} }()

	time.Sleep(5 * time.Millisecond)
	ts1 := w.Body.String()

	// trigger reload
	lr.Reload()

	time.Sleep(5 * time.Millisecond)
	ts2 := w.Body.String()

	if ts2 <= ts1 {
		t.Fatalf("expected second timestamp '%s' to be greather than the first '%s'", ts2, ts1)
	}
}

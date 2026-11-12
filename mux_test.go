package httpx_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/bluescreen10/httpx"
)

func TestGroup(t *testing.T) {
	mux := httpx.NewServeMux()
	api := mux.Group("/api")
	var count int
	api.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		count++
	})

	r := httptest.NewRequest("GET", "/api/test", &bytes.Buffer{})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if count != 1 {
		t.Fatalf("expected to be called '1' got '%d'", count)
	}
}

func TestUseMiddleware(t *testing.T) {
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Test-1", "test")
			next.ServeHTTP(w, r)
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Test-2", "test")
			next.ServeHTTP(w, r)
		})
	}

	mux := httpx.NewServeMux()
	mux.Use(mw1)
	mux.Use(mw2)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})

	r := httptest.NewRequest("GET", "/", &bytes.Buffer{})
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	body, err := io.ReadAll(w.Body)
	if err != nil || string(body) != "hello world" {
		t.Fatalf("excted 'hello world' got '%s'", body)
	}

	if h := w.Result().Header.Get("Test-1"); h != "test" {
		t.Fatalf("expected header value to be 'test' got '%s'", h)
	}

	if h := w.Result().Header.Get("Test-2"); h != "test" {
		t.Fatalf("expected header value to be 'test' got '%s'", h)
	}
}

func TestUseMiddlewareOrder(t *testing.T) {
	var order []string
	middleware := func(name string) httpx.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+" before")
				next.ServeHTTP(w, r)
				order = append(order, name+" after")
			})
		}
	}

	mux := httpx.NewServeMux()
	mux.Use(middleware("first"))
	mux.Use(middleware("second"))
	group := mux.Group("/api", middleware("group first"), middleware("group second"))
	group.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})
	mux.Use(middleware("third"))

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/test", nil))
	want := []string{
		"first before", "second before", "third before",
		"group first before", "group second before", "handler",
		"group second after", "group first after",
		"third after", "second after", "first after",
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("middleware order = %v, want %v", order, want)
	}
}

func TestUseMiddlewarePreservesStateAcrossRequests(t *testing.T) {
	mux := httpx.NewServeMux()
	mux.Use(func(next http.Handler) http.Handler {
		requests := 0
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.Header().Set("X-Requests", fmt.Sprint(requests))
			next.ServeHTTP(w, r)
		})
	})
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})

	for _, want := range []string{"1", "2"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if got := w.Header().Get("X-Requests"); got != want {
			t.Fatalf("request count = %q, want %q", got, want)
		}
	}
}

func TestGroupWithMiddlewares(t *testing.T) {
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Test", "test")
			next.ServeHTTP(w, r)
		})
	}

	mux := httpx.NewServeMux()
	api := mux.Group("/api", mw)
	api.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})

	r := httptest.NewRequest("GET", "/api/test", &bytes.Buffer{})
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	body, err := io.ReadAll(w.Body)
	if err != nil || string(body) != "hello world" {
		t.Fatalf("excted 'hello world' got '%s'", body)
	}

	if h := w.Result().Header.Get("Test"); h != "test" {
		t.Fatalf("expected header value to be 'test' got '%s'", h)
	}
}

func TestStatic(t *testing.T) {
	mux := httpx.NewServeMux()
	mux.Static("/static/", ".")

	r := httptest.NewRequest("GET", "/static/README.md", &bytes.Buffer{})
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("error reading body")
	}

	if len(body) == 0 {
		t.Fatal("expected body to have contenst but got ''")
	}
}

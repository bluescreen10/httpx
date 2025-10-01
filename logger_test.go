package httpx_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluescreen10/httpx"
)

func TestLogger(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	output := &bytes.Buffer{}
	logger := httpx.LoggerWithConfig(httpx.LoggerConfig{Format: "${method} ${path} ${status}", Output: output})

	r := httptest.NewRequest("GET", "/endpoint", &bytes.Buffer{})
	w := httptest.NewRecorder()

	logger(h).ServeHTTP(w, r)

	got := output.String()
	expected := "GET /endpoint 404"
	if got != expected {
		t.Fatalf("invalid log expected '%s' got '%s'", expected, got)
	}
}

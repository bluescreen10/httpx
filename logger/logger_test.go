package logger_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluescreen10/httpx/logger"
)

func TestLogger(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	output := &bytes.Buffer{}
	logger := logger.New(logger.WithOutput(output))

	r := httptest.NewRequest("GET", "/endpoint", &bytes.Buffer{})
	w := httptest.NewRecorder()

	logger.Handler(h).ServeHTTP(w, r)

	log := output.String()
	if !strings.Contains(log, " GET ") ||
		!strings.Contains(log, " /endpoint ") ||
		!strings.Contains(log, " 404 ") {
		t.Fatal("invalid log")
	}
}

package httpx_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluescreen10/httpx"
)

func TestLogger(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Microsecond)
		w.WriteHeader(404)
	})

	format := httpx.DefaultLoggerConfig.Format
	output := bytes.Buffer{}
	logger := httpx.Logger(h, httpx.LoggerConfig{Format: format, Output: &output})

	r := httptest.NewRequest("GET", "/endpoint", &bytes.Buffer{})
	w := httptest.NewRecorder()

	logger.ServeHTTP(w, r)

	log := output.String()
	if !strings.Contains(log, " GET ") ||
		!strings.Contains(log, " /endpoint ") ||
		!strings.Contains(log, " 404 ") {
		t.Fatal("invalid log")
	}
}

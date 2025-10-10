package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bluescreen10/httpx"
)

func TestRenderer(t *testing.T) {
	r := httpx.NewRenderer(os.DirFS("."), ".html")
	w := httptest.NewRecorder()
	testString := "hello world"
	r.Html(w, "renderer_test", httpx.Vals{"test": testString})

	if body := w.Body.String(); body != testString {
		t.Fatalf("expected body '%s' got '%s'", testString, body)
	}

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected status code '200' got '%d'", w.Result().StatusCode)
	}

	if ct := w.Result().Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected content type 'text/html; charset=utf-8' got '%s'", ct)
	}
}

func TestRendererWithLayout(t *testing.T) {
	r := httpx.NewRenderer(os.DirFS("."), ".html")
	w := httptest.NewRecorder()
	testString := "hello world"
	r.Html(w, "renderer_test", httpx.Vals{"test": testString}, "renderer_layout_test")

	expectedBody := fmt.Sprintf("<html>\n%s\n\n</html>", testString)
	if body := w.Body.String(); body != expectedBody {
		t.Fatalf("expected body '%s' got '%s'", expectedBody, body)
	}

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected status code '200' got '%d'", w.Result().StatusCode)
	}

	if ct := w.Result().Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected content type 'text/html; charset=utf-8' got '%s'", ct)
	}
}

package httpx_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestServerMuxResolvesProxyBeforeMiddleware(t *testing.T) {
	mux := httpx.NewServeMuxWithConfig(httpx.ServeMuxConfig{
		TrustProxy:     true,
		TrustedProxies: []string{"172.18.0.1"},
		ProxyHeader:    httpx.HeaderXForwardedFor,
	})
	var logs bytes.Buffer
	mux.Use(httpx.LoggerWithConfig(httpx.LoggerConfig{Format: "${ip}", Output: &logs}))
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RemoteAddr != "203.0.113.27:0" {
				t.Errorf("middleware RemoteAddr = %q, want visitor address", r.RemoteAddr)
			}
			next.ServeHTTP(w, r)
		})
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "203.0.113.27:0" {
			t.Errorf("handler RemoteAddr = %q, want visitor address", r.RemoteAddr)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "172.18.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.27")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if logs.String() != "203.0.113.27" {
		t.Fatalf("logger IP = %q, want visitor IP", logs.String())
	}
	if r.RemoteAddr != "172.18.0.1:1234" {
		t.Fatal("mux must not mutate the caller request")
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestServerMuxProxyTrustAndAddressChains(t *testing.T) {
	for _, test := range []struct {
		name   string
		config httpx.ServeMuxConfig
		peer   string
		header string
		values []string
		want   string
	}{
		{name: "default trust disabled", peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "172.18.0.1:1234"},
		{name: "master switch disabled", config: httpx.ServeMuxConfig{TrustedProxies: []string{"172.18.0.1"}, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "172.18.0.1:1234"},
		{name: "no trusted peers", config: httpx.ServeMuxConfig{TrustProxy: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "172.18.0.1:1234"},
		{name: "untrusted public peer", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "198.51.100.2:1234", values: []string{"203.0.113.27"}, want: "198.51.100.2:1234"},
		{name: "private peer opt in", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "loopback peer opt in", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "[::1]:1234", values: []string{"2001:db8::27"}, want: "[2001:db8::27]:0"},
		{name: "IPv6 private peer", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "[fd00::1]:1234", values: []string{"2001:db8::27"}, want: "[2001:db8::27]:0"},
		{name: "CIDR peer", config: httpx.ServeMuxConfig{TrustProxy: true, TrustedProxies: []string{"172.18.0.0/24"}}, peer: "172.18.0.3:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "peer outside CIDR", config: httpx.ServeMuxConfig{TrustProxy: true, TrustedProxies: []string{"172.18.0.0/24"}}, peer: "172.18.1.3:1234", values: []string{"203.0.113.27"}, want: "172.18.1.3:1234"},
		{name: "IPv4 mapped peer", config: httpx.ServeMuxConfig{TrustProxy: true, TrustedProxies: []string{"172.18.0.1"}}, peer: "[::ffff:172.18.0.1]:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "trusted chain", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27, 10.0.0.2"}, want: "203.0.113.27:0"},
		{name: "forged leftmost address", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"198.51.100.99, 203.0.113.27, 10.0.0.2"}, want: "203.0.113.27:0"},
		{name: "multiple header lines", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"198.51.100.99, 203.0.113.27", "10.0.0.2"}, want: "203.0.113.27:0"},
		{name: "private terminal visitor", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"10.0.0.7, 10.0.0.2"}, want: "10.0.0.7:0"},
		{name: "visitor port", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"[2001:db8::27]:4321"}, want: "[2001:db8::27]:4321"},
		{name: "custom header", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true, ProxyHeader: "CF-Connecting-IP"}, peer: "172.18.0.1:1234", header: "CF-Connecting-IP", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "missing header", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", want: "172.18.0.1:1234"},
		{name: "malformed nearest address", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27, invalid"}, want: "172.18.0.1:1234"},
		{name: "malformed trusted chain", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27, invalid, 10.0.0.2"}, want: "172.18.0.1:1234"},
		{name: "IPv6 zone rejected", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"fe80::1%eth0"}, want: "172.18.0.1:1234"},
		{name: "unspecified address rejected", config: httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"0.0.0.0"}, want: "172.18.0.1:1234"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mux := httpx.NewServeMuxWithConfig(test.config)
			seen := ""
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				seen = r.RemoteAddr
			})
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = test.peer
			header := test.header
			if header == "" {
				header = httpx.HeaderXForwardedFor
			}
			for _, value := range test.values {
				r.Header.Add(header, value)
			}
			mux.ServeHTTP(httptest.NewRecorder(), r)
			if seen != test.want {
				t.Fatalf("RemoteAddr = %q, want %q", seen, test.want)
			}
		})
	}
}

func TestServerMuxNestedGroupsResolveTheAddressOnce(t *testing.T) {
	root := httpx.NewServeMuxWithConfig(httpx.ServeMuxConfig{TrustProxy: true, TrustedProxies: []string{"172.18.0.1"}})
	var visits int
	check := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			visits++
			if r.RemoteAddr != "10.0.0.7:0" {
				t.Errorf("nested middleware RemoteAddr = %q, want 10.0.0.7:0", r.RemoteAddr)
			}
			next.ServeHTTP(w, r)
		})
	}
	root.Use(check)
	group := root.Group("/api", check)
	nested := httpx.NewServeMuxWithConfig(httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true})
	nested.Use(check)
	nested.Handle("/", check(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	group.Handle("/nested/", http.StripPrefix("/nested", nested))
	r := httptest.NewRequest("GET", "/api/nested/", nil)
	r.RemoteAddr = "172.18.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.99, 10.0.0.7")
	root.ServeHTTP(httptest.NewRecorder(), r)
	if visits != 4 {
		t.Fatalf("middleware visits = %d, want 4", visits)
	}
}

func TestNewServeMuxUsesDefaultServeMuxConfig(t *testing.T) {
	saved := httpx.DefaultServeMuxConfig
	defer func() { httpx.DefaultServeMuxConfig = saved }()
	if saved.TrustProxy || saved.TrustPrivateIPs || len(saved.TrustedProxies) != 0 || saved.ProxyHeader != httpx.HeaderXForwardedFor {
		t.Fatal("default config must disable trust and select X-Forwarded-For")
	}
	httpx.DefaultServeMuxConfig = httpx.ServeMuxConfig{TrustProxy: true, TrustPrivateIPs: true}
	mux := httpx.NewServeMux()
	seen := ""
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr })
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.27")
	mux.ServeHTTP(httptest.NewRecorder(), r)
	if seen != "203.0.113.27:0" {
		t.Fatalf("default-configured RemoteAddr = %q", seen)
	}
}

func TestServerMuxRejectsInvalidTrustedProxyConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("invalid trusted proxy config must panic")
		}
	}()
	httpx.NewServeMuxWithConfig(httpx.ServeMuxConfig{TrustProxy: true, TrustedProxies: []string{"invalid"}})
}

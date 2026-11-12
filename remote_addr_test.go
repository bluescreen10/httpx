package httpx_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluescreen10/httpx"
)

func ExampleRemoteAddr() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.RemoteAddr)
	})
	handler := httpx.RemoteAddr(httpx.RemoteAddrConfig{
		TrustedProxies: []string{"127.0.0.1"},
		ProxyHeader:    "CF-Connecting-IP",
	})(mux)

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("CF-Connecting-IP", "203.0.113.27")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	fmt.Println(w.Body.String())
	// Output: 203.0.113.27:0
}

func TestServeMuxWithoutRemoteAddrMiddleware(t *testing.T) {
	mux := httpx.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "127.0.0.1:1234" {
			t.Errorf("RemoteAddr = %q, want original peer", r.RemoteAddr)
		}
	})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set(httpx.HeaderXForwardedFor, "203.0.113.27")
	mux.ServeHTTP(httptest.NewRecorder(), r)
}

func TestRemoteAddrResolvesBeforeMiddleware(t *testing.T) {
	mux := httpx.NewServeMux()
	mux.Use(httpx.RemoteAddr(httpx.RemoteAddrConfig{TrustedProxies: []string{"172.18.0.1"}}))
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

func TestRemoteAddrTrustAndAddressChains(t *testing.T) {
	for _, test := range []struct {
		name   string
		config httpx.RemoteAddrConfig
		peer   string
		header string
		values []string
		want   string
	}{
		{name: "zero config trusts no peers", peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "172.18.0.1:1234"},
		{name: "header alone grants no trust", config: httpx.RemoteAddrConfig{ProxyHeader: "CF-Connecting-IP"}, peer: "172.18.0.1:1234", header: "CF-Connecting-IP", values: []string{"203.0.113.27"}, want: "172.18.0.1:1234"},
		{name: "untrusted public peer", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "198.51.100.2:1234", values: []string{"203.0.113.27"}, want: "198.51.100.2:1234"},
		{name: "private peer opt in", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "loopback peer opt in", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "[::1]:1234", values: []string{"2001:db8::27"}, want: "[2001:db8::27]:0"},
		{name: "IPv6 private peer", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "[fd00::1]:1234", values: []string{"2001:db8::27"}, want: "[2001:db8::27]:0"},
		{name: "CIDR peer", config: httpx.RemoteAddrConfig{TrustedProxies: []string{"172.18.0.0/24"}}, peer: "172.18.0.3:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "peer outside CIDR", config: httpx.RemoteAddrConfig{TrustedProxies: []string{"172.18.0.0/24"}}, peer: "172.18.1.3:1234", values: []string{"203.0.113.27"}, want: "172.18.1.3:1234"},
		{name: "IPv4 mapped peer", config: httpx.RemoteAddrConfig{TrustedProxies: []string{"172.18.0.1"}}, peer: "[::ffff:172.18.0.1]:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "IPv4 mapped CIDR", config: httpx.RemoteAddrConfig{TrustedProxies: []string{"::ffff:172.18.0.0/120"}}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "IPv6 CIDR", config: httpx.RemoteAddrConfig{TrustedProxies: []string{"2001:db8::/64"}}, peer: "[2001:db8::1]:1234", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "trusted chain", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27, 10.0.0.2"}, want: "203.0.113.27:0"},
		{name: "forged leftmost address", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"198.51.100.99, 203.0.113.27, 10.0.0.2"}, want: "203.0.113.27:0"},
		{name: "multiple header lines", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"198.51.100.99, 203.0.113.27", "10.0.0.2"}, want: "203.0.113.27:0"},
		{name: "private terminal visitor", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"10.0.0.7, 10.0.0.2"}, want: "10.0.0.7:0"},
		{name: "visitor port", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"[2001:db8::27]:4321"}, want: "[2001:db8::27]:4321"},
		{name: "IPv4 visitor port", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27:4321"}, want: "203.0.113.27:4321"},
		{name: "custom header", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true, ProxyHeader: "CF-Connecting-IP"}, peer: "172.18.0.1:1234", header: "CF-Connecting-IP", values: []string{"203.0.113.27"}, want: "203.0.113.27:0"},
		{name: "missing header", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", want: "172.18.0.1:1234"},
		{name: "empty header", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{""}, want: "172.18.0.1:1234"},
		{name: "malformed peer", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "invalid", values: []string{"203.0.113.27"}, want: "invalid"},
		{name: "malformed nearest address", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27, invalid"}, want: "172.18.0.1:1234"},
		{name: "malformed trusted chain", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"203.0.113.27, invalid, 10.0.0.2"}, want: "172.18.0.1:1234"},
		{name: "IPv6 zone rejected", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"fe80::1%eth0"}, want: "172.18.0.1:1234"},
		{name: "unspecified address rejected", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"0.0.0.0"}, want: "172.18.0.1:1234"},
		{name: "multicast address rejected", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"224.0.0.1"}, want: "172.18.0.1:1234"},
		{name: "ignore malformed spoofed prefix", config: httpx.RemoteAddrConfig{TrustPrivateIPs: true}, peer: "172.18.0.1:1234", values: []string{"invalid, 203.0.113.27, 10.0.0.2"}, want: "203.0.113.27:0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			seen := ""
			handler := httpx.RemoteAddr(test.config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.RemoteAddr
			}))
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = test.peer
			header := test.header
			if header == "" {
				header = httpx.HeaderXForwardedFor
			}
			for _, value := range test.values {
				r.Header.Add(header, value)
			}
			handler.ServeHTTP(httptest.NewRecorder(), r)
			if seen != test.want {
				t.Fatalf("RemoteAddr = %q, want %q", seen, test.want)
			}
			if r.RemoteAddr != test.peer {
				t.Fatal("middleware must not mutate the caller request")
			}
		})
	}
}

func TestRemoteAddrNestedGroupsResolveTheAddressOnce(t *testing.T) {
	root := httpx.NewServeMux()
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
	root.Use(httpx.RemoteAddr(httpx.RemoteAddrConfig{TrustedProxies: []string{"172.18.0.1"}}))
	root.Use(check)
	group := root.Group("/api", check)
	nested := httpx.NewServeMux()
	nested.Use(httpx.RemoteAddr(httpx.RemoteAddrConfig{TrustPrivateIPs: true}))
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

func TestRemoteAddrRejectsInvalidTrustedProxyConfig(t *testing.T) {
	for _, entry := range []string{"invalid", "172.18.0.0/33", "fe80::1%eth0"} {
		t.Run(entry, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid trusted proxy config must panic")
				}
			}()
			httpx.RemoteAddr(httpx.RemoteAddrConfig{TrustedProxies: []string{entry}})
		})
	}
}

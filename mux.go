package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// ServeMux is a wrapper around http.ServeMux that adds support for
// route grouping and applying middlewares with syntax sugar.
//
// Usage:
//
//	mux := httpx.NewServeMux()
//
//	// Global middleware for all routes
//	mux.Use(httpx.Logger())
//
//	// Route group with prefix "/api" and additional middleware
//	api := mux.Group("/api", authMiddleware)
//
//	// Register handlers on the group
//	api.HandleFunc("/users", usersHandler)
//	api.HandleFunc("/posts", postsHandler)
//
//	http.ListenAndServe(":8080", mux)
type ServeMux struct {
	*http.ServeMux
	handler http.Handler
	proxy   *proxyConfig
}

// NewServeMux creates a new ServeMux instance.
func NewServeMux() *ServeMux {
	return NewServeMuxWithConfig(DefaultServeMuxConfig)
}

// NewServeMuxWithConfig creates a mux with explicit configuration
// It panics if proxy trust is enabled and a trusted address is invalid.
func NewServeMuxWithConfig(config ServeMuxConfig) *ServeMux {
	mux := new(ServeMux)
	mux.proxy = newProxyConfig(config)
	mux.ServeMux = http.NewServeMux()
	mux.handler = mux.ServeMux
	return mux
}

// Group creates a sub-router with the given prefix and optional middlewares.
// The returned sub-router can register its own handlers, which will inherit
// the parent middlewares automatically.
func (mux *ServeMux) Group(prefix string, middlewares ...Middleware) *ServeMux {
	prefix = strings.TrimSuffix(prefix, "/")
	subMux := NewServeMuxWithConfig(ServeMuxConfig{})

	var wrapped http.Handler = subMux

	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}

	mux.Handle(prefix+"/", http.StripPrefix(prefix, wrapped))
	return subMux
}

// Static mounts a file server with the give prefix and optional middlewares.
// The dir specifies the root of the file server.
func (mux *ServeMux) Static(prefix, dir string, middlewares ...Middleware) {
	handler := http.FileServer(http.Dir(dir))

	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	mux.Handle(prefix, http.StripPrefix(prefix, handler))
}

// Use adds a global middleware to the ServeMux. These middlewares are applied
// to all routes registered on this mux.
func (mux *ServeMux) Use(middleware Middleware) {
	mux.handler = middleware(mux.handler)
}

// ServeHTTP implements http.Handler and applies global middlewares
// before dispatching to the underlying http.ServeMux.
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if mux.proxy != nil {
		r = mux.proxy.request(r)
	}
	mux.handler.ServeHTTP(w, r)
}

// HeaderXForwardedFor names the header containing the forwarding address chain.
const HeaderXForwardedFor = "X-Forwarded-For"

// ServeMuxConfig controls forwarded client addresses before mux middleware runs.
// ProxyHeader names a header containing an IP address or a comma-separated chain.
// Forwarded addresses use IP:port syntax in RemoteAddr, with port 0 when unknown.
// The outermost enabled mux resolves the address once for the request.
type ServeMuxConfig struct {
	TrustProxy      bool
	TrustedProxies  []string // Exact IP addresses or CIDR prefixes.
	TrustPrivateIPs bool     // Also trust private and loopback proxy addresses.
	ProxyHeader     string
}

// DefaultServeMuxConfig disables proxy trust and selects X-Forwarded-For.
// NewServeMux uses these defaults. Pass a copy to NewServeMuxWithConfig for custom settings.
var DefaultServeMuxConfig = ServeMuxConfig{
	ProxyHeader: HeaderXForwardedFor,
}

type proxyConfig struct {
	header          string
	trustPrivateIPs bool
	trusted         []netip.Prefix
}

type proxyResolvedKey struct{}

func newProxyConfig(config ServeMuxConfig) *proxyConfig {
	if !config.TrustProxy {
		return nil
	}
	proxy := &proxyConfig{header: config.ProxyHeader, trustPrivateIPs: config.TrustPrivateIPs}
	if proxy.header == "" {
		proxy.header = HeaderXForwardedFor
	}
	for _, entry := range config.TrustedProxies {
		value := strings.TrimSpace(entry)
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			address, addressErr := netip.ParseAddr(value)
			if addressErr != nil || address.Zone() != "" {
				panic(fmt.Sprintf("httpx: invalid trusted proxy %q", entry))
			}
			address = address.Unmap()
			prefix = netip.PrefixFrom(address, address.BitLen())
		} else if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		proxy.trusted = append(proxy.trusted, prefix.Masked())
	}
	return proxy
}

func (proxy *proxyConfig) trusts(address netip.Addr) bool {
	address = address.Unmap()
	if proxy.trustPrivateIPs && (address.IsPrivate() || address.IsLoopback()) {
		return true
	}
	for _, prefix := range proxy.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func (proxy *proxyConfig) request(r *http.Request) *http.Request {
	if r.Context().Value(proxyResolvedKey{}) != nil {
		return r
	}
	resolved := r.WithContext(context.WithValue(r.Context(), proxyResolvedKey{}, true))
	peer, ok := proxyAddress(r.RemoteAddr)
	if !ok || !proxy.trusts(peer.Addr()) {
		return resolved
	}
	values := r.Header.Values(proxy.header)
	client := peer
	// Walk from the connected proxy toward the visitor. Stop at the first untrusted address.
	for i := len(values) - 1; i >= 0; i-- {
		addresses := strings.Split(values[i], ",")
		for j := len(addresses) - 1; j >= 0; j-- {
			forwarded, valid := proxyAddress(addresses[j])
			if !valid {
				return resolved
			}
			client = forwarded
			if !proxy.trusts(client.Addr()) {
				resolved.RemoteAddr = client.String()
				return resolved
			}
		}
	}
	if len(values) > 0 {
		resolved.RemoteAddr = client.String()
	}
	return resolved
}

func proxyAddress(value string) (netip.AddrPort, bool) {
	value = strings.TrimSpace(value)
	if address, err := netip.ParseAddr(value); err == nil {
		if address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() {
			return netip.AddrPort{}, false
		}
		return netip.AddrPortFrom(address.Unmap(), 0), true
	}
	address, err := netip.ParseAddrPort(value)
	if err != nil || address.Addr().Zone() != "" || address.Addr().IsUnspecified() || address.Addr().IsMulticast() {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(address.Addr().Unmap(), address.Port()), true
}

package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// HeaderXForwardedFor names the header containing the forwarding address chain.
const HeaderXForwardedFor = "X-Forwarded-For"

// RemoteAddrConfig controls which proxies may supply forwarded client addresses.
// The zero value trusts no proxies and leaves RemoteAddr unchanged.
type RemoteAddrConfig struct {
	// TrustedProxies contains exact IP addresses or CIDR prefixes.
	TrustedProxies []string
	// TrustPrivateIPs additionally trusts RFC 1918, IPv6 ULA, and loopback addresses.
	// Enable only when those peers can supply authoritative forwarding headers.
	TrustPrivateIPs bool
	// ProxyHeader contains an IP address or a comma-separated address chain.
	// It defaults to HeaderXForwardedFor when empty.
	ProxyHeader string
}

// RemoteAddr returns middleware that resolves r.RemoteAddr from a trusted proxy's
// forwarding header. It works with any http.Handler and must wrap middleware that
// needs the client address, including Logger. With ServeMux.Use, add it first so
// it runs first.
//
// The connected peer must be trusted. The header chain is walked from right to
// left, stopping at the first untrusted address. Malformed addresses encountered
// during this walk preserve the original peer address. Resolved addresses use
// IP:port syntax, with port 0 when absent from the header.
//
// The caller's request is not modified. When multiple RemoteAddr middlewares
// wrap a handler, only the outermost resolves the address for that request.
// RemoteAddr panics if a trusted IP address or CIDR prefix is invalid.
func RemoteAddr(config RemoteAddrConfig) Middleware {
	proxy := newProxyConfig(config)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, proxy.request(r))
		})
	}
}

type proxyConfig struct {
	header          string
	trustPrivateIPs bool
	trusted         []netip.Prefix
}

type proxyResolvedKey struct{}

func newProxyConfig(config RemoteAddrConfig) *proxyConfig {
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

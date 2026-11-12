# HTTPx - HTTP Extras for Go

![logo](docs/logo.png)

HTTPx is a collection of HTTP utilities for Go that extend the standard `net/http` package with commonly needed functionality for web development. It provides middlewares, request helpers, and session management to simplify building robust webapps.

## Features
* 🔄 **LiveReload** – Automatically reloads web pages during development using Server-Sent Events.
* 🔎 **Body Parsing** – Parse request bodies into a user-defined struct, supporting JSON, XML, and form data.
* 🪵 **Logger** – Log HTTP requests with customizable formats to console or any `io.Writer`.
* 🏷️ **ETag** – Enables efficient client-side caching via automatic ETag headers.
* ⏰ **Session** – Secure and simple session management with cookie-based storage and pluggable backends.
* 🧩 **Mux** – Grouped routing with middleware support, making it easy to organize complex HTTP routes.
* 🌐 **RemoteAddr** – Resolve client addresses from trusted reverse proxies.

## Quick Start

Below is an example of using `httpx.ServeMux` with the Logger middleware:

```go
package main

import (
    "fmt"
    "net/http"
    "time"

    "github.com/bluescreen10/httpx"
    "github.com/bluescreen10/httpx/logger"
)

func main() {
    // Create a new ServeMux
    mux := httpx.NewServeMux()

    // Attach Logger middleware
    mux.Use(httpx.Logger())

    // Define routes
    mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "Hello, HTTPx!")
    }))

    // Start the server
    fmt.Println("Server running on http://localhost:8080")
    http.ListenAndServe(":8080", mux)
}
```

## Trusted proxies

Wrap your handler with `RemoteAddr` to resolve `r.RemoteAddr` before logging, routing, and other request processing. It works with any `http.Handler`, including the standard library's mux.

```go
mux := httpx.NewServeMux()
mux.Use(httpx.Logger())

remoteAddr := httpx.RemoteAddr(httpx.RemoteAddrConfig{
    TrustedProxies:  []string{"127.0.0.1", "172.18.0.0/24"},
    TrustPrivateIPs: false,
    ProxyHeader:    httpx.HeaderXForwardedFor,
})
// Register routes on mux, then serve the wrapped handler.
http.ListenAndServe(":8080", remoteAddr(mux))
```

`TrustedProxies` accepts exact IP addresses and CIDR prefixes. `TrustPrivateIPs` additionally trusts RFC 1918, IPv6 ULA, and loopback addresses.
The connected peer must be trusted. The middleware then walks the header chain from right to left and selects the first untrusted address (or the leftmost address if all are trusted).
Malformed addresses in that part of the chain preserve the original peer address. The caller's request is not modified. If installed more than once, only the outermost `RemoteAddr` middleware resolves the address.

Forwarded addresses retain Go's `IP:port` format. A header without a port produces port `0`.
IPv6 and forwarded addresses with explicit ports are supported. `ProxyHeader` defaults to `HeaderXForwardedFor` when empty. To use Cloudflare Tunnel's visitor header, set `ProxyHeader: "CF-Connecting-IP"` and trust the address of the connected `cloudflared` peer.

You can also install it through `mux.Use`. Middleware runs in the order it is added, so **add `RemoteAddr` first**:

```go
mux := httpx.NewServeMux()
mux.Use(httpx.RemoteAddr(httpx.RemoteAddrConfig{
    TrustedProxies: []string{"127.0.0.1"},
    ProxyHeader:   "CF-Connecting-IP",
}))
mux.Use(httpx.Logger())
// Add other global middleware here.
```

Configure all middleware before serving requests. Each `Use` call rebuilds the chain; requests reuse the completed chain and its middleware state.

Installing the middleware enables proxy processing; `RemoteAddrConfig{}` trusts no peers and leaves `RemoteAddr` unchanged. Enable private-IP trust only when those peers can supply authoritative forwarding headers.
An invalid trusted IP or CIDR causes `RemoteAddr` to panic when creating the middleware.

Migrating from v0.9.0: replace `NewServeMuxWithConfig` with `NewServeMux` and move `TrustedProxies`, `TrustPrivateIPs`, and `ProxyHeader` into `RemoteAddrConfig`. `ServeMuxConfig`, `DefaultServeMuxConfig`, and `TrustProxy` have been removed.
Also update existing `mux.Use` calls to the desired execution order: the first added now runs first, matching group middleware. Previously, the last added ran first. Response processing unwinds in reverse order.

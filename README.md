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

Configure proxy trust on the mux. The mux resolves `r.RemoteAddr` before global middleware, group middleware, and handlers run.

```go
mux := httpx.NewServeMuxWithConfig(httpx.ServeMuxConfig{
    TrustProxy:      true,
    TrustedProxies:  []string{"127.0.0.1", "172.18.0.0/24"},
    TrustPrivateIPs: false,
    ProxyHeader:    httpx.HeaderXForwardedFor,
})
mux.Use(httpx.Logger())
```

`TrustedProxies` accepts exact IP addresses and CIDR prefixes. `TrustPrivateIPs` additionally trusts RFC 1918, IPv6 ULA, and loopback addresses.
The connected peer must be trusted. The mux then walks the header chain from right to left and selects the first untrusted address.
Malformed addresses in that part of the chain preserve the original peer address. Nested muxes resolve a request once.

Forwarded addresses retain Go's `IP:port` format. A header without a port produces port `0`.
IPv6 and forwarded addresses with explicit ports are supported. `ProxyHeader` can name another header with the same address syntax, such as `CF-Connecting-IP`.

`DefaultServeMuxConfig` disables proxy trust and selects `HeaderXForwardedFor`. `NewServeMux()` uses this default configuration.
You can also customize a copy explicitly:

```go
config := httpx.DefaultServeMuxConfig
config.TrustProxy = true
config.TrustPrivateIPs = true
mux := httpx.NewServeMuxWithConfig(config)
```

`TrustProxy` enables all proxy processing. Enable private-IP trust only when those peers can supply authoritative forwarding headers.
An invalid trusted IP or CIDR causes `NewServerMux` to panic when proxy trust is enabled.

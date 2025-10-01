# HTTPx - HTTP Extras for Go

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
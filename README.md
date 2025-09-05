# HTTPx - HTTP Extras for Go
HTTPx is a collection of HTTP utilities for Go that extend the standard net/http package with commonly needed functionality for web development.

## Features
🔄 Live Reload: Automatic page reloading during development using Server-Sent Events

## Quick Start
```go
package main

import (
    "fmt"
    "net/http"
    "github.com/bluescreen10/httpx"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", homeHandler)
    
    // Wrap with live reload middleware
    server := httpx.LiveReload(mux, livereload.DefaultConfig)
    
    fmt.Println("Server with live reload running on http://localhost:8080")
    http.ListenAndServe(":8080", server)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    html := `
<!DOCTYPE html>
<html>
<head>
    <title>HTTPx Live Reload</title>
</head>
<body>
    <h1>Hello World!</h1>
    <p>Edit this file and watch it reload automatically!</p>
</body>
</html>`
    
    w.Header().Set("Content-Type", "text/html")
    fmt.Fprint(w, html)
}
```

## How It Works
* Script Injection: The middleware intercepts HTML responses and injects a JavaScript snippet before the closing </body> tag
* SSE Connection: The injected script establishes a Server-Sent Events connection to the configured endpoint
* Reload Trigger: When you trigger a reload event, all connected clients automatically refresh their pages

## Custom Configuration
```go
// Custom SSE endpoint path
config := LiveReloadConfig{
    Path: "/my-reload-endpoint",
}

server := httpx.LiveReload(mux, config)
```

## Contributing
Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

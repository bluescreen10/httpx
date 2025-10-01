// Package logger provides an HTTP middleware for logging server activity.
// It allows customizable log formats and output destinations.
//
// Log entries can include variables such as time, HTTP status, latency,
// client IP, request method, request path, and error (currently unused).
//
// Usage:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("Hello, world!"))
//	})
//
//	// Create a new Logger middleware with default settings
//	logger := httpx.LoggerWithconfig( LoggerConfig{
//		Format: ("${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n"),
//		Output: os.Stdout
//	})
//
//	http.ListenAndServe(":8080", logger(mux))
//
// The middleware wraps the http.Handler, recording request start time,
// status code, latency, client IP, HTTP method, and path. Log entries
// are written to the configured output, defaulting to os.Stdout.
package httpx

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type LoggerConfig struct {
	Format string
	Output io.Writer
}

var DefaultLoggerConfig = LoggerConfig{
	Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
	Output: os.Stdout,
}

// Logger returns a middleware with the default configuration. It logs
// requests using the configured format and output. It records start time,
// response status code, latency, client IP, HTTP method, and path.
func Logger() Middleware {
	return LoggerWithConfig(DefaultLoggerConfig)
}

// LoggerWithConfig returns a Logger middleware with the specified configuration.
func LoggerWithConfig(cfg LoggerConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w, w.Header(), w.WriteHeader)
			next.ServeHTTP(rw, r)

			latency := time.Since(start)
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)

			status := 200
			if rw.status != 0 {
				status = rw.status
			}

			replacer := strings.NewReplacer(
				"${time}", start.Format(time.DateTime),
				"${status}", strconv.Itoa(status),
				"${latency}", latency.String(),
				"${ip}", ip,
				"${method}", r.Method,
				"${path}", r.URL.Path,
				"${error}", "", // not sure how to do this.
			)

			fmt.Fprint(cfg.Output, replacer.Replace(cfg.Format))
		})
	}
}

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
//	l := logger.New(
//	    logger.WithFormat("${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n"),
//	    logger.WithOutput(os.Stdout),
//	)
//
//	// Wrap the mux with the Logger middleware
//	handler := l.Handler(mux)
//
//	http.ListenAndServe(":8080", handler)
//
// The middleware wraps the http.Handler, recording request start time,
// status code, latency, client IP, HTTP method, and path. Log entries
// are written to the configured output, defaulting to os.Stdout.
package logger

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

// Format specifies the log entry format using template variables.
// Available variables:
//   ${time}    - Request start time in RFC 3339 format
//   ${status}  - HTTP response status code
//   ${latency} - Request processing duration
//   ${ip}      - Client IP address
//   ${method}  - HTTP request method
//   ${path}    - Request URL path
//   ${error}   - Error information (currently unused)

const defaultFormat = "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n"

// responseWriter wraps http.ResponseWriter to capture the response status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before delegating to the underlying ResponseWriter.
// It implements the http.ResponseWriter interface.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logger is a middleware that captures request details and writes
// formatted log entries to the configured output.
type Logger struct {
	format string
	output io.Writer
}

type config func(*Logger)

// WithFormat sets a custom log format using template variables.
func WithFormat(format string) config {
	return config(func(l *Logger) {
		l.format = format
	})
}

// WithOutput sets the output destination for log entries.
func WithOutput(output io.Writer) config {
	return config(func(l *Logger) {
		l.output = output
	})
}

// Handler wraps an http.Handler and logs requests using the configured format
// and output. It records start time, response status code, latency, client IP,
// HTTP method, and path.
func (l *Logger) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)

		latency := time.Since(start)
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)

		vars := map[string]string{
			"${time}":    start.Format(time.DateTime),
			"${status}":  strconv.Itoa(rw.statusCode),
			"${latency}": latency.String(),
			"${ip}":      ip,
			"${method}":  r.Method,
			"${path}":    r.URL.Path,
			"${error}":   "", // not sure how to do this.
		}

		fmt.Fprint(l.output, loggerRender(l.format, vars))
	})
}

// loggerRender replaces template variables in the log entry with actual values.
func loggerRender(entry string, vars map[string]string) string {
	for k, v := range vars {
		entry = strings.ReplaceAll(entry, k, v)
	}
	return entry
}

// New creates a new Logger middleware with optional configuration.
func New(cfgs ...config) *Logger {
	lgr := &Logger{
		format: defaultFormat,
		output: os.Stdout,
	}

	for _, cfg := range cfgs {
		cfg(lgr)
	}

	return lgr
}

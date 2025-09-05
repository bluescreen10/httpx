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

// LoggerConfig defines the configuration for the Logger middleware.
// It allows customization of the log format and output destination.
type LoggerConfig struct {
	// Format specifies the log entry format using template variables.
	// Available variables:
	//   ${time}    - Request start time in RFC 3339 format
	//   ${status}  - HTTP response status code
	//   ${latency} - Request processing duration
	//   ${ip}      - Client IP address
	//   ${method}  - HTTP request method
	//   ${path}    - Request URL path
	//   ${error}   - Error information (currently unused)
	Format string

	// Output specifies where log entries should be written.
	// Defaults to os.Stdout.
	Output io.Writer
}

// DefaultLoggerConfig provides a sensible default configuration for the Logger middleware.
// It formats log entries with timestamp, status, latency, IP, method, path, and error fields,
// and writes to standard output.
var DefaultLoggerConfig = LoggerConfig{
	Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
	Output: os.Stdout,
}

// loggerResponseWriter wraps http.ResponseWriter to capture the response status code.
// This is necessary because the standard http.ResponseWriter interface doesn't
// provide access to the status code after it's written.
type loggerResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before delegating to the underlying ResponseWriter.
// It implements the http.ResponseWriter interface.
func (rw *loggerResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logger returns an HTTP middleware that logs request information.
// It wraps the provided handler and logs details about each request including
// timestamp, response status, processing latency, client IP, HTTP method, and request path.
//
// The middleware measures request processing time from start to completion and
// extracts the client IP address from the request's RemoteAddr field.
//
// Usage:
//
//	handler := httpx.Logger(myHandler, DefaultLoggerConfig)
//	http.Handle("/", handler)
//
// Custom configuration example:
//
//	config := LoggerConfig{
//	    Format: "[${time}] ${method} ${path} -> ${status} (${latency})\n",
//	    Output: logFile,
//	}
//	handler := Logger(myHandler, config)
func Logger(next http.Handler, config LoggerConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &loggerResponseWriter{w, http.StatusOK}
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

		fmt.Fprint(config.Output, loggerRender(config.Format, vars))
	})
}

// loggerRender processes the log format template by replacing template variables
// with their corresponding values. It performs simple string replacement for each
// variable in the provided vars map.
func loggerRender(entry string, vars map[string]string) string {
	for k, v := range vars {
		entry = strings.ReplaceAll(entry, k, v)
	}
	return entry
}

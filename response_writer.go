package httpx

import (
	"io"
	"net/http"
)

type responseWriter struct {
	header http.Header
	status int
	writer io.Writer
}

var _ http.ResponseWriter = &responseWriter{}

func newResponseWriter(buf io.Writer, header http.Header) *responseWriter {
	return &responseWriter{
		header: header,
		writer: buf,
	}
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	return rw.writer.Write(data)
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
}

func (rw *responseWriter) Header() http.Header {
	return rw.header
}

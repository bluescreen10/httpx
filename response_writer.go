package httpx

import (
	"io"
	"net/http"
)

type responseWriter struct {
	header      http.Header
	status      int
	writer      io.Writer
	writeHeader func(int)
}

var _ http.ResponseWriter = &responseWriter{}

func newResponseWriter(buf io.Writer, header http.Header, writeHeader func(int)) *responseWriter {
	return &responseWriter{
		header:      header,
		writer:      buf,
		writeHeader: writeHeader,
	}
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	return rw.writer.Write(data)
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	if rw.writeHeader != nil {
		rw.writeHeader(status)
	}

}

func (rw *responseWriter) Header() http.Header {
	return rw.header
}

package treq

import (
	"bytes"
	"net/http"
)

type customResponseWriter struct {
	body       *bytes.Buffer
	statusCode int
	header     http.Header
}

func (w *customResponseWriter) Header() http.Header {
	return w.header
}

func (w *customResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *customResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *customResponseWriter) String() string {
	return w.body.String()
}

func (w *customResponseWriter) Flush() {
}

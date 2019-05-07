package httpwares

import "net/http"

// WrapResponseWriter wraps the http.ResponseWriter in a helper thats useful for building middlewares.
//
// This call *reuses* the existing WrappedResponseWriter, i.e. if it is already wrapped, the existing wrapper will be
// returned.
func WrapResponseWriter(w http.ResponseWriter) WrappedResponseWriter {
	if wrapped, ok := w.(WrappedResponseWriter); ok {
		return wrapped
	}
	return newWrappedResponseWriter(w)
}

// WrappedResponseWriter is a wrapper around http.ResponseWriter that is useful for building middlewares.
//
// If you want to instantiate this, please use `WrapResponseWriter` function.
type WrappedResponseWriter interface {
	http.ResponseWriter
	// Status returns the HTTP status of the request, or 0 if one has not
	// yet been sent.
	StatusCode() int
	// MessageLength returns the size of the HTTP Response Message (after headers), as returned to the client.
	MessageLength() int

	// ObserveWriteHeader adds to the list of callbacks to be triggered when WriteHeader is executed.
	ObserveWriteHeader(func(t WrappedResponseWriter, code int))

	// ObserveWrite adds to the list of callbacks to be triggered when a Write() is executed.
	ObserveWrite(func(t WrappedResponseWriter, buf []byte, n int, err error))
}

// wrappedResponseWriter implements http.ResponseWriter without extensions.
type wrappedResponseWriter struct {
	http.ResponseWriter
	code           int
	bytes          int
	wroteHdr       bool
	observerHeader []func(t WrappedResponseWriter, code int)
	observerWrite  []func(t WrappedResponseWriter, buf []byte, n int, err error)
}

func (w *wrappedResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *wrappedResponseWriter) ObserveWriteHeader(o func(t WrappedResponseWriter, code int)) {
	w.observerHeader = append(w.observerHeader, o)
}

func (w *wrappedResponseWriter) ObserveWrite(o func(t WrappedResponseWriter, buf []byte, n int, err error)) {
	w.observerWrite = append(w.observerWrite, o)
}

func (w *wrappedResponseWriter) WriteHeader(code int) {
	if !w.wroteHdr {
		w.wroteHdr = true
		w.code = code
		w.ResponseWriter.WriteHeader(code)
		for _, o := range w.observerHeader {
			o(w, code)
		}
	}
}

func (w *wrappedResponseWriter) Write(buf []byte) (int, error) {
	w.WriteHeader(http.StatusOK) // double writes are ignored.
	n, err := w.ResponseWriter.Write(buf)
	for _, o := range w.observerWrite {
		o(w, buf, n, err)
	}
	w.bytes += n
	return n, err
}

func (w *wrappedResponseWriter) StatusCode() int {
	return w.code
}

func (w *wrappedResponseWriter) MessageLength() int {
	return w.bytes
}

func (w *wrappedResponseWriter) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (w *wrappedResponseWriter) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
}

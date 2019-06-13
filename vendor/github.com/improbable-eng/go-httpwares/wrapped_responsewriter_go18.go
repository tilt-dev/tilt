// +build go1.8

package httpwares

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

// newWrappedResponseWriter handles the four different methods of upgrading a
// http.ResponseWriter to delegator.
func newWrappedResponseWriter(w http.ResponseWriter) WrappedResponseWriter {
	wrapped := &wrappedResponseWriter{ResponseWriter: w}

	_, isCloseNotifier := w.(http.CloseNotifier)
	_, isFlusher := w.(http.Flusher)
	_, isHijacker := w.(http.Hijacker)
	_, isPusher := w.(http.Pusher)
	_, isReaderFrom := w.(io.ReaderFrom)

	// Check for the four most common combination of interfaces a
	// http.ResponseWriter might implement.
	if !isHijacker && isPusher && isCloseNotifier { // http2.responseWriter (http 2.0)
		return &http2WrappedResponseWriter{wrapped}
	} else if isCloseNotifier && isFlusher && isHijacker && isReaderFrom { // http.response (http 1.1)
		return &http1WrappedResponseWriter{wrapped}
	}
	return wrapped
}

type http2WrappedResponseWriter struct {
	*wrappedResponseWriter
}

func (w *http2WrappedResponseWriter) Flush() {
	w.wrappedResponseWriter.ResponseWriter.(http.Flusher).Flush()
}

func (w *http2WrappedResponseWriter) CloseNotify() <-chan bool {
	return w.wrappedResponseWriter.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (w *http2WrappedResponseWriter) Push(target string, opts *http.PushOptions) error {
	return w.wrappedResponseWriter.ResponseWriter.(http.Pusher).Push(target, opts)
}

type http1WrappedResponseWriter struct {
	*wrappedResponseWriter
}

func (w *http1WrappedResponseWriter) Flush() {
	w.wrappedResponseWriter.ResponseWriter.(http.Flusher).Flush()
}

func (w *http1WrappedResponseWriter) CloseNotify() <-chan bool {
	return w.wrappedResponseWriter.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (w *http1WrappedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.wrappedResponseWriter.ResponseWriter.(http.Hijacker).Hijack()
}

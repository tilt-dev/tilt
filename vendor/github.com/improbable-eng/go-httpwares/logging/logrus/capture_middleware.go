package http_logrus

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/improbable-eng/go-httpwares"
	"github.com/improbable-eng/go-httpwares/logging"
	"github.com/improbable-eng/go-httpwares/logging/logrus/ctxlogrus"
	"github.com/sirupsen/logrus"
	"net"
)

// ContentCaptureMiddleware is a server-side http ware for logging contents of HTTP requests and responses (body and headers).
//
// Only requests with a set Content-Length will be captured, with no streaming or chunk encoding supported.
// Only responses with Content-Length set are captured, no gzipped, chunk-encoded responses are supported.
//
// The body will be recorded as a separate log message. Body of `application/json` will be captured as
// http.request.body_json (in structured JSON form) and others will be captured as http.request.body_raw logrus field
// (raw base64-encoded value).
//
// This *must* be used together with http_logrus.Middleware, as it relies on the logger provided there. However, you can
// override the `logrus.Entry` that is used for logging, allowing for logging to a separate backend (e.g. a different file).
func ContentCaptureMiddleware(entry *logrus.Entry, decider http_logging.ContentCaptureDeciderFunc) httpwares.Middleware {
	return func(nextHandler http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !decider(req) {
				nextHandler.ServeHTTP(resp, req)
				return
			}
			logger := entry.WithFields(ctxlogrus.Extract(req.Context()).Data)
			logger.WithFields(defaultRequestFields(req))
			if err := captureMiddlewareRequestContent(req, logger); err != nil {
				// this is *really* bad, we failed to read a body because of a read error.
				resp.WriteHeader(500)
				logger.WithError(err).Warningf("error in logrus middleware on body read")
				return
			}
			wrappedResp := httpwares.WrapResponseWriter(resp)
			responseCapture := captureMiddlewareResponseContent(wrappedResp, logger)
			nextHandler.ServeHTTP(wrappedResp, req)
			responseCapture.finish() // captureResponse has a nil check, this can be nil
		})
	}
}

func headerIsJson(header http.Header) bool {
	return strings.HasPrefix(strings.ToLower(header.Get("content-type")), "application/json")
}

func captureMiddlewareRequestContent(req *http.Request, entry *logrus.Entry) error {
	if req.ContentLength <= 0 || req.Body == nil {
		// -1 value means that the length cannot be determined, and that it is probably a multipart stremaing call
		if req.ContentLength != 0 || req.Body == nil {
			entry.Infof("request body capture skipped, content length negative")
		}
		return nil
	}
	content, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	// Make sure we give the Response back its body so the client can read it.
	req.Body = ioutil.NopCloser(bytes.NewReader(content))
	if strings.HasPrefix(strings.ToLower(req.Header.Get("content-type")), "application/json") {
		entry.WithField("http.request.body_json", json.RawMessage(content)).Info("request body captured in http.request.body_json field")
	} else {
		entry.WithField("http.request.body_raw", base64.StdEncoding.EncodeToString(content)).Info("request body captured in http.request.body_raw field")
	}
	return nil
}

type responseCapture struct {
	content bytes.Buffer
	isJson  bool
	entry   *logrus.Entry
}

func (c *responseCapture) observeWrite(resp httpwares.WrappedResponseWriter, buf []byte, n int, err error) {
	if err == nil {
		c.content.Write(buf[:n])
	}
}

func (c *responseCapture) finish() {
	if c == nil {
		return
	}
	if c.content.Len() == 0 {
		return
	}

	if c.isJson {
		e := c.entry.WithField("http.response.body_json", json.RawMessage(c.content.Bytes()))
		e.Info("response body captured in http.response.body_json field")
	} else {
		e := c.entry.WithField("http.response.body_raw", base64.StdEncoding.EncodeToString(c.content.Bytes()))
		e.Info("response body captured in http.response.body_raw field")
	}
}

func captureMiddlewareResponseContent(w httpwares.WrappedResponseWriter, entry *logrus.Entry) *responseCapture {
	c := &responseCapture{entry: entry}
	w.ObserveWriteHeader(func(w httpwares.WrappedResponseWriter, code int) {
		if te := w.Header().Get("transfer-encoding"); te != "" {
			entry.Infof("response body capture skipped, transfer encoding is not identity")
			return
		}
		c.isJson = headerIsJson(w.Header())
		w.ObserveWrite(c.observeWrite)
	})
	return c
}

func defaultRequestFields(req *http.Request) logrus.Fields {
	fields := logrus.Fields{}
	if addr := req.RemoteAddr; addr != "" {
		if strings.Contains(addr, ":") {
			if host, port, err := net.SplitHostPort(addr); err == nil {
				fields["peer.address"] = host
				fields["peer.port"] = port
			}
		} else {
			fields["peer.address"] = addr
		}
	}
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	fields["http.host"] = host
	return fields
}

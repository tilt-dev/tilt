package http_logrus

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/improbable-eng/go-httpwares"
	"github.com/improbable-eng/go-httpwares/logging/logrus/ctxlogrus"
	"github.com/sirupsen/logrus"
)

var (
	// SystemField is used in every log statement made through http_logrus. Can be overwritten before any initialization code.
	SystemField = "http"
)

// Middleware is a server-side http ware for logging using logrus.
//
// All handlers will have a Logrus logger in their context, which can be fetched using `ctxlogrus.Extract`.
func Middleware(entry *logrus.Entry, opts ...Option) httpwares.Middleware {
	return func(nextHandler http.Handler) http.Handler {
		options := evaluateMiddlewareOpts(opts)
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			wrappedResp := httpwares.WrapResponseWriter(resp)

			requestFields := appendFields(newServerRequestFields(req), options.requestFieldExtractor(req))
			newEntry := entry.WithFields(requestFields)
			newReq := req.WithContext(ctxlogrus.ToContext(req.Context(), newEntry))
			var capture *responseCapture
			wrappedResp.ObserveWriteHeader(func(w httpwares.WrappedResponseWriter, code int) {
				if options.responseCaptureFunc(req, code) {
					capture = captureMiddlewareResponseContent(w, ctxlogrus.Extract(newReq.Context()))
				}
			})

			startTime := time.Now()
			nextHandler.ServeHTTP(wrappedResp, newReq)
			capture.finish() // captureResponse has a nil check, this can be nil

			if options.shouldLog != nil && !options.shouldLog(wrappedResp, newReq) {
				return
			}

			postCallFields := appendFields(responseFields(wrappedResp, startTime), options.responseFieldExtractor(wrappedResp))
			level := options.levelFunc(wrappedResp.StatusCode())
			levelLogf(
				ctxlogrus.Extract(newReq.Context()).WithFields(postCallFields), // re-extract logger from newCtx, as it may have extra fields that changed in the holder.
				level,
				fmt.Sprintf("finished HTTP call with code %d %s", wrappedResp.StatusCode(), http.StatusText(wrappedResp.StatusCode())))
		})
	}
}

func appendFields(a, b logrus.Fields) logrus.Fields {
	for k, v := range b {
		a[k] = v
	}
	return a
}

func responseFields(wrappedResp httpwares.WrappedResponseWriter, startTime time.Time) logrus.Fields {
	postCallFields := logrus.Fields{
		"http.time_ms":               timeDiffToMilliseconds(startTime),
		"http.response.status":       wrappedResp.StatusCode(),
		"http.response.length_bytes": wrappedResp.MessageLength(),
	}
	return postCallFields
}

func newServerRequestFields(req *http.Request) logrus.Fields {
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}

	fields := logrus.Fields{
		"system":                    SystemField,
		"span.kind":                 "server",
		"http.url.path":             req.URL.Path,
		"http.proto_major":          req.ProtoMajor,
		"http.host":                 host,
		"http.request.method":       req.Method,
		"http.request.user_agent":   req.Header.Get("User-Agent"),
		"http.request.length_bytes": req.ContentLength,
		"http.request.referer":      req.Referer(),
	}

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

	return fields
}

func levelLogf(entry *logrus.Entry, level logrus.Level, format string, args ...interface{}) {
	switch level {
	case logrus.DebugLevel:
		entry.Debugf(format, args...)
	case logrus.InfoLevel:
		entry.Infof(format, args...)
	case logrus.WarnLevel:
		entry.Warningf(format, args...)
	case logrus.ErrorLevel:
		entry.Errorf(format, args...)
	case logrus.FatalLevel:
		entry.Fatalf(format, args...)
	case logrus.PanicLevel:
		entry.Panicf(format, args...)
	default:
		// Unexpected logrus value.
		entry.Panicf(format, args...)
	}
}

func timeDiffToMilliseconds(then time.Time) float32 {
	sub := time.Now().Sub(then).Nanoseconds()
	if sub < 0 {
		return 0.0
	}
	return float32(sub/1000) / 1000.0
}

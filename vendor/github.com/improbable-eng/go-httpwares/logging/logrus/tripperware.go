package http_logrus

import (
	"net/http"
	"time"

	"github.com/improbable-eng/go-httpwares"
	"github.com/sirupsen/logrus"
)

// Tripperware is a server-side http ware for logging using logrus.
//
// This tripperware *does not* propagate a context-based logger, but act as a logger of requests.
// This includes logging of errors.
func Tripperware(entry *logrus.Entry, opts ...Option) httpwares.Tripperware {
	return func(next http.RoundTripper) http.RoundTripper {
		o := evaluateTripperwareOpts(opts)
		return httpwares.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			startTime := time.Now()
			fields := newClientRequestFields(req)
			resp, err := next.RoundTrip(req)
			if err != nil {
				logError(o.levelForConnectivityError, entry.WithFields(fields), err)
				return resp, err
			}
			fields["http.time_ms"] = timeDiffToMilliseconds(startTime)
			fields["http.proto_major"] = resp.ProtoMajor
			fields["http.response.length_bytes"] = resp.ContentLength
			fields["http.status"] = resp.StatusCode
			levelLogf(entry.WithFields(fields), o.levelFunc(resp.StatusCode), "request completed")
			return resp, nil
		})
	}
}

func logError(level logrus.Level, e *logrus.Entry, err error) {
	levelLogf(e.WithError(err), level, "request failed to execute, see err")
}

func newClientRequestFields(req *http.Request) logrus.Fields {
	fields := logrus.Fields{
		"system":                    SystemField,
		"span.kind":                 "client",
		"http.url.path":             req.URL.Path,
		"http.request.length_bytes": req.ContentLength,
	}

	for k, v := range defaultRequestFields(req) {
		fields[k] = v
	}

	return fields
}

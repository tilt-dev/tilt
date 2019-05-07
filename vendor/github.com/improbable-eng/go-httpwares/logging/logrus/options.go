package http_logrus

import (
	"net/http"

	"github.com/improbable-eng/go-httpwares"
	"github.com/sirupsen/logrus"
)

var (
	defaultOptions = &options{
		levelFunc:                 nil,
		levelForConnectivityError: logrus.WarnLevel,
		requestCaptureFunc:        func(r *http.Request) bool { return false },
		responseCaptureFunc:       func(r *http.Request, status int) bool { return false },
		requestFieldExtractor:     func(req *http.Request) map[string]interface{} { return map[string]interface{}{} },
		responseFieldExtractor: func(res httpwares.WrappedResponseWriter) map[string]interface{} {
			return map[string]interface{}{}
		},
	}
)

type options struct {
	levelFunc                 CodeToLevel
	levelForConnectivityError logrus.Level
	requestCaptureFunc        func(r *http.Request) bool
	responseCaptureFunc       func(r *http.Request, status int) bool
	requestFieldExtractor     RequestFieldExtractorFunc
	responseFieldExtractor    ResponseFieldExtractorFunc
	shouldLog                 Decider
}

func evaluateTripperwareOpts(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions
	optCopy.levelFunc = DefaultTripperwareCodeToLevel
	for _, o := range opts {
		o(optCopy)
	}
	return optCopy
}

func evaluateMiddlewareOpts(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions
	optCopy.levelFunc = DefaultMiddlewareCodeToLevel
	for _, o := range opts {
		o(optCopy)
	}
	return optCopy
}

type Option func(*options)

// CodeToLevel user functions define the mapping between HTTP status codes and logrus log levels.
type CodeToLevel func(httpStatusCode int) logrus.Level

// WithLevels customizes the function that maps HTTP client or server side status codes to log levels.
//
// By default `DefaultMiddlewareCodeToLevel` is used for server-side middleware, and `DefaultTripperwareCodeToLevel`
// is used for client-side tripperware.
func WithLevels(f CodeToLevel) Option {
	return func(o *options) {
		o.levelFunc = f
	}
}

// WithConnectivityErrorLevel customizes
func WithConnectivityErrorLevel(level logrus.Level) Option {
	return func(o *options) {
		o.levelForConnectivityError = level
	}
}

// WithRequestBodyCapture enables recording of request body pre-handling/pre-call.
//
// The body will be recorded as a separate log message. Body of `application/json` will be captured as
// http.request.body_json (in structured JSON form) and others will be captured as http.request.body_raw logrus field
// (raw base64-encoded value).
//
// For tripperware, only requests with Body of type `bytes.Buffer`, `strings.Reader`, `bytes.Buffer`, or with
// a specified `GetBody` function will be captured.
//
// For middleware, only requests with a set Content-Length will be captured, with no streaming or chunk encoding supported.
//
// This option creates a copy of the body per request, so please use with care.
func WithRequestBodyCapture(deciderFunc func(r *http.Request) bool) Option {
	return func(o *options) {
		o.requestCaptureFunc = deciderFunc
	}
}

// WithResponseBodyCapture enables recording of response body post-handling/post-call.
//
// The body will be recorded as a separate log message. Body of `application/json` will be captured as
// http.response.body_json (in structured JSON form) and others will be captured as http.response.body_raw logrus field
// (raw base64-encoded value).
//
// Only responses with Content-Length will be captured, with non-default Transfer-Encoding not being supported.
func WithResponseBodyCapture(deciderFunc func(r *http.Request, status int) bool) Option {
	return func(o *options) {
		o.responseCaptureFunc = deciderFunc
	}
}

// WithDecider customizes the function for deciding if the middleware logs at the end of the request.
func WithDecider(f Decider) Option {
	return func(o *options) {
		o.shouldLog = f
	}
}

// Decider function defines rules for suppressing any interceptor logs
type Decider func(w httpwares.WrappedResponseWriter, r *http.Request) bool

// DefaultMiddlewareCodeToLevel is the default of a mapper between HTTP server-side status codes and logrus log levels.
func DefaultMiddlewareCodeToLevel(httpStatusCode int) logrus.Level {
	if httpStatusCode < 400 || httpStatusCode == http.StatusNotFound {
		return logrus.InfoLevel
	} else if httpStatusCode < 500 {
		return logrus.WarnLevel
	} else if httpStatusCode < 600 {
		return logrus.ErrorLevel
	} else {
		return logrus.ErrorLevel
	}
}

// DefaultTripperwareCodeToLevel is the default of a mapper between HTTP client-side status codes and logrus log levels.
func DefaultTripperwareCodeToLevel(httpStatusCode int) logrus.Level {
	if httpStatusCode < 400 {
		return logrus.DebugLevel
	} else if httpStatusCode < 500 {
		return logrus.InfoLevel
	} else {
		return logrus.WarnLevel
	}
}

// WithRequestFieldExtractor adds a field, allowing you to customize what fields get populated from the request.
func WithRequestFieldExtractor(f RequestFieldExtractorFunc) Option {
	return func(o *options) {
		o.requestFieldExtractor = f
	}
}

// WithRequestFieldExtractor adds a field, allowing you to customize what fields get populated from the response.
func WithResponseFieldExtractor(f ResponseFieldExtractorFunc) Option {
	return func(o *options) {
		o.responseFieldExtractor = f
	}
}

// RequestFieldExtractorFunc is a signature of user-customizable functions for extracting log fields from requests.
type RequestFieldExtractorFunc func(req *http.Request) map[string]interface{}

// ResponseFieldExtractorFunc is a signature of user-customizable functions for extracting log fields from responses.
type ResponseFieldExtractorFunc func(res httpwares.WrappedResponseWriter) map[string]interface{}

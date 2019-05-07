package http_logging

import (
	"net/http"
)

// ContentCaptureDeciderFunc is a user-provide function that decides whether the given request-response should be captured
// for logging purposes.
type ContentCaptureDeciderFunc func(req *http.Request) bool

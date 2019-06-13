package http_logrus

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/improbable-eng/go-httpwares"
	"github.com/improbable-eng/go-httpwares/logging"
	"github.com/sirupsen/logrus"
)

// ContentCaptureTripperware is a client-side http ware for logging contents of HTTP requests and responses (body and headers).
//
// Only requests with a set GetBody field will be captured (strings, bytes etc).
// Only responses with Content-Length are captured, with no support for chunk-encoded responses.
//
// The body will be recorded as a separate log message. Body of `application/json` will be captured as
// http.request.body_json (in structured JSON form) and others will be captured as http.request.body_raw logrus field
// (raw base64-encoded value).
func ContentCaptureTripperware(entry *logrus.Entry, decider http_logging.ContentCaptureDeciderFunc) httpwares.Tripperware {
	return func(next http.RoundTripper) http.RoundTripper {
		return httpwares.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if !decider(req) {
				return next.RoundTrip(req)
			}
			fields := newClientRequestFields(req)

			if err := captureTripperwareRequestContent(req, entry.WithFields(fields)); err != nil {
				return nil, err // errors reading GetBody and other problems on client side
			}
			resp, err := next.RoundTrip(req)
			if err != nil {
				return nil, err
			}
			if err := captureTripperwareResponseContent(resp, entry.WithFields(fields)); err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
}

func captureTripperwareRequestContent(req *http.Request, entry *logrus.Entry) error {
	// All requests created with http.NewRequest will have a GetBody method set, even if the user created
	// a body manually.
	if getBody(req) == nil {
		if req.Body != nil {
			entry.Infof("request body capture skipped, missing GetBody method while Body set")
		}
		return nil
	}
	bodyReader, err := getBody(req)()
	if err != nil {
		return err
	}
	content, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		return err
	}
	if headerIsJson(req.Header) {
		entry.WithField("http.request.body_json", json.RawMessage(content)).Info("request body captured in http.request.body_json field")
	} else {
		entry.WithField("http.request.body_raw", base64.StdEncoding.EncodeToString(content)).Info("request body captured in http.request.body_raw field")
	}
	return nil
}

func captureTripperwareResponseContent(resp *http.Response, entry *logrus.Entry) error {
	if resp.ContentLength <= 0 {
		// TODO(mwitkow): Deal with response.Uncompressed and gzip encoding (Content Length -1).
		if resp.ContentLength != 0 {
			entry.Infof("response body capture skipped, content length negative")
		}
		return nil
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err // this is an error form the response reading, potentially a connection failure
	}
	// Make sure we give the Response back its body so the client can read it.
	resp.Body = ioutil.NopCloser(bytes.NewReader(content))
	if headerIsJson(resp.Header) {
		entry.WithField("http.response.body_json", json.RawMessage(content)).Info("request body captured in http.response.body_json field")
	} else {
		entry.WithField("http.response.body_raw", base64.StdEncoding.EncodeToString(content)).Info("request body captured in http.response.body_raw field")
	}
	return nil
}

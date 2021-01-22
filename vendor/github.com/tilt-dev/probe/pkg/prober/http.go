/*
Copyright 2015 The Kubernetes Authors.
Modified 2021 Windmill Engineering.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prober

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"k8s.io/klog/v2"
)

const (
	maxRespBodyLength = 10 * 1 << 10 // 10KB
)

// ErrLimitReached means that the read limit is reached.
var ErrLimitReached = errors.New("the read limit is reached")

func defaultTransport() *http.Transport {
	// TODO(milas): add http2 healthcheck -> https://github.com/kubernetes/apimachinery/blob/master/pkg/util/net/http.go#L173-L189
	return &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
		Proxy:             http.ProxyURL(nil),
	}
}

// NewHTTPGetProber creates a HTTPGetProber that will perform an HTTP GET request to determine service status.
func NewHTTPGetProber() HTTPGetProber {
	return httpProber{defaultTransport()}
}

// HTTPGetProber executes HTTP GET requests to determine service status.
type HTTPGetProber interface {
	// Probe executes an HTTP GET request to determine service status.
	//
	// Any non-successful status code (< 200 or >= 400) or HTTP/network communication error will return Failure.
	// A potentially truncated version of the HTTP response body is returned as output.
	Probe(ctx context.Context, url *url.URL, headers http.Header) (Result, string, error)
}

type httpProber struct {
	transport *http.Transport
}

// Probe executes an HTTP GET request to determine service status.
func (pr httpProber) Probe(ctx context.Context, url *url.URL, headers http.Header) (Result, string, error) {
	pr.transport.DisableCompression = true // removes Accept-Encoding header
	client := &http.Client{
		Transport:     pr.transport,
		CheckRedirect: redirectChecker(),
	}
	return doHTTPProbe(ctx, url, headers, client)
}

// httpClient is an interface for making HTTP requests that returns a response and error.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// doHTTPProbe checks if a GET request to the url succeeds.
// If the HTTP response code is successful (i.e. 400 > code >= 200), it returns Success.
// If the HTTP response code is unsuccessful or HTTP communication fails, it returns Failure.
func doHTTPProbe(ctx context.Context, url *url.URL, headers http.Header, client httpClient) (Result, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		// Convert errors into failures to catch timeouts.
		return Failure, err.Error(), nil
	}
	if _, ok := headers["User-Agent"]; !ok {
		if headers == nil {
			headers = http.Header{}
		}
		// TODO(milas): add version support to package
		// explicitly set User-Agent so it's not set to default Go value
		headers.Set("User-Agent", fmt.Sprintf("tilt-probe/%s.%s", "0", "1"))
	}
	if _, ok := headers["Accept"]; !ok {
		// Accept header was not defined. accept all
		headers.Set("Accept", "*/*")
	} else if headers.Get("Accept") == "" {
		// Accept header was overridden but is empty. removing
		headers.Del("Accept")
	}
	req.Header = headers
	req.Host = headers.Get("Host")
	res, err := client.Do(req)
	if err != nil {
		// Convert errors into failures to catch timeouts.
		return Failure, err.Error(), nil
	}
	defer res.Body.Close()
	b, err := readAtMost(res.Body, maxRespBodyLength)
	if err != nil {
		if err == ErrLimitReached {
			klog.V(4).Infof("Non fatal body truncation for %s, Response: %v", url.String(), *res)
		} else {
			return Failure, "", err
		}
	}
	body := string(b)
	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest {
		if res.StatusCode >= http.StatusMultipleChoices { // Redirect
			klog.V(4).Infof("Probe terminated redirects for %s, Response: %v", url.String(), *res)
			return Warning, body, nil
		}
		klog.V(4).Infof("Probe succeeded for %s, Response: %v", url.String(), *res)
		return Success, body, nil
	}
	klog.V(4).Infof("Probe failed for %s with request headers %v, response body: %v", url.String(), headers, body)
	return Failure, fmt.Sprintf("HTTP probe failed with statuscode: %d", res.StatusCode), nil
}

func redirectChecker() func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Hostname() != via[0].URL.Hostname() {
			return http.ErrUseLastResponse
		}
		// Default behavior: stop after 10 redirects.
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

// readAtMost reads up to `limit` bytes from `r`, and reports an error
// when `limit` bytes are read.
func readAtMost(r io.Reader, limit int64) ([]byte, error) {
	limitedReader := &io.LimitedReader{R: r, N: limit}
	data, err := ioutil.ReadAll(limitedReader)
	if err != nil {
		return data, err
	}
	if limitedReader.N <= 0 {
		return data, ErrLimitReached
	}
	return data, nil
}

package httpwares

import "net/http"

// RoundTripperFunc wraps a func to make it into a http.RoundTripper. Similar to http.HandleFunc.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Tripperware is a signature for all http client-side middleware.
type Tripperware func(http.RoundTripper) http.RoundTripper

// WrapClient takes an http.Client and wraps its transport in the chain of tripperwares.
func WrapClient(client *http.Client, wares ...Tripperware) *http.Client {
	if len(wares) == 0 {
		return client
	}

	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	for i := len(wares) - 1; i >= 0; i-- {
		transport = wares[i](transport)
	}

	clone := *client
	clone.Transport = transport
	return &clone
}

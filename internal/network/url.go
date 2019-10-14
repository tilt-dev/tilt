package network

import "net/url"

// Given a host (hostname + port), return a URL with a scheme:
// http if this is localhost
// https otherwise
func HostToURL(host string) *url.URL {
	var u url.URL
	u.Host = host
	u.Scheme = "https"
	if u.Hostname() == Localhost {
		u.Scheme = "http"
	}
	return &u
}

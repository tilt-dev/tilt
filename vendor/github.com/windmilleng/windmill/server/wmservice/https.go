package wmservice

import (
	"net/http"
)

// We want to redirect all http requests to https.
// This is a standard loadbalancer feature that ingress-gce doesn't support yet.
// For now, they advise us to implement the redirect ourselves.
// https://github.com/kubernetes/ingress-gce#redirecting-http-to-https
type httpsRedirector struct {
	delegate http.Handler
}

func ForceHTTPS(h http.Handler) httpsRedirector {
	return httpsRedirector{delegate: h}
}

func (h httpsRedirector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	protocol := r.Header.Get("X-Forwarded-Proto")
	host := r.Host
	if protocol == "http" && host != "" {
		url := *r.URL
		url.Host = host
		url.Scheme = "https"
		http.Redirect(w, r, url.String(), http.StatusTemporaryRedirect)
		return
	}
	h.delegate.ServeHTTP(w, r)
}

var _ http.Handler = httpsRedirector{}

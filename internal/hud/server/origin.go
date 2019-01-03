package server

import (
	"net/http"
	"net/url"
	"unicode/utf8"
)

// Code forked from
// https://github.com/gorilla/websocket
// for doing cross-domain origin checks to work with our dev server.

// checkSameOrigin returns true if the origin is
// 1) not set or
// 2) is equal to the request host or
// 3) is equal to the dev server host
func checkWebsocketOrigin(r *http.Request) bool {
	origin := r.Header["Origin"]
	if len(origin) == 0 {
		return true
	}

	u, err := url.Parse(origin[0])
	if err != nil {
		return false
	}

	dev, err := url.Parse(devServerOrigin)
	if err != nil {
		return false
	}

	return equalASCIIFold(u.Host, dev.Host) || equalASCIIFold(u.Host, r.Host)
}

// equalASCIIFold returns true if s is equal to t with ASCII case folding as
// defined in RFC 4790.
func equalASCIIFold(s, t string) bool {
	for s != "" && t != "" {
		sr, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		tr, size := utf8.DecodeRuneInString(t)
		t = t[size:]
		if sr == tr {
			continue
		}
		if 'A' <= sr && sr <= 'Z' {
			sr = sr + 'a' - 'A'
		}
		if 'A' <= tr && tr <= 'Z' {
			tr = tr + 'a' - 'A'
		}
		if sr != tr {
			return false
		}
	}
	return s == t
}

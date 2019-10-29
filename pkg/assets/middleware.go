package assets

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/windmilleng/tilt/pkg/model"
)

// Middleware that attaches a server at a subpath.
// Modeled after http.StripPrefix, but attaches the work it did to the Request Context.
func StripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" {
		return h
	}

	delegate := http.StripPrefix(prefix, h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := strings.TrimPrefix(r.URL.Path, prefix); len(p) < len(r.URL.Path) {
			// If the prefix was stripped successfully, add it to the context.
			r = appendPublicPathPrefix(prefix, r)
		}
		delegate.ServeHTTP(w, r)
	})
}

// Middleware that injects version information into the request.
// We rewrite the URL to contain the version.
//
// If no default version is passed, and we can't find the version in the url path,
// we will write a 500 error.
func InferVersion(defaultVersion model.WebVersion, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origPath := r.URL.Path

		if matches := versionRe.FindStringSubmatch(origPath); len(matches) > 1 {
			h.ServeHTTP(w, appendPublicPathPrefixForVersion(matches[1], r))
			return
		}
		if matches := shaRe.FindStringSubmatch(origPath); len(matches) > 1 {
			h.ServeHTTP(w, appendPublicPathPrefixForVersion(matches[1], r))
			return
		}

		if !(strings.HasPrefix(origPath, "/static/")) {
			// redirect everything else to the main entry point.
			origPath = "index.html"
		}

		version := r.URL.Query().Get(WebVersionKey)
		if version == "" {
			version = string(defaultVersion)
		}

		if version == "" {
			http.Error(w, fmt.Sprintf("Asset version not found: %s", r.URL.String()),
				http.StatusInternalServerError)
			return
		}

		// Stanza for proxying a request (see http.StripPrefix)
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = path.Join(string(version), origPath)
		h.ServeHTTP(w, appendPublicPathPrefixForVersion(version, r2))
	})
}

func appendPublicPathPrefixForVersion(version string, r *http.Request) *http.Request {
	return appendPublicPathPrefix(fmt.Sprintf("/%s", version), r)
}

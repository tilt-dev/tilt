package assets

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/tilt-dev/tilt/pkg/model"
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

func isAssetPath(path string) bool {
	return strings.HasPrefix(path, "/static/") || path == "/favicon.ico"
}

// Middleware that injects version information into the request.
// We rewrite the URL to contain the version.
//
// If no default version is passed, and we can't find the version in the url path,
// we will write a 500 error.
func InferVersion(defaultVersion model.WebVersion, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origPath := r.URL.Path
		w = cacheAssets(w, origPath, r.Method)

		if matches := versionRe.FindStringSubmatch(origPath); len(matches) > 1 {
			h.ServeHTTP(w, appendPublicPathPrefixForVersion(matches[1], r))
			return
		}
		if matches := shaRe.FindStringSubmatch(origPath); len(matches) > 1 {
			h.ServeHTTP(w, appendPublicPathPrefixForVersion(matches[1], r))
			return
		}

		if !isAssetPath(origPath) {
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
		r2.URL.Path = path.Join(version, origPath)
		h.ServeHTTP(w, appendPublicPathPrefixForVersion(version, r2))
	})
}

func appendPublicPathPrefixForVersion(version string, r *http.Request) *http.Request {
	return appendPublicPathPrefix(fmt.Sprintf("/%s", version), r)
}

type cacheWriter struct {
	writer               http.ResponseWriter
	assetPath, reqMethod string
}

func (w cacheWriter) Header() http.Header {
	return w.writer.Header()
}

func (w cacheWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

func (w cacheWriter) WriteHeader(statusCode int) {
	if statusCode == 200 && w.reqMethod == http.MethodGet {
		// Set caching headers according to this doc:
		// https://create-react-app.dev/docs/production-build/#static-file-caching
		//
		// Static artifacts are checksummed and can be cached indefinitely
		// The main index html page should never be cached.
		cacheControl := "no-store, max-age=0"
		if isAssetPath(w.assetPath) {
			cacheControl = "public, max-age=31536000"
		}
		w.writer.Header().Set("Cache-Control", cacheControl)
	}
	w.writer.WriteHeader(statusCode)
}

func cacheAssets(w http.ResponseWriter, assetPath, reqMethod string) http.ResponseWriter {
	return cacheWriter{
		writer:    w,
		assetPath: assetPath,
		reqMethod: reqMethod,
	}
}

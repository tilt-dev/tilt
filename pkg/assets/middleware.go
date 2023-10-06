package assets

import (
	"net/http"
	"strings"
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
	return strings.HasPrefix(path, "/static/") || path == "/favicon.ico" || path == "/manifest.json"
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

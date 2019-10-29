package assets

import (
	"context"
	"net/http"
)

type PublicPathPrefixContextKey struct{}

func getPublicPathPrefix(r *http.Request) string {
	val := r.Context().Value(PublicPathPrefixContextKey{})
	s, _ := val.(string)
	return s
}

func appendPublicPathPrefix(prefix string, r *http.Request) *http.Request {
	key := PublicPathPrefixContextKey{}
	existingPrefix := getPublicPathPrefix(r)
	return r.WithContext(context.WithValue(r.Context(), key, existingPrefix+prefix))
}

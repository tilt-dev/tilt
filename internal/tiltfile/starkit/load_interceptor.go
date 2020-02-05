package starkit

import (
	"go.starlark.net/starlark"
)

// LoadInterceptor allows an Extension to intercept a load to set the contents based on the requested path.
type LoadInterceptor interface {
	// LocalPath returns the path that the Tiltfile code should be read from.
	// Must be stable, because it's used as a cache key
	// Ensure the content is present in the path returned
	// Returns "" if this interceptor doesn't act on this path
	LocalPath(t *starlark.Thread, path string) (string, error)
}

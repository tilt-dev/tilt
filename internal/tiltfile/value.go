package tiltfile

import "go.starlark.net/starlark"

// Wrapper around starlark.AsString
func AsString(x starlark.Value) (string, bool) {
	b, ok := x.(*blob)
	if ok {
		return b.text, true
	}
	return starlark.AsString(x)
}

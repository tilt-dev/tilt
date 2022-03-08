package tiltfile

import (
	"embed"
	"io/fs"
)

//go:embed api/*.py api/config/*.py api/os/*.py api/shlex/*.py api/sys/*.py api/v1alpha1/*.py
var api embed.FS

func ApiStubs() fs.FS {
	return api
}

func WalkApiStubs(fn fs.WalkDirFunc) error {
	return fs.WalkDir(ApiStubs(), "api", fn)
}

package tiltfile

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed api/*.py api/*/*.py
var api embed.FS

func ApiStubs() fs.FS {
	return api
}

func WalkApiStubs(fn fs.WalkDirFunc) error {
	return fs.WalkDir(ApiStubs(), "api", fn)
}

func DumpApiStubs(dir string, callback func(string, error)) error {
	return WalkApiStubs(func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		var err error
		dest := filepath.Join(dir, path)
		if d.IsDir() {
			err = os.MkdirAll(dest, 0755)
		} else {
			var bytes []byte
			bytes, err = api.ReadFile(path)
			if err != nil {
				return err
			}
			err = os.WriteFile(dest, bytes, 0644)
		}
		callback(path, err)
		return err
	})
}

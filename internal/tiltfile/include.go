package tiltfile

import (
	"fmt"
	"path/filepath"

	"go.starlark.net/starlark"
)

type loadCacheEntry struct {
	status  loadStatus
	exports starlark.StringDict
	err     error
}

type loadStatus int

const (
	loadStatusNone loadStatus = iota
	loadStatusExecuting
	loadStatusDone
)

// Tiltfiles support both load() and include().
//
// The main difference is that include() doesn't bind any arguments into the
// global scope, whereas load() forces you to bind at least one argument into the global
// scope (i.e., you can't load() a Tilfile for its side-effects).
func (s *tiltfileState) include(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var p string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &p)
	if err != nil {
		return nil, err
	}

	_, err = s.exec(s.absPath(thread, p))
	return starlark.None, err
}

func (s *tiltfileState) exec(tiltfilePath string) (starlark.StringDict, error) {
	if !filepath.IsAbs(tiltfilePath) {
		return starlark.StringDict{}, fmt.Errorf("internal error: tiltfilePath must be absolute")
	}

	entry := s.loadCache[tiltfilePath]
	status := entry.status
	if status == loadStatusExecuting {
		return starlark.StringDict{}, fmt.Errorf("Circular tiltfile load: %s", tiltfilePath)
	} else if status == loadStatusDone {
		return entry.exports, entry.err
	}

	s.loadCache[tiltfilePath] = loadCacheEntry{
		status: loadStatusExecuting,
	}
	s.configFiles = append(s.configFiles, tiltfilePath, tiltIgnorePath(tiltfilePath))

	newT := s.starlarkThread()
	newT.Name = tiltfilePath
	exports, err := starlark.ExecFile(newT, tiltfilePath, nil, s.predeclared())
	s.loadCache[tiltfilePath] = loadCacheEntry{
		status:  loadStatusDone,
		exports: exports,
		err:     err,
	}
	return exports, err
}

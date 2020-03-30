package starkit

import (
	"path/filepath"

	"go.starlark.net/starlark"
)

// We want to resolve paths relative to the dir where the currently executing file lives,
// not relative to the working directory.
func AbsPath(t *starlark.Thread, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(AbsWorkingDir(t), path)
}

func AbsWorkingDir(t *starlark.Thread) string {
	return filepath.Dir(CurrentExecPath(t))
}

// Path to the file that's currently executing
func CurrentExecPath(t *starlark.Thread) string {
	ret := t.Local(execingTiltfileKey)
	if ret == nil {
		panic("internal error: currentExecPath must be called from an active starlark thread")
	}
	return ret.(string)
}

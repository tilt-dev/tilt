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

// Path to the file at the bottom of the call stack.
func CurrentExecPath(t *starlark.Thread) string {
	depth := t.CallStackDepth()
	for i := 0; i < depth; i++ {
		filename := t.CallFrame(i).Pos.Filename()
		if filename == "<builtin>" {
			continue
		}
		return filename
	}
	panic("internal error: currentExecPath must be called from an active starlark thread")
}

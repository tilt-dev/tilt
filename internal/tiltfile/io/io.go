package io

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

type WatchType int

const (
	// If it's a file, only watch the file. If it's a directory, don't watch at all.
	WatchFileOnly WatchType = iota

	// If it's a file, only watch the file. If it's a directory, watch it recursively.
	WatchRecursive
)

type Plugin struct{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (Plugin) NewState() interface{} {
	return ReadState{}
}

func (Plugin) OnStart(e *starkit.Environment) error {
	err := e.AddBuiltin("read_file", readFile)
	if err != nil {
		return err
	}

	err = e.AddBuiltin("watch_file", watchFile)
	if err != nil {
		return err
	}

	err = e.AddBuiltin("listdir", listdir)
	if err != nil {
		return err
	}

	err = e.AddBuiltin("blob", blob)
	if err != nil {
		return err
	}

	return nil
}

func (Plugin) OnExec(t *starlark.Thread, tiltfilePath string, contents []byte) error {
	return RecordReadPath(t, WatchFileOnly, tiltfilePath)
}

func readFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path := value.NewLocalPathUnpacker(thread)
	var defaultReturnValue value.Optional[starlark.String]
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path, "default?", &defaultReturnValue)
	if err != nil {
		return nil, err
	}

	p := path.Value
	bs, err := ReadFile(thread, p)
	if os.IsNotExist(err) && defaultReturnValue.IsSet {
		bs = []byte(defaultReturnValue.Value)
	} else if err != nil {
		return nil, err
	}

	return NewBlob(string(bs), fmt.Sprintf("file: %s", p)), nil
}

func watchFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path := value.NewLocalPathUnpacker(thread)
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	p := path.Value
	err = RecordReadPath(thread, WatchRecursive, p)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func listdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	dir := value.NewLocalPathUnpacker(thread)
	var recursive bool
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "dir", &dir, "recursive?", &recursive)
	if err != nil {
		return nil, err
	}

	localPath := dir.Value

	// We currently don't watch the directory only, because Tilt doesn't have any
	// way to watch a directory without watching it recursively.
	if recursive {
		err = RecordReadPath(thread, WatchRecursive, localPath)
		if err != nil {
			return nil, err
		}
	}

	var files []string
	err = filepath.WalkDir(localPath, func(path string, info fs.DirEntry, err error) error {
		if path == localPath {
			return nil
		}
		if !info.IsDir() {
			files = append(files, path)
		} else if info.IsDir() && !recursive {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var ret []starlark.Value
	for _, f := range files {
		ret = append(ret, starlark.String(f))
	}

	return starlark.NewList(ret), nil
}

func blob(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var input starlark.String
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "input", &input)
	if err != nil {
		return nil, err
	}

	return NewBlob(input.GoString(), "Tiltfile blob() call"), nil
}

// Track all the paths read while loading
type ReadState struct {
	Paths []string
}

func ReadFile(thread *starlark.Thread, p string) ([]byte, error) {
	err := RecordReadPath(thread, WatchFileOnly, p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func RecordReadPath(t *starlark.Thread, wt WatchType, files ...string) error {
	toWatch := make([]string, 0, len(files))
	for _, f := range files {
		switch wt {
		case WatchRecursive:
			toWatch = append(toWatch, f)

		case WatchFileOnly:
			info, err := os.Lstat(f)
			shouldWatch := false
			if os.IsNotExist(err) {
				// If a file does not exist, we should watch the space
				// to see if the file does appear.
				shouldWatch = true
			} else if err != nil {
				// If we got a permission denied error, we should stop.
				return err
			} else if !info.IsDir() {
				// Tilt only knows how to do recursive watches. If we read a directory
				// during Tiltfile execution, we'd rather not watch the directory at all
				// rather than overwatch and over-trigger Tiltfile reloads.
				//
				// https://github.com/tilt-dev/tilt/issues/3387
				shouldWatch = true
			}

			if shouldWatch {
				toWatch = append(toWatch, f)
			}

		default:
			return fmt.Errorf("Unknown watch type: %v", t)
		}
	}

	err := starkit.SetState(t, func(s ReadState) ReadState {
		s.Paths = sliceutils.AppendWithoutDupes(s.Paths, toWatch...)
		return s
	})
	return errors.Wrap(err, "error recording read file")
}

var _ starkit.StatefulPlugin = Plugin{}
var _ starkit.OnExecPlugin = Plugin{}

func MustState(model starkit.Model) ReadState {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (ReadState, error) {
	var state ReadState
	err := m.Load(&state)
	return state, err
}

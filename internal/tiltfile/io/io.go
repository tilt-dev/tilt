package io

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

func (Extension) NewState() interface{} {
	return ReadState{}
}

func (Extension) OnStart(e *starkit.Environment) error {
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

func (Extension) OnExec(t *starlark.Thread, tiltfilePath string) error {
	return RecordReadFile(t, tiltfilePath)
}

func readFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	// the type of default is Union[None, Str], so that we can distinguish unspecified from the empty string
	// which means we can't simply lean on Unpack args for type-checking
	var defaultReturnValue starlark.Value = starlark.None
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path, "default?", &defaultReturnValue)
	if err != nil {
		return nil, err
	}

	var defaultReturn starlark.String
	if defaultReturnValue != starlark.None {
		var ok bool
		defaultReturn, ok = defaultReturnValue.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("default must be %T or %T. got %T", starlark.None, starlark.String(""), defaultReturnValue)
		}
	}

	p, err := value.ValueToAbsPath(thread, path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for paths: %v", err)
	}

	bs, err := ReadFile(thread, p)
	if os.IsNotExist(err) && defaultReturnValue != starlark.None {
		bs = []byte(defaultReturn.GoString())
	} else if err != nil {
		return nil, err
	}

	return NewBlob(string(bs), fmt.Sprintf("file: %s", p)), nil
}

func watchFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.Value
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	p, err := value.ValueToAbsPath(thread, path)
	if err != nil {
		return nil, fmt.Errorf("invalid type for paths: %v", err)
	}

	err = RecordReadFile(thread, p)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func listdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dir starlark.String
	var recursive bool
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "dir", &dir, "recursive?", &recursive)
	if err != nil {
		return nil, err
	}

	localPath, err := value.ValueToAbsPath(thread, dir)
	if err != nil {
		return nil, fmt.Errorf("Argument 0 (paths): %v", err)
	}

	err = RecordReadFile(thread, localPath)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
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

// Track all the files read while loading
type ReadState struct {
	Files []string
}

func ReadFile(thread *starlark.Thread, p string) ([]byte, error) {
	err := RecordReadFile(thread, p)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(p)
}

func RecordReadFile(t *starlark.Thread, files ...string) error {
	err := starkit.SetState(t, func(s ReadState) ReadState {
		s.Files = sliceutils.AppendWithoutDupes(s.Files, files...)
		return s
	})
	return errors.Wrap(err, "error recording read file")
}

var _ starkit.StatefulExtension = Extension{}
var _ starkit.OnExecExtension = Extension{}

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

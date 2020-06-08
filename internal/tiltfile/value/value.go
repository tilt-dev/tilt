package value

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

// If `v` is a `starlark.Sequence`, return a slice of its elements
// Otherwise, return it as a single-element slice
// For functions that take `Union[List[T], T]`
func ValueOrSequenceToSlice(v starlark.Value) []starlark.Value {
	if seq, ok := v.(starlark.Sequence); ok {
		var ret []starlark.Value
		it := seq.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			ret = append(ret, i)
		}
		return ret
	} else if v == nil || v == starlark.None {
		return nil
	} else {
		return []starlark.Value{v}
	}
}

func ValueToStringMap(v starlark.Value) (map[string]string, error) {
	var result map[string]string
	if v != nil && v != starlark.None {
		d, ok := v.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("expected dict, got %T", v)
		}

		var err error
		result, err = skylarkStringDictToGoMap(d)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func skylarkStringDictToGoMap(d *starlark.Dict) (map[string]string, error) {
	r := map[string]string{}

	for _, tuple := range d.Items() {
		kV, ok := AsString(tuple[0])
		if !ok {
			return nil, fmt.Errorf("key is not a string: %T (%v)", tuple[0], tuple[0])
		}

		k := string(kV)

		vV, ok := AsString(tuple[1])
		if !ok {
			return nil, fmt.Errorf("value is not a string: %T (%v)", tuple[1], tuple[1])
		}

		v := string(vV)

		r[k] = v
	}

	return r, nil
}

func ValueToAbsPath(thread *starlark.Thread, v starlark.Value) (string, error) {
	pathMaker, ok := v.(PathMaker)
	if ok {
		return pathMaker.MakeLocalPath("."), nil
	}

	str, ok := v.(starlark.String)
	if ok {
		return starkit.AbsPath(thread, string(str)), nil
	}

	return "", fmt.Errorf("expected path | string. Actual type: %T", v)
}

type PathMaker interface {
	MakeLocalPath(relPath string) string
}

func SequenceToStringSlice(seq starlark.Sequence) ([]string, error) {
	if seq == nil {
		return nil, nil
	}
	it := seq.Iterate()
	defer it.Done()
	var ret []string
	var v starlark.Value
	for it.Next(&v) {
		s, ok := v.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("'%v' is a %T, not a string", v, v)
		}
		ret = append(ret, string(s))
	}
	return ret, nil
}

func StringSliceToList(slice []string) *starlark.List {
	v := []starlark.Value{}
	for _, s := range slice {
		v = append(v, starlark.String(s))
	}
	return starlark.NewList(v)
}

// In other similar build systems (Buck and Bazel),
// there's a "main" command, and then various per-platform overrides.
// https://docs.bazel.build/versions/master/be/general.html#genrule.cmd_bat
// This helper function abstracts out the precedence rules.
func ValueGroupToCmdHelper(cmdVal, cmdBatVal starlark.Value) (model.Cmd, error) {
	if cmdBatVal != nil && runtime.GOOS == "windows" {
		return ValueToBatCmd(cmdBatVal)
	}
	return ValueToHostCmd(cmdVal)
}

// provides dockerfile-style behavior of:
// a string gets interpreted as a shell command (like, sh -c 'foo bar $X')
// an array of strings gets interpreted as a raw argv to exec
func ValueToHostCmd(v starlark.Value) (model.Cmd, error) {
	return valueToCmdHelper(v, model.ToHostCmd)
}

func ValueToBatCmd(v starlark.Value) (model.Cmd, error) {
	return valueToCmdHelper(v, model.ToBatCmd)
}

func ValueToUnixCmd(v starlark.Value) (model.Cmd, error) {
	return valueToCmdHelper(v, model.ToUnixCmd)
}

func valueToCmdHelper(v starlark.Value, stringToCmd func(string) model.Cmd) (model.Cmd, error) {
	switch x := v.(type) {
	// If a starlark function takes an optional command argument, then UnpackArgs will set its starlark.Value to nil
	// we convert nils here to an empty Cmd, since otherwise every callsite would have to do a nil check with presumably
	// the same outcome
	case nil:
		return model.Cmd{}, nil
	case starlark.String:
		return stringToCmd(string(x)), nil
	case starlark.Sequence:
		argv, err := SequenceToStringSlice(x)
		if err != nil {
			return model.Cmd{}, errors.Wrap(err, "a command must be a string or a list of strings")
		}
		return model.Cmd{Argv: argv}, nil
	default:
		return model.Cmd{}, fmt.Errorf("a command must be a string or list of strings. found %T", x)
	}
}

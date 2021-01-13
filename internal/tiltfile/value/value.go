package value

import (
	"fmt"
	"runtime"
	"sort"

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

func ValueToAbsPath(thread *starlark.Thread, v starlark.Value) (string, error) {
	pathMaker, ok := v.(PathMaker)
	if ok {
		return pathMaker.MakeLocalPath("."), nil
	}

	str, ok := starlark.AsString(v)
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
func ValueGroupToCmdHelper(t *starlark.Thread, cmdVal, cmdBatVal starlark.Value, env map[string]string) (model.Cmd, error) {
	if cmdBatVal != nil && runtime.GOOS == "windows" {
		return ValueToBatCmd(t, cmdBatVal, env)
	}
	return ValueToHostCmd(t, cmdVal, env)
}

// provides dockerfile-style behavior of:
// a string gets interpreted as a shell command (like, sh -c 'foo bar $X')
// an array of strings gets interpreted as a raw argv to exec
func ValueToHostCmd(t *starlark.Thread, v starlark.Value, env map[string]string) (model.Cmd, error) {
	return valueToCmdHelper(t, v, env, model.ToHostCmd)
}

func ValueToBatCmd(t *starlark.Thread, v starlark.Value, env map[string]string) (model.Cmd, error) {
	return valueToCmdHelper(t, v, env, model.ToBatCmd)
}

func ValueToUnixCmd(t *starlark.Thread, v starlark.Value, env map[string]string) (model.Cmd, error) {
	return valueToCmdHelper(t, v, env, model.ToUnixCmd)
}

func valueToCmdHelper(t *starlark.Thread, cmdVal starlark.Value, cmdEnv map[string]string, stringToCmd func(string) model.Cmd) (model.Cmd, error) {
	dir := starkit.AbsWorkingDir(t)
	env, err := envTuples(cmdEnv)
	if err != nil {
		return model.Cmd{}, err
	}

	switch x := cmdVal.(type) {
	// If a starlark function takes an optional command argument, then UnpackArgs will set its starlark.Value to nil
	// we convert nils here to an empty Cmd, since otherwise every callsite would have to do a nil check with presumably
	// the same outcome
	case nil:
		return model.Cmd{}, nil
	case starlark.String:
		cmd := stringToCmd(string(x))
		cmd.Dir = dir
		cmd.Env = env
		return cmd, nil
	case starlark.Sequence:
		argv, err := SequenceToStringSlice(x)
		if err != nil {
			return model.Cmd{}, errors.Wrap(err, "a command must be a string or a list of strings")
		}
		return model.Cmd{Argv: argv, Dir: dir, Env: env}, nil
	default:
		return model.Cmd{}, fmt.Errorf("a command must be a string or list of strings. found %T", x)
	}
}

func envTuples(env map[string]string) ([]string, error) {
	var kv []string
	for k, v := range env {
		if k == "" {
			return nil, fmt.Errorf("environment has empty key for value %q", v)
		}
		kv = append(kv, k+"="+v)
	}
	// sorting here is for consistency/predictability; since the input is a map, uniqueness
	// is guaranteed so order is not actually relevant, but this simplifies usage in tests,
	// for example
	sort.Strings(kv)
	return kv, nil
}

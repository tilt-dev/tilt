package value

import (
	"fmt"
	"math"
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
		return starkit.AbsPath(thread, str), nil
	}

	return "", fmt.Errorf("expected path | string. Actual type: %T", v)
}

type PathMaker interface {
	MakeLocalPath(relPath string) string
}

// StringSequence is a convenience type for dealing with string slices in Starlark.
type StringSequence []string

func (s StringSequence) Sequence() starlark.Sequence {
	elems := make([]starlark.Value, 0, len(s))
	for _, v := range s {
		elems = append(elems, starlark.String(v))
	}
	return starlark.NewList(elems)
}

func (s *StringSequence) Unpack(v starlark.Value) error {
	if v == nil {
		*s = nil
		return nil
	}
	seq, ok := v.(starlark.Sequence)
	if !ok {
		return fmt.Errorf("'%v' is a %T, not a sequence", v, v)
	}
	out, err := SequenceToStringSlice(seq)
	if err != nil {
		return err
	}
	*s = out
	return nil
}

func SequenceToStringSlice(seq starlark.Sequence) ([]string, error) {
	if seq == nil || seq.Len() == 0 {
		return nil, nil
	}
	it := seq.Iterate()
	defer it.Done()
	ret := make([]string, 0, seq.Len())
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
func ValueGroupToCmdHelper(t *starlark.Thread, cmdVal, cmdBatVal, cmdDir starlark.Value, env map[string]string) (model.Cmd, error) {
	if cmdBatVal != nil && runtime.GOOS == "windows" {
		return ValueToBatCmd(t, cmdBatVal, cmdDir, env)
	}
	return ValueToHostCmd(t, cmdVal, cmdDir, env)
}

// provides dockerfile-style behavior of:
// a string gets interpreted as a shell command (like, sh -c 'foo bar $X')
// an array of strings gets interpreted as a raw argv to exec
func ValueToHostCmd(t *starlark.Thread, v, dir starlark.Value, env map[string]string) (model.Cmd, error) {
	return valueToCmdHelper(t, v, dir, env, model.ToHostCmd)
}

func ValueToBatCmd(t *starlark.Thread, v, dir starlark.Value, env map[string]string) (model.Cmd, error) {
	return valueToCmdHelper(t, v, dir, env, model.ToBatCmd)
}

func ValueToUnixCmd(t *starlark.Thread, v, dir starlark.Value, env map[string]string) (model.Cmd, error) {
	return valueToCmdHelper(t, v, dir, env, model.ToUnixCmd)
}

func valueToCmdHelper(t *starlark.Thread, cmdVal, cmdDirVal starlark.Value, cmdEnv map[string]string, stringToCmd func(string) model.Cmd) (model.Cmd, error) {

	var dir string
	var dirErr error

	switch cmdDirVal.(type) {
	case nil:
		dir = starkit.AbsWorkingDir(t)
	case starlark.NoneType:
		dir = starkit.AbsWorkingDir(t)
	default:
		dir, dirErr = ValueToAbsPath(t, cmdDirVal)
		if dirErr != nil {
			return model.Cmd{}, errors.Wrap(dirErr, "a command directory must be empty or a string")
		}
	}

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

// Int32 is a convenience type for unpacking int32 bounded values.
type Int32 struct {
	starlark.Int
}

// Int32 returns the value as an int32.
//
// It will panic if the value cannot be accurate represented as an int32.
func (i Int32) Int32() int32 {
	v, err := starlarkIntAsInt32(i.Int)
	if err != nil {
		// bounds check should have happened during unpacking, so something
		// is very wrong if we get here
		panic(err)
	}
	return v
}

func (i *Int32) Unpack(v starlark.Value) error {
	if v == nil {
		return fmt.Errorf("got %s, want int", starlark.None.Type())
	}
	x, ok := v.(starlark.Int)
	if !ok {
		return fmt.Errorf("got %s, want int", v.Type())
	}
	if _, err := starlarkIntAsInt32(x); err != nil {
		return err
	}
	i.Int = x
	return nil
}

func starlarkIntAsInt32(v starlark.Int) (int32, error) {
	x, ok := v.Int64()
	if !ok || x < math.MinInt32 || x > math.MaxInt32 {
		return 0, fmt.Errorf("value out of range for int32: %s", v.String())
	}
	return int32(x), nil
}

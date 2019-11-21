package value

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
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

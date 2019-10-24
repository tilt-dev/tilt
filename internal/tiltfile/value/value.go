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

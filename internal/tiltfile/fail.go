package tiltfile

import (
	"fmt"

	"go.starlark.net/starlark"
)

func (s *tiltfileState) fail(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	err := s.unpackArgs(fn.Name(), args, kwargs, "msg", &msg)
	if err != nil {
		return nil, err
	}

	return nil, fmt.Errorf(msg)
}

package tiltfile

import (
	"go.starlark.net/starlark"
)

func (s *tiltfileState) enableFeature(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var flag string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "msg", &flag)
	if err != nil {
		return nil, err
	}

	err = s.f.Enable(flag)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func (s *tiltfileState) disableFeature(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var flag string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "msg", &flag)
	if err != nil {
		return nil, err
	}

	err = s.f.Disable(flag)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

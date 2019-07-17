package tiltfile

import (
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/feature"
)

func (s *tiltfileState) enableFeature(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var flag string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "msg", &flag)
	if err != nil {
		return nil, err
	}

	err = s.features.Set(flag, true)
	if err != nil {
		if _, ok := err.(feature.ObsoleteError); !ok {
			return nil, err
		}
		s.warnings = append(s.warnings, err.Error())
	}

	return starlark.None, nil
}

func (s *tiltfileState) disableFeature(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var flag string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "msg", &flag)
	if err != nil {
		return nil, err
	}

	err = s.features.Set(flag, false)
	if err != nil {
		if _, ok := err.(feature.ObsoleteError); !ok {
			return nil, err
		}
		s.warnings = append(s.warnings, err.Error())
	}

	return starlark.None, nil
}

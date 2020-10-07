package tiltfile

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

func (s *tiltfileState) link(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var url, name string

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"url", &url,
		"name?", &name); err != nil {
		return nil, err
	}

	return value.Link{
		Link: model.Link{
			URL:  url,
			Name: name,
		},
	}, nil
}

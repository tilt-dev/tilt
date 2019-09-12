package tiltfile

import (
	"github.com/windmilleng/tilt/pkg/model"
	"go.starlark.net/starlark"
)

type localResource struct {
	name        string
	cmd         model.Cmd
	deps        []string
	triggerMode triggerMode
}

func (s *tiltfileState) localResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var cmd, deps, triggerMode starlark.Value

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"cmd", &cmd,
		"deps?", &deps,
		"trigger_mode?", &triggerMode,
	); err != nil {
		return nil, err
	}

	// ✨ TODO ✨: unpack args...
	res := localResource{}
	s.localResources = append(s.localResources, res)

	return starlark.None, nil
}

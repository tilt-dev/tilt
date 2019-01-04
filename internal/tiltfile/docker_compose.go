package tiltfile

import (
	"fmt"
	"reflect"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"go.starlark.net/starlark"
)

// dcResource represents a single docker-compose config file and all its associated services
type dcResource struct {
	configPath string

	services []dockercompose.Service
}

func (dc dcResource) Empty() bool { return reflect.DeepEqual(dc, dcResource{}) }

func (s *tiltfileState) dockerCompose(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var configPath string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "configPath", &configPath)
	if err != nil {
		return nil, err
	}
	configPath = s.absPath(configPath)
	if err != nil {
		return nil, err
	}

	services, err := dockercompose.ParseConfig(s.ctx, configPath)
	if err != nil {
		return nil, err
	}

	if !s.dc.Empty() {
		return starlark.None, fmt.Errorf("already have a docker-compose resource declared (%s), cannot declare another (%s)", s.dc.configPath, configPath)
	}

	s.dc = dcResource{configPath: configPath, services: services}

	return starlark.None, nil
}

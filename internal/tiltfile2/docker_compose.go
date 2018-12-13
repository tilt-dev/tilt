package tiltfile2

import (
	"fmt"

	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/dockercompose"
)

// dcResource represents a single docker-compose config file and all its associated services
type dcResource struct {
	yamlPath string

	services []dockercompose.Service
}

func (s *tiltfileState) dockerCompose(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var path skylark.String
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	// ~~ i still don't understand why path.String() returns it quoted / if there's a more idiomatic way
	yamlPath, ok := skylark.AsString(path)
	if !ok {
		return nil, fmt.Errorf("couldn't AsString skylark.String: %v", path)
	}

	// TODO: support more than one docker-compose.yaml file
	services, _, err := dockercompose.ParseConfig(s.ctx, []string{yamlPath})
	if err != nil {
		return nil, err
	}

	dc := dcResource{yamlPath: yamlPath, services: services}

	s.dc = append(s.dc, dc)

	return skylark.None, nil
}

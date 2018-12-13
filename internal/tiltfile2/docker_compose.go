package tiltfile2

import (
	"fmt"
	"reflect"

	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/dockercompose"
)

// dcResource represents a single docker-compose config file and all its associated services
type dcResource struct {
	yamlPath string

	services []dockercompose.Service
}

func (dc dcResource) Empty() bool { return reflect.DeepEqual(dc, dcResource{}) }

func (s *tiltfileState) dockerCompose(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var yamlPath string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "yamlPath", &yamlPath)
	if err != nil {
		return nil, err
	}

	services, _, err := dockercompose.ParseConfig(s.ctx, []string{yamlPath})
	if err != nil {
		return nil, err
	}

	if !s.dc.Empty() {
		return skylark.None, fmt.Errorf("already have a docker-compose resource declared (%s), cannot declare another (%s)", s.dc.yamlPath, yamlPath)
	}

	s.dc = dcResource{yamlPath: yamlPath, services: services}

	return skylark.None, nil
}

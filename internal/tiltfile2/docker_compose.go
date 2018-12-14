package tiltfile2

import (
	"fmt"
	"reflect"

	"github.com/google/skylark"
	"github.com/windmilleng/tilt/internal/dockercompose"
)

// dcResource represents a single docker-compose config file and all its associated services
type dcResource struct {
	configPath string

	services []dockercompose.Service
}

func (dc dcResource) Empty() bool { return reflect.DeepEqual(dc, dcResource{}) }

func (s *tiltfileState) dockerCompose(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var configPath string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "configPath", &configPath)
	if err != nil {
		return nil, err
	}

	services, err := dockercompose.ParseConfig(s.ctx, configPath)
	if err != nil {
		return nil, err
	}

	if !s.dc.Empty() {
		return skylark.None, fmt.Errorf("already have a docker-compose resource declared (%s), cannot declare another (%s)", s.dc.configPath, configPath)
	}

	s.dc = dcResource{configPath: configPath, services: services}

	return skylark.None, nil
}

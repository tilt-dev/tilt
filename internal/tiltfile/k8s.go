package tiltfile

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

type k8sResource struct {
	name     string
	k8s      []k8s.K8sEntity
	imageRef string

	portForwards []portForward
}

func (s *tiltfileState) k8sYaml(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var yamlValue starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"yaml", &yamlValue,
	); err != nil {
		return nil, err
	}

	entities, err := s.yamlEntitiesFromSkylarkValueOrList(yamlValue)
	if err != nil {
		return nil, err
	}
	s.k8sUnresourced = append(s.k8sUnresourced, entities...)

	return starlark.None, nil
}

func (s *tiltfileState) k8sResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var yamlValue starlark.Value
	var imageVal starlark.Value
	var portForwardsVal starlark.Value

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"yaml?", &yamlValue,
		"image?", &imageVal,
		"port_forwards?", &portForwardsVal,
	); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("k8s_resource: name must not be empty")
	}
	r, err := s.makeK8sResource(name)
	if err != nil {
		return nil, err
	}

	entities, err := s.yamlEntitiesFromSkylarkValueOrList(yamlValue)
	if err != nil {
		return nil, err
	}

	var imageRef string
	switch imageVal := imageVal.(type) {
	case nil:
		// empty
	case starlark.String:
		imageRef = string(imageVal)
	case *fastBuild:
		imageRef = imageVal.img.ref.Name()
	default:
		return nil, fmt.Errorf("image arg must be a string or fast_build; got %T", imageVal)
	}

	portForwards, err := s.convertPortForwards(name, portForwardsVal)
	if err != nil {
		return nil, err
	}

	r.k8s = entities
	r.imageRef = imageRef
	r.portForwards = portForwards

	return starlark.None, nil
}

func (s *tiltfileState) makeK8sResource(name string) (*k8sResource, error) {
	if s.k8sByName[name] != nil {
		return nil, fmt.Errorf("k8s_resource named %q already exists", name)
	}
	r := &k8sResource{
		name: name,
	}
	s.k8s = append(s.k8s, r)
	s.k8sByName[name] = r

	return r, nil
}

func (s *tiltfileState) yamlEntitiesFromSkylarkValueOrList(v starlark.Value) ([]k8s.K8sEntity, error) {
	if v, ok := v.(starlark.Sequence); ok {
		var result []k8s.K8sEntity
		it := v.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			entities, err := s.yamlEntitiesFromSkylarkValue(i)
			if err != nil {
				return nil, err
			}
			result = append(result, entities...)
		}
		return result, nil
	}
	return s.yamlEntitiesFromSkylarkValue(v)
}

func (s *tiltfileState) yamlEntitiesFromSkylarkValue(v starlark.Value) ([]k8s.K8sEntity, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case *blob:
		return k8s.ParseYAMLFromString(v.String())
	default:
		yamlPath, err := s.localPathFromSkylarkValue(v)
		if err != nil {
			return nil, err
		}
		bs, err := s.readFile(yamlPath)
		if err != nil {
			return nil, err
		}
		entities, err := k8s.ParseYAMLFromString(string(bs))
		if err != nil {
			if strings.Contains(err.Error(), "json parse error: ") {
				return entities, fmt.Errorf("%s is not a valid YAML file: %s", yamlPath.String(), err)
			}
			return entities, err
		}

		return entities, nil
	}
}

func (s *tiltfileState) convertPortForwards(name string, val starlark.Value) ([]portForward, error) {
	if val == nil {
		return nil, nil
	}
	switch val := val.(type) {
	case starlark.Int:
		pf, err := intToPortForward(val)
		if err != nil {
			return nil, err
		}
		return []portForward{pf}, nil
	case portForward:
		return []portForward{val}, nil
	case starlark.Sequence:
		var result []portForward
		it := val.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			switch i := i.(type) {
			case starlark.Int:
				pf, err := intToPortForward(i)
				if err != nil {
					return nil, err
				}
				result = append(result, pf)
			case portForward:
				result = append(result, i)
			default:
				return nil, fmt.Errorf("k8s_resource %q: port_forwards arg %v includes element %v which must be an int or a port_forward; is a %T", name, val, i, i)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("k8s_resource %q: port_forwards must be an int, a port_forward, or a sequence of those; is a %T", name, val)
	}
}

func (s *tiltfileState) portForward(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var local int
	var container int

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "local", &local, "container?", &container); err != nil {
		return nil, err
	}

	return portForward{local: local, container: container}, nil
}

type portForward struct {
	local     int
	container int
}

var _ starlark.Value = portForward{}

func (f portForward) String() string {
	return fmt.Sprintf("port_forward(%d, %d)", f.local, f.container)
}

func (f portForward) Type() string {
	return "port_forward"
}

func (f portForward) Freeze() {}

func (f portForward) Truth() starlark.Bool {
	return f.local != 0 && f.container != 0
}

func (f portForward) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: port_forward")
}

func intToPortForward(i starlark.Int) (portForward, error) {
	n, ok := i.Int64()
	if !ok {
		return portForward{}, fmt.Errorf("portForward value %v is not representable as an int64", i)
	}
	if n < 0 || n > 65535 {
		return portForward{}, fmt.Errorf("portForward value %v is not in the range for a port [0-65535]", n)
	}
	return portForward{local: int(n)}, nil
}

func (s *tiltfileState) portForwardsToDomain(r *k8sResource) []model.PortForward {
	var result []model.PortForward
	for _, pf := range r.portForwards {
		result = append(result, model.PortForward{LocalPort: pf.local, ContainerPort: pf.container})
	}
	return result
}

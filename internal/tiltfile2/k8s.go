package tiltfile2

import (
	"fmt"

	"github.com/google/skylark"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// k8sResource
type k8sResource struct {
	name     string
	k8s      []k8s.K8sEntity
	imageRef string

	portForwards []portForward

	expandedFrom string // this resource was not declared but expanded from another resource
}

func (s *tiltfileState) k8sResource(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var name string
	var yamlValue skylark.Value
	var imageVal skylark.Value
	var portForwardsVal skylark.Value

	if err := skylark.UnpackArgs(fn.Name(), args, kwargs,
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
	if s.k8sByName[name] != nil {
		return nil, fmt.Errorf("resource named %q has already been defined", name)
	}

	yamlText := ""
	if yamlValue == nil {
	} else if b, ok := yamlValue.(*blob); ok {
		yamlText = b.String()
	} else {

		yamlPath, err := s.localPathFromSkylarkValue(yamlValue)
		if err != nil {
			return nil, err
		}

		bs, err := s.readFile(yamlPath)
		if err != nil {
			return nil, err
		}
		yamlText = string(bs)
	}

	entities, err := k8s.ParseYAMLFromString(yamlText)
	if err != nil {
		return nil, err
	}

	var imageRef string
	switch imageVal := imageVal.(type) {
	case nil:
		// empty
	case skylark.String:
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

	r := &k8sResource{
		name:     name,
		k8s:      entities,
		imageRef: imageRef,

		portForwards: portForwards,
	}
	s.k8s = append(s.k8s, r)
	s.k8sByName[name] = r
	return skylark.None, nil
}

func (s *tiltfileState) convertPortForwards(name string, val skylark.Value) ([]portForward, error) {
	if val == nil {
		return nil, nil
	}
	switch val := val.(type) {
	case skylark.Int:
		pf, err := intToPortForward(val)
		if err != nil {
			return nil, err
		}
		return []portForward{pf}, nil
	case portForward:
		return []portForward{val}, nil
	case skylark.Sequence:
		var result []portForward
		it := val.Iterate()
		defer it.Done()
		var i skylark.Value
		for it.Next(&i) {
			switch i := i.(type) {
			case skylark.Int:
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

func (s *tiltfileState) portForward(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var local int
	var container int

	if err := skylark.UnpackArgs(fn.Name(), args, kwargs, "local", &local, "container?", &container); err != nil {
		return nil, err
	}

	return portForward{local: local, container: container}, nil
}

type portForward struct {
	local     int
	container int
}

var _ skylark.Value = portForward{}

func (f portForward) String() string {
	return fmt.Sprintf("port_forward(%d, %d)", f.local, f.container)
}

func (f portForward) Type() string {
	return "port_forward"
}

func (f portForward) Freeze() {}

func (f portForward) Truth() skylark.Bool {
	return f.local != 0 && f.container != 0
}

func (f portForward) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: port_forward")
}

func intToPortForward(i skylark.Int) (portForward, error) {
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

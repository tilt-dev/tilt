package tiltfile

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	"go.starlark.net/starlark"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

type referenceList []reference.Named

func (l referenceList) Len() int           { return len(l) }
func (l referenceList) Less(i, j int) bool { return l[i].String() < l[j].String() }
func (l referenceList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

type k8sResource struct {
	// The name of this group, for display in the UX.
	name string

	// All k8s resources to be deployed.
	entities []k8s.K8sEntity

	// Image refs that the user manually asked to be associated with this resource.
	providedImageRefNames map[string]bool

	// All image refs, including:
	// 1) one that the user manually asked to be associated with this resources, and
	// 2) images that were auto-inferred from the included k8s resources.
	imageRefNames map[string]bool

	imageRefs referenceList

	portForwards []portForward

	// labels for pods that we should watch and associate with this resource
	extraPodSelectors []labels.Selector

	dependencyIDs []model.TargetID
}

func (r *k8sResource) addProvidedImageRef(ref reference.Named) {
	r.providedImageRefNames[ref.Name()] = true
	if !r.imageRefNames[ref.Name()] {
		r.imageRefNames[ref.Name()] = true
		r.imageRefs = append(r.imageRefs, ref)
		sort.Sort(r.imageRefs)
	}
}

func (r *k8sResource) addEntities(entities []k8s.K8sEntity) error {
	r.entities = append(r.entities, entities...)

	for _, entity := range entities {
		images, err := entity.FindImages()
		if err != nil {
			return err
		}
		for _, image := range images {
			if !r.imageRefNames[image.Name()] {
				r.imageRefNames[image.Name()] = true
				r.imageRefs = append(r.imageRefs, image)
			}
		}
	}

	return nil
}

// Return the provided image refs in a deterministic order.
func (r k8sResource) providedImageRefNameList() []string {
	result := make([]string, 0, len(r.providedImageRefNames))
	for ref := range r.providedImageRefNames {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result
}

// Return the image refs in a deterministic order.
func (r k8sResource) imageRefNameList() []string {
	result := make([]string, 0, len(r.imageRefNames))
	for ref := range r.imageRefNames {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result
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

func (s *tiltfileState) filterYaml(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var yamlValue starlark.Value
	var labelsValue starlark.Value
	err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"yaml", &yamlValue,
		"labels?", &labelsValue)
	if err != nil {
		return nil, err
	}

	var metaLabels map[string]string
	if labelsValue != nil {
		d, ok := labelsValue.(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("kwarg `labels`: expected dict, got %T", labelsValue)
		}

		metaLabels, err = skylarkStringDictToGoMap(d)
		if err != nil {
			return nil, fmt.Errorf("kwarg `labels`: %v", err)
		}
	}

	entities, err := s.yamlEntitiesFromSkylarkValueOrList(yamlValue)
	if err != nil {
		return nil, err
	}

	match, rest, err := k8s.FilterByMetadataLabels(entities, metaLabels)
	if err != nil {
		return nil, err
	}

	matchingStr, err := k8s.SerializeYAML(match)
	if err != nil {
		return nil, err
	}
	restStr, err := k8s.SerializeYAML(rest)
	if err != nil {
		return nil, err
	}

	return starlark.Tuple{
		newBlob(matchingStr), newBlob(restStr),
	}, nil
}

func (s *tiltfileState) k8sResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var yamlValue starlark.Value
	var imageVal starlark.Value
	var portForwardsVal starlark.Value
	var extraPodSelectorsVal starlark.Value

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"yaml?", &yamlValue,
		"image?", &imageVal,
		"port_forwards?", &portForwardsVal,
		"extra_pod_selectors?", &extraPodSelectorsVal,
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

	var imageRefAsStr string
	switch imageVal := imageVal.(type) {
	case nil:
		// empty
	case starlark.String:
		imageRefAsStr = string(imageVal)
	case *fastBuild:
		imageRefAsStr = imageVal.img.ref.String()
	default:
		return nil, fmt.Errorf("image arg must be a string or fast_build; got %T", imageVal)
	}

	portForwards, err := s.convertPortForwards(name, portForwardsVal)
	if err != nil {
		return nil, err
	}

	err = r.addEntities(entities)
	if err != nil {
		return nil, err
	}

	if imageRefAsStr != "" {
		imageRef, err := reference.ParseNormalizedNamed(imageRefAsStr)
		if err != nil {
			return nil, err
		}
		r.addProvidedImageRef(imageRef)
	}
	r.portForwards = portForwards

	r.extraPodSelectors, err = podLabelsFromStarlarkValue(extraPodSelectorsVal)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func selectorFromSkylarkDict(d *starlark.Dict) (labels.Selector, error) {
	ret := make(labels.Set)

	for _, t := range d.Items() {
		kVal := t[0]
		k, ok := kVal.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("pod label keys must be strings; got '%s' of type %T", kVal.String(), kVal)
		}
		vVal := t[1]
		v, ok := vVal.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("pod label values must be strings; got '%s' of type %T", vVal.String(), vVal)
		}
		ret[string(k)] = string(v)
	}
	if len(ret) > 0 {
		return ret.AsSelector(), nil
	} else {
		return nil, nil
	}
}

func podLabelsFromStarlarkValue(v starlark.Value) ([]labels.Selector, error) {
	if v == nil {
		return nil, nil
	}

	switch x := v.(type) {
	case *starlark.Dict:
		s, err := selectorFromSkylarkDict(x)
		if err != nil {
			return nil, err
		} else if s == nil {
			return nil, nil
		} else {
			return []labels.Selector{s}, nil
		}
	case *starlark.List:
		var ret []labels.Selector

		it := x.Iterate()
		defer it.Done()
		var i starlark.Value
		for it.Next(&i) {
			d, ok := i.(*starlark.Dict)
			if !ok {
				return nil, fmt.Errorf("pod labels elements must be dicts; got %T", i)
			}
			s, err := selectorFromSkylarkDict(d)
			if err != nil {
				return nil, err
			} else if s != nil {
				ret = append(ret, s)
			}
		}

		return ret, nil
	default:
		return nil, fmt.Errorf("pod labels must be a dict or a list; got %T", v)
	}
}

func (s *tiltfileState) makeK8sResource(name string) (*k8sResource, error) {
	if s.k8sByName[name] != nil {
		return nil, fmt.Errorf("k8s_resource named %q already exists", name)
	}
	r := &k8sResource{
		name:                  name,
		providedImageRefNames: make(map[string]bool),
		imageRefNames:         make(map[string]bool),
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

	case starlark.String:
		pf, err := stringToPortForward(val)
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

			case starlark.String:
				pf, err := stringToPortForward(i)
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

func stringToPortForward(s starlark.String) (portForward, error) {
	parts := strings.SplitN(string(s), ":", 2)
	local, err := strconv.Atoi(parts[0])
	if err != nil || local < 0 || local > 65535 {
		return portForward{}, fmt.Errorf("portForward value %q is not in the range for a port [0-65535]", parts[0])
	}

	var container int
	if len(parts) == 2 {
		container, err = strconv.Atoi(parts[1])
		if err != nil || container < 0 || container > 65535 {
			return portForward{}, fmt.Errorf("portForward value %q is not in the range for a port [0-65535]", parts[1])
		}
	}
	return portForward{local: local, container: container}, nil
}

func (s *tiltfileState) portForwardsToDomain(r *k8sResource) []model.PortForward {
	var result []model.PortForward
	for _, pf := range r.portForwards {
		result = append(result, model.PortForward{LocalPort: pf.local, ContainerPort: pf.container})
	}
	return result
}

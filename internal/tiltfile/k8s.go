package tiltfile

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/windmilleng/tilt/internal/sliceutils"

	"go.starlark.net/syntax"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/container"
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

	// The single workload entity for this resource (only applies to v2 resource assembly)
	hasWorkloadEntity bool
	workloadEntity    k8s.K8sEntity

	// All extra k8s resources to be deployed, beyond the workload entity.
	entities []k8s.K8sEntity

	// Image selectors that the user manually asked to be associated with this resource.
	refSelectors []container.RefSelector

	imageRefs referenceList

	// Map of imageRefs, to avoid dupes
	imageRefMap map[string]bool

	portForwards []portForward

	// labels for pods that we should watch and associate with this resource
	extraPodSelectors []labels.Selector

	dependencyIDs []model.TargetID

	// whether this resource has already had a k8sResourceOptions applied to it
	k8sResourceOptionsApplied bool
	appliedOptions            k8sResourceOptions
}

func (k k8sResource) Entities() []k8s.K8sEntity {
	if k.hasWorkloadEntity {
		return append([]k8s.K8sEntity{k.workloadEntity}, k.entities...)
	} else {
		return k.entities
	}
}

const deprecatedResourceAssemblyV1Warning = "This Tiltfile is using k8s resource assembly version 1, which has been " +
	"deprecated. It will no longer be supported after 2019-04-17. Sorry for the inconvenience! " +
	"See https://tilt.dev/resource_assembly_migration.html for information on how to migrate."

// holds options passed to `k8s_resource` until assembly happens
type k8sResourceOptions struct {
	// if non-empty, how to rename this resource
	newName           string
	portForwards      []portForward
	extraPodSelectors []labels.Selector
	tiltfilePosition  syntax.Position
	consumed          bool
}

func (r *k8sResource) addRefSelector(selector container.RefSelector) {
	r.refSelectors = append(r.refSelectors, selector)
}

func (r *k8sResource) addWorkloadEntity(workload k8s.K8sEntity, imageJSONPaths func(e k8s.K8sEntity) []k8s.JSONPath) error {
	if r.hasWorkloadEntity {
		return fmt.Errorf("tried to add workload %q to resource %q, when it already has workload %q", workload.Name(), r.name, r.workloadEntity.Name())
	}

	r.hasWorkloadEntity = true
	r.workloadEntity = workload

	return r.addEntityImages([]k8s.K8sEntity{workload}, imageJSONPaths)
}

func (r *k8sResource) addEntities(entities []k8s.K8sEntity, imageJSONPaths func(e k8s.K8sEntity) []k8s.JSONPath) error {
	r.entities = append(r.entities, entities...)
	return r.addEntityImages(entities, imageJSONPaths)
}

func (r *k8sResource) addEntityImages(entities []k8s.K8sEntity, imageJSONPaths func(e k8s.K8sEntity) []k8s.JSONPath) error {
	for _, entity := range entities {
		images, err := entity.FindImages(imageJSONPaths(entity))
		if err != nil {
			return err
		}
		for _, image := range images {
			if !r.imageRefMap[image.String()] {
				r.imageRefMap[image.String()] = true
				r.imageRefs = append(r.imageRefs, image)
			}
		}
	}

	return nil
}

// Return the image selectors in a deterministic order.
func (r k8sResource) refSelectorList() []string {
	result := make([]string, 0, len(r.refSelectors))
	for _, selector := range r.refSelectors {
		result = append(result, selector.String())
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
	var yamlValue, labelsValue starlark.Value
	var name, namespace, kind, apiVersion string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"yaml", &yamlValue,
		"labels?", &labelsValue,
		"name?", &name,
		"namespace?", &namespace,
		"kind?", &kind,
		"api_version?", &apiVersion,
	)
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

	k, err := newK8SObjectSelector(apiVersion, kind, name, namespace)
	if err != nil {
		return nil, err
	}

	var match, rest []k8s.K8sEntity
	for _, e := range entities {
		if k.matches(e) {
			match = append(match, e)
		} else {
			rest = append(rest, e)
		}
	}

	if len(metaLabels) > 0 {
		var r []k8s.K8sEntity
		match, r, err = k8s.FilterByMetadataLabels(match, metaLabels)
		if err != nil {
			return nil, err
		}
		rest = append(rest, r...)
	}

	matchingStr, err := k8s.SerializeYAML(match)
	if err != nil {
		return nil, err
	}
	restStr, err := k8s.SerializeYAML(rest)
	if err != nil {
		return nil, err
	}

	var source string
	switch y := yamlValue.(type) {
	case *blob:
		source = y.source
	default:
		source = "filter_yaml"
	}

	return starlark.Tuple{
		newBlob(matchingStr, source), newBlob(restStr, source),
	}, nil
}

func (s *tiltfileState) k8sResourceV1(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	s.warnings = sliceutils.AppendWithoutDupes(s.warnings, deprecatedResourceAssemblyV1Warning)

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
		imageRefAsStr = imageVal.img.configurationRef.String()
	default:
		return nil, fmt.Errorf("image arg must be a string or fast_build; got %T", imageVal)
	}

	portForwards, err := convertPortForwards(portForwardsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s %q", fn.Name(), name)
	}

	err = r.addEntities(entities, s.imageJSONPaths)
	if err != nil {
		return nil, err
	}

	if imageRefAsStr != "" {
		imageRef, err := reference.ParseNormalizedNamed(imageRefAsStr)
		if err != nil {
			return nil, err
		}
		r.addRefSelector(container.NewRefSelector(imageRef))
	}
	r.portForwards = portForwards

	r.extraPodSelectors, err = podLabelsFromStarlarkValue(extraPodSelectorsVal)
	if err != nil {
		return nil, err
	}

	return starlark.None, nil
}

func (s *tiltfileState) k8sResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	switch s.k8sResourceAssemblyVersion {
	case 1:
		return s.k8sResourceV1(thread, fn, args, kwargs)
	case 2:
		return s.k8sResourceV2(thread, fn, args, kwargs)
	default:
		return starlark.None, fmt.Errorf("invalid k8s resource assembly version: %v", s.k8sResourceAssemblyVersion)
	}
}

func (s *tiltfileState) k8sResourceV2(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var workload string
	var newName string
	var portForwardsVal starlark.Value
	var extraPodSelectorsVal starlark.Value

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"workload", &workload,
		"new_name?", &newName,
		"port_forwards?", &portForwardsVal,
		"extra_pod_selectors?", &extraPodSelectorsVal,
	); err != nil {
		return nil, err
	}

	if workload == "" {
		return nil, fmt.Errorf("%s: workload must not be empty", fn.Name())
	}

	portForwards, err := convertPortForwards(portForwardsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s %q", fn.Name(), workload)
	}

	extraPodSelectors, err := podLabelsFromStarlarkValue(extraPodSelectorsVal)
	if err != nil {
		return nil, err
	}

	if opts, ok := s.k8sResourceOptions[workload]; ok {
		return nil, fmt.Errorf("%s already called for %s, at %s", fn.Name(), workload, opts.tiltfilePosition.String())
	}

	s.k8sResourceOptions[workload] = k8sResourceOptions{
		newName:           newName,
		portForwards:      portForwards,
		extraPodSelectors: extraPodSelectors,
		tiltfilePosition:  thread.Caller().Position(),
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

func starlarkValuesToJSONPaths(values []starlark.Value) ([]k8s.JSONPath, error) {
	var paths []k8s.JSONPath
	for _, v := range values {
		s, ok := v.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("path must be a string or list of strings, found a list containing value '%+v' of type '%T'", v, v)
		}

		jp, err := k8s.NewJSONPath(s.String())
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing json path '%s'", s.String())
		}

		paths = append(paths, jp)
	}

	return paths, nil
}

func (s *tiltfileState) k8sImageJsonPath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var apiVersion, kind, name, namespace string
	var imageJSONPath starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"path", &imageJSONPath,
		"api_version?", &apiVersion,
		"kind?", &kind,
		"name?", &name,
		"namespace?", &namespace,
	); err != nil {
		return nil, err
	}

	if kind == "" && name == "" && namespace == "" {
		return nil, errors.New("at least one of kind, name, or namespace must be specified")
	}

	values := starlarkValueOrSequenceToSlice(imageJSONPath)

	paths, err := starlarkValuesToJSONPaths(values)
	if err != nil {
		return nil, err
	}

	k, err := newK8SObjectSelector(apiVersion, kind, name, namespace)
	if err != nil {
		return nil, err
	}

	s.k8sImageJSONPaths[k] = paths

	return starlark.None, nil
}

func (s *tiltfileState) k8sKind(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// require image_json_path to be passed as a kw arg since `k8s_kind("Environment", "{.foo.bar}")` feels confusing
	if len(args) > 1 {
		return nil, fmt.Errorf("%s: got %d arguments, want at most %d", fn.Name(), len(args), 1)
	}

	var apiVersion, kind string
	var imageJSONPath starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"kind", &kind,
		"image_json_path", &imageJSONPath,
		"api_version?", &apiVersion,
	); err != nil {
		return nil, err
	}

	values := starlarkValueOrSequenceToSlice(imageJSONPath)
	paths, err := starlarkValuesToJSONPaths(values)
	if err != nil {
		return nil, err
	}

	k, err := newK8SObjectSelector(apiVersion, kind, "", "")
	if err != nil {
		return nil, err
	}

	s.k8sImageJSONPaths[k] = paths

	return starlark.None, nil
}

func (s *tiltfileState) makeK8sResource(name string) (*k8sResource, error) {
	if s.k8sByName[name] != nil {
		return nil, fmt.Errorf("k8s_resource named %q already exists", name)
	}
	r := &k8sResource{
		name:        name,
		imageRefMap: make(map[string]bool),
	}
	s.k8s = append(s.k8s, r)
	s.k8sByName[name] = r

	return r, nil
}

func (s *tiltfileState) yamlEntitiesFromSkylarkValueOrList(v starlark.Value) ([]k8s.K8sEntity, error) {
	values := starlarkValueOrSequenceToSlice(v)
	var ret []k8s.K8sEntity
	for _, value := range values {
		entities, err := s.yamlEntitiesFromSkylarkValue(value)
		if err != nil {
			return nil, err
		}
		ret = append(ret, entities...)
	}

	return ret, nil
}

func parseYAMLFromBlob(blob blob) ([]k8s.K8sEntity, error) {
	ret, err := k8s.ParseYAMLFromString(blob.String())
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading yaml from %s", blob.source)
	}
	return ret, nil
}

func (s *tiltfileState) yamlEntitiesFromSkylarkValue(v starlark.Value) ([]k8s.K8sEntity, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case *blob:
		return parseYAMLFromBlob(*v)
	default:
		yamlPath, err := s.localPathFromSkylarkValue(v)
		if err != nil {
			return nil, err
		}
		bs, err := s.readFile(yamlPath)
		if err != nil {
			return nil, errors.Wrap(err, "error reading yaml file")
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

func convertPortForwards(val starlark.Value) ([]portForward, error) {
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
				return nil, fmt.Errorf("port_forwards arg %v includes element %v which must be an int or a port_forward; is a %T", val, i, i)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("port_forwards must be an int, a port_forward, or a sequence of those; is a %T", val)
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

// returns any defined image JSON paths that apply to the given entity
func (s *tiltfileState) imageJSONPaths(e k8s.K8sEntity) []k8s.JSONPath {
	var ret []k8s.JSONPath

	for k, v := range s.k8sImageJSONPaths {
		if !k.matches(e) {
			continue
		}
		ret = append(ret, v...)
	}

	return ret
}

func (s *tiltfileState) k8sResourceAssemblyVersionFn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var version int
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"version", &version,
	); err != nil {
		return nil, err
	}

	if len(s.k8sUnresourced) > 0 || len(s.k8s) > 0 || len(s.k8sResourceOptions) > 0 {
		return starlark.None, fmt.Errorf("%s can only be called before introducing any k8s resources", fn.Name())
	}

	if version < 1 || version > 2 {
		return starlark.None, fmt.Errorf("invalid %s %d. Must be 1 or 2.", fn.Name(), version)
	}

	if version == 1 {
		s.warnings = append(s.warnings, deprecatedResourceAssemblyV1Warning)
	}

	s.k8sResourceAssemblyVersion = version

	return starlark.None, nil
}

func uniqueResourceNames(es []k8s.K8sEntity) ([]string, error) {
	ret := make([]string, len(es))
	// how many resources potentially map to a given name
	counts := make(map[string]int)

	// returns a list of potential names, in order of preference
	potentialNames := func(e k8s.K8sEntity) []string {
		components := []string{
			e.Name(),
			e.Kind.Kind,
			e.Namespace().String(),
			e.Kind.Group,
		}
		var ret []string
		for i := 0; i < len(components); i++ {
			ret = append(ret, strings.ToLower(strings.Join(components[:i+1], ":")))
		}
		return ret
	}

	// count how many entity want each potential name
	for _, e := range es {
		for _, name := range potentialNames(e) {
			counts[name]++
		}
	}

	// for each entity, take the shortest name that is uniquely wanted by that entity
	for i, e := range es {
		names := potentialNames(e)
		for _, name := range names {
			if counts[name] == 1 {
				ret[i] = name
				break
			}
		}
		if ret[i] == "" {
			return nil, fmt.Errorf("unable to find a unique resource name for '%s'", names[len(names)-1])
		}
	}

	return ret, nil
}

func parseK8SObjectSelector(s string) (k8sObjectSelector, error) {
	parts := strings.Split(s, ":")
	if len(parts) == 0 {
		return k8sObjectSelector{}, errors.New("k8s object selector cannot be empty")
	}
	if len(parts) > 4 {
		return k8sObjectSelector{}, fmt.Errorf("k8s object selector '%s' has more than 4 parts", s)
	}

	for len(parts) < 4 {
		parts = append(parts, "")
	}

	name, kind, namespace, apiVersion := parts[0], parts[1], parts[2], parts[3]

	return newK8SObjectSelector(apiVersion, kind, name, namespace)
}

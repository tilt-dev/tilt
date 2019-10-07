package tiltfile

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/pkg/model"
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

	// Image selectors that the user manually asked to be associated with this resource.
	refSelectors []container.RefSelector

	imageRefs referenceList

	// Map of imageRefs -> count, to avoid dupes / know how many times we've injected each
	imageRefMap map[string]int

	portForwards []portForward

	// labels for pods that we should watch and associate with this resource
	extraPodSelectors []labels.Selector

	dependencyIDs []model.TargetID

	triggerMode triggerMode
}

const deprecatedResourceAssemblyV1Warning = "This Tiltfile is using k8s resource assembly version 1, which has been " +
	"deprecated. See https://docs.tilt.dev/resource_assembly_migration.html for more information."

// holds options passed to `k8s_resource` until assembly happens
type k8sResourceOptions struct {
	// if non-empty, how to rename this resource
	newName           string
	portForwards      []portForward
	extraPodSelectors []labels.Selector
	triggerMode       triggerMode
	tiltfilePosition  syntax.Position
	consumed          bool
}

func (r *k8sResource) addRefSelector(selector container.RefSelector) {
	r.refSelectors = append(r.refSelectors, selector)
}

func (r *k8sResource) addEntities(entities []k8s.K8sEntity,
	imageJSONPaths func(e k8s.K8sEntity) []k8s.JSONPath, envVarImages []container.RefSelector) error {
	r.entities = append(r.entities, entities...)

	for _, entity := range entities {
		images, err := entity.FindImages(imageJSONPaths(entity), envVarImages)
		if err != nil {
			return err
		}
		for _, image := range images {
			count := r.imageRefMap[image.String()]
			if count == 0 {
				r.imageRefs = append(r.imageRefs, image)
			}
			r.imageRefMap[image.String()] += 1
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
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"yaml", &yamlValue,
	); err != nil {
		return nil, err
	}

	entities, err := s.yamlEntitiesFromSkylarkValueOrList(thread, yamlValue)
	if err != nil {
		return nil, err
	}
	s.k8sUnresourced = append(s.k8sUnresourced, entities...)

	return starlark.None, nil
}

func (s *tiltfileState) extractSecrets() model.SecretSet {
	result := model.SecretSet{}
	for _, e := range s.k8sUnresourced {
		secrets := s.maybeExtractSecrets(e)
		result.AddAll(secrets)
	}

	for _, k := range s.k8s {
		for _, e := range k.entities {
			secrets := s.maybeExtractSecrets(e)
			result.AddAll(secrets)
		}
	}
	return result
}

func (s *tiltfileState) maybeExtractSecrets(e k8s.K8sEntity) model.SecretSet {
	secret, ok := e.Obj.(*v1.Secret)
	if !ok {
		return nil
	}

	result := model.SecretSet{}
	for key, data := range secret.Data {
		result.AddSecret(secret.Name, key, data)
	}

	for key, data := range secret.StringData {
		result.AddSecret(secret.Name, key, []byte(data))
	}
	return result
}

func (s *tiltfileState) filterYaml(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var yamlValue, labelsValue starlark.Value
	var name, namespace, kind, apiVersion string
	err := s.unpackArgs(fn.Name(), args, kwargs,
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

	entities, err := s.yamlEntitiesFromSkylarkValueOrList(thread, yamlValue)
	if err != nil {
		return nil, err
	}

	k, err := newK8sObjectSelector(apiVersion, kind, name, namespace)
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

	matchingStr, err := k8s.SerializeSpecYAML(match)
	if err != nil {
		return nil, err
	}
	restStr, err := k8s.SerializeSpecYAML(rest)
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

	if err := s.unpackArgs(fn.Name(), args, kwargs,
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

	entities, err := s.yamlEntitiesFromSkylarkValueOrList(thread, yamlValue)
	if err != nil {
		return nil, err
	}

	var imageRefAsStr string
	switch imageVal := imageVal.(type) {
	case nil:
		// empty
	case starlark.String:
		imageRefAsStr = string(imageVal)
	default:
		return nil, fmt.Errorf("image arg must be a string; got %T", imageVal)
	}

	portForwards, err := convertPortForwards(portForwardsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s %q", fn.Name(), name)
	}

	err = r.addEntities(entities, s.imageJSONPaths, nil)
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

// v1 syntax:
// `k8s_resource(name, yaml='', image='', port_forwards=[], extra_pod_selectors=[])`
// v2 syntax:
// `k8s_resource(workload, new_name='', port_forwards=[], extra_pod_selectors=[])`
// this function tries to tell if they're still using a v1 tiltfile after we made v2 the default
func (s *tiltfileState) isProbablyK8sResourceV1Call(args starlark.Tuple, kwargs []starlark.Tuple) (bool, string) {
	var k8sResourceV1OnlyNames = map[string]bool{
		"name":  true,
		"yaml":  true,
		"image": true,
	}
	for _, item := range kwargs {
		name := string(item[0].(starlark.String))
		if _, ok := k8sResourceV1OnlyNames[name]; ok {
			return true, fmt.Sprintf("it was called with kwarg %q, which no longer exists", name)
		}
	}

	// check positional args
	// check if the second arg is yaml (v1) instead of a resource name (v2)
	if args.Len() >= 2 {
		switch x := args[1].(type) {
		case starlark.Sequence:
			return true, "second arg was a sequence"
		case *blob:
			return true, "second arg was a blob"
		// if a Tiltfile contains `k8s_resource('foo', 'foo.yaml')`
		// in v1, the second arg is a yaml file name
		// in v2, it's the new resource name
		case starlark.String:
			if strings.HasSuffix(string(x), ".yaml") || strings.HasSuffix(string(x), ".yml") {
				return true, "second arg looks like a yaml file name, not a resource name"
			}
		default:
			// this is invalid in both v1 and v2 syntax, so fall back and let v2 parsing error out
		}
	}

	// we don't need to check the subsequent positional args because they can't include a third positional arg without
	// including a second!

	return false, ""
}

func (s *tiltfileState) k8sResourceV2(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	isV1, msg := s.isProbablyK8sResourceV1Call(args, kwargs)
	if isV1 {
		return starlark.None, fmt.Errorf("It looks like k8s_resource is being called with deprecated arguments: %s.\n\n%s", msg, deprecatedResourceAssemblyV1Warning)
	}
	var workload string
	var newName string
	var portForwardsVal starlark.Value
	var extraPodSelectorsVal starlark.Value
	var triggerMode triggerMode

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"workload", &workload,
		"new_name?", &newName,
		"port_forwards?", &portForwardsVal,
		"extra_pod_selectors?", &extraPodSelectorsVal,
		"trigger_mode?", &triggerMode,
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
		tiltfilePosition:  thread.CallFrame(1).Pos,
		triggerMode:       triggerMode,
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
			return nil, fmt.Errorf("paths must be a string or list of strings, found a list containing value '%+v' of type '%T'", v, v)
		}

		jp, err := k8s.NewJSONPath(s.String())
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing json paths '%s'", s.String())
		}

		paths = append(paths, jp)
	}

	return paths, nil
}

func (s *tiltfileState) k8sImageJsonPath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var apiVersion, kind, name, namespace string
	var imageJSONPath starlark.Value
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"paths", &imageJSONPath,
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

	k, err := newK8sObjectSelector(apiVersion, kind, name, namespace)
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
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"kind", &kind,
		"image_json_path?", &imageJSONPath,
		"api_version?", &apiVersion,
	); err != nil {
		return nil, err
	}

	k, err := newK8sObjectSelector(apiVersion, kind, "", "")
	if err != nil {
		return nil, err
	}

	if imageJSONPath == nil {
		s.workloadTypes = append(s.workloadTypes, k)
	} else {
		values := starlarkValueOrSequenceToSlice(imageJSONPath)
		paths, err := starlarkValuesToJSONPaths(values)
		if err != nil {
			return nil, err
		}

		s.k8sImageJSONPaths[k] = paths
	}

	return starlark.None, nil
}

func (s *tiltfileState) workloadToResourceFunctionFn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var wtrf *starlark.Function
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"func", &wtrf); err != nil {
		return nil, err
	}

	workloadToResourceFunction, err := makeWorkloadToResourceFunction(wtrf)
	if err != nil {
		return starlark.None, err
	}

	s.workloadToResourceFunction = workloadToResourceFunction

	return starlark.None, nil
}

type k8sObjectID struct {
	name      string
	kind      string
	namespace string
	group     string
}

func (k k8sObjectID) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(k.name), nil
	case "kind":
		return starlark.String(k.kind), nil
	case "namespace":
		return starlark.String(k.namespace), nil
	case "group":
		return starlark.String(k.group), nil
	default:
		return starlark.None, fmt.Errorf("%T has no attribute '%s'", k, name)
	}
}

func (k k8sObjectID) AttrNames() []string {
	return []string{"name", "kind", "namespace", "group"}
}

func (k k8sObjectID) String() string {
	return strings.ToLower(fmt.Sprintf("%s:%s:%s:%s", k.name, k.kind, k.namespace, k.group))
}

func (k k8sObjectID) Type() string {
	return "K8sObjectID"
}

func (k k8sObjectID) Freeze() {
}

func (k k8sObjectID) Truth() starlark.Bool {
	return k.name != "" || k.kind != "" || k.namespace != "" || k.group != ""
}

func (k k8sObjectID) Hash() (uint32, error) {
	return starlark.Tuple{starlark.String(k.name), starlark.String(k.kind), starlark.String(k.namespace), starlark.String(k.group)}.Hash()
}

var _ starlark.Value = k8sObjectID{}

type workloadToResourceFunction struct {
	fn  func(thread *starlark.Thread, id k8sObjectID) (string, error)
	pos syntax.Position
}

func makeWorkloadToResourceFunction(f *starlark.Function) (workloadToResourceFunction, error) {
	if f.NumParams() != 1 {
		return workloadToResourceFunction{}, fmt.Errorf("%s arg must take 1 argument. %s takes %d", workloadToResourceFunctionN, f.Name(), f.NumParams())
	}
	fn := func(thread *starlark.Thread, id k8sObjectID) (string, error) {
		ret, err := starlark.Call(thread, f, starlark.Tuple{id}, nil)
		if err != nil {
			return "", err
		}
		s, ok := ret.(starlark.String)
		if !ok {
			return "", fmt.Errorf("%s: invalid return value. wanted: string. got: %T", f.Name(), ret)
		}
		return string(s), nil
	}

	return workloadToResourceFunction{
		fn:  fn,
		pos: f.Position(),
	}, nil
}

func (s *tiltfileState) makeK8sResource(name string) (*k8sResource, error) {
	if s.k8sByName[name] != nil {
		return nil, fmt.Errorf("k8s_resource named %q already exists", name)
	}
	r := &k8sResource{
		name:        name,
		imageRefMap: make(map[string]int),
	}
	s.k8s = append(s.k8s, r)
	s.k8sByName[name] = r

	return r, nil
}

func (s *tiltfileState) yamlEntitiesFromSkylarkValueOrList(thread *starlark.Thread, v starlark.Value) ([]k8s.K8sEntity, error) {
	values := starlarkValueOrSequenceToSlice(v)
	var ret []k8s.K8sEntity
	for _, value := range values {
		entities, err := s.yamlEntitiesFromSkylarkValue(thread, value)
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

func (s *tiltfileState) yamlEntitiesFromSkylarkValue(thread *starlark.Thread, v starlark.Value) ([]k8s.K8sEntity, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case *blob:
		return parseYAMLFromBlob(*v)
	default:
		yamlPath, err := s.absPathFromStarlarkValue(thread, v)
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
				return entities, fmt.Errorf("%s is not a valid YAML file: %s", yamlPath, err)
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

	if err := s.unpackArgs(fn.Name(), args, kwargs, "local", &local, "container?", &container); err != nil {
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
	if err := s.unpackArgs(fn.Name(), args, kwargs,
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
	s.k8sResourceAssemblyVersionReason = k8sResourceAssemblyVersionReasonExplicit

	return starlark.None, nil
}

func (s *tiltfileState) calculateResourceNames(workloads []k8s.K8sEntity) ([]string, error) {
	if s.workloadToResourceFunction.fn != nil {
		names, err := s.workloadToResourceFunctionNames(workloads)
		if err != nil {
			return nil, errors.Wrapf(err, "error applying workload_to_resource_function %s", s.workloadToResourceFunction.pos.String())
		}
		return names, nil
	} else {
		return k8s.UniqueNames(workloads, 1), nil
	}
}

// calculates names for workloads using s.workloadToResourceFunction
func (s *tiltfileState) workloadToResourceFunctionNames(workloads []k8s.K8sEntity) ([]string, error) {
	takenNames := make(map[string]k8s.K8sEntity)
	ret := make([]string, len(workloads))
	thread := &starlark.Thread{
		Print: s.print,
	}
	for i, e := range workloads {
		id := newK8sObjectID(e)
		name, err := s.workloadToResourceFunction.fn(thread, id)
		if err != nil {
			return nil, errors.Wrapf(err, "error determing resource name for '%s'", id.String())
		}

		if conflictingWorkload, ok := takenNames[name]; ok {
			return nil, fmt.Errorf("both '%s' and '%s' mapped to resource name '%s'", newK8sObjectID(e).String(), newK8sObjectID(conflictingWorkload).String(), name)
		}

		ret[i] = name
		takenNames[name] = e
	}
	return ret, nil
}

func newK8sObjectID(e k8s.K8sEntity) k8sObjectID {
	gvk := e.GVK()
	return k8sObjectID{
		name:      e.Name(),
		kind:      gvk.Kind,
		namespace: e.Namespace().String(),
		group:     gvk.Group,
	}
}

func (s *tiltfileState) k8sContext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(s.kubeContext), nil
}

func (s *tiltfileState) allowK8SContexts(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contexts starlark.Value
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"contexts", &contexts,
	); err != nil {
		return nil, err
	}

	for _, c := range starlarkValueOrSequenceToSlice(contexts) {
		switch val := c.(type) {
		case starlark.String:
			s.allowedK8SContexts = append(s.allowedK8SContexts, k8s.KubeContext(val))
		default:
			return nil, fmt.Errorf("allow_k8s_contexts contexts must be a string or a sequence of strings; found a %T", val)

		}
	}

	return starlark.None, nil
}

func (s *tiltfileState) validateK8SContext() error {
	if s.kubeEnv == k8s.EnvNone || s.kubeEnv.IsLocalCluster() {
		return nil
	}

	for _, c := range s.allowedK8SContexts {
		if c == s.kubeContext {
			return nil
		}
	}

	return fmt.Errorf("'%s' is not a known local context", s.kubeContext)
}

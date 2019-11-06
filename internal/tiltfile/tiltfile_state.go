package tiltfile

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/looplab/tarjan"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/tiltfile/dockerprune"
	"github.com/windmilleng/tilt/internal/tiltfile/git"
	"github.com/windmilleng/tilt/internal/tiltfile/include"
	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/k8scontext"
	"github.com/windmilleng/tilt/internal/tiltfile/os"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type resourceSet struct {
	dc  dcResourceSet // currently only support one d-c.yml
	k8s []*k8sResource
}

type tiltfileState struct {
	// set at creation
	ctx             context.Context
	dcCli           dockercompose.DockerComposeClient
	k8sContextExt   k8scontext.Extension
	privateRegistry container.Registry
	features        feature.FeatureSet

	// added to during execution
	buildIndex         *buildIndex
	k8s                []*k8sResource
	k8sByName          map[string]*k8sResource
	k8sUnresourced     []k8s.K8sEntity
	dc                 dcResourceSet // currently only support one d-c.yml
	k8sResourceOptions map[string]k8sResourceOptions
	localResources     []localResource

	// ensure that any pushed images are pushed instead to this registry, rewriting names if needed
	defaultRegistryHost container.Registry

	// JSON paths to images in k8s YAML (other than Container specs)
	k8sImageJSONPaths map[k8sObjectSelector][]k8s.JSONPath
	// objects of these types are considered workloads, whether or not they have an image
	workloadTypes []k8sObjectSelector

	k8sResourceAssemblyVersion       int
	k8sResourceAssemblyVersionReason k8sResourceAssemblyVersionReason
	workloadToResourceFunction       workloadToResourceFunction

	// for assembly
	usedImages map[string]bool

	// count how many times each builtin is called, for analytics
	builtinCallCounts map[string]int
	// how many times each arg is used on each builtin
	builtinArgCounts map[string]map[string]int

	// any LiveUpdate steps that have been created but not used by a LiveUpdate will cause an error, to ensure
	// that users aren't accidentally using step-creating functions incorrectly
	// stored as a map of string(declarationPosition) -> step
	// it'd be appealing to store this as a map[liveUpdateStep]bool, but then things get weird if we have two steps
	// with the same hashcode (like, all restartcontainer steps)
	unconsumedLiveUpdateSteps map[string]liveUpdateStep

	// global trigger mode -- will be the default for all manifests (tho user can still explicitly set
	// triggerMode for a specific manifest)
	triggerMode triggerMode

	// for error reporting in case it's called twice
	triggerModeCallPosition syntax.Position

	teamName string

	logger            logger.Logger
	warnings          []string
	postExecReadFiles []string
}

type k8sResourceAssemblyVersionReason int

const (
	// assembly version is just at the default; the user hasn't set anything
	k8sResourceAssemblyVersionReasonDefault k8sResourceAssemblyVersionReason = iota
	// the user has explicit set the assembly version
	k8sResourceAssemblyVersionReasonExplicit
)

func newTiltfileState(
	ctx context.Context,
	dcCli dockercompose.DockerComposeClient,
	k8sContextExt k8scontext.Extension,
	privateRegistry container.Registry,
	features feature.FeatureSet) *tiltfileState {
	return &tiltfileState{
		ctx:                        ctx,
		dcCli:                      dcCli,
		k8sContextExt:              k8sContextExt,
		privateRegistry:            privateRegistry,
		buildIndex:                 newBuildIndex(),
		k8sByName:                  make(map[string]*k8sResource),
		k8sImageJSONPaths:          make(map[k8sObjectSelector][]k8s.JSONPath),
		usedImages:                 make(map[string]bool),
		logger:                     logger.Get(ctx),
		builtinCallCounts:          make(map[string]int),
		builtinArgCounts:           make(map[string]map[string]int),
		unconsumedLiveUpdateSteps:  make(map[string]liveUpdateStep),
		k8sResourceAssemblyVersion: 2,
		k8sResourceOptions:         make(map[string]k8sResourceOptions),
		localResources:             []localResource{},
		triggerMode:                TriggerModeAuto,
		features:                   features,
	}
}

// print() for fulfilling the starlark thread callback
func (s *tiltfileState) print(_ *starlark.Thread, msg string) {
	s.logger.Infof("%s", msg)
}

// Load loads the Tiltfile in `filename`, and returns the manifests matching `matching`.
//
// This often returns a starkit.Model even on error, because the starkit.Model
// has a record of what happened during the execution (what files were read, etc).
//
// TODO(nick): Eventually this will just return a starkit.Model, which will contain
// all the mutable state collected by execution.
func (s *tiltfileState) loadManifests(absFilename string, requested []model.ManifestName) ([]model.Manifest, starkit.Model, error) {
	s.logger.Infof("Beginning Tiltfile execution")

	result, err := starkit.ExecFile(absFilename,
		s,
		include.IncludeFn{},
		git.NewExtension(),
		os.NewExtension(),
		io.NewExtension(),
		s.k8sContextExt,
		dockerprune.NewExtension(),
	)
	if err != nil {
		return nil, result, starkit.UnpackBacktrace(err)
	}

	resources, unresourced, err := s.assemble()
	if err != nil {
		return nil, result, err
	}

	var manifests []model.Manifest
	k8sContextState, err := k8scontext.GetState(result)
	if err != nil {
		return nil, result, err
	}

	if len(resources.k8s) > 0 {
		manifests, err = s.translateK8s(resources.k8s)
		if err != nil {
			return nil, result, err
		}

		isAllowed := k8sContextState.IsAllowed()
		if !isAllowed {
			kubeContext := k8sContextState.KubeContext()
			return nil, result, fmt.Errorf(`Stop! %s might be production.
If you're sure you want to deploy there, add:
allow_k8s_contexts('%s')
to your Tiltfile. Otherwise, switch k8s contexts and restart Tilt.`, kubeContext, kubeContext)
		}
	} else {
		manifests, err = s.translateDC(resources.dc)
		if err != nil {
			return nil, result, err
		}
	}

	err = s.checkForUnconsumedLiveUpdateSteps()
	if err != nil {
		return nil, result, err
	}

	localManifests, err := s.translateLocal()
	if err != nil {
		return nil, result, err
	}
	manifests = append(manifests, localManifests...)

	manifests, err = match(manifests, requested)
	if err != nil {
		return nil, result, err
	}

	yamlManifest := model.Manifest{}
	if len(unresourced) > 0 {
		yamlManifest, err = k8s.NewK8sOnlyManifest(model.UnresourcedYAMLManifestName, unresourced)
		if err != nil {
			return nil, starkit.Model{}, err
		}
		manifests = append(manifests, yamlManifest)
	}

	err = validateResourceDependencies(manifests)
	if err != nil {
		return nil, starkit.Model{}, err
	}

	return manifests, result, nil
}

// Builtin functions

const (
	// build functions
	dockerBuildN     = "docker_build"
	fastBuildN       = "fast_build"
	customBuildN     = "custom_build"
	defaultRegistryN = "default_registry"

	// docker compose functions
	dockerComposeN = "docker_compose"
	dcResourceN    = "dc_resource"

	// k8s functions
	k8sResourceAssemblyVersionN = "k8s_resource_assembly_version"
	k8sYamlN                    = "k8s_yaml"
	filterYamlN                 = "filter_yaml"
	k8sResourceN                = "k8s_resource"
	localResourceN              = "local_resource"
	portForwardN                = "port_forward"
	k8sKindN                    = "k8s_kind"
	k8sImageJSONPathN           = "k8s_image_json_path"
	workloadToResourceFunctionN = "workload_to_resource_function"

	// file functions
	localN      = "local"
	kustomizeN  = "kustomize"
	helmN       = "helm"
	decodeJSONN = "decode_json"
	readJSONN   = "read_json"
	readYAMLN   = "read_yaml"

	// live update functions
	fallBackOnN       = "fall_back_on"
	syncN             = "sync"
	runN              = "run"
	restartContainerN = "restart_container"

	// trigger mode
	triggerModeN       = "trigger_mode"
	triggerModeAutoN   = "TRIGGER_MODE_AUTO"
	triggerModeManualN = "TRIGGER_MODE_MANUAL"

	// feature flags
	enableFeatureN  = "enable_feature"
	disableFeatureN = "disable_feature"

	disableSnapshotsN = "disable_snapshots"

	// other functions
	failN    = "fail"
	setTeamN = "set_team"
)

type triggerMode int

func (m triggerMode) String() string {
	switch m {
	case TriggerModeAuto:
		return triggerModeAutoN
	case TriggerModeManual:
		return triggerModeManualN
	default:
		return fmt.Sprintf("unknown trigger mode with value %d", m)
	}
}

func (t triggerMode) Type() string {
	return "TriggerMode"
}

func (t triggerMode) Freeze() {
	// noop
}

func (t triggerMode) Truth() starlark.Bool {
	return starlark.MakeInt(int(t)).Truth()
}

func (t triggerMode) Hash() (uint32, error) {
	return starlark.MakeInt(int(t)).Hash()
}

var _ starlark.Value = triggerMode(0)

const (
	TriggerModeUnset  triggerMode = iota
	TriggerModeAuto   triggerMode = iota
	TriggerModeManual triggerMode = iota
)

func (s *tiltfileState) triggerModeForResource(resourceTriggerMode triggerMode) triggerMode {
	if resourceTriggerMode != TriggerModeUnset {
		return resourceTriggerMode
	} else {
		return s.triggerMode
	}
}

func starlarkTriggerModeToModel(triggerMode triggerMode, autoInit bool) (model.TriggerMode, error) {
	switch triggerMode {
	case TriggerModeAuto:
		if !autoInit {
			return 0, errors.New("auto_init=False incompatible with trigger_mode=TRIGGER_MODE_AUTO")
		}
		return model.TriggerModeAuto, nil
	case TriggerModeManual:
		if autoInit {
			return model.TriggerModeManualAfterInitial, nil
		} else {
			return model.TriggerModeManualIncludingInitial, nil
		}
	default:
		return 0, fmt.Errorf("unknown triggerMode %v", triggerMode)
	}
}

// count how many times each builtin is called, for analytics
func (s *tiltfileState) OnBuiltinCall(name string, fn *starlark.Builtin) {
	s.builtinCallCounts[name]++
}

func (s *tiltfileState) OnExec(t *starlark.Thread, tiltfilePath string) error {
	return io.RecordReadFile(t, tiltIgnorePath(tiltfilePath))
}

type builtin func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

// wrap a builtin such that it's only allowed to run when we have a known safe k8s context
// (none (e.g., docker-compose), local, or specified by `allow_k8s_contexts`)
func (s *tiltfileState) potentiallyK8sUnsafeBuiltin(f builtin) builtin {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		model, err := starkit.ModelFromThread(thread)
		if err != nil {
			return nil, err
		}

		k8sContextState, err := k8scontext.GetState(model)
		if err != nil {
			return nil, err
		}

		isAllowed := k8sContextState.IsAllowed()
		if !isAllowed {
			kubeContext := k8sContextState.KubeContext()
			return nil, fmt.Errorf(`Refusing to run '%s' because %s might be a production kube context.
If you're sure you want to continue add:
allow_k8s_contexts('%s')
before this function call in your Tiltfile. Otherwise, switch k8s contexts and restart Tilt.`, fn.Name(), kubeContext, kubeContext)
		}

		return f(thread, fn, args, kwargs)
	}
}

func (s *tiltfileState) unpackArgs(fnname string, args starlark.Tuple, kwargs []starlark.Tuple, pairs ...interface{}) error {
	err := starlark.UnpackArgs(fnname, args, kwargs, pairs...)
	if err == nil {
		var paramNames []string
		for i, o := range pairs {
			if i%2 == 0 {
				name := strings.TrimSuffix(o.(string), "?")
				paramNames = append(paramNames, name)
			}
		}

		usedParamNames := paramNames[:args.Len()]
		for _, p := range kwargs {
			name := strings.TrimSuffix(string(p[0].(starlark.String)), "?")
			usedParamNames = append(usedParamNames, name)
		}
		_, ok := s.builtinArgCounts[fnname]
		if !ok {
			s.builtinArgCounts[fnname] = make(map[string]int)
		}
		for _, paramName := range usedParamNames {
			s.builtinArgCounts[fnname][paramName]++
		}
	}
	return err
}

// TODO(nick): Split these into separate extensions
func (s *tiltfileState) OnStart(e *starkit.Environment) error {
	e.SetArgUnpacker(s.unpackArgs)
	e.SetPrint(s.print)

	for _, b := range []struct {
		name    string
		builtin builtin
	}{
		{localN, s.potentiallyK8sUnsafeBuiltin(s.local)},
		{dockerBuildN, s.dockerBuild},
		{fastBuildN, s.fastBuild},
		{customBuildN, s.customBuild},
		{defaultRegistryN, s.defaultRegistry},
		{dockerComposeN, s.dockerCompose},
		{dcResourceN, s.dcResource},
		{k8sResourceAssemblyVersionN, s.k8sResourceAssemblyVersionFn},
		{k8sYamlN, s.k8sYaml},
		{filterYamlN, s.filterYaml},
		{k8sResourceN, s.k8sResource},
		{localResourceN, s.localResource},
		{portForwardN, s.portForward},
		{k8sKindN, s.k8sKind},
		{k8sImageJSONPathN, s.k8sImageJsonPath},
		{workloadToResourceFunctionN, s.workloadToResourceFunctionFn},
		{kustomizeN, s.kustomize},
		{helmN, s.helm},
		{failN, s.fail},
		{decodeJSONN, s.decodeJSON},
		{readJSONN, s.readJson},
		{readYAMLN, s.readYaml},
		{triggerModeN, s.triggerModeFn},
		{fallBackOnN, s.liveUpdateFallBackOn},
		{syncN, s.liveUpdateSync},
		{runN, s.liveUpdateRun},
		{restartContainerN, s.liveUpdateRestartContainer},
		{enableFeatureN, s.enableFeature},
		{disableFeatureN, s.disableFeature},
		{disableSnapshotsN, s.disableSnapshots},
		{setTeamN, s.setTeam},
	} {
		err := e.AddBuiltin(b.name, b.builtin)
		if err != nil {
			return err
		}
	}

	for _, v := range []struct {
		name  string
		value starlark.Value
	}{
		{triggerModeAutoN, TriggerModeAuto},
		{triggerModeManualN, TriggerModeManual},
	} {
		err := e.AddValue(v.name, v.value)
		if err != nil {
			return err
		}
	}

	return nil
}

// Returns the current orchestrator.
//
// Note that assemble() will eventually error out if this has
// both DC and K8s resources.
func (s *tiltfileState) orchestrator() model.Orchestrator {
	if !s.dc.Empty() {
		return model.OrchestratorDC
	}
	return model.OrchestratorK8s
}

func (s *tiltfileState) assemble() (resourceSet, []k8s.K8sEntity, error) {
	err := s.assembleImages()
	if err != nil {
		return resourceSet{}, nil, err
	}

	switch s.k8sResourceAssemblyVersion {
	case 1:
		err = s.assembleK8sV1()
	case 2:
		err = s.assembleK8sV2()
	}
	if err != nil {
		return resourceSet{}, nil, err
	}

	err = s.assembleDC()
	if err != nil {
		return resourceSet{}, nil, err
	}

	if !s.dc.Empty() && (len(s.k8s) > 0 || len(s.k8sUnresourced) > 0) {
		return resourceSet{}, nil, fmt.Errorf("can't declare both k8s " +
			"resources/entities and docker-compose resources")
	}

	err = s.buildIndex.assertAllMatched()
	if err != nil {
		s.warnings = append(s.warnings, err.Error())
	}

	return resourceSet{
		dc:  s.dc,
		k8s: s.k8s,
	}, s.k8sUnresourced, nil
}

func (s *tiltfileState) assembleImages() error {
	registry := s.defaultRegistryHost
	if s.orchestrator() == model.OrchestratorK8s && s.privateRegistry != "" {
		// If we've found a private registry in the cluster at run-time,
		// use that instead of the one in the tiltfile
		s.logger.Infof("Auto-detected private registry from environment: %s", s.privateRegistry)
		registry = s.privateRegistry
	}

	for _, imageBuilder := range s.buildIndex.images {
		var err error
		imageBuilder.deploymentRef, err = container.ReplaceRegistry(registry, imageBuilder.configurationRef)
		if err != nil {
			return err
		}

		var depImages []reference.Named
		if imageBuilder.dbDockerfile != "" {
			depImages, err = imageBuilder.dbDockerfile.FindImages()
		}

		if err != nil {
			return err
		}

		for _, depImage := range depImages {
			depBuilder := s.buildIndex.findBuilderForConsumedImage(depImage)
			if depBuilder != nil {
				imageBuilder.dependencyIDs = append(imageBuilder.dependencyIDs, depBuilder.ID())
			}
		}
	}
	return nil
}

func (s *tiltfileState) assembleDC() error {
	if len(s.dc.services) > 0 && s.defaultRegistryHost != "" {
		return errors.New("default_registry is not supported with docker compose")
	}

	for _, svc := range s.dc.services {
		if svc.ImageRef() != nil {
			builder := s.buildIndex.findBuilderForConsumedImage(svc.ImageRef())
			if builder != nil {
				svc.DependencyIDs = append(svc.DependencyIDs, builder.ID())
			}
			// TODO(maia): throw warning if
			//  a. there is an img ref from config, and img ref from user doesn't match
			//  b. there is no img ref from config, and img ref from user is not of form .*_<svc_name>
		}
	}
	return nil
}

func (s *tiltfileState) assembleK8sV1() error {
	err := s.assembleK8sWithImages()
	if err != nil {
		return err
	}

	err = s.assembleK8sUnresourced()
	if err != nil {
		return err
	}

	for _, r := range s.k8s {
		if err := s.validateK8s(r); err != nil {
			return err
		}
	}
	return nil

}

func (s *tiltfileState) assembleK8sV2() error {
	err := s.assembleK8sByWorkload()
	if err != nil {
		return err
	}

	err = s.assembleK8sUnresourced()
	if err != nil {
		return err
	}

	for workload, opts := range s.k8sResourceOptions {
		if r, ok := s.k8sByName[workload]; ok {
			r.extraPodSelectors = opts.extraPodSelectors
			r.portForwards = opts.portForwards
			r.triggerMode = opts.triggerMode
			r.resourceDeps = opts.resourceDeps
			if opts.newName != "" && opts.newName != r.name {
				if _, ok := s.k8sByName[opts.newName]; ok {
					return fmt.Errorf("k8s_resource at %s specified to rename '%s' to '%s', but there is already a resource with that name", opts.tiltfilePosition.String(), r.name, opts.newName)
				}
				delete(s.k8sByName, r.name)
				r.name = opts.newName
				s.k8sByName[r.name] = r
			}
		} else {
			var knownResources []string
			for name := range s.k8sByName {
				knownResources = append(knownResources, name)
			}
			return fmt.Errorf("k8s_resource at %s specified unknown resource '%s'. known resources: %s\n\nNote: Tilt's resource naming has recently changed. See https://docs.tilt.dev/resource_assembly_migration.html for more info.", opts.tiltfilePosition.String(), workload, strings.Join(knownResources, ", "))
		}
	}

	for _, r := range s.k8s {
		if err := s.validateK8s(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *tiltfileState) assembleK8sByWorkload() error {
	var workloads, rest []k8s.K8sEntity
	for _, e := range s.k8sUnresourced {
		isWorkload, err := s.isWorkload(e)
		if err != nil {
			return err
		}
		if isWorkload {
			workloads = append(workloads, e)
		} else {
			rest = append(rest, e)
		}
	}
	s.k8sUnresourced = rest

	resourceNames, err := s.calculateResourceNames(workloads)
	if err != nil {
		return err
	}
	for i, resourceName := range resourceNames {
		workload := workloads[i]
		res, err := s.makeK8sResource(resourceName)
		if err != nil {
			return errors.Wrapf(err, "error making resource for workload %s", newK8sObjectID(workload))
		}
		err = res.addEntities([]k8s.K8sEntity{workload}, s.imageJSONPaths, s.envVarImages())
		if err != nil {
			return err
		}

		// find any other entities that match the workload's labels (e.g., services),
		// and move them from unresourced to this resource
		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(workload, s.k8sUnresourced)
		if err != nil {
			return err
		}

		err = res.addEntities(match, s.imageJSONPaths, s.envVarImages())
		if err != nil {
			return err
		}

		s.k8sUnresourced = rest
	}

	return nil
}

func (s *tiltfileState) envVarImages() []container.RefSelector {
	var r []container.RefSelector
	// explicitly don't care about order
	for _, img := range s.buildIndex.images {
		if !img.matchInEnvVars {
			continue
		}
		r = append(r, img.configurationRef)
	}
	return r
}

func (s *tiltfileState) isWorkload(e k8s.K8sEntity) (bool, error) {
	for _, sel := range s.workloadTypes {
		if sel.matches(e) {
			return true, nil
		}
	}

	images, err := e.FindImages(s.imageJSONPaths(e), s.envVarImages())
	if err != nil {
		return false, errors.Wrapf(err, "finding images in %s", e.Name())
	} else {
		return len(images) > 0, nil
	}
}

// assembleK8sWithImages matches images we know how to build with any k8s entities
// that use that image, storing the resulting resource(s) on the tiltfileState.
func (s *tiltfileState) assembleK8sWithImages() error {
	// find all images mentioned in k8s entities that don't yet belong to k8sResources
	k8sRefs, err := s.findUnresourcedImages()
	if err != nil {
		return err
	}

	for _, k8sRef := range k8sRefs {
		image := s.buildIndex.findBuilderForConsumedImage(k8sRef)
		if image == nil {
			// only expand for images we know how to build
			continue
		}

		ref := image.configurationRef
		target, err := s.k8sResourceForImage(ref)
		if err != nil {
			return err
		}
		// find k8s entities that use this image; pull them out of pool of
		// unresourced entities and instead attach them to the target k8sResource
		if err := s.extractEntities(target, ref); err != nil {
			return err
		}
	}

	return nil
}

// assembleK8sUnresourced makes k8sResources for all k8s entities that:
// a. are not already attached to a Tilt resource, and
// b. will result in pods,
// and stores the resulting resource(s) on the tiltfileState.
// (We smartly grouping pod-creating entities with some kinds of
// corresponding entities, e.g. services),
func (s *tiltfileState) assembleK8sUnresourced() error {
	withPodSpec, allRest, err := k8s.FilterByHasPodTemplateSpec(s.k8sUnresourced)
	if err != nil {
		return nil
	}
	for _, e := range withPodSpec {
		target, err := s.k8sResourceForName(e.Name())
		if err != nil {
			return err
		}
		target.entities = append(target.entities, e)

		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(e, allRest)
		if err != nil {
			return err
		}
		target.entities = append(target.entities, match...)
		allRest = rest
	}

	s.k8sUnresourced = allRest

	return nil
}

func (s *tiltfileState) validateK8s(r *k8sResource) error {
	if len(r.entities) == 0 {
		if len(r.refSelectors) > 0 {
			return fmt.Errorf("resource %q: could not find k8s entities matching "+
				"image(s) %q; perhaps there's a typo?",
				r.name, strings.Join(r.refSelectorList(), "; "))
		}
		return fmt.Errorf("resource %q: could not associate any k8s YAML with this resource", r.name)
	}

	for _, ref := range r.imageRefs {
		builder := s.buildIndex.findBuilderForConsumedImage(ref)
		if builder != nil {
			r.dependencyIDs = append(r.dependencyIDs, builder.ID())
		}
	}

	return nil
}

// k8sResourceForImage returns the k8sResource with which this image is associated
// (either an existing resource or a new one).
func (s *tiltfileState) k8sResourceForImage(image container.RefSelector) (*k8sResource, error) {
	// first, look thru all the resources that have already been created,
	// and see if any of them already have a reference to this image.
	for _, r := range s.k8s {
		for _, ref := range r.imageRefs {
			if image.Matches(ref) {
				return r, nil
			}
		}

		for _, selector := range r.refSelectors {
			if image.RefsEqual(selector) {
				return r, nil
			}
		}
	}

	// next, look thru all the resources that have already been created,
	// and see if any of them match the basename of the image.
	refName := image.RefName()
	name := filepath.Base(refName)
	if r, ok := s.k8sByName[name]; ok {
		return r, nil
	}

	// otherwise, create a new resource
	return s.makeK8sResource(name)
}

// k8sResourceForName returns the k8sResource with which this name is associated
// (either an existing resource or a new one).
func (s *tiltfileState) k8sResourceForName(name string) (*k8sResource, error) {
	if r, ok := s.k8sByName[name]; ok {
		return r, nil
	}

	// otherwise, create a new resource
	return s.makeK8sResource(name)
}

func (s *tiltfileState) findUnresourcedImages() ([]reference.Named, error) {
	var result []reference.Named
	seen := make(map[string]bool)

	for _, e := range s.k8sUnresourced {
		images, err := e.FindImages(s.imageJSONPaths(e), s.envVarImages())
		if err != nil {
			return nil, err
		}
		for _, img := range images {
			if !seen[img.String()] {
				result = append(result, img)
				seen[img.String()] = true
			}
		}
	}
	return result, nil
}

// extractEntities extracts k8s entities matching the image ref and stores them on the dest k8sResource
func (s *tiltfileState) extractEntities(dest *k8sResource, imageRef container.RefSelector) error {
	extracted, remaining, err := k8s.FilterByImage(s.k8sUnresourced, imageRef, s.imageJSONPaths, false)
	if err != nil {
		return err
	}

	err = dest.addEntities(extracted, s.imageJSONPaths, s.envVarImages())
	if err != nil {
		return err
	}

	s.k8sUnresourced = remaining

	for _, e := range extracted {
		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(e, s.k8sUnresourced)
		if err != nil {
			return err
		}

		err = dest.addEntities(match, s.imageJSONPaths, s.envVarImages())
		if err != nil {
			return err
		}
		s.k8sUnresourced = rest
	}

	return nil
}

func unmatchedManifestNames(manifests []model.Manifest, requestedManifests []model.ManifestName) []string {
	requestedManifestsByName := make(map[model.ManifestName]bool)
	for _, m := range requestedManifests {
		requestedManifestsByName[m] = true
	}

	var ret []string
	for _, m := range manifests {
		if _, ok := requestedManifestsByName[m.Name]; !ok {
			ret = append(ret, string(m.Name))
		}
	}

	return ret
}

// add `manifestToAdd` and all of its transitive deps to `result`
func addManifestAndDeps(result map[model.ManifestName]bool, allManifestsByName map[model.ManifestName]model.Manifest, manifestToAdd model.ManifestName) {
	if result[manifestToAdd] {
		return
	}
	result[manifestToAdd] = true
	for _, dep := range allManifestsByName[manifestToAdd].ResourceDependencies {
		addManifestAndDeps(result, allManifestsByName, dep)
	}
}

// If the user requested only a subset of manifests, get just those manifests
func match(manifests []model.Manifest, requestedManifests []model.ManifestName) ([]model.Manifest, error) {
	if len(requestedManifests) == 0 {
		return manifests, nil
	}

	manifestsByName := make(map[model.ManifestName]model.Manifest)
	for _, m := range manifests {
		manifestsByName[m.Name] = m
	}

	manifestsToRun := make(map[model.ManifestName]bool)
	var unknownNames []string

	for _, m := range requestedManifests {
		if _, ok := manifestsByName[m]; !ok {
			unknownNames = append(unknownNames, string(m))
			continue
		}

		addManifestAndDeps(manifestsToRun, manifestsByName, m)
	}

	var result []model.Manifest
	for _, m := range manifests {
		if manifestsToRun[m.Name] {
			result = append(result, m)
		}
	}

	if len(unknownNames) > 0 {
		unmatchedNames := unmatchedManifestNames(manifests, requestedManifests)

		return nil, fmt.Errorf(`You specified some resources that could not be found: %s
Is this a typo? Existing resources in Tiltfile: %s`,
			sliceutils.QuotedStringList(unknownNames),
			sliceutils.QuotedStringList(unmatchedNames))
	}

	return result, nil
}

func (s *tiltfileState) translateK8s(resources []*k8sResource) ([]model.Manifest, error) {
	var result []model.Manifest
	for _, r := range resources {
		mn := model.ManifestName(r.name)
		tm, err := starlarkTriggerModeToModel(s.triggerModeForResource(r.triggerMode), true)
		if err != nil {
			return nil, err
		}

		var mds []model.ManifestName
		for _, md := range r.resourceDeps {
			mds = append(mds, model.ManifestName(md))
		}
		m := model.Manifest{
			Name:                 mn,
			TriggerMode:          tm,
			ResourceDependencies: mds,
		}

		k8sTarget, err := k8s.NewTarget(mn.TargetName(), r.entities, r.portForwards, r.extraPodSelectors, r.dependencyIDs, r.imageRefMap)
		if err != nil {
			return nil, err
		}

		m = m.WithDeployTarget(k8sTarget)

		iTargets, err := s.imgTargetsForDependencyIDs(r.dependencyIDs)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", r.name)
		}

		m = m.WithImageTargets(iTargets)

		if !s.features.Get(feature.MultipleContainersPerPod) {
			err = s.checkForImpossibleLiveUpdates(m)
			if err != nil {
				return nil, err
			}
		}

		result = append(result, m)
	}

	return result, nil
}

// checkForImpossibleLiveUpdates logs a warning if the group of image targets contains
// any impossible LiveUpdates (or FastBuilds).
//
// Currently, we only collect container information for the first Tilt-built container
// on the pod (b/c of how we assemble resources, this corresponds to the first image target).
// We won't collect container info on any subsequent containers (i.e. subsequent image
// targets), so will never be able to LiveUpdate them.
func (s *tiltfileState) checkForImpossibleLiveUpdates(m model.Manifest) error {
	g, err := model.NewTargetGraph(m.TargetSpecs())
	if err != nil {
		return err
	}

	for _, iTarg := range m.ImageTargets {
		isDeployed := m.IsImageDeployed(iTarg)

		// This check only applies to images with live updates.
		isInPlaceUpdate := !iTarg.AnyFastBuildInfo().Empty() || !iTarg.AnyLiveUpdateInfo().Empty()
		if !isInPlaceUpdate {
			continue
		}

		// TODO(nick): If an undeployed base image has a live-update component, we
		// should probably emit a different kind of warning.
		if !isDeployed {
			continue
		}

		err = s.validateLiveUpdate(iTarg, g)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *tiltfileState) validateLiveUpdate(iTarget model.ImageTarget, g model.TargetGraph) error {
	lu := iTarget.AnyLiveUpdateInfo()
	if lu.Empty() {
		return nil
	}

	var watchedPaths []string
	err := g.VisitTree(iTarget, func(t model.TargetSpec) error {
		current, ok := t.(model.ImageTarget)
		if !ok {
			return nil
		}

		for _, dep := range current.Dependencies() {
			watchedPaths = append(watchedPaths, dep)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Verify that all a) sync step src's and b) fall_back_on files are children of a watched paths.
	// (If not, we'll never even get "file changed" events for them--they're nonsensical input, throw an error.)
	for _, sync := range lu.SyncSteps() {
		if !ospath.IsChildOfOne(watchedPaths, sync.LocalPath) {
			return fmt.Errorf("sync step source '%s' is not a child of any watched filepaths (%v)",
				sync.LocalPath, watchedPaths)
		}
	}

	for _, path := range lu.FallBackOnFiles().Paths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("internal error: path not resolved correctly! Please report to https://github.com/windmilleng/tilt/issues : %s", path)
		}
		if !ospath.IsChildOfOne(watchedPaths, path) {
			return fmt.Errorf("fall_back_on paths '%s' is not a child of any watched filepaths (%v)",
				path, watchedPaths)
		}

	}

	return nil
}

// Grabs all image targets for the given references,
// as well as any of their transitive dependencies.
func (s *tiltfileState) imgTargetsForDependencyIDs(ids []model.TargetID) ([]model.ImageTarget, error) {
	claimStatus := make(map[model.TargetID]claim, len(ids))
	return s.imgTargetsForDependencyIDsHelper(ids, claimStatus)
}

func (s *tiltfileState) imgTargetsForDependencyIDsHelper(ids []model.TargetID, claimStatus map[model.TargetID]claim) ([]model.ImageTarget, error) {
	iTargets := make([]model.ImageTarget, 0, len(ids))
	for _, id := range ids {
		image := s.buildIndex.findBuilderByID(id)
		if image == nil {
			return nil, fmt.Errorf("Internal error: no image builder found for id %s", id)
		}

		claim := claimStatus[id]
		if claim == claimFinished {
			// Skip this target, an earlier call has already built it
			continue
		} else if claim == claimPending {
			return nil, fmt.Errorf("Image dependency cycle: %s", image.configurationRef)
		}
		claimStatus[id] = claimPending

		iTarget := model.ImageTarget{
			ConfigurationRef: image.configurationRef,
			DeploymentRef:    image.deploymentRef,
			MatchInEnvVars:   image.matchInEnvVars,
		}.WithCachePaths(image.cachePaths)

		if !image.entrypoint.Empty() {
			iTarget = iTarget.WithOverrideCommand(image.entrypoint)
		}

		lu := image.liveUpdate

		switch image.Type() {
		case DockerBuild:
			iTarget = iTarget.WithBuildDetails(model.DockerBuild{
				Dockerfile:  image.dbDockerfile.String(),
				BuildPath:   image.dbBuildPath,
				BuildArgs:   image.dbBuildArgs,
				LiveUpdate:  lu,
				TargetStage: model.DockerBuildTarget(image.targetStage),
			})
		case CustomBuild:
			r := model.CustomBuild{
				Command:     image.customCommand,
				Deps:        image.customDeps,
				Tag:         image.customTag,
				DisablePush: image.disablePush,
				LiveUpdate:  lu,
			}
			iTarget = iTarget.WithBuildDetails(r)
			// TODO(dbentley): validate that syncs is a subset of deps
		case UnknownBuild:
			return nil, fmt.Errorf("no build info for image %s", image.configurationRef)
		}

		iTarget = iTarget.
			WithRepos(s.reposForImage(image)).
			WithDockerignores(s.dockerignoresForImage(image)). // used even for custom build
			WithTiltFilename(image.tiltfilePath).
			WithDependencyIDs(image.dependencyIDs)

		depTargets, err := s.imgTargetsForDependencyIDsHelper(image.dependencyIDs, claimStatus)
		if err != nil {
			return nil, err
		}

		iTargets = append(iTargets, depTargets...)
		iTargets = append(iTargets, iTarget)

		claimStatus[id] = claimFinished
	}
	return iTargets, nil
}

func (s *tiltfileState) translateDC(dc dcResourceSet) ([]model.Manifest, error) {
	var result []model.Manifest

	for _, svc := range dc.services {
		m, configFiles, err := s.dcServiceToManifest(svc, dc)
		if err != nil {
			return nil, err
		}

		iTargets, err := s.imgTargetsForDependencyIDs(svc.DependencyIDs)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", svc.Name)
		}

		for _, iTarg := range iTargets {
			if !iTarg.OverrideCmd.Empty() {
				return nil, fmt.Errorf("docker_build/custom_build.entrypoint not supported for Docker Compose resources")
			}
		}

		m = m.WithImageTargets(iTargets)

		err = s.checkForImpossibleLiveUpdates(m)
		if err != nil {
			return nil, err
		}

		result = append(result, m)

		// TODO(maia): might get config files from dc.yml that are overridden by imageTarget :-/
		// e.g. dc.yml specifies one Dockerfile but the imageTarget specifies another
		s.postExecReadFiles = sliceutils.AppendWithoutDupes(s.postExecReadFiles, configFiles...)
	}
	if len(dc.configPaths) != 0 {
		s.postExecReadFiles = sliceutils.AppendWithoutDupes(s.postExecReadFiles, dc.configPaths...)
	}

	return result, nil
}

func badTypeErr(b *starlark.Builtin, ex interface{}, v starlark.Value) error {
	return fmt.Errorf("%v expects a %T; got %T (%v)", b.Name(), ex, v, v)
}

type claim int

const (
	claimNone claim = iota
	claimPending
	claimFinished
)

// A selector matches an entity if all non-empty selector fields match the corresponding entity fields
type k8sObjectSelector struct {
	apiVersion *regexp.Regexp
	kind       *regexp.Regexp

	name      *regexp.Regexp
	namespace *regexp.Regexp
}

// Creates a new k8sObjectSelector
// If an arg is an empty string, it will become an empty regex that matches all input
func newK8sObjectSelector(apiVersion string, kind string, name string, namespace string) (k8sObjectSelector, error) {
	ret := k8sObjectSelector{}
	var err error

	makeCaseInsensitive := func(s string) string {
		if s == "" {
			return s
		} else {
			return "(?i)" + s
		}
	}

	ret.apiVersion, err = regexp.Compile(makeCaseInsensitive(apiVersion))
	if err != nil {
		return k8sObjectSelector{}, errors.Wrap(err, "error parsing apiVersion regexp")
	}

	ret.kind, err = regexp.Compile(makeCaseInsensitive(kind))
	if err != nil {
		return k8sObjectSelector{}, errors.Wrap(err, "error parsing kind regexp")
	}

	ret.name, err = regexp.Compile(makeCaseInsensitive(name))
	if err != nil {
		return k8sObjectSelector{}, errors.Wrap(err, "error parsing name regexp")
	}

	ret.namespace, err = regexp.Compile(makeCaseInsensitive(namespace))
	if err != nil {
		return k8sObjectSelector{}, errors.Wrap(err, "error parsing namespace regexp")
	}

	return ret, nil
}

func (k k8sObjectSelector) matches(e k8s.K8sEntity) bool {
	gvk := e.GVK()
	return k.apiVersion.MatchString(gvk.GroupVersion().String()) &&
		k.kind.MatchString(gvk.Kind) &&
		k.name.MatchString(e.Name()) &&
		k.namespace.MatchString(e.Namespace().String())
}

func (s *tiltfileState) triggerModeFn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var triggerMode triggerMode
	err := s.unpackArgs(fn.Name(), args, kwargs, "trigger_mode", &triggerMode)
	if err != nil {
		return nil, err
	}

	if s.triggerModeCallPosition.IsValid() {
		return starlark.None, fmt.Errorf("%s can only be called once. It was already called at %s", fn.Name(), s.triggerModeCallPosition.String())
	}

	s.triggerMode = triggerMode
	s.triggerModeCallPosition = thread.CallFrame(1).Pos

	return starlark.None, nil
}

func (s *tiltfileState) setTeam(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var teamName string
	err := s.unpackArgs(fn.Name(), args, kwargs, "team_name", &teamName)
	if err != nil {
		return nil, err
	}

	if len(teamName) == 0 {
		return nil, errors.New("team_name cannot be empty")
	}

	if s.teamName != "" {
		return nil, fmt.Errorf("team_name set multiple times (to '%s' and '%s')", s.teamName, teamName)
	}

	s.teamName = teamName

	return starlark.None, nil
}

func (s *tiltfileState) translateLocal() ([]model.Manifest, error) {
	var result []model.Manifest

	for _, r := range s.localResources {
		mn := model.ManifestName(r.name)
		tm, err := starlarkTriggerModeToModel(s.triggerModeForResource(r.triggerMode), r.autoInit)
		if err != nil {
			return nil, err
		}

		paths := append(r.deps, r.workdir)

		var ignores []model.Dockerignore
		ignoreContents := ignoresToDockerignoreContents(r.ignores)
		if ignoreContents != "" {
			ignores = append(ignores, model.Dockerignore{Contents: ignoreContents, LocalPath: r.workdir})
		}

		lt := model.NewLocalTarget(model.TargetName(r.name), r.cmd, r.workdir, r.deps).
			WithRepos(reposForPaths(paths)).
			WithIgnores(ignores)
		var mds []model.ManifestName
		for _, md := range r.resourceDeps {
			mds = append(mds, model.ManifestName(md))
		}
		m := model.Manifest{
			Name:                 mn,
			TriggerMode:          tm,
			ResourceDependencies: mds,
		}.WithDeployTarget(lt)

		result = append(result, m)
	}

	return result, nil
}

func validateResourceDependencies(ms []model.Manifest) error {
	// make sure that:
	// 1. all deps exist
	// 2. we have a DAG

	knownResources := make(map[model.ManifestName]bool)
	for _, m := range ms {
		knownResources[m.Name] = true
	}

	// construct the graph and make sure all edges are valid
	edges := make(map[interface{}][]interface{})
	for _, m := range ms {
		for _, b := range m.ResourceDependencies {
			if m.Name == b {
				return fmt.Errorf("resource %s specified a dependency on itself", m.Name)
			}
			if _, ok := knownResources[b]; !ok {
				return fmt.Errorf("resource %s specified a dependency on unknown resource %s", m.Name, b)
			}
			edges[m.Name] = append(edges[m.Name], b)
		}
	}

	// check for cycles
	connections := tarjan.Connections(edges)
	for _, g := range connections {
		if len(g) > 1 {
			var nodes []string
			for i := range g {
				nodes = append(nodes, string(g[len(g)-i-1].(model.ManifestName)))
			}
			nodes = append(nodes, string(g[len(g)-1].(model.ManifestName)))
			return fmt.Errorf("cycle detected in resource dependency graph: %s", strings.Join(nodes, " -> "))
		}
	}

	return nil
}

var _ starkit.Extension = &tiltfileState{}
var _ starkit.OnExecExtension = &tiltfileState{}
var _ starkit.OnBuiltinCallExtension = &tiltfileState{}

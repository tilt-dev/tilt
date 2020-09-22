package tiltfile

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/looplab/tarjan"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/tiltfile/analytics"
	"github.com/tilt-dev/tilt/internal/tiltfile/config"
	"github.com/tilt-dev/tilt/internal/tiltfile/dockerprune"
	"github.com/tilt-dev/tilt/internal/tiltfile/encoding"
	"github.com/tilt-dev/tilt/internal/tiltfile/git"
	"github.com/tilt-dev/tilt/internal/tiltfile/include"
	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	tiltfile_k8s "github.com/tilt-dev/tilt/internal/tiltfile/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/k8scontext"
	"github.com/tilt-dev/tilt/internal/tiltfile/metrics"
	"github.com/tilt-dev/tilt/internal/tiltfile/os"
	"github.com/tilt-dev/tilt/internal/tiltfile/secretsettings"
	"github.com/tilt-dev/tilt/internal/tiltfile/shlex"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/starlarkstruct"
	"github.com/tilt-dev/tilt/internal/tiltfile/telemetry"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
	"github.com/tilt-dev/tilt/internal/tiltfile/updatesettings"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
	"github.com/tilt-dev/tilt/internal/tiltfile/watch"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var unmatchedImageNoConfigsWarning = "No Kubernetes or Docker Compose configs found.\n" +
	"Skipping all image builds until we have a place to deploy them"
var unmatchedImageAllUnresourcedWarning = "No Kubernetes configs with images found.\n" +
	"If you are using CRDs, add k8s_kind() to tell Tilt how to find images.\n" +
	"https://docs.tilt.dev/api.html#api.k8s_kind"

type resourceSet struct {
	dc  dcResourceSet // currently only support one d-c.yml
	k8s []*k8sResource
}

type tiltfileState struct {
	// set at creation
	ctx           context.Context
	dcCli         dockercompose.DockerComposeClient
	webHost       model.WebHost
	k8sContextExt k8scontext.Extension
	versionExt    version.Extension
	configExt     *config.Extension
	localRegistry container.Registry
	features      feature.FeatureSet

	// added to during execution
	buildIndex     *buildIndex
	k8sObjectIndex *tiltfile_k8s.State

	// The mutation semantics of these 3 things are a bit fuzzy
	// Objects are moved back and forth between them in different
	// phases of tiltfile execution and post-execution assembly.
	//
	// TODO(nick): Move these into a unified k8sObjectIndex that
	// maintains consistent internal state. Right now the state
	// is duplicated.
	k8s            []*k8sResource
	k8sByName      map[string]*k8sResource
	k8sUnresourced []k8s.K8sEntity

	dc                 dcResourceSet // currently only support one d-c.yml
	k8sResourceOptions map[string]k8sResourceOptions
	localResources     []localResource

	// ensure that any images are pushed to/pulled from this registry, rewriting names if needed
	defaultReg container.Registry

	k8sKinds map[k8s.ObjectSelector]*tiltfile_k8s.KindInfo

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

	teamID string

	secretSettings model.SecretSettings

	logger                           logger.Logger
	warnedDeprecatedResourceAssembly bool

	// postExecReadFiles is generally a mistake -- it means that if tiltfile execution fails,
	// these will never be read. Remove these when you can!!!
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
	webHost model.WebHost,
	k8sContextExt k8scontext.Extension,
	versionExt version.Extension,
	configExt *config.Extension,
	localRegistry container.Registry,
	features feature.FeatureSet) *tiltfileState {
	return &tiltfileState{
		ctx:                        ctx,
		dcCli:                      dcCli,
		webHost:                    webHost,
		k8sContextExt:              k8sContextExt,
		versionExt:                 versionExt,
		configExt:                  configExt,
		localRegistry:              localRegistry,
		buildIndex:                 newBuildIndex(),
		k8sObjectIndex:             tiltfile_k8s.NewState(),
		k8sByName:                  make(map[string]*k8sResource),
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
		secretSettings:             model.DefaultSecretSettings(),
		k8sKinds:                   make(map[k8s.ObjectSelector]*tiltfile_k8s.KindInfo),
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
func (s *tiltfileState) loadManifests(absFilename string, userConfigState model.UserConfigState) ([]model.Manifest, starkit.Model, error) {
	s.logger.Infof("Beginning Tiltfile execution")

	s.configExt.UserConfigState = userConfigState

	dlr, err := tiltextension.NewTempDirDownloader()
	if err != nil {
		return nil, starkit.Model{}, err
	}
	fetcher := tiltextension.NewGithubFetcher(dlr)

	result, err := starkit.ExecFile(absFilename,
		s,
		include.IncludeFn{},
		git.NewExtension(),
		os.NewExtension(),
		io.NewExtension(),
		s.k8sContextExt,
		dockerprune.NewExtension(),
		analytics.NewExtension(),
		s.versionExt,
		s.configExt,
		starlarkstruct.NewExtension(),
		telemetry.NewExtension(),
		metrics.NewExtension(),
		updatesettings.NewExtension(),
		secretsettings.NewExtension(),
		encoding.NewExtension(),
		shlex.NewExtension(),
		watch.NewExtension(),
		tiltextension.NewExtension(fetcher, tiltextension.NewLocalStore(filepath.Dir(absFilename))),
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

	err = s.validateLiveUpdatesForManifests(manifests)
	if err != nil {
		return nil, result, err
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

	configSettings, _ := config.GetState(result)
	manifests, err = configSettings.EnabledResources(manifests)
	if err != nil {
		return nil, starkit.Model{}, err
	}

	if len(unresourced) > 0 {
		yamlManifest, err := k8s.NewK8sOnlyManifest(model.UnresourcedYAMLManifestName, unresourced, s.k8sImageLocatorsList())
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
	localN     = "local"
	kustomizeN = "kustomize"
	helmN      = "helm"

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

// count how many times each Builtin is called, for analytics
func (s *tiltfileState) OnBuiltinCall(name string, fn *starlark.Builtin) {
	s.builtinCallCounts[name]++
}

func (s *tiltfileState) OnExec(t *starlark.Thread, tiltfilePath string) error {
	return nil
}

// wrap a builtin such that it's only allowed to run when we have a known safe k8s context
// (none (e.g., docker-compose), local, or specified by `allow_k8s_contexts`)
func (s *tiltfileState) potentiallyK8sUnsafeBuiltin(f starkit.Function) starkit.Function {
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
	e.SetContext(s.ctx)

	for _, b := range []struct {
		name    string
		builtin starkit.Function
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

	err = s.assertAllImagesMatched()
	if err != nil {
		s.logger.Warnf("%s", err.Error())
	}

	return resourceSet{
		dc:  s.dc,
		k8s: s.k8s,
	}, s.k8sUnresourced, nil
}

// Emit an error if there are unmatches images.
//
// There are 4 mistakes people commonly make if they
// have unmatched images:
// 1) They didn't include any Kubernetes or Docker Compose configs at all.
// 2) They included Kubernetes configs, but they're custom resources
//    and Tilt can't infer the image.
// 3) They typo'd the image name, and need help finding the right name.
// 4) The tooling they're using to generating the k8s resources
//    isn't generating what they expect.
//
// This function intends to help with cases (1)-(3).
// Long-term, we want to have better tooling to help with (4),
// like being able to see k8s resources as they move thru
// the build system.
func (s *tiltfileState) assertAllImagesMatched() error {
	unmatchedImages := s.buildIndex.unmatchedImages()
	if len(unmatchedImages) == 0 {
		return nil
	}

	if len(s.dc.services) == 0 && len(s.k8s) == 0 && len(s.k8sUnresourced) == 0 {
		return fmt.Errorf(unmatchedImageNoConfigsWarning)
	}

	if len(s.k8s) == 0 && len(s.k8sUnresourced) != 0 {
		return fmt.Errorf(unmatchedImageAllUnresourcedWarning)
	}

	configType := "Kubernetes"
	if len(s.dc.services) > 0 {
		configType = "Docker Compose"
	}
	return s.buildIndex.unmatchedImageWarning(unmatchedImages[0], configType)
}

func (s *tiltfileState) assembleImages() error {
	for _, imageBuilder := range s.buildIndex.images {
		var err error

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
	if len(s.dc.services) > 0 && !s.defaultReg.Empty() {
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

	resourcedEntities := []k8s.K8sEntity{}
	for _, r := range s.k8sByName {
		resourcedEntities = append(resourcedEntities, r.entities...)
	}

	allEntities := append(resourcedEntities, s.k8sUnresourced...)

	fragmentsToEntities := k8s.FragmentsToEntities(allEntities)

	fullNames := make([]string, len(allEntities))
	for i, e := range allEntities {
		fullNames[i] = fullNameFromK8sEntity(e)
	}

	for workload, opts := range s.k8sResourceOptions {
		if opts.manuallyGrouped {
			r, err := s.makeK8sResource(opts.newName)
			if err != nil {
				return err
			}
			r.manuallyGrouped = true
			s.k8sByName[opts.newName] = r
		}
		if r, ok := s.k8sByName[workload]; ok {
			r.extraPodSelectors = opts.extraPodSelectors
			r.podReadinessMode = opts.podReadinessMode
			r.portForwards = opts.portForwards
			r.triggerMode = opts.triggerMode
			r.autoInit = opts.autoInit
			r.resourceDeps = opts.resourceDeps
			if opts.newName != "" && opts.newName != r.name {
				if _, ok := s.k8sByName[opts.newName]; ok {
					return fmt.Errorf("k8s_resource at %s specified to rename %q to %q, but there already exists a resource with that name", opts.tiltfilePosition.String(), r.name, opts.newName)
				}
				delete(s.k8sByName, r.name)
				r.name = opts.newName
				s.k8sByName[r.name] = r
			}

			selectors := make([]k8s.ObjectSelector, len(opts.objects))
			for i, o := range opts.objects {
				s, err := k8s.SelectorFromString(o)
				if err != nil {
					return errors.Wrapf(err, "Error making selector from string %q", o)
				}
				selectors[i] = s
			}

			for i, o := range opts.objects {
				entities, ok := fragmentsToEntities[strings.ToLower(o)]
				if !ok || len(entities) == 0 {
					return fmt.Errorf("No object identified by the fragment %q could be found. Possible objects are: %s", o, sliceutils.QuotedStringList(fullNames))
				}
				if len(entities) > 1 {
					matchingObjects := make([]string, len(entities))
					for i, e := range entities {
						matchingObjects[i] = fullNameFromK8sEntity(e)
					}
					return fmt.Errorf("%q is not a unique fragment. Objects that match %q are %s", o, o, sliceutils.QuotedStringList(matchingObjects))
				}

				entitiesToRemove := filterEntitiesBySelector(s.k8sUnresourced, selectors[i])
				if len(entitiesToRemove) == 0 {
					// we've already taken these entities out of unresourced
					remainingUnresourced := make([]string, len(s.k8sUnresourced))
					for i, entity := range s.k8sUnresourced {
						remainingUnresourced[i] = fullNameFromK8sEntity(entity)
					}
					return fmt.Errorf("No object identified by the fragment %q could be found in remaining YAML. Valid remaining fragments are: %s", o, sliceutils.QuotedStringList(remainingUnresourced))
				}
				if len(entitiesToRemove) > 1 {
					panic(fmt.Sprintf("Fragment %q matches %d resources. Each object fragment must match exactly 1 resource. This should NOT be possible at this point in the code, we should have already checked that this fragment was unique", o, len(entitiesToRemove)))
				}

				s.addEntityToResourceAndRemoveFromUnresourced(entitiesToRemove[0], r)
			}

		} else {
			var knownResources []string
			for name := range s.k8sByName {
				knownResources = append(knownResources, name)
			}
			return fmt.Errorf("k8s_resource at %s specified unknown resource %q. known resources: %s\n\nNote: Tilt's resource naming has recently changed. See https://docs.tilt.dev/resource_assembly_migration.html for more info", opts.tiltfilePosition.String(), workload, strings.Join(knownResources, ", "))
		}
	}

	for _, r := range s.k8s {
		if err := s.validateK8s(r); err != nil {
			return err
		}
	}
	return nil
}

// NOTE(dmiller): This isn't _technically_ a fullname since it is missing "group" (core, apps, data, etc)
// A true full name would look like "foo:secret:mynamespace:core"
// However because we
// a) couldn't think of a concrete case where you would need to specify group
// b) being able to do so would make things more complicated, like in the case where you want to specify the group of
//    a cluster scoped object but are unable to specify the namespace (e.g. foo:clusterrole::rbac.authorization.k8s.io)
//
// we decided to leave it off for now. When we encounter a concrete use case for specifying group it shouldn't be too
// hard to add it here and in the docs.
func fullNameFromK8sEntity(e k8s.K8sEntity) string {
	return k8s.SelectorStringFromParts([]string{e.Name(), e.GVK().Kind, e.Namespace().String()})
}

func filterEntitiesBySelector(entities []k8s.K8sEntity, sel k8s.ObjectSelector) []k8s.K8sEntity {
	ret := []k8s.K8sEntity{}

	for _, e := range entities {
		if sel.Matches(e) {
			ret = append(ret, e)
		}
	}

	return ret
}

func (s *tiltfileState) addEntityToResourceAndRemoveFromUnresourced(e k8s.K8sEntity, r *k8sResource) {
	r.entities = append(r.entities, e)
	for i, ur := range s.k8sUnresourced {
		if ur == e {
			// delete from unresourced
			s.k8sUnresourced = append(s.k8sUnresourced[:i], s.k8sUnresourced[i+1:]...)
			return
		}
	}

	panic("Unable to find entity in unresourced YAML after checking that it was there. This should never happen")
}

func (s *tiltfileState) assembleK8sByWorkload() error {
	locators := s.k8sImageLocatorsList()

	var workloads, rest []k8s.K8sEntity
	for _, e := range s.k8sUnresourced {
		isWorkload, err := s.isWorkload(e, locators)
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
		err = res.addEntities([]k8s.K8sEntity{workload}, locators, s.envVarImages())
		if err != nil {
			return err
		}

		// find any other entities that match the workload's labels (e.g., services),
		// and move them from unresourced to this resource
		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(workload, s.k8sUnresourced)
		if err != nil {
			return err
		}

		err = res.addEntities(match, locators, s.envVarImages())
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

func (s *tiltfileState) isWorkload(e k8s.K8sEntity, locators []k8s.ImageLocator) (bool, error) {
	for sel := range s.k8sKinds {
		if sel.Matches(e) {
			return true, nil
		}
	}

	images, err := e.FindImages(locators, s.envVarImages())
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

	locators := s.k8sImageLocatorsList()
	for _, e := range s.k8sUnresourced {
		images, err := e.FindImages(locators, s.envVarImages())
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
	locators := s.k8sImageLocatorsList()
	extracted, remaining, err := k8s.FilterByImage(s.k8sUnresourced, imageRef, locators, false)
	if err != nil {
		return err
	}

	err = dest.addEntities(extracted, locators, s.envVarImages())
	if err != nil {
		return err
	}

	s.k8sUnresourced = remaining

	for _, e := range extracted {
		match, rest, err := k8s.FilterByMatchesPodTemplateSpec(e, s.k8sUnresourced)
		if err != nil {
			return err
		}

		err = dest.addEntities(match, locators, s.envVarImages())
		if err != nil {
			return err
		}
		s.k8sUnresourced = rest
	}

	return nil
}

// decideRegistry returns the image registry we should use; if detected, a pre-configured
// local registry; otherwise, the registry specified by the user via default_registry.
// Otherwise, we'll return the zero value of `s.defaultReg`, which is an empty registry.
// It has side-effects (a log line) and so should only be called once.
func (s *tiltfileState) decideRegistry() container.Registry {
	if s.orchestrator() == model.OrchestratorK8s && !s.localRegistry.Empty() {
		// If we've found a local registry in the cluster at run-time, use that
		// instead of the default_registry (if any) declared in the Tiltfile
		s.logger.Infof("Auto-detected local registry from environment: %s", s.localRegistry)
		return s.localRegistry
	}
	return s.defaultReg
}

// Auto-infer the readiness mode
//
// CONVO:
// jazzdan: This still feels overloaded to me
// nicks: i think whenever we define a new CRD, we need to know:

// how to find the images in it
// how to find any pods it deploys (if they can't be found by owner references)
// if it should not expect pods at all (e.g., PostgresVersion)
// if it should wait for the pods to be ready before building the next resource (e.g., servers)
// if it should wait for the pods to be complete before building the next resource (e.g., jobs)
// and it's complicated a bit by the fact that there are both normal CRDs where the image shows up in the same place each time, and more meta CRDs (like HelmRelease) where it might appear in different places
//
// feels like wer're still doing this very ad-hoc rather than holistically
func (s *tiltfileState) inferPodReadinessMode(r *k8sResource) model.PodReadinessMode {
	// The mode set directly on the resource has highest priority.
	if r.podReadinessMode != model.PodReadinessNone {
		return r.podReadinessMode
	}

	// Next, check if any of the k8s kinds have a mode.
	hasWaitMode := false
	hasIgnoreMode := false
	for _, e := range r.entities {
		for sel, info := range s.k8sKinds {
			if sel.Matches(e) {
				if info.PodReadinessMode == model.PodReadinessWait {
					hasWaitMode = true
				}

				if info.PodReadinessMode == model.PodReadinessIgnore {
					hasIgnoreMode = true
				}
			}
		}
	}

	if hasWaitMode {
		return model.PodReadinessWait
	}

	if hasIgnoreMode {
		return model.PodReadinessIgnore
	}

	// Auto-infer based on context
	//
	// If the resource was
	// 1) manually grouped (i.e., we didn't find any images in it)
	// 2) doesn't have pod selectors, and
	// 3) doesn't depend on images
	// assume that it will never create pods.
	if r.manuallyGrouped && len(r.extraPodSelectors) == 0 && len(r.dependencyIDs) == 0 {
		return model.PodReadinessIgnore
	}

	return model.PodReadinessWait
}

func (s *tiltfileState) translateK8s(resources []*k8sResource) ([]model.Manifest, error) {
	var result []model.Manifest
	locators := s.k8sImageLocatorsList()
	registry := s.decideRegistry()
	for _, r := range resources {
		mn := model.ManifestName(r.name)
		tm, err := starlarkTriggerModeToModel(s.triggerModeForResource(r.triggerMode), r.autoInit)
		if err != nil {
			return nil, errors.Wrapf(err, "error in resource %s options", mn)
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

		k8sTarget, err := k8s.NewTarget(mn.TargetName(), r.entities, s.defaultedPortForwards(r.portForwards),
			r.extraPodSelectors, r.dependencyIDs, r.imageRefMap, s.inferPodReadinessMode(r), locators)
		if err != nil {
			return nil, err
		}

		m = m.WithDeployTarget(k8sTarget)

		iTargets, err := s.imgTargetsForDependencyIDs(r.dependencyIDs, registry)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", r.name)
		}

		m = m.WithImageTargets(iTargets)

		result = append(result, m)
	}

	err := maybeRestartContainerDeprecationError(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Fill in default values in port-forwarding.
//
// In Kubernetes, "defaulted" is used as a verb to say "if a YAML value of a specification
// was left blank, the API server should fill in the value with a default". See:
//
// https://kubernetes.io/docs/tasks/manage-kubernetes-objects/declarative-config/#default-field-values
//
// In Tilt, we typically do this in the Tiltfile loader post-execution.
// Here, we default the port-forward Host to the WebHost.
//
// TODO(nick): I think the "right" way to do this is to give the starkit extension system
// a "default"-ing hook that runs post-execution.
func (s *tiltfileState) defaultedPortForwards(pfs []model.PortForward) []model.PortForward {
	result := make([]model.PortForward, 0, len(pfs))
	for _, pf := range pfs {
		if pf.Host == "" {
			pf.Host = string(s.webHost)
		}
		result = append(result, pf)
	}
	return result
}

func (s *tiltfileState) validateLiveUpdatesForManifests(manifests []model.Manifest) error {
	for _, m := range manifests {
		err := s.validateLiveUpdatesForManifest(m)
		if err != nil {
			return err
		}
	}
	return nil
}

// validateLiveUpdatesForManifest checks any image targets on the
// given manifest the contain any illegal LiveUpdates
func (s *tiltfileState) validateLiveUpdatesForManifest(m model.Manifest) error {
	g, err := model.NewTargetGraph(m.TargetSpecs())
	if err != nil {
		return err
	}

	for _, iTarg := range m.ImageTargets {
		isDeployed := m.IsImageDeployed(iTarg)

		// This check only applies to images with live updates.
		if iTarg.LiveUpdateInfo().Empty() {
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
	lu := iTarget.LiveUpdateInfo()
	if lu.Empty() {
		return nil
	}

	var watchedPaths []string
	err := g.VisitTree(iTarget, func(t model.TargetSpec) error {
		current, ok := t.(model.ImageTarget)
		if !ok {
			return nil
		}

		watchedPaths = append(watchedPaths, current.Dependencies()...)
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
			return fmt.Errorf("internal error: path not resolved correctly! Please report to https://github.com/tilt-dev/tilt/issues : %s", path)
		}
		if !ospath.IsChildOfOne(watchedPaths, path) {
			return fmt.Errorf("fall_back_on paths '%s' is not a child of any watched filepaths (%v)",
				path, watchedPaths)
		}
	}

	return nil
}

func maybeRestartContainerDeprecationError(manifests []model.Manifest) error {
	var needsError []model.ManifestName
	for _, m := range manifests {
		if needsRestartContainerDeprecationError(m) {
			needsError = append(needsError, m.Name)
		}
	}

	if len(needsError) > 0 {
		return fmt.Errorf("%s", restartContainerDeprecationError(needsError))
	}
	return nil
}
func needsRestartContainerDeprecationError(m model.Manifest) bool {
	// 7/2/20: we've deprecated restart_container() in favor of the restart_process extension.
	// If this is a k8s resource with a restart_container step, throw a deprecation error.
	// (restart_container is still allowed for Docker Compose resources)
	if !m.IsK8s() {
		return false
	}

	for _, iTarg := range m.ImageTargets {
		if iTarg.LiveUpdateInfo().ShouldRestart() {
			return true
		}
	}

	return false
}

// Grabs all image targets for the given references,
// as well as any of their transitive dependencies.
func (s *tiltfileState) imgTargetsForDependencyIDs(ids []model.TargetID, reg container.Registry) ([]model.ImageTarget, error) {
	claimStatus := make(map[model.TargetID]claim, len(ids))
	return s.imgTargetsForDependencyIDsHelper(ids, claimStatus, reg)
}

func (s *tiltfileState) imgTargetsForDependencyIDsHelper(ids []model.TargetID, claimStatus map[model.TargetID]claim, reg container.Registry) ([]model.ImageTarget, error) {
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

		refs, err := container.NewRefSet(image.configurationRef, reg)
		if err != nil {
			return nil, errors.Wrapf(err, "Something went wrong deriving "+
				"references for your image: %q. Check the image name (and your "+
				"`default_registry()` call, if any) for errors", image.configurationRef)
		}

		iTarget := model.ImageTarget{
			Refs:           refs,
			MatchInEnvVars: image.matchInEnvVars,
		}

		if !image.entrypoint.Empty() {
			iTarget = iTarget.WithOverrideCommand(image.entrypoint)
		}

		if image.containerArgs.ShouldOverride {
			iTarget.OverrideArgs = image.containerArgs
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
				SSHSpecs:    image.sshSpecs,
				SecretSpecs: image.secretSpecs,
				Network:     image.network,
				CacheFrom:   image.cacheFrom,
				PullParent:  image.pullParent,
				ExtraTags:   image.extraTags,
			})
		case CustomBuild:
			r := model.CustomBuild{
				WorkDir:           image.workDir,
				Command:           image.customCommand,
				Deps:              image.customDeps,
				Tag:               image.customTag,
				DisablePush:       image.disablePush,
				SkipsLocalDocker:  image.skipsLocalDocker,
				OutputsImageRefTo: image.outputsImageRefTo,
				LiveUpdate:        lu,
			}
			iTarget = iTarget.WithBuildDetails(r).
				MaybeIgnoreRegistry()

			// TODO(dbentley): validate that syncs is a subset of deps
		case UnknownBuild:
			return nil, fmt.Errorf("no build info for image %s", image.configurationRef.RefFamiliarString())
		}

		dIgnores, err := s.dockerignoresForImage(image)
		if err != nil {
			return nil, fmt.Errorf("Reading dockerignore for %s: %v", image.configurationRef.RefFamiliarString(), err)
		}

		iTarget = iTarget.
			WithRepos(s.reposForImage(image)).
			WithDockerignores(dIgnores). // used even for custom build
			WithTiltFilename(image.workDir).
			WithDependencyIDs(image.dependencyIDs)

		depTargets, err := s.imgTargetsForDependencyIDsHelper(image.dependencyIDs, claimStatus, reg)
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
		m, err := s.dcServiceToManifest(svc, dc)
		if err != nil {
			return nil, err
		}

		iTargets, err := s.imgTargetsForDependencyIDs(svc.DependencyIDs, container.Registry{}) // Registry not relevant to DC
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", svc.Name)
		}

		for _, iTarg := range iTargets {
			if !iTarg.OverrideCmd.Empty() {
				return nil, fmt.Errorf("docker_build/custom_build.entrypoint not supported for Docker Compose resources")
			}
		}

		m = m.WithImageTargets(iTargets)

		result = append(result, m)
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
	var teamID string
	err := s.unpackArgs(fn.Name(), args, kwargs, "team_id", &teamID)
	if err != nil {
		return nil, err
	}

	if len(teamID) == 0 {
		return nil, errors.New("team_id cannot be empty")
	}

	if s.teamID != "" {
		return nil, fmt.Errorf("team_id set multiple times (to '%s' and '%s')", s.teamID, teamID)
	}

	s.teamID = teamID

	return starlark.None, nil
}

func (s *tiltfileState) translateLocal() ([]model.Manifest, error) {
	var result []model.Manifest

	for _, r := range s.localResources {
		mn := model.ManifestName(r.name)
		tm, err := starlarkTriggerModeToModel(s.triggerModeForResource(r.triggerMode), r.autoInit)
		if err != nil {
			return nil, errors.Wrapf(err, "error in resource %s options", mn)
		}

		paths := append(r.deps, r.workdir)

		var ignores []model.Dockerignore
		if len(r.ignores) != 0 {
			ignores = append(ignores, model.Dockerignore{
				Patterns:  r.ignores,
				Source:    fmt.Sprintf("local_resource(%q)", r.name),
				LocalPath: r.workdir,
			})
		}

		lt := model.NewLocalTarget(model.TargetName(r.name), r.updateCmd, r.serveCmd, r.deps, r.workdir).
			WithRepos(reposForPaths(paths)).
			WithIgnores(ignores).
			WithAllowParallel(r.allowParallel)
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

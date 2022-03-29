package tiltfile

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/looplab/tarjan"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"golang.org/x/mod/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cmdimage"
	"github.com/tilt-dev/tilt/internal/controllers/apis/dockerimage"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/apiset"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/tiltfile/hasher"
	"github.com/tilt-dev/tilt/internal/tiltfile/links"
	"github.com/tilt-dev/tilt/internal/tiltfile/print"
	"github.com/tilt-dev/tilt/internal/tiltfile/probe"
	"github.com/tilt-dev/tilt/internal/tiltfile/sys"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
	"github.com/tilt-dev/tilt/pkg/apis"

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
	"github.com/tilt-dev/tilt/internal/tiltfile/loaddynamic"
	"github.com/tilt-dev/tilt/internal/tiltfile/metrics"
	"github.com/tilt-dev/tilt/internal/tiltfile/os"
	"github.com/tilt-dev/tilt/internal/tiltfile/secretsettings"
	"github.com/tilt-dev/tilt/internal/tiltfile/shlex"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/starlarkstruct"
	"github.com/tilt-dev/tilt/internal/tiltfile/telemetry"
	"github.com/tilt-dev/tilt/internal/tiltfile/updatesettings"
	tfv1alpha1 "github.com/tilt-dev/tilt/internal/tiltfile/v1alpha1"
	"github.com/tilt-dev/tilt/internal/tiltfile/version"
	"github.com/tilt-dev/tilt/internal/tiltfile/watch"
	fwatch "github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var unmatchedImageNoConfigsWarning = "No Kubernetes or Docker Compose configs found.\n" +
	"Skipping all image builds until we have a place to deploy them"
var unmatchedImageAllUnresourcedWarning = "No Kubernetes configs with images found.\n" +
	"If you are using CRDs, add k8s_kind() to tell Tilt how to find images.\n" +
	"https://docs.tilt.dev/api.html#api.k8s_kind"

var pkgInitTime = time.Now()

type resourceSet struct {
	dc  dcResourceSet // currently only support one d-c.yml
	k8s []*k8sResource
}

type tiltfileState struct {
	// set at creation
	ctx              context.Context
	dcCli            dockercompose.DockerComposeClient
	webHost          model.WebHost
	execer           localexec.Execer
	k8sContextPlugin k8scontext.Plugin
	versionPlugin    version.Plugin
	configPlugin     *config.Plugin
	extensionPlugin  *tiltextension.Plugin
	features         feature.FeatureSet

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

	dc           dcResourceSet // currently only support one d-c.yml
	dcByName     map[string]*dcService
	dcResOptions map[string]*dcResourceOptions

	k8sResourceOptions []k8sResourceOptions
	localResources     []*localResource
	localByName        map[string]*localResource

	// ensure that any images are pushed to/pulled from this registry, rewriting names if needed
	defaultReg container.Registry

	k8sKinds map[k8s.ObjectSelector]*tiltfile_k8s.KindInfo

	workloadToResourceFunction workloadToResourceFunction

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

	apiObjects apiset.ObjectSet

	logger logger.Logger

	// postExecReadFiles is generally a mistake -- it means that if tiltfile execution fails,
	// these will never be read. Remove these when you can!!!
	postExecReadFiles []string

	// Temporary directory for storing generated artifacts during the lifetime of the tiltfile context.
	// The directory is recursively deleted when the context is done.
	scratchDir *fwatch.TempDir
}

func newTiltfileState(
	ctx context.Context,
	dcCli dockercompose.DockerComposeClient,
	webHost model.WebHost,
	execer localexec.Execer,
	k8sContextPlugin k8scontext.Plugin,
	versionPlugin version.Plugin,
	configPlugin *config.Plugin,
	extensionPlugin *tiltextension.Plugin,
	features feature.FeatureSet) *tiltfileState {
	return &tiltfileState{
		ctx:                       ctx,
		dcCli:                     dcCli,
		webHost:                   webHost,
		execer:                    execer,
		k8sContextPlugin:          k8sContextPlugin,
		versionPlugin:             versionPlugin,
		configPlugin:              configPlugin,
		extensionPlugin:           extensionPlugin,
		buildIndex:                newBuildIndex(),
		k8sObjectIndex:            tiltfile_k8s.NewState(),
		k8sByName:                 make(map[string]*k8sResource),
		dcByName:                  make(map[string]*dcService),
		dcResOptions:              make(map[string]*dcResourceOptions),
		localByName:               make(map[string]*localResource),
		usedImages:                make(map[string]bool),
		logger:                    logger.Get(ctx),
		builtinCallCounts:         make(map[string]int),
		builtinArgCounts:          make(map[string]map[string]int),
		unconsumedLiveUpdateSteps: make(map[string]liveUpdateStep),
		localResources:            []*localResource{},
		triggerMode:               TriggerModeAuto,
		features:                  features,
		secretSettings:            model.DefaultSecretSettings(),
		apiObjects:                apiset.ObjectSet{},
		k8sKinds:                  tiltfile_k8s.InitialKinds(),
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
func (s *tiltfileState) loadManifests(tf *v1alpha1.Tiltfile) ([]model.Manifest, starkit.Model, error) {
	s.logger.Infof("Loading Tiltfile at: %s", tf.Spec.Path)

	result, err := starkit.ExecFile(tf,
		s,
		include.IncludeFn{},
		git.NewPlugin(),
		os.NewPlugin(),
		sys.NewPlugin(),
		io.NewPlugin(),
		s.k8sContextPlugin,
		dockerprune.NewPlugin(),
		analytics.NewPlugin(),
		s.versionPlugin,
		s.configPlugin,
		starlarkstruct.NewPlugin(),
		telemetry.NewPlugin(),
		metrics.NewPlugin(),
		updatesettings.NewPlugin(),
		secretsettings.NewPlugin(),
		encoding.NewPlugin(),
		shlex.NewPlugin(),
		watch.NewPlugin(),
		loaddynamic.NewPlugin(),
		s.extensionPlugin,
		links.NewPlugin(),
		print.NewPlugin(),
		probe.NewPlugin(),
		tfv1alpha1.NewPlugin(),
		hasher.NewPlugin(),
	)
	if err != nil {
		return nil, result, starkit.UnpackBacktrace(err)
	}

	resources, unresourced, err := s.assemble()
	if err != nil {
		return nil, result, err
	}

	us, err := updatesettings.GetState(result)
	if err != nil {
		return nil, result, err
	}

	err = s.assertAllImagesMatched(us)
	if err != nil {
		s.logger.Warnf("%s", err.Error())
	}

	manifests := []model.Manifest{}
	k8sContextState, err := k8scontext.GetState(result)
	if err != nil {
		return nil, result, err
	}

	if len(resources.k8s) > 0 || len(unresourced) > 0 {
		ms, err := s.translateK8s(resources.k8s, us)
		if err != nil {
			return nil, result, err
		}
		manifests = append(manifests, ms...)

		isAllowed := k8sContextState.IsAllowed(tf)
		if !isAllowed {
			kubeContext := k8sContextState.KubeContext()
			return nil, result, fmt.Errorf(`Stop! %s might be production.
If you're sure you want to deploy there, add:
	allow_k8s_contexts('%s')
to your Tiltfile. Otherwise, switch k8s contexts and restart Tilt.`, kubeContext, kubeContext)
		}
	}

	if !resources.dc.Empty() {
		if err := s.validateDockerComposeVersion(); err != nil {
			return nil, result, err
		}

		ms, err := s.translateDC(resources.dc)
		if err != nil {
			return nil, result, err
		}
		manifests = append(manifests, ms...)
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

	if len(unresourced) > 0 {
		mn := model.UnresourcedYAMLManifestName
		r := &k8sResource{
			name:             mn.String(),
			entities:         unresourced,
			podReadinessMode: model.PodReadinessIgnore,
		}
		kt, err := s.k8sDeployTarget(mn.TargetName(), r, nil, us)
		if err != nil {
			return nil, starkit.Model{}, err
		}

		yamlManifest := model.Manifest{Name: mn}.WithDeployTarget(kt)
		manifests = append(manifests, yamlManifest)
	}

	err = s.validateResourceDependencies(manifests)
	if err != nil {
		return nil, starkit.Model{}, err
	}

	for i := range manifests {
		// ensure all manifests have a label indicating they're owned
		// by the Tiltfile - some reconcilers have special handling
		l := manifests[i].Labels
		if l == nil {
			l = make(map[string]string)
		}
		manifests[i] = manifests[i].WithLabels(l)

		err := manifests[i].Validate()
		if err != nil {
			// Even on manifest validation errors, we may be able
			// to use other kinds of models (e.g., watched files)
			return manifests, result, err
		}
	}

	return manifests, result, nil
}

// Builtin functions

const (
	// build functions
	dockerBuildN     = "docker_build"
	customBuildN     = "custom_build"
	defaultRegistryN = "default_registry"

	// docker compose functions
	dockerComposeN = "docker_compose"
	dcResourceN    = "dc_resource"

	// k8s functions
	k8sYamlN                    = "k8s_yaml"
	filterYamlN                 = "filter_yaml"
	k8sResourceN                = "k8s_resource"
	portForwardN                = "port_forward"
	k8sKindN                    = "k8s_kind"
	k8sImageJSONPathN           = "k8s_image_json_path"
	workloadToResourceFunctionN = "workload_to_resource_function"
	k8sCustomDeployN            = "k8s_custom_deploy"

	// local resource functions
	localResourceN = "local_resource"
	testN          = "test" // a deprecated fork of local resource

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
			return model.TriggerModeAutoWithManualInit, nil
		}
		return model.TriggerModeAuto, nil
	case TriggerModeManual:
		if autoInit {
			return model.TriggerModeManualWithAutoInit, nil
		} else {
			return model.TriggerModeManual, nil
		}
	default:
		return 0, fmt.Errorf("unknown triggerMode %v", triggerMode)
	}
}

// count how many times each Builtin is called, for analytics
func (s *tiltfileState) OnBuiltinCall(name string, fn *starlark.Builtin) {
	s.builtinCallCounts[name]++
}

func (s *tiltfileState) OnExec(t *starlark.Thread, tiltfilePath string, contents []byte) error {
	return nil
}

// wrap a builtin such that it's only allowed to run when we have a known safe k8s context
// (none (e.g., docker-compose), local, or specified by `allow_k8s_contexts`)
func (s *tiltfileState) potentiallyK8sUnsafeBuiltin(f starkit.Function) starkit.Function {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		tf, err := starkit.StartTiltfileFromThread(thread)
		if err != nil {
			return nil, err
		}

		model, err := starkit.ModelFromThread(thread)
		if err != nil {
			return nil, err
		}

		k8sContextState, err := k8scontext.GetState(model)
		if err != nil {
			return nil, err
		}

		isAllowed := k8sContextState.IsAllowed(tf)
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

// TODO(nick): Split these into separate plugins
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
		{customBuildN, s.customBuild},
		{defaultRegistryN, s.defaultRegistry},
		{dockerComposeN, s.dockerCompose},
		{dcResourceN, s.dcResource},
		{k8sYamlN, s.k8sYaml},
		{filterYamlN, s.filterYaml},
		{k8sResourceN, s.k8sResource},
		{k8sCustomDeployN, s.k8sCustomDeploy},
		{localResourceN, s.localResource},
		{testN, s.localResource},
		{portForwardN, s.portForward},
		{k8sKindN, s.k8sKind},
		{k8sImageJSONPathN, s.k8sImageJsonPath},
		{workloadToResourceFunctionN, s.workloadToResourceFunctionFn},
		{kustomizeN, s.kustomize},
		{helmN, s.helm},
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

func (s *tiltfileState) assemble() (resourceSet, []k8s.K8sEntity, error) {
	err := s.assembleImages()
	if err != nil {
		return resourceSet{}, nil, err
	}

	err = s.assembleK8s()
	if err != nil {
		return resourceSet{}, nil, err
	}

	err = s.assembleDC()
	if err != nil {
		return resourceSet{}, nil, err
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
func (s *tiltfileState) assertAllImagesMatched(us model.UpdateSettings) error {
	unmatchedImages := s.buildIndex.unmatchedImages()
	unmatchedImages = filterUnmatchedImages(us, unmatchedImages)
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
		if imageBuilder.dbDockerfile != "" {
			depImages, err := imageBuilder.dbDockerfile.FindImages(imageBuilder.dbBuildArgs)
			if err != nil {
				return err
			}
			for _, depImage := range depImages {
				depBuilder := s.buildIndex.findBuilderForConsumedImage(depImage)
				if depBuilder == nil {
					// Images in the Dockerfile that don't have docker_build
					// instructions are OK. We'll pull them as prebuilt images.
					continue
				}

				imageBuilder.imageMapDeps = append(imageBuilder.imageMapDeps, depBuilder.ImageMapName())
			}
		}

		for _, depImage := range imageBuilder.customImgDeps {
			depBuilder := s.buildIndex.findBuilderForConsumedImage(depImage)
			if depBuilder == nil {
				// If the user specifically said to depend on this image, there
				// must be a build instruction for it.
				return fmt.Errorf("image %q: image dep %q not found",
					imageBuilder.configurationRef.RefFamiliarString(), container.FamiliarString(depImage))
			}
			imageBuilder.imageMapDeps = append(imageBuilder.imageMapDeps, depBuilder.ImageMapName())
		}

	}
	return nil
}

func (s *tiltfileState) assembleDC() error {
	if len(s.dc.services) > 0 && !s.defaultReg.Empty() {
		return errors.New("default_registry is not supported with docker compose")
	}

	for _, svc := range s.dc.services {
		builder := s.buildIndex.findBuilderForConsumedImage(svc.ImageRef())
		if builder != nil {
			// there's a Tilt-managed builder (e.g. docker_build or custom_build) for this image reference, so use that
			svc.ImageMapDeps = append(svc.ImageMapDeps, builder.ImageMapName())
		} else {
			// create a DockerComposeBuild image target and consume it if this service has a build section in YAML
			err := s.maybeAddDockerComposeImageBuilder(svc)
			if err != nil {
				return err
			}
		}
		// TODO(maia): throw warning if
		//  a. there is an img ref from config, and img ref from user doesn't match
		//  b. there is no img ref from config, and img ref from user is not of form .*_<svc_name>
	}
	return nil
}

func (s *tiltfileState) maybeAddDockerComposeImageBuilder(svc *dcService) error {
	build := svc.ServiceConfig.Build
	if build == nil || build.Context == "" {
		// this Docker Compose service has no build info - it relies purely on
		// a pre-existing image (e.g. from a registry)
		return nil
	}

	buildContext := build.Context
	if !filepath.IsAbs(buildContext) {
		// the Compose loader should always ensure that context paths are absolute upfront
		return fmt.Errorf("Docker Compose service %q has a relative build path: %q", svc.Name, buildContext)
	}

	dfPath := build.Dockerfile
	if dfPath == "" {
		// Per Compose spec, the default is "Dockerfile" (in the context dir)
		dfPath = "Dockerfile"
	}

	if !filepath.IsAbs(dfPath) {
		dfPath = filepath.Join(buildContext, dfPath)
	}

	imageRef := svc.ImageRef()
	err := s.buildIndex.addImage(
		&dockerImage{
			buildType:                     DockerComposeBuild,
			configurationRef:              container.NewRefSelector(imageRef),
			dockerComposeService:          svc.Name,
			dockerComposeLocalVolumePaths: svc.MountedLocalDirs,
			dbBuildPath:                   buildContext,
			dbDockerfilePath:              dfPath,
		})
	if err != nil {
		return err
	}
	b := s.buildIndex.findBuilderForConsumedImage(imageRef)
	svc.ImageMapDeps = append(svc.ImageMapDeps, b.ImageMapName())
	return nil
}

func (s *tiltfileState) assembleK8s() error {
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

	for _, opts := range s.k8sResourceOptions {
		if opts.manuallyGrouped {
			r, err := s.makeK8sResource(opts.newName)
			if err != nil {
				return err
			}
			r.manuallyGrouped = true
			s.k8sByName[opts.newName] = r
		}
		if r, ok := s.k8sByName[opts.workload]; ok {
			// Options are added, so aggregate options from previous resource calls.
			r.extraPodSelectors = append(r.extraPodSelectors, opts.extraPodSelectors...)
			if opts.podReadinessMode != model.PodReadinessNone {
				r.podReadinessMode = opts.podReadinessMode
			}
			if opts.discoveryStrategy != "" {
				r.discoveryStrategy = opts.discoveryStrategy
			}
			r.portForwards = append(r.portForwards, opts.portForwards...)
			if opts.triggerMode != TriggerModeUnset {
				r.triggerMode = opts.triggerMode
			}
			if opts.autoInit.IsSet {
				r.autoInit = opts.autoInit.Value
			}
			r.resourceDeps = append(r.resourceDeps, opts.resourceDeps...)
			r.links = append(r.links, opts.links...)
			for k, v := range opts.labels {
				r.labels[k] = v
			}
			if opts.newName != "" && opts.newName != r.name {
				err := s.checkResourceConflict(opts.newName)
				if err != nil {
					return fmt.Errorf("k8s_resource at %s specified to rename %q to %q: %v",
						opts.tiltfilePosition.String(), r.name, opts.newName, err)
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
			return fmt.Errorf("k8s_resource at %s specified unknown resource %q. known resources: %s",
				opts.tiltfilePosition.String(), opts.workload, strings.Join(knownResources, ", "))
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
	if len(r.entities) == 0 && r.customDeploy == nil {
		return fmt.Errorf("resource %q: could not associate any k8s_yaml() or k8s_custom_deploy() with this resource", r.name)
	}

	for _, ref := range r.imageRefs {
		builder := s.buildIndex.findBuilderForConsumedImage(ref)
		if builder != nil {
			r.imageMapDeps = append(r.imageMapDeps, builder.ImageMapName())
			continue
		}

		metadata, ok := r.imageDepsMetadata[ref.String()]
		if ok && metadata.required {
			return fmt.Errorf("resource %q: image build %q not found", r.name, container.FamiliarString(ref))
		}
	}

	return nil
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
// feels like we're still doing this very ad-hoc rather than holistically
func (s *tiltfileState) inferPodReadinessMode(r *k8sResource) model.PodReadinessMode {
	// The mode set directly on the resource has highest priority.
	if r.podReadinessMode != model.PodReadinessNone {
		return r.podReadinessMode
	}

	// Next, check if any of the k8s kinds have a mode.
	hasMode := make(map[model.PodReadinessMode]bool)
	for _, e := range r.entities {
		for sel, info := range s.k8sKinds {
			if sel.Matches(e) {
				hasMode[info.PodReadinessMode] = true
			}
		}
	}

	modes := []model.PodReadinessMode{model.PodReadinessWait, model.PodReadinessIgnore, model.PodReadinessSucceeded}
	for _, m := range modes {
		if hasMode[m] {
			return m
		}
	}

	// Auto-infer based on context
	//
	// If the resource was
	// 1) manually grouped (i.e., we didn't find any images in it)
	// 2) doesn't have pod selectors, and
	// 3) doesn't depend on images
	// assume that it will never create pods.
	if r.manuallyGrouped && len(r.extraPodSelectors) == 0 && len(r.imageMapDeps) == 0 {
		return model.PodReadinessIgnore
	}

	return model.PodReadinessWait
}

func (s *tiltfileState) translateK8s(resources []*k8sResource, updateSettings model.UpdateSettings) ([]model.Manifest, error) {
	var result []model.Manifest
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

		m = m.WithLabels(r.labels)

		iTargets, err := s.imgTargetsForDeps(mn, r.imageMapDeps)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", r.name)
		}

		for i, iTarget := range iTargets {
			if liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec) {
				continue
			}
			iTarget.LiveUpdateReconciler = true
			iTargets[i] = iTarget
		}

		m = m.WithImageTargets(iTargets)

		k8sTarget, err := s.k8sDeployTarget(mn.TargetName(), r, iTargets, updateSettings)
		if err != nil {
			return nil, errors.Wrapf(err, "creating K8s deploy target for %s", r.name)
		}

		m = m.WithDeployTarget(k8sTarget)
		result = append(result, m)
	}

	err := maybeRestartContainerDeprecationError(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *tiltfileState) k8sDeployTarget(targetName model.TargetName, r *k8sResource, imageTargets []model.ImageTarget, updateSettings model.UpdateSettings) (model.K8sTarget, error) {
	var kdTemplateSpec *v1alpha1.KubernetesDiscoveryTemplateSpec
	if len(r.extraPodSelectors) != 0 {
		kdTemplateSpec = &v1alpha1.KubernetesDiscoveryTemplateSpec{
			ExtraSelectors: k8s.SetsAsLabelSelectors(r.extraPodSelectors),
		}
	}

	sinceTime := apis.NewTime(pkgInitTime)
	applySpec := v1alpha1.KubernetesApplySpec{
		Timeout:                         metav1.Duration{Duration: updateSettings.K8sUpsertTimeout()},
		PortForwardTemplateSpec:         k8s.PortForwardTemplateSpec(s.defaultedPortForwards(r.portForwards)),
		DiscoveryStrategy:               r.discoveryStrategy,
		KubernetesDiscoveryTemplateSpec: kdTemplateSpec,
		PodLogStreamTemplateSpec: &v1alpha1.PodLogStreamTemplateSpec{
			SinceTime: &sinceTime,
			IgnoreContainers: []string{
				string(container.IstioInitContainerName),
				string(container.IstioSidecarContainerName),
			},
		},
	}

	var deps []string
	if r.customDeploy != nil {
		deps = r.customDeploy.deps
		applySpec.ApplyCmd = toKubernetesApplyCmd(r.customDeploy.applyCmd)
		applySpec.DeleteCmd = toKubernetesApplyCmd(r.customDeploy.deleteCmd)
		applySpec.RestartOn = &v1alpha1.RestartOnSpec{
			FileWatches: []string{apis.SanitizeName(fmt.Sprintf("%s:apply", targetName.String()))},
		}
	} else {
		entities := k8s.SortedEntities(r.entities)
		var err error
		applySpec.YAML, err = k8s.SerializeSpecYAML(entities)
		if err != nil {
			return model.K8sTarget{}, err
		}

		for _, locator := range s.k8sImageLocatorsList() {
			if k8s.LocatorMatchesOne(locator, entities) {
				applySpec.ImageLocators = append(applySpec.ImageLocators, locator.ToSpec())
			}
		}
	}

	t, err := k8s.NewTarget(targetName, applySpec, s.inferPodReadinessMode(r), r.links)
	if err != nil {
		return model.K8sTarget{}, err
	}

	t = t.WithImageDependencies(model.FilterLiveUpdateOnly(r.imageMapDeps, imageTargets)).
		WithRefInjectCounts(r.imageRefInjectCounts()).
		WithPathDependencies(deps, reposForPaths(deps))

	return t, nil
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
// TODO(nick): I think the "right" way to do this is to give the starkit plugin system
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
		if liveupdate.IsEmptySpec(iTarg.LiveUpdateSpec) {
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
	luSpec := iTarget.LiveUpdateSpec
	if liveupdate.IsEmptySpec(luSpec) {
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
	for _, sync := range liveupdate.SyncSteps(luSpec) {
		if !ospath.IsChildOfOne(watchedPaths, sync.LocalPath) {
			return fmt.Errorf("sync step source '%s' is not a child of any watched filepaths (%v)",
				sync.LocalPath, watchedPaths)
		}
	}

	pathSet := liveupdate.FallBackOnFiles(luSpec)
	for _, path := range pathSet.Paths {
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(pathSet.BaseDirectory, path)
		}
		if !ospath.IsChildOfOne(watchedPaths, resolved) {
			return fmt.Errorf("fall_back_on paths '%s' is not a child of any watched filepaths (%v)",
				resolved, watchedPaths)
		}
	}

	return nil
}

func (s *tiltfileState) validateDockerComposeVersion() error {
	const minimumDockerComposeVersion = "v1.28.3"

	dcVersion, _, err := s.dcCli.Version(s.ctx)
	if err != nil {
		logger.Get(s.ctx).Debugf("Failed to determine Docker Compose version: %v", err)
	} else if semver.Compare(dcVersion, minimumDockerComposeVersion) == -1 {
		return fmt.Errorf(
			"Tilt requires Docker Compose %s+ (you have %s). Please upgrade and re-launch Tilt.",
			minimumDockerComposeVersion,
			dcVersion)
	} else if semver.Major(dcVersion) == "v2" && semver.Compare(dcVersion, "v2.2") < 0 {
		logger.Get(s.ctx).Warnf("Using Docker Compose %s (version < 2.2) may result in errors or broken functionality.\n"+
			"For best results, we recommend upgrading to Docker Compose >= v2.2.0.", dcVersion)
	} else if semver.Prerelease(dcVersion) != "" {
		logger.Get(s.ctx).Warnf("You are running a pre-release version of Docker Compose (%s), which is unsupported.\n"+
			"You might encounter errors or broken functionality.", dcVersion)
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
	// 7/2/20: we've deprecated restart_container() in favor of the restart_process plugin.
	// If this is a k8s resource with a restart_container step, throw a deprecation error.
	// (restart_container is still allowed for Docker Compose resources)
	if !m.IsK8s() {
		return false
	}

	for _, iTarg := range m.ImageTargets {
		if liveupdate.ShouldRestart(iTarg.LiveUpdateSpec) {
			return true
		}
	}

	return false
}

// Grabs all image targets for the given references,
// as well as any of their transitive dependencies.
func (s *tiltfileState) imgTargetsForDeps(mn model.ManifestName, imageMapDeps []string) ([]model.ImageTarget, error) {
	claimStatus := make(map[string]claim, len(imageMapDeps))
	return s.imgTargetsForDepsHelper(mn, imageMapDeps, claimStatus)
}

func (s *tiltfileState) imgTargetsForDepsHelper(mn model.ManifestName, imageMapDeps []string, claimStatus map[string]claim) ([]model.ImageTarget, error) {
	iTargets := make([]model.ImageTarget, 0, len(imageMapDeps))
	for _, imName := range imageMapDeps {
		image := s.buildIndex.findBuilderByImageMapName(imName)
		if image == nil {
			return nil, fmt.Errorf("Internal error: no image builder found for id %s", imName)
		}

		claim := claimStatus[imName]
		if claim == claimFinished {
			// Skip this target, an earlier call has already built it
			continue
		} else if claim == claimPending {
			return nil, fmt.Errorf("Image dependency cycle: %s", image.configurationRef)
		}
		claimStatus[imName] = claimPending

		var overrideCommand *v1alpha1.ImageMapOverrideCommand
		if !image.entrypoint.Empty() {
			overrideCommand = &v1alpha1.ImageMapOverrideCommand{
				Command: image.entrypoint.Argv,
			}
		}

		iTarget := model.ImageTarget{
			ImageMapSpec: v1alpha1.ImageMapSpec{
				Selector:        image.configurationRef.RefFamiliarString(),
				MatchInEnvVars:  image.matchInEnvVars,
				MatchExact:      image.configurationRef.MatchExact(),
				OverrideCommand: overrideCommand,
				OverrideArgs:    image.overrideArgs,
			},
			LiveUpdateSpec: image.liveUpdate,
		}
		if !liveupdate.IsEmptySpec(image.liveUpdate) {
			iTarget.LiveUpdateName = liveupdate.GetName(mn, iTarget.ID())
		}

		switch image.Type() {
		case DockerBuild:
			iTarget.DockerImageName = dockerimage.GetName(mn, iTarget.ID())

			spec := v1alpha1.DockerImageSpec{
				DockerfileContents: image.dbDockerfile.String(),
				Context:            image.dbBuildPath,
				Args:               image.dbBuildArgs,
				Target:             image.targetStage,
				SSHAgentConfigs:    image.sshSpecs,
				Secrets:            image.secretSpecs,
				Network:            image.network,
				CacheFrom:          image.cacheFrom,
				Pull:               image.pullParent,
				Platform:           image.platform,
				ExtraTags:          image.extraTags,
			}
			iTarget = iTarget.WithBuildDetails(model.DockerBuild{DockerImageSpec: spec})
		case CustomBuild:
			iTarget.CmdImageName = cmdimage.GetName(mn, iTarget.ID())

			spec := v1alpha1.CmdImageSpec{
				Args:              image.customCommand.Argv,
				Dir:               image.workDir,
				OutputTag:         image.customTag,
				OutputsImageRefTo: image.outputsImageRefTo,
			}
			if image.skipsLocalDocker {
				spec.OutputMode = v1alpha1.CmdImageOutputRemote
			} else if image.disablePush {
				spec.OutputMode = v1alpha1.CmdImageOutputLocalDockerAndRemote
			} else {
				spec.OutputMode = v1alpha1.CmdImageOutputLocalDocker
			}
			r := model.CustomBuild{
				CmdImageSpec: spec,
				Deps:         image.customDeps,
			}
			iTarget = iTarget.WithBuildDetails(r)
		case DockerComposeBuild:
			bd := model.DockerComposeBuild{
				Service:          image.dockerComposeService,
				Context:          image.dbBuildPath,
				LocalVolumePaths: image.dockerComposeLocalVolumePaths,
			}
			iTarget = iTarget.WithBuildDetails(bd)
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
			WithTiltFilename(image.tiltfilePath).
			WithImageMapDeps(image.imageMapDeps)

		depTargets, err := s.imgTargetsForDepsHelper(mn, image.imageMapDeps, claimStatus)
		if err != nil {
			return nil, err
		}

		iTargets = append(iTargets, depTargets...)
		iTargets = append(iTargets, iTarget)

		claimStatus[imName] = claimFinished
	}
	return iTargets, nil
}

func (s *tiltfileState) translateDC(dc dcResourceSet) ([]model.Manifest, error) {
	var result []model.Manifest

	for _, svc := range dc.services {

		iTargets, err := s.imgTargetsForDeps(model.ManifestName(svc.Name), svc.ImageMapDeps)
		if err != nil {
			return nil, errors.Wrapf(err, "getting image build info for %s", svc.Name)
		}

		for _, iTarg := range iTargets {
			if iTarg.OverrideCommand != nil {
				return nil, fmt.Errorf("docker_build/custom_build.entrypoint not supported for Docker Compose resources")
			}
		}

		m, err := s.dcServiceToManifest(svc, dc, iTargets)
		if err != nil {
			return nil, err
		}

		result = append(result, m)
	}

	return result, nil
}

type claim int

const (
	claimNone claim = iota
	claimPending
	claimFinished
)

var _ claim = claimNone

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

		paths := append([]string{}, r.deps...)
		paths = append(paths, r.threadDir)

		var ignores []model.Dockerignore
		if len(r.ignores) != 0 {
			ignores = append(ignores, model.Dockerignore{
				Patterns:  r.ignores,
				Source:    fmt.Sprintf("local_resource(%q)", r.name),
				LocalPath: r.threadDir,
			})
		}

		lt := model.NewLocalTarget(model.TargetName(r.name), r.updateCmd, r.serveCmd, r.deps).
			WithRepos(reposForPaths(paths)).
			WithIgnores(ignores).
			WithAllowParallel(r.allowParallel || r.updateCmd.Empty()).
			WithLinks(r.links).
			WithReadinessProbe(r.readinessProbe)
		var mds []model.ManifestName
		for _, md := range r.resourceDeps {
			mds = append(mds, model.ManifestName(md))
		}
		m := model.Manifest{
			Name:                 mn,
			TriggerMode:          tm,
			ResourceDependencies: mds,
		}.WithDeployTarget(lt)

		m = m.WithLabels(r.labels)

		result = append(result, m)
	}

	return result, nil
}

func (s *tiltfileState) tempDir() (*fwatch.TempDir, error) {
	if s.scratchDir == nil {
		dir, err := fwatch.NewDir("tiltfile")
		if err != nil {
			return dir, err
		}
		s.scratchDir = dir
		go func() {
			<-s.ctx.Done()
			_ = s.scratchDir.TearDown()
		}()
	}
	return s.scratchDir, nil
}

func (s *tiltfileState) validateResourceDependencies(ms []model.Manifest) error {
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
				logger.Get(s.ctx).Warnf("resource %s specified a dependency on unknown resource %s - dependency ignored", m.Name, b)
				continue
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

func toKubernetesApplyCmd(cmd model.Cmd) *v1alpha1.KubernetesApplyCmd {
	if cmd.Empty() {
		return nil
	}
	return &v1alpha1.KubernetesApplyCmd{
		Args: cmd.Argv,
		Dir:  cmd.Dir,
		Env:  cmd.Env,
	}
}

var _ starkit.Plugin = &tiltfileState{}
var _ starkit.OnExecPlugin = &tiltfileState{}
var _ starkit.OnBuiltinCallPlugin = &tiltfileState{}

package tiltfile

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

// dcResourceSet represents a single docker-compose config file and all its associated services
type dcResourceSet struct {
	configPaths []string

	services     []*dcService
	tiltfilePath string
}

func (dc dcResourceSet) Empty() bool { return reflect.DeepEqual(dc, dcResourceSet{}) }

func (s *tiltfileState) dockerCompose(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var configPathsValue starlark.Value

	err := s.unpackArgs(fn.Name(), args, kwargs, "configPaths", &configPathsValue)
	if err != nil {
		return nil, err
	}

	pathSlice := starlarkValueOrSequenceToSlice(configPathsValue)
	var configPaths []string
	for _, v := range pathSlice {
		path, err := value.ValueToAbsPath(thread, v)
		if err != nil {
			return nil, fmt.Errorf("docker_compose files must be a string or a sequence of strings; found a %T", v)
		}
		configPaths = append(configPaths, path)

		err = io.RecordReadPath(thread, io.WatchFileOnly, path)
		if err != nil {
			return nil, err
		}
	}

	dc := s.dc
	currentTiltfilePath := starkit.CurrentExecPath(thread)
	if dc.tiltfilePath != "" && dc.tiltfilePath != currentTiltfilePath {
		return starlark.None, fmt.Errorf("Cannot load docker-compose files from two different Tiltfiles.\n"+
			"docker-compose must have a single working directory:\n"+
			"(%s, %s)", dc.tiltfilePath, currentTiltfilePath)
	}

	// To make sure all the docker-compose files are compatible together,
	// parse them all together.
	allConfigPaths := append([]string{}, dc.configPaths...)
	allConfigPaths = append(allConfigPaths, configPaths...)

	services, err := parseDCConfig(s.ctx, s.dcCli, allConfigPaths)
	if err != nil {
		return nil, err
	}

	for _, s := range services {
		dfPath := s.DfPath
		if dfPath == "" {
			continue
		}

		err = io.RecordReadPath(thread, io.WatchFileOnly, s.DfPath)
		if err != nil {
			return nil, err
		}
	}

	s.dc = dcResourceSet{
		configPaths:  allConfigPaths,
		services:     services,
		tiltfilePath: starkit.CurrentExecPath(thread),
	}

	return starlark.None, nil
}

// DCResource allows you to adjust specific settings on a DC resource that we assume
// to be defined in a `docker_compose.yml`
func (s *tiltfileState) dcResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var imageVal starlark.Value
	var triggerMode triggerMode
	var resourceDepsVal starlark.Sequence

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,

		// TODO(maia): if you docker_build('myimg') and dc.yml refers to 'myimg', we
		//  associate the docker_build with your dc resource automatically. What we
		//  CAN'T do is use the arg to dc_resource.image to OVERRIDE the image named
		//  in dc.yml, which we should probs be able to do?
		// (If your dc.yml does NOT specify `Image`, DC will expect an image of name
		// <directory>_<service>, and Tilt has no way of figuring this out yet, so
		// can't auto-associate that image, you need to use dc_resource.)
		"image?", &imageVal,

		"trigger_mode?", &triggerMode,
		"resource_deps?", &resourceDepsVal,
	); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("dc_resource: `name` must not be empty")
	}

	var imageRefAsStr *string
	switch imageVal := imageVal.(type) {
	case nil: // optional arg, this is fine
	case starlark.String:
		s := string(imageVal)
		imageRefAsStr = &s
	default:
		return nil, fmt.Errorf("image arg must be a string; got %T", imageVal)
	}

	svc, err := s.getDCService(name)
	if err != nil {
		return nil, err
	}

	svc.TriggerMode = triggerMode

	if imageRefAsStr != nil {
		normalized, err := container.ParseNamed(*imageRefAsStr)
		if err != nil {
			return nil, err
		}
		svc.imageRefFromUser = normalized
	}

	rds, err := value.SequenceToStringSlice(resourceDepsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: resource_deps", fn.Name())
	}
	svc.resourceDeps = rds

	return starlark.None, nil
}

func (s *tiltfileState) getDCService(name string) (*dcService, error) {
	allNames := make([]string, len(s.dc.services))
	for i, svc := range s.dc.services {
		if svc.Name == name {
			return svc, nil
		}
		allNames[i] = svc.Name
	}
	return nil, fmt.Errorf("no Docker Compose service found with name '%s'. "+
		"Found these instead:\n\t%s", name, strings.Join(allNames, "; "))
}

// A docker-compose service, according to Tilt.
type dcService struct {
	Name         string
	BuildContext string
	DfPath       string
	// these are the host machine paths that DC will sync from the local volume into the container
	// https://docs.docker.com/compose/compose-file/#volumes
	MountedLocalDirs []string

	// RefSelector of the image associated with this service
	// The user-provided image ref overrides the config-provided image ref
	imageRefFromConfig reference.Named // from docker-compose.yml `Image` field
	imageRefFromUser   reference.Named // set via dc_resource

	// Currently just use these to diff against when config files are edited to see if manifest has changed
	ServiceConfig []byte
	DfContents    []byte

	DependencyIDs  []model.TargetID
	PublishedPorts []int

	TriggerMode triggerMode

	resourceDeps []string
}

func (svc dcService) ImageRef() reference.Named {
	if svc.imageRefFromUser != nil {
		return svc.imageRefFromUser
	}
	return svc.imageRefFromConfig
}

func DockerComposeConfigToService(c dockercompose.Config, name string) (dcService, error) {
	svcConfig, ok := c.Services[name]
	if !ok {
		return dcService{}, fmt.Errorf("no service %s found in config", name)
	}

	buildContext := svcConfig.Build.Context
	dfPath := svcConfig.Build.Dockerfile
	if buildContext != "" {
		if dfPath == "" {
			// We only expect a Dockerfile if there's a build context specified.
			dfPath = "Dockerfile"
		}
		dfPath = filepath.Join(buildContext, dfPath)
	}

	var mountedLocalDirs []string
	for _, v := range svcConfig.Volumes {
		mountedLocalDirs = append(mountedLocalDirs, v.Source)
	}

	var publishedPorts []int
	for _, portSpec := range svcConfig.Ports {
		if portSpec.Published != 0 {
			publishedPorts = append(publishedPorts, portSpec.Published)
		}
	}

	svc := dcService{
		Name:             name,
		BuildContext:     buildContext,
		DfPath:           dfPath,
		MountedLocalDirs: mountedLocalDirs,

		ServiceConfig:  svcConfig.RawYAML,
		PublishedPorts: publishedPorts,
	}

	if svcConfig.Image != "" {
		ref, err := container.ParseNamed(svcConfig.Image)
		if err != nil {
			// TODO(nick): This doesn't seem like the right place to report this
			// error, but we don't really have a better way right now.
			return dcService{}, fmt.Errorf("Error parsing image name %q: %v", ref, err)
		} else {
			svc.imageRefFromConfig = ref
		}
	}

	if dfPath != "" {
		dfContents, err := ioutil.ReadFile(dfPath)
		if err != nil {
			return svc, err
		}
		svc.DfContents = dfContents
	}
	return svc, nil
}

func parseDCConfig(ctx context.Context, dcc dockercompose.DockerComposeClient, configPaths []string) ([]*dcService, error) {

	config, svcNames, err := dockercompose.ReadConfigAndServiceNames(ctx, dcc, configPaths)
	if err != nil {
		return nil, err
	}

	var services []*dcService
	for _, name := range svcNames {
		svc, err := DockerComposeConfigToService(config, name)
		if err != nil {
			return nil, errors.Wrapf(err, "getting service %s", name)
		}
		services = append(services, &svc)
	}

	return services, nil
}

func (s *tiltfileState) dcServiceToManifest(service *dcService, dcSet dcResourceSet) (model.Manifest, error) {
	dcInfo := model.DockerComposeTarget{
		ConfigPaths: dcSet.configPaths,
		YAMLRaw:     service.ServiceConfig,
		DfRaw:       service.DfContents,
	}.WithDependencyIDs(service.DependencyIDs).
		WithPublishedPorts(service.PublishedPorts).
		WithIgnoredLocalDirectories(service.MountedLocalDirs)

	um, err := starlarkTriggerModeToModel(s.triggerModeForResource(service.TriggerMode), true)
	if err != nil {
		return model.Manifest{}, err
	}

	var mds []model.ManifestName
	for _, md := range service.resourceDeps {
		mds = append(mds, model.ManifestName(md))
	}

	m := model.Manifest{
		Name:                 model.ManifestName(service.Name),
		TriggerMode:          um,
		ResourceDependencies: mds,
	}.WithDeployTarget(dcInfo)

	if service.DfPath == "" {
		// DC service may not have Dockerfile -- e.g. may be just an image that we pull and run.
		return m, nil
	}

	dcInfo = dcInfo.WithBuildPath(service.BuildContext)

	paths := []string{filepath.Dir(service.DfPath)}
	for _, configPath := range dcSet.configPaths {
		paths = append(paths, filepath.Dir(configPath))
	}
	paths = append(paths, dcInfo.LocalPaths()...)
	paths = append(paths, filepath.Dir(dcSet.tiltfilePath))

	dIgnores, err := s.dockerignoresFromPathsAndContextFilters(
		fmt.Sprintf("docker-compose %s", service.Name),
		paths, []string{}, []string{}, service.DfPath)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("Reading dockerignore for %s: %v", service.Name, err)
	}

	dcInfo = dcInfo.WithDockerignores(dIgnores)

	localPaths := []string{dcSet.tiltfilePath}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return model.Manifest{}, fmt.Errorf("internal error: path not resolved correctly! Please report to https://github.com/tilt-dev/tilt/issues : %s", p)
		}
		localPaths = append(localPaths, p)
	}
	dcInfo = dcInfo.WithRepos(reposForPaths(localPaths)).
		WithTiltFilename(dcSet.tiltfilePath)

	m = m.WithDeployTarget(dcInfo)

	return m, nil
}

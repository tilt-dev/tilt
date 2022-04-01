package tiltfile

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/types"

	// DANGER: some compose-go types are not friendly to being marshaled with gopkg.in/yaml.v3
	// and will trigger a stack overflow panic
	// see https://github.com/tilt-dev/tilt/issues/4797
	composeyaml "gopkg.in/yaml.v2"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/links"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// dcResourceSet represents a single docker-compose config file and all its associated services
type dcResourceSet struct {
	Project v1alpha1.DockerComposeProject

	configPaths  []string
	services     []*dcService
	tiltfilePath string
}

func (dc dcResourceSet) Empty() bool { return reflect.DeepEqual(dc, dcResourceSet{}) }

func (s *tiltfileState) dockerCompose(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var configPaths starlark.Value
	envFile := value.NewLocalPathUnpacker(thread)

	err := s.unpackArgs(fn.Name(), args, kwargs, "configPaths", &configPaths, "env_file?", &envFile)
	if err != nil {
		return nil, err
	}

	paths := starlarkValueOrSequenceToSlice(configPaths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("Nothing to compose")
	}

	dc := s.dc

	currentTiltfilePath := starkit.CurrentExecPath(thread)
	if dc.tiltfilePath != "" && dc.tiltfilePath != currentTiltfilePath {
		return starlark.None, fmt.Errorf("Cannot load docker-compose files from two different Tiltfiles.\n"+
			"docker-compose must have a single working directory:\n"+
			"(%s, %s)", dc.tiltfilePath, currentTiltfilePath)
	}

	project := v1alpha1.DockerComposeProject{
		ConfigPaths: dc.configPaths,
		ProjectPath: dc.Project.ProjectPath,
		Name:        model.NormalizeName(filepath.Base(filepath.Dir(currentTiltfilePath))),
		EnvFile:     envFile.Value,
	}

	if project.EnvFile != "" {
		err = io.RecordReadPath(thread, io.WatchFileOnly, project.EnvFile)
		if err != nil {
			return nil, err
		}
	}

	for _, val := range paths {
		switch v := val.(type) {
		case nil:
			continue
		case io.Blob:
			yaml := v.String()
			message := "unable to store yaml blob"
			tmpdir, err := s.tempDir()
			if err != nil {
				return nil, errors.Wrap(err, message)
			}
			tmpfile, err := os.Create(filepath.Join(tmpdir.Path(), fmt.Sprintf("%x.yml", sha256.Sum256([]byte(yaml)))))
			if err != nil {
				return nil, errors.Wrap(err, message)
			}
			_, err = tmpfile.WriteString(yaml)
			if err != nil {
				tmpfile.Close()
				return nil, errors.Wrap(err, message)
			}
			err = tmpfile.Close()
			if err != nil {
				return nil, errors.Wrap(err, message)
			}
			project.ConfigPaths = append(project.ConfigPaths, tmpfile.Name())
		default:
			path, err := value.ValueToAbsPath(thread, val)
			if err != nil {
				return starlark.None, fmt.Errorf("expected blob | path (string). Actual type: %T", val)
			}

			// Set project path to dir of first compose file, like DC CLI does
			if project.ProjectPath == "" {
				project.ProjectPath = filepath.Dir(path)
			}

			project.ConfigPaths = append(project.ConfigPaths, path)
			err = io.RecordReadPath(thread, io.WatchFileOnly, path)
			if err != nil {
				return nil, err
			}
		}
	}

	// Set to tiltfile directory for YAML blob tempfiles
	if project.ProjectPath == "" {
		project.ProjectPath = filepath.Dir(currentTiltfilePath)
	}

	services, err := parseDCConfig(s.ctx, s.dcCli, project)
	if err != nil {
		return nil, err
	}

	for _, svc := range services {
		previousSvc := s.dcByName[svc.Name]
		if previousSvc != nil {
			delete(s.dcByName, svc.Name)
		}
		err := s.checkResourceConflict(svc.Name)
		if err != nil {
			return nil, err
		}
		svc.Options = s.dcResOptions[svc.Name]
		s.dcByName[svc.Name] = svc
	}

	s.dc = dcResourceSet{
		Project:      project,
		configPaths:  project.ConfigPaths,
		services:     services,
		tiltfilePath: currentTiltfilePath,
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
	var links links.LinkList
	var labels value.LabelSet
	var autoInit = value.Optional[starlark.Bool]{Value: true}

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		// TODO(milas): this argument is undocumented and arguably unnecessary
		// 	now that Tilt correctly infers the Docker Compose image ref format
		"image?", &imageVal,
		"trigger_mode?", &triggerMode,
		"resource_deps?", &resourceDepsVal,
		"links?", &links,
		"labels?", &labels,
		"auto_init?", &autoInit,
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

	options := s.dcResOptions[name]
	if options == nil {
		options = newDcResourceOptions()
	}

	if triggerMode != TriggerModeUnset {
		options.TriggerMode = triggerMode
	}

	options.Links = append(options.Links, links.Links...)

	for key, val := range labels.Values {
		options.Labels[key] = val
	}

	if imageRefAsStr != nil {
		normalized, err := container.ParseNamed(*imageRefAsStr)
		if err != nil {
			return nil, err
		}
		options.imageRefFromUser = normalized
	}

	rds, err := value.SequenceToStringSlice(resourceDepsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: resource_deps", fn.Name())
	}
	options.resourceDeps = append(options.resourceDeps, rds...)

	if autoInit.IsSet {
		options.AutoInit = autoInit
	}

	s.dcResOptions[name] = options
	svc.Options = options
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
	Name string

	// these are the host machine paths that DC will sync from the local volume into the container
	// https://docs.docker.com/compose/compose-file/#volumes
	MountedLocalDirs []string

	// RefSelector of the image associated with this service
	// The user-provided image ref overrides the config-provided image ref
	imageRefFromConfig reference.Named // from docker-compose.yml `Image` field

	ServiceConfig types.ServiceConfig

	// Currently just use this to diff against when config files are edited to see if manifest has changed
	ServiceYAML []byte

	ImageMapDeps   []string
	PublishedPorts []int

	Options *dcResourceOptions
}

// Options set via dc_resource
type dcResourceOptions struct {
	imageRefFromUser reference.Named
	TriggerMode      triggerMode
	Links            []model.Link
	AutoInit         value.Optional[starlark.Bool]

	Labels map[string]string

	resourceDeps []string
}

func newDcResourceOptions() *dcResourceOptions {
	return &dcResourceOptions{
		Labels: make(map[string]string),
	}
}

func (svc dcService) ImageRef() reference.Named {
	if svc.Options != nil && svc.Options.imageRefFromUser != nil {
		return svc.Options.imageRefFromUser
	}
	return svc.imageRefFromConfig
}

func dockerComposeConfigToService(projectName string, svcConfig types.ServiceConfig) (dcService, error) {
	var mountedLocalDirs []string
	for _, v := range svcConfig.Volumes {
		mountedLocalDirs = append(mountedLocalDirs, v.Source)
	}

	var publishedPorts []int
	for _, portSpec := range svcConfig.Ports {
		// a published port can be a string range of ports (e.g. "80-90")
		// this case is unusual and unsupported/ignored by Tilt for now
		publishedPort, err := strconv.Atoi(portSpec.Published)
		if err == nil && publishedPort != 0 {
			publishedPorts = append(publishedPorts, publishedPort)
		}
	}

	rawConfig, err := composeyaml.Marshal(svcConfig)
	if err != nil {
		return dcService{}, err
	}

	imageName := svcConfig.Image
	if imageName == "" {
		// see https://github.com/docker/compose/blob/7b84f2c2a538a1241dcf65f4b2828232189ef0ad/pkg/compose/create.go#L221-L227
		imageName = fmt.Sprintf("%s_%s", projectName, svcConfig.Name)
	}

	imageRef, err := container.ParseNamed(imageName)
	if err != nil {
		// TODO(nick): This doesn't seem like the right place to report this
		// error, but we don't really have a better way right now.
		return dcService{}, fmt.Errorf("Error parsing image name %q: %v", imageName, err)
	}

	svc := dcService{
		Name:               svcConfig.Name,
		ServiceConfig:      svcConfig,
		MountedLocalDirs:   mountedLocalDirs,
		ServiceYAML:        rawConfig,
		PublishedPorts:     publishedPorts,
		imageRefFromConfig: imageRef,
	}

	return svc, nil
}

func parseDCConfig(ctx context.Context, dcc dockercompose.DockerComposeClient, spec v1alpha1.DockerComposeProject) ([]*dcService, error) {
	proj, err := dcc.Project(ctx, spec)
	if err != nil {
		return nil, err
	}

	var services []*dcService
	err = proj.WithServices(proj.ServiceNames(), func(svcConfig types.ServiceConfig) error {
		svc, err := dockerComposeConfigToService(proj.Name, svcConfig)
		if err != nil {
			return errors.Wrapf(err, "getting service %s", svcConfig.Name)
		}
		services = append(services, &svc)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return services, nil
}

func (s *tiltfileState) dcServiceToManifest(service *dcService, dcSet dcResourceSet, iTargets []model.ImageTarget) (model.Manifest, error) {
	options := service.Options
	if options == nil {
		options = newDcResourceOptions()
	}

	dcInfo := model.DockerComposeTarget{
		Name: model.TargetName(service.Name),
		Spec: v1alpha1.DockerComposeServiceSpec{
			Service: service.Name,
			Project: dcSet.Project,
		},
		ServiceYAML:      string(service.ServiceYAML),
		Links:            options.Links,
		LocalVolumePaths: service.MountedLocalDirs,
	}.WithImageMapDeps(model.FilterLiveUpdateOnly(service.ImageMapDeps, iTargets)).
		WithPublishedPorts(service.PublishedPorts)

	autoInit := true
	if options.AutoInit.IsSet {
		autoInit = bool(options.AutoInit.Value)
	}
	um, err := starlarkTriggerModeToModel(s.triggerModeForResource(options.TriggerMode), autoInit)
	if err != nil {
		return model.Manifest{}, err
	}

	var mds []model.ManifestName
	for _, md := range options.resourceDeps {
		mds = append(mds, model.ManifestName(md))
	}

	for i, iTarget := range iTargets {
		if liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec) {
			continue
		}
		iTarget.LiveUpdateReconciler = true
		iTargets[i] = iTarget
	}

	m := model.Manifest{
		Name:                 model.ManifestName(service.Name),
		TriggerMode:          um,
		ResourceDependencies: mds,
	}.WithDeployTarget(dcInfo).
		WithLabels(options.Labels).
		WithImageTargets(iTargets)

	return m, nil
}

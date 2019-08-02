package tiltfile

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/model"
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

	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "configPaths", &configPathsValue)
	if err != nil {
		return nil, err
	}

	pathSlice := starlarkValueOrSequenceToSlice(configPathsValue)
	var configPaths []string
	for _, v := range pathSlice {
		path, err := s.absPathFromStarlarkValue(thread, v)
		if err != nil {
			return nil, fmt.Errorf("docker_compose files must be a string or a sequence of strings; found a %T", v)
		}
		configPaths = append(configPaths, path)
	}

	var services []*dcService
	tempServices, err := parseDCConfig(s.ctx, s.dcCli, configPaths)
	services = append(services, tempServices...)
	if err != nil {
		return nil, err
	}
	if !s.dc.Empty() {
		return starlark.None, fmt.Errorf("already have a docker-compose resource declared (%v), cannot declare another", s.dc.configPaths)
	}

	s.dc = dcResourceSet{
		configPaths:  configPaths,
		services:     services,
		tiltfilePath: s.currentTiltfilePath(thread),
	}

	return starlark.None, nil
}

// DCResource allows you to adjust specific settings on a DC resource that we assume
// to be defined in a `docker_compose.yml`
func (s *tiltfileState) dcResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var imageVal starlark.Value
	var triggerMode triggerMode

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"image", &imageVal, // in future this will be optional
		"trigger_mode?", &triggerMode,
	); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("dc_resource: `name` must not be empty")
	}

	var imageRefAsStr string
	switch imageVal := imageVal.(type) {
	case nil:
		return nil, fmt.Errorf("must specify an image arg (string or fast_build)")
	case starlark.String:
		imageRefAsStr = string(imageVal)
	case *fastBuild:
		imageRefAsStr = imageVal.img.configurationRef.String()
	default:
		return nil, fmt.Errorf("image arg must be a string or fast_build; got %T", imageVal)
	}

	svc, err := s.getDCService(name)
	if err != nil {
		return nil, err
	}

	svc.TriggerMode = triggerMode

	normalized, err := container.ParseNamed(imageRefAsStr)
	if err != nil {
		return nil, err
	}
	svc.ImageRef = normalized

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

// Go representations of docker-compose.yml
// (Add fields as we need to support more things)
type DcConfig struct {
	Services map[string]dcServiceConfig
}

type dcServiceConfig struct {
	RawYAML []byte        // We store this to diff against when docker-compose.yml is edited to see if the manifest has changed
	Build   dcBuildConfig `yaml:"build"`
	Image   string        `yaml:"image"`
	Volumes Volumes       `yaml:"volumes"`
	Ports   Ports         `yaml:"ports"`
}

type Volumes []Volume

type Volume struct {
	Source string
}

func (v *Volumes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var sliceType []interface{}
	err := unmarshal(&sliceType)
	if err != nil {
		return errors.Wrap(err, "unmarshalling volumes")
	}

	for _, volume := range sliceType {
		// Volumes syntax documented here: https://docs.docker.com/compose/compose-file/#volumes
		// This implementation far from comprehensive. It will silently ignore:
		// 1. "short" syntax using volume keys instead of paths
		// 2. all "long" syntax volumes
		// Ideally, we'd let the user know we didn't handle their case, but getting a ctx here is not easy
		switch a := volume.(type) {
		case string:
			parts := strings.Split(a, ":")
			*v = append(*v, Volume{Source: parts[0]})
		}
	}

	return nil
}

type Ports []Port
type Port struct {
	Published int `yaml:"published"`
}

func (p *Ports) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var sliceType []interface{}
	err := unmarshal(&sliceType)
	if err != nil {
		return errors.Wrap(err, "unmarshalling ports")
	}

	for _, portSpec := range sliceType {
		// Port syntax documented here:
		// https://docs.docker.com/compose/compose-file/#ports
		// ports aren't critical, so on any error we want to continue quietly.
		//
		// Fortunately, `docker-compose config` does a lot of normalization for us,
		// like resolving port ranges and ensuring the protocol (tcp vs udp)
		// is always included.
		switch portSpec := portSpec.(type) {
		case string:
			withoutProtocol := strings.Split(portSpec, "/")[0]
			parts := strings.Split(withoutProtocol, ":")
			publishedPart := parts[0]
			if len(parts) == 3 {
				// For "127.0.0.1:3000:3000"
				publishedPart = parts[1]
			}
			port, err := strconv.Atoi(publishedPart)
			if err != nil {
				continue
			}
			*p = append(*p, Port{Published: port})
		case map[interface{}]interface{}:
			var portStruct Port
			b, err := yaml.Marshal(portSpec) // so we can unmarshal it again
			if err != nil {
				continue
			}

			err = yaml.Unmarshal(b, &portStruct)
			if err != nil {
				continue
			}
			*p = append(*p, portStruct)
		}
	}

	return nil
}

// We use a custom Unmarshal method here so that we can store the RawYAML in addition
// to unmarshaling the fields we care about into structs.
func (c *DcConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Services map[string]interface{} `yaml:"services"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	if c.Services == nil {
		c.Services = make(map[string]dcServiceConfig)
	}

	for k, v := range aux.Services {
		b, err := yaml.Marshal(v) // so we can unmarshal it again
		if err != nil {
			return err
		}

		svcConf := &dcServiceConfig{}
		err = yaml.Unmarshal(b, svcConf)
		if err != nil {
			return err
		}

		svcConf.RawYAML = b
		c.Services[k] = *svcConf
	}
	return nil
}

type dcBuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// A docker-compose service, according to Tilt.
type dcService struct {
	Name         string
	BuildContext string
	DfPath       string
	// these are the host machine paths that DC will sync from the local volume into the container
	// https://docs.docker.com/compose/compose-file/#volumes
	MountedLocalDirs []string

	// RefSelector of an image described via docker_build || fast_build call.
	// Can be explicitly linked to this service via dc_service call,
	// or implicitly via an image name in the docker-compose.yml
	ImageRef reference.Named

	// Currently just use these to diff against when config files are edited to see if manifest has changed
	ServiceConfig []byte
	DfContents    []byte

	DependencyIDs  []model.TargetID
	PublishedPorts []int

	TriggerMode triggerMode
}

func (c DcConfig) GetService(name string) (dcService, error) {
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
			svc.ImageRef = ref
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

func serviceNames(ctx context.Context, dcc dockercompose.DockerComposeClient, configPaths []string) ([]string, error) {
	servicesText, err := dcc.Services(ctx, configPaths)
	if err != nil {
		return nil, err
	}

	serviceNames := strings.Split(servicesText, "\n")

	var result []string

	for _, name := range serviceNames {
		if name == "" {
			continue
		}
		result = append(result, name)
	}

	return result, nil
}

func parseDCConfig(ctx context.Context, dcc dockercompose.DockerComposeClient, configPaths []string) ([]*dcService, error) {

	config, svcNames, err := getConfigAndServiceNames(ctx, dcc, configPaths)
	if err != nil {
		return nil, err
	}

	var services []*dcService
	for _, name := range svcNames {
		svc, err := config.GetService(name)
		if err != nil {
			return nil, errors.Wrapf(err, "getting service %s", name)
		}
		services = append(services, &svc)
	}

	return services, nil
}

func getConfigAndServiceNames(ctx context.Context, dcc dockercompose.DockerComposeClient,
	configPaths []string) (conf DcConfig, svcNames []string, err error) {
	// calls to `docker-compose config` take a bit, and we need two,
	// so do them in parallel to make things faster
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {

		configOut, err := dcc.Config(ctx, configPaths)

		if err != nil {
			return err
		}

		err = yaml.Unmarshal([]byte(configOut), &conf)
		if err != nil {
			return err
		}
		return nil
	})

	g.Go(func() error {
		var err error
		svcNames, err = serviceNames(ctx, dcc, configPaths)
		if err != nil {
			return err
		}
		return nil
	})

	err = g.Wait()
	return conf, svcNames, err
}

func (s *tiltfileState) dcServiceToManifest(service *dcService, dcSet dcResourceSet) (manifest model.Manifest,
	configFiles []string, err error) {
	dcInfo := model.DockerComposeTarget{
		ConfigPaths: dcSet.configPaths,
		YAMLRaw:     service.ServiceConfig,
		DfRaw:       service.DfContents,
	}.WithDependencyIDs(service.DependencyIDs).
		WithPublishedPorts(service.PublishedPorts).
		WithIgnoredLocalDirectories(service.MountedLocalDirs)

	um, err := starlarkTriggerModeToModel(s.triggerModeForResource(service.TriggerMode))
	if err != nil {
		return model.Manifest{}, nil, err
	}
	m := model.Manifest{
		Name:        model.ManifestName(service.Name),
		TriggerMode: um,
	}.WithDeployTarget(dcInfo)

	if service.DfPath == "" {
		// DC service may not have Dockerfile -- e.g. may be just an image that we pull and run.
		return m, nil, nil
	}

	dcInfo = dcInfo.WithBuildPath(service.BuildContext)

	paths := []string{filepath.Dir(service.DfPath)}
	for _, configPath := range dcSet.configPaths {
		paths = append(paths, filepath.Dir(configPath))
	}
	paths = append(paths, dcInfo.LocalPaths()...)
	paths = append(paths, filepath.Dir(dcSet.tiltfilePath))

	dcInfo = dcInfo.WithDockerignores(s.dockerignoresFromPathsAndContextFilters(paths, []string{}, []string{}))

	localPaths := []string{dcSet.tiltfilePath}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return model.Manifest{}, nil, fmt.Errorf("internal error: path not resolved correctly! Please report to https://github.com/windmilleng/tilt/issues : %s", p)
		}
		localPaths = append(localPaths, p)
	}
	dcInfo = dcInfo.WithRepos(reposForPaths(localPaths)).
		WithTiltFilename(dcSet.tiltfilePath)

	m = m.WithDeployTarget(dcInfo)

	return m, []string{service.DfPath}, nil
}

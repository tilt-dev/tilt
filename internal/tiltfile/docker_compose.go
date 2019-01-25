package tiltfile

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"
	"go.starlark.net/starlark"
	"gopkg.in/yaml.v2"
)

// dcResourceSet represents a single docker-compose config file and all its associated services
type dcResourceSet struct {
	configPath string

	services []*dcService
}

func (dc dcResourceSet) Empty() bool { return reflect.DeepEqual(dc, dcResourceSet{}) }

func (dc dcResourceSet) imagesUsed() map[string]bool {
	imgs := make(map[string]bool)
	for _, svc := range dc.services {
		if svc.ImageRef != "" {
			imgs[svc.ImageRef] = true
		}
	}
	return imgs
}

func (s *tiltfileState) dockerCompose(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var configPath string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "configPath", &configPath)
	if err != nil {
		return nil, err
	}
	configPath = s.absPath(configPath)
	if err != nil {
		return nil, err
	}

	services, err := parseDCConfig(s.ctx, s.dcCli, configPath)
	if err != nil {
		return nil, err
	}

	if !s.dc.Empty() {
		return starlark.None, fmt.Errorf("already have a docker-compose resource declared (%s), cannot declare another (%s)", s.dc.configPath, configPath)
	}

	s.dc = dcResourceSet{configPath: configPath, services: services}

	return starlark.None, nil
}

// DCResource allows you to adjust specific settings on a DC resource that we assume
// to be defined in a `docker_compose.yml`
func (s *tiltfileState) dcResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var imageVal starlark.Value

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"image", &imageVal, // in future this will be optional
	); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("dc_resource: `name` must not be empty")
	}

	var imageRef string
	switch imageVal := imageVal.(type) {
	case nil:
		return nil, fmt.Errorf("must specify an image arg (string or fast_build)")
	case starlark.String:
		imageRef = string(imageVal)
	case *fastBuild:
		imageRef = imageVal.img.ref.Name()
	default:
		return nil, fmt.Errorf("image arg must be a string or fast_build; got %T", imageVal)
	}

	svc, err := s.getDCService(name)
	if err != nil {
		return nil, err
	}
	svc.ImageRef = imageRef

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
type dcConfig struct {
	Services []dcServiceConfig
}

type dcServiceConfig struct {
	Name    string
	RawYAML []byte        // We store this to diff against when docker-compose.yml is edited to see if the manifest has changed
	Build   dcBuildConfig `yaml:"build"`
	Volumes Volumes
}

type Volumes struct {
	Volumes []Volume
}

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
			v.Volumes = append(v.Volumes, Volume{Source: parts[0]})
		}
	}

	return nil
}

// We use a custom Unmarshal method here so that we can store the RawYAML in addition
// to unmarshaling the fields we care about into structs.
func (c *dcConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Services yaml.MapSlice `yaml:"services"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	if c.Services == nil {
		c.Services = make([]dcServiceConfig, len(aux.Services))
	}

	for i, svc := range aux.Services {
		b, err := yaml.Marshal(svc.Value) // so we can unmarshal it again
		if err != nil {
			return err
		}

		svcConf := &dcServiceConfig{}
		err = yaml.Unmarshal(b, svcConf)
		if err != nil {
			return err
		}

		svcConf.RawYAML = b
		keyStr, ok := svc.Key.(string)
		if !ok {
			return fmt.Errorf("can't convert `services` key to string: %v", svc.Key)
		}
		svcConf.Name = keyStr

		c.Services[i] = *svcConf
	}
	return nil
}

type dcBuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// A docker-compose service, according to Tilt.
type dcService struct {
	Name    string
	Context string
	DfPath  string
	// these are the host machine paths that DC will mount from the local volume into the container
	// https://docs.docker.com/compose/compose-file/#volumes
	MountedLocalDirs []string

	// Ref of an image described via docker_build || fast_build call
	// (explicitly linked to this service via dc_service call)
	ImageRef string

	// Currently just use these to diff against when config files are edited to see if manifest has changed
	ServiceConfig []byte
	DfContents    []byte
}

func (svcConf dcServiceConfig) toService() (dcService, error) {
	df := svcConf.Build.Dockerfile
	if df == "" && svcConf.Build.Context != "" {
		// We only expect a Dockerfile if there's a build context specified.
		df = "Dockerfile"
	}

	var mountedLocalDirs []string
	for _, v := range svcConf.Volumes.Volumes {
		mountedLocalDirs = append(mountedLocalDirs, v.Source)
	}

	dfPath := filepath.Join(svcConf.Build.Context, df)
	svc := dcService{
		Name:             svcConf.Name,
		Context:          svcConf.Build.Context,
		DfPath:           dfPath,
		MountedLocalDirs: mountedLocalDirs,

		ServiceConfig: svcConf.RawYAML,
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

func parseDCConfig(ctx context.Context, dcCli dockercompose.DockerComposeClient, configPath string) ([]*dcService, error) {
	configOut, err := dcCli.Config(ctx, configPath)
	if err != nil {
		return nil, err
	}

	config := dcConfig{}
	err = yaml.Unmarshal([]byte(configOut), &config)
	if err != nil {
		return nil, err
	}

	var services []*dcService

	for _, svcConfig := range config.Services {
		svc, err := svcConfig.toService()
		if err != nil {
			return nil, errors.Wrapf(err, "getting service %s", svcConfig.Name)
		}
		services = append(services, &svc)
	}

	return services, nil
}

func (s *tiltfileState) dcServiceToManifest(service *dcService, dcConfigPath string) (manifest model.Manifest,
	configFiles []string, err error) {
	dcInfo := model.DockerComposeTarget{
		ConfigPath: dcConfigPath,
		YAMLRaw:    service.ServiceConfig,
		DfRaw:      service.DfContents,
	}

	m := model.Manifest{
		Name: model.ManifestName(service.Name),
	}.WithDeployTarget(dcInfo)

	if service.DfPath == "" {
		// DC service may not have Dockerfile -- e.g. may be just an image that we pull and run.
		// So, don't parse a non-existent Dockerfile for mount info.
		return m, nil, nil
	}

	df := dockerfile.Dockerfile(service.DfContents)
	mounts, err := df.DeriveMounts(service.Context)
	if err != nil {
		return model.Manifest{}, nil, err
	}

	dcInfo.Mounts = mounts

	paths := []string{path.Dir(service.DfPath), path.Dir(dcConfigPath)}
	for _, mount := range mounts {
		paths = append(paths, mount.LocalPath)
	}

	dcInfo = dcInfo.WithDockerignores(dockerignoresForPaths(append(paths, path.Dir(s.filename.path))))

	localPaths := []localPath{s.filename}
	for _, p := range paths {
		localPaths = append(localPaths, s.localPathFromString(p))
	}
	dcInfo = dcInfo.WithRepos(reposForPaths(localPaths)).
		WithTiltFilename(s.filename.path).
		WithIgnoredLocalDirectories(service.MountedLocalDirs)

	m = m.WithDeployTarget(dcInfo)

	return m, []string{service.DfPath}, nil
}

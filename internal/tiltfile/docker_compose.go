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

// dcResource represents a single docker-compose config file and all its associated services
type dcResource struct {
	configPath string

	services []dcService
}

func (dc dcResource) Empty() bool { return reflect.DeepEqual(dc, dcResource{}) }

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

	services, err := parseDCConfig(s.ctx, configPath)
	if err != nil {
		return nil, err
	}

	if !s.dc.Empty() {
		return starlark.None, fmt.Errorf("already have a docker-compose resource declared (%s), cannot declare another (%s)", s.dc.configPath, configPath)
	}

	s.dc = dcResource{configPath: configPath, services: services}

	return starlark.None, nil
}

// Go representations of docker-compose.yml
// (Add fields as we need to support more things)
type dcConfig struct {
	Services map[string]dcServiceConfig
}

type dcServiceConfig struct {
	RawYAML []byte        // We store this to diff against when docker-compose.yml is edited to see if the manifest has changed
	Build   dcBuildConfig `yaml:"build"`
}

// We use a custom Unmarshal method here so that we can store the RawYAML in addition
// to unmarshaling the fields we care about into structs.
func (c *dcConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
	Name    string
	Context string
	DfPath  string

	// Currently just use these to diff against when config files are edited to see if manifest has changed
	ServiceConfig []byte
	DfContents    []byte
}

func (c dcConfig) GetService(name string) (dcService, error) {
	svcConfig, ok := c.Services[name]
	if !ok {
		return dcService{}, fmt.Errorf("no service %s found in config", name)
	}

	df := svcConfig.Build.Dockerfile
	if df == "" && svcConfig.Build.Context != "" {
		// We only expect a Dockerfile if there's a build context specified.
		df = "Dockerfile"
	}

	dfPath := filepath.Join(svcConfig.Build.Context, df)
	svc := dcService{
		Name:          name,
		Context:       svcConfig.Build.Context,
		DfPath:        dfPath,
		ServiceConfig: svcConfig.RawYAML,
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

func svcNames(ctx context.Context, dcc dockercompose.DockerComposeClient, configPath string) ([]string, error) {
	servicesText, err := dcc.Services(ctx, configPath)
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

func parseDCConfig(ctx context.Context, configPath string) ([]dcService, error) {
	dcc := dockercompose.NewDockerComposeClient()
	configOut, err := dcc.Config(ctx, configPath)
	if err != nil {
		return nil, err
	}

	config := dcConfig{}
	err = yaml.Unmarshal([]byte(configOut), &config)
	if err != nil {
		return nil, err
	}

	svcNames, err := svcNames(ctx, dcc, configPath)
	if err != nil {
		return nil, err
	}

	var services []dcService

	for _, name := range svcNames {
		svc, err := config.GetService(name)
		if err != nil {
			return nil, errors.Wrapf(err, "getting service %s", name)
		}
		services = append(services, svc)
	}

	return services, nil
}

func (s *tiltfileState) dcServiceToManifest(service dcService, dcConfigPath string) (manifest model.Manifest,
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

	// TODO(maia): ignore volumes mounted via dc.yml (b/c those auto-update)
	df := dockerfile.Dockerfile(service.DfContents)
	mounts, err := df.DeriveMounts(service.Context)
	if err != nil {
		return model.Manifest{}, nil, err
	}

	dcInfo.Mounts = mounts
	m = m.WithDeployTarget(dcInfo)

	paths := []string{path.Dir(service.DfPath), path.Dir(dcConfigPath)}
	for _, mount := range mounts {
		paths = append(paths, mount.LocalPath)
	}

	m = m.WithDockerignores(dockerignoresForPaths(append(paths, path.Dir(s.filename.path))))

	localPaths := []localPath{s.filename}
	for _, p := range paths {
		localPaths = append(localPaths, s.localPathFromString(p))
	}
	m = m.WithRepos(reposForPaths(localPaths))

	return m, []string{service.DfPath}, nil
}

package dockercompose

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"

	"gopkg.in/yaml.v2"
)

// Go representations of docker-compose.yml
// (Add fields as we need to support more things)
type Config struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	Build BuildConfig `yaml:"build"`
}

type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// A docker-compose service, according to Tilt.
type Service struct {
	Name    string
	Context string
	DfPath  string
}

func (c Config) GetService(name string) (Service, error) {
	svcConfig, ok := c.Services[name]
	if !ok {
		return Service{}, fmt.Errorf("no service %s found in config", name)
	}

	df := svcConfig.Build.Dockerfile
	if df == "" && svcConfig.Build.Context != "" {
		// We only expect a Dockerfile if there's a build context specified.
		df = "Dockerfile"
	}

	return Service{
		Name:    name,
		Context: svcConfig.Build.Context,
		DfPath:  path.Join(svcConfig.Build.Context, df),
	}, nil
}

func svcNames(ctx context.Context, configPath string) ([]string, error) {
	servicesText, err := dcOutput(ctx, configPath, "config", "--services")
	if err != nil {
		return nil, err
	}

	serviceNames := strings.Split(string(servicesText), "\n")

	var result []string

	for _, name := range serviceNames {
		if name == "" {
			continue
		}
		result = append(result, name)
	}

	return result, nil
}

func ParseConfig(ctx context.Context, configPath string) ([]Service, error) {
	configOut, err := dcOutput(ctx, configPath, "config")
	if err != nil {
		return nil, err
	}

	config := Config{}
	err = yaml.Unmarshal([]byte(configOut), &config)
	if err != nil {
		return nil, err
	}

	svcNames, err := svcNames(ctx, configPath)
	if err != nil {
		return nil, err
	}

	var services []Service

	for _, name := range svcNames {
		if name == "" {
			continue
		}
		svc, err := config.GetService(name)
		if err != nil {
			return nil, errors.Wrapf(err, "getting service %s", name)
		}
		services = append(services, svc)
	}

	return services, nil
}

func (s Service) ToManifest(dcConfigPath string) (model.Manifest, error) {
	m := model.Manifest{
		Name:       model.ManifestName(s.Name),
		DcYAMLPath: dcConfigPath,
	}

	if s.DfPath == "" {
		// DC service may not have Dockerfile -- e.g. may be just an image that we pull and run.
		return m, nil
	}

	// TODO(maia): record s.DfPath as config file!
	bs, err := ioutil.ReadFile(s.DfPath)
	if err != nil {
		return model.Manifest{}, errors.Wrapf(err, "opening Dockerfile for %s: '%s'", s.Name, s.DfPath)
	}

	// TODO(maia): ignore volumes mounted via dc.yml (b/c those auto-update)
	df := dockerfile.Dockerfile(bs)
	mounts, err := df.DeriveMounts(s.Context)
	if err != nil {
		return model.Manifest{}, err
	}

	m.Mounts = mounts
	return m, nil
}

func dcOutput(ctx context.Context, configPath string, args ...string) (string, error) {
	args = append([]string{"-f", configPath}, args...)
	output, err := exec.CommandContext(ctx, "docker-compose", args...).Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command 'docker-compose %q' failed.\nerror: '%v'\nstdout: '%v'", args, err, string(output))
		if err, ok := err.(*exec.ExitError); ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
		}
		err = fmt.Errorf(errorMessage)
	}
	return string(output), err
}

func FormatError(cmd *exec.Cmd, stdout []byte, err error) error {
	if err == nil {
		return nil
	}
	errorMessage := fmt.Sprintf("command '%q %q' failed.\nerror: '%v'\n", cmd.Path, cmd.Args, err)
	if len(stdout) > 0 {
		errorMessage += fmt.Sprintf("\nstdout: '%v'", string(stdout))
	}
	if err, ok := err.(*exec.ExitError); ok && len(err.Stderr) > 0 {
		errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
	}
	return fmt.Errorf(errorMessage)
}

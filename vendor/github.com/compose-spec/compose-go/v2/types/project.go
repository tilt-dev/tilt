/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package types

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/compose-spec/compose-go/v2/errdefs"
	"github.com/compose-spec/compose-go/v2/utils"
	"github.com/distribution/reference"
	godigest "github.com/opencontainers/go-digest"
	"go.yaml.in/yaml/v4"
	"golang.org/x/sync/errgroup"
)

// Project is the result of loading a set of compose files
// Since v2, Project are managed as immutable objects.
// Each public functions which mutate Project state now return a copy of the original Project with the expected changes.
type Project struct {
	Name       string     `yaml:"name,omitempty" json:"name,omitempty"`
	WorkingDir string     `yaml:"-" json:"-"`
	Services   Services   `yaml:"services" json:"services"`
	Networks   Networks   `yaml:"networks,omitempty" json:"networks,omitempty"`
	Volumes    Volumes    `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Secrets    Secrets    `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Configs    Configs    `yaml:"configs,omitempty" json:"configs,omitempty"`
	Models     Models     `yaml:"models,omitempty" json:"models,omitempty"`
	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"` // https://github.com/golang/go/issues/6213

	ComposeFiles []string `yaml:"-" json:"-"`
	Environment  Mapping  `yaml:"-" json:"-"`

	// DisabledServices track services which have been disable as profile is not active
	DisabledServices Services `yaml:"-" json:"-"`
	Profiles         []string `yaml:"-" json:"-"`
}

// ServiceNames return names for all services in this Compose config
func (p *Project) ServiceNames() []string {
	var names []string
	for k := range p.Services {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// DisabledServiceNames return names for all disabled services in this Compose config
func (p *Project) DisabledServiceNames() []string {
	var names []string
	for k := range p.DisabledServices {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// VolumeNames return names for all volumes in this Compose config
func (p *Project) VolumeNames() []string {
	var names []string
	for k := range p.Volumes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// NetworkNames return names for all volumes in this Compose config
func (p *Project) NetworkNames() []string {
	var names []string
	for k := range p.Networks {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// SecretNames return names for all secrets in this Compose config
func (p *Project) SecretNames() []string {
	var names []string
	for k := range p.Secrets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ConfigNames return names for all configs in this Compose config
func (p *Project) ConfigNames() []string {
	var names []string
	for k := range p.Configs {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ModelNames return names for all models in this Compose config
func (p *Project) ModelNames() []string {
	var names []string
	for k := range p.Models {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func (p *Project) ServicesWithBuild() []string {
	servicesBuild := p.Services.Filter(func(s ServiceConfig) bool {
		return s.Build != nil && s.Build.Context != ""
	})
	return slices.Collect(maps.Keys(servicesBuild))
}

func (p *Project) ServicesWithExtends() []string {
	servicesExtends := p.Services.Filter(func(s ServiceConfig) bool {
		return s.Extends != nil && *s.Extends != (ExtendsConfig{})
	})
	return slices.Collect(maps.Keys(servicesExtends))
}

func (p *Project) ServicesWithDependsOn() []string {
	servicesDependsOn := p.Services.Filter(func(s ServiceConfig) bool {
		return len(s.DependsOn) > 0
	})
	return slices.Collect(maps.Keys(servicesDependsOn))
}

func (p *Project) ServicesWithModels() []string {
	servicesModels := p.Services.Filter(func(s ServiceConfig) bool { return len(s.Models) > 0 })
	return slices.Collect(maps.Keys(servicesModels))
}

func (p *Project) ServicesWithCapabilities() ([]string, []string, []string) {
	capabilities := []string{}
	gpu := []string{}
	tpu := []string{}
	for _, service := range p.Services {
		deploy := service.Deploy
		if deploy == nil {
			continue
		}
		reservation := deploy.Resources.Reservations
		if reservation == nil {
			continue
		}
		devices := reservation.Devices
		for _, d := range devices {
			if len(d.Capabilities) > 0 {
				capabilities = append(capabilities, service.Name)
			}
			for _, c := range d.Capabilities {
				switch c {
				case "gpu":
					gpu = append(gpu, service.Name)
				case "tpu":
					tpu = append(tpu, service.Name)
				}
			}
		}
	}

	return utils.RemoveDuplicates(capabilities), utils.RemoveDuplicates(gpu), utils.RemoveDuplicates(tpu)
}

// GetServices retrieve services by names, or return all services if no name specified
func (p *Project) GetServices(names ...string) (Services, error) {
	if len(names) == 0 {
		return p.Services, nil
	}
	services := Services{}
	for _, name := range names {
		service, err := p.GetService(name)
		if err != nil {
			return nil, err
		}
		services[name] = service
	}
	return services, nil
}

func (p *Project) getServicesByNames(names ...string) (Services, []string) {
	if len(names) == 0 {
		return p.Services, nil
	}

	services := Services{}
	var servicesNotFound []string
	for _, name := range names {
		matched := false

		for serviceName, service := range p.Services {
			match, _ := filepath.Match(name, serviceName)
			if match {
				services[serviceName] = service
				matched = true
			}
		}

		if !matched {
			servicesNotFound = append(servicesNotFound, name)
		}
	}

	return services, servicesNotFound
}

// GetDisabledService retrieve disabled service by name
func (p Project) GetDisabledService(name string) (ServiceConfig, error) {
	service, ok := p.DisabledServices[name]
	if !ok {
		return ServiceConfig{}, fmt.Errorf("no such service: %s", name)
	}
	return service, nil
}

// GetService retrieve a specific service by name
func (p *Project) GetService(name string) (ServiceConfig, error) {
	service, ok := p.Services[name]
	if !ok {
		_, ok := p.DisabledServices[name]
		if ok {
			return ServiceConfig{}, fmt.Errorf("no such service: %s: %w", name, errdefs.ErrDisabled)
		}
		return ServiceConfig{}, fmt.Errorf("no such service: %s: %w", name, errdefs.ErrNotFound)
	}
	return service, nil
}

func (p *Project) AllServices() Services {
	all := Services{}
	for name, service := range p.Services {
		all[name] = service
	}
	for name, service := range p.DisabledServices {
		all[name] = service
	}
	return all
}

type ServiceFunc func(name string, service *ServiceConfig) error

// ForEachService runs ServiceFunc on each service and dependencies according to DependencyPolicy
func (p *Project) ForEachService(names []string, fn ServiceFunc, options ...DependencyOption) error {
	if len(options) == 0 {
		// backward compatibility
		options = []DependencyOption{IncludeDependencies}
	}
	return p.withServices(names, fn, map[string]bool{}, options, map[string]ServiceDependency{})
}

type withServicesOptions struct {
	dependencyPolicy int
}

const (
	includeDependencies = iota
	includeDependents
	ignoreDependencies
)

func (p *Project) withServices(names []string, fn ServiceFunc, seen map[string]bool, options []DependencyOption, dependencies map[string]ServiceDependency) error {
	services, servicesNotFound := p.getServicesByNames(names...)
	if len(servicesNotFound) > 0 {
		for _, serviceNotFound := range servicesNotFound {
			if dependency, ok := dependencies[serviceNotFound]; !ok || dependency.Required {
				return fmt.Errorf("no such service: %s", serviceNotFound)
			}
		}
	}
	opts := withServicesOptions{
		dependencyPolicy: includeDependencies,
	}
	for _, option := range options {
		option(&opts)
	}

	for name, service := range services {
		if seen[name] {
			continue
		}
		seen[name] = true
		var dependencies map[string]ServiceDependency
		switch opts.dependencyPolicy {
		case includeDependents:
			dependencies = utils.MapsAppend(dependencies, p.dependentsForService(service))
		case includeDependencies:
			dependencies = utils.MapsAppend(dependencies, service.DependsOn)
		case ignoreDependencies:
			// Noop
		}
		if len(dependencies) > 0 {
			err := p.withServices(utils.MapKeys(dependencies), fn, seen, options, dependencies)
			if err != nil {
				return err
			}
		}
		if err := fn(name, service.deepCopy()); err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) GetDependentsForService(s ServiceConfig, filter ...func(ServiceDependency) bool) []string {
	return utils.MapKeys(p.dependentsForService(s, filter...))
}

func (p *Project) dependentsForService(s ServiceConfig, filter ...func(ServiceDependency) bool) map[string]ServiceDependency {
	dependent := make(map[string]ServiceDependency)
	for _, service := range p.Services {
		for name, dependency := range service.DependsOn {
			if name == s.Name {
				depends := true
				for _, f := range filter {
					if !f(dependency) {
						depends = false
						break
					}
				}
				if depends {
					dependent[service.Name] = dependency
				}
			}
		}
	}
	return dependent
}

// RelativePath resolve a relative path based project's working directory
func (p *Project) RelativePath(path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.WorkingDir, path)
}

// HasProfile return true if service has no profile declared or has at least one profile matching
func (s ServiceConfig) HasProfile(profiles []string) bool {
	if len(s.Profiles) == 0 {
		return true
	}
	for _, p := range profiles {
		if p == "*" {
			return true
		}
		for _, sp := range s.Profiles {
			if sp == p {
				return true
			}
		}
	}
	return false
}

// WithProfiles disables services which don't match selected profiles
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p *Project) WithProfiles(profiles []string) (*Project, error) {
	newProject := p.deepCopy()
	enabled := Services{}
	disabled := Services{}
	for name, service := range newProject.AllServices() {
		if service.HasProfile(profiles) {
			enabled[name] = service
		} else {
			disabled[name] = service
		}
	}
	newProject.Services = enabled
	newProject.DisabledServices = disabled
	newProject.Profiles = profiles
	return newProject, nil
}

// WithServicesEnabled ensures services are enabled and activate profiles accordingly
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p *Project) WithServicesEnabled(names ...string) (*Project, error) {
	newProject := p.deepCopy()
	if len(names) == 0 {
		return newProject, nil
	}

	profiles := append([]string{}, p.Profiles...)
	for _, name := range names {
		if _, ok := newProject.Services[name]; ok {
			// already enabled
			continue
		}
		service := p.DisabledServices[name]
		profiles = append(profiles, service.Profiles...)
	}
	return newProject.WithProfiles(profiles)
}

// WithoutUnnecessaryResources drops networks/volumes/secrets/configs that are not referenced by active services
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p *Project) WithoutUnnecessaryResources() *Project {
	newProject := p.deepCopy()
	requiredNetworks := map[string]struct{}{}
	requiredVolumes := map[string]struct{}{}
	requiredSecrets := map[string]struct{}{}
	requiredConfigs := map[string]struct{}{}
	requiredModels := map[string]struct{}{}
	for _, s := range newProject.Services {
		for k := range s.Networks {
			requiredNetworks[k] = struct{}{}
		}
		for _, v := range s.Volumes {
			if v.Type != VolumeTypeVolume || v.Source == "" {
				continue
			}
			requiredVolumes[v.Source] = struct{}{}
		}
		for _, v := range s.Secrets {
			requiredSecrets[v.Source] = struct{}{}
		}
		if s.Build != nil {
			for _, v := range s.Build.Secrets {
				requiredSecrets[v.Source] = struct{}{}
			}
		}
		for _, v := range s.Configs {
			requiredConfigs[v.Source] = struct{}{}
		}
		for m := range s.Models {
			requiredModels[m] = struct{}{}
		}
	}

	networks := Networks{}
	for k := range requiredNetworks {
		if value, ok := p.Networks[k]; ok {
			networks[k] = value
		}
	}
	newProject.Networks = networks

	volumes := Volumes{}
	for k := range requiredVolumes {
		if value, ok := p.Volumes[k]; ok {
			volumes[k] = value
		}
	}
	newProject.Volumes = volumes

	secrets := Secrets{}
	for k := range requiredSecrets {
		if value, ok := p.Secrets[k]; ok {
			secrets[k] = value
		}
	}
	newProject.Secrets = secrets

	configs := Configs{}
	for k := range requiredConfigs {
		if value, ok := p.Configs[k]; ok {
			configs[k] = value
		}
	}
	newProject.Configs = configs

	models := Models{}
	for k := range requiredModels {
		if value, ok := p.Models[k]; ok {
			models[k] = value
		}
	}
	newProject.Models = models
	return newProject
}

type DependencyOption func(options *withServicesOptions)

func IncludeDependencies(options *withServicesOptions) {
	options.dependencyPolicy = includeDependencies
}

func IncludeDependents(options *withServicesOptions) {
	options.dependencyPolicy = includeDependents
}

func IgnoreDependencies(options *withServicesOptions) {
	options.dependencyPolicy = ignoreDependencies
}

// WithSelectedServices restricts the project model to selected services and dependencies
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p *Project) WithSelectedServices(names []string, options ...DependencyOption) (*Project, error) {
	newProject := p.deepCopy()
	if len(names) == 0 {
		// All services
		return newProject, nil
	}

	set := utils.NewSet[string]()
	err := p.ForEachService(names, func(name string, _ *ServiceConfig) error {
		set.Add(name)
		return nil
	}, options...)
	if err != nil {
		return nil, err
	}

	// Disable all services which are not explicit target or dependencies
	enabled := Services{}
	for name, s := range newProject.Services {
		if _, ok := set[name]; ok {
			// remove all dependencies but those implied by explicitly selected services
			dependencies := s.DependsOn
			for d := range dependencies {
				if _, ok := set[d]; !ok {
					delete(dependencies, d)
				}
			}
			s.DependsOn = dependencies
			enabled[name] = s
		} else {
			newProject = newProject.WithServicesDisabled(name)
		}
	}
	newProject.Services = enabled
	return newProject, nil
}

// WithServicesDisabled removes from the project model the given services and their references in all dependencies
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p *Project) WithServicesDisabled(names ...string) *Project {
	newProject := p.deepCopy()
	if len(names) == 0 {
		return newProject
	}
	if newProject.DisabledServices == nil {
		newProject.DisabledServices = Services{}
	}
	for _, name := range names {
		// We should remove all dependencies which reference the disabled service
		for i, s := range newProject.Services {
			if _, ok := s.DependsOn[name]; ok {
				delete(s.DependsOn, name)
				newProject.Services[i] = s
			}
		}
		if service, ok := newProject.Services[name]; ok {
			newProject.DisabledServices[name] = service
			delete(newProject.Services, name)
		}
	}
	return newProject
}

// WithImagesResolved updates services images to include digest computed by a resolver function
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p *Project) WithImagesResolved(resolver func(named reference.Named) (godigest.Digest, error)) (*Project, error) {
	return p.WithServicesTransform(func(_ string, service ServiceConfig) (ServiceConfig, error) {
		if service.Image == "" {
			return service, nil
		}
		named, err := reference.ParseDockerRef(service.Image)
		if err != nil {
			return service, err
		}

		if _, ok := named.(reference.Canonical); !ok {
			// image is named but not digested reference
			digest, err := resolver(named)
			if err != nil {
				return service, err
			}
			named, err = reference.WithDigest(named, digest)
			if err != nil {
				return service, err
			}
		}
		service.Image = named.String()
		return service, nil
	})
}

type marshallOptions struct {
	secretsContent bool
}

func WithSecretContent(o *marshallOptions) {
	o.secretsContent = true
}

func (opt *marshallOptions) apply(p *Project) *Project {
	if opt.secretsContent {
		p = p.deepCopy()
		for name, config := range p.Secrets {
			config.marshallContent = true
			p.Secrets[name] = config
		}
	}
	return p
}

func applyMarshallOptions(p *Project, options ...func(*marshallOptions)) *Project {
	opts := &marshallOptions{}
	for _, option := range options {
		option(opts)
	}
	p = opts.apply(p)
	return p
}

// MarshalYAML marshal Project into a yaml tree
func (p *Project) MarshalYAML(options ...func(*marshallOptions)) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	encoder := yaml.NewEncoder(buf)
	encoder.SetIndent(2)
	// encoder.CompactSeqIndent() FIXME https://github.com/go-yaml/yaml/pull/753
	src := applyMarshallOptions(p, options...)
	err := encoder.Encode(src)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalJSON marshal Project into a json document
func (p *Project) MarshalJSON(options ...func(*marshallOptions)) ([]byte, error) {
	src := applyMarshallOptions(p, options...)
	m := map[string]interface{}{
		"name":     src.Name,
		"services": src.Services,
	}

	if len(src.Networks) > 0 {
		m["networks"] = src.Networks
	}
	if len(src.Volumes) > 0 {
		m["volumes"] = src.Volumes
	}
	if len(src.Secrets) > 0 {
		m["secrets"] = src.Secrets
	}
	if len(src.Configs) > 0 {
		m["configs"] = src.Configs
	}
	for k, v := range src.Extensions {
		m[k] = v
	}
	return json.MarshalIndent(m, "", "  ")
}

// WithServicesEnvironmentResolved parses env_files set for services to resolve the actual environment map for services
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p Project) WithServicesEnvironmentResolved(discardEnvFiles bool) (*Project, error) {
	newProject := p.deepCopy()
	for i, service := range newProject.Services {
		service.Environment = service.Environment.Resolve(newProject.Environment.Resolve)

		environment := service.Environment.ToMapping()
		for _, envFile := range service.EnvFiles {
			err := loadEnvFile(envFile, environment, func(k string) (string, bool) {
				// project.env has precedence doing interpolation
				if resolve, ok := p.Environment.Resolve(k); ok {
					return resolve, true
				}
				// then service.environment
				if s, ok := service.Environment[k]; ok && s != nil {
					return *s, true
				}
				return "", false
			})
			if err != nil {
				return nil, err
			}
		}

		service.Environment = environment.ToMappingWithEquals().OverrideBy(service.Environment)

		if discardEnvFiles {
			service.EnvFiles = nil
		}
		newProject.Services[i] = service
	}
	return newProject, nil
}

// WithServicesLabelsResolved parses label_files set for services to resolve the actual label map for services
// It returns a new Project instance with the changes and keep the original Project unchanged
func (p Project) WithServicesLabelsResolved(discardLabelFiles bool) (*Project, error) {
	newProject := p.deepCopy()
	for i, service := range newProject.Services {
		labels := MappingWithEquals{}
		// resolve variables based on other files we already parsed
		var resolve dotenv.LookupFn = func(s string) (string, bool) {
			v, ok := labels[s]
			if ok && v != nil {
				return *v, ok
			}
			return "", false
		}

		for _, labelFile := range service.LabelFiles {
			vars, err := loadLabelFile(labelFile, resolve)
			if err != nil {
				return nil, err
			}
			labels.OverrideBy(vars.ToMappingWithEquals())
		}

		labels = labels.OverrideBy(service.Labels.ToMappingWithEquals())
		if len(labels) == 0 {
			labels = nil
		} else {
			service.Labels = NewLabelsFromMappingWithEquals(labels)
		}

		if discardLabelFiles {
			service.LabelFiles = nil
		}
		newProject.Services[i] = service
	}
	return newProject, nil
}

func loadEnvFile(envFile EnvFile, environment Mapping, resolve dotenv.LookupFn) error {
	if _, err := os.Stat(envFile.Path); os.IsNotExist(err) {
		if envFile.Required {
			return fmt.Errorf("env file %s not found: %w", envFile.Path, err)
		}
		return nil
	}

	err := loadMappingFile(envFile.Path, envFile.Format, environment, resolve)
	return err
}

func loadLabelFile(labelFile string, resolve dotenv.LookupFn) (Mapping, error) {
	if _, err := os.Stat(labelFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("label file %s not found: %w", labelFile, err)
	}

	labels := Mapping{}
	err := loadMappingFile(labelFile, "", labels, resolve)
	return labels, err
}

func loadMappingFile(path string, format string, vars Mapping, resolve dotenv.LookupFn) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return dotenv.ParseWithFormat(file, path, vars, resolve, format)
}

func (p *Project) deepCopy() *Project {
	if p == nil {
		return nil
	}
	n := &Project{}
	deriveDeepCopyProject(n, p)
	return n
}

// WithServicesTransform applies a transformation to project services and return a new project with transformation results
func (p *Project) WithServicesTransform(fn func(name string, s ServiceConfig) (ServiceConfig, error)) (*Project, error) {
	type result struct {
		name    string
		service ServiceConfig
	}
	expect := len(p.Services)
	resultCh := make(chan result, expect)
	newProject := p.deepCopy()

	eg, ctx := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		s := Services{}
		for expect > 0 {
			select {
			case <-ctx.Done():
				// interrupted as some goroutine returned an error
				return nil
			case r := <-resultCh:
				s[r.name] = r.service
				expect--
			}
		}
		newProject.Services = s
		return nil
	})
	for n, s := range newProject.Services {
		name := n
		service := s
		eg.Go(func() error {
			updated, err := fn(name, service)
			if err != nil {
				return err
			}
			resultCh <- result{
				name:    name,
				service: updated,
			}
			return nil
		})
	}
	return newProject, eg.Wait()
}

// CheckContainerNameUnicity validate project doesn't have services declaring the same container_name
func (p *Project) CheckContainerNameUnicity() error {
	names := utils.Set[string]{}
	for name, s := range p.Services {
		if s.ContainerName != "" {
			if existing, ok := names[s.ContainerName]; ok {
				return fmt.Errorf(`services.%s: container name %q is already in use by service %s"`, name, s.ContainerName, existing)
			}
			names.Add(s.ContainerName)
		}
	}
	return nil
}

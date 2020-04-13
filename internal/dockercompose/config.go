package dockercompose

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Go representations of docker-compose.yml
// (Add fields as we need to support more things)
type Config struct {
	Services map[string]ServiceConfig
}

// TODO(maia): maybe don't even need a custom func here anymore
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Services map[string]map[string]interface{} `yaml:"services"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	if c.Services == nil {
		c.Services = make(map[string]ServiceConfig)
	}

	for k, v := range aux.Services {
		c.Services[k] = NewServiceConfig(k, v)
	}
	return nil
}

type ServiceConfig struct {
	name string
	raw  ServiceConfigRaw
}
type ServiceConfigRaw map[string]interface{}

func NewServiceConfig(name string, raw ServiceConfigRaw) ServiceConfig {
	if raw == nil {
		raw = make(ServiceConfigRaw)
	}
	return ServiceConfig{
		name: name,
		raw:  raw,
	}
}

func (c ServiceConfig) GetImage() string {
	val, _ := c.raw["image"].(string)
	// TODO(maia): handle errors??
	return val
}

func (c ServiceConfig) WithImage(image string) ServiceConfig {
	c.raw["image"] = image
	return c
}

func (c ServiceConfig) GetBuild() BuildConfig {
	// TODO(maia): handle errors
	bc := BuildConfig{}

	val := c.raw["build"]
	b, err := yaml.Marshal(val)
	if err != nil {
		return bc
	}

	err = yaml.Unmarshal(b, &bc)
	if err != nil {
		return bc
	}
	return bc
}

func (c ServiceConfig) WithBuildContext(context string) ServiceConfig {
	build, ok := c.raw["build"]
	if !ok {
		c.raw["build"] = map[string]string{"context": context}
		return c
	}
	buildMap, ok := build.(map[string]string)
	if !ok {
		// TODO(maia): better error handling?
		//   I thiiink we can assume the format of this value if it exists,
		//   b/c this is a normalized config, but uh??
		panic("whelp")
	}
	buildMap["context"] = context
	return c
}

func (c ServiceConfig) GetVolumes() Volumes {
	// TODO(maia): handle errors
	vols := Volumes{}

	val := c.raw["volumes"]
	b, err := yaml.Marshal(val)
	if err != nil {
		return vols
	}

	err = yaml.Unmarshal(b, &vols)
	if err != nil {
		return vols
	}
	return vols
}

func (c ServiceConfig) GetPorts() Ports {
	// TODO(maia): handle errors
	ports := Ports{}

	val := c.raw["ports"]
	b, err := yaml.Marshal(val)
	if err != nil {
		return ports
	}

	err = yaml.Unmarshal(b, &ports)
	if err != nil {
		return ports
	}
	return ports
}

func (c ServiceConfig) SerializeYAML() string {
	services := map[string]ServiceConfigRaw{c.name: c.raw}

	// TODO(maia): handle errors
	b, err := yaml.Marshal(services)
	if err != nil {
		return ""
	}
	return string(b)
}

type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
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

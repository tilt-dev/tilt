package dockercompose

import (
	"fmt"
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

// We use a custom Unmarshal method here so that we can store the RawYAML in addition
// to unmarshaling the fields we care about into structs.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Services map[string]interface{} `yaml:"services"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	if c.Services == nil {
		c.Services = make(map[string]ServiceConfig)
	}

	for k, v := range aux.Services {
		b, err := yaml.Marshal(v) // so we can unmarshal it again
		if err != nil {
			return err
		}

		svcConf := &ServiceConfig{}
		err = yaml.Unmarshal(b, svcConf)
		if err != nil {
			return err
		}

		svcConf.RawYAML = b
		c.Services[k] = *svcConf
	}
	return nil
}

type ServiceConfig struct {
	RawYAML []byte      // We store this to diff against when docker-compose.yml is edited to see if the manifest has changed
	Build   BuildConfig `yaml:"build"`
	Image   string      `yaml:"image"`
	Volumes Volumes     `yaml:"volumes"`
	Ports   Ports       `yaml:"ports"`
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
			source := parts[0]

			// docker-compose uses : as a separator, but also normalizes
			// windows paths to absolute (C:\foo\bar), so special-case this.
			if len(parts) >= 2 && strings.HasPrefix(parts[1], "\\") {
				source = fmt.Sprintf("%s:%s", parts[0], parts[1])
			}
			*v = append(*v, Volume{Source: source})
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

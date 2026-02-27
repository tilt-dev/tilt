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

// BuildConfig is a type for build
type BuildConfig struct {
	Context            string                    `yaml:"context,omitempty" json:"context,omitempty"`
	Dockerfile         string                    `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`
	DockerfileInline   string                    `yaml:"dockerfile_inline,omitempty" json:"dockerfile_inline,omitempty"`
	Entitlements       []string                  `yaml:"entitlements,omitempty" json:"entitlements,omitempty"`
	Args               MappingWithEquals         `yaml:"args,omitempty" json:"args,omitempty"`
	Provenance         string                    `yaml:"provenance,omitempty" json:"provenance,omitempty"`
	SBOM               string                    `yaml:"sbom,omitempty" json:"sbom,omitempty"`
	SSH                SSHConfig                 `yaml:"ssh,omitempty" json:"ssh,omitempty"`
	Labels             Labels                    `yaml:"labels,omitempty" json:"labels,omitempty"`
	CacheFrom          StringList                `yaml:"cache_from,omitempty" json:"cache_from,omitempty"`
	CacheTo            StringList                `yaml:"cache_to,omitempty" json:"cache_to,omitempty"`
	NoCache            bool                      `yaml:"no_cache,omitempty" json:"no_cache,omitempty"`
	NoCacheFilter      StringList                `yaml:"no_cache_filter,omitempty" json:"no_cache_filter,omitempty"`
	AdditionalContexts Mapping                   `yaml:"additional_contexts,omitempty" json:"additional_contexts,omitempty"`
	Pull               bool                      `yaml:"pull,omitempty" json:"pull,omitempty"`
	ExtraHosts         HostsList                 `yaml:"extra_hosts,omitempty" json:"extra_hosts,omitempty"`
	Isolation          string                    `yaml:"isolation,omitempty" json:"isolation,omitempty"`
	Network            string                    `yaml:"network,omitempty" json:"network,omitempty"`
	Target             string                    `yaml:"target,omitempty" json:"target,omitempty"`
	Secrets            []ServiceSecretConfig     `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	ShmSize            UnitBytes                 `yaml:"shm_size,omitempty" json:"shm_size,omitempty"`
	Tags               StringList                `yaml:"tags,omitempty" json:"tags,omitempty"`
	Ulimits            map[string]*UlimitsConfig `yaml:"ulimits,omitempty" json:"ulimits,omitempty"`
	Platforms          StringList                `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Privileged         bool                      `yaml:"privileged,omitempty" json:"privileged,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

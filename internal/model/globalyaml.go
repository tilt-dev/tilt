package model

import "github.com/docker/distribution/reference"

const GlobalYAMLManifestName = ManifestName("GlobalYAML")

type YAMLManifest struct {
	name    ManifestName
	k8sYAML string

	configFiles []string
}

func NewYAMLManifest(name ManifestName, k8sYaml string, configFiles []string) YAMLManifest {
	return YAMLManifest{
		name:        name,
		k8sYAML:     k8sYaml,
		configFiles: configFiles,
	}
}
func (y YAMLManifest) Dependencies() []string {
	return y.configFiles
}

func (y YAMLManifest) ManifestName() ManifestName {
	return y.name
}

func (y YAMLManifest) ConfigMatcher() (PathMatcher, error) {
	configMatcher, err := NewSimpleFileMatcher(y.configFiles...)
	if err != nil {
		return nil, err
	}

	return configMatcher, nil
}

func (YAMLManifest) LocalRepos() []LocalGithubRepo {
	return []LocalGithubRepo{}
}

func (y YAMLManifest) K8sYAML() string {
	return y.k8sYAML
}

// TODO(dmiller): not sure if this is right
func (YAMLManifest) DockerRef() reference.Named {
	n, err := reference.ParseNamed("")
	if err != nil {
		// This space intentionally left blank
	}

	return n
}

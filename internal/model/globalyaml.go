package model

import "github.com/docker/distribution/reference"

type YAMLManifest struct {
	name    ManifestName
	k8sYAML string

	configFiles []string
}

func NewYAMLManifest(name ManifestName, k8sYAML string, configFiles []string) YAMLManifest {
	return YAMLManifest{
		name:        name,
		k8sYAML:     k8sYAML,
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
	n, err := reference.ParseNamed("windmill.build/fakeimage:tilt-11cd0b38bc3ce")
	if err != nil {
		// This space intentionally left blank
	}

	return n
}

func (y YAMLManifest) Empty() bool {
	return y.k8sYAML == ""
}

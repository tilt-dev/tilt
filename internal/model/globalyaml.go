package model

type YAMLManifest struct {
	name    ManifestName
	k8sYaml string

	configFiles []string
}

func NewYAMLManifest(name ManifestName, k8sYaml string, configFiles []string) YAMLManifest {
	return YAMLManifest{
		name:        name,
		k8sYaml:     k8sYaml,
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

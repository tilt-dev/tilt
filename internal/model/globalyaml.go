package model

// TODO(maia): throw an error if you try to name a manifest this in your Tiltfile?
<<<<<<< HEAD
const GlobalYAMLManifestName = ManifestName("GlobalYAML")

type YAMLManifest struct {
	name    ManifestName
	k8sYAML string

	configFiles []string
	resources   []YAMLManifestResource
}

type YAMLManifestResource struct {
	Name string
	Kind string
}

func NewYAMLManifest(name ManifestName, k8sYaml string, configFiles []string, resources []YAMLManifestResource) YAMLManifest {
	return YAMLManifest{
		name:        name,
		k8sYAML:     k8sYaml,
		configFiles: configFiles,
		resources:   resources,
	}
}

func (y YAMLManifest) Dependencies() []string {
	return y.configFiles
}

func (y YAMLManifest) Resources() []YAMLManifestResource {
	return y.resources
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

func (y YAMLManifest) K8sYAML() string {
	return y.k8sYAML
}

func (y YAMLManifest) Empty() bool {
	return y.K8sYAML() == ""
}
=======
const GlobalYAMLManifestName = ManifestName("k8s_yaml")
>>>>>>> master

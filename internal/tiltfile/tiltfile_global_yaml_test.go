package tiltfile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/yaml"
)

func TestGlobalYAML(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("global.yaml", "this is the global yaml")
	f.WriteFile("Tiltfile", `yaml = read_file('./global.yaml')
global_yaml(yaml)`)

	globalYAML := f.LoadGlobalYAML()
	assert.Equal(t, globalYAML.K8sYAML(), "this is the global yaml")
	assert.Equal(t, globalYAML.Dependencies(), []string{f.JoinPath("global.yaml")})
}

func TestGlobalYAMLMultipleCallsThrowsError(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", `global_yaml('abc')
global_yaml('def')`)

	_, err := Load(f.ctx, FileName)
	if assert.Error(t, err, "expect multiple invocations of `global_yaml` to result in error") {
		assert.Equal(t, err.Error(), "`global_yaml` can be called only once per Tiltfile")
	}
}

func TestAllManifestsHaveGlobalYAMLDependencies(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	f.WriteFile("fileA", "apples")
	f.WriteFile("fileB", "bananas")
	f.WriteFile("Dockerfile", "FROM iron/go:dev")

	f.WriteFile("global.yaml", testyaml.SecretYaml)
	f.WriteFile("Tiltfile", `yaml = read_file('./global.yaml')
global_yaml(yaml)

def manifestA():
  stuff = read_file('./fileA')  # this is a dependency of manifestA now
  image = static_build('Dockerfile', 'tag-a')
  return k8s_service("yamlA", image)

def manifestB():
  stuff = read_file('./fileB')  # this is a dependency of manifestB now
  image = static_build('Dockerfile', 'tag-b')
  return k8s_service("yamlB", image)
`)

	manifests := f.LoadManifests("manifestA", "manifestB")

	expectedDepsA := []string{"fileA", "Dockerfile", "Tiltfile", "global.yaml"}
	expectedDepsB := []string{"fileB", "Dockerfile", "Tiltfile", "global.yaml"}
	assert.ElementsMatch(t, manifests[0].ConfigFiles, f.JoinPaths(expectedDepsA))
	assert.ElementsMatch(t, manifests[1].ConfigFiles, f.JoinPaths(expectedDepsB))
}

func TestPerManifestYAMLExtractedFromGlobalYAML(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml, testyaml.SecretYaml)
	f.WriteFile("global.yaml", multiManifestYAML)
	f.WriteFile("Dockerfile", "FROM iron/go:dev")
	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

def doggos():
  image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
  return k8s_service("", image)

def snack():
  image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/snack')
  return k8s_service("", image)
`)

	manifests, gYAML := f.LoadManifestsAndGlobalYAML("doggos", "snack")

	assertYAMLEqual(t, testyaml.DoggosDeploymentYaml, manifests[0].K8sYAML())
	assertYAMLEqual(t, testyaml.SnackYaml, manifests[1].K8sYAML())
	assertYAMLEqual(t, testyaml.SecretYaml, gYAML.K8sYAML())
}

func TestAllYAMLExtractedFromGlobalYAML(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml)
	f.WriteFile("global.yaml", multiManifestYAML)
	f.WriteFile("Dockerfile", "FROM iron/go:dev")
	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

def doggos():
  image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
  return k8s_service("", image)

def snack():
  image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/snack')
  return k8s_service("", image)
`)

	manifests, gYAML := f.LoadManifestsAndGlobalYAML("doggos", "snack")

	assertYAMLEqual(t, testyaml.DoggosDeploymentYaml, manifests[0].K8sYAML())
	assertYAMLEqual(t, testyaml.SnackYaml, manifests[1].K8sYAML())
	assert.Empty(t, gYAML.K8sYAML())
}

func TestExtractedGlobalYAMLStacksWithExplicitYaml(t *testing.T) {
	f := newGitRepoFixture(t)
	defer f.TearDown()

	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SecretYaml)
	f.WriteFile("global.yaml", multiManifestYAML)
	f.WriteFile("Dockerfile", "FROM iron/go:dev")
	f.WriteFile("Tiltfile", fmt.Sprintf(`global_yaml(read_file('./global.yaml'))

def doggos_with_secret():
  image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
  return k8s_service("""%s""", image)
`, testyaml.DoggosServiceYaml))

	manifests, _ := f.LoadManifestsAndGlobalYAML("doggos_with_secret")

	assertYAMLContains(t, manifests[0].K8sYAML(), testyaml.DoggosDeploymentYaml)
	assertYAMLContains(t, manifests[0].K8sYAML(), testyaml.DoggosServiceYaml)
}

func assertYAMLEqual(t *testing.T, y1, y2 string) {
	// Obviously equal
	if y1 == y2 {
		return
	}

	// Strings aren't equal, but compare as k8sEntities -- can be equivalent but
	// lose string equivalency in un/parsing
	e1, err := k8s.ParseYAMLFromString(y1)
	if err != nil {
		t.Fatal("parsing y1 to assert equality:", err)
	}
	e2, err := k8s.ParseYAMLFromString(y2)
	if err != nil {
		t.Fatal("parsing y2 to assert equality:", err)
	}

	assert.ElementsMatch(t, e1, e2)
}

func assertYAMLContains(t *testing.T, yaml, check string) {
	entities, err := k8s.ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal("parsing yaml to assert YAML-contains:", err)
	}
	checkEntities, err := k8s.ParseYAMLFromString(check)
	if err != nil {
		t.Fatal("parsing 'contains' yaml to assert YAML-contains:", err)
	}

	for _, chk := range checkEntities {
		assert.Contains(t, entities, chk)
	}
}

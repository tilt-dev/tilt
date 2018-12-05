package tiltfile

// import (
// 	"fmt"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/windmilleng/tilt/internal/k8s"
// 	"github.com/windmilleng/tilt/internal/k8s/testyaml"
// 	"github.com/windmilleng/tilt/internal/yaml"
// )

// func TestGlobalYAML(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	f.WriteFile("global.yaml", "this is the global yaml")
// 	f.WriteFile("Tiltfile", `yaml = read_file('./global.yaml')
// global_yaml(yaml)

// def manifestA():
//   stuff = read_file('./fileA')
//   image = static_build('Dockerfile', 'tag-a')
//   return k8s_service("yamlA", image)
// 	`)

// 	globalYAML := f.LoadGlobalYAML()
// 	assert.Equal(t, globalYAML.K8sYAML(), "this is the global yaml")
// 	assert.Equal(t, globalYAML.Dependencies(), []string{f.JoinPath("global.yaml")})
// }

// func TestGlobalYAMLMultipleCallsThrowsError(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	f.WriteFile("Tiltfile", `global_yaml('abc')
// global_yaml('def')`)

// 	_, err := Load(f.ctx, FileName)
// 	if assert.Error(t, err, "expect multiple invocations of `global_yaml` to result in error") {
// 		assert.Contains(t, err.Error(), "`global_yaml` can be called only once per Tiltfile")
// 	}
// }

// func TestConfigFilesIncludeGlobalYAMLDependencies(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	f.WriteFile("fileA", "apples")
// 	f.WriteFile("fileB", "bananas")
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")

// 	f.WriteFile("global.yaml", testyaml.SecretYaml)
// 	f.WriteFile("Tiltfile", `yaml = read_file('./global.yaml')
// global_yaml(yaml)

// def manifestA():
//   stuff = read_file('./fileA')  # this is a dependency of manifestA now
//   image = static_build('Dockerfile', 'tag-a')
//   return k8s_service(image, yaml="yamlA")

// def manifestB():
//   stuff = read_file('./fileB')  # this is a dependency of manifestB now
//   image = static_build('Dockerfile', 'tag-b')
//   return k8s_service(image, yaml="yamlB")
// `)

// 	_, _, configFiles := f.LoadAllFromTiltfile("manifestA", "manifestB")

// 	expectedConfigFiles := []string{"fileA", "fileB", "Dockerfile", "global.yaml"}
// 	for _, expected := range expectedConfigFiles {
// 		assert.Contains(t, configFiles, f.JoinPath(expected))
// 	}
// }

// func TestPerManifestYAMLExtractedFromGlobalYAML(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml, testyaml.SecretYaml)
// 	f.WriteFile("global.yaml", multiManifestYAML)
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")
// 	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

// def doggos():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
//   return k8s_service(image)

// def snack():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/snack')
//   return k8s_service(image)
// `)

// 	manifests, gYAML, _ := f.LoadAllFromTiltfile("doggos", "snack")

// 	assertYAMLEqual(t, testyaml.DoggosDeploymentYaml, manifests[0].K8sYAML())
// 	assertYAMLEqual(t, testyaml.SnackYaml, manifests[1].K8sYAML())
// 	assertYAMLEqual(t, testyaml.SecretYaml, gYAML.K8sYAML())
// }

// func TestPerManifestYAMLExtractedFromGlobalYAMLForCompositeService(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml, testyaml.SecretYaml)
// 	f.WriteFile("global.yaml", multiManifestYAML)
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")
// 	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

// def compserv():
//   return composite_service([doggos, snack])

// def doggos():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
//   return k8s_service(image)

// def snack():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/snack')
//   return k8s_service(image)
// `)

// 	manifests, gYAML, _ := f.LoadAllFromTiltfile("compserv")

// 	assertYAMLEqual(t, testyaml.DoggosDeploymentYaml, manifests[0].K8sYAML())
// 	assertYAMLEqual(t, testyaml.SnackYaml, manifests[1].K8sYAML())
// 	assertYAMLEqual(t, testyaml.SecretYaml, gYAML.K8sYAML())
// }

// func TestAllYAMLExtractedFromGlobalYAML(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml)
// 	f.WriteFile("global.yaml", multiManifestYAML)
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")
// 	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

// def doggos():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
//   return k8s_service(image)

// def snack():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/snack')
//   return k8s_service(image)
// `)

// 	manifests, gYAML, _ := f.LoadAllFromTiltfile("doggos", "snack")

// 	assertYAMLEqual(t, testyaml.DoggosDeploymentYaml, manifests[0].K8sYAML())
// 	assertYAMLEqual(t, testyaml.SnackYaml, manifests[1].K8sYAML())
// 	assert.Empty(t, gYAML.K8sYAML())
// }

// func TestExtractedGlobalYAMLStacksWithExplicitYaml(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	multiManifestYAML := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SecretYaml)
// 	f.WriteFile("global.yaml", multiManifestYAML)
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")
// 	f.WriteFile("Tiltfile", fmt.Sprintf(`global_yaml(read_file('./global.yaml'))

// def doggos_with_secret():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
//   return k8s_service(image, yaml="""%s""")
// `, testyaml.DoggosServiceYaml))

// 	manifests, _, _ := f.LoadAllFromTiltfile("doggos_with_secret")

// 	assertYAMLContains(t, manifests[0].K8sYAML(), testyaml.DoggosDeploymentYaml)
// 	assertYAMLContains(t, manifests[0].K8sYAML(), testyaml.DoggosServiceYaml)
// }

// func TestExtractedYAMLAssociatesViaImageAndSelector(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	gYAMLStr := yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml, testyaml.SecretYaml)
// 	f.WriteFile("global.yaml", gYAMLStr)
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")
// 	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

// def doggos():
//   image = static_build('Dockerfile', 'gcr.io/windmill-public-containers/servantes/doggos')
//   return k8s_service(image)
// `)

// 	manifests, gYAML, _ := f.LoadAllFromTiltfile("doggos")

// 	assertYAMLContains(t, manifests[0].K8sYAML(), testyaml.DoggosDeploymentYaml,
// 		"expected Deployment yaml on Doggos manifest")
// 	assertYAMLContains(t, manifests[0].K8sYAML(), testyaml.DoggosServiceYaml,
// 		"expected Service yaml on Doggos manifest")
// 	assertYAMLEqual(t, testyaml.SecretYaml, gYAML.K8sYAML(),
// 		"expected global YAML to only contain SecretYaml (all else extracted)")
// }

// func TestTwinsInGlobalYAML(t *testing.T) {
// 	f := newGitRepoFixture(t)
// 	defer f.TearDown()

// 	yaml := yaml.ConcatYAML(testyaml.SanchoYAML, testyaml.SanchoTwinYAML)
// 	f.WriteFile("global.yaml", yaml)
// 	f.WriteFile("Dockerfile", "FROM iron/go:dev")
// 	f.WriteFile("Tiltfile", `global_yaml(read_file('./global.yaml'))

// def sancho():
//   image = static_build('Dockerfile', 'gcr.io/some-project-162817/sancho')
//   return k8s_service(image)
// `)

// 	manifests, gYAML, _ := f.LoadAllFromTiltfile("sancho")

// 	assertYAMLEqual(t, yaml, manifests[0].K8sYAML())
// 	assertYAMLEqual(t, "", gYAML.K8sYAML())
// }

// func assertYAMLEqual(t *testing.T, y1, y2 string, msgAndArgs ...interface{}) {
// 	// Obviously equal
// 	if y1 == y2 {
// 		return
// 	}

// 	// Strings aren't equal, but compare as k8sEntities -- can be equivalent but
// 	// lose string equivalency in un/parsing
// 	e1, err := k8s.ParseYAMLFromString(y1)
// 	if err != nil {
// 		t.Fatal("parsing y1 to assert equality:", err)
// 	}
// 	e2, err := k8s.ParseYAMLFromString(y2)
// 	if err != nil {
// 		t.Fatal("parsing y2 to assert equality:", err)
// 	}

// 	assert.ElementsMatch(t, e1, e2, msgAndArgs...)
// }

// func assertYAMLContains(t *testing.T, yaml, check string, msgAndArgs ...interface{}) {
// 	entities, err := k8s.ParseYAMLFromString(yaml)
// 	if err != nil {
// 		t.Fatal("parsing yaml to assert YAML-contains:", err)
// 	}
// 	checkEntities, err := k8s.ParseYAMLFromString(check)
// 	if err != nil {
// 		t.Fatal("parsing 'contains' yaml to assert YAML-contains:", err)
// 	}

// 	for _, chk := range checkEntities {
// 		assert.Contains(t, entities, chk, msgAndArgs...)
// 	}
// }

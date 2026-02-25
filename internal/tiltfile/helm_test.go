//go:build !skiplargetiltfiletests
// +build !skiplargetiltfiletests

// On windows, running Helm can take ~0.5 seconds,
// which starts to blow up test times.

package tiltfile

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/testdata"
)

func TestHelm(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()

	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("chart-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

func TestHelmArgs(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace='garnet', values=['./dev/helm/values-dev.yaml'])
k8s_yaml(yml)
`)

	f.load()

	m := f.assertNextManifestUnresourced("rose-quartz-helloworld-chart")
	yaml := m.K8sTarget().YAML
	assert.Contains(t, yaml, "release: rose-quartz")
	assert.Contains(t, yaml, "namespace: garnet")
	assert.Contains(t, yaml, "namespaceLabel: garnet")
	assert.Contains(t, yaml, "name: nginx-dev")

	entities, err := k8s.ParseYAMLFromString(yaml)
	require.NoError(t, err)

	names := k8s.UniqueNames(entities, 2)
	expectedNames := []string{"rose-quartz-helloworld-chart:service"}
	assert.ElementsMatch(t, expectedNames, names)

	f.assertConfigFiles("./helm/", "./dev/helm/values-dev.yaml", ".tiltignore", "Tiltfile")
}

func TestHelmNamespaceFlagDoesNotInsertNSEntityIfNSInChart(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()

	valuesWithNamespace := `
namespace:
  enabled: true
  name: foobarbaz`
	f.file("helm/extra_values.yaml", valuesWithNamespace)

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace="foobarbaz", values=['./helm/extra_values.yaml'])
k8s_yaml(yml)
`)

	f.load()

	m := f.assertNextManifestUnresourced("foobarbaz", "rose-quartz-helloworld-chart")
	yaml := m.K8sTarget().YAML

	entities, err := k8s.ParseYAMLFromString(yaml)
	require.NoError(t, err)
	require.Len(t, entities, 2)
	e := entities[0]
	require.Equal(t, "Namespace", e.GVK().Kind)
	assert.Equal(t, "foobarbaz", e.Name())
	assert.Equal(t, "indeed", e.Labels()["somePersistedLabel"],
		"label originally specified in chart YAML should persist")
}

func TestHelmNamespaceFlagInsertsNSEntityIfDifferentNSInChart(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()

	valuesWithNamespace := `
namespace:
  enabled: true
  name: not-the-one-specified-in-flag` // what kind of jerk would do this?
	f.file("helm/extra_values.yaml", valuesWithNamespace)

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace="foobarbaz", values=['./helm/extra_values.yaml'])
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("not-the-one-specified-in-flag", "rose-quartz-helloworld-chart")
}

func TestHelmInvalidDirectory(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.loadErrString("Could not read Helm chart directory")
}

func TestHelmFromRepoPath(t *testing.T) {
	f := newFixture(t)

	f.gitInit(".")
	f.setupHelm()

	f.file("Tiltfile", `
r = local_git_repo('.')
yml = helm(r.paths('helm'))
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("chart-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

func TestHelmMalformedChart(t *testing.T) {
	f := newFixture(t)

	f.WriteFile("./helm/Chart.yaml", "brrrrr")

	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.loadErrString("error unmarshaling JSON")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

func TestHelmNamespace(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()
	f.file("helm/templates/public-config.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: public-config
  namespace: kube-public
data:
  noData: "true"
`)

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace='garnet')
k8s_yaml(yml)
`)

	f.load()

	m := f.assertNextManifestUnresourced(
		"public-config",
		"rose-quartz-helloworld-chart")
	yaml := m.K8sTarget().YAML

	assert.Contains(t, yaml, "name: rose-quartz-helloworld-chart\n  namespace: garnet")
	assert.Contains(t, yaml, "name: public-config\n  namespace: kube-public")
}

func TestHelmSetArgs(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace='garnet', set=[
  'ingress.enabled=true',
  'service.externalPort=1234',
  'service.internalPort=5678'
])
k8s_yaml(yml)
`)

	f.load()

	m := f.assertNextManifestUnresourced(
		// A service and ingress with the same name
		"rose-quartz-helloworld-chart",
		"rose-quartz-helloworld-chart")
	yaml := m.K8sTarget().YAML

	// Set on the service
	assert.Contains(t, yaml, "port: 1234")
	assert.Contains(t, yaml, "targetPort: 5678")

	// Set on the ingress
	assert.Contains(t, yaml, "serviceName: rose-quartz-helloworld-chart")
	assert.Contains(t, yaml, "servicePort: 1234")
}

func TestHelmSetArgsMap(t *testing.T) {
	f := newFixture(t)

	f.setupHelm()

	f.file("Tiltfile", `
yml = helm('./helm', name='rose-quartz', namespace='garnet', set={'a': 'b'})
k8s_yaml(yml)
`)

	f.loadErrString("helm: for parameter \"set\"", "string", "List", "type dict")
}

const exampleHelmV2VersionOutput = `Client: v2.12.3geecf22f`
const exampleHelmV3_0VersionOutput = `v3.0.0`
const exampleHelmV3_1VersionOutput = `v3.1.0`
const exampleHelmV3_2VersionOutput = `v3.2.4`
const examplePkgxHelmV3_15VersionOutput = `3.15.2`
const exampleHelmV4_0VersionOutput = `v4.0.0+g99cd196`

// see https://github.com/tilt-dev/tilt/issues/3788
const exampleHelmV3_3VersionOutput = `WARNING: Kubernetes configuration file is group-readable. This is insecure. Location: /Users/someone/.kube/config
WARNING: Kubernetes configuration file is world-readable. This is insecure. Location: /Users/someone/.kube/config
v3.3.3+g55e3ca0
`

func TestParseHelmV2Version(t *testing.T) {
	_, err := parseVersion(exampleHelmV2VersionOutput)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse Helm version from string")
}

func TestParseHelmV3Version(t *testing.T) {
	expected := helmV3_0
	assertHelmVersion(t, exampleHelmV3_0VersionOutput, expected)
}

func TestParseHelmV3_1Version(t *testing.T) {
	expected := helmV3_1andAbove
	assertHelmVersion(t, exampleHelmV3_1VersionOutput, expected)
}

func TestParseHelmV3_2Version(t *testing.T) {
	expected := helmV3_1andAbove
	assertHelmVersion(t, exampleHelmV3_2VersionOutput, expected)
}

func TestParseHelmV3_3Version(t *testing.T) {
	expected := helmV3_1andAbove
	assertHelmVersion(t, exampleHelmV3_3VersionOutput, expected)
}

func TestParsePkgxHelmV3_15Version(t *testing.T) {
	expected := helmV3_1andAbove
	assertHelmVersion(t, examplePkgxHelmV3_15VersionOutput, expected)
}

func TestParsePkgxHelmV4(t *testing.T) {
	assertHelmVersion(t, exampleHelmV4_0VersionOutput, helmV3_1andAbove)
}

func TestHelmUnknownVersionError(t *testing.T) {
	_, err := parseVersion("vx.1.2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse Helm version from string")
}

const fileRequirementsYAML = `dependencies:
  - name: foobar
    version: 1.0.1
    repository: file://./foobar`

func TestLocalSubchartFileDependencies(t *testing.T) {
	input := []byte(fileRequirementsYAML)
	expected := "./foobar"
	actual, err := localSubchartDependencies(input)
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, actual, expected)
}

const remoteRequirementsYAML = `
dependencies:
- name: etcd
  version: 0.6.2
  repository: https://kubernetes-charts-incubator.storage.googleapis.com/
  condition: etcd.deployChart`

func TestSubchartRemoteDependencies(t *testing.T) {
	input := []byte(remoteRequirementsYAML)
	actual, err := localSubchartDependencies(input)
	if err != nil {
		t.Fatal(err)
	}

	assert.Empty(t, actual)
}

func TestHelmReleaseName(t *testing.T) {
	f := newFixture(t)

	f.file("helm/Chart.yaml", `apiVersion: v1
description: grafana chart
name: grafana
version: 0.1.0`)

	f.file("helm/values.yaml", testdata.GrafanaHelmValues)
	f.file("helm/templates/_helpers.tpl", testdata.GrafanaHelmHelpers)
	f.file("helm/templates/service-account.yaml", testdata.GrafanaHelmServiceAccount)

	f.file("Tiltfile", `
k8s_yaml(helm('./helm'))
`)

	f.load()

	manifests := f.loadResult.Manifests
	require.Equal(t, 1, len(manifests))

	m := manifests[0]
	yaml := m.K8sTarget().YAML
	assert.NotContains(t, yaml, "RELEASE-NAME")
	assert.Contains(t, yaml, "name: chart-grafana")
}

func TestHelm3CRD(t *testing.T) {
	f := newFixture(t)

	f.file("helm/Chart.yaml", `apiVersion: v1
description: crd chart
name: crd
version: 0.1.0`)

	f.file("helm/templates/service-account.yaml", `apiVersion: v1
kind: ServiceAccount
metadata:
  name: crd-sa`)

	// Only works in Helm3
	// https://helm.sh/docs/chart_best_practices/custom_resource_definitions/
	f.file("helm/crds/um.yaml", `apiVersion: tilt.dev/v1alpha1
kind: UselessMachine
metadata:
  name: bobo
spec:
  image: bobo`)

	f.file("Tiltfile", `
k8s_yaml(helm('./helm'))
`)

	f.load()

	manifests := f.loadResult.Manifests
	require.Equal(t, 1, len(manifests))

	m := manifests[0]
	yaml := m.K8sTarget().YAML
	out, execErr := exec.Command("helm", "version", "--short").Output()
	var v helmVersion
	if execErr == nil {
		v, _ = parseVersion(string(out))
	}
	assert.NoError(t, execErr)
	assert.Contains(t, yaml, "kind: ServiceAccount")
	if v == helmV3_0 || v == helmV3_1andAbove {
		assert.Contains(t, yaml, "kind: UselessMachine")
	} else {
		assert.NotContains(t, yaml, "kind: UselessMachine")
	}
}

func assertHelmVersion(t *testing.T, versionOutput string, expectedV helmVersion) {
	actualV, err := parseVersion(versionOutput)
	require.NoError(t, err, "parsing helm version")
	require.Equal(t, expectedV, actualV)
}

func TestYamlErrorFromHelm(t *testing.T) {
	f := newFixture(t)
	f.setupHelm()
	f.file("helm/templates/foo.yaml", "hi")
	f.file("Tiltfile", `
k8s_yaml(helm('helm'))
`)
	f.loadErrString("in helm")
}

func TestHelmSkipsTests(t *testing.T) {
	f := newFixture(t)

	f.setupHelmWithTest()
	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("chart-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

func TestHelmIncludesRequirements(t *testing.T) {
	f := newFixture(t)

	f.setupHelmWithRequirements()
	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.load()
	f.assertNextManifest("chart-nginx-ingress-controller")
}

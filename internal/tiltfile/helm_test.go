package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/testdata"
)

func TestHelmMalformedChart(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

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
	defer f.TearDown()

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
	defer f.TearDown()

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
	defer f.TearDown()

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

func TestParseHelmV2Version(t *testing.T) {
	expected := helmV2
	actual := parseVersion(exampleHelmV2VersionOutput)

	assert.Equal(t, expected, actual)
}

func TestParseHelmV3Version(t *testing.T) {
	expected := helmV3_0
	actual := parseVersion(exampleHelmV3_0VersionOutput)

	assert.Equal(t, expected, actual)
}

func TestParseHelmV3_1Version(t *testing.T) {
	expected := helmV3_1andAbove
	actual := parseVersion(exampleHelmV3_1VersionOutput)

	assert.Equal(t, expected, actual)
}

func TestParseHelmV3_2Version(t *testing.T) {
	expected := helmV3_1andAbove
	actual := parseVersion(exampleHelmV3_2VersionOutput)

	assert.Equal(t, expected, actual)
}

func TestHelmUnknownVersion(t *testing.T) {
	expected := unknownHelmVersion
	actual := parseVersion("v4.1.2")

	assert.Equal(t, expected, actual)
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
	defer f.TearDown()

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
	defer f.TearDown()

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
	v, err := getHelmVersion()
	assert.NoError(t, err)
	assert.Contains(t, yaml, "kind: ServiceAccount")
	if v == helmV3_0 || v == helmV3_1andAbove {
		assert.Contains(t, yaml, "kind: UselessMachine")
	} else {
		assert.NotContains(t, yaml, "kind: UselessMachine")
	}
}

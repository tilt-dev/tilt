package kustomize

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
)

func TestNoFile(t *testing.T) {
	f := newKustomizeFixture(t)

	f.assertErrorContains("unable to find one of [kustomization.yaml kustomization.yml Kustomization] in directory ")
}

func TestTooManyFiles(t *testing.T) {
	f := newKustomizeFixture(t)
	f.tempdir.WriteFile("kustomization.yml", "")
	f.tempdir.WriteFile("kustomization.yaml", "")

	f.assertErrorContains("Found multiple kustomization files under")
}

func TestEmpty(t *testing.T) {
	f := newKustomizeFixture(t)
	kustomizeFile := ""
	f.writeRootKustomize(kustomizeFile)

	expected := []string{"kustomization.yaml"}

	f.assertDeps(expected)
}

func TestSimple(t *testing.T) {
	f := newKustomizeFixture(t)
	kustomizeFile := `# Example configuration for the webserver
# at https://github.com/monopole/hello
commonLabels:
  app: my-hello

resources:
  - deployment.yaml
  - service.yaml
  - configMap.yaml`
	f.writeRootKustomize(kustomizeFile)

	expected := []string{"kustomization.yaml", "deployment.yaml", "service.yaml", "configMap.yaml"}
	f.assertDeps(expected)
}

func TestComplex(t *testing.T) {
	f := newKustomizeFixture(t)
	kustomizeFile := `
# declare ConfigMap as a resource
resources:
- configmap.yaml

# declare ConfigMap from a ConfigMapGenerator
configMapGenerator:
- name: a-configmap
  files:
    - configs/configfile
    - configs/another_configfile

patchesJson6902:
  - target:
      group: extensions
      version: v1beta1
      kind: Ingress
      name: my-ingress
    path: ingress_patch.yaml`
	f.writeRootKustomize(kustomizeFile)

	expected := []string{"kustomization.yaml", "configmap.yaml", "ingress_patch.yaml", "configs/configfile", "configs/another_configfile"}
	f.assertDeps(expected)
}

func TestRecursive(t *testing.T) {
	f := newKustomizeFixture(t)

	// these used to be only specified under "bases", but now that's deprecated and "resources"
	// that purpose. for now, test both.
	// https://github.com/tilt-dev/tilt/blob/15d0c94ccc08230d3a528b14cb0a3455b947d13c/vendor/sigs.k8s.io/kustomize/api/types/kustomization.go#L102
	kustomize := `bases:
- ./dev
components:
- ./component
resources:
- ./staging
- ./production

namePrefix: cluster-a-`

	f.writeRootKustomize(kustomize)

	base := `resources:
- pod.yaml`
	f.writeBaseKustomize("base", base)
	basePod := `apiVersion: v1
  kind: Pod
  metadata:
    name: myapp-pod
    labels:
      app: myapp
  spec:
    containers:
    - name: nginx
      image: nginx:1.7.9`
	f.writeBaseFile("base", "pod.yaml", basePod)

	dev := `bases:
- ./../base
namePrefix: dev-`
	f.writeBaseKustomize("dev", dev)

	component := `labels:
  - pairs:
      instance: myapp`
	f.writeBaseKustomize("component", component)

	staging := `bases:
- ./../base
namePrefix: stag-`
	f.writeBaseKustomize("staging", staging)

	production := `bases:
- ./../base
namePrefix: prod-`
	f.writeBaseKustomize("production", production)

	expected := []string{
		"base/kustomization.yaml",
		"base/pod.yaml",
		"component/kustomization.yaml",
		"dev/kustomization.yaml",
		"staging/kustomization.yaml",
		"production/kustomization.yaml",
		"kustomization.yaml",
	}
	f.assertDeps(expected)
}

// patches was deprecated and then re-added with a different meaning
// https://github.com/tilt-dev/tilt/issues/4081
func TestPatches(t *testing.T) {
	f := newKustomizeFixture(t)
	kustomizeFile := `# Example configuration for the webserver
# at https://github.com/monopole/hello
commonLabels:
  app: my-hello

resources:
  - deployment.yaml
  - service.yaml
  - configMap.yaml

patches:
  - path: patch.yaml
    target:
      kind: Deployment
      name: foo
`
	f.writeRootKustomize(kustomizeFile)

	expected := []string{"kustomization.yaml", "deployment.yaml", "service.yaml", "configMap.yaml", "patch.yaml"}
	f.assertDeps(expected)
}

type kustomizeFixture struct {
	t       *testing.T
	tempdir *tempdir.TempDirFixture
}

func newKustomizeFixture(t *testing.T) *kustomizeFixture {

	return &kustomizeFixture{
		t:       t,
		tempdir: tempdir.NewTempDirFixture(t),
	}
}

func (f *kustomizeFixture) writeRootKustomize(contents string) {
	f.tempdir.WriteFile("kustomization.yaml", contents)
}

func (f *kustomizeFixture) writeBaseKustomize(pathToContainingDirectory, contents string) {
	f.tempdir.WriteFile(filepath.Join(pathToContainingDirectory, "kustomization.yaml"), contents)
}

func (f *kustomizeFixture) writeBaseFile(pathToContainingDirectory, name, contents string) {
	f.tempdir.WriteFile(filepath.Join(pathToContainingDirectory, name), contents)
}

func (f *kustomizeFixture) getDeps() []string {
	deps, err := Deps(f.tempdir.Path())
	if err != nil {
		f.t.Fatal(err)
	}

	return deps
}

func (f *kustomizeFixture) assertDeps(expected []string) {
	fullExpected := f.tempdir.JoinPaths(expected)

	actual := f.getDeps()

	require.ElementsMatch(f.t, fullExpected, actual)
}

func (f *kustomizeFixture) assertErrorContains(expected string) {
	_, err := Deps(f.tempdir.Path())
	if err == nil {
		f.t.Fatal("Expected an error, got nil")
	}

	if !strings.Contains(err.Error(), expected) {
		f.t.Errorf("Expected %s to contain %s", err.Error(), expected)
	}
}

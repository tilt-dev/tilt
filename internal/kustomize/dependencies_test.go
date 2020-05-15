package kustomize

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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

	kustomize := `bases:
- ./dev
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
		"dev/kustomization.yaml",
		"staging/kustomization.yaml",
		"production/kustomization.yaml",
		"kustomization.yaml",
	}
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

	if !reflect.DeepEqual(actual, fullExpected) {
		f.t.Errorf("Expected \n%v\n to equal \n%v\n", actual, fullExpected)
	}
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

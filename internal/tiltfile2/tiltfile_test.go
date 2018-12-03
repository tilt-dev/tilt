package tiltfile2

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestSimple(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.file("foo/Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo/Dockerfile')
k8s_resource('foo', 'foo.yaml')
`)

	f.load()

	f.assertManifest("foo",
		image("gcr.io/foo"),
		deployment("foo"))
}

func TestMissingDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo/Dockerfile')
k8s_resource('foo', 'foo.yaml')
`)

	f.loadErrString("Reading path foo/Dockerfile", "no such file or directory")
}

type fixture struct {
	ctx context.Context
	t   *testing.T
	tmp *tempdir.TempDirFixture

	// created by load
	manifests   []model.Manifest
	configFiles []string
}

func newFixture(t *testing.T) *fixture {
	out := new(bytes.Buffer)
	ctx := output.ForkedCtxForTest(out)
	r := &fixture{
		ctx: ctx,
		t:   t,
		tmp: tempdir.NewTempDirFixture(t),
	}
	return r
}

func (f *fixture) tearDown() {
	f.tmp.TearDown()
}

func (f *fixture) file(path string, contents string) {
	f.tmp.WriteFile(path, contents)
}

type k8sOpts interface{}

func (f *fixture) yaml(path string, entities ...k8sOpts) {
	var entityObjs []k8s.K8sEntity

	for _, e := range entities {
		switch e := e.(type) {
		case deployHelper:
			s := testyaml.SnackYaml
			if e.image != "" {
				s = strings.Replace(s, testyaml.SnackImage, e.image, -1)
			}
			s = strings.Replace(s, testyaml.SnackName, e.name, -1)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}

			entityObjs = append(entityObjs, objs...)
		default:
			f.t.Fatalf("unexpected entity %T %v", e, e)
		}
	}

	s, err := k8s.SerializeYAML(entityObjs)
	if err != nil {
		f.t.Fatal(err)
	}

	f.file(path, s)
}

func (f *fixture) load() {
	manifests, _, configFiles, err := Load(f.ctx, f.tmp.JoinPath("Tiltfile"))
	if err != nil {
		f.t.Fatal(err)
	}
	f.manifests = manifests
	f.configFiles = configFiles
}

func (f *fixture) loadErrString(msgs ...string) {
	manifests, _, configFiles, err := Load(f.ctx, f.tmp.JoinPath("Tiltfile"))
	if err == nil {
		f.t.Fatalf("expected error but got nil")
	}
	f.manifests = manifests
	f.configFiles = configFiles
	errText := err.Error()
	for _, msg := range msgs {
		if !strings.Contains(errText, msg) {
			f.t.Fatalf("error %q does not contain string %q", errText, msg)
		}
	}
}

func (f *fixture) assertManifest(name string, opts ...interface{}) model.Manifest {
	if len(f.manifests) == 0 {
		f.t.Fatalf("no more manifests; trying to find %q", name)
	}

	m := f.manifests[0]
	f.manifests = f.manifests[1:]

	for _, opt := range opts {
		switch opt := opt.(type) {
		case imageHelper:
			if m.DockerRef().Name() != opt.ref {
				f.t.Fatalf("manifest %v image ref: %q; expected %q", m.Name, m.DockerRef().Name(), opt.ref)
			}
		case deployHelper:

		}
	}
	return m

}

type deployHelper struct {
	name  string
	image string
}

func deployment(name string, opts ...interface{}) deployHelper {
	r := deployHelper{name: name}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case imageHelper:
			r.image = opt.ref
		}
	}
	return r
}

type imageHelper struct {
	ref string
}

func image(ref string) imageHelper {
	return imageHelper{ref: ref}
}

package tiltfile2

import (
	"testing"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestSimple(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.file("Tiltfile", `
docker_build('gcr.io/foo')
k8s_resource('foo', 'foo.yaml')
`)
	f.yaml("foo.yaml", deployment("foo", image("gcr.io")))

	f.parse()

	f.assertManifest("foo",
		image("gcr.io/foo"),
		deployment("foo"))
}

type fixture struct {
	t   *testing.T
	tmp *tempdir.TempDirFixture
}

func newFixture(t *testing.T) *fixture {
	r := &fixture{
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
			// FIXME(dbentley)
			entityObjs = append(entityObjs, obj)
		default:
			f.t.Fatalf("unexpected entity %T %v", e, e)
		}
	}

	s, err := k8s.SerializeYAML(entities)
	if err != nil {
		f.t.Fatal(err)
	}

	f.file(path, s)
}

type deployHelper struct {
	name   string
	images []string
}

func (f *fixture) deployment(name string, opts []interface{}) deployHelper {
	r := deployHelper{name: name}
	return r
}

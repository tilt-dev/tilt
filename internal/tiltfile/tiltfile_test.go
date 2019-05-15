package tiltfile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/yaml"

	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

const simpleDockerfile = "FROM golang:1.10"

func TestNoTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.loadErrString("No Tiltfile found at")
	f.assertConfigFiles("Tiltfile")
}

func TestEmpty(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", "")
	f.load()
}

func TestMissingDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
`)

	f.loadErrString("foo/Dockerfile", "no such file or directory", "error reading dockerfile")
}

func TestGitRepoBadMethodCall(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
local_git_repo('.').asdf()
`)

	f.loadErrString("Tiltfile:2: in <toplevel>", "Error: gitRepo has no .asdf field or method")
}

func TestFastBuildBadMethodCall(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
hfb = fast_build(
  'gcr.io/foo',
  'foo/Dockerfile',
  './entrypoint',
).asdf()
`)

	f.loadErrString("Error: fast_build has no .asdf field or method")
}

func TestCustomBuildBadMethodCall(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
hfb = custom_build(
  'gcr.io/foo',
  'docker build -t $TAG foo',
  ['foo']
).asdf()
`)

	f.loadErrString("Error: custom_build has no .asdf field or method")
}

func TestSimpleV1(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
`)

	f.loadResourceAssemblyV1("foo")

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestSimple(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load("foo")

	m := f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")

	iTarget := m.ImageTargetAt(0)

	// Make sure there's no fast build / live update in the default case.
	assert.True(t, iTarget.IsDockerBuild())
	assert.False(t, iTarget.IsFastBuild())
	assert.True(t, iTarget.AnyFastBuildInfo().Empty())
	assert.True(t, iTarget.AnyLiveUpdateInfo().Empty())
}

// I.e. make sure that we handle de/normalization between `fooimage` <--> `docker.io/library/fooimage`
func TestLocalImageRef(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("fooimage")))

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('fooimage', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(imageNormalized("fooimage")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestExplicitDockerfileIsConfigFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile='other/Dockerfile')
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "other/Dockerfile", "foo/.dockerignore")
}

func TestExplicitDockerfileAsLocalPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
r = local_git_repo('.')
docker_build('gcr.io/foo', 'foo', dockerfile=r.path('other/Dockerfile'))
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "other/Dockerfile", "foo/.dockerignore")
}

func TestExplicitDockerfileContents(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile_contents='FROM alpine')
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/.dockerignore")
	f.assertNextManifest("foo", db(image("gcr.io/foo")))
}

func TestExplicitDockerfileContentsAsBlob(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
df = read_file('other/Dockerfile')
docker_build('gcr.io/foo', 'foo', dockerfile_contents=df)
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "other/Dockerfile", "foo/.dockerignore")
	f.assertNextManifest("foo", db(image("gcr.io/foo")))
}

func TestCantSpecifyDFPathAndContents(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile_contents='FROM alpine', dockerfile='foo/Dockerfile')
k8s_yaml('foo.yaml')
`)

	f.loadErrString("Cannot specify both dockerfile and dockerfile_contents")
}

func TestFastBuildSimple(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add('foo', 'src/') \
  .run("echo hi")
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi"), hotReload(false)),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestFastBuildHotReload(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add('foo', 'src/') \
  .run("echo hi") \
  .hot_reload()
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi"), hotReload(true)),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestFastBuildPassedToResource(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
fb = fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add('foo', 'src/') \
  .run("echo hi")
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi")),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestFastBuildValidates(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("foo/Dockerfile", `
from golang:1.10
ADD . .`)
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add('foo', 'src/') \
  .run("echo hi")
k8s_yaml('foo.yaml')
`)
	f.loadErrString("base Dockerfile contains an ADD/COPY")
}

func TestFastBuildRunBeforeAdd(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .run("echo hi") \
  .add('foo', 'src/')
k8s_yaml('foo.yaml')
`)
	f.loadErrString("fast_build(\"gcr.io/foo\").add() called after .run()")
}

func TestDockerBuildWithEmbeddedFastBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
k8s_yaml(['foo.yaml', 'bar.yaml'])

docker_build('gcr.io/foo', 'foo')
fastbar = docker_build('gcr.io/bar', 'bar')

fastbar.add('local/path', 'remote/path')
fastbar.run('echo hi')
`)

	f.load()
	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertNextManifest("bar",
		db(image("gcr.io/bar"), nestedFB(add("local/path", "remote/path"), run("echo hi"))),
		deployment("bar"))
}

func TestFastBuildTriggers(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add('foo', 'src/') \
  .run("echo hi", trigger=['a', 'b']) \
  .run("echo again", trigger='c')
k8s_yaml('foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"),
			add("foo", "src/"),
			run("echo hi", "a", "b"),
			run("echo again", "c"),
		),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestVerifiesGitRepo(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", "local_git_repo('.')")
	f.loadErrString("isn't a valid git repo")
}

func TestLocal(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
yaml = local('cat foo.yaml')
k8s_yaml(yaml)
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
}

func TestReadFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
yaml = read_file('foo.yaml')
k8s_yaml(yaml)
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestKustomize(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("kustomization.yaml", kustomizeFileText)
	f.file("configMap.yaml", kustomizeConfigMapText)
	f.file("deployment.yaml", kustomizeDeploymentText)
	f.file("service.yaml", kustomizeServiceText)
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build("gcr.io/foo", "foo")
k8s_yaml(kustomize("."))
k8s_resource("the-deployment", "foo")
`)
	f.load()
	f.assertNextManifest("foo", deployment("the-deployment"), numEntities(2))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "configMap.yaml", "deployment.yaml", "kustomization.yaml", "service.yaml")
}

func TestKustomization(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Kustomization", kustomizeFileText)
	f.file("configMap.yaml", kustomizeConfigMapText)
	f.file("deployment.yaml", kustomizeDeploymentText)
	f.file("service.yaml", kustomizeServiceText)
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build("gcr.io/foo", "foo")
k8s_yaml(kustomize("."))
k8s_resource("the-deployment", "foo")
`)
	f.load()
	f.assertNextManifest("foo", deployment("the-deployment"), numEntities(2))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "configMap.yaml", "deployment.yaml", "Kustomization", "service.yaml")
}

func TestDockerBuildCache(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build("gcr.io/foo", "foo", cache='/path/to/cache')
`)
	f.load()
	f.assertNextManifest("foo", dbWithCache(image("gcr.io/foo"), "/path/to/cache"))
}

func TestFastBuildCache(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
fast_build("gcr.io/foo", 'foo/Dockerfile', cache='/path/to/cache')
`)
	f.load()
	f.assertNextManifest("foo", fbWithCache(image("gcr.io/foo"), "/path/to/cache"))
}

func TestDuplicateResourceNames(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExpand()
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('all.yaml')
k8s_resource('a')
k8s_resource('a')
`)

	f.loadErrString("k8s_resource already called for a")
}

func TestDuplicateImageNames(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/a', 'a')
`)

	f.loadErrString("Image for ref \"gcr.io/a\" has already been defined")
}

func TestInvalidImageName(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build("ceci n'est pas une valid image ref", 'a')
`)

	f.loadErrString("invalid reference format")
}

func TestFastBuildAddString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", fmt.Sprintf(`
k8s_yaml(['foo.yaml', 'bar.yaml'])

# fb.add() on string of relative path
fast_build('gcr.io/foo', 'foo/Dockerfile').add('./foo', '/foo')

# fb.add() on string of absolute path
fast_build('gcr.io/bar', 'foo/Dockerfile').add('%s', '/bar')
`, f.JoinPath("./bar")))

	f.load()
	f.assertNextManifest("foo", fb(image("gcr.io/foo"), add("foo", "/foo")))
	f.assertNextManifest("bar", fb(image("gcr.io/bar"), add("bar", "/bar")))
}

func TestFastBuildAddLocalPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
k8s_yaml('foo.yaml')
fast_build('gcr.io/foo', 'foo/Dockerfile').add(repo.path('foo'), '/foo')
`)

	f.load()
	f.assertNextManifest("foo", fb(image("gcr.io/foo"), add("foo", "/foo")))
}

func TestFastBuildAddGitRepo(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
k8s_yaml('foo.yaml')

# add the whole repo
fast_build('gcr.io/foo', 'foo/Dockerfile').add(repo, '/whole_repo')
`)

	f.load()
	f.assertNextManifest("foo", fb(image("gcr.io/foo"), add(".", "/whole_repo")))
}

type portForwardCase struct {
	name     string
	expr     string
	expected []model.PortForward
	errorMsg string
}

func TestPortForward(t *testing.T) {
	portForwardCases := []portForwardCase{
		{"value_local", "8000", []model.PortForward{{LocalPort: 8000}}, ""},
		{"value_local_negative", "-1", nil, "not in the range for a port"},
		{"value_local_large", "8000000", nil, "not in the range for a port"},
		{"value_string_local", "'10000'", []model.PortForward{{LocalPort: 10000}}, ""},
		{"value_string_both", "'10000:8000'", []model.PortForward{{LocalPort: 10000, ContainerPort: 8000}}, ""},
		{"value_string_garbage", "'garbage'", nil, "not in the range for a port"},
		{"value_string_3x80", "'80:80:80'", nil, "not in the range for a port"},
		{"value_string_empty", "''", nil, "not in the range for a port"},
		{"value_both", "port_forward(8001, 443)", []model.PortForward{{LocalPort: 8001, ContainerPort: 443}}, ""},
		{"list", "[8000, port_forward(8001, 443)]", []model.PortForward{{LocalPort: 8000}, {LocalPort: 8001, ContainerPort: 443}}, ""},
		{"list_string", "['8000', '8001:443']", []model.PortForward{{LocalPort: 8000}, {LocalPort: 8001, ContainerPort: 443}}, ""},
	}

	for _, c := range portForwardCases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()
			f.setupFoo()
			s := `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=EXPR)
`
			s = strings.Replace(s, "EXPR", c.expr, -1)
			f.file("Tiltfile", s)

			if c.errorMsg != "" {
				f.loadErrString(c.errorMsg)
				return
			}

			f.load()
			f.assertNextManifest("foo",
				c.expected,
				db(image("gcr.io/foo")),
				deployment("foo"))
		})
	}
}

func TestExpand(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
	f.assertNoMoreManifests() // should be no unresourced yaml remaining
	f.assertConfigFiles("Tiltfile", ".tiltignore", "all.yaml", "a/Dockerfile", "a/.dockerignore", "b/Dockerfile", "b/.dockerignore", "c/Dockerfile", "c/.dockerignore", "d/Dockerfile", "d/.dockerignore")
}

func TestExpandUnresourced(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.dockerfile("a/Dockerfile")

	f.yaml("all.yaml",
		deployment("a", image("gcr.io/a")),
		secret("a-secret"),
	)

	f.gitInit("")
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
`)

	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifestUnresourced("a-secret")
}

func TestExpandExplicit(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupExpand()
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
k8s_resource('explicit_a', image='gcr.io/a', port_forwards=8000)
`)
	f.loadResourceAssemblyV1()
	f.assertNextManifest("explicit_a", db(image("gcr.io/a")), deployment("a"), []model.PortForward{{LocalPort: 8000}})
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
}

func TestUnresourcedPodCreatorYamlAsManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.yaml("pod_creator.yaml", deployment("pod-creator"), secret("not-pod-creator"))

	f.file("Tiltfile", `
k8s_yaml('pod_creator.yaml')
`)
	f.load()

	f.assertNextManifest("pod-creator", deployment("pod-creator"))
	f.assertNextManifestUnresourced("not-pod-creator")
}

func TestUnresourcedYamlGroupingV1(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	labelsA := map[string]string{"keyA": "valueA"}
	labelsB := map[string]string{"keyB": "valueB"}
	labelsC := map[string]string{"keyC": "valueC"}
	f.yaml("all.yaml",
		deployment("deployment-a", withLabels(labelsA)),

		deployment("deployment-b", withLabels(labelsB)),
		service("service-b", withLabels(labelsB)),

		deployment("deployment-c", withLabels(labelsC)),
		service("service-c1", withLabels(labelsC)),
		service("service-c2", withLabels(labelsC)),

		secret("someSecret"),
	)

	f.file("Tiltfile", `k8s_yaml('all.yaml')`)
	f.load()

	f.assertNextManifest("deployment-a", deployment("deployment-a"))
	f.assertNextManifest("deployment-b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("deployment-c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
	f.assertNextManifestUnresourced("someSecret")
}

func TestUnresourcedYamlGroupingV2(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	labelsA := map[string]string{"keyA": "valueA"}
	labelsB := map[string]string{"keyB": "valueB"}
	labelsC := map[string]string{"keyC": "valueC"}
	f.yaml("all.yaml",
		deployment("deployment-a", withLabels(labelsA)),

		deployment("deployment-b", withLabels(labelsB)),
		service("service-b", withLabels(labelsB)),

		deployment("deployment-c", withLabels(labelsC)),
		service("service-c1", withLabels(labelsC)),
		service("service-c2", withLabels(labelsC)),

		secret("someSecret"),
	)

	f.file("Tiltfile", `
k8s_yaml('all.yaml')`)
	f.load()

	f.assertNextManifest("deployment-a", deployment("deployment-a"))
	f.assertNextManifest("deployment-b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("deployment-c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
	f.assertNextManifestUnresourced("someSecret")
}

func TestK8sGroupedWhenAddedToResource(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupExpand()

	labelsA := map[string]string{"keyA": "valueA"}
	labelsB := map[string]string{"keyB": "valueB"}
	labelsC := map[string]string{"keyC": "valueC"}
	f.yaml("all.yaml",
		deployment("deployment-a", image("gcr.io/a"), withLabels(labelsA)),

		deployment("deployment-b", image("gcr.io/b"), withLabels(labelsB)),
		service("service-b", withLabels(labelsB)),

		deployment("deployment-c", image("gcr.io/c"), withLabels(labelsC)),
		service("service-c1", withLabels(labelsC)),
		service("service-c2", withLabels(labelsC)),
	)

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
`)
	f.load()

	f.assertNextManifest("deployment-a", deployment("deployment-a"))
	f.assertNextManifest("deployment-b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("deployment-c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
}

func TestK8sResourceWithoutDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
k8s_resource('foo', yaml='foo.yaml', port_forwards=8000)
`)
	f.loadResourceAssemblyV1()
	f.assertNextManifest("foo", []model.PortForward{{LocalPort: 8000}})
}

func TestImplicitK8sResourceWithoutDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=8000)
`)
	f.load()
	f.assertNextManifest("foo", []model.PortForward{{LocalPort: 8000}})
}

func TestExpandTwoDeploymentsWithSameImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupExpand()
	f.yaml("all.yaml",
		deployment("a", image("gcr.io/a")),
		deployment("a2", image("gcr.io/a")),
		deployment("b", image("gcr.io/b")),
		deployment("c", image("gcr.io/c")),
		deployment("d", image("gcr.io/d")),
	)
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifest("a2", db(image("gcr.io/a")), deployment("a2"))
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
}

func TestMultipleYamlFiles(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExpand()
	f.yaml("a.yaml", deployment("a", image("gcr.io/a")))
	f.yaml("b.yaml", deployment("b", image("gcr.io/b")))
	f.yaml("c.yaml", deployment("c", image("gcr.io/c")))
	f.yaml("d.yaml", deployment("d", image("gcr.io/d")))
	f.file("Tiltfile", `
k8s_yaml(['a.yaml', 'b.yaml', 'c.yaml', 'd.yaml'])
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertNextManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertNextManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertNextManifest("d", db(image("gcr.io/d")), deployment("d"))
}

func TestLoadOneManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml('bar.yaml')
`)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml")
}

func TestLoadTypoManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml('bar.yaml')
`)

	_, err := f.tfl.Load(f.ctx, f.JoinPath("Tiltfile"), matchMap("baz"))
	if assert.Error(t, err) {
		assert.Equal(t, `You specified some resources that could not be found: "baz"
Is this a typo? Existing resources in Tiltfile: "foo", "bar"`, err.Error())
	}
}

func TestBasicGitPathFilter(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters(".git"),
		fileChangeFilters(".git"),
		buildFilters("Tiltfile"),
		fileChangeMatches("Tiltfile"),
		buildMatches("foo.yaml"),
		fileChangeMatches("foo.yaml"),
	)
}

func TestDockerignorePathFilter(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.file(".dockerignore", "*.txt")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("a.txt"),
		fileChangeFilters("a.txt"),
		buildMatches("txt.a"),
		fileChangeMatches("txt.a"),
	)
}

func TestDockerignorePathFilterSubdir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("foo/Dockerfile", "FROM golang:1.10")
	f.file("foo/.dockerignore", "*.txt")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("foo/a.txt"),
		fileChangeFilters("foo/a.txt"),
		buildMatches("foo/txt.a"),
		fileChangeMatches("foo/txt.a"),
	)
}

func TestFastBuildDockerignoreRoot(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file(".dockerignore", "foo/*.txt")
	f.file("Tiltfile", `
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi")
k8s_yaml('foo.yaml')
`)
	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("foo/a.txt"),
		fileChangeFilters("foo/a.txt"),
	)
}

func TestFastBuildDockerignoreSubdir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("foo/.dockerignore", "*.txt")
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add('foo', 'src/') \
  .run("echo hi")
k8s_yaml('foo.yaml')
`)
	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("foo/a.txt"),
		fileChangeFilters("foo/a.txt"),
		buildMatches("foo/subdir/a.txt"),
		fileChangeMatches("foo/subdir/a.txt"),
	)
}

func TestK8sYAMLInputBareString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.WriteFile("bar.yaml", "im not yaml")
	f.file("Tiltfile", `
k8s_yaml('bar.yaml')
docker_build("gcr.io/foo", "foo", cache='/path/to/cache')
`)

	f.loadErrString("bar.yaml is not a valid YAML file")
}

func TestK8sYAMLInputFromReadFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml(str(read_file('foo.yaml')))
docker_build("gcr.io/foo", "foo", cache='/path/to/cache')
`)

	f.loadErrString("no such file or directory")
}

func TestFilterYamlByLabel(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
labels = {'app': 'doggos'}
doggos, rest = filter_yaml('k8s.yaml', labels=labels)
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.loadResourceAssemblyV1()

	f.assertNextManifest("doggos", deployment("doggos"), service("doggos"))
	f.assertNextManifest("rest", deployment("snack"), deployment("sancho"))
}

func TestFilterYamlByName(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
doggos, rest = filter_yaml('k8s.yaml', name='doggos')
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.loadResourceAssemblyV1()

	f.assertNextManifest("doggos", deployment("doggos"), service("doggos"))
	f.assertNextManifest("rest", deployment("snack"), deployment("sancho"))
}

func TestFilterYamlByNameKind(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
doggos, rest = filter_yaml('k8s.yaml', name='doggos', kind='deployment')
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.loadResourceAssemblyV1()

	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNextManifest("rest", service("doggos"), deployment("snack"), deployment("sancho"))
}

func TestFilterYamlByNamespace(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
doggos, rest = filter_yaml('k8s.yaml', namespace='the-dog-zone')
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.loadResourceAssemblyV1()

	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNextManifest("rest", service("doggos"), deployment("snack"), deployment("sancho"))
}

func TestFilterYamlByApiVersion(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(
		testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml,
		testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
doggos, rest = filter_yaml('k8s.yaml', name='doggos', api_version='apps/v1')
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.loadResourceAssemblyV1()

	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNextManifest("rest", service("doggos"), deployment("snack"), deployment("sancho"))
}

func TestFilterYamlNoMatch(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
doggos, rest = filter_yaml('k8s.yaml', namespace='dne', kind='deployment')
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.loadErrString("could not associate any k8s YAML with this resource")
}

// These tests are for behavior that we specifically enabled in Starlark
// in the init() function
func TestTopLevelIfStatement(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
if True:
  docker_build('gcr.io/foo', 'foo')
  k8s_yaml('foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestTopLevelForLoop(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
for i in range(1, 3):
	print(i)
`)

	f.load()
}

func TestTopLevelVariableRename(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
x = 1
x = 2
`)

	f.load()
}

func TestHelm(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupHelm()

	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("release-name-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

func TestHelmInvalidDirectory(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.loadErrString("Expected to be able to read Helm templates in ")
}

func TestHelmFromRepoPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit(".")
	f.setupHelm()

	f.file("Tiltfile", `
r = local_git_repo('.')
yml = helm(r.path('helm'))
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("release-name-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

func TestEmptyDockerfileDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("foo/Dockerfile", "")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)
	f.load()
	m := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.True(t, m.ImageTargetAt(0).IsDockerBuild())
	assert.False(t, m.ImageTargetAt(0).IsFastBuild())
}

func TestEmptyDockerfileFastBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("foo/Dockerfile", "")
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile')
k8s_yaml('foo.yaml')
`)
	f.load()
	m := f.assertNextManifest("foo", fb(image("gcr.io/foo")))
	assert.False(t, m.ImageTargetAt(0).IsDockerBuild())
	assert.True(t, m.ImageTargetAt(0).IsFastBuild())
}

func TestSanchoSidecar(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("k8s.yaml", testyaml.SanchoSidecarYAML)
	f.file("Tiltfile", `
k8s_yaml('k8s.yaml')
docker_build('gcr.io/some-project-162817/sancho', '.')
docker_build('gcr.io/some-project-162817/sancho-sidecar', '.')
`)
	f.load()

	assert.Equal(t, 1, len(f.loadResult.Manifests))
	m := f.assertNextManifest("sancho")
	assert.Equal(t, 2, len(m.ImageTargets))
	assert.Equal(t, "gcr.io/some-project-162817/sancho",
		m.ImageTargetAt(0).ConfigurationRef.String())
	assert.Equal(t, "gcr.io/some-project-162817/sancho-sidecar",
		m.ImageTargetAt(1).ConfigurationRef.String())
}

func TestSanchoRedisSidecar(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Dockerfile", "FROM golang:1.10")
	f.file("k8s.yaml", testyaml.SanchoRedisSidecarYAML)
	f.file("Tiltfile", `
k8s_yaml('k8s.yaml')
docker_build('gcr.io/some-project-162817/sancho', '.')
`)
	f.load()

	assert.Equal(t, 1, len(f.loadResult.Manifests))
	m := f.assertNextManifest("sancho")
	assert.Equal(t, 1, len(m.ImageTargets))
	assert.Equal(t, "gcr.io/some-project-162817/sancho",
		m.ImageTargetAt(0).ConfigurationRef.String())
}

func TestExtraPodSelectors(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExtraPodSelectors("[{'foo': 'bar', 'baz': 'qux'}, {'quux': 'corge'}]")
	f.load()

	f.assertNextManifest("foo",
		extraPodSelectors(labels.Set{"foo": "bar", "baz": "qux"}, labels.Set{"quux": "corge"}))
}

func TestExtraPodSelectorsNotList(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExtraPodSelectors("'hello'")
	f.loadErrString("got starlark.String", "dict or a list")
}

func TestExtraPodSelectorsDict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExtraPodSelectors("{'foo': 'bar'}")
	f.load()
	f.assertNextManifest("foo",
		extraPodSelectors(labels.Set{"foo": "bar"}))
}

func TestExtraPodSelectorsElementNotDict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExtraPodSelectors("['hello']")
	f.loadErrString("must be dicts", "starlark.String")
}

func TestExtraPodSelectorsKeyNotString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExtraPodSelectors("[{54321: 'hello'}]")
	f.loadErrString("keys must be strings", "54321")
}

func TestExtraPodSelectorsValueNotString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExtraPodSelectors("[{'hello': 54321}]")
	f.loadErrString("values must be strings", "54321")
}

func TestDockerBuildMatchingTag(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:stable")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		deployment("foo"),
	)
}

func TestDockerBuildButK8sMissingTag(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
k8s_yaml('foo.yaml')
`)

	w := unusedImageWarning("gcr.io/foo:stable", []string{"gcr.io/foo", "docker.io/library/golang"})
	f.loadAssertWarnings(w)
}

func TestDockerBuildButK8sNonMatchingTag(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo:beta")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo:stable', '.')
k8s_yaml('foo.yaml')
`)

	w := unusedImageWarning("gcr.io/foo:stable", []string{"gcr.io/foo", "docker.io/library/golang"})
	f.loadAssertWarnings(w)
}

func TestFail(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
fail("this is an error")
print("not this")
fail("or this")
`)

	f.loadErrString("this is an error")
}

func TestBlob(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file(
		"Tiltfile",
		fmt.Sprintf(`k8s_yaml(blob('''%s'''))`, testyaml.SnackYaml),
	)

	f.load()

	f.assertNextManifest("snack", deployment("snack"))
}

func TestBlobErr(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file(
		"Tiltfile",
		`blob(42)`,
	)

	f.loadErrString("for parameter 1: got int, want string")
}

func TestImageDependencyV1(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.loadResourceAssemblyV1()
	m := f.assertNextManifest("image-b", deployment("foo"))
	assert.Equal(t, []string{"gcr.io/image-a", "gcr.io/image-b"}, f.imageTargetNames(m))
}

func TestImageDependency(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()
	f.assertNextManifest("foo", deployment("foo", image("gcr.io/image-a"), image("gcr.io/image-b")))
}

func TestImageDependencyLiveUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("message.txt", "Hello!")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", `FROM gcr.io/image-a
ADD message.txt /tmp/message.txt`)
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile',
             live_update=[sync('message.txt', '/tmp/message.txt')])
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()
	m := f.assertNextManifest("foo",
		deployment("foo", image("gcr.io/image-a"), image("gcr.io/image-b")))

	assert.True(t, m.ImageTargetAt(0).AnyLiveUpdateInfo().Empty())
	assert.False(t, m.ImageTargetAt(1).AnyLiveUpdateInfo().Empty())
}

func TestImageDependencyCycle(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM gcr.io/image-b")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.loadErrString("Image dependency cycle: gcr.io/image-b")
}

func TestImageDependencyDiamond(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.file("imageC.dockerfile", "FROM gcr.io/image-a")
	f.file("imageD.dockerfile", `
FROM gcr.io/image-b
FROM gcr.io/image-c
`)
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-d")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-c', '.', dockerfile='imageC.dockerfile')
docker_build('gcr.io/image-d', '.', dockerfile='imageD.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()

	m := f.assertNextManifest("foo", deployment("foo"))
	assert.Equal(t, []string{
		"gcr.io/image-a",
		"gcr.io/image-b",
		"gcr.io/image-c",
		"gcr.io/image-d",
	}, f.imageTargetNames(m))
}

func TestImageDependencyTwice(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", `FROM golang:1.10
COPY --from=gcr.io/image-a /src/package.json /src/package.json
COPY --from=gcr.io/image-a /src/package.lock /src/package.lock
`)
	f.file("snack.yaml", `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: snack
  labels:
    app: snack
spec:
  selector:
    matchLabels:
      app: snack
  template:
    metadata:
      labels:
        app: snack
    spec:
      containers:
      - name: snack1
        image: gcr.io/image-b
        command: ["/go/bin/snack"]
      - name: snack2
        image: gcr.io/image-b
        command: ["/go/bin/snack"]
`)
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
k8s_yaml('snack.yaml')
`)

	f.load()

	m := f.assertNextManifest("snack")
	assert.Equal(t, []string{
		"gcr.io/image-a",
		"gcr.io/image-b",
	}, f.imageTargetNames(m))
	assert.Equal(t, []string{
		"gcr.io/image-a",
		"gcr.io/image-b",
		"snack", // the deploy name
	}, f.idNames(m.DependencyIDs()))
	assert.Equal(t, []string{}, f.idNames(m.ImageTargets[0].DependencyIDs()))
	assert.Equal(t, []string{"gcr.io/image-a"}, f.idNames(m.ImageTargets[1].DependencyIDs()))
	assert.Equal(t, []string{"gcr.io/image-b"}, f.idNames(m.DeployTarget().DependencyIDs()))
}

func TestImageDependencyNormalization(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("common.dockerfile", "FROM golang:1.10")
	f.file("auth.dockerfile", "FROM vandelay/common")
	f.yaml("auth.yaml", deployment("auth", image("vandelay/auth")))
	f.file("Tiltfile", `
docker_build('vandelay/common', '.', dockerfile='common.dockerfile')
docker_build('vandelay/auth', '.', dockerfile='auth.dockerfile')
k8s_yaml('auth.yaml')
`)

	f.load()

	m := f.assertNextManifest("auth", deployment("auth"))
	assert.Equal(t, []string{
		"docker.io/vandelay/common",
		"docker.io/vandelay/auth",
	}, f.imageTargetNames(m))
}

func TestImagesWithSameNameAssemblyV1(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("app.dockerfile", "FROM golang:1.10")
	f.file("app-jessie.dockerfile", "FROM golang:1.10-jessie")
	f.yaml("app.yaml",
		deployment("app", image("vandelay/app")),
		deployment("app-jessie", image("vandelay/app:jessie")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
docker_build('vandelay/app', '.', dockerfile='app.dockerfile')
docker_build('vandelay/app:jessie', '.', dockerfile='app-jessie.dockerfile')
k8s_yaml('app.yaml')
`)

	f.loadResourceAssemblyV1()

	m := f.assertNextManifest("app", deployment("app"), deployment("app-jessie"))
	assert.Equal(t, []string{
		"docker.io/vandelay/app",
		"docker.io/vandelay/app:jessie",
	}, f.imageTargetNames(m))
}

func TestImagesWithSameNameAssembly(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("app.dockerfile", "FROM golang:1.10")
	f.file("app-jessie.dockerfile", "FROM golang:1.10-jessie")
	f.yaml("app.yaml",
		deployment("app", image("vandelay/app")),
		deployment("app-jessie", image("vandelay/app:jessie")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('vandelay/app', '.', dockerfile='app.dockerfile')
docker_build('vandelay/app:jessie', '.', dockerfile='app-jessie.dockerfile')
k8s_yaml('app.yaml')
`)

	f.load()

	f.assertNextManifest("app", deployment("app", image("docker.io/vandelay/app")))
	f.assertNextManifest("app-jessie", deployment("app-jessie", image("docker.io/vandelay/app:jessie")))
}

func TestImagesWithSameNameDifferentManifests(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("app.dockerfile", "FROM golang:1.10")
	f.file("app-jessie.dockerfile", "FROM golang:1.10-jessie")
	f.yaml("app.yaml",
		deployment("app", image("vandelay/app")),
		deployment("app-jessie", image("vandelay/app:jessie")))
	f.file("Tiltfile", `
k8s_resource_assembly_version(1)
docker_build('vandelay/app', '.', dockerfile='app.dockerfile')
docker_build('vandelay/app:jessie', '.', dockerfile='app-jessie.dockerfile')
k8s_yaml('app.yaml')
k8s_resource('jessie', image='vandelay/app:jessie')
`)

	f.loadResourceAssemblyV1()

	m := f.assertNextManifest("jessie", deployment("app-jessie"))
	assert.Equal(t, []string{
		"docker.io/vandelay/app:jessie",
	}, f.imageTargetNames(m))

	m = f.assertNextManifest("app", deployment("app"))
	assert.Equal(t, []string{
		"docker.io/vandelay/app",
	}, f.imageTargetNames(m))
}

func TestImageRefSuggestion(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.typo.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)
	w := unusedImageWarning("gcr.typo.io/foo", []string{"gcr.io/foo", "docker.io/library/golang"})
	f.loadAssertWarnings(w)
}

func TestDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.yaml("config/foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.yaml("config/bar.yaml", deployment("bar", image("gcr.io/bar")))
	f.file("Tiltfile", `k8s_yaml(listdir('config'))`)

	f.load("foo", "bar")
	f.assertNumManifests(2)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "config/foo.yaml", "config/bar.yaml", "config")
}

func TestDirRecursive(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("foo/bar", "bar")
	f.file("foo/baz/qux", "qux")
	f.file("Tiltfile", `files = listdir('foo', recursive=True)

for f in files:
  read_file(f)
`)

	f.load()
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo", "foo/bar", "foo/baz/qux")
}

func TestCallCounts(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml",
		deployment("foo", image("gcr.io/foo")),
		deployment("bar", image("gcr.io/bar")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
docker_build('gcr.io/bar', '.')
k8s_yaml('foo.yaml')
`)

	f.load()
	expected := []analytics.CountEvent{{
		Name: "tiltfile.loaded",
		Tags: map[string]string{
			"tiltfile.invoked.docker_build": "2",
			"tiltfile.invoked.k8s_yaml":     "1",
		},
		N: 1,
	}}
	assert.Equal(t, expected, f.an.Counts)
}

func TestYamlErrorFromLocal(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `
yaml = local('echo hi')
k8s_yaml(yaml)
`)
	f.loadErrString("cmd: 'echo hi'")
}

func TestYamlErrorFromReadFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("foo.yaml", "hi")
	f.file("Tiltfile", `
k8s_yaml(read_file('foo.yaml'))
`)
	f.loadErrString(fmt.Sprintf("file: %s", f.JoinPath("foo.yaml")))
}

func TestYamlErrorFromHelm(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupHelm()
	f.file("helm/templates/foo.yaml", "hi")
	f.file("Tiltfile", `
k8s_yaml(helm('helm'))
`)
	f.loadErrString("from helm")
}

func TestYamlErrorFromBlob(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `
k8s_yaml(blob('hi'))
`)
	f.loadErrString("from Tiltfile blob() call")
}

func TestCustomBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	tiltfile := `k8s_yaml('foo.yaml')
hfb = custom_build(
  'gcr.io/foo',
  'docker build -t $EXPECTED_REF foo',
  ['foo']
).add_fast_build()
hfb.add('foo', '/app')
hfb.run('cd /app && pip install -r requirements.txt')
hfb.hot_reload()`

	f.setupFoo()
	f.file("Tiltfile", tiltfile)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/.dockerignore")
	f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
			deps(f.JoinPath("foo")),
			cmd("docker build -t $EXPECTED_REF foo"),
			disablePush(false),
			fb(
				image("gcr.io/foo"),
				add("foo", "/app"),
				run("cd /app && pip install -r requirements.txt"),
			),
		),
		deployment("foo"))
}

func TestCustomBuildWithTag(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	tiltfile := `k8s_yaml('foo.yaml')
custom_build(
  'gcr.io/foo',
  'docker build -t gcr.io/foo:my-great-tag foo',
  ['foo'],
  tag='my-great-tag'
)`

	f.setupFoo()
	f.file("Tiltfile", tiltfile)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml")
	f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
			deps(f.JoinPath("foo")),
			cmd("docker build -t gcr.io/foo:my-great-tag foo"),
			tag("my-great-tag"),
		),
		deployment("foo"))
}

func TestCustomBuildDisablePush(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	tiltfile := `k8s_yaml('foo.yaml')
hfb = custom_build(
  'gcr.io/foo',
  'docker build -t $TAG foo',
	['foo'],
	disable_push=True,
).add_fast_build()`

	f.setupFoo()
	f.file("Tiltfile", tiltfile)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml")
	f.assertNextManifest("foo",
		cb(
			image("gcr.io/foo"),
			deps(f.JoinPath("foo")),
			cmd("docker build -t $TAG foo"),
			disablePush(true),
		),
		deployment("foo"))
}

func TestExtraImageLocationOneImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('crd.yaml')
k8s_image_json_path(kind='Environment', path='{.spec.runtime.image}')
docker_build('test/mycrd-env', 'env')
`)

	f.load("mycrd")
	f.assertNextManifest("mycrd",
		db(
			image("docker.io/test/mycrd-env"),
		),
		k8sObject("mycrd", "Environment"),
	)
}

func TestConflictingWorkloadNames(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo1/Dockerfile")
	f.dockerfile("foo2/Dockerfile")
	f.yaml("foo1.yaml", deployment("foo", image("gcr.io/foo1"), namespace("ns1")))
	f.yaml("foo2.yaml", deployment("foo", image("gcr.io/foo2"), namespace("ns2")))

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml(['foo1.yaml', 'foo2.yaml'])
docker_build('gcr.io/foo1', 'foo1')
docker_build('gcr.io/foo2', 'foo2')
`)
	f.load("foo:deployment:ns1", "foo:deployment:ns2")

	f.assertNextManifest("foo:deployment:ns1", db(image("gcr.io/foo1")))
	f.assertNextManifest("foo:deployment:ns2", db(image("gcr.io/foo2")))
}

type k8sKindTest struct {
	name                 string
	args                 string
	expectMatch          bool
	expectedError        string
	preamble             string
	expectedResourceName string
}

func TestK8SKind(t *testing.T) {
	tests := []k8sKindTest{
		{name: "match kind", args: "'Environment', image_json_path='{.spec.runtime.image}'", expectMatch: true},
		{name: "don't match kind", args: "'fdas', image_json_path='{.spec.runtime.image}'", expectMatch: false},
		{name: "match apiVersion", args: "'Environment', image_json_path='{.spec.runtime.image}', api_version='fission.io/v1'", expectMatch: true},
		{name: "don't match apiVersion", args: "'Environment', image_json_path='{.spec.runtime.image}', api_version='fission.io/v2'"},
		{name: "invalid kind regexp", args: "'*', image_json_path='{.spec.runtime.image}'", expectedError: "error parsing kind regexp"},
		{name: "invalid apiVersion regexp", args: "'Environment', api_version='*', image_json_path='{.spec.runtime.image}'", expectedError: "error parsing apiVersion regexp"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()
			f.setupCRD()
			f.dockerfile("env/Dockerfile")
			f.dockerfile("builder/Dockerfile")
			f.file("Tiltfile", fmt.Sprintf(`
k8s_resource_assembly_version(2)
%s
k8s_yaml('crd.yaml')
k8s_kind(%s)
docker_build('test/mycrd-env', 'env')
`, test.preamble, test.args))

			if test.expectMatch {
				if test.expectedError != "" {
					t.Fatal("invalid test: cannot expect both match and error")
				}
				expectedResourceName := "mycrd"
				if test.expectedResourceName != "" {
					expectedResourceName = test.expectedResourceName
				}
				f.load(expectedResourceName)
				f.assertNextManifest(expectedResourceName,
					db(
						image("docker.io/test/mycrd-env"),
					),
					k8sObject("mycrd", "Environment"),
				)
			} else {
				if test.expectedError == "" {
					w := unusedImageWarning("docker.io/test/mycrd-env", []string{"docker.io/library/golang"})
					f.loadAssertWarnings(w)
				} else {
					f.loadErrString(test.expectedError)
				}
			}
		})
	}
}

func TestK8SKindImageJSONPathPositional(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `k8s_yaml('crd.yaml')
k8s_kind('Environment', '{.spec.runtime.image}')
docker_build('test/mycrd-env', 'env')
`)

	f.loadErrString("got 2 arguments, want at most 1")
}

func TestExtraImageLocationTwoImages(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('crd.yaml')
k8s_image_json_path(['{.spec.runtime.image}', '{.spec.builder.image}'], kind='Environment')
docker_build('test/mycrd-builder', 'builder')
docker_build('test/mycrd-env', 'env')
`)

	f.load("mycrd")
	f.assertNextManifest("mycrd",
		db(
			image("docker.io/test/mycrd-env"),
		),
		db(
			image("docker.io/test/mycrd-builder"),
		),
		k8sObject("mycrd", "Environment"),
	)
}

func TestExtraImageLocationDeploymentEnvVarByName(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.dockerfile("foo-fetcher/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
	f.dockerfile("bar/Dockerfile")
	// just throwing bar in here to make sure it doesn't error out because it has no FETCHER_IMAGE
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/bar")))
	f.gitInit("")

	f.file("Tiltfile", `k8s_yaml(['foo.yaml', 'bar.yaml'])
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
docker_build('gcr.io/bar', 'bar')
k8s_image_json_path("{.spec.template.spec.containers[*].env[?(@.name=='FETCHER_IMAGE')].value}", name='foo')
	`)
	f.load("foo", "bar")
	f.assertNextManifest("foo",
		db(
			image("gcr.io/foo"),
		),
		db(
			image("gcr.io/foo-fetcher"),
		),
	)
	f.assertNextManifest("bar",
		db(
			image("gcr.io/bar"),
		),
	)
}

func TestK8SImageJSONPathArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          string
		expectMatch   bool
		expectedError string
	}{
		{"match name", "name='foo'", true, ""},
		{"don't match name", "name='bar'", false, ""},
		{"match name w/ regex", "name='.*o'", true, ""},
		{"match kind", "name='foo', kind='Deployment'", true, ""},
		{"don't match kind", "name='bar', kind='asdf'", false, ""},
		{"match apiVersion", "name='foo', api_version='apps/v1'", true, ""},
		{"match apiVersion+kind w/ regex", "name='foo', kind='Deployment', api_version='apps/.*'", true, ""},
		{"don't match apiVersion", "name='bar', api_version='apps/v2'", false, ""},
		{"match namespace", "name='foo', namespace='default'", true, ""},
		{"don't match namespace", "name='bar', namespace='asdf'", false, ""},
		{"invalid name regex", "name='*'", false, "error parsing name regexp"},
		{"invalid kind regex", "kind='*'", false, "error parsing kind regexp"},
		{"invalid apiVersion regex", "name='foo', api_version='*'", false, "error parsing apiVersion regexp"},
		{"invalid namespace regex", "namespace='*'", false, "error parsing namespace regexp"},
		{"regexes are case-insensitive", "name='FOO'", true, ""},
		{"regexes that specify case insensitivity still work", "name='(?i)FOO'", true, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			f.dockerfile("foo/Dockerfile")
			f.dockerfile("foo-fetcher/Dockerfile")
			f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
			f.gitInit("")

			f.file("Tiltfile", fmt.Sprintf(`k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
k8s_image_json_path("{.spec.template.spec.containers[*].env[?(@.name=='FETCHER_IMAGE')].value}", %s)
	`, test.args))
			if test.expectMatch {
				if test.expectedError != "" {
					t.Fatal("illegal test definition: cannot expect both match and error")
				}
				f.load("foo")
				f.assertNextManifest("foo",
					db(
						image("gcr.io/foo"),
					),
					db(
						image("gcr.io/foo-fetcher"),
					),
				)
			} else {
				if test.expectedError == "" {
					w := unusedImageWarning("gcr.io/foo-fetcher", []string{"gcr.io/foo", "docker.io/library/golang"})
					f.loadAssertWarnings(w)
				} else {
					f.loadErrString(test.expectedError)
				}
			}
		})
	}
}

func TestExtraImageLocationDeploymentEnvVarByNameAndNamespace(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.dockerfile("foo-fetcher/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo"), withEnvVars("FETCHER_IMAGE", "gcr.io/foo-fetcher")))
	f.gitInit("")

	f.file("Tiltfile", `k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/foo-fetcher', 'foo-fetcher')
k8s_image_json_path("{.spec.template.spec.containers[*].env[?(@.name=='FETCHER_IMAGE')].value}", name='foo', namespace='default')
	`)
	f.load("foo")
	f.assertNextManifest("foo",
		db(
			image("gcr.io/foo"),
		),
		db(
			image("gcr.io/foo-fetcher"),
		),
	)
}

func TestExtraImageLocationNoMatch(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `k8s_yaml('crd.yaml')
k8s_image_json_path('{.foobar}', kind='Environment')
docker_build('test/mycrd-env', 'env')
`)

	f.loadErrString("{.foobar}", "foobar is not found")
}

func TestExtraImageLocationInvalidJsonPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupCRD()
	f.dockerfile("env/Dockerfile")
	f.dockerfile("builder/Dockerfile")
	f.file("Tiltfile", `k8s_yaml('crd.yaml')
k8s_image_json_path('{foobar()}', kind='Environment')
docker_build('test/mycrd-env', 'env')
`)

	f.loadErrString("{foobar()}", "unrecognized identifier foobar()")
}

func TestExtraImageLocationNoPaths(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `k8s_image_json_path(kind='MyType')`)
	f.loadErrString("missing argument for path")
}

func TestExtraImageLocationNotListOrString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `k8s_image_json_path(kind='MyType', path=8)`)
	f.loadErrString("path must be a string or list of strings", "Int")
}

func TestExtraImageLocationListContainsNonString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `k8s_image_json_path(kind='MyType', path=["foo", 8])`)
	f.loadErrString("path must be a string or list of strings", "8", "Int")
}

func TestExtraImageLocationNoSelectorSpecified(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `k8s_image_json_path(path=["foo"])`)
	f.loadErrString("at least one of kind, name, or namespace must be specified")
}

func TestFastBuildEmptyDockerfileArg(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.gitInit(f.Path())
	f.file("Tiltfile", `
repo = local_git_repo('.')

(fast_build('web/api', '')
    .add(repo.path('src'), '/app/src').run(''))
`)
	f.loadErrString("error reading dockerfile")
}

func TestDockerBuildEmptyDockerFileArg(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `
docker_build('web/api', '', dockerfile='')
`)
	f.loadErrString("error reading dockerfile")
}

func TestK8SYamlEmptyArg(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("Tiltfile", `
k8s_yaml('')
`)
	f.loadErrString("error reading yaml file")
}

func TestParseJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
result = decode_json('["foo", {"baz":["bar", "", 1, 2]}]')

docker_build('gcr.io/foo', 'foo')
k8s_yaml(result[0] + '.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml(result[1]["baz"][0] + '.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertNextManifest("bar",
		db(image("gcr.io/bar")),
		deployment("bar"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml")
}

func TestReadYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	var document = `
key1: foo
key2:
    key3: "bar"
    key4: true
key5: 3
`
	f.file("options.yaml", document)

	f.file("Tiltfile", `
result = read_yaml("options.yaml")

docker_build('gcr.io/foo', 'foo')
k8s_yaml(result['key1'] + '.yaml')
docker_build('gcr.io/bar', 'bar')

if result['key2']['key4'] and result['key5'] == 3:
		k8s_yaml(result['key2']['key3'] + '.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertNextManifest("bar",
		db(image("gcr.io/bar")),
		deployment("bar"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml", "options.yaml")
}

func TestYAMLDoesntExist(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
result = read_yaml("dne.yaml")

docker_build('gcr.io/foo', 'foo')
k8s_yaml(result['key1'] + '.yaml')
docker_build('gcr.io/bar', 'bar')

if result['key2']['key4'] and result['key5'] == 3:
		k8s_yaml(result['key2']['key3'] + '.yaml')
`)
	f.loadErrString("dne.yaml: no such file or directory")

	f.assertConfigFiles("Tiltfile", ".tiltignore", "dne.yaml")
}

func TestMalformedYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	var document = `
key1: foo
key2:
    key3: "bar
    key4: true
key5: 3
`
	f.file("options.yaml", document)

	f.file("Tiltfile", `
result = read_yaml("options.yaml")

docker_build('gcr.io/foo', 'foo')
k8s_yaml(result['key1'] + '.yaml')
docker_build('gcr.io/bar', 'bar')

if result['key2']['key4'] and result['key5'] == 3:
		k8s_yaml(result['key2']['key3'] + '.yaml')
`)
	f.loadErrString("error parsing YAML: error converting YAML to JSON: yaml: line 7: found unexpected end of stream in options.yaml")
}

func TestReadJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("options.json", `["foo", {"baz":["bar", "", 1, 2]}]`)
	f.file("Tiltfile", `
result = read_json("options.json")

docker_build('gcr.io/foo', 'foo')
k8s_yaml(result[0] + '.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_yaml(result[1]["baz"][0] + '.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertNextManifest("bar",
		db(image("gcr.io/bar")),
		deployment("bar"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml", "options.json")
}

func TestJSONDoesntExist(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
result = read_json("dne.json")

docker_build('gcr.io/foo', 'foo')
k8s_resource(result[0], 'foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_resource(result[1]["baz"][0], 'bar.yaml')
`)
	f.loadErrString("dne.json: no such file or directory")

	f.assertConfigFiles("Tiltfile", ".tiltignore", "dne.json")
}

func TestMalformedJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("options.json", `["foo", {"baz":["bar", "", 1, 2]}`)
	f.file("Tiltfile", `
result = read_json("options.json")

docker_build('gcr.io/foo', 'foo')
k8s_resource(result[0], 'foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_resource(result[1]["baz"][0], 'bar.yaml')
`)
	f.loadErrString("JSON parsing error: unexpected end of JSON input")
}

func TestTwoDefaultRegistries(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
default_registry("foo")
default_registry("bar")`)

	f.loadErrString("default registry already defined")
}

func TestDefaultRegistryInvalid(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
default_registry("foo")
docker_build('gcr.io/foo', 'foo')
`)

	f.loadErrString("repository name must be canonical")
}

func TestDefaultRegistryAtEndOfTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	// default_registry is the last entry to test that it doesn't only affect subsequently defined images
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
default_registry('bar.com')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo").withInjectedRef("bar.com/gcr.io_foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml")
}

func TestDefaultRegistryTwoImagesOnlyDifferByTag(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("bar/Dockerfile")
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/foo:bar")))

	f.dockerfile("baz/Dockerfile")
	f.yaml("baz.yaml", deployment("baz", image("gcr.io/foo:baz")))

	f.gitInit("")
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo:bar', 'bar')
docker_build('gcr.io/foo:baz', 'baz')
k8s_yaml('bar.yaml')
k8s_yaml('baz.yaml')
default_registry('example.com')
`)

	f.load()

	f.assertNextManifest("bar",
		db(image("gcr.io/foo:bar").withInjectedRef("example.com/gcr.io_foo")),
		deployment("bar"))
	f.assertNextManifest("baz",
		db(image("gcr.io/foo:baz").withInjectedRef("example.com/gcr.io_foo")),
		deployment("baz"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "bar/Dockerfile", "bar/.dockerignore", "bar.yaml", "baz/Dockerfile", "baz/.dockerignore", "baz.yaml")
}

func TestDefaultRegistryWithDockerCompose(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `
docker_compose('docker-compose.yml')
default_registry('bar.com')
`)

	f.loadErrString("default_registry is not supported with docker compose")
}

func TestDefaultReadFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFooAndBar()
	tiltfile := `
result = read_file("this_file_does_not_exist", default="foo")
docker_build('gcr.io/foo', 'foo')
k8s_yaml(str(result) + '.yaml')
`

	f.file("Tiltfile", tiltfile)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "this_file_does_not_exist", "foo.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestDefaultReadJSON(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	tiltfile := `
result = read_json("this_file_does_not_exist", default={"name": "foo"})
docker_build('gcr.io/foo', 'foo')
k8s_yaml(str(result["name"]) + '.yaml')
`

	f.file("Tiltfile", tiltfile)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "this_file_does_not_exist", "foo.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestWatchFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("hello", "world")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
watch_file('hello')
k8s_yaml('foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo/Dockerfile", "foo/.dockerignore", "foo.yaml", "hello")
}

func TestK8SResourceAssemblyVersionAfterYAML(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource('foo', 'bar')
k8s_resource_assembly_version(2)
`)

	f.loadErrString("k8s_resource_assembly_version can only be called before introducing any k8s resources")
}

func TestK8SResourceAssemblyK8SResourceYAMLNamedArg(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource('foo', yaml='bar')
`)

	f.loadErrString("kwarg \"yaml\"", "deprecated", "https://docs.tilt.dev/resource_assembly_migration.html")
}

func TestK8SResourceAssemblyK8SResourceImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource('foo', image='bar')
`)

	f.loadErrString("kwarg \"image\"", "deprecated", "https://docs.tilt.dev/resource_assembly_migration.html")
}

func TestK8SResourceAssemblyK8SResourceYAMLPositionalArg(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource('foo', 'foo.yaml')
`)

	f.loadErrString("second arg", "looks like a yaml file name", "deprecated", "https://docs.tilt.dev/resource_assembly_migration.html")
}

func TestK8SResourceAssemblyK8SResourceNameKeywordArg(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource('foo', name='bar')
`)

	f.loadErrString("kwarg \"name\"", "deprecated", "https://docs.tilt.dev/resource_assembly_migration.html")
}

func TestAssemblyVersion2Basic(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
`)

	f.load("foo")

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestAssemblyVersion2TwoWorkloadsSameImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/foo")))

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml(['foo.yaml', 'bar.yaml'])
`)

	f.load("foo", "bar")

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertNextManifest("bar",
		db(image("gcr.io/foo")),
		deployment("bar"))

	f.assertConfigFiles("Tiltfile", ".tiltignore", "foo.yaml", "bar.yaml", "foo/Dockerfile", "foo/.dockerignore")
}

func TestK8SResourceNoMatch(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('foo.yaml')
k8s_resource('bar', new_name='baz')
`)

	f.loadErrString("specified unknown resource 'bar'. known resources: foo", "https://docs.tilt.dev/resource_assembly_migration.html")
}

func TestK8SResourceNewName(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('foo.yaml')
k8s_resource('foo', new_name='bar')
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("bar", deployment("foo"))
}

func TestK8SResourceNewNameConflict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml(['foo.yaml', 'bar.yaml'])
k8s_resource('foo', new_name='bar')
`)

	f.loadErrString("'foo' to 'bar'", "already a resource with that name")
}

func TestK8SResourceRenameConflictingNames(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo1/Dockerfile")
	f.dockerfile("foo2/Dockerfile")
	f.yaml("foo1.yaml", deployment("foo", image("gcr.io/foo1"), namespace("ns1")))
	f.yaml("foo2.yaml", deployment("foo", image("gcr.io/foo2"), namespace("ns2")))

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml(['foo1.yaml', 'foo2.yaml'])
docker_build('gcr.io/foo1', 'foo1')
docker_build('gcr.io/foo2', 'foo2')
k8s_resource('foo:deployment:ns2', new_name='foo')
`)
	f.load("foo:deployment:ns1", "foo")

	f.assertNextManifest("foo:deployment:ns1", db(image("gcr.io/foo1")))
	f.assertNextManifest("foo", db(image("gcr.io/foo2")))
}

func TestMultipleK8SResourceOptionsForOneResource(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('foo.yaml')
k8s_resource('foo', port_forwards=8001)
k8s_resource('foo', port_forwards=8000)
`)
	f.loadErrString("k8s_resource already called for foo")
}

func TestK8SResourceEmptyWorkloadSpecifier(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
k8s_yaml('foo.yaml')
k8s_resource('', port_forwards=8000)
`)

	f.loadErrString("workload must not be empty")
}

func TestWorkloadToResourceFunction(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(id):
	return 'hello-' + id.name
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("hello-foo", db(image("gcr.io/foo")), []model.PortForward{{LocalPort: 8000}})
}

func TestWorkloadToResourceFunctionConflict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
docker_build('gcr.io/bar', 'bar')
k8s_yaml(['foo.yaml', 'bar.yaml'])
def wtrf(id):
	return 'baz'
workload_to_resource_function(wtrf)
`)

	f.loadErrString("workload_to_resource_function", "bar:deployment:default:apps", "foo:deployment:default:apps", "'baz'")
}

func TestWorkloadToResourceFunctionError(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(id):
	return 1 + 'asdf'
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("'foo:deployment:default:apps'", "unknown binary op: int + string", "Tiltfile:5:1", workloadToResourceFunctionN)
}

func TestWorkloadToResourceFunctionReturnsNonString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(id):
	return 1
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("'foo:deployment:default:apps'", "invalid return value", "wanted: string. got: starlark.Int", "Tiltfile:5:1", workloadToResourceFunctionN)
}

func TestWorkloadToResourceFunctionTakesNoArgs(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf():
	return "hello"
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("workload_to_resource_function arg must take 1 argument. wtrf takes 0")
}

func TestWorkloadToResourceFunctionTakesTwoArgs(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
def wtrf(a, b):
	return "hello"
workload_to_resource_function(wtrf)
k8s_resource('hello-foo', port_forwards=8000)
`)

	f.loadErrString("workload_to_resource_function arg must take 1 argument. wtrf takes 2")
}

func TestImpossibleLiveUpdates(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("sancho/Dockerfile", "FROM golang:1.10")
	f.file("sidecar/Dockerfile", "FROM golang:1.10")
	f.file("sancho.yaml", testyaml.SanchoSidecarYAML) // two containers
	f.file("Tiltfile", `
k8s_yaml('sancho.yaml')
docker_build('gcr.io/some-project-162817/sancho', './sancho')
docker_build('gcr.io/some-project-162817/sancho-sidecar', './sidecar',
  live_update=[restart_container()]
)
`)

	f.loadAssertWarnings("Sorry, but Tilt only supports in-place updates for the first Tilt-built container on a pod, so we can't in-place update your image 'gcr.io/some-project-162817/sancho-sidecar'. If this is a feature you need, let us know!")
}

func TestImpossibleLiveUpdatesOKNoLiveUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("sancho/Dockerfile", "FROM golang:1.10")
	f.file("sidecar/Dockerfile", "FROM golang:1.10")
	f.file("sancho.yaml", testyaml.SanchoSidecarYAML) // two containers
	f.file("Tiltfile", `
k8s_yaml('sancho.yaml')
docker_build('gcr.io/some-project-162817/sancho', './sancho')

# no LiveUpdate on this so nothing meriting a warning
docker_build('gcr.io/some-project-162817/sancho-sidecar', './sidecar')
`)

	// Expect no warnings!
	f.load()
}

func TestImpossibleLiveUpdatesOKSecondContainerLiveUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("sancho/Dockerfile", "FROM golang:1.10")
	f.file("sidecar/Dockerfile", "FROM golang:1.10")
	f.file("sancho.yaml", testyaml.SanchoSidecarYAML) // two containers
	f.file("Tiltfile", `
k8s_yaml('sancho.yaml')

# this is the second k8s container, but only the first image target, so should be OK
docker_build('gcr.io/some-project-162817/sancho-sidecar', './sidecar')
`)

	// Expect no warnings!
	f.load()
}

func TestUpdateModeK8S(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		globalSetting      updateMode
		k8sResourceSetting updateMode
		expectedUpdateMode model.UpdateMode
	}{
		{"default", UpdateModeUnset, UpdateModeUnset, model.UpdateModeAuto},
		{"explicit global auto", UpdateModeAuto, UpdateModeUnset, model.UpdateModeAuto},
		{"explicit global manual", UpdateModeManual, UpdateModeUnset, model.UpdateModeManual},
		{"kr auto", UpdateModeUnset, UpdateModeUnset, model.UpdateModeAuto},
		{"kr manual", UpdateModeUnset, UpdateModeManual, model.UpdateModeManual},
		{"kr override auto", UpdateModeManual, UpdateModeAuto, model.UpdateModeAuto},
		{"kr override manual", UpdateModeAuto, UpdateModeManual, model.UpdateModeManual},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			f.setupFoo()

			var globalUpdateModeDirective string
			switch testCase.globalSetting {
			case UpdateModeUnset:
				globalUpdateModeDirective = ""
			case UpdateModeManual:
				globalUpdateModeDirective = "update_mode(UPDATE_MODE_MANUAL)"
			case UpdateModeAuto:
				globalUpdateModeDirective = "update_mode(UPDATE_MODE_AUTO)"
			}

			var k8sResourceDirective string
			switch testCase.k8sResourceSetting {
			case UpdateModeUnset:
				k8sResourceDirective = ""
			case UpdateModeManual:
				k8sResourceDirective = "k8s_resource('foo', update_mode=UPDATE_MODE_MANUAL)"
			case UpdateModeAuto:
				k8sResourceDirective = "k8s_resource('foo', update_mode=UPDATE_MODE_AUTO)"
			}

			f.file("Tiltfile", fmt.Sprintf(`
%s
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
%s
`, globalUpdateModeDirective, k8sResourceDirective))

			f.load()

			f.assertNumManifests(1)
			f.assertNextManifest("foo", testCase.expectedUpdateMode)
		})
	}
}

func TestUpdateModeDC(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		globalSetting      updateMode
		dcResourceSetting  updateMode
		expectedUpdateMode model.UpdateMode
	}{
		{"default", UpdateModeUnset, UpdateModeUnset, model.UpdateModeAuto},
		{"explicit global auto", UpdateModeAuto, UpdateModeUnset, model.UpdateModeAuto},
		{"explicit global manual", UpdateModeManual, UpdateModeUnset, model.UpdateModeManual},
		{"dc auto", UpdateModeUnset, UpdateModeUnset, model.UpdateModeAuto},
		{"dc manual", UpdateModeUnset, UpdateModeManual, model.UpdateModeManual},
		{"dc override auto", UpdateModeManual, UpdateModeAuto, model.UpdateModeAuto},
		{"dc override manual", UpdateModeAuto, UpdateModeManual, model.UpdateModeManual},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			f.dockerfile("foo/Dockerfile")
			f.file("docker-compose.yml", simpleConfig)

			var globalUpdateModeDirective string
			switch testCase.globalSetting {
			case UpdateModeUnset:
				globalUpdateModeDirective = ""
			case UpdateModeManual:
				globalUpdateModeDirective = "update_mode(UPDATE_MODE_MANUAL)"
			case UpdateModeAuto:
				globalUpdateModeDirective = "update_mode(UPDATE_MODE_AUTO)"
			}

			var dcResourceDirective string
			switch testCase.dcResourceSetting {
			case UpdateModeUnset:
				dcResourceDirective = ""
			case UpdateModeManual:
				dcResourceDirective = "dc_resource('foo', 'gcr.io/foo', update_mode=UPDATE_MODE_MANUAL)"
			case UpdateModeAuto:
				dcResourceDirective = "dc_resource('foo', 'gcr.io/foo', update_mode=UPDATE_MODE_AUTO)"
			}

			f.file("Tiltfile", fmt.Sprintf(`
%s
docker_compose('docker-compose.yml')
%s
`, globalUpdateModeDirective, dcResourceDirective))

			f.load()

			f.assertNumManifests(1)
			f.assertNextManifest("foo", testCase.expectedUpdateMode)
		})
	}
}

func TestUpdateModeInt(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
update_mode(1)
`)
	f.loadErrString("got int, want UpdateMode")
}

func TestMultipleUpdateMode(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
update_mode(UPDATE_MODE_MANUAL)
update_mode(UPDATE_MODE_MANUAL)
`)
	f.loadErrString("update_mode can only be called once")
}

func TestHelmSkipsTests(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupHelmWithTest()
	f.file("Tiltfile", `
yml = helm('helm')
k8s_yaml(yml)
`)

	f.load()

	f.assertNextManifestUnresourced("release-name-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		".tiltignore",
		"helm",
	)
}

type fixture struct {
	ctx context.Context
	t   *testing.T
	*tempdir.TempDirFixture

	tfl TiltfileLoader
	an  *analytics.MemoryAnalytics

	loadResult TiltfileLoadResult
}

func newFixture(t *testing.T) *fixture {
	out := new(bytes.Buffer)
	ctx := output.ForkedCtxForTest(out)
	f := tempdir.NewTempDirFixture(t)
	an := analytics.NewMemoryAnalytics()
	dcc := dockercompose.NewDockerComposeClient(docker.Env{})
	tfl := ProvideTiltfileLoader(an, dcc)

	r := &fixture{
		ctx:            ctx,
		t:              t,
		TempDirFixture: f,
		an:             an,
		tfl:            tfl,
	}
	return r
}

func (f *fixture) file(path string, contents string) {
	f.WriteFile(path, contents)
}

type k8sOpts interface{}

func (f *fixture) dockerfile(path string) {
	f.file(path, simpleDockerfile)
}

func (f *fixture) yaml(path string, entities ...k8sOpts) {
	var entityObjs []k8s.K8sEntity

	for _, e := range entities {
		switch e := e.(type) {
		case deploymentHelper:
			s := testyaml.SnackYaml
			if e.image != "" {
				s = strings.Replace(s, testyaml.SnackImage, e.image, -1)
			}
			s = strings.Replace(s, testyaml.SnackName, e.name, -1)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}

			if len(e.templateLabels) > 0 {
				for i, obj := range objs {
					withLabels, err := k8s.OverwriteLabels(obj, model.ToLabelPairs(e.templateLabels))
					if err != nil {
						f.t.Fatal(err)
					}
					objs[i] = withLabels
				}
			}

			for i, obj := range objs {
				de := obj.Obj.(*appsv1.Deployment)
				for i, c := range de.Spec.Template.Spec.Containers {
					for _, ev := range e.envVars {
						c.Env = append(c.Env, v1.EnvVar{
							Name:  ev.name,
							Value: ev.value,
						})
					}
					de.Spec.Template.Spec.Containers[i] = c
				}
				if e.namespace != "" {
					de.Namespace = e.namespace
				}
				obj.Obj = de
				objs[i] = obj
			}

			entityObjs = append(entityObjs, objs...)
		case serviceHelper:
			s := testyaml.DoggosServiceYaml
			s = strings.Replace(s, testyaml.DoggosName, e.name, -1)
			objs, err := k8s.ParseYAMLFromString(s)
			if err != nil {
				f.t.Fatal(err)
			}

			if len(e.selectorLabels) > 0 {
				for _, obj := range objs {
					err := overwriteSelectorsForService(&obj, e.selectorLabels)
					if err != nil {
						f.t.Fatal(err)
					}
				}
			}

			entityObjs = append(entityObjs, objs...)

		case secretHelper:
			s := testyaml.SecretYaml
			s = strings.Replace(s, testyaml.SecretName, e.name, -1)
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

func matchMap(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// Default load. Fails if there are any warnings.
func (f *fixture) load(names ...string) {
	f.loadAllowWarnings(names...)
	if len(f.loadResult.Warnings) != 0 {
		f.t.Fatalf("Unexpected no warnings. Actual: %s", f.loadResult.Warnings)
	}
}

func (f *fixture) loadResourceAssemblyV1(names ...string) {
	tlr, err := f.tfl.Load(f.ctx, f.JoinPath("Tiltfile"), matchMap(names...))
	if err != nil {
		f.t.Fatal(err)
	}
	f.loadResult = tlr
	assert.Equal(f.t, []string{deprecatedResourceAssemblyV1Warning}, tlr.Warnings)
}

// Load the manifests, expecting warnings.
// Warnigns should be asserted later with assertWarnings
func (f *fixture) loadAllowWarnings(names ...string) {
	tlr, err := f.tfl.Load(f.ctx, f.JoinPath("Tiltfile"), matchMap(names...))
	if err != nil {
		f.t.Fatal(err)
	}
	f.loadResult = tlr
}

func unusedImageWarning(unusedImage string, suggestedImages []string) string {
	ret := fmt.Sprintf("Image not used in any resource:\n    ✕ %s", unusedImage)
	if len(suggestedImages) > 0 {
		ret = ret + fmt.Sprintf("\nDid you mean…")
		for _, s := range suggestedImages {
			ret = ret + fmt.Sprintf("\n    - %s", s)
		}
	}
	return ret
}

// Load the manifests, expecting warnings.
func (f *fixture) loadAssertWarnings(warnings ...string) {
	f.loadAllowWarnings()
	f.assertWarnings(warnings...)
}

func (f *fixture) loadErrString(msgs ...string) {
	tlr, err := f.tfl.Load(f.ctx, f.JoinPath("Tiltfile"), nil)
	if err == nil {
		f.t.Fatalf("expected error but got nil")
	}
	f.loadResult = tlr
	errText := err.Error()
	for _, msg := range msgs {
		if !strings.Contains(errText, msg) {
			f.t.Fatalf("error %q does not contain string %q", errText, msg)
		}
	}
}

func (f *fixture) gitInit(path string) {
	if err := os.MkdirAll(f.JoinPath(path, ".git"), os.FileMode(0777)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) assertNoMoreManifests() {
	if len(f.loadResult.Manifests) != 0 {
		names := make([]string, len(f.loadResult.Manifests))
		for i, m := range f.loadResult.Manifests {
			names[i] = m.Name.String()
		}
		f.t.Fatalf("expected no more manifests but found %d: %s",
			len(names), strings.Join(names, ", "))
	}
}

// Helper func for asserting that the next manifest is Unresourced
// k8s YAML containing the given k8s entities.
func (f *fixture) assertNextManifestUnresourced(expectedEntities ...string) {
	next := f.assertNextManifest(model.UnresourcedYAMLManifestName.String())

	entities, err := k8s.ParseYAML(bytes.NewBufferString(next.K8sTarget().YAML))
	assert.NoError(f.t, err)

	entityNames := make([]string, len(entities))
	for i, e := range entities {
		entityNames[i] = e.Name()
	}
	assert.Equal(f.t, expectedEntities, entityNames)
}

// assert functions and helpers
func (f *fixture) assertNextManifest(name string, opts ...interface{}) model.Manifest {
	if len(f.loadResult.Manifests) == 0 {
		f.t.Fatalf("no more manifests; trying to find %q (did you call `f.load`?)", name)
	}

	m := f.loadResult.Manifests[0]
	if m.Name != model.ManifestName(name) {
		f.t.Fatalf("expected next manifest to be '%s' but found '%s'", name, m.Name)
	}

	f.loadResult.Manifests = f.loadResult.Manifests[1:]

	imageIndex := 0
	nextImageTarget := func() model.ImageTarget {
		ret := m.ImageTargetAt(imageIndex)
		imageIndex++
		return ret
	}

	for _, opt := range opts {
		switch opt := opt.(type) {
		case dbHelper:
			image := nextImageTarget()

			ref := image.ConfigurationRef
			if ref.Empty() {
				f.t.Fatalf("manifest %v has no more image refs; expected %q", m.Name, opt.image.ref)
			}
			if !assert.Equal(f.t, opt.image.ref, ref.String(), "manifest %v image ref", m.Name) {
				f.t.FailNow()
			}

			if !assert.Equal(f.t, opt.image.deploymentRef, image.DeploymentRef.String(), "manifest %v image injected ref", m.Name) {
				f.t.FailNow()
			}

			if opt.cache != "" {
				assert.Contains(f.t, image.CachePaths(), opt.cache,
					"manifest %v cache paths don't include expected value", m.Name)
			}

			if !image.IsDockerBuild() {
				f.t.Fatalf("expected docker build but manifest %v has no docker build info", m.Name)
			}

			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case nestedFBHelper:
					dbInfo := image.DockerBuildInfo()
					if matcher.fb == nil {
						if !dbInfo.FastBuild.Empty() {
							f.t.Fatalf("expected docker build for manifest %v to have "+
								"no nested fastbuild, but found one: %v", m.Name, dbInfo.FastBuild)
						}
					} else {
						if dbInfo.FastBuild.Empty() {
							f.t.Fatalf("expected docker build for manifest %v to have "+
								"nested fastbuild, but found none", m.Name)
						}
						matcher.fb.checkMatchers(f, m, dbInfo.FastBuild)
					}
				case model.LiveUpdate:
					lu := image.AnyLiveUpdateInfo()
					assert.False(f.t, lu.Empty())
					assert.Equal(f.t, matcher, lu)
				default:
					f.t.Fatalf("unknown dbHelper matcher: %T %v", matcher, matcher)
				}
			}
		case fbHelper:
			image := nextImageTarget()

			ref := image.ConfigurationRef
			if ref.RefName() != opt.image.ref {
				f.t.Fatalf("manifest %v image ref: %q; expected %q", m.Name, ref.RefName(), opt.image.ref)
			}

			if opt.cache != "" {
				assert.Contains(f.t, image.CachePaths(), opt.cache,
					"manifest %v cache paths don't include expected value", m.Name)
			}

			if !image.IsFastBuild() {
				f.t.Fatalf("expected fast build but manifest %v has no fast build info", m.Name)
			}

			opt.checkMatchers(f, m, image.TopFastBuildInfo())
		case cbHelper:
			image := nextImageTarget()
			ref := image.ConfigurationRef
			if !assert.Equal(f.t, opt.image.ref, ref.String(), "manifest %v image ref", m.Name) {
				f.t.FailNow()
			}

			if !image.IsCustomBuild() {
				f.t.Fatalf("Expected custom build but manifest %v has no custom build info", m.Name)
			}
			cbInfo := image.CustomBuildInfo()

			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case depsHelper:
					assert.Equal(f.t, matcher.deps, cbInfo.Deps)
				case cmdHelper:
					assert.Equal(f.t, matcher.cmd, cbInfo.Command)
				case tagHelper:
					assert.Equal(f.t, matcher.tag, cbInfo.Tag)
				case disablePushHelper:
					assert.Equal(f.t, matcher.disabled, cbInfo.DisablePush)
				case fbHelper:
					if cbInfo.Fast.Empty() {
						f.t.Fatalf("Expected manifest %v to have fast build, but it didn't", m.Name)
					}

					matcher.checkMatchers(f, m, cbInfo.Fast)
				case model.LiveUpdate:
					lu := image.AnyLiveUpdateInfo()
					assert.False(f.t, lu.Empty())
					assert.Equal(f.t, matcher, lu)
				}
			}

		case deploymentHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.Kind.Kind == "Deployment" && e.Name() == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("deployment %v not found in yaml %q", opt.name, yaml)
			}
		case serviceHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.Kind.Kind == "Service" && e.Name() == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("service %v not found in yaml %q", opt.name, yaml)
			}
		case k8sObjectHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.Kind.Kind == opt.kind && e.Name() == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("entity of kind %s with name %s not found in yaml %q", opt.kind, opt.name, yaml)
			}
		case extraPodSelectorsHelper:
			assert.ElementsMatch(f.t, opt.labels, m.K8sTarget().ExtraPodSelectors)
		case numEntitiesHelper:
			yaml := m.K8sTarget().YAML
			entities := f.entities(yaml)
			if opt.num != len(f.entities(yaml)) {
				f.t.Fatalf("manifest %v has %v entities in %v; expected %v", m.Name, len(entities), yaml, opt.num)
			}

		case matchPathHelper:
			// Make sure the path matches one of the syncs.
			isDep := false
			path := f.JoinPath(opt.path)
			for _, d := range m.LocalPaths() {
				if ospath.IsChild(d, path) {
					isDep = true
				}
			}

			if !isDep {
				f.t.Errorf("Path %s is not a dependency of manifest %s", path, m.Name)
			}

			expectedFilter := opt.missing
			filter := ignore.CreateBuildContextFilter(m.ImageTargetAt(0))
			if m.IsDC() {
				filter = ignore.CreateBuildContextFilter(m.DockerComposeTarget())
			}
			filterName := "BuildContextFilter"
			if opt.fileChange {
				var err error
				if m.IsDC() {
					filter, err = ignore.CreateFileChangeFilter(m.DockerComposeTarget())
				} else {
					filter, err = ignore.CreateFileChangeFilter(m.ImageTargetAt(0))
				}
				if err != nil {
					f.t.Fatalf("Error creating file change filter: %v", err)
				}
				filterName = "FileChangeFilter"
			}

			actualFilter, err := filter.Matches(path, false)
			if err != nil {
				f.t.Fatalf("Error matching filter (%s): %v", path, err)
			}
			if actualFilter != expectedFilter {
				if expectedFilter {
					f.t.Errorf("%s should filter %s", filterName, path)
				} else {
					f.t.Errorf("%s should not filter %s", filterName, path)
				}
			}

		case []model.PortForward:
			assert.Equal(f.t, opt, m.K8sTarget().PortForwards)
		case model.UpdateMode:
			assert.Equal(f.t, opt, m.UpdateMode)
		default:
			f.t.Fatalf("unexpected arg to assertNextManifest: %T %v", opt, opt)
		}
	}

	f.assertManifestConsistency(m)

	return m
}

// All manifests currently contain redundant information
// such that each Deploy target lists its image ID dependencies.
func (f *fixture) assertManifestConsistency(m model.Manifest) {
	iTargetIDs := map[model.TargetID]bool{}
	for _, iTarget := range m.ImageTargets {
		if iTargetIDs[iTarget.ID()] {
			f.t.Fatalf("Image Target %s appears twice in manifest: %s", iTarget.ID(), m.Name)
		}
		iTargetIDs[iTarget.ID()] = true
	}

	deployTarget := m.DeployTarget()
	for _, depID := range deployTarget.DependencyIDs() {
		if !iTargetIDs[depID] {
			f.t.Fatalf("Image Target needed by deploy target is missing: %s", depID)
		}
	}
}

func (f *fixture) imageTargetNames(m model.Manifest) []string {
	result := []string{}
	for _, iTarget := range m.ImageTargets {
		result = append(result, iTarget.ID().Name.String())
	}
	return result
}

func (f *fixture) idNames(ids []model.TargetID) []string {
	result := []string{}
	for _, id := range ids {
		result = append(result, id.Name.String())
	}
	return result
}

func (f *fixture) assertNumManifests(expected int) {
	assert.Equal(f.t, expected, len(f.loadResult.Manifests))
}

func (f *fixture) assertConfigFiles(filenames ...string) {
	var expected []string
	for _, filename := range filenames {
		expected = append(expected, f.JoinPath(filename))
	}
	sort.Strings(expected)
	sort.Strings(f.loadResult.ConfigFiles)
	assert.Equal(f.t, expected, f.loadResult.ConfigFiles)
}

func (f *fixture) assertWarnings(warnings ...string) {
	var expected []string
	for _, warning := range warnings {
		expected = append(expected, warning)
	}
	sort.Strings(expected)
	sort.Strings(f.loadResult.Warnings)
	assert.Equal(f.t, expected, f.loadResult.Warnings)
}

func (f *fixture) entities(y string) []k8s.K8sEntity {
	es, err := k8s.ParseYAMLFromString(y)
	if err != nil {
		f.t.Fatal(err)
	}
	return es
}

type secretHelper struct {
	name string
}

func secret(name string) secretHelper {
	return secretHelper{name: name}
}

type namespaceHelper struct {
	namespace string
}

func namespace(namespace string) namespaceHelper {
	return namespaceHelper{namespace}
}

type deploymentHelper struct {
	name           string
	image          string
	templateLabels map[string]string
	envVars        []envVar
	namespace      string
}

func deployment(name string, opts ...interface{}) deploymentHelper {
	r := deploymentHelper{name: name}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case imageHelper:
			r.image = opt.ref
		case labelsHelper:
			r.templateLabels = opt.labels
		case envVarHelper:
			r.envVars = opt.envVars
		case namespaceHelper:
			r.namespace = opt.namespace
		default:
			panic(fmt.Errorf("unexpected arg to deployment: %T %v", opt, opt))
		}
	}
	return r
}

type serviceHelper struct {
	name           string
	selectorLabels map[string]string
}

func service(name string, opts ...interface{}) serviceHelper {
	r := serviceHelper{name: name}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case labelsHelper:
			r.selectorLabels = opt.labels
		default:
			panic(fmt.Errorf("unexpected arg to deployment: %T %v", opt, opt))
		}
	}
	return r
}

type k8sObjectHelper struct {
	name string
	kind string
}

func k8sObject(name string, kind string) k8sObjectHelper {
	return k8sObjectHelper{name: name, kind: kind}
}

type extraPodSelectorsHelper struct {
	labels []labels.Selector
}

func extraPodSelectors(labels ...labels.Set) extraPodSelectorsHelper {
	ret := extraPodSelectorsHelper{}
	for _, ls := range labels {
		ret.labels = append(ret.labels, ls.AsSelector())
	}
	return ret
}

type numEntitiesHelper struct {
	num int
}

func numEntities(num int) numEntitiesHelper {
	return numEntitiesHelper{num}
}

type matchPathHelper struct {
	path       string
	missing    bool
	fileChange bool
}

func buildMatches(path string) matchPathHelper {
	return matchPathHelper{
		path: path,
	}
}

func buildFilters(path string) matchPathHelper {
	return matchPathHelper{
		path:    path,
		missing: true,
	}
}

func fileChangeMatches(path string) matchPathHelper {
	return matchPathHelper{
		path:       path,
		fileChange: true,
	}
}

func fileChangeFilters(path string) matchPathHelper {
	return matchPathHelper{
		path:       path,
		missing:    true,
		fileChange: true,
	}
}

type imageHelper struct {
	ref           string
	deploymentRef string
}

func image(ref string) imageHelper {
	return imageHelper{ref: ref, deploymentRef: ref}
}

func imageNormalized(ref string) imageHelper {
	return image(container.MustNormalizeRef(ref))
}

func (ih imageHelper) withInjectedRef(injectedRef string) imageHelper {
	ih.deploymentRef = injectedRef
	return ih
}

type labelsHelper struct {
	labels map[string]string
}

func withLabels(labels map[string]string) labelsHelper {
	return labelsHelper{labels: labels}
}

type envVar struct {
	name  string
	value string
}

type envVarHelper struct {
	envVars []envVar
}

// usage: withEnvVars("key1", "value1", "key2", "value2")
func withEnvVars(envVars ...string) envVarHelper {
	var ret envVarHelper

	for i := 0; i < len(envVars); i += 2 {
		if i+1 >= len(envVars) {
			panic("withEnvVars called with odd number of strings")
		}
		ret.envVars = append(ret.envVars, envVar{envVars[i], envVars[i+1]})
	}

	return ret
}

// docker build helper
type dbHelper struct {
	image    imageHelper
	cache    string
	matchers []interface{}
}

func db(img imageHelper, opts ...interface{}) dbHelper {
	return dbHelper{image: img, matchers: opts}
}

func dbWithCache(img imageHelper, cache string, opts ...interface{}) dbHelper {
	return dbHelper{image: img, cache: cache, matchers: opts}
}

// fast build helper
type fbHelper struct {
	image    imageHelper
	cache    string
	matchers []interface{}
}

func fb(img imageHelper, opts ...interface{}) fbHelper {
	return fbHelper{image: img, matchers: opts}
}

func fbWithCache(img imageHelper, cache string, opts ...interface{}) fbHelper {
	return fbHelper{image: img, cache: cache, matchers: opts}
}

func (fb fbHelper) checkMatchers(f *fixture, m model.Manifest, fbInfo model.FastBuild) {
	syncs := fbInfo.Syncs
	runs := fbInfo.Runs
	for _, matcher := range fb.matchers {
		switch matcher := matcher.(type) {
		case addHelper:
			sync := syncs[0]
			syncs = syncs[1:]
			if sync.LocalPath != f.JoinPath(matcher.src) {
				f.t.Fatalf("manifest %v sync %+v src: %q; expected %q", m.Name, sync, sync.LocalPath, f.JoinPath(matcher.src))
			}
			if sync.ContainerPath != matcher.dest {
				f.t.Fatalf("manifest %v sync %+v dest: %q; expected %q", m.Name, sync, sync.ContainerPath, matcher.dest)
			}
		case runHelper:
			run := runs[0]
			runs = runs[1:]
			assert.Equal(f.t, model.ToShellCmd(matcher.cmd), run.Cmd)
			assert.Equal(f.t, matcher.triggers, run.Triggers.Paths)
			if !run.Triggers.Empty() {
				assert.Equal(f.t, f.Path(), run.Triggers.BaseDirectory)
			}
		case hotReloadHelper:
			assert.Equal(f.t, matcher.on, fbInfo.HotReload)
		default:
			f.t.Fatalf("unknown fbHelper matcher: %T %v", matcher, matcher)
		}
	}
}

// custom build helper
type cbHelper struct {
	image    imageHelper
	matchers []interface{}
}

func cb(img imageHelper, opts ...interface{}) cbHelper {
	return cbHelper{img, opts}
}

type nestedFBHelper struct {
	fb *fbHelper
}

func nestedFB(opts ...interface{}) nestedFBHelper {
	if len(opts) == 0 {
		return nestedFBHelper{nil}
	}
	return nestedFBHelper{&fbHelper{matchers: opts}}
}

type addHelper struct {
	src  string
	dest string
}

func add(src string, dest string) addHelper {
	return addHelper{src, dest}
}

type runHelper struct {
	cmd      string
	triggers []string
}

func run(cmd string, triggers ...string) runHelper {
	return runHelper{cmd, triggers}
}

type hotReloadHelper struct {
	on bool
}

func hotReload(on bool) hotReloadHelper {
	return hotReloadHelper{on: on}
}

type cmdHelper struct {
	cmd string
}

func cmd(cmd string) cmdHelper {
	return cmdHelper{cmd}
}

type tagHelper struct {
	tag string
}

func tag(tag string) tagHelper {
	return tagHelper{tag}
}

type depsHelper struct {
	deps []string
}

func deps(deps ...string) depsHelper {
	return depsHelper{deps}
}

type disablePushHelper struct {
	disabled bool
}

func disablePush(disable bool) disablePushHelper {
	return disablePushHelper{disable}
}

// useful scenarios to setup

// foo just has one image and one yaml
func (f *fixture) setupFoo() {
	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.gitInit("")
}

// bar just has one image and one yaml
func (f *fixture) setupFooAndBar() {
	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))

	f.dockerfile("bar/Dockerfile")
	f.yaml("bar.yaml", deployment("bar", image("gcr.io/bar")))

	f.gitInit("")
}

// expand has 4 images, a-d, and a yaml with all of it
func (f *fixture) setupExpand() {
	f.dockerfile("a/Dockerfile")
	f.dockerfile("b/Dockerfile")
	f.dockerfile("c/Dockerfile")
	f.dockerfile("d/Dockerfile")

	f.yaml("all.yaml",
		deployment("a", image("gcr.io/a")),
		deployment("b", image("gcr.io/b")),
		deployment("c", image("gcr.io/c")),
		deployment("d", image("gcr.io/d")),
	)

	f.gitInit("")
}

func (f *fixture) setupHelm() {
	f.file("helm/Chart.yaml", chartYAML)
	f.file("helm/values.yaml", valuesYAML)

	f.file("helm/templates/_helpers.tpl", helpersTPL)
	f.file("helm/templates/deployment.yaml", deploymentYAML)
	f.file("helm/templates/ingress.yaml", ingressYAML)
	f.file("helm/templates/service.yaml", serviceYAML)
}

func (f *fixture) setupHelmWithTest() {
	f.setupHelm()
	f.file("helm/templates/tests/test-mariadb-connection.yaml", helmTestYAML)
}

func (f *fixture) setupExtraPodSelectors(s string) {
	f.setupFoo()

	tiltfile := fmt.Sprintf(`
k8s_resource_assembly_version(2)
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo.yaml')
k8s_resource('foo', extra_pod_selectors=%s)
`, s)

	f.file("Tiltfile", tiltfile)
}

func (f *fixture) setupCRD() {
	f.file("crd.yaml", `apiVersion: fission.io/v1
kind: Environment
metadata:
  name: mycrd
spec:
  builder:
    command: build
    image: test/mycrd-builder
  poolsize: 1
  runtime:
    image: test/mycrd-env`)
}

func overwriteSelectorsForService(entity *k8s.K8sEntity, labels map[string]string) error {
	svc, ok := entity.Obj.(*v1.Service)
	if !ok {
		return fmt.Errorf("don't know how to set selectors for %T", entity.Obj)
	}
	svc.Spec.Selector = labels
	return nil
}

package tiltfile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

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

	f.loadErrString("foo/Dockerfile", "no such file or directory")
}

func TestSimple(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
}

// I.e. make sure that we handle de/normalization between `fooimage` <--> `docker.io/library/fooimage`
func TestLocalImageRef(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image("fooimage")))

	f.file("Tiltfile", `
docker_build('fooimage', 'foo')
k8s_resource('foo', 'foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(imageNormalized("fooimage")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
}

func TestExplicitDockerfileIsConfigFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile='other/Dockerfile')
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", "foo.yaml", "other/Dockerfile")
}

func TestExplicitDockerfileAsLocalPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
r = local_git_repo('.')
docker_build('gcr.io/foo', 'foo', dockerfile=r.path('other/Dockerfile'))
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", "foo.yaml", "other/Dockerfile")
}

func TestExplicitDockerfileContents(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile_contents='FROM alpine')
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", "foo.yaml")
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
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertConfigFiles("Tiltfile", "foo.yaml", "other/Dockerfile")
	f.assertNextManifest("foo", db(image("gcr.io/foo")))
}

func TestCantSpecifyDFPathAndContents(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.dockerfile("other/Dockerfile")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo', dockerfile_contents='FROM alpine', dockerfile='foo/Dockerfile')
k8s_resource('foo', 'foo.yaml')
`)

	f.loadErrString("Cannot specify both dockerfile and dockerfile_contents")
}

func TestFastBuildSimple(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi")
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi"), hotReload(false)),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
}

func TestFastBuildHotReload(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi") \
  .hot_reload()
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi"), hotReload(true)),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
}

func TestFastBuildPassedToResource(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
fb = fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi")
k8s_resource('foo', 'foo.yaml', image=fb)
`)
	f.load()
	f.assertNextManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi")),
		deployment("foo"),
	)
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
}

func TestFastBuildValidates(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("foo/Dockerfile", `
from golang:1.10
ADD . .`)
	f.file("Tiltfile", `
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi")
k8s_resource('foo', 'foo.yaml')
`)
	f.loadErrString("base Dockerfile contains an ADD/COPY")
}

func TestFastBuildRunBeforeAdd(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .run("echo hi") \
  .add(repo.path('foo'), 'src/')
k8s_resource('foo', 'foo.yaml')
`)
	f.loadErrString("fast_build(\"gcr.io/foo\").add() called after .run()")
}

func TestFastBuildTriggers(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi", trigger=['a', 'b']) \
  .run("echo again", trigger='c')
k8s_resource('foo', 'foo.yaml')
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
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
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
k8s_resource('foo', yaml)
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
k8s_resource('foo', yaml)
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
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
docker_build("gcr.io/foo", "foo")
k8s_resource('foo', kustomize("."))
`)
	f.load()
	f.assertNextManifest("foo", deployment("the-deployment"), numEntities(3))
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "configMap.yaml", "deployment.yaml", "kustomization.yaml", "service.yaml")
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
	f.assertNextManifest("foo", db(image("gcr.io/foo"), cache("/path/to/cache")))
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
	f.assertNextManifest("foo", db(image("gcr.io/foo"), cache("/path/to/cache")))
}

func TestDuplicateResourceNames(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
k8s_resource('a')
k8s_resource('a')
`)

	f.loadErrString("k8s_resource named \"a\" already exists")
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

func TestFastBuildAddStringFails(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
fast_build('gcr.io/foo', 'foo/Dockerfile').add('/foo', '/foo')
`)

	f.loadErrString("invalid type for src. Got string, want gitRepo OR localPath",
		"fast_build(\"gcr.io/foo\").add()")
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
k8s_resource('foo', 'foo.yaml', port_forwards=EXPR)
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
	f.assertNoYAMLManifest("")
	f.assertConfigFiles("Tiltfile", "all.yaml", "a/Dockerfile", "b/Dockerfile", "c/Dockerfile", "d/Dockerfile")
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
	f.assertYAMLManifest("a-secret")
}

func TestExpandExplicit(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupExpand()
	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
k8s_resource('explicit_a', image='gcr.io/a', port_forwards=8000)
`)
	f.load()
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
	f.assertYAMLManifest("not-pod-creator")
}

func TestUnresourcedYamlGrouping(t *testing.T) {
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
	f.assertYAMLManifest("someSecret")
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

	f.file("Tiltfile", `k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
`)
	f.load()

	f.assertNextManifest("a", deployment("deployment-a"))
	f.assertNextManifest("b", deployment("deployment-b"), service("service-b"))
	f.assertNextManifest("c", deployment("deployment-c"), service("service-c1"), service("service-c2"))
}

func TestK8sResourceWithoutDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `
k8s_resource('foo', yaml='foo.yaml', port_forwards=8000)
`)
	f.load()
	f.assertNextManifest("foo", []model.PortForward{{LocalPort: 8000}})
}

func TestImplicitK8sResourceWithoutDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("Tiltfile", `k8s_yaml('foo.yaml')
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
k8s_yaml('all.yaml')
docker_build('gcr.io/a', 'a')
docker_build('gcr.io/b', 'b')
docker_build('gcr.io/c', 'c')
docker_build('gcr.io/d', 'd')
`)
	f.load()
	f.assertNextManifest("a", db(image("gcr.io/a")), deployment("a"), deployment("a2"))
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
k8s_resource('foo', 'foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_resource('bar', 'bar.yaml')
`)

	f.load("foo")
	f.assertNumManifests(1)
	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))

	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml", "bar/Dockerfile", "bar.yaml")
}

func TestLoadTypoManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')

docker_build('gcr.io/bar', 'bar')
k8s_resource('bar', 'bar.yaml')
`)

	_, _, _, _, err := Load(f.ctx, f.JoinPath("Tiltfile"), matchMap("baz"), os.Stdout)
	if assert.Error(t, err) {
		assert.Equal(t, "Could not find resources: baz. Existing resources in Tiltfile: foo, bar", err.Error())
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

func TestGitignorePathFilter(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file(".gitignore", ".#*")
	f.file("Dockerfile", "FROM golang:1.10")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', '.')
k8s_yaml('foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters(".#foo.yaml"),
		fileChangeFilters(".#foo.yaml"),
	)
}

func TestAncestorGitignorePathFilter(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file(".gitignore", ".#*")
	f.file("foo/Dockerfile", "FROM golang:1.10")
	f.yaml("foo/foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_yaml('foo/foo.yaml')
`)

	f.load("foo")
	f.assertNextManifest("foo",
		buildFilters("foo/.#foo.yaml"),
		fileChangeFilters("foo/.#foo.yaml"),
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
k8s_resource('foo', 'foo.yaml')
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
repo = local_git_repo('.')
fast_build('gcr.io/foo', 'foo/Dockerfile') \
  .add(repo.path('foo'), 'src/') \
  .run("echo hi")
k8s_resource('foo', 'foo.yaml')
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

func TestFilterYaml(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.file("k8s.yaml", yaml.ConcatYAML(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml, testyaml.SanchoYAML))
	f.file("Tiltfile", `labels = {'app': 'doggos'}
doggos, rest = filter_yaml('k8s.yaml', labels=labels)
k8s_resource('doggos', yaml=doggos)
k8s_resource('rest', yaml=rest)
`)
	f.load()

	f.assertNextManifest("doggos", deployment("doggos"))
	f.assertNextManifest("rest", deployment("snack"), deployment("sancho"))
}

func TestTopLevelIfStatement(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
if True:
  docker_build('gcr.io/foo', 'foo')
  k8s_resource('foo', 'foo.yaml')
`)

	f.load()

	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertConfigFiles("Tiltfile", "foo/Dockerfile", "foo.yaml")
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

	f.assertYAMLManifest("release-name-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		"helm",
	)
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

	f.assertYAMLManifest("release-name-helloworld-chart")
	f.assertConfigFiles(
		"Tiltfile",
		"helm",
	)
}

func TestEmptyDockerfileStaticBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("foo/Dockerfile", "")
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	m := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.True(t, m.ImageTargetAt(0).IsStaticBuild())
	assert.False(t, m.ImageTargetAt(0).IsFastBuild())
}

func TestEmptyDockerfileFastBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.setupFoo()
	f.file("foo/Dockerfile", "")
	f.file("Tiltfile", `
fast_build('gcr.io/foo', 'foo/Dockerfile')
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	m := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.False(t, m.ImageTargetAt(0).IsStaticBuild())
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

	assert.Equal(t, 1, len(f.manifests))
	m := f.assertNextManifest("sancho")
	assert.Equal(t, 2, len(m.ImageTargets))
	assert.Equal(t, "gcr.io/some-project-162817/sancho",
		m.ImageTargetAt(0).Ref.String())
	assert.Equal(t, "gcr.io/some-project-162817/sancho-sidecar",
		m.ImageTargetAt(1).Ref.String())
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

	assert.Equal(t, 1, len(f.manifests))
	m := f.assertNextManifest("sancho")
	assert.Equal(t, 1, len(m.ImageTargets))
	assert.Equal(t, "gcr.io/some-project-162817/sancho",
		m.ImageTargetAt(0).Ref.String())
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

	f.load()
	f.assertWarnings("Image not used in any resource:\n    ✕ gcr.io/foo:stable\nDid you mean…\n    - gcr.io/foo\n    - docker.io/library/golang")
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

	f.load()
	f.assertWarnings("Image not used in any resource:\n    ✕ gcr.io/foo:stable\nDid you mean…\n    - gcr.io/foo\n    - docker.io/library/golang")
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

func TestImageDependency(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.gitInit("")
	f.file("imageA.dockerfile", "FROM golang:1.10")
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()
	m := f.assertNextManifest("image-b", deployment("foo"))
	assert.Equal(t, []string{"gcr.io/image-a", "gcr.io/image-b"}, f.imageTargetNames(m))
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
docker_build('gcr.io/image-a', '.', dockerfile='imageA.dockerfile')
docker_build('gcr.io/image-b', '.', dockerfile='imageB.dockerfile')
docker_build('gcr.io/image-c', '.', dockerfile='imageC.dockerfile')
docker_build('gcr.io/image-d', '.', dockerfile='imageD.dockerfile')
k8s_yaml('foo.yaml')
`)

	f.load()

	m := f.assertNextManifest("image-d", deployment("foo"))
	assert.Equal(t, []string{
		"gcr.io/image-a",
		"gcr.io/image-b",
		"gcr.io/image-c",
		"gcr.io/image-d",
	}, f.imageTargetNames(m))
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

func TestImageRefSuggestion(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
docker_build('gcr.typo.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
`)
	f.load()
	f.assertWarnings("Image not used in any resource:\n    ✕ gcr.typo.io/foo\nDid you mean…\n    - gcr.io/foo\n    - docker.io/library/golang")
}

func TestDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Chdir(wd)
	}()
	err = os.Chdir(f.TempDirFixture.Path())
	if err != nil {
		t.Fatal(err)
	}

	f.gitInit("")
	f.yaml("config/foo.yaml", deployment("foo", image("gcr.io/foo")))
	f.yaml("config/bar.yaml", deployment("bar", image("gcr.io/bar")))
	f.file("Tiltfile", `k8s_yaml(listdir('config'))`)

	f.load("foo", "bar")
	f.assertNumManifests(2)
	f.assertConfigFiles("Tiltfile", "config/foo.yaml", "config/bar.yaml")
}

type fixture struct {
	ctx context.Context
	t   *testing.T
	*tempdir.TempDirFixture

	// created by load
	manifests    []model.Manifest
	yamlManifest model.Manifest
	configFiles  []string
	warnings     []string
}

func newFixture(t *testing.T) *fixture {
	out := new(bytes.Buffer)
	ctx := output.ForkedCtxForTest(out)
	f := tempdir.NewTempDirFixture(t)

	r := &fixture{
		ctx:            ctx,
		t:              t,
		TempDirFixture: f,
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

func (f *fixture) load(names ...string) {
	manifests, yamlManifest, configFiles, warnings, err := Load(f.ctx, f.JoinPath("Tiltfile"), matchMap(names...), os.Stdout)
	if err != nil {
		f.t.Fatal(err)
	}
	f.manifests = manifests
	f.yamlManifest = yamlManifest
	f.configFiles = configFiles
	f.warnings = warnings
}

func (f *fixture) loadErrString(msgs ...string) {
	manifests, _, configFiles, _, err := Load(f.ctx, f.JoinPath("Tiltfile"), nil, os.Stdout)
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

func (f *fixture) gitInit(path string) {
	if err := os.Mkdir(f.JoinPath(path, ".git"), os.FileMode(0777)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) assertNoYAMLManifest(name string) {
	assert.Equal(f.t, model.Manifest{}, f.yamlManifest)
}

func (f *fixture) assertYAMLManifest(resNames ...string) {
	assert.Equal(f.t, unresourcedName, f.yamlManifest.ManifestName().String())

	entities, err := k8s.ParseYAML(bytes.NewBufferString(f.yamlManifest.K8sTarget().YAML))
	assert.NoError(f.t, err)

	entityNames := make([]string, len(entities))
	for i, e := range entities {
		entityNames[i] = e.Name()
	}
	assert.Equal(f.t, resNames, entityNames)
}

// assert functions and helpers
func (f *fixture) assertNextManifest(name string, opts ...interface{}) model.Manifest {
	if len(f.manifests) == 0 {
		f.t.Fatalf("no more manifests; trying to find %q (did you call `f.load`?)", name)
	}

	m := f.manifests[0]
	if m.Name != model.ManifestName(name) {
		f.t.Fatalf("expected next manifest to be '%s' but found '%s'", name, m.Name)
	}

	f.manifests = f.manifests[1:]

	for _, opt := range opts {
		switch opt := opt.(type) {
		case dbHelper:
			caches := m.ImageTargetAt(0).CachePaths()
			ref := m.ImageTargetAt(0).Ref
			if ref == nil {
				f.t.Fatalf("manifest %v has no image ref; expected %q", m.Name, opt.image.ref)
			}
			if ref.Name() != opt.image.ref {
				f.t.Fatalf("manifest %v image ref: %q; expected %q", m.Name, ref.Name(), opt.image.ref)
			}
			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case cacheHelper:
					cache := caches[0]
					caches = caches[1:]
					if cache != matcher.path {
						f.t.Fatalf("manifest %v cache %q; expected %q", m.Name, cache, matcher.path)
					}
				default:
					f.t.Fatalf("unknown dbHelper matcher: %T %v", matcher, matcher)
				}
			}
		case fbHelper:
			image := m.ImageTargetAt(0)
			ref := image.Ref
			if ref.Name() != opt.image.ref {
				f.t.Fatalf("manifest %v image ref: %q; expected %q", m.Name, ref.Name(), opt.image.ref)
			}

			if !image.IsFastBuild() {
				f.t.Fatalf("expected fast build but manifest %v has no fast build info", m.Name)
			}
			fbInfo := image.FastBuildInfo()

			mounts := fbInfo.Mounts
			steps := fbInfo.Steps
			for _, matcher := range opt.matchers {
				switch matcher := matcher.(type) {
				case addHelper:
					mount := mounts[0]
					mounts = mounts[1:]
					if mount.LocalPath != f.JoinPath(matcher.src) {
						f.t.Fatalf("manifest %v mount %+v src: %q; expected %q", m.Name, mount, mount.LocalPath, f.JoinPath(matcher.src))
					}
				case runHelper:
					step := steps[0]
					steps = steps[1:]
					assert.Equal(f.t, model.ToShellCmd(matcher.cmd), step.Cmd)
					assert.Equal(f.t, matcher.triggers, step.Triggers)
				case hotReloadHelper:
					assert.Equal(f.t, matcher.on, fbInfo.HotReload)
				default:
					f.t.Fatalf("unknown fbHelper matcher: %T %v", matcher, matcher)
				}
			}
		case deploymentHelper:
			yaml := m.K8sTarget().YAML
			found := false
			for _, e := range f.entities(yaml) {
				if e.Kind.Kind == "Deployment" && f.k8sName(e) == opt.name {
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
				if e.Kind.Kind == "Service" && f.k8sName(e) == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("service %v not found in yaml %q", opt.name, yaml)
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
			// Make sure the path matches one of the mounts.
			isDep := false
			path := f.JoinPath(opt.path)
			for _, d := range m.LocalPaths() {
				_, isChild := ospath.Child(d, path)
				if isChild {
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

func (f *fixture) assertNumManifests(expected int) {
	assert.Equal(f.t, expected, len(f.manifests))
}

func (f *fixture) assertConfigFiles(filenames ...string) {
	var expected []string
	for _, filename := range filenames {
		expected = append(expected, f.JoinPath(filename))
	}
	sort.Strings(expected)
	sort.Strings(f.configFiles)
	assert.Equal(f.t, expected, f.configFiles)
}

func (f *fixture) assertWarnings(warnings ...string) {
	var expected []string
	for _, warning := range warnings {
		expected = append(expected, warning)
	}
	sort.Strings(expected)
	sort.Strings(f.warnings)
	assert.Equal(f.t, expected, f.warnings)
}

func (f *fixture) entities(y string) []k8s.K8sEntity {
	es, err := k8s.ParseYAMLFromString(y)
	if err != nil {
		f.t.Fatal(err)
	}
	return es
}

func (f *fixture) k8sName(e k8s.K8sEntity) string {
	// Every k8s object we care about has is a pointer to a struct with a field ObjectMeta that has a field "Name" that's a string.
	name := reflect.ValueOf(e.Obj).Elem().FieldByName("ObjectMeta").FieldByName("Name")
	if !name.IsValid() {
		return ""
	}
	return name.String()
}

type secretHelper struct {
	name string
}

func secret(name string) secretHelper {
	return secretHelper{name: name}
}

type deploymentHelper struct {
	name           string
	image          string
	templateLabels map[string]string
}

func deployment(name string, opts ...interface{}) deploymentHelper {
	r := deploymentHelper{name: name}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case imageHelper:
			r.image = opt.ref
		case labelsHelper:
			r.templateLabels = opt.labels
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
	ref string
}

func image(ref string) imageHelper {
	return imageHelper{ref: ref}
}

func imageNormalized(ref string) imageHelper {
	return imageHelper{ref: docker.MustNormalizeRefName(ref)}
}

type labelsHelper struct {
	labels map[string]string
}

func withLabels(labels map[string]string) labelsHelper {
	return labelsHelper{labels: labels}
}

// match a docker_build
type dbHelper struct {
	image    imageHelper
	matchers []interface{}
}

func db(img imageHelper, opts ...interface{}) dbHelper {
	return dbHelper{img, opts}
}

type fbHelper struct {
	image    imageHelper
	matchers []interface{}
}

type cacheHelper struct {
	path string
}

func cache(path string) cacheHelper {
	return cacheHelper{path}
}

func fb(img imageHelper, opts ...interface{}) fbHelper {
	return fbHelper{img, opts}
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

func (f *fixture) setupExtraPodSelectors(s string) {
	f.setupFoo()

	tiltfile := fmt.Sprintf(`
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml', extra_pod_selectors=%s)
`, s)

	f.file("Tiltfile", tiltfile)
}

func overwriteSelectorsForService(entity *k8s.K8sEntity, labels map[string]string) error {
	svc, ok := entity.Obj.(*v1.Service)
	if !ok {
		return fmt.Errorf("don't know how to set selectors for %T", entity.Obj)
	}
	svc.Spec.Selector = labels
	return nil
}

package tiltfile2

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

	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestNoTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.loadErrString("no such file")
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

	f.assertManifest("foo",
		db(image("gcr.io/foo")),
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
	f.assertManifest("foo",
		fb(image("gcr.io/foo"), add("foo", "src/"), run("echo hi")),
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
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

	f.assertManifest("foo",
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

	f.assertManifest("foo",
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
	f.assertManifest("foo", deployment("the-deployment"), numEntities(3))
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
	f.assertManifest("foo", db(image("gcr.io/foo"), cache("/path/to/cache")))
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
	f.assertManifest("foo", db(image("gcr.io/foo"), cache("/path/to/cache")))
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

func TestFastBuildAddStringFailes(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
fast_build('gcr.io/foo', 'foo/Dockerfile').add('/foo', '/foo')
`)

	f.loadErrString("invalid type for src. Got string want gitRepo OR localPath")
}

type portForwardCase struct {
	name     string
	expr     string
	expected []model.PortForward
}

func TestPortForward(t *testing.T) {
	portForwardCases := []portForwardCase{
		{"value_local", "8000", []model.PortForward{{LocalPort: 8000}}},
		{"value_both", "port_forward(8001, 443)", []model.PortForward{{LocalPort: 8001, ContainerPort: 443}}},
		{"list", "[8000, port_forward(8001, 443)]", []model.PortForward{{LocalPort: 8000}, {LocalPort: 8001, ContainerPort: 443}}},
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
			f.load()
			f.assertManifest("foo",
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
	f.assertManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertManifest("d", db(image("gcr.io/d")), deployment("d"))
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
	f.assertManifest("a", db(image("gcr.io/a")), deployment("a"))
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
	f.assertManifest("explicit_a", db(image("gcr.io/a")), deployment("a"), []model.PortForward{{LocalPort: 8000}})
	f.assertManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertManifest("d", db(image("gcr.io/d")), deployment("d"))
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
	f.assertManifest("a", db(image("gcr.io/a")), deployment("a"), deployment("a2"))
	f.assertManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertManifest("d", db(image("gcr.io/d")), deployment("d"))
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
	f.assertManifest("a", db(image("gcr.io/a")), deployment("a"))
	f.assertManifest("b", db(image("gcr.io/b")), deployment("b"))
	f.assertManifest("c", db(image("gcr.io/c")), deployment("c"))
	f.assertManifest("d", db(image("gcr.io/d")), deployment("d"))
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
	f.assertManifest("foo",
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

	_, _, _, err := Load(f.ctx, f.JoinPath("Tiltfile"), matchMap("baz"))
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
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
	f.assertManifest("foo",
		buildFilters("foo/a.txt"),
		fileChangeFilters("foo/a.txt"),
		buildMatches("foo/subdir/a.txt"),
		fileChangeMatches("foo/subdir/a.txt"),
	)
}

func TestDockerComposeResourceCreation(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("docker-compose.yml", `
version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:12312"`)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.load("foo")
	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.assertManifest("foo", dcConfigPath(configPath))

	expectedConfFiles := []string{"Tiltfile", "docker-compose.yml", "foo/Dockerfile"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestMultipleDockerComposeNotSupported(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("docker-compose1.yml", `
version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:12312"`)
	f.file("docker-compose2.yml", `
version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:12312"`)

	tf := `docker_compose('docker-compose1.yml')
docker_compose('docker-compose2.yml')`
	f.file("Tiltfile", tf)

	f.loadErrString("already have a docker-compose resource declared")
}

func TestDockerComposeAndK8sNotSupported(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("docker-compose.yml", `
version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:12312"`)
	tf := `docker_compose('docker-compose.yml')
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')`
	f.file("Tiltfile", tf)

	f.loadErrString("can't declare both k8s resources and docker-compose resources")
}

func TestDockerComposeResourceCreationFromAbsPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.setupFoo()
	f.file("docker-compose.yml", `
version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:12312"`)
	f.file("Tiltfile", fmt.Sprintf("docker_compose('%s')", configPath))

	f.load("foo")
	f.assertManifest("foo", dcConfigPath(configPath))
}

type fixture struct {
	ctx context.Context
	t   *testing.T
	*tempdir.TempDirFixture

	// created by load
	manifests    []model.Manifest
	yamlManifest model.YAMLManifest
	configFiles  []string
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
	f.file(path, "FROM golang:1.10")
}

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
	manifests, yamlManifest, configFiles, err := Load(f.ctx, f.JoinPath("Tiltfile"), matchMap(names...))
	if err != nil {
		f.t.Fatal(err)
	}
	f.manifests = manifests
	f.yamlManifest = yamlManifest
	f.configFiles = configFiles
}

func (f *fixture) loadErrString(msgs ...string) {
	manifests, _, configFiles, err := Load(f.ctx, f.JoinPath("Tiltfile"), nil)
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
	assert.Equal(f.t, model.YAMLManifest{}, f.yamlManifest)
}

func (f *fixture) assertYAMLManifest(resNames ...string) {
	assert.Equal(f.t, unresourcedName, f.yamlManifest.ManifestName().String())

	entities, err := k8s.ParseYAML(bytes.NewBufferString(f.yamlManifest.K8sYAML()))
	assert.NoError(f.t, err)

	entityNames := make([]string, len(entities))
	for i, e := range entities {
		entityNames[i] = e.Name()
	}
	assert.Equal(f.t, resNames, entityNames)
}

// assert functions and helpers
func (f *fixture) assertManifest(name string, opts ...interface{}) model.Manifest {
	if len(f.manifests) == 0 {
		f.t.Fatalf("no more manifests; trying to find %q", name)
	}

	m := f.manifests[0]
	f.manifests = f.manifests[1:]

	for _, opt := range opts {
		switch opt := opt.(type) {
		case dbHelper:
			caches := m.CachePaths()
			if m.DockerRef() == nil {
				f.t.Fatalf("manifest %v has no image ref; expected %q", m.Name, opt.image.ref)
			}
			if m.DockerRef().Name() != opt.image.ref {
				f.t.Fatalf("manifest %v image ref: %q; expected %q", m.Name, m.DockerRef().Name(), opt.image.ref)
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
			if m.DockerRef().Name() != opt.image.ref {
				f.t.Fatalf("manifest %v image ref: %q; expected %q", m.Name, m.DockerRef().Name(), opt.image.ref)
			}

			mounts := m.Mounts
			steps := m.Steps
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
				default:
					f.t.Fatalf("unknown fbHelper matcher: %T %v", matcher, matcher)
				}
			}
		case deployHelper:
			found := false
			for _, e := range f.entities(m) {
				if e.Kind.Kind == "Deployment" && f.k8sName(e) == opt.name {
					found = true
					break
				}
			}
			if !found {
				f.t.Fatalf("deployment %v not found in yaml %q", opt.name, m.K8sYAML())
			}
		case numEntitiesHelper:
			if opt.num != len(f.entities(m)) {
				f.t.Fatalf("manifest %v has %v entities in %v; expected %v", m.Name, len(f.entities(m)), m.K8sYAML(), opt.num)
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
			filter := ignore.CreateBuildContextFilter(m)
			filterName := "BuildContextFilter"
			if opt.fileChange {
				var err error
				filter, err = ignore.CreateFileChangeFilter(m)
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
			assert.Equal(f.t, opt, m.PortForwards())
		case dcConfigPathHelper:
			dcInfo := m.DCInfo()
			if assert.False(f.t, dcInfo.Empty(), "expected a docker-compose manifest") {
				assert.Equal(f.t, opt.path, dcInfo.ConfigPath)
			}
		default:
			f.t.Fatalf("unexpected arg to assertManifest: %T %v", opt, opt)
		}
	}
	return m
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

func (f *fixture) entities(m model.Manifest) []k8s.K8sEntity {
	yamlText := m.K8sYAML()
	es, err := k8s.ParseYAMLFromString(yamlText)
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

type dcConfigPathHelper struct {
	path string
}

func dcConfigPath(path string) dcConfigPathHelper {
	return dcConfigPathHelper{path}
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
		default:
			panic(fmt.Errorf("unexpected arg to deployment: %T %v", opt, opt))
		}
	}
	return r
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

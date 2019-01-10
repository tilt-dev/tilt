package tiltfile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
)

const simpleConfig = `version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:12312"`

// YAML for Foo config looks a little different from the above after being read into
// a struct and YAML'd back out...
func (f *fixture) simpleConfigFooYAML() string {
	return fmt.Sprintf(`build:
  context: %s
command: sleep 100
ports:
- 12312:12312/tcp`, f.JoinPath("foo"))
}

func TestDockerComposeManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.load("foo")
	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.assertDcManifest("foo",
		dcConfigPath(configPath),
		dcYAMLRaw(f.simpleConfigFooYAML()),
		dcDfRaw(simpleDockerfile),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", "docker-compose.yml", "foo/Dockerfile"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeManifestNoDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("docker-compose.yml", `version: '3'
services:
  bar:
    image: redis:alpine`)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.load("bar")
	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.assertDcManifest("foo",
		dcConfigPath(configPath),
		dcYAMLRaw("image: redis:alpine"),
		dcDfRaw(""),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", "docker-compose.yml"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeManifestAlternateDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	dcYAML := fmt.Sprintf(`build:
  context: %s
  dockerfile: alternate-Dockerfile`,
		f.JoinPath("baz"))
	f.dockerfile("baz/alternate-Dockerfile")
	f.file("docker-compose.yml", fmt.Sprintf(`
version: '3'
services:
  baz:
    build:
      context: %s
      dockerfile: alternate-Dockerfile`, f.JoinPath("baz")))
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.load("baz")
	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.assertDcManifest("baz",
		dcConfigPath(configPath),
		dcYAMLRaw(dcYAML),
		dcDfRaw(simpleDockerfile),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", "docker-compose.yml", "baz/alternate-Dockerfile"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestMultipleDockerComposeNotSupported(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("foo/Dockerfile")
	f.file("docker-compose1.yml", simpleConfig)
	f.file("docker-compose2.yml", simpleConfig)

	tf := `docker_compose('docker-compose1.yml')
docker_compose('docker-compose2.yml')`
	f.file("Tiltfile", tf)

	f.loadErrString("already have a docker-compose resource declared")
}

func TestDockerComposeAndK8sNotSupported(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("docker-compose.yml", simpleConfig)
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
	f.dockerfile("foo/Dockerfile")
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
	f.assertDcManifest("foo", dcConfigPath(configPath))
}

func TestDockerComposeManifestComputesMountsFromDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	df := `FROM alpine

ADD ./src /app
COPY ./thing.go /stuff
RUN echo hi`
	f.file("foo/Dockerfile", df)

	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	expectedMounts := []model.Mount{
		model.Mount{
			LocalPath:     f.JoinPath("foo/src"),
			ContainerPath: "/app",
		},
		model.Mount{
			LocalPath:     f.JoinPath("foo/thing.go"),
			ContainerPath: "/stuff",
		},
	}

	f.load("foo")
	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.assertDcManifest("foo",
		dcConfigPath(configPath),
		dcYAMLRaw(f.simpleConfigFooYAML()),
		dcDfRaw(df),
		dcMounts(expectedMounts),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", "docker-compose.yml", "foo/Dockerfile"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeHonorsDockerIgnore(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	df := `FROM alpine

ADD . /app
COPY ./thing.go /stuff
RUN echo hi`
	f.file("foo/Dockerfile", df)

	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.file("foo/.dockerignore", "tmp")
	f.file(".dockerignore", "foo/tmp2")

	f.load("foo")

	f.assertManifest("foo",
		buildFilters("foo/tmp2"),
		fileChangeFilters("foo/tmp2"),
		buildFilters("foo/tmp"),
		fileChangeFilters("foo/tmp"),
	)
}

func TestDockerComposeHonorsGitIgnore(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	df := `FROM alpine

ADD . /app
COPY ./thing.go /stuff
RUN echo hi`
	f.file("foo/Dockerfile", df)

	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")
	f.gitInit(".")

	f.file(".gitignore", "foo/tmp")

	f.load("foo")

	f.assertManifest("foo",
		buildFilters("foo/tmp"),
		fileChangeFilters("foo/tmp"),
	)
}

func (f *fixture) assertDcManifest(name string, opts ...interface{}) model.Manifest {
	m := f.assertManifest(name)

	if !m.IsDC() {
		f.t.Error("expected a docker-compose manifest")
	}
	dcInfo := m.DockerComposeTarget()

	for _, opt := range opts {
		switch opt := opt.(type) {
		case dcConfigPathHelper:
			assert.Equal(f.t, opt.path, dcInfo.ConfigPath)
		case dcMountsHelper:
			assert.ElementsMatch(f.t, opt.mounts, dcInfo.Mounts)
		case dcYAMLRawHelper:
			assert.Equal(f.t, strings.TrimSpace(opt.yaml), strings.TrimSpace(string(dcInfo.YAMLRaw)))
		case dcDfRawHelper:
			assert.Equal(f.t, strings.TrimSpace(opt.df), strings.TrimSpace(string(dcInfo.DfRaw)))
		default:
			f.t.Fatalf("unexpected arg to assertDcManifest: %T %v", opt, opt)
		}
	}
	return m
}

type dcConfigPathHelper struct {
	path string
}

func dcConfigPath(path string) dcConfigPathHelper {
	return dcConfigPathHelper{path}
}

type dcYAMLRawHelper struct {
	yaml string
}

func dcYAMLRaw(yaml string) dcYAMLRawHelper {
	return dcYAMLRawHelper{yaml}
}

type dcDfRawHelper struct {
	df string
}

func dcDfRaw(df string) dcDfRawHelper {
	return dcDfRawHelper{df}
}

type dcMountsHelper struct {
	mounts []model.Mount
}

func dcMounts(mounts []model.Mount) dcMountsHelper {
	return dcMountsHelper{mounts}
}

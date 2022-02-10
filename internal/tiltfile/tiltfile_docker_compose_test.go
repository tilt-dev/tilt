package tiltfile

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/model"
)

const simpleConfig = `version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:80"`

const configWithMounts = `version: '3.2'
services:
  foo:
    build: ./foo
    command: sleep 100
    volumes:
      - ./foo:/foo
      # these volumes are currently unsupported, but included here to ensure we don't blow up on them
      - bar:/bar
      - type: volume
        source: baz
        target: /baz
    ports:
      - "12312:80"
volumes:
  bar: {}
  baz: {}`

const barServiceConfig = `version: '3'
services:
  bar:
    image: bar-image
    expose:
      - "3000"
    depends_on:
      - foo
`

const twoServiceConfig = `version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:80"
  bar:
    image: bar-image
    expose:
      - "3000"
    depends_on:
      - foo
`

// YAML for Foo config looks a little different from the above after being read into
// a struct and YAML'd back out...
func (f *fixture) simpleConfigAfterParse() string {
	return fmt.Sprintf(`build:
    context: %s
    dockerfile: Dockerfile
command:
    - sleep
    - "100"
networks:
    default: null
ports:
    - mode: ingress
      target: 80
      published: 12312
      protocol: tcp`, f.JoinPath("foo"))
}

func TestDockerComposeNothingError(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", "docker_compose(None)")

	f.loadErrString("Nothing to compose")
}

func TestDockerComposeBadTypeError(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", "docker_compose(True)")

	f.loadErrString("expected blob | path (string). Actual type: starlark.Bool")
}

func TestDockerComposeManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.load()
	f.assertDcManifest("foo",
		dcServiceYAML(f.simpleConfigAfterParse()),
		dockerComposeManagedImage(f.JoinPath("foo", "Dockerfile"), f.JoinPath("foo")),
		dcPublishedPorts(12312),
	)

	expectedConfFiles := []string{
		"Tiltfile",
		".tiltignore",
		"docker-compose.yml",
		f.JoinPath("foo", ".dockerignore"),
	}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeEnvFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("docker-compose.yml", `services:
  bar:
    image: bar-image
    ports:
      - "$BAR_PORT:$BAR_PORT"
`)
	f.file("local.env", "BAR_PORT=4000\n")
	f.file("Tiltfile", "docker_compose('docker-compose.yml', env_file='local.env')")

	f.load()
	f.assertDcManifest("bar", dcPublishedPorts(4000))

	expectedConfFiles := []string{
		"Tiltfile",
		".tiltignore",
		"local.env",
		"docker-compose.yml",
	}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeConflict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `
local_resource("foo", "foo")
docker_compose('docker-compose.yml')
`)

	f.loadErrString(`local_resource named "foo" already exists`)
}

func TestDockerComposeYAMLBlob(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose(read_file('docker-compose.yml'))")

	f.load()
	f.assertDcManifest("foo",
		dcServiceYAML(f.simpleConfigAfterParse()),
		dockerComposeManagedImage(f.JoinPath("foo", "Dockerfile"), f.JoinPath("foo")),
		dcPublishedPorts(12312),
	)

	expectedConfFiles := []string{
		"Tiltfile",
		".tiltignore",
		"docker-compose.yml",
		f.JoinPath("foo", ".dockerignore"),
	}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeTwoInlineBlobs(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("Tiltfile", fmt.Sprintf(`docker_compose([blob("""\n%s\n"""), blob("""\n%s\n""")])`, simpleConfig, barServiceConfig))

	f.load()

	assert.Equal(t, 2, len(f.loadResult.Manifests))
}

func TestDockerComposeBlobAndFileUsesFileDirForProjectPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", fmt.Sprintf(`docker_compose([blob("""\n%s\n"""), 'docker-compose.yml'])`, barServiceConfig))

	f.load()

	assert.Equal(t, 2, len(f.loadResult.Manifests))
	f.assertDcManifest("foo",
		dcServiceYAML(f.simpleConfigAfterParse()),
		dockerComposeManagedImage(f.JoinPath("foo", "Dockerfile"), f.JoinPath("foo")),
		dcPublishedPorts(12312),
	)
}

func TestDockerComposeManifestNoDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("docker-compose.yml", `version: '3'
services:
  bar:
    image: redis:alpine`)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	expectedYAML := `image: redis:alpine
networks:
    default: null`

	f.load("bar")
	f.assertDcManifest("bar",
		dcServiceYAML(expectedYAML),
		noImage(),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", ".tiltignore", "docker-compose.yml"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeManifestAlternateDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("baz/alternate-Dockerfile")
	f.file("docker-compose.yml", fmt.Sprintf(`
version: '3'
services:
  baz:
    build:
      context: %s
      dockerfile: alternate-Dockerfile`, f.JoinPath("baz")))
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	expectedYAML := fmt.Sprintf(`build:
    context: %s
    dockerfile: alternate-Dockerfile
networks:
    default: null`,
		f.JoinPath("baz"))

	f.load("baz")
	f.assertDcManifest("baz",
		dcServiceYAML(expectedYAML),
		dockerComposeManagedImage(f.JoinPath("baz", "alternate-Dockerfile"), f.JoinPath("baz")),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", ".tiltignore", "docker-compose.yml", "baz/.dockerignore"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeManifestAbsoluteDockerfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	dockerfilePath := f.JoinPath("baz", "Dockerfile")
	f.dockerfile(dockerfilePath)
	f.file("docker-compose.yml", fmt.Sprintf(`
version: '3'
services:
  baz:
    build:
      context: %s
      dockerfile: %s`, f.JoinPath("baz"), dockerfilePath))
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	expectedYAML := fmt.Sprintf(`build:
    context: %s
    dockerfile: %s
networks:
    default: null`,
		f.JoinPath("baz"),
		dockerfilePath)

	f.load("baz")
	f.assertDcManifest("baz",
		dcServiceYAML(expectedYAML),
		dockerComposeManagedImage(f.JoinPath("baz", "alternate-Dockerfile"), f.JoinPath("baz")),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{"Tiltfile", ".tiltignore", "docker-compose.yml", "baz/.dockerignore"}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeManifestAlternateDockerfileAndDockerIgnore(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile("baz/alternate-Dockerfile")
	f.dockerignore("baz/alternate-Dockerfile.dockerignore")
	f.file("docker-compose.yml", fmt.Sprintf(`
version: '3'
services:
  baz:
    build:
      context: %s
      dockerfile: alternate-Dockerfile`, f.JoinPath("baz")))
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	expectedYAML := fmt.Sprintf(`build:
    context: %s
    dockerfile: alternate-Dockerfile
networks:
    default: null`,
		f.JoinPath("baz"))

	f.load("baz")
	f.assertDcManifest("baz",
		dcServiceYAML(expectedYAML),
		dockerComposeManagedImage(f.JoinPath("baz", "alternate-Dockerfile"), f.JoinPath("baz")),
		// TODO(maia): assert m.tiltFilename
	)

	expectedConfFiles := []string{
		"Tiltfile",
		".tiltignore",
		"docker-compose.yml",
		"baz/alternate-Dockerfile.dockerignore",
	}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestMultipleDockerComposeDifferentDirsNotSupported(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose1.yml", simpleConfig)

	f.dockerfile(filepath.Join("subdir", "foo", "Dockerfile"))
	f.file(filepath.Join("subdir", "Tiltfile"), `docker_compose('docker-compose2.yml')`)
	f.file(filepath.Join("subdir", "docker-compose2.yml"), simpleConfig)

	tf := `
include('./subdir/Tiltfile')
docker_compose('docker-compose1.yml')`
	f.file("Tiltfile", tf)

	f.loadErrString("Cannot load docker-compose files from two different Tiltfiles")
}

func TestMultipleDockerComposeSameDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose1.yml", simpleConfig)
	f.file("docker-compose2.yml", barServiceConfig)

	tf := `
docker_compose('docker-compose1.yml')
docker_compose('docker-compose2.yml')`
	f.file("Tiltfile", tf)

	f.load()

	assert.Equal(t, 2, len(f.loadResult.Manifests))
}

func TestDockerComposeAndK8sNotSupported(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("docker-compose.yml", simpleConfig)
	tf := `docker_compose('docker-compose.yml')
k8s_yaml('bar.yaml')`
	f.file("Tiltfile", tf)

	f.loadErrString("can't declare both k8s " +
		"resources/entities and docker-compose resources")
}

func TestDockerComposeResourceCreationFromAbsPath(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", `
version: '3'
services:
  foo:
    build: ./foo
    command: sleep 100
    ports:
      - "12312:80"`)
	f.file("Tiltfile", fmt.Sprintf("docker_compose(%q)", configPath))

	f.load("foo")
	f.assertDcManifest("foo")
}

func TestDockerComposeMultiStageBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	df := `FROM alpine as builder
ADD ./src /app
RUN echo hi

FROM alpine
COPY --from=builder /app /app
RUN echo bye`
	f.file(filepath.Join("foo", "Dockerfile"), df)
	f.file(filepath.Join("foo", "docker-compose.yml"), `version: '3'
services:
  foo:
    build:
      context: ./
    command: sleep 100
    ports:
      - "12312:80"`)
	f.file("Tiltfile", "docker_compose('foo/docker-compose.yml')")
	f.load("foo")
	f.assertDcManifest("foo",
		dcServiceYAML(f.simpleConfigAfterParse()),
		dockerComposeManagedImage(f.JoinPath("foo", "Dockerfile"), f.JoinPath("foo")),
		dcPublishedPorts(12312),
	)

	expectedConfFiles := []string{
		"Tiltfile",
		".tiltignore",
		filepath.Join("foo", "docker-compose.yml"),
		filepath.Join("foo", ".dockerignore"),
	}
	f.assertConfigFiles(expectedConfFiles...)
}

func TestDockerComposeHonorsDockerIgnore(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	df := `FROM alpine

ADD . /app
COPY ./thing.go /stuff
RUN echo hi`
	f.file(filepath.Join("foo", "Dockerfile"), df)

	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	// the build context is ./foo so tmp should be ignored
	f.file(filepath.Join("foo", ".dockerignore"), "tmp")
	// this dockerignore is unrelated despite being a sibling to docker-compose.yml, so won't be used
	f.file(".dockerignore", "foo/tmp2")

	f.load("foo")

	f.assertNextManifest("foo",
		buildMatches(filepath.Join("foo", "tmp2")),
		fileChangeMatches(filepath.Join("foo", "tmp2")),
		buildFilters(filepath.Join("foo", "tmp")),
		fileChangeFilters(filepath.Join("foo", "tmp")),
	)
}

func TestDockerComposeIgnoresFileChangesOnMountedVolumes(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	df := `FROM alpine

ADD . /app
COPY ./thing.go /stuff
RUN echo hi`
	f.file(filepath.Join("foo", "Dockerfile"), df)

	f.file("docker-compose.yml", configWithMounts)
	f.file("Tiltfile", "docker_compose('docker-compose.yml')")

	f.load("foo")

	f.assertNextManifest("foo",
		// ensure that DC syncs are *not* ignored for builds, because all files are still relevant to builds
		buildMatches(filepath.Join("foo", "Dockerfile")),
		// ensure that DC syncs *are* ignored for file watching, i.e., won't trigger builds
		fileChangeFilters(filepath.Join("foo", "blah")),
	)
}

func TestDockerComposeWithDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `docker_build('gcr.io/foo', './foo')
docker_compose('docker-compose.yml')
dc_resource('foo', 'gcr.io/foo')
`)

	f.load()

	m := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	iTarget := m.ImageTargetAt(0)

	// Make sure there's no live update in the default case.
	assert.True(t, iTarget.IsDockerBuild())
	assert.True(t, liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec))

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	assert.Equal(t, m.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
}

func TestDockerComposeWithDockerBuildAutoAssociate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", `version: '3'
services:
  foo:
    image: gcr.io/as_specified_in_config
    build: ./foo
    command: sleep 100
    ports:
      - "12312:80"`)
	f.file("Tiltfile", `docker_build('gcr.io/as_specified_in_config', './foo')
docker_compose('docker-compose.yml')
`)

	f.load()

	// don't need a dc_resource call if the docker_build image matches the
	// `Image` specified in dc.yml
	m := f.assertNextManifest("foo", db(image("gcr.io/as_specified_in_config")))
	iTarget := m.ImageTargetAt(0)

	// Make sure there's no live update in the default case.
	assert.True(t, iTarget.IsDockerBuild())
	assert.True(t, liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec))

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	assert.Equal(t, m.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
}

// I.e. make sure that we handle de/normalization between `fooimage` <--> `docker.io/library/fooimage`
func TestDockerComposeWithDockerBuildLocalRef(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `docker_build('fooimage', './foo')
docker_compose('docker-compose.yml')
dc_resource('foo', 'fooimage')
`)

	f.load()

	m := f.assertNextManifest("foo", db(image("fooimage")))
	assert.True(t, m.ImageTargetAt(0).IsDockerBuild())

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	assert.Equal(t, m.DockerComposeTarget().Spec.Project.ConfigPaths,
		[]string{configPath})
}

func TestMultipleDockerComposeWithDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.dockerfile(filepath.Join("bar", "Dockerfile"))
	f.file("docker-compose.yml", twoServiceConfig)
	f.file("Tiltfile", `docker_build('gcr.io/foo', './foo')
docker_build('gcr.io/bar', './bar')
docker_compose('docker-compose.yml')
dc_resource('foo', 'gcr.io/foo')
dc_resource('bar', 'gcr.io/bar')
`)

	f.load()

	foo := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.True(t, foo.ImageTargetAt(0).IsDockerBuild())

	bar := f.assertNextManifest("bar", db(image("gcr.io/bar")))
	assert.True(t, foo.ImageTargetAt(0).IsDockerBuild())

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	assert.Equal(t, foo.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
	assert.Equal(t, bar.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
}

func TestMultipleDockerComposeWithDockerBuildImageNames(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.dockerfile(filepath.Join("bar", "Dockerfile"))
	config := `version: '3'
services:
  foo:
    image: gcr.io/foo
  bar:
    image: gcr.io/bar
    depends_on: [foo]
`
	f.file("docker-compose.yml", config)
	f.file("Tiltfile", `
docker_build('gcr.io/foo', './foo')
docker_build('gcr.io/bar', './bar')
docker_compose('docker-compose.yml')
`)

	f.load()

	foo := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.True(t, foo.ImageTargetAt(0).IsDockerBuild())

	bar := f.assertNextManifest("bar", db(image("gcr.io/bar")))
	assert.True(t, bar.ImageTargetAt(0).IsDockerBuild())

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	assert.Equal(t, foo.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
	assert.Equal(t, bar.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
}

func TestDCImageRefSuggestion(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("docker-compose.yml", `version: '3'
services:
  foo:
    image: gcr.io/foo
`)
	f.file("Tiltfile", `
docker_build('gcr.typo.io/foo', 'foo')
docker_compose('docker-compose.yml')
`)
	f.loadAssertWarnings(`Image not used in any Docker Compose config:
    ✕ gcr.typo.io/foo
Did you mean…
    - gcr.io/foo
Skipping this image build
If this is deliberate, suppress this warning with: update_settings(suppress_unused_image_warnings=["gcr.typo.io/foo"])`)
}

func TestDockerComposeOnlySomeWithDockerBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", twoServiceConfig)
	f.file("Tiltfile", `img_name = 'gcr.io/foo'
docker_build(img_name, './foo')
docker_compose('docker-compose.yml')
dc_resource('foo', img_name)
`)

	f.load()

	foo := f.assertNextManifest("foo", db(image("gcr.io/foo")))
	assert.True(t, foo.ImageTargetAt(0).IsDockerBuild())

	bar := f.assertNextManifest("bar")
	assert.Empty(t, bar.ImageTargets)

	configPath := f.TempDirFixture.JoinPath("docker-compose.yml")
	assert.Equal(t, foo.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
	assert.Equal(t, bar.DockerComposeTarget().Spec.Project.ConfigPaths, []string{configPath})
}

func TestDockerComposeResourceNoImageMatch(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `docker_build('gcr.io/foo', './foo')
docker_compose('docker-compose.yml')
dc_resource('no-svc-with-this-name-eek', 'gcr.io/foo')
`)
	f.loadErrString("no Docker Compose service found with name")
}

func TestDockerComposeLoadConfigFilesOnFailure(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `docker_build('gcr.io/foo', './foo')
docker_compose('docker-compose.yml')
fail("deliberate exit")
`)
	f.loadErrString("deliberate exit")

	// Make sure that even though tiltfile execution failed, we still
	// loaded config files correctly.
	f.assertConfigFiles(".tiltignore", "Tiltfile", "docker-compose.yml", "foo/Dockerfile")
}

func TestDockerComposeDoesntSupportEntrypointOverride(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `docker_build('gcr.io/foo', './foo', entrypoint='./foo')
docker_compose('docker-compose.yml')
dc_resource('foo', 'gcr.io/foo')
`)

	f.loadErrString("docker_build/custom_build.entrypoint not supported for Docker Compose resources")
}

func TestDefaultRegistryWithDockerCompose(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `
docker_compose('docker-compose.yml')
default_registry('bar.com')
`)

	f.loadErrString("default_registry is not supported with docker compose")
}

func TestDockerComposeLabels(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `
docker_compose('docker-compose.yml')
dc_resource("foo", labels="test")
`)

	f.load("foo")
	f.assertNextManifest("foo", resourceLabels("test"))
}

func TestTriggerModeDC(t *testing.T) {
	for _, testCase := range []struct {
		name                string
		globalSetting       triggerMode
		dcResourceSetting   triggerMode
		specifyAutoInit     bool
		autoInit            bool
		expectedTriggerMode model.TriggerMode
	}{
		{"default", TriggerModeUnset, TriggerModeUnset, false, false, model.TriggerModeAuto},
		{"explicit global auto", TriggerModeAuto, TriggerModeUnset, false, false, model.TriggerModeAuto},
		{"explicit global manual", TriggerModeManual, TriggerModeUnset, false, false, model.TriggerModeManualWithAutoInit},
		{"dc auto", TriggerModeUnset, TriggerModeUnset, false, false, model.TriggerModeAuto},
		{"dc manual", TriggerModeUnset, TriggerModeManual, false, false, model.TriggerModeManualWithAutoInit},
		{"dc manual, auto_init=False", TriggerModeUnset, TriggerModeManual, true, false, model.TriggerModeManual},
		{"dc manual, auto_init=True", TriggerModeUnset, TriggerModeManual, true, true, model.TriggerModeManualWithAutoInit},
		{"dc override auto", TriggerModeManual, TriggerModeAuto, false, false, model.TriggerModeAuto},
		{"dc override manual", TriggerModeAuto, TriggerModeManual, false, false, model.TriggerModeManualWithAutoInit},
		{"dc override manual, auto_init=False", TriggerModeAuto, TriggerModeManual, true, false, model.TriggerModeManual},
		{"dc override manual, auto_init=True", TriggerModeAuto, TriggerModeManual, true, true, model.TriggerModeManualWithAutoInit},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			f.dockerfile(filepath.Join("foo", "Dockerfile"))
			f.file("docker-compose.yml", simpleConfig)

			var globalTriggerModeDirective string
			switch testCase.globalSetting {
			case TriggerModeUnset:
				globalTriggerModeDirective = ""
			default:
				globalTriggerModeDirective = fmt.Sprintf("trigger_mode(%s)", testCase.globalSetting.String())
			}

			var dcResourceDirective string
			switch testCase.dcResourceSetting {
			case TriggerModeUnset:
				dcResourceDirective = ""
			default:
				autoInitOption := ""
				if testCase.specifyAutoInit {
					autoInitOption = ", auto_init="
					if testCase.autoInit {
						autoInitOption += "True"
					} else {
						autoInitOption += "False"
					}
				}
				dcResourceDirective = fmt.Sprintf("dc_resource('foo', trigger_mode=%s%s)", testCase.dcResourceSetting.String(), autoInitOption)
			}

			f.file("Tiltfile", fmt.Sprintf(`
%s
docker_compose('docker-compose.yml')
%s
`, globalTriggerModeDirective, dcResourceDirective))

			f.load()

			f.assertNumManifests(1)
			f.assertNextManifest("foo", testCase.expectedTriggerMode)
		})
	}
}

func TestDCResourceNoImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()
	f.file("docker-compose.yml", simpleConfig)
	f.file("Tiltfile", `
docker_compose('docker-compose.yml')
dc_resource('foo', trigger_mode=TRIGGER_MODE_AUTO)
`)

	f.load()
}

func TestDCDependsOn(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.dockerfile(filepath.Join("foo", "Dockerfile"))
	f.file("docker-compose.yml", twoServiceConfig)
	f.file("Tiltfile", `
docker_compose('docker-compose.yml')
dc_resource('bar', resource_deps=['foo'])
`)

	f.load()
	f.assertNextManifest("foo", resourceDeps())
	f.assertNextManifest("bar", resourceDeps("foo"))
}

func TestDockerComposeVersionWarnings(t *testing.T) {
	type tc struct {
		version string
		warning string
		error   string
	}
	tcs := []tc{
		{version: "v1.28.0", error: "Tilt requires Docker Compose v1.28.3+ (you have v1.28.0). Please upgrade and re-launch Tilt."},
		{version: "v2.0.0-rc.3", warning: "Using Docker Compose v2.0.0-rc.3 (version < 2.2) may result in errors or broken functionality.\n" +
			"For best results, we recommend upgrading to Docker Compose >= v2.2.0."},
		{version: "v1.29.2" /* no errors or warnings */},
		{version: "v2.2.0" /* no errors or warnings */},
		{version: "v1.99.0-beta.4", warning: "You are running a pre-release version of Docker Compose (v1.99.0-beta.4), which is unsupported.\n" +
			"You might encounter errors or broken functionality."},
	}

	for _, tc := range tcs {
		t.Run(tc.version, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			f.dockerfile(filepath.Join("foo", "Dockerfile"))
			f.file("docker-compose.yml", simpleConfig)
			f.file("Tiltfile", "docker_compose('docker-compose.yml')")

			f.load("foo")

			loader := f.newTiltfileLoader()
			if tl, ok := loader.(tiltfileLoader); ok {
				dcCli := dockercompose.NewFakeDockerComposeClient(t, f.ctx)
				dcCli.ConfigOutput = simpleConfig
				dcCli.VersionOutput = semver.Canonical(tc.version)
				tl.dcCli = dcCli
				loader = tl
			} else {
				require.Fail(t, "Could not set up fake Docker Compose client")
			}

			f.loadResult = loader.Load(f.ctx, ctrltiltfile.MainTiltfile(f.JoinPath("Tiltfile"), nil), nil)
			if tc.error == "" {
				require.NoError(t, f.loadResult.Error, "Tiltfile load result had unexpected error")
			} else {
				require.Contains(t, f.loadResult.Error.Error(), tc.error)
			}

			if tc.warning != "" {
				require.Len(t, f.warnings, 1)
				require.Contains(t, f.warnings[0], tc.warning)
			} else {
				require.Empty(t, f.warnings, "Tiltfile load result had unexpected warning(s)")
			}
		})
	}
}

func (f *fixture) assertDcManifest(name model.ManifestName, opts ...interface{}) model.Manifest {
	f.t.Helper()
	m := f.assertNextManifest(name)

	if !m.IsDC() {
		f.t.Error("expected a docker-compose manifest")
	}
	dcInfo := m.DockerComposeTarget()

	for _, opt := range opts {
		switch opt := opt.(type) {
		case dcServiceYAMLHelper:
			assert.YAMLEq(f.t, opt.yaml, dcInfo.ServiceYAML, "docker compose YAML")
		case noImageHelper:
			assert.Empty(f.t, m.ImageTargets, "Manifest should have had no ImageTargets")
		case dockerComposeImageHelper:
			ok, iTarget := assertImageTargetType(f.t, m.ImageTargets, model.DockerComposeBuild{})
			if ok {
				assert.Equal(f.t, opt.buildContext, iTarget.DockerComposeBuildInfo().Context,
					"Build context path did not match")
			}
		case dcPublishedPortsHelper:
			assert.Equal(f.t, opt.ports, dcInfo.PublishedPorts(), "docker compose published ports")
		default:
			f.t.Fatalf("unexpected arg to assertDcManifest: %T %v", opt, opt)
		}
	}
	return m
}

func assertImageTargetType(t *testing.T, iTargets []model.ImageTarget,
	buildDetailsType interface{}) (bool, model.ImageTarget) {
	t.Helper()
	if !assert.Len(t, iTargets, 1, "Manifest should have exactly one image target") {
		return false, model.ImageTarget{}
	}
	if !assert.IsType(t, buildDetailsType, iTargets[0].BuildDetails, "BuildDetails was not of expected type") {
		return false, model.ImageTarget{}
	}
	return true, iTargets[0]
}

type dcServiceYAMLHelper struct {
	yaml string
}

func dcServiceYAML(yaml string) dcServiceYAMLHelper {
	return dcServiceYAMLHelper{yaml}
}

type dockerComposeImageHelper struct {
	dfPath       string
	buildContext string
}

func dockerComposeManagedImage(dfPath string, buildContext string) dockerComposeImageHelper {
	return dockerComposeImageHelper{
		dfPath:       dfPath,
		buildContext: buildContext,
	}
}

type noImageHelper struct{}

func noImage() noImageHelper {
	return noImageHelper{}
}

type dcPublishedPortsHelper struct {
	ports []int
}

func dcPublishedPorts(ports ...int) dcPublishedPortsHelper {
	return dcPublishedPortsHelper{ports: ports}
}

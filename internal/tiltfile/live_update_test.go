package tiltfile

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestLiveUpdateStepNotUsed(t *testing.T) {
	f := newFixture(t)

	f.WriteFile("Tiltfile", "restart_container()")

	f.loadErrString("steps that were created but not used in a live_update", "restart_container", "Tiltfile:1")
}

func TestLiveUpdateRestartContainerNotLast(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    restart_container(),
    sync('foo', '/baz'),
  ]
)`)
	f.loadErrString("live_update", "restart container is only valid as the last step")
}

func TestLiveUpdateSyncRelDest(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    sync('foo', 'baz'),
  ]
)`)
	f.loadErrString("sync destination", "baz", "is not absolute")
}

func TestLiveUpdateRunBeforeSync(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
	run('quu'),
    sync('foo', '/baz'),
  ]
)`)
	f.loadErrString("live_update", "all sync steps must precede all run steps")
}

func TestLiveUpdateNonStepInSteps(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    'quu',
    sync('bar', '/baz'),
  ]
)`)
	f.loadErrString("'steps' must be a list of live update steps - got value '\"quu\"' of type 'string'")
}

func TestLiveUpdateNonStringInFullBuildTriggers(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
	fall_back_on(4),
    sync('bar', '/baz'),
  ],
)`)
	f.loadErrString("fall_back_on",
		"fall_back_on: for parameter paths: value should be a string or List or Tuple of strings, but is of type int")
}

func TestLiveUpdateNonStringInRunTriggers(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    run('bar', [4]),
  ]
)`)
	f.loadErrString("run", "triggers", "'bar'", "contained value '4' of type 'int'. it may only contain strings")
}

func TestLiveUpdateDockerBuildUnqualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)

	f.tiltfileCode = "docker_build('foo', 'foo', live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", db(image("foo"), f.expectedLU))
}

func TestLiveUpdateDockerBuildQualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)

	f.expectedImage = "gcr.io/foo"
	f.tiltfileCode = "docker_build('gcr.io/foo', 'foo', live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", db(image("gcr.io/foo"), f.expectedLU))
}

func TestLiveUpdateDockerBuildDefaultRegistry(t *testing.T) {
	f := newLiveUpdateFixture(t)

	f.tiltfileCode = `
default_registry('gcr.io')
docker_build('foo', 'foo', live_update=%s)`
	f.init()

	f.load("foo")

	i := image("foo")
	i.localRef = "gcr.io/foo"
	f.assertNextManifest("foo", db(i, f.expectedLU))
}

func TestLiveUpdateCustomBuild(t *testing.T) {
	f := newLiveUpdateFixture(t)

	f.tiltfileCode = "custom_build('foo', 'docker build -t $TAG foo', ['foo'], live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", cb(image("foo"), f.expectedLU))
}

func TestLiveUpdateOnlyCustomBuild(t *testing.T) {
	f := newLiveUpdateFixture(t)

	f.tiltfileCode = `
default_registry('gcr.io/myrepo')
custom_build('foo', ':', ['foo'], live_update=%s)
`
	f.init()

	f.load("foo")

	m := f.assertNextManifest("foo", cb(image("foo"), f.expectedLU))
	assert.True(t, m.ImageTargets[0].IsLiveUpdateOnly)

	require.NoError(t, m.InferLiveUpdateSelectors(), "Failed to infer Live Update selectors")
	luSpec := m.ImageTargets[0].LiveUpdateSpec
	require.NotNil(t, luSpec.Selector.Kubernetes)
	assert.Empty(t, luSpec.Selector.Kubernetes.ContainerName)
	// NO registry rewriting should be applied here because Tilt isn't actually building the image
	assert.Equal(t, "foo", luSpec.Selector.Kubernetes.Image)
}

func TestLiveUpdateSyncFilesOutsideOfDockerBuildContext(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    sync('bar', '/baz'),
  ]
)`)
	f.loadErrString("sync step source", f.JoinPath("bar"), f.JoinPath("foo"), "child", "any watched filepaths")
}

func TestLiveUpdateSyncFilesImageDep(t *testing.T) {
	f := newFixture(t)

	f.gitInit("")
	f.file("a/message.txt", "message")
	f.file("imageA.dockerfile", `FROM golang:1.10
ADD message.txt /src/message.txt
`)
	f.file("imageB.dockerfile", "FROM gcr.io/image-a")
	f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-b")))
	f.file("Tiltfile", `
docker_build('gcr.io/image-b', 'b', dockerfile='imageB.dockerfile',
             live_update=[
               sync('a/message.txt', '/src/message.txt'),
             ])
docker_build('gcr.io/image-a', 'a', dockerfile='imageA.dockerfile')
k8s_yaml('foo.yaml')
`)
	f.load()

	lu := v1alpha1.LiveUpdateSpec{
		BasePath: f.Path(),
		Syncs: []v1alpha1.LiveUpdateSync{
			v1alpha1.LiveUpdateSync{
				LocalPath:     filepath.Join("a", "message.txt"),
				ContainerPath: "/src/message.txt",
			},
		},
	}

	f.assertNextManifest("foo",
		db(image("gcr.io/image-a")),
		db(image("gcr.io/image-b"), lu))
}

func TestLiveUpdateRun(t *testing.T) {
	for _, tc := range []struct {
		name         string
		tiltfileText string
		expectedArgv []string
	}{
		{"string cmd", `"echo hi"`, []string{"sh", "-c", "echo hi"}},
		{"array cmd", `["echo", "hi"]`, []string{"echo", "hi"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)

			f.gitInit("")
			f.yaml("foo.yaml", deployment("foo", image("gcr.io/image-a")))
			f.file("imageA.dockerfile", `FROM golang:1.10`)
			f.file("Tiltfile", fmt.Sprintf(`
docker_build('gcr.io/image-a', 'a', dockerfile='imageA.dockerfile',
             live_update=[
               run(%s)
             ])
k8s_yaml('foo.yaml')
`, tc.tiltfileText))
			f.load()

			lu := v1alpha1.LiveUpdateSpec{
				BasePath: f.Path(),
				Execs: []v1alpha1.LiveUpdateExec{
					v1alpha1.LiveUpdateExec{
						Args: tc.expectedArgv,
					},
				},
			}
			f.assertNextManifest("foo",
				db(image("gcr.io/image-a"), lu))
		})
	}
}

func TestLiveUpdateFallBackTriggersOutsideOfDockerBuildContext(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    fall_back_on('bar'),
    sync('foo/bar', '/baz'),
  ]
)`)
	f.loadErrString("fall_back_on", f.JoinPath("bar"), f.JoinPath("foo"), "child", "any watched filepaths")
}

func TestLiveUpdateSyncFilesOutsideOfCustomBuildDeps(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
custom_build('gcr.io/foo', 'docker build -t $TAG foo', ['./foo'],
  live_update=[
    sync('bar', '/baz'),
  ]
)`)
	f.loadErrString("sync step source", f.JoinPath("bar"), f.JoinPath("foo"), "child", "any watched filepaths")
}

func TestLiveUpdateFallBackTriggersOutsideOfCustomBuildDeps(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
custom_build('gcr.io/foo', 'docker build -t $TAG foo', ['./foo'],
  live_update=[
    fall_back_on('bar'),
    sync('foo/bar', '/baz'),
  ]
)`)
	f.loadErrString("fall_back_on", f.JoinPath("bar"), f.JoinPath("foo"), "child", "any watched filepaths")
}

func TestLiveUpdateRestartContainerDeprecationErrorK8s(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', './foo',
  live_update=[
    sync('foo/bar', '/baz'),
	restart_container(),
  ]
)`)
	f.loadErrString(restartContainerDeprecationError([]model.ManifestName{"foo"}))
}

func TestLiveUpdateRestartContainerDeprecationErrorK8sCustomBuild(t *testing.T) {
	f := newFixture(t)

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
custom_build('gcr.io/foo', 'docker build -t $TAG foo', ['./foo'],
  live_update=[
    sync('foo/bar', '/baz'),
	restart_container(),
  ]
)`)

	f.loadErrString(restartContainerDeprecationError([]model.ManifestName{"foo"}))
}

func TestLiveUpdateRestartContainerDeprecationErrorMultiple(t *testing.T) {
	f := newFixture(t)

	f.setupExpand()

	f.file("Tiltfile", `
k8s_yaml('all.yaml')
docker_build('gcr.io/a', './a',
  live_update=[
    sync('./a', '/'),
	restart_container(),
  ]
)
docker_build('gcr.io/b', './b')
docker_build('gcr.io/c', './c',
  live_update=[
    sync('./c', '/'),
	restart_container(),
  ]
)
docker_build('gcr.io/d', './d',
  live_update=[sync('./d', '/')]
)`)

	f.loadErrString(restartContainerDeprecationError([]model.ManifestName{"a", "c"}))
}

func TestLiveUpdateNoRestartContainerDeprecationErrorK8sDockerCompose(t *testing.T) {
	f := newFixture(t)
	f.setupFoo()
	f.file("docker-compose.yml", `version: '3'
services:
  foo:
    image: gcr.io/foo
`)
	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
docker_compose('docker-compose.yml')
`)

	// Expect no deprecation error b/c restart_container() is still allowed on Docker Compose resources
	f.load()
	f.assertNextManifest("foo", db(image("gcr.io/foo")))
}

type liveUpdateFixture struct {
	*fixture

	tiltfileCode  string
	expectedImage string
	expectedLU    v1alpha1.LiveUpdateSpec

	skipYAML bool
}

func (f *liveUpdateFixture) init() {
	f.dockerfile("foo/Dockerfile")

	if !f.skipYAML {
		f.yaml("foo.yaml", deployment("foo", image(f.expectedImage)))
	}

	luSteps := `[
    fall_back_on(['foo/i', 'foo/j']),
	sync('foo/b', '/c'),
	run('f', ['g', 'h']),
]`
	codeToInsert := fmt.Sprintf(f.tiltfileCode, luSteps)

	var tiltfile string
	if !f.skipYAML {
		tiltfile = `k8s_yaml('foo.yaml')`
	}
	tiltfile = strings.Join([]string{tiltfile, codeToInsert}, "\n")
	f.file("Tiltfile", tiltfile)
}

func newLiveUpdateFixture(t *testing.T) *liveUpdateFixture {
	f := &liveUpdateFixture{
		fixture: newFixture(t),
	}

	f.expectedLU = v1alpha1.LiveUpdateSpec{
		BasePath:  f.Path(),
		StopPaths: []string{filepath.Join("foo", "i"), filepath.Join("foo", "j")},
		Syncs: []v1alpha1.LiveUpdateSync{
			v1alpha1.LiveUpdateSync{
				LocalPath:     filepath.Join("foo", "b"),
				ContainerPath: "/c",
			},
		},
		Execs: []v1alpha1.LiveUpdateExec{
			v1alpha1.LiveUpdateExec{
				Args:         []string{"sh", "-c", "f"},
				TriggerPaths: []string{"g", "h"},
			},
		},
	}
	f.expectedImage = "foo"

	return f
}

package tiltfile

import (
	"fmt"
	"testing"

	"github.com/windmilleng/tilt/internal/model"
)

func TestLiveUpdateStepNotUsed(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", "restart_container()")
	f.loadErrString("not used by any live_update", "restart_container", "<builtin>:1")
}

func TestLiveUpdateRestartContainerNotLast(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

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
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    sync('foo', 'baz'),
  ]
)`)
	f.loadErrString("sync destination", "'baz'", "is not absolute")
}

func TestLiveUpdateRunBeforeSync(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

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
	defer f.TearDown()

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
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
	fall_back_on(4),
    sync('bar', '/baz'),
  ],
)`)
	f.loadErrString("fall_back_on", "value '4' of type 'int'")
}

func TestLiveUpdateNonStringInRunTriggers(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

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

func TestLiveUpdateSyncFilesOutsideOfDockerContext(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    sync('bar', '/baz'),
  ]
)`)
	f.loadErrString("sync step source", f.JoinPath("bar"), f.JoinPath("foo"), "child", "docker build context")
}

func TestLiveUpdateDockerBuildUnqualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "docker_build('foo', 'foo', live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", db(imageNormalized("foo"), f.expectedLU))
}

func TestLiveUpdateDockerBuildQualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "docker_build('gcr.io/foo', 'foo', live_update=%s)"
	f.configuredImageName = "gcr.io/foo"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", db(image("gcr.io/foo"), f.expectedLU))
}

func TestLiveUpdateDockerBuildDefaultRegistry(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = `
default_registry('gcr.io')
docker_build('foo', 'foo', live_update=%s)`
	f.init()

	f.load("foo")

	i := imageNormalized("foo")
	i.deploymentRef = "gcr.io/foo"
	f.assertNextManifest("foo", db(i, f.expectedLU))
}

func TestLiveUpdateCustomBuild(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "custom_build('foo', 'docker build -t $TAG foo', ['foo'], live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", cb(imageNormalized("foo"), f.expectedLU))
}

func TestLiveUpdateRebuildTriggersOutsideOfDockerContext(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
k8s_yaml('foo.yaml')
docker_build('gcr.io/foo', 'foo',
  live_update=[
    fall_back_on('bar'),
    sync('foo/bar', '/baz'),
  ]
)`)
	f.loadErrString("fall_back_on", f.JoinPath("bar"), f.JoinPath("foo"), "child", "docker build context")
}

type liveUpdateFixture struct {
	*fixture

	tiltfileCode        string
	configuredImageName string
	expectedLU          model.LiveUpdate
}

func (f *liveUpdateFixture) init() {
	f.dockerfile("foo/Dockerfile")
	f.yaml("foo.yaml", deployment("foo", image(f.configuredImageName)))

	luSteps := `[
    fall_back_on(['foo/i', 'foo/j']),
	sync('foo/b', '/c'),
	run('f', ['g', 'h']),
	restart_container(),
]`
	codeToInsert := fmt.Sprintf(f.tiltfileCode, luSteps)
	tiltfile := fmt.Sprintf(`
k8s_yaml('foo.yaml')
%s
`, codeToInsert)
	f.file("Tiltfile", tiltfile)
}

func newLiveUpdateFixture(t *testing.T) *liveUpdateFixture {
	f := &liveUpdateFixture{
		fixture:             newFixture(t),
		configuredImageName: "foo",
	}

	var steps []model.LiveUpdateStep

	steps = append(steps,
		model.LiveUpdateSyncStep{Source: f.JoinPath("foo", "b"), Dest: "/c"},
		model.LiveUpdateRunStep{
			Command:  model.ToShellCmd("f"),
			Triggers: f.NewPathSet("g", "h"),
		},
		model.LiveUpdateRestartContainerStep{},
	)

	f.expectedLU = model.LiveUpdate{
		Steps:               steps,
		FullRebuildTriggers: f.NewPathSet("foo/i", "foo/j"),
	}

	return f
}

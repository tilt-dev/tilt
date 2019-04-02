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
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    restart_container(),
    sync('foo', '/baz'),
  ])`)
	f.loadErrString("image build info for foo", "live_update", "restart container is only valid as the last step")
}

func TestLiveUpdateSyncRelDest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    sync('foo', 'baz'),
  ])`)
	f.loadErrString("sync destination", "'baz'", "is not absolute")
}

func TestLiveUpdateRunBeforeSync(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
	run('quu'),
    sync('foo', '/baz'),
  ])`)
	f.loadErrString("image build info for foo", "live_update", "all sync steps must precede all run steps")
}

func TestLiveUpdateWithNoCorrespondingBuild(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/bar',
  [
    restart_container(),
    sync('bar', '/baz'),
  ])`)
	f.loadErrString("live_update was specified for 'gcr.io/bar', but no built resource uses that image")
}

func TestLiveUpdateNonStepInSteps(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/bar',
  [
    'quu',
    sync('bar', '/baz'),
  ])`)
	f.loadErrString("'steps' must be a list of live update steps - got value '\"quu\"' of type 'string'")
}

func TestLiveUpdateNonStringInFullBuildTriggers(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/bar',
  [
    sync('bar', '/baz'),
  ],
  4)`)
	f.loadErrString("'full_rebuild_triggers' must only contain strings - got value '4' of type 'int'")
}

func TestLiveUpdateNonStringInRunTriggers(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/bar',
  [
    run('bar', [4]),
  ])`)
	f.loadErrString("run", "triggers", "'bar'", "contained value '4' of type 'int'. it may only contain strings")
}

func TestLiveUpdateSyncFilesOutsideOfDockerContext(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    sync('bar', '/baz'),
  ])`)
	f.loadErrString("sync step source", f.JoinPath("bar"), f.JoinPath("foo"), "child", "docker build context")
}

func TestLiveUpdateDockerBuildUnqualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "docker_build('foo', 'foo')"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", db(imageNormalized("foo"), f.expectedLU))
}

func TestLiveUpdateDockerBuildQualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "docker_build('gcr.io/foo', 'foo')"
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
docker_build('foo', 'foo')`
	f.init()

	f.load("foo")

	i := imageNormalized("foo")
	i.deploymentRef = "gcr.io/foo"
	f.assertNextManifest("foo", db(i, f.expectedLU))
}

func TestLiveUpdateCustomBuild(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "custom_build('foo', 'docker build -t $TAG foo', ['foo'])"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", cb(imageNormalized("foo"), f.expectedLU))
}

func TestLiveUpdateRebuildTriggersOutsideOfDockerContext(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [sync('foo/bar', '/baz')],
  ['bar'],
)`)
	f.loadErrString("full_rebuild_trigger", f.JoinPath("bar"), f.JoinPath("foo"), "child", "docker build context")
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

	tiltfile := fmt.Sprintf(`
%s
k8s_resource('foo', 'foo.yaml')
live_update('%s',
  [
	sync('foo/b', '/c'),
	run('f', ['g', 'h']),
	restart_container(),
  ],
  ['foo/i', 'foo/j']
)`, f.tiltfileCode, f.configuredImageName)
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

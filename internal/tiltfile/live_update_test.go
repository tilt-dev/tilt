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

func TestLiveUpdateBuild(t *testing.T) {
	type testCase struct {
		name, buildCmd string
		assert         func(f *fixture, expectedLu model.LiveUpdate)
	}

	dbManifestOpt := func(f *fixture, expectedLu model.LiveUpdate) {
		f.assertNextManifest("foo", db(image("gcr.io/foo"), expectedLu))
	}
	cbManifestOpt := func(f *fixture, expectedLu model.LiveUpdate) {
		f.assertNextManifest("foo", cb(image("gcr.io/foo"), expectedLu))
	}

	tests := []testCase{
		{"docker_build", "docker_build('gcr.io/foo', 'foo')", dbManifestOpt},
		{"custom_build", "custom_build('gcr.io/foo', 'docker build -t $TAG foo', ['foo'])", cbManifestOpt},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			f.setupFoo()

			tiltfile := fmt.Sprintf(`
%s
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
	sync('foo/b', '/c'),
	run('f', ['g', 'h']),
	restart_container(),
  ],
  ['i', 'j']
)`, test.buildCmd)

			f.file("Tiltfile", tiltfile)

			f.load("foo")

			f.assertNumManifests(1)

			var steps []model.LiveUpdateStep

			steps = append(steps,
				model.LiveUpdateSyncStep{Source: f.JoinPath("foo/b"), Dest: "/c"},
				model.LiveUpdateRunStep{
					Command:  model.ToShellCmd("f"),
					Triggers: []string{f.JoinPath("g"), f.JoinPath("h")},
				},
				model.LiveUpdateRestartContainerStep{},
			)

			lu := model.LiveUpdate{
				Steps:               steps,
				FullRebuildTriggers: []string{f.JoinPath("i"), f.JoinPath("j")},
			}
			test.assert(f, lu)
		})
	}
}

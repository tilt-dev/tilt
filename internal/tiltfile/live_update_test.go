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

func TestRestartContainerNotLast(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    restart_container(),
    sync('bar', '/baz'),
  ])`)
	f.loadErrString("image build info for foo", "live_update", "restart container is only valid as the last step")
}

func TestWorkDirNotFirst(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    sync('bar', '/baz'),
    work_dir('/quu'),
  ])`)
	f.loadErrString("image build info for foo", "live_update", "workdir is only valid as the first step")
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
    sync('bar', '/baz'),
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

func TestLiveUpdateNonAbsWorkDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", "work_dir('foo')")
	f.loadErrString("'foo'", "is not absolute", "work_dir")
}

func TestLiveUpdateNonAbsSyncDestFollowingWorkDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
docker_build('gcr.io/foo', 'foo')
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    work_dir('/baz'),
    sync('foo', 'bar'),
  ])`)
	f.load("foo")
}

func TestLiveUpdateNonAbsSyncDestNotFollowingWorkDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFoo()

	f.file("Tiltfile", `
live_update('gcr.io/foo',
  [
    sync('foo', 'bar'),
  ])`)
	f.loadErrString("follow a work_dir or have an absolute remote_path", fmt.Sprintf("'%s'", f.JoinPath("foo")), "'bar'")
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

			f.file("Tiltfile", fmt.Sprintf(`
%s
k8s_resource('foo', 'foo.yaml')
live_update('gcr.io/foo',
  [
    work_dir('/a'),
	sync('b', '/c'),
	sync('d', '/e'),
	run('f', ['g', 'h']),
	restart_container(),
  ],
  ['i', 'j']
)`, test.buildCmd))

			f.load("foo")

			f.assertNumManifests(1)

			lu := model.LiveUpdate{
				Steps: []model.LiveUpdateStep{
					model.LiveUpdateWorkDirStep("/a"),
					model.LiveUpdateSyncStep{Source: f.JoinPath("b"), Dest: "/c"},
					model.LiveUpdateSyncStep{Source: f.JoinPath("d"), Dest: "/e"},
					model.LiveUpdateRunStep{Command: model.ToShellCmd("f"), Triggers: []string{f.JoinPath("g"), f.JoinPath("h")}},
					model.LiveUpdateRestartContainerStep{},
				},
				FullRebuildTriggers: []string{f.JoinPath("i"), f.JoinPath("j")},
			}
			test.assert(f, lu)
		})
	}
}

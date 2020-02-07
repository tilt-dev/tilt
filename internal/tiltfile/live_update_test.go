package tiltfile

import (
	"fmt"
	"testing"

	"github.com/windmilleng/tilt/pkg/model"
)

func TestLiveUpdateStepNotUsed(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.WriteFile("Tiltfile", "restart_container()")

	f.loadErrString("steps that were created but not used in a live_update", "restart_container", "Tiltfile:1")
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

func TestLiveUpdateDockerBuildUnqualifiedImageName(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "docker_build('foo', 'foo', live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", db(image("foo"), f.expectedLU))
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

	i := image("foo")
	i.localRef = "gcr.io/foo"
	f.assertNextManifest("foo", db(i, f.expectedLU))
}

func TestLiveUpdateCustomBuild(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.tiltfileCode = "custom_build('foo', 'docker build -t $TAG foo', ['foo'], live_update=%s)"
	f.init()

	f.load("foo")

	f.assertNextManifest("foo", cb(image("foo"), f.expectedLU))
}

func TestLiveUpdateSyncFilesOutsideOfDockerBuildContext(t *testing.T) {
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
	f.loadErrString("sync step source", f.JoinPath("bar"), f.JoinPath("foo"), "child", "any watched filepaths")
}

func TestLiveUpdateSyncFilesImageDep(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

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

	lu := model.LiveUpdate{
		Steps: []model.LiveUpdateStep{
			model.LiveUpdateSyncStep{
				Source: f.JoinPath("a/message.txt"),
				Dest:   "/src/message.txt",
			},
		},
		BaseDir: f.Path(),
	}
	f.assertNextManifest("foo",
		db(image("gcr.io/image-a")),
		db(image("gcr.io/image-b"), lu))
}

func TestLiveUpdateRun(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	for _, tc := range []struct {
		name         string
		tiltfileText string
		expectedCmd  model.Cmd
	}{
		{"string cmd", `"echo hi"`, model.ToShellCmd("echo hi")},
		{"array cmd", `["echo", "hi"]`, model.Cmd{Argv: []string{"echo", "hi"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
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

			lu := model.LiveUpdate{
				Steps: []model.LiveUpdateStep{
					model.LiveUpdateRunStep{
						Command:  tc.expectedCmd,
						Triggers: model.NewPathSet(nil, f.Path()),
					},
				},
				BaseDir: f.Path(),
			}
			f.assertNextManifest("foo",
				db(image("gcr.io/image-a"), lu))
		})
	}
}

func TestLiveUpdateFallBackTriggersOutsideOfDockerBuildContext(t *testing.T) {
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
	f.loadErrString("fall_back_on", f.JoinPath("bar"), f.JoinPath("foo"), "child", "any watched filepaths")
}

func TestLiveUpdateSyncFilesOutsideOfCustomBuildDeps(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

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
	defer f.TearDown()

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
		model.LiveUpdateFallBackOnStep{
			Files: []string{f.JoinPath("foo/i"), f.JoinPath("foo/j")},
		},
		model.LiveUpdateSyncStep{Source: f.JoinPath("foo", "b"), Dest: "/c"},
		model.LiveUpdateRunStep{
			Command:  model.ToShellCmd("f"),
			Triggers: model.NewPathSet([]string{"g", "h"}, f.Path()),
		},
		model.LiveUpdateRestartContainerStep{},
	)

	f.expectedLU = model.LiveUpdate{
		Steps:   steps,
		BaseDir: f.Path(),
	}

	return f
}

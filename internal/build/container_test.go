//+build !skipcontainertests

// Tests that involve spinning up/interacting with actual containers
package build

import (
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"
)

// * * * IMAGE BUILDER * * *

func TestStaticDockerfile(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := dockerfile.Dockerfile(`
FROM alpine
WORKDIR /src
ADD a.txt .
RUN cp a.txt b.txt
ADD dir/c.txt .
`)

	f.WriteFile("a.txt", "a")
	f.WriteFile("dir/c.txt", "c")
	f.WriteFile("missing.txt", "missing")

	ref, err := f.b.BuildDockerfile(f.ctx, f.ps, f.getNameFromTest(), df, f.Path(), model.EmptyMatcher, model.DockerBuildArgs{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/a.txt", Contents: "a"},
		expectedFile{Path: "/src/b.txt", Contents: "a"},
		expectedFile{Path: "/src/c.txt", Contents: "c"},
		expectedFile{Path: "/src/dir/c.txt", Missing: true},
		expectedFile{Path: "/src/missing.txt", Missing: true},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestStaticDockerfileWithBuildArgs(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := dockerfile.Dockerfile(`FROM alpine
ARG some_variable_name

ADD $some_variable_name /test.txt`)

	f.WriteFile("awesome_variable", "hi im an awesome variable")

	ba := model.DockerBuildArgs{
		"some_variable_name": "awesome_variable",
	}
	ref, err := f.b.BuildDockerfile(f.ctx, f.ps, f.getNameFromTest(), df, f.Path(), model.EmptyMatcher, ba)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "/test.txt", Contents: "hi im an awesome variable"},
	}
	f.assertFilesInImage(ref, expected)
}

func TestMount(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	// write some files in to it
	f.WriteFile("hi/hello", "hi hello")
	f.WriteFile("sup", "my name is dan")

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/hi/hello", Contents: "hi hello"},
		expectedFile{Path: "/src/sup", Contents: "my name is dan"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestMountFileToDirectory(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("sup", "my name is dan")

	m := model.Mount{
		LocalPath:     f.JoinPath("sup"),
		ContainerPath: "/src/",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/sup", Contents: "my name is dan"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestMultipleMounts(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	// write some files in to it
	f.WriteFile("hi/hello", "hi hello")
	f.WriteFile("bye/ciao/goodbye", "bye laterz")

	m1 := model.Mount{
		LocalPath:     f.JoinPath("hi"),
		ContainerPath: "/hello_there",
	}
	m2 := model.Mount{
		LocalPath:     f.JoinPath("bye"),
		ContainerPath: "goodbye_there",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m1, m2}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/hello_there/hello", Contents: "hi hello"},
		expectedFile{Path: "/goodbye_there/ciao/goodbye", Contents: "bye laterz"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestMountCollisions(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	// write some files in to it
	f.WriteFile("hi/hello", "hi hello")
	f.WriteFile("bye/hello", "bye laterz")

	// Mounting two files to the same place in the container -- expect the second mount
	// to take precedence (file should contain "bye laterz")
	m1 := model.Mount{
		LocalPath:     f.JoinPath("hi"),
		ContainerPath: "/hello_there",
	}
	m2 := model.Mount{
		LocalPath:     f.JoinPath("bye"),
		ContainerPath: "/hello_there",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m1, m2}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/hello_there/hello", Contents: "bye laterz"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestPush(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.startRegistry()

	// write some files in to it
	f.WriteFile("hi/hello", "hi hello")
	f.WriteFile("sup", "my name is dan")

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}

	name, err := reference.WithName("localhost:5005/myimage")
	if err != nil {
		t.Fatal(err)
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, name, simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	namedTagged, err := f.b.PushImage(f.ctx, ref, ioutil.Discard)
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/hi/hello", Contents: "hi hello"},
		expectedFile{Path: "/src/sup", Contents: "my name is dan"},
	}

	f.assertFilesInImage(namedTagged, pcs)
}

func TestPushInvalid(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}
	name, err := reference.WithName("localhost:5005/myimage")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, name, simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.b.PushImage(f.ctx, ref, ioutil.Discard)
	msg := `pushing image "localhost:5005/myimage"`
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expected error containing %q, actual: %v", msg, err)
	}
}

func TestBuildOneStep(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := model.ToSteps(f.Path(), []model.Cmd{
		model.ToShellCmd("echo -n hello >> hi"),
	})

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, model.EmptyMatcher, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "hi", Contents: "hello"},
	}
	f.assertFilesInImage(ref, expected)
}

func TestBuildMultipleSteps(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := model.ToSteps(f.Path(), []model.Cmd{
		model.ToShellCmd("echo -n hello >> hi"),
		model.ToShellCmd("echo -n sup >> hi2"),
	})

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, model.EmptyMatcher, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "hi", Contents: "hello"},
		expectedFile{Path: "hi2", Contents: "sup"},
	}
	f.assertFilesInImage(ref, expected)
}

func TestBuildMultipleStepsRemoveFiles(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := model.ToSteps(f.Path(), []model.Cmd{
		model.Cmd{Argv: []string{"sh", "-c", "echo -n hello >> hi"}},
		model.Cmd{Argv: []string{"sh", "-c", "echo -n sup >> hi2"}},
		model.Cmd{Argv: []string{"sh", "-c", "rm hi"}},
	})

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, model.EmptyMatcher, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "hi2", Contents: "sup"},
		expectedFile{Path: "hi", Missing: true},
	}
	f.assertFilesInImage(ref, expected)
}

func TestBuildFailingStep(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := model.ToSteps(f.Path(), []model.Cmd{
		model.ToShellCmd("echo hello && exit 1"),
	})

	_, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, model.EmptyMatcher, steps, model.Cmd{})
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "hello")

		// Different versions of docker have a different error string
		hasExitCode1 := strings.Contains(err.Error(), "exit code 1") ||
			strings.Contains(err.Error(), "returned a non-zero code: 1") ||
			strings.Contains(err.Error(), "exit code: 1")
		if !hasExitCode1 {
			t.Errorf("Expected failure with exit code 1, actual: %v", err)
		}
	}
}

func TestEntrypoint(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	entrypoint := model.ToShellCmd("echo -n hello >> hi")
	d, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, nil, model.EmptyMatcher, nil, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "hi", Contents: "hello"},
	}

	// Start container WITHOUT overriding entrypoint (which assertFilesInImage... does)
	cID := f.startContainer(f.ctx, containerConfig(d))
	f.assertFilesInContainer(f.ctx, cID, expected)
}

func TestDockerfileWithEntrypointPermitted(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := dockerfile.Dockerfile(`FROM alpine
ENTRYPOINT ["sleep", "100000"]`)

	_, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), df, nil, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

// TODO(maia): test mount err cases
// TODO(maia): tests for tar code

func TestSelectiveAddFilesToExisting(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("hi/hello", "hi hello")
	f.WriteFile("sup", "we should delete this file")
	f.WriteFile("nested/sup", "we should delete this file (and the whole dir)")
	f.WriteFile("unchanged", "should be unchanged")
	mounts := []model.Mount{
		model.Mount{
			LocalPath:     f.Path(),
			ContainerPath: "/src",
		},
	}

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, mounts, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	f.WriteFile("hi/hello", "hello world") // change contents
	f.Rm("sup")                            // delete a file
	f.Rm("nested")                         // delete a directory
	files := []string{"hi/hello", "sup", "nested"}
	pms, err := FilesToPathMappings(f.JoinPaths(files), mounts)
	if err != nil {
		f.t.Fatal("FilesToPathMappings:", err)
	}

	ref, err := f.b.BuildImageFromExisting(f.ctx, f.ps, existing, pms, model.EmptyMatcher, nil)
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/hi/hello", Contents: "hello world"},
		expectedFile{Path: "/src/sup", Missing: true},
		expectedFile{Path: "/src/nested/sup", Missing: true}, // should have deleted whole directory
		expectedFile{Path: "/src/unchanged", Contents: "should be unchanged"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestExecStepsOnExisting(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("foo", "hello world")
	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	step := model.ToShellCmd("echo -n foo contains: $(cat /src/foo) >> /src/bar")

	steps := model.ToSteps(f.Path(), []model.Cmd{step})
	ref, err := f.b.BuildImageFromExisting(f.ctx, f.ps, existing, MountsToPathMappings([]model.Mount{m}), model.EmptyMatcher, steps)
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/foo", Contents: "hello world"},
		expectedFile{Path: "/src/bar", Contents: "foo contains: hello world"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestBuildImageFromExistingPreservesEntrypoint(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("foo", "hello world")
	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}
	entrypoint := model.ToShellCmd("echo -n foo contains: $(cat /src/foo) >> /src/bar")

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, nil, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	// change contents of `foo` so when entrypoint exec's the second time, it
	// will change the contents of `bar`
	f.WriteFile("foo", "a whole new world")

	ref, err := f.b.BuildImageFromExisting(f.ctx, f.ps, existing, MountsToPathMappings([]model.Mount{m}), model.EmptyMatcher, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "/src/foo", Contents: "a whole new world"},
		expectedFile{Path: "/src/bar", Contents: "foo contains: a whole new world"},
	}

	// Start container WITHOUT overriding entrypoint (which assertFilesInImage... does)
	cID := f.startContainer(f.ctx, containerConfig(ref))
	f.assertFilesInContainer(f.ctx, cID, expected)
}

func TestBuildDockerWithStepsFromExistingPreservesEntrypoint(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("foo", "hello world")
	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}
	step := model.ToShellCmd("echo -n hello >> /src/baz")
	entrypoint := model.ToShellCmd("echo -n foo contains: $(cat /src/foo) >> /src/bar")

	steps := model.ToSteps(f.Path(), []model.Cmd{step})
	existing, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, steps, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	// change contents of `foo` so when entrypoint exec's the second time, it
	// will change the contents of `bar`
	f.WriteFile("foo", "a whole new world")

	ref, err := f.b.BuildImageFromExisting(f.ctx, f.ps, existing, MountsToPathMappings([]model.Mount{m}), model.EmptyMatcher, steps)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "/src/foo", Contents: "a whole new world"},
		expectedFile{Path: "/src/bar", Contents: "foo contains: a whole new world"},
		expectedFile{Path: "/src/baz", Contents: "hellohello"},
	}

	// Start container WITHOUT overriding entrypoint (which assertFilesInImage... does)
	cID := f.startContainer(f.ctx, containerConfig(ref))
	f.assertFilesInContainer(f.ctx, cID, expected)
}

// * * * CONTAINER UPDATER * * *

func TestUpdateInContainerE2E(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("delete_me", "will be deleted")
	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}

	// Allows us to track number of times the entrypoint has been called (i.e. how
	// many times container has been (re)started -- also, sleep a bit so container
	// stays alive for us to manipulate.
	initStartcount := model.ToShellCmd("echo -n 0 > /src/startcount")
	entrypoint := model.ToShellCmd(
		"echo -n $(($(cat /src/startcount)+1)) > /src/startcount && sleep 210")

	steps := model.ToSteps(f.Path(), []model.Cmd{initStartcount})
	imgRef, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, steps, entrypoint)
	if err != nil {
		t.Fatal(err)
	}
	cID := f.startContainer(f.ctx, containerConfig(imgRef))

	f.Rm("delete_me") // expect to be deleted from container on update
	f.WriteFile("foo", "hello world")

	paths := []PathMapping{
		PathMapping{LocalPath: f.JoinPath("delete_me"), ContainerPath: "/src/delete_me"},
		PathMapping{LocalPath: f.JoinPath("foo"), ContainerPath: "/src/foo"},
	}
	touchBar := model.ToShellCmd("touch /src/bar")

	cUpdater := ContainerUpdater{dCli: f.dCli}
	err = cUpdater.UpdateInContainer(f.ctx, cID, paths, model.EmptyMatcher, []model.Cmd{touchBar}, f.ps.Writer(f.ctx))
	if err != nil {
		f.t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{Path: "/src/delete_me", Missing: true},
		expectedFile{Path: "/src/foo", Contents: "hello world"},
		expectedFile{Path: "/src/bar", Contents: ""},         // from cmd
		expectedFile{Path: "/src/startcount", Contents: "2"}, // from entrypoint (confirm container restarted)
	}

	f.assertFilesInContainer(f.ctx, cID, expected)
}

func TestReapOneImage(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}

	df1 := simpleDockerfile
	ref1, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), df1, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	label := dockerfile.Label("tilt.reaperTest")
	f.b.extraLabels[label] = "1"
	df2 := simpleDockerfile.Run(model.ToShellCmd("echo hi >> hi.txt"))
	ref2, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), df2, []model.Mount{m}, model.EmptyMatcher, nil, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	err = f.reaper.RemoveTiltImages(f.ctx, time.Now().Add(time.Second), false, FilterByLabel(label))
	if err != nil {
		t.Fatal(err)
	}

	f.assertImageExists(ref1)
	f.assertImageNotExists(ref2)
}

func TestConditionalRunInRealDocker(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	m := model.Mount{
		LocalPath:     f.Path(),
		ContainerPath: "/src",
	}
	step1 := model.Step{
		Cmd:           model.ToShellCmd("cat /src/a.txt >> /src/c.txt"),
		Triggers:      []string{"a.txt"},
		BaseDirectory: f.Path(),
	}
	step2 := model.Step{
		Cmd: model.ToShellCmd("cat /src/b.txt >> /src/d.txt"),
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.ps, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, model.EmptyMatcher, []model.Step{step1, step2}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{Path: "/src/a.txt", Contents: "a"},
		expectedFile{Path: "/src/b.txt", Contents: "b"},
		expectedFile{Path: "/src/c.txt", Contents: "a"},
		expectedFile{Path: "/src/d.txt", Contents: "b"},
	}
	f.assertFilesInImage(ref, pcs)
}

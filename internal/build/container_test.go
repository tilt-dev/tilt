//+build !skipcontainertests

// Tests that involve spinning up/interacting with actual containers
package build

import (
	"strings"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
)

// * * * IMAGE BUILDER * * *

func TestMount(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	// write some files in to it
	f.WriteFile("hi/hello", "hi hello")
	f.WriteFile("sup", "my name is dan")

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{path: "/src/hi/hello", contents: "hi hello"},
		expectedFile{path: "/src/sup", contents: "my name is dan"},
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
		Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("hi")},
		ContainerPath: "/hello_there",
	}
	m2 := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("bye")},
		ContainerPath: "goodbye_there",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m1, m2}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{path: "/hello_there/hello", contents: "hi hello"},
		expectedFile{path: "/goodbye_there/ciao/goodbye", contents: "bye laterz"},
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
		Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("hi")},
		ContainerPath: "/hello_there",
	}
	m2 := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("bye")},
		ContainerPath: "/hello_there",
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m1, m2}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{path: "/hello_there/hello", contents: "bye laterz"},
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
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}

	name, err := reference.WithName("localhost:5005/myimage")
	if err != nil {
		t.Fatal(err)
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, name, simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	namedTagged, err := f.b.PushImage(f.ctx, ref)
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{path: "/src/hi/hello", contents: "hi hello"},
		expectedFile{path: "/src/sup", contents: "my name is dan"},
	}

	f.assertFilesInImage(namedTagged, pcs)
}

func TestPushInvalid(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}
	name, err := reference.WithName("localhost:5005/myimage")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.b.BuildImageFromScratch(f.ctx, name, simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.b.PushImage(f.ctx, ref)
	if err == nil || !strings.Contains(err.Error(), "PushImage#getDigestFromPushOutput") {
		t.Fatal(err)
	}
}

func TestBuildOneStep(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.ToShellCmd("echo -n hello >> hi"),
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "hi", contents: "hello"},
	}
	f.assertFilesInImage(ref, expected)
}

func TestBuildMultipleSteps(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.ToShellCmd("echo -n hello >> hi"),
		model.ToShellCmd("echo -n sup >> hi2"),
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "hi", contents: "hello"},
		expectedFile{path: "hi2", contents: "sup"},
	}
	f.assertFilesInImage(ref, expected)
}

func TestBuildMultipleStepsRemoveFiles(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.Cmd{Argv: []string{"sh", "-c", "echo -n hello >> hi"}},
		model.Cmd{Argv: []string{"sh", "-c", "echo -n sup >> hi2"}},
		model.Cmd{Argv: []string{"sh", "-c", "rm hi"}},
	}

	ref, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "hi2", contents: "sup"},
		expectedFile{path: "hi", missing: true},
	}
	f.assertFilesInImage(ref, expected)
}

func TestBuildFailingStep(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.ToShellCmd("echo hello && exit 1"),
	}

	_, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "hello")

		// Different versions of docker have3 a different error string
		hasExitCode1 := strings.Contains(err.Error(), "exit code 1") ||
			strings.Contains(err.Error(), "returned a non-zero code: 1")
		if !hasExitCode1 {
			t.Errorf("Expected failure with exit code 1, actual: %v", err)
		}
	}
}

func TestEntrypoint(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	entrypoint := model.ToShellCmd("echo -n hello >> hi")
	d, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{}, []model.Cmd{}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "hi", contents: "hello"},
	}

	// Start container WITHOUT overriding entrypoint (which assertFilesInImage... does)
	cID := f.startContainer(f.ctx, containerConfig(d))
	f.assertFilesInContainer(f.ctx, cID, expected)
}

func TestDockerfileWithEntrypointNotPermitted(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	df := Dockerfile(`FROM alpine
ENTRYPOINT ["sleep", "100000"]`)

	_, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), df, []model.Mount{}, []model.Cmd{}, model.Cmd{})
	if err == nil {
		t.Fatal("expected an err b/c dockerfile contains an ENTRYPOINT")
	}
	if !strings.Contains(err.Error(), ErrEntrypointInDockerfile.Error()) {
		t.Fatalf("error '%v' did not contain expected string '%v'",
			err.Error(), ErrEntrypointInDockerfile.Error())
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
			Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
			ContainerPath: "/src",
		},
	}

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, mounts, []model.Cmd{}, model.Cmd{})
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

	ref, err := f.b.BuildImageFromExisting(f.ctx, existing, pms, []model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{path: "/src/hi/hello", contents: "hello world"},
		expectedFile{path: "/src/sup", missing: true},
		expectedFile{path: "/src/nested/sup", missing: true}, // should have deleted whole directory
		expectedFile{path: "/src/unchanged", contents: "should be unchanged"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestExecStepsOnExisting(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("foo", "hello world")
	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	step := model.ToShellCmd("echo -n foo contains: $(cat /src/foo) >> /src/bar")

	ref, err := f.b.BuildImageFromExisting(f.ctx, existing, MountsToPathMappings([]model.Mount{m}), []model.Cmd{step})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []expectedFile{
		expectedFile{path: "/src/foo", contents: "hello world"},
		expectedFile{path: "/src/bar", contents: "foo contains: hello world"},
	}
	f.assertFilesInImage(ref, pcs)
}

func TestBuildImageFromExistingPreservesEntrypoint(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("foo", "hello world")
	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}
	entrypoint := model.ToShellCmd("echo -n foo contains: $(cat /src/foo) >> /src/bar")

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	// change contents of `foo` so when entrypoint exec's the second time, it
	// will change the contents of `bar`
	f.WriteFile("foo", "a whole new world")

	ref, err := f.b.BuildImageFromExisting(f.ctx, existing, MountsToPathMappings([]model.Mount{m}), []model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "/src/foo", contents: "a whole new world"},
		expectedFile{path: "/src/bar", contents: "foo contains: a whole new world"},
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
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}
	step := model.ToShellCmd("echo -n hello >> /src/baz")
	entrypoint := model.ToShellCmd("echo -n foo contains: $(cat /src/foo) >> /src/bar")

	existing, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Cmd{step}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	// change contents of `foo` so when entrypoint exec's the second time, it
	// will change the contents of `bar`
	f.WriteFile("foo", "a whole new world")

	ref, err := f.b.BuildImageFromExisting(f.ctx, existing, MountsToPathMappings([]model.Mount{m}), []model.Cmd{step})
	if err != nil {
		t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "/src/foo", contents: "a whole new world"},
		expectedFile{path: "/src/bar", contents: "foo contains: a whole new world"},
		expectedFile{path: "/src/baz", contents: "hellohello"},
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
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}

	// Allows us to track number of times the entrypoint has been called (i.e. how
	// many times container has been (re)started -- also, sleep a bit so container
	// stays alive for us to manipulate.
	initStartcount := model.ToShellCmd("echo -n 0 > /src/startcount")
	entrypoint := model.ToShellCmd(
		"echo -n $(($(cat /src/startcount)+1)) > /src/startcount && sleep 210")

	imgRef, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Cmd{initStartcount}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}
	cID := f.startContainer(f.ctx, containerConfig(imgRef))

	f.Rm("delete_me") // expect to be deleted from container on update
	f.WriteFile("foo", "hello world")

	paths := []pathMapping{
		pathMapping{LocalPath: f.JoinPath("delete_me"), ContainerPath: "/src/delete_me"},
		pathMapping{LocalPath: f.JoinPath("foo"), ContainerPath: "/src/foo"},
	}
	touchBar := model.ToShellCmd("touch /src/bar")

	cUpdater := ContainerUpdater{dcli: f.dcli}
	err = cUpdater.UpdateInContainer(f.ctx, cID, paths, []model.Cmd{touchBar})
	if err != nil {
		f.t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "/src/delete_me", missing: true},
		expectedFile{path: "/src/foo", contents: "hello world"},
		expectedFile{path: "/src/bar", contents: ""},         // from cmd
		expectedFile{path: "/src/startcount", contents: "2"}, // from entrypoint (confirm container restarted)
	}

	f.assertFilesInContainer(f.ctx, cID, expected)
}

func TestReapOneImage(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}

	df1 := simpleDockerfile
	ref1, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), df1, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	label := Label("tilt.reaperTest")
	f.b.extraLabels[label] = "1"
	df2 := simpleDockerfile.Run(model.ToShellCmd("echo hi >> hi.txt"))
	ref2, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), df2, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	err = f.reaper.RemoveTiltImages(f.ctx, time.Now().Add(time.Second), FilterByLabel(label))
	if err != nil {
		t.Fatal(err)
	}

	f.assertImageExists(ref1)
	f.assertImageNotExists(ref2)
}

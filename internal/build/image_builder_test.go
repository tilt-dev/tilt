//+build !skipcontainertests

package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/stretchr/testify/assert"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/opencontainers/go-digest"
)

const simpleDockerfile = Dockerfile("FROM alpine")

func TestDigestAsTag(t *testing.T) {
	dig := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	tag, err := digestAsTag(dig)
	if err != nil {
		t.Fatal(err)
	}

	expected := "tilt-cc5f4c463f81c551"
	if tag != expected {
		t.Errorf("Expected %s, actual: %s", expected, tag)
	}
}

func TestDigestAsTagToShort(t *testing.T) {
	dig := digest.Digest("sha256:cc")
	_, err := digestAsTag(dig)
	expected := "too short"
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error %q, actual: %v", expected, err)
	}
}

func TestDigestFromSingleStepOutput(t *testing.T) {
	f := newDockerBuildFixture(t)
	input := ExampleBuildOutput1

	expected := digest.Digest("sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	actual, err := f.b.getDigestFromBuildOutput(f.ctx, bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestDigestFromPushOutput(t *testing.T) {
	f := newDockerBuildFixture(t)
	input := ExamplePushOutput1

	expected := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	actual, err := f.b.getDigestFromPushOutput(f.ctx, bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

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
		if runtime.GOOS == "darwin" {
			assert.Contains(t, err.Error(), "exit code 1")
		} else {
			assert.Contains(t, err.Error(), "returned a non-zero code: 1")
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

func TestBuildDockerFromExistingPreservesEntrypoint(t *testing.T) {
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

type dockerBuildFixture struct {
	*testutils.TempDirFixture
	t        testing.TB
	ctx      context.Context
	dcli     *DockerCli
	b        *dockerImageBuilder
	registry *exec.Cmd
}

func newDockerBuildFixture(t testing.TB) *dockerBuildFixture {
	ctx := testutils.CtxForTest()
	dcli, err := DefaultDockerClient(ctx, k8s.EnvGKE)
	if err != nil {
		t.Fatal(err)
	}

	return &dockerBuildFixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		dcli:           dcli,
		b:              NewLocalDockerBuilder(dcli, DefaultConsole(), DefaultOut()),
	}
}

func (f *dockerBuildFixture) teardown() {
	if f.registry != nil && f.registry.Process != nil {
		go func() {
			err := f.registry.Process.Kill()
			if err != nil {
				log.Printf("killing the registry failed: %v\n", err)
			}
		}()

		// ignore the error. we expect it to be killed
		_ = f.registry.Wait()

		_ = exec.Command("docker", "kill", "tilt-registry").Run()
		_ = exec.Command("docker", "rm", "tilt-registry").Run()
	}
	f.TempDirFixture.TearDown()
}

func (f *dockerBuildFixture) getNameFromTest() reference.Named {
	x := fmt.Sprintf("windmill.build/%s", strings.ToLower(f.t.Name()))
	name, err := reference.WithName(x)
	if err != nil {
		f.t.Fatal(err)
	}

	return name
}

func (f *dockerBuildFixture) startRegistry() {
	stdout := &bytes.Buffer{}
	stdoutSafe := makeThreadSafe(stdout)
	stderr := &bytes.Buffer{}
	cmd := exec.Command("docker", "run", "--name", "tilt-registry", "-p", "5005:5000", "registry:2")
	cmd.Stdout = stdoutSafe
	cmd.Stderr = stderr
	f.registry = cmd

	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	// Wait until the registry starts
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		stdoutSafe.mu.Lock()
		result := stdout.String()
		stdoutSafe.mu.Unlock()
		if strings.Contains(result, "listening on") {
			return
		}
	}
	f.t.Fatalf("Timed out waiting for registry to start. Output:\n%s\n%s", stdout.String(), stderr.String())
}

type expectedFile struct {
	path     string
	contents string

	// If true, we will assert that the file is not in the container.
	missing bool
}

func (f *dockerBuildFixture) assertFilesInImage(ref reference.NamedTagged, expectedFiles []expectedFile) {
	cID := f.startContainer(f.ctx, containerConfigRunCmd(ref, model.Cmd{}))
	f.assertFilesInContainer(f.ctx, cID, expectedFiles)
}

func (f *dockerBuildFixture) assertFilesInContainer(
	ctx context.Context, containerID string, expectedFiles []expectedFile) {
	for _, expectedFile := range expectedFiles {
		reader, _, err := f.dcli.CopyFromContainer(ctx, containerID, expectedFile.path)
		if expectedFile.missing {
			if err == nil {
				f.t.Errorf("Expected path %q to not exist", expectedFile.path)
			} else if !strings.Contains(err.Error(), "No such container:path") {
				f.t.Errorf("Expected path %q to not exist, but got a different error: %v", expectedFile.path, err)
			}

			continue
		}

		if err != nil {
			f.t.Fatal(err)
		}

		f.assertFileInTar(tar.NewReader(reader), expectedFile)
	}
}

func (f *dockerBuildFixture) assertFileInTar(tr *tar.Reader, expected expectedFile) {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			f.t.Fatalf("File not found in container: %s", expected.path)
		} else if err != nil {
			f.t.Fatalf("Error reading tar file: %v", err)
		}

		if header.Typeflag == tar.TypeReg {
			contents := bytes.NewBuffer(nil)
			_, err = io.Copy(contents, tr)
			if err != nil {
				f.t.Fatalf("Error reading tar file: %v", err)
			}

			if contents.String() != expected.contents {
				f.t.Errorf("Wrong contents in %q. Expected: %q. Actual: %q",
					expected.path, expected.contents, contents.String())
			}
			return // we found it!
		}
	}
}

// startContainer starts a container from the given config
func (f *dockerBuildFixture) startContainer(ctx context.Context, config *container.Config) string {
	resp, err := f.dcli.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}
	containerID := resp.ID

	err = f.dcli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}

	return containerID
}

type threadSafeWriter struct {
	writer io.Writer
	mu     *sync.Mutex
}

func makeThreadSafe(writer io.Writer) threadSafeWriter {
	return threadSafeWriter{writer: writer, mu: &sync.Mutex{}}
}

func (w threadSafeWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(b)
}

package build

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/tiltd"
	"github.com/windmilleng/wmclient/pkg/os/temp"
)

const simpleDockerfile = "FROM alpine"

func TestDigestFromSingleStepOutput(t *testing.T) {
	input := `{"stream":"Step 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"aux":{"ID":"sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"}}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`

	expected := digest.Digest("sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	actual, err := getDigestFromOutput(bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestMount(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("sup", "my name is dan")

	m := tiltd.Mount{
		Repo:          tiltd.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{m}, []tiltd.Cmd{}, tiltd.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/src/hi/hello", contents: "hi hello"},
		pathContent{path: "/src/sup", contents: "my name is dan"},
	}
	f.assertFilesInImageWithContents(string(digest), pcs)
}

func TestMultipleMounts(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("bye/ciao/goodbye", "bye laterz")

	m1 := tiltd.Mount{
		Repo:          tiltd.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "hi")},
		ContainerPath: "/hello_there",
	}
	m2 := tiltd.Mount{
		Repo:          tiltd.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "bye")},
		ContainerPath: "goodbye_there",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{m1, m2}, []tiltd.Cmd{}, tiltd.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/hello_there/hello", contents: "hi hello"},
		pathContent{path: "/goodbye_there/ciao/goodbye", contents: "bye laterz"},
	}
	f.assertFilesInImageWithContents(string(digest), pcs)
}

func TestMountCollisions(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("bye/hello", "bye laterz")

	// Mounting two files to the same place in the container -- expect the second mount
	// to take precedence (file should contain "bye laterz")
	m1 := tiltd.Mount{
		Repo:          tiltd.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "hi")},
		ContainerPath: "/hello_there",
	}
	m2 := tiltd.Mount{
		Repo:          tiltd.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "bye")},
		ContainerPath: "/hello_there",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{m1, m2}, []tiltd.Cmd{}, tiltd.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/hello_there/hello", contents: "bye laterz"},
	}
	f.assertFilesInImageWithContents(string(digest), pcs)
}

func TestPush(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	f.startRegistry()

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("sup", "my name is dan")

	m := tiltd.Mount{
		Repo:          tiltd.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{m}, []tiltd.Cmd{}, tiltd.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	name := "localhost:5005/myimage"
	err = f.b.PushDocker(context.Background(), name, digest)
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/src/hi/hello", contents: "hi hello"},
		pathContent{path: "/src/sup", contents: "my name is dan"},
	}

	f.assertFilesInImageWithContents(fmt.Sprintf("%s:%s", name, pushTag), pcs)
}

func TestPushInvalid(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	m := Mount{
		Repo:          LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []Mount{m}, []Cmd{}, Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	name := "localhost:6666/myimage"
	err = f.b.PushDocker(context.Background(), name, digest)
	if err == nil || !strings.Contains(err.Error(), "PushDocker#ImagePush") {
		t.Fatal(err)
	}
}

func TestBuildOneStep(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	steps := []tiltd.Cmd{
		tiltd.Cmd{Argv: []string{"sh", "-c", "echo hello >> hi"}},
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{}, steps, tiltd.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello"},
	}
	f.assertFilesInImageWithContents(string(digest), contents)
}

func TestBuildMultipleSteps(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	steps := []tiltd.Cmd{
		tiltd.Cmd{Argv: []string{"sh", "-c", "echo hello >> hi"}},
		tiltd.Cmd{Argv: []string{"sh", "-c", "echo sup >> hi2"}},
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{}, steps, tiltd.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello"},
		pathContent{path: "hi2", contents: "sup"},
	}
	f.assertFilesInImageWithContents(string(digest), contents)
}

func TestEntrypoint(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()
	entrypoint := tiltd.Cmd{Argv: []string{"sh", "-c", "echo hello >> hi"}}
	d, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []tiltd.Mount{}, []tiltd.Cmd{}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello"},
	}
	f.assertFilesInImageWithContents(string(d), contents)
}

// TODO(maia): test mount err cases
// TODO(maia): tests for tar code

type testFixture struct {
	t        *testing.T
	repo     *temp.TempDir
	dcli     *client.Client
	b        *localDockerBuilder
	registry *exec.Cmd
}

func newTestFixture(t *testing.T) *testFixture {
	repo, err := temp.NewDir(t.Name())
	if err != nil {
		t.Fatalf("Error making temp dir: %v", err)
	}

	opts := make([]func(*client.Client) error, 0)
	opts = append(opts, client.FromEnv)

	// Use client for docker 17
	// https://docs.docker.com/develop/sdk/#api-version-matrix
	// API version 1.30 is the first version where the full digest
	// shows up in the API output of BuildImage
	opts = append(opts, client.WithVersion("1.30"))
	dcli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		t.Fatal(err)
	}

	return &testFixture{
		t:    t,
		repo: repo,
		dcli: dcli,
		b:    NewLocalDockerBuilder(dcli),
	}
}

func (f *testFixture) teardown() {
	if f.registry != nil {
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
	f.repo.TearDown()
}

func (f *testFixture) startRegistry() {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.Command("docker", "run", "--name", "tilt-registry", "-p", "5005:5000", "registry:2")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	f.registry = cmd

	err := cmd.Start()
	if err != nil {
		f.t.Fatal(err)
	}

	// Wait until the registry starts
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		if strings.Contains(stdout.String(), "listening on") {
			return
		}
	}
	f.t.Fatalf("Timed out waiting for registry to start. Output:\n%s\n%s", stdout.String(), stderr.String())
}

func (f *testFixture) writeFile(pathInRepo string, contents string) {
	fullPath := filepath.Join(f.repo.Path(), pathInRepo)
	base := filepath.Dir(fullPath)
	err := os.MkdirAll(base, os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	err = ioutil.WriteFile(fullPath, []byte(contents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

type pathContent struct {
	path     string
	contents string
}

func (f *testFixture) assertFilesInImageWithContents(ref string, contents []pathContent) {
	ctx := context.Background()
	var cmd strings.Builder
	for _, c := range contents {
		notFound := fmt.Sprintf("ERROR: file %s not found or didn't have expected contents '%s'",
			c.path, c.contents)
		cs := fmt.Sprintf("cat %s | grep \"%s\" || echo \"%s\"; ",
			c.path, c.contents, notFound)
		cmd.WriteString(cs)
	}
	cmdToRun := tiltd.Cmd{Argv: []string{"sh", "-c", cmd.String()}}

	cId, err := f.b.startContainer(ctx, ref, &cmdToRun)
	if err != nil {
		f.t.Fatal(err)
	}

	out, err := f.dcli.ContainerLogs(ctx, cId, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		f.t.Fatal(err)
	}
	output := &strings.Builder{}
	io.Copy(output, out)

	if strings.Contains(output.String(), "ERROR:") {
		f.t.Errorf("Failed to find one or more expected files in container with output:\n%s", output)
	}
}

package build

import (
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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/wmclient/pkg/os/temp"
)

func TestDigestFromSingleStepOutput(t *testing.T) {
	input := `{"stream":"Step 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"aux":{"ID":"sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"}}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`

	expected := digest.Digest("sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	actual, err := getDigestFromOutput(input)
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
	baseDockerFile := "FROM alpine"

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("sup", "my name is dan")

	m := Mount{
		Repo:          LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	builder := f.newBuilderForTesting()

	digest, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{m}, []Cmd{})
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
	baseDockerFile := "FROM alpine"

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("bye/ciao/goodbye", "bye laterz")

	m1 := Mount{
		Repo:          LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "hi")},
		ContainerPath: "/hello_there",
	}
	m2 := Mount{
		Repo:          LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "bye")},
		ContainerPath: "goodbye_there",
	}

	builder := f.newBuilderForTesting()

	digest, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{m1, m2}, []Cmd{})
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
	baseDockerFile := "FROM alpine"

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("bye/hello", "bye laterz")

	// Mounting two files to the same place in the container -- expect the second mount
	// to take precedence (file should contain "bye laterz")
	m1 := Mount{
		Repo:          LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "hi")},
		ContainerPath: "/hello_there",
	}
	m2 := Mount{
		Repo:          LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "bye")},
		ContainerPath: "/hello_there",
	}

	builder := f.newBuilderForTesting()
	digest, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{m1, m2}, []Cmd{})
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

	baseDockerFile := "FROM alpine"

	// write some files in to it
	f.writeFile("hi/hello", "hi hello")
	f.writeFile("sup", "my name is dan")

	m := Mount{
		Repo:          LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	builder := f.newBuilderForTesting()

	digest, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{m}, []Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	name := "localhost:5000/myimage"
	err = builder.PushDocker(context.Background(), name, digest)
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/src/hi/hello", contents: "hi hello"},
		pathContent{path: "/src/sup", contents: "my name is dan"},
	}

	f.assertFilesInImageWithContents(fmt.Sprintf("%s:%s", name, pushTag), pcs)
}

func TestBuildOneStep(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()
	baseDockerFile := "FROM alpine"

	steps := []Cmd{
		Cmd{argv: []string{"sh", "-c", "echo hello >> hi"}},
	}

	builder := f.newBuilderForTesting()

	digest, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{}, steps)
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
	baseDockerFile := "FROM alpine"

	steps := []Cmd{
		Cmd{argv: []string{"sh", "-c", "echo hello >> hi"}},
		Cmd{argv: []string{"sh", "-c", "echo sup >> hi2"}},
	}

	builder := f.newBuilderForTesting()

	tag, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{}, steps)
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello"},
		pathContent{path: "hi2", contents: "sup"},
	}
	f.assertFilesInImageWithContents(string(tag), contents)
}

// TODO(maia): test mount err cases
// TODO(maia): tests for tar code

type testFixture struct {
	t        *testing.T
	repo     *temp.TempDir
	dcli     *client.Client
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
	}
	f.repo.TearDown()
}

func (f *testFixture) startRegistry() {
	f.registry = exec.Command("docker", "run", "-p", "5005:5000", "registry:2")
	err := f.registry.Start()
	if err != nil {
		f.t.Fatal(err)
	}
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

func (f *testFixture) newBuilderForTesting() *localDockerBuilder {
	return NewLocalDockerBuilder(f.dcli)
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

	resp, err := f.dcli.ContainerCreate(ctx, &container.Config{
		Image: ref,
		Cmd:   []string{"sh", "-c", cmd.String()},
		Tty:   true,
	}, nil, nil, "")
	if err != nil {
		f.t.Fatal(err)
	}

	containerID := resp.ID

	err = f.dcli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		f.t.Fatal(err)
	}

	statusCh, errCh := f.dcli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			f.t.Fatal(err)
		}
	case <-statusCh:
	}

	out, err := f.dcli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		f.t.Fatal(err)
	}
	output := &strings.Builder{}
	io.Copy(output, out)

	if strings.Contains(output.String(), "ERROR:") {
		f.t.Errorf("Failed to find one or more expected files in container with output:\n%s", output)
	}
}

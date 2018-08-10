package build

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/windmilleng/wmclient/pkg/os/temp"
)

func TestDigiestFromSingleStepOutput(t *testing.T) {
	input := `{"stream":"Step 1/1 : FROM alpine"}
	{"stream":"\n"}
	{"stream":" ---\u003e 11cd0b38bc3c\n"}
	{"aux":{"ID":"sha256:11cd0b38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab"}}
	{"stream":"Successfully built 11cd0b38bc3c\n"}
	{"stream":"Successfully tagged hi:latest\n"}
`

	expected := "11cd0b38bc3c"
	actual, err := getDigestFromOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestBuildBase(t *testing.T) {
	if os.Getenv("CIRCLECI") == "true" {
		t.Skipf("Skipping on CircleCI")
	}
	f := newTestFixture(t)
	defer f.teardown()
	baseDockerFile := `FROM alpine
	RUN touch hi
	`

	builder := f.newBuilderForTesting()

	tag, err := builder.buildBaseWithMounts(context.Background(), baseDockerFile, "hi", []Mount{})
	if err != nil {
		t.Fatal(err)
	}

	f.assertFileInImageWithContents(tag, "hi", "")
}

func TestMount(t *testing.T) {
	if os.Getenv("CIRCLECI") == "true" {
		t.Skipf("Skipping on CircleCI")
	}
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
	imageTag := strings.ToLower(f.t.Name())

	tag, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{m}, []Cmd{}, imageTag)
	if err != nil {
		t.Fatal(err)
	}

	// TODO(dmiller): this is slow
	f.assertFileInImageWithContents(tag, "/src/hi/hello", "hi hello")
	f.assertFileInImageWithContents(tag, "/src/sup", "my name is dan")
}

type testFixture struct {
	t    *testing.T
	repo *temp.TempDir
	dcli *client.Client
}

func newTestFixture(t *testing.T) *testFixture {
	repo, err := temp.NewDir(t.Name())
	if err != nil {
		t.Fatalf("Error making temp dir: %v", err)
	}
	dcli, err := client.NewEnvClient()
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
	f.repo.TearDown()
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
	return &localDockerBuilder{
		dcli: f.dcli,
	}
}

func (f *testFixture) assertFileInImageWithContents(tag string, path string, contents string) {
	ctx := context.Background()
	resp, err := f.dcli.ContainerCreate(ctx, &container.Config{
		Image: tag,
		Cmd:   []string{"cat", path},
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

	if strings.Contains(output.String(), "No such file") {
		f.t.Errorf("Expected to find file %s in container. Got: %s", path, output)
	}

	if !strings.Contains(output.String(), contents) {
		f.t.Errorf("Expected file %s to have contents %s but got %s", path, contents, output)
	}
}

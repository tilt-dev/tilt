package build

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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

	imageTag := "hi"
	f.imageTag = imageTag
	tag, err := builder.buildBase(context.Background(), baseDockerFile, imageTag)
	if err != nil {
		t.Fatal(err)
	}

	f.assertFileInImage(tag, "hi")
}

func TestMount(t *testing.T) {
	t.Skipf("Not implemented yet")
	f := newTestFixture(t)
	defer f.teardown()
	baseDockerFile := "FROM alpine"

	// write some files in to it
	f.writeFile("hello", "hi hello")

	m := Mount{
		Repo:          LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	builder := f.newBuilderForTesting()
	imageTag := f.t.Name()
	f.imageTag = imageTag

	tag, err := builder.BuildDocker(context.Background(), baseDockerFile, []Mount{m}, []Cmd{}, imageTag)
	if err != nil {
		t.Fatal(err)
	}

	f.assertFileInImage(tag, "src/hello")
}

type testFixture struct {
	t           *testing.T
	repo        *temp.TempDir
	dcli        *client.Client
	containerID string
	imageTag    string
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
	err := f.dcli.ContainerRemove(context.Background(), f.containerID, types.ContainerRemoveOptions{})
	if err != nil {
		fmt.Printf("Error removing container ID %s: %s", f.containerID, err.Error())
	}
	_, err = f.dcli.ImageRemove(context.Background(), f.imageTag, types.ImageRemoveOptions{})
	if err != nil {
		fmt.Printf("Error removing image tag %s: %s", f.imageTag, err.Error())
	}
}

func (f *testFixture) writeFile(pathInRepo string, contents string) {
	err := ioutil.WriteFile(pathInRepo, []byte(contents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *testFixture) newBuilderForTesting() *localDockerBuilder {
	return &localDockerBuilder{
		dcli: f.dcli,
	}
}

func (f *testFixture) assertFileInImage(tag string, path string) {
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
}

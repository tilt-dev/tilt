package build

import (
	"archive/tar"
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
	"sync"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/stretchr/testify/assert"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
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
	actual, err := getDigestFromBuildOutput(bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestDigestFromPushOutput(t *testing.T) {
	input := `{"status":"The push refers to repository [localhost:5005/myimage]"}
	{"status":"Preparing","progressDetail":{},"id":"2a88b569da78"}
	{"status":"Preparing","progressDetail":{},"id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":512,"total":41},"progress":"[==================================================\u003e]     512B","id":"2a88b569da78"}
	{"status":"Pushing","progressDetail":{"current":68608,"total":4413370},"progress":"[\u003e                                                  ]  68.61kB/4.413MB","id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":5120,"total":41},"progress":"[==================================================\u003e]   5.12kB","id":"2a88b569da78"}
	{"status":"Pushed","progressDetail":{},"id":"2a88b569da78"}
	{"status":"Pushing","progressDetail":{"current":1547776,"total":4413370},"progress":"[=================\u003e                                 ]  1.548MB/4.413MB","id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":3247616,"total":4413370},"progress":"[====================================\u003e              ]  3.248MB/4.413MB","id":"73046094a9b8"}
	{"status":"Pushing","progressDetail":{"current":4672000,"total":4413370},"progress":"[==================================================\u003e]  4.672MB","id":"73046094a9b8"}
	{"status":"Pushed","progressDetail":{},"id":"73046094a9b8"}
	{"status":"wm-tilt: digest: sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1 size: 735"}
	{"progressDetail":{},"aux":{"Tag":"wm-tilt","Digest":"sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1","Size":735}}`

	expected := digest.Digest("sha256:cc5f4c463f81c55183d8d737ba2f0d30b3e6f3670dbe2da68f0aac168e93fbb1")
	actual, err := getDigestFromPushOutput(bytes.NewBuffer([]byte(input)))
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

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
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

	m1 := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "hi")},
		ContainerPath: "/hello_there",
	}
	m2 := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "bye")},
		ContainerPath: "goodbye_there",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{m1, m2}, []model.Cmd{}, model.Cmd{})
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
	m1 := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "hi")},
		ContainerPath: "/hello_there",
	}
	m2 := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.repo.Path(), "bye")},
		ContainerPath: "/hello_there",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{m1, m2}, []model.Cmd{}, model.Cmd{})
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

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	name, _ := reference.ParseNormalizedNamed("localhost:5005/myimage")
	_, err = f.b.PushDocker(context.Background(), name, digest)
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

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	name, _ := reference.ParseNormalizedNamed("localhost:6666/myimage")
	_, err = f.b.PushDocker(context.Background(), name, digest)
	if err == nil || !strings.Contains(err.Error(), "PushDocker#getDigestFromPushOutput") {
		t.Fatal(err)
	}
}

func TestBuildOneStep(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.Cmd{Argv: []string{"sh", "-c", "echo hello >> hi"}},
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello\n"},
	}
	f.assertFilesInImageWithContents(string(digest), contents)
}

func TestBuildMultipleSteps(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.Cmd{Argv: []string{"sh", "-c", "echo hello >> hi"}},
		model.Cmd{Argv: []string{"sh", "-c", "echo sup >> hi2"}},
	}

	digest, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello\n"},
		pathContent{path: "hi2", contents: "sup\n"},
	}
	f.assertFilesInImageWithContents(string(digest), contents)
}

func TestBuildFailingStep(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	steps := []model.Cmd{
		model.Cmd{Argv: []string{"sh", "-c", "echo hello && exit 1"}},
	}

	_, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{}, steps, model.Cmd{})
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "hello")
		assert.Contains(t, err.Error(), "exit code 1")
	}
}

func TestEntrypoint(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	entrypoint := model.Cmd{Argv: []string{"sh", "-c", "echo hello >> hi"}}
	d, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{}, []model.Cmd{}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}

	contents := []pathContent{
		pathContent{path: "hi", contents: "hello\n"},
	}
	f.assertFilesInImageWithContents(string(d), contents)
}

func TestDockerfileWithEntrypointNotPermitted(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	df := `FROM alpine
ENTRYPOINT ["sleep", "100000"]`

	_, err := f.b.BuildDocker(context.Background(), df, []model.Mount{}, []model.Cmd{}, model.Cmd{})
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

func TestAddMountsToExisting(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	f.writeFile("hi/hello", "hi hello")
	f.writeFile("sup", "yo dawg, i heard you like docker")

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	existing, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	f.writeFile("hi/hello", "hello world") // change contents
	f.rm("sup")

	digest, err := f.b.BuildDockerFromExisting(context.Background(), existing, []model.Mount{m}, []model.Cmd{}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/src/hi/hello", contents: "hello world"},
	}
	f.assertFilesInImageWithContents(string(digest), pcs)

	//TODO(maia): assert file 'sup' does NOT exist
}

func todo_TestExecStepsOnExisting(t *testing.T) {
	f := newTestFixture(t)
	defer f.teardown()

	// NOTE(maia): currently this should fail b/c if an img has entrypoint,
	// we can't exec steps on it/its ancestors w/o removing the entrypoint;
	// but can't accurately test without smarter assert's.
	sayHi := model.Cmd{Argv: []string{"sh", "-c", "echo hello"}}
	existing, err := f.b.BuildDocker(context.Background(), simpleDockerfile, []model.Mount{}, []model.Cmd{}, sayHi)
	if err != nil {
		t.Fatal(err)
	}

	f.writeFile("foo", "hello world")
	step := model.Cmd{Argv: []string{"sh", "-c", "echo foo contains: $(cat /src/foo) >> /src/bar"}}
	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.repo.Path()},
		ContainerPath: "/src",
	}

	digest, err := f.b.BuildDockerFromExisting(context.Background(), existing, []model.Mount{m}, []model.Cmd{step}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	pcs := []pathContent{
		pathContent{path: "/src/foo", contents: "hello world"},
		pathContent{path: "/src/bar", contents: "foo contains: hello world"},
	}
	f.assertFilesInImageWithContents(string(digest), pcs)
}

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

func (f *testFixture) rm(pathInRepo string) {
	fullPath := filepath.Join(f.repo.Path(), pathInRepo)
	err := os.Remove(fullPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

type pathContent struct {
	path     string
	contents string
}

func (f *testFixture) startContainerWithOutput(ctx context.Context, ref string, cmd *model.Cmd) string {
	cId, err := f.b.startContainer(ctx, ref, cmd)
	if err != nil {
		f.t.Fatal(err)
	}

	out, err := f.dcli.ContainerLogs(ctx, cId, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() {
		err := out.Close()
		if err != nil {
			f.t.Fatal("closing container logs reader:", err)
		}
	}()

	output, err := ioutil.ReadAll(out)
	if err != nil {
		f.t.Fatal("reading container logs:", err)
	}
	return string(output)
}

func (f *testFixture) assertFilesInImageWithContents(ref string, contents []pathContent) {
	ctx := context.Background()
	cId, err := f.b.startContainer(ctx, ref, &model.Cmd{Argv: []string{"sh"}})
	if err != nil {
		f.t.Fatal(err)
	}

	for _, c := range contents {
		reader, _, err := f.dcli.CopyFromContainer(ctx, cId, c.path)
		if err != nil {
			f.t.Fatal(err)
		}

		f.assertFileInTar(tar.NewReader(reader), c)
	}
}

func (f *testFixture) assertFileInTar(tr *tar.Reader, content pathContent) {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			f.t.Fatalf("File not found in container: %s", content.path)
		} else if err != nil {
			f.t.Fatalf("Error reading tar file: %v", err)
		}

		if header.Typeflag == tar.TypeReg {
			contents := bytes.NewBuffer(nil)
			_, err = io.Copy(contents, tr)
			if err != nil {
				f.t.Fatalf("Error reading tar file: %v", err)
			}

			if contents.String() != content.contents {
				f.t.Errorf("Wrong contents in %q. Expected: %q. Actual: %q",
					content.path, content.contents, contents.String())
			}
			return // we found it!
		}
	}
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

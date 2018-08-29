package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
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
	defer f.teardown()

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
	defer f.teardown()

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
	ctx context.Context, cID containerID, expectedFiles []expectedFile) {
	for _, expectedFile := range expectedFiles {
		reader, _, err := f.dcli.CopyFromContainer(ctx, cID.String(), expectedFile.path)
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
func (f *dockerBuildFixture) startContainer(ctx context.Context, config *container.Config) containerID {
	resp, err := f.dcli.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}
	cID := resp.ID

	err = f.dcli.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}

	return containerID(cID)
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

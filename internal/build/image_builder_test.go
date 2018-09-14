package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
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
	f := newFakeDockerBuildFixture(t)
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
	f := newFakeDockerBuildFixture(t)
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

func TestConditionalRunInFakeDocker(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPaths: []string{f.Path()}},
		ContainerPath: "/src",
	}
	inputs, _ := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"a.txt"})
	step1 := model.Step{
		Cmd:     model.ToShellCmd("cat /src/a.txt > /src/c.txt"),
		Trigger: inputs,
	}
	step2 := model.Step{
		Cmd: model.ToShellCmd("cat /src/b.txt > /src/d.txt"),
	}

	_, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Step{step1, step2}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := expectedFile{
		path: "Dockerfile",
		contents: `FROM alpine
LABEL "tilt.buildMode"="scratch"
LABEL "tilt.test"="1"
COPY /src/a.txt /src/a.txt
RUN cat /src/a.txt > /src/c.txt
ADD . /
RUN cat /src/b.txt > /src/d.txt`,
	}
	assertFileInTar(f.t, tar.NewReader(f.fakeDocker.BuildOptions.Context), expected)
}

func TestAllConditionalRunsInFakeDocker(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("a.txt", "a")
	f.WriteFile("b.txt", "b")

	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPaths: []string{f.Path()}},
		ContainerPath: "/src",
	}
	inputs, _ := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"a.txt"})
	step1 := model.Step{
		Cmd:     model.ToShellCmd("cat /src/a.txt > /src/c.txt"),
		Trigger: inputs,
	}

	_, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Step{step1}, model.Cmd{})
	if err != nil {
		t.Fatal(err)
	}

	expected := expectedFile{
		path: "Dockerfile",
		contents: `FROM alpine
LABEL "tilt.buildMode"="scratch"
LABEL "tilt.test"="1"
COPY /src/a.txt /src/a.txt
RUN cat /src/a.txt > /src/c.txt
ADD . /`,
	}
	assertFileInTar(f.t, tar.NewReader(f.fakeDocker.BuildOptions.Context), expected)
}

type dockerBuildFixture struct {
	*tempdir.TempDirFixture
	t            testing.TB
	ctx          context.Context
	dcli         *DockerCli
	fakeDocker   *FakeDockerClient
	b            *dockerImageBuilder
	registry     *exec.Cmd
	reaper       ImageReaper
	containerIDs []k8s.ContainerID
}

func newDockerBuildFixture(t testing.TB) *dockerBuildFixture {
	ctx := output.CtxForTest()
	dcli, err := DefaultDockerClient(ctx, k8s.EnvGKE)
	if err != nil {
		t.Fatal(err)
	}

	labels := Labels(map[Label]LabelValue{
		TestImage: "1",
	})
	return &dockerBuildFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		dcli:           dcli,
		b:              NewDockerImageBuilder(dcli, DefaultConsole(), DefaultOut(), labels),
		reaper:         NewImageReaper(dcli),
	}
}

func newFakeDockerBuildFixture(t testing.TB) *dockerBuildFixture {
	ctx := output.CtxForTest()
	dcli := NewFakeDockerClient()
	labels := Labels(map[Label]LabelValue{
		TestImage: "1",
	})
	return &dockerBuildFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		fakeDocker:     dcli,
		b:              NewDockerImageBuilder(dcli, DefaultConsole(), DefaultOut(), labels),
		reaper:         NewImageReaper(dcli),
	}
}

func (f *dockerBuildFixture) teardown() {
	for _, cID := range f.containerIDs {
		// ignore failures
		_ = f.dcli.ContainerRemove(f.ctx, string(cID), types.ContainerRemoveOptions{
			Force: true,
		})
	}

	// ignore failures
	_ = f.reaper.RemoveTiltImages(f.ctx, time.Now(), true /*force*/, FilterByLabel(TestImage))

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

func (f *dockerBuildFixture) assertImageExists(ref reference.NamedTagged) {
	_, _, err := f.dcli.ImageInspectWithRaw(f.ctx, ref.String())
	if err != nil {
		f.t.Errorf("Expected image %q to exist, got: %v", ref, err)
	}
}

func (f *dockerBuildFixture) assertImageNotExists(ref reference.NamedTagged) {
	_, _, err := f.dcli.ImageInspectWithRaw(f.ctx, ref.String())
	if err == nil || !client.IsErrNotFound(err) {
		f.t.Errorf("Expected image %q to fail with ErrNotFound, got: %v", ref, err)
	}
}

func (f *dockerBuildFixture) assertFilesInImage(ref reference.NamedTagged, expectedFiles []expectedFile) {
	cID := f.startContainer(f.ctx, containerConfigRunCmd(ref, model.Cmd{}))
	f.assertFilesInContainer(f.ctx, cID, expectedFiles)
}

func (f *dockerBuildFixture) assertFilesInContainer(
	ctx context.Context, cID k8s.ContainerID, expectedFiles []expectedFile) {
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

		// When you copy a single file out of a container, you get
		// back a tarball with 1 entry, the file basename.
		adjustedFile := expectedFile
		adjustedFile.path = filepath.Base(adjustedFile.path)
		assertFileInTar(f.t, tar.NewReader(reader), adjustedFile)
	}
}

// startContainer starts a container from the given config
func (f *dockerBuildFixture) startContainer(ctx context.Context, config *container.Config) k8s.ContainerID {
	resp, err := f.dcli.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}
	cID := resp.ID

	err = f.dcli.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}

	result := k8s.ContainerID(cID)
	f.containerIDs = append(f.containerIDs, result)
	return result
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

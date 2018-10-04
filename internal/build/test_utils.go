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
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

type dockerBuildFixture struct {
	*tempdir.TempDirFixture
	t            testing.TB
	ctx          context.Context
	dcli         *docker.DockerCli
	fakeDocker   *docker.FakeDockerClient
	b            *dockerImageBuilder
	registry     *exec.Cmd
	reaper       ImageReaper
	containerIDs []k8s.ContainerID
}

func newDockerBuildFixture(t testing.TB) *dockerBuildFixture {
	ctx := output.CtxWithPipelineForTest()
	dcli, err := docker.DefaultDockerClient(ctx, k8s.EnvGKE)
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
	ctx := output.CtxWithPipelineForTest()
	dcli := docker.NewFakeDockerClient()
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

type expectedFile = testutils.ExpectedFile

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
		reader, _, err := f.dcli.CopyFromContainer(ctx, cID.String(), expectedFile.Path)
		if expectedFile.Missing {
			if err == nil {
				f.t.Errorf("Expected path %q to not exist", expectedFile.Path)
			} else if !strings.Contains(err.Error(), "No such container:path") {
				f.t.Errorf("Expected path %q to not exist, but got a different error: %v", expectedFile.Path, err)
			}

			continue
		}

		if err != nil {
			f.t.Fatal(err)
		}

		// When you copy a single file out of a container, you get
		// back a tarball with 1 entry, the file basename.
		adjustedFile := expectedFile
		adjustedFile.Path = filepath.Base(adjustedFile.Path)
		testutils.AssertFileInTar(f.t, tar.NewReader(reader), adjustedFile)
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

// Get a container config to run a container with a given command instead of
// the existing entrypoint. If cmd is nil, we run nothing.
func containerConfigRunCmd(imgRef reference.NamedTagged, cmd model.Cmd) *container.Config {
	config := containerConfig(imgRef)

	// In Docker, both the Entrypoint and the Cmd are used to determine what
	// process the container runtime uses, where Entrypoint takes precedence over
	// command. We set both here to ensure that we don't get weird results due
	// to inheritance.
	//
	// If cmd is nil, we use a fake cmd that does nothing.
	//
	// https://github.com/opencontainers/image-spec/blob/master/config.md#properties
	if cmd.Empty() {
		config.Cmd = model.ToShellCmd("# NOTE(nick): a fake cmd").Argv
	} else {
		config.Cmd = cmd.Argv
	}
	config.Entrypoint = []string{}
	return config
}

// Get a container config to run a container as-is.
func containerConfig(imgRef reference.NamedTagged) *container.Config {
	return &container.Config{Image: imgRef.String()}
}

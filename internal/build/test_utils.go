package build

import (
	"archive/tar"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/clusterid"
	wmcontainer "github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

type dockerBuildFixture struct {
	*tempdir.TempDirFixture
	t            testing.TB
	ctx          context.Context
	dCli         *docker.Cli
	fakeDocker   *docker.FakeClient
	b            *DockerBuilder
	registry     *exec.Cmd
	reaper       ImageReaper
	containerIDs []wmcontainer.ID
	ps           *PipelineState
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

func newDockerBuildFixture(t testing.TB) *dockerBuildFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	env := clusterid.ProductGKE

	kCli := k8s.NewFakeK8sClient(t)
	kCli.Runtime = wmcontainer.RuntimeDocker
	dEnv := docker.ProvideClusterEnv(ctx, docker.RealClientCreator{}, "gke", env, kCli, k8s.FakeMinikube{})
	dCli := docker.NewDockerClient(ctx, docker.Env(dEnv))
	_, ok := dCli.(*docker.Cli)
	// If it wasn't an actual Docker client, it's an exploding client
	if !ok {
		// Call the simplest interface function that returns the error which originally occurred in NewDockerClient()
		t.Fatal(dCli.CheckConnected())
	}
	ps := NewPipelineState(ctx, 3, fakeClock{})

	labels := dockerfile.Labels(map[dockerfile.Label]dockerfile.LabelValue{
		TestImage: "1",
	})
	ret := &dockerBuildFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		dCli:           dCli.(*docker.Cli),
		b:              NewDockerBuilder(dCli, labels),
		reaper:         NewImageReaper(dCli),
		ps:             ps,
	}

	t.Cleanup(ret.teardown)
	return ret
}

func newFakeDockerBuildFixture(t testing.TB) *dockerBuildFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	dCli := docker.NewFakeClient()
	labels := dockerfile.Labels(map[dockerfile.Label]dockerfile.LabelValue{
		TestImage: "1",
	})

	ps := NewPipelineState(ctx, 3, realClock{})

	ret := &dockerBuildFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		fakeDocker:     dCli,
		b:              NewDockerBuilder(dCli, labels),
		reaper:         NewImageReaper(dCli),
		ps:             ps,
	}

	t.Cleanup(ret.teardown)
	return ret
}

func (f *dockerBuildFixture) teardown() {
	for _, cID := range f.containerIDs {
		// ignore failures
		_ = f.dCli.ContainerRemove(f.ctx, string(cID), types.ContainerRemoveOptions{
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
}

func (f *dockerBuildFixture) getNameFromTest() wmcontainer.RefSet {
	x := fmt.Sprintf("windmill.build/%s", strings.ToLower(f.t.Name()))
	sel := wmcontainer.MustParseSelector(x)
	return wmcontainer.MustSimpleRefSet(sel)
}

type expectedFile = testutils.ExpectedFile

func (f *dockerBuildFixture) assertImageHasLabels(ref reference.Named, expected map[string]string) {
	inspect, _, err := f.dCli.ImageInspectWithRaw(f.ctx, ref.String())
	if err != nil {
		f.t.Fatalf("error inspecting image %s: %v", ref.String(), err)
	}

	if inspect.Config == nil {
		f.t.Fatalf("'inspect' result for image %s has nil config", ref.String())
	}

	actual := inspect.Config.Labels
	for k, expectV := range expected {
		actualV, ok := actual[k]
		if assert.True(f.t, ok, "key %q not found in actual labels: %v", k, actual) {
			assert.Equal(f.t, expectV, actualV, "actual label (%s = %s) did not match expected (%s = %s)",
				k, actualV, k, expectV)
		}
	}

}

func (f *dockerBuildFixture) assertFilesInImage(ref reference.NamedTagged, expectedFiles []expectedFile) {
	cID := f.startContainer(f.ctx, containerConfigRunCmd(ref, model.Cmd{}))
	f.assertFilesInContainer(f.ctx, cID, expectedFiles)
}

func (f *dockerBuildFixture) assertFilesInContainer(
	ctx context.Context, cID wmcontainer.ID, expectedFiles []expectedFile) {
	for _, expectedFile := range expectedFiles {
		reader, _, err := f.dCli.CopyFromContainer(ctx, cID.String(), expectedFile.Path)
		if expectedFile.Missing {
			if err == nil {
				f.t.Errorf("Expected path %q to not exist", expectedFile.Path)
			} else if !strings.Contains(err.Error(), "No such container:path") && !strings.Contains(err.Error(), "Could not find the file") {
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
func (f *dockerBuildFixture) startContainer(ctx context.Context, config *container.Config) wmcontainer.ID {
	resp, err := f.dCli.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}
	cID := resp.ID

	err = f.dCli.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	if err != nil {
		f.t.Fatalf("startContainer: %v", err)
	}

	result := wmcontainer.ID(cID)
	f.containerIDs = append(f.containerIDs, result)
	return result
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
		config.Cmd = model.ToUnixCmd("# NOTE(nick): a fake cmd").Argv
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

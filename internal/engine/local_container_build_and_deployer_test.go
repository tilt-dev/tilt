package engine

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const pod1 = k8s.PodID("pod1")
const pod2 = k8s.PodID("pod2")

const container1 = k8s.ContainerID("container1")
const container2 = k8s.ContainerID("container2")

var image1 = k8s.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")

const digest1 = "sha256:936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"

var containerListOutput = map[string][]types.Container{
	pod1.String(): []types.Container{
		types.Container{ID: container1.String(), ImageID: digest1},
	},
}

func TestPostProcessBuild(t *testing.T) {
	f := newContainerBadFixture()

	f.dCli.SetContainerListOutput(containerListOutput)
	f.kCli.SetPodWithImageResp(pod1)

	res := BuildResult{Image: image1}
	f.cbad.PostProcessBuild(f.ctx, res)

	if assert.NotEmpty(t, f.cbad.deployInfo) {
		assert.Equal(t, container1, f.cbad.deployInfo[docker.ToImgNameAndTag(image1)])
	}
}

func TestPostProcessBuildNoopIfAlreadyHaveInfo(t *testing.T) {
	f := newContainerBadFixture()

	f.dCli.SetContainerListOutput(containerListOutput)
	f.kCli.SetPodWithImageResp(pod1)

	f.cbad.deployInfo[docker.ToImgNameAndTag(image1)] = k8s.ContainerID("ohai")

	res := BuildResult{Image: image1}
	f.cbad.PostProcessBuild(f.ctx, res)

	if assert.NotEmpty(t, f.cbad.deployInfo) {
		assert.Equal(t, k8s.ContainerID("ohai"), f.cbad.deployInfo[docker.ToImgNameAndTag(image1)], "Getting info again for same image -- contents should not have changed")
	}
}

type containerBaDFixture struct {
	ctx  context.Context
	dCli *docker.FakeDockerClient
	kCli *k8s.FakeK8sClient
	cbad *LocalContainerBuildAndDeployer
}

func newContainerBadFixture() *containerBaDFixture {
	// TODO(maia): wire this
	fakeDocker := docker.NewFakeDockerClient()
	fakeK8s := k8s.NewFakeK8sClient()

	cu := build.NewContainerUpdater(fakeDocker)
	cr := build.NewContainerResolver(fakeDocker)
	a := analytics.NewMemoryAnalytics()

	cbad := NewLocalContainerBuildAndDeployer(cu, cr, k8s.EnvDockerDesktop, fakeK8s, a)

	return &containerBaDFixture{
		ctx:  output.CtxForTest(),
		dCli: fakeDocker,
		kCli: fakeK8s,
		cbad: cbad,
	}
}

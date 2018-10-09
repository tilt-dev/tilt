package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const pod1 = k8s.PodID("pod1")

var image1 = k8s.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")

const digest1 = "sha256:936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"

func TestPostProcessBuild(t *testing.T) {
	f := newContainerBadFixture()

	f.kCli.SetPodsWithImageResp(pod1)

	res := BuildResult{Image: image1}
	f.cbad.PostProcessBuild(f.ctx, res, res)

	info, ok := f.cbad.dd.DeployInfoForImageBlocking(f.ctx, image1)
	assert.True(t, ok)
	assert.Equal(t, string(k8s.MagicTestContainerID), string(info.containerID))
}

func TestPostProcessBuildNoopIfAlreadyHaveInfo(t *testing.T) {
	f := newContainerBadFixture()

	f.kCli.SetPodsWithImageResp(pod1)

	info := newEmptyDeployInfo()
	info.containerID = k8s.ContainerID("ohai")
	info.markReady()
	f.cbad.dd.deployInfo[docker.ToImgNameAndTag(image1)] = info

	res := BuildResult{Image: image1}
	f.cbad.PostProcessBuild(f.ctx, res, res)

	info, ok := f.cbad.dd.DeployInfoForImageBlocking(f.ctx, image1)
	assert.True(t, ok)
	assert.Equal(t, k8s.ContainerID("ohai"), info.containerID,
		"Getting info again for same image -- contents should not have changed")
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
	a := analytics.NewMemoryAnalytics()
	dd := NewDeployDiscovery(fakeK8s)

	cbad := NewLocalContainerBuildAndDeployer(cu, a, dd)

	return &containerBaDFixture{
		ctx:  output.CtxForTest(),
		dCli: fakeDocker,
		kCli: fakeK8s,
		cbad: cbad,
	}
}

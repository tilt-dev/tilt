package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/container"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const pod1 = k8s.PodID("pod1")

var image1 = container.MustParseNamedTagged("re.po/project/myapp:tilt-936a185caaa266bb")

const digest1 = "sha256:936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"

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

	cbad := NewLocalContainerBuildAndDeployer(cu, a)

	return &containerBaDFixture{
		ctx:  output.CtxForTest(),
		dCli: fakeDocker,
		kCli: fakeK8s,
		cbad: cbad,
	}
}

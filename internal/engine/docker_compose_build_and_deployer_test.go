package engine

import (
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

type dcbdFixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	dcCli *dockercompose.FakeDCClient
	dCli  *docker.FakeClient
	dcbad *DockerComposeBuildAndDeployer
}

func newDCBDFixture(t *testing.T) *dcbdFixture {
	ctx := output.CtxForTest()

	f := tempdir.NewTempDirFixture(t)

	dir := dirs.NewWindmillDirAt(f.Path())
	dcCli := dockercompose.NewFakeDockerComposeClient(t, ctx)
	dCli := docker.NewFakeClient()
	dcbad, err := provideDockerComposeBuildAndDeployer(ctx, dcCli, dCli, dir)
	if err != nil {
		t.Fatal(err)
	}
	return &dcbdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		dcCli:          dcCli,
		dCli:           dCli,
		dcbad:          dcbad,
	}
}

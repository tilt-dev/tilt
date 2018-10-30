package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestDeployEmpty(t *testing.T) {
	f := newDeployFixture(t)
	defer f.TearDown()

	info, err := f.dd.DeployInfoForImageBlocking(f.ctx, image1)
	assert.Nil(t, err)
	assert.True(t, info.Empty())
}

func TestDeploy(t *testing.T) {
	f := newDeployFixture(t)
	defer f.TearDown()

	f.kClient.SetPodsWithImageResp(pod1)
	f.dd.EnsureDeployInfoFetchStarted(f.ctx, image1, k8s.DefaultNamespace)

	info, err := f.dd.DeployInfoForImageBlocking(f.ctx, image1)
	assert.Nil(t, err)
	assert.Equal(t, string(k8s.MagicTestContainerID), string(info.containerID))
}

func TestDeployFail(t *testing.T) {
	f := newDeployFixture(t)
	defer f.TearDown()

	fakeErr := fmt.Errorf("my-error")
	f.kClient.PodsWithImageError = fakeErr
	f.dd.EnsureDeployInfoFetchStarted(f.ctx, image1, k8s.DefaultNamespace)

	info, err := f.dd.DeployInfoForImageBlocking(f.ctx, image1)
	assert.Contains(t, err.Error(), fakeErr.Error())
	assert.True(t, info.Empty())
}

type deployFixture struct {
	*tempdir.TempDirFixture
	ctx     context.Context
	kClient *k8s.FakeK8sClient
	dd      *DeployDiscovery
}

func newDeployFixture(t *testing.T) *deployFixture {
	f := tempdir.NewTempDirFixture(t)
	kClient := k8s.NewFakeK8sClient()
	dd := NewDeployDiscovery(kClient, store.NewStoreForTesting())
	return &deployFixture{
		TempDirFixture: f,
		ctx:            output.CtxForTest(),
		kClient:        kClient,
		dd:             dd,
	}
}

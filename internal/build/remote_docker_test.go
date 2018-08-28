package build

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestContainerIdForPodOneMatch(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "docker-for-mac")
	defer f.teardown()
	cID, err := f.b.containerIdForPod(context.Background())
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "container-for-d4m")
}

func TestContainerIdForPodFiltersOutPauseCmd(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "gke")
	defer f.teardown()
	cID, err := f.b.containerIdForPod(context.Background())
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "container-for-gke")
}

func TestContainerIdForPodTooManyMatches(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "too-many")
	defer f.teardown()
	_, err := f.b.containerIdForPod(context.Background())
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "too many matching containers")
	}
}

func TestContainerIdForPodNoNonPause(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "all-pause")
	defer f.teardown()
	_, err := f.b.containerIdForPod(context.Background())
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "no matching non-'/pause' containers")
	}
}

type remoteDockerFixture struct {
	*testutils.TempDirFixture
	t   testing.TB
	ctx context.Context
	b   *remoteDockerBuilder
}

func newRemoteDockerFixtureForPod(t testing.TB, podName string) *remoteDockerFixture {
	builder := &remoteDockerBuilder{
		dcli: NewFakeDockerClient(),
		pod:  podName,
	}

	return &remoteDockerFixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
		ctx:            testutils.CtxForTest(),
		b:              builder,
	}
}

func (f *remoteDockerFixture) teardown() {
	f.TempDirFixture.TearDown()
}

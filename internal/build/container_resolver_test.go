package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainerIdForPodOneMatch(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cr.ContainerIDForPod(f.ctx, testPod)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), testContainer)
}

func TestContainerIdForPodFiltersOutPauseCmd(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cr.ContainerIDForPod(f.ctx, "one-pause-cmd")
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "the right container")
}

func TestContainerIdForPodTooManyMatches(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	_, err := f.cr.ContainerIDForPod(f.ctx, "too-many")
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "too many matching containers")
	}
}

func TestContainerIdForPodNoNonPause(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	_, err := f.cr.ContainerIDForPod(f.ctx, "all-pause")
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "no matching non-'/pause' containers")
	}
}

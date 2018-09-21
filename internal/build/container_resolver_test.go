package build

import (
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/docker"
)

var image, _ = reference.ParseNamed("windmill.build/image:tilt-11cd0b38bc3ce")
var imageTagged = image.(reference.NamedTagged)

func TestContainerIDForPodOneMatch(t *testing.T) {
	f := newRemoteDockerFixture(t)
	f.dcli.SetDefaultContainerListOutput()
	defer f.teardown()

	cID, err := f.cr.ContainerIDForPod(f.ctx, docker.TestPod, imageTagged)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), docker.TestContainer)
}

func TestContainerIDForPodTwoContainers(t *testing.T) {
	f := newRemoteDockerFixture(t)
	f.dcli.SetDefaultContainerListOutput()
	defer f.teardown()

	cID, err := f.cr.ContainerIDForPod(f.ctx, "two-containers", imageTagged)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "the right container")
}

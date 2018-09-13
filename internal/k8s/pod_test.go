package k8s

import (
	"context"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
)

const podsToImagesOut = `blorg-fe-6b4477ffcd-xf98f	blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f
cockroachdb-0	cockroachdb/cockroach:v2.0.5
cockroachdb-1	cockroachdb/cockroach:v2.0.5
cockroachdb-2	cockroachdb/cockroach:v2.0.5
`

const podsManyContainers = `blorg-fe-6b4477ffcd-xf98f	blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f
cockroachdb-0	cockroachdb/cockroach:v2.0.5
cockroachdb-1	cockroachdb/cockroach:v2.0.5
cockroachdb-2	cockroachdb/cockroach:v2.0.5
two-containers	nginx	debian
`

func TestPodImgMapFromOutput(t *testing.T) {
	podImgMap, err := imgPodMapFromOutput(podsToImagesOut)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]PodID{
		"blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f": []PodID{PodID("blorg-fe-6b4477ffcd-xf98f")},
		"cockroachdb/cockroach:v2.0.5":                           []PodID{PodID("cockroachdb-0"), PodID("cockroachdb-1"), PodID("cockroachdb-2")},
	}
	assert.Equal(t, expected, podImgMap)
}

func TestMultipleContainersOnePod(t *testing.T) {
	podImgMap, err := imgPodMapFromOutput(podsManyContainers)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]PodID{
		"blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f": []PodID{PodID("blorg-fe-6b4477ffcd-xf98f")},
		"cockroachdb/cockroach:v2.0.5":                           []PodID{PodID("cockroachdb-0"), PodID("cockroachdb-1"), PodID("cockroachdb-2")},
		"nginx":  []PodID{PodID("two-containers")},
		"debian": []PodID{PodID("two-containers")},
	}
	assert.Equal(t, expected, podImgMap)
}

func (c clientTestFixture) FindAppByNodeWithOutput(output string) (PodID, error) {
	c.setOutput(output)
	return c.client.FindAppByNode(context.Background(), "synclet", NodeID("foo"))
}

func (c clientTestFixture) FindAppByNodeWithError(err error) (PodID, error) {
	c.setError(err)
	return c.client.FindAppByNode(context.Background(), "synclet", NodeID("foo"))
}

func TestFindAppByNode(t *testing.T) {
	f := newClientTestFixture(t)
	podId, err := f.FindAppByNodeWithOutput("foobar")
	if assert.NoError(t, err) {
		assert.Equal(t, PodID("foobar"), podId)
	}
}

func TestFindAppByNodeNotFound(t *testing.T) {
	f := newClientTestFixture(t)
	_, err := f.FindAppByNodeWithOutput("")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unable to find")
	}
}

func TestFindAppByNodeMultipleFound(t *testing.T) {
	f := newClientTestFixture(t)
	output := "foobar\nbazquu\n"
	_, err := f.FindAppByNodeWithOutput(output)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "multiple")
		assert.Contains(t, err.Error(), output)
	}
}

func TestFindAppByNodeKubectlError(t *testing.T) {
	f := newClientTestFixture(t)
	e := errors.New("asdffdsa")
	_, err := f.FindAppByNodeWithError(e)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), e.Error())
	}
}

func (c clientTestFixture) GetNodeForPodWithOutput(output string) (NodeID, error) {
	c.setOutput(output)
	return c.client.GetNodeForPod(context.Background(), PodID("foo"))
}

func (c clientTestFixture) GetNodeForPodWithError(err error) (NodeID, error) {
	c.setError(err)
	return c.client.GetNodeForPod(context.Background(), PodID("foo"))
}

func TestGetNodeForPod(t *testing.T) {
	f := newClientTestFixture(t)
	nodeID, err := f.GetNodeForPodWithOutput("foobar")
	if assert.NoError(t, err) {
		assert.Equal(t, NodeID("foobar"), nodeID)
	}
}

func TestGetNodeForPodNotFound(t *testing.T) {
	f := newClientTestFixture(t)
	_, err := f.GetNodeForPodWithOutput("")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "did not contain")
	}
}

func TestGetNodeForPodMultipleFound(t *testing.T) {
	f := newClientTestFixture(t)
	output := "foobar\nbazquu\n"
	_, err := f.GetNodeForPodWithOutput(output)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "multiple")
		assert.Contains(t, err.Error(), output)
	}
}

func TestGetNodeForPodKubectlError(t *testing.T) {
	f := newClientTestFixture(t)
	e := errors.New("asdffdsa")
	_, err := f.GetNodeForPodWithError(e)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), e.Error())
	}
}

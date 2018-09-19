package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
)

const expectedPod = "blorg-fe-6b4477ffcd-xf98f"
const blorgDevImgStr = "blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f"

var podsToImagesOut = fmt.Sprintf(`%s	%s
cockroachdb-0	cockroachdb/cockroach:v2.0.5
cockroachdb-1	cockroachdb/cockroach:v2.0.5
cockroachdb-2	cockroachdb/cockroach:v2.0.5
`, expectedPod, blorgDevImgStr)

var podsManyContainers = fmt.Sprintf(`%s	%s
cockroachdb-0	cockroachdb/cockroach:v2.0.5
cockroachdb-1	cockroachdb/cockroach:v2.0.5
cockroachdb-2	cockroachdb/cockroach:v2.0.5
two-containers	nginx	debian
`, expectedPod, blorgDevImgStr)

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

func (c clientTestFixture) FindAppByNodeWithOptions(options FindAppByNodeOptions) (PodID, error) {
	c.setOutput("foo")
	return c.client.FindAppByNode(context.Background(), NodeID("foo"), "synclet", options)
}

func (c clientTestFixture) FindAppByNodeWithOutput(output string) (PodID, error) {
	c.setOutput(output)
	return c.client.FindAppByNode(context.Background(), NodeID("foo"), "synclet", FindAppByNodeOptions{})
}

func (c clientTestFixture) FindAppByNodeWithError(err error) (PodID, error) {
	c.setError(err)
	return c.client.FindAppByNode(context.Background(), NodeID("foo"), "synclet", FindAppByNodeOptions{})
}

func (c clientTestFixture) AssertCallExistsWithArg(expectedArg string) {
	foundMatchingCall := false
	var errorOutput string
	for _, call := range c.runner.calls {
		for _, arg := range call.argv {
			if expectedArg == arg {
				foundMatchingCall = true
			}
		}
		errorOutput += fmt.Sprintf("%v\n", call.argv)
	}

	assert.True(c.t, foundMatchingCall, "did not find arg '%s' in of the calls to kubectlRunner: %v", expectedArg, errorOutput)
}

func TestPodWithImage(t *testing.T) {
	f := newClientTestFixture(t)
	f.setOutput(podsToImagesOut)
	nt := MustParseNamedTagged(blorgDevImgStr)
	pID, err := f.client.PodWithImage(f.ctx, nt)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(t, expectedPod, pID.String())
}

func TestPollForPodWithImage(t *testing.T) {
	f := newClientTestFixture(t)
	go func() {
		time.Sleep(time.Second)
		f.setOutput(podsToImagesOut)
	}()

	nt := MustParseNamedTagged(blorgDevImgStr)
	pID, err := f.client.PollForPodWithImage(f.ctx, nt, 2*time.Second)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(t, expectedPod, pID.String())
}

func TestPollForPodWithImageTimesOut(t *testing.T) {
	f := newClientTestFixture(t)
	go func() {
		time.Sleep(time.Second)
		f.setOutput(podsToImagesOut)
	}()

	nt := MustParseNamedTagged(blorgDevImgStr)
	_, err := f.client.PollForPodWithImage(f.ctx, nt, 500*time.Millisecond)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "timed out polling for pod running image")
	}
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
	output := "foobar bazquu"
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

func TestFindAppByNodeWithOwner(t *testing.T) {
	f := newClientTestFixture(t)
	_, _ = f.FindAppByNodeWithOptions(FindAppByNodeOptions{Owner: "bob"})
	f.AssertCallExistsWithArg("-lapp=synclet,owner=bob")
}

func TestFindAppByNodeWithNamespace(t *testing.T) {
	f := newClientTestFixture(t)
	_, _ = f.FindAppByNodeWithOptions(FindAppByNodeOptions{Namespace: "kube-system"})
	f.AssertCallExistsWithArg("--namespace=kube-system")
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

package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

const expectedPod = PodID("blorg-fe-6b4477ffcd-xf98f")
const blorgDevImgStr = "blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f"

var resourceVersion = 1

func fakePod(podID PodID, imageID string) v1.Pod {
	resourceVersion++
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            string(podID),
			Namespace:       "default",
			Labels:          make(map[string]string, 0),
			ResourceVersion: fmt.Sprintf("%d", resourceVersion),
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
			Containers: []v1.Container{
				v1.Container{
					Image: imageID,
				},
			},
		},
	}
}

func podList(pods ...v1.Pod) v1.PodList {
	return v1.PodList{
		Items: pods,
	}
}

var fakePodList = podList(
	fakePod("cockroachdb-0", "cockroachdb/cockroach:v2.0.5"),
	fakePod("cockroachdb-1", "cockroachdb/cockroach:v2.0.5"),
	fakePod("cockroachdb-2", "cockroachdb/cockroach:v2.0.5"),
	fakePod(expectedPod, blorgDevImgStr))

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

func TestPodsWithImage(t *testing.T) {
	f := newClientTestFixture(t)
	f.addObject(&fakePodList)
	nt := MustParseNamedTagged(blorgDevImgStr)
	pods, err := f.client.PodsWithImage(f.ctx, nt, DefaultNamespace, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	pod := &(pods[0])
	assert.Equal(t, expectedPod, PodIDFromPod(pod))
}

func TestPodsWithImageLabels(t *testing.T) {
	f := newClientTestFixture(t)

	pod1 := fakePod("cockroach-0", "cockroachdb/cockroach:v2.0.5")
	pod1.ObjectMeta.Labels["type"] = "primary"
	pod2 := fakePod("cockroach-1", "cockroachdb/cockroach:v2.0.5")
	pod2.ObjectMeta.Labels["type"] = "replica"

	f.addObject(&pod1)
	f.addObject(&pod2)
	nt := MustParseNamedTagged("cockroachdb/cockroach:v2.0.5")

	pods, err := f.client.PodsWithImage(f.ctx, nt, DefaultNamespace, []LabelPair{{"type", "primary"}})
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(t, 1, len(pods)) {
		assert.Equal(t, "primary", pods[0].ObjectMeta.Labels["type"])
	}

	pods, err = f.client.PodsWithImage(f.ctx, nt, DefaultNamespace, []LabelPair{{"type", "replica"}})
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(t, 1, len(pods)) {
		assert.Equal(t, "replica", pods[0].ObjectMeta.Labels["type"])
	}
}

func TestPollForPodsWithImage(t *testing.T) {
	f := newClientTestFixture(t)
	go func() {
		time.Sleep(time.Second)
		f.addObject(&fakePodList)
	}()

	nt := MustParseNamedTagged(blorgDevImgStr)
	pods, err := f.client.PollForPodsWithImage(f.ctx, nt, DefaultNamespace, nil, 2*time.Second)
	if err != nil {
		f.t.Fatal(err)
	}
	pod := &(pods[0])
	assert.Equal(t, expectedPod, PodIDFromPod(pod))
}

func TestPollForPodsWithImageTimesOut(t *testing.T) {
	f := newClientTestFixture(t)
	go func() {
		time.Sleep(time.Second)
		f.addObject(&fakePodList)
	}()

	nt := MustParseNamedTagged(blorgDevImgStr)
	_, err := f.client.PollForPodsWithImage(f.ctx, nt, DefaultNamespace, nil, 500*time.Millisecond)
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

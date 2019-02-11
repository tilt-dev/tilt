package k8s

import (
	"fmt"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/windmilleng/tilt/internal/container"
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
	nt := container.MustParseNamedTagged(blorgDevImgStr)
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
	nt := container.MustParseNamedTagged("cockroachdb/cockroach:v2.0.5")

	pods, err := f.client.PodsWithImage(f.ctx, nt, DefaultNamespace, []model.LabelPair{{Key: "type", Value: "primary"}})
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(t, 1, len(pods)) {
		assert.Equal(t, "primary", pods[0].ObjectMeta.Labels["type"])
	}

	pods, err = f.client.PodsWithImage(f.ctx, nt, DefaultNamespace, []model.LabelPair{{Key: "type", Value: "replica"}})
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

	nt := container.MustParseNamedTagged(blorgDevImgStr)
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

	nt := container.MustParseNamedTagged(blorgDevImgStr)
	_, err := f.client.PollForPodsWithImage(f.ctx, nt, DefaultNamespace, nil, 500*time.Millisecond)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "timed out polling for pod running image")
	}
}

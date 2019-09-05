package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

const expectedPod = PodID("blorg-fe-6b4477ffcd-xf98f")
const blorgDevImgStr = "blorg.io/blorgdev/blorg-frontend:tilt-361d98a2d335373f"

func podList(pods ...v1.Pod) v1.PodList {
	return v1.PodList{
		Items: pods,
	}
}

var fakePodList = podList(
	*fakePod("cockroachdb-0", "cockroachdb/cockroach:v2.0.5"),
	*fakePod("cockroachdb-1", "cockroachdb/cockroach:v2.0.5"),
	*fakePod("cockroachdb-2", "cockroachdb/cockroach:v2.0.5"),
	*fakePod(expectedPod, blorgDevImgStr))

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

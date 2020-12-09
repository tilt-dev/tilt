package k8s

import (
	v1 "k8s.io/api/core/v1"
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

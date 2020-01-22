package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Common fakes

var resourceVersion = 1

func fakePod(podID PodID, imageID string) *v1.Pod {
	resourceVersion++
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            string(podID),
			Namespace:       "default",
			Labels:          make(map[string]string),
			ResourceVersion: fmt.Sprintf("%d", resourceVersion),
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
			Containers: []v1.Container{
				v1.Container{
					Name:  "default",
					Image: imageID,
				},
			},
		},
	}
}

func fakeService(name string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func fakeEvent(name string, message string, count int) *v1.Event {
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Message: message,
		Count:   int32(count),
	}
}

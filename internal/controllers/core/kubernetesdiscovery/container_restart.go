package kubernetesdiscovery

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ContainerRestartDetector struct{}

func NewContainerRestartDetector() *ContainerRestartDetector {
	return &ContainerRestartDetector{}
}

func (c *ContainerRestartDetector) Detect(dispatcher Dispatcher, prevStatus v1alpha1.KubernetesDiscoveryStatus, current *v1alpha1.KubernetesDiscovery) {
	mn := model.ManifestName(current.Annotations[v1alpha1.AnnotationManifest])
	if mn == "" {
		// log actions are dispatched by manifest, so if this spec isn't associated with a manifest,
		// there's no reason to proceed
		return
	}
	prevPods := podsByUID(prevStatus)
	currentPods := podsByUID(current.Status)
	for uid, currentPod := range currentPods {
		c.handlePod(dispatcher, mn, prevPods[uid], currentPod)
	}
}

func (c *ContainerRestartDetector) handlePod(dispatcher Dispatcher, mn model.ManifestName, prev *v1alpha1.Pod, current *v1alpha1.Pod) {
	if prev == nil {
		// this is a new pod, so there's nothing to diff off for restarts
		return
	}
	c.logRestarts(dispatcher, mn, current, restartedNames(prev.InitContainers, current.InitContainers))
	c.logRestarts(dispatcher, mn, current, restartedNames(prev.Containers, current.Containers))
}

func (c *ContainerRestartDetector) logRestarts(dispatcher Dispatcher, mn model.ManifestName, pod *v1alpha1.Pod, restarted []container.Name) {
	spanID := k8sconv.SpanIDForPod(mn, k8s.PodID(pod.Name))
	for _, containerName := range restarted {
		msg := fmt.Sprintf("Detected container restart. Pod: %s. Container: %s.", pod.Name, containerName)
		dispatcher.Dispatch(store.NewLogAction(mn, spanID, logger.WarnLvl, nil, []byte(msg)))
	}
}

func podsByUID(status v1alpha1.KubernetesDiscoveryStatus) map[types.UID]*v1alpha1.Pod {
	pods := make(map[types.UID]*v1alpha1.Pod)
	for _, p := range status.Pods {
		pods[types.UID(p.UID)] = &p
	}
	return pods
}

func restartedNames(prev []v1alpha1.Container, current []v1alpha1.Container) []container.Name {
	var result []container.Name
	for i, c := range current {
		if i >= len(prev) {
			break
		}

		existing := prev[i]
		if existing.Name != c.Name {
			continue
		}

		if c.Restarts > existing.Restarts {
			result = append(result, container.Name(c.Name))
		}
	}
	return result
}

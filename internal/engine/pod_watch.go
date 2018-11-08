package engine

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
	"k8s.io/api/core/v1"
)

type PodWatcher struct {
	kCli     k8s.Client
	watching bool
}

func NewPodWatcher(kCli k8s.Client) *PodWatcher {
	return &PodWatcher{
		kCli: kCli,
	}
}

func (w *PodWatcher) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.WatchMounts && !w.watching
}

func (w *PodWatcher) OnChange(ctx context.Context, st store.RStore) {
	if !w.needsWatch(st) {
		return
	}
	w.watching = true

	ch, err := w.kCli.WatchPods(ctx, []k8s.LabelPair{TiltRunLabel()})
	if err != nil {
		st.Dispatch(NewErrorAction(err))
		return
	}

	go dispatchPodChangesLoop(ctx, ch, st)
}

func dispatchPodChangesLoop(ctx context.Context, ch <-chan *v1.Pod, st store.RStore) {
	for {
		select {
		case pod, ok := <-ch:
			if !ok {
				return
			}
			st.Dispatch(NewPodChangeAction(pod))
		case <-ctx.Done():
			return
		}
	}
}

// copied from https://github.com/kubernetes/kubernetes/blob/aedeccda9562b9effe026bb02c8d3c539fc7bb77/pkg/kubectl/resource_printer.go#L692-L764
// to match the status column of `kubectl get pods`
func podStatusToString(pod v1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			}
		}
	}

	return reason
}

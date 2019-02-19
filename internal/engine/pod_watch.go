package engine

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
)

type PodWatcher struct {
	kCli    k8s.Client
	watches []PodWatch
}

func NewPodWatcher(kCli k8s.Client) *PodWatcher {
	return &PodWatcher{
		kCli: kCli,
	}
}

type PodWatch struct {
	labels labels.Selector
	cancel context.CancelFunc
}

// returns all elements of `a` that are not in `b`
func subtract(a, b []PodWatch) []PodWatch {
	var ret []PodWatch
	// silly O(n^3) diff here on assumption that lists will be trivially small
	for _, pwa := range a {
		inB := false
		for _, pwb := range b {
			if k8s.SelectorEqual(pwa.labels, pwb.labels) {
				inB = true
				break
			}
		}
		if !inB {
			ret = append(ret, pwa)
		}
	}
	return ret
}

func (w *PodWatcher) diff(ctx context.Context, st store.RStore) (setup []PodWatch, teardown []PodWatch) {
	state := st.RLockState()
	defer st.RUnlockState()

	atLeastOneK8S := false
	var neededWatches []PodWatch
	for _, m := range state.Manifests() {
		if m.IsK8s() {
			atLeastOneK8S = true
			for _, ls := range m.K8sTarget().ExtraPodSelectors {
				if !ls.Empty() {
					neededWatches = append(neededWatches, PodWatch{labels: ls})
				}
			}
		}
	}
	if atLeastOneK8S {
		neededWatches = append(neededWatches, PodWatch{labels: k8s.TiltRunSelector()})
	}

	return subtract(neededWatches, w.watches), subtract(w.watches, neededWatches)
}

func (w *PodWatcher) OnChange(ctx context.Context, st store.RStore) {
	setup, teardown := w.diff(ctx, st)

	for _, pw := range setup {
		ctx, cancel := context.WithTimeout(ctx, watchTimeout)
		pw = PodWatch{labels: pw.labels, cancel: cancel}
		w.watches = append(w.watches, pw)
		ch, err := w.kCli.WatchPods(ctx, pw.labels)
		if err != nil {
			err = errors.Wrap(err, "Error watching pods. Are you connected to kubernetes?\n")
			st.Dispatch(NewErrorAction(err))
			return
		}
		go dispatchPodChangesLoop(ctx, ch, st)
	}

	for _, pw := range teardown {
		pw.cancel()
		w.removeWatch(pw)
	}
}

func (w *PodWatcher) removeWatch(toRemove PodWatch) {
	oldWatches := append([]PodWatch{}, w.watches...)
	w.watches = nil
	for _, e := range oldWatches {
		if !k8s.SelectorEqual(e.labels, toRemove.labels) {
			w.watches = append(w.watches, e)
		}
	}
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

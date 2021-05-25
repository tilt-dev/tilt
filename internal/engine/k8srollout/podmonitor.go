package k8srollout

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type podManifest struct {
	pod      k8s.PodID
	manifest model.ManifestName
}

type PodMonitor struct {
	pods            map[podManifest]podStatus
	trackingStarted map[podManifest]bool
}

func NewPodMonitor() *PodMonitor {
	return &PodMonitor{
		pods:            make(map[podManifest]podStatus),
		trackingStarted: make(map[podManifest]bool),
	}
}

func (m *PodMonitor) diff(st store.RStore) []podStatus {
	state := st.RLockState()
	defer st.RUnlockState()

	updates := make([]podStatus, 0)
	active := make(map[podManifest]bool)

	for _, mt := range state.Targets() {
		ms := mt.State
		manifest := mt.Manifest
		pod := ms.MostRecentPod()
		podID := k8s.PodID(pod.Name)
		if podID.Empty() {
			continue
		}

		key := podManifest{pod: podID, manifest: manifest.Name}
		active[key] = true

		// pod status updates during an active build are likely to be misleading or lost
		// in the noise, so wait until the build finishes to process them
		if !mt.State.ActiveBuild().Empty() {
			continue
		}
		// ignore updates to pods that don't match the currently deployed pod template spec
		if !mt.State.K8sRuntimeState().HasOKPodTemplateSpecHash(&pod) {
			continue
		}
		currentStatus := newPodStatus(pod, mt.State.LastBuild().StartTime, manifest.Name)
		if !podStatusesEqual(currentStatus, m.pods[key]) {
			updates = append(updates, currentStatus)
			m.pods[key] = currentStatus
		}
	}

	for key := range m.pods {
		if !active[key] {
			delete(m.pods, key)
		}
	}

	return updates
}

func (m *PodMonitor) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) {
	updates := m.diff(st)
	for _, update := range updates {
		ctx := logger.CtxWithLogHandler(ctx, podStatusWriter{
			store:        st,
			manifestName: update.manifestName,
			podID:        update.podID,
		})
		m.print(ctx, update)
	}
}

func (m *PodMonitor) print(ctx context.Context, update podStatus) {
	reusingPod := update.podStartTime.Before(update.updateStartTime)
	if reusingPod {
		logger.Get(ctx).Infof("\nExisting pod still matches build (%s)", update.podID)
		return
	}

	key := podManifest{pod: update.podID, manifest: update.manifestName}
	if !m.trackingStarted[key] {
		logger.Get(ctx).Infof("\nTracking new pod rollout (%s):", update.podID)
		m.trackingStarted[key] = true
	}

	m.printCondition(ctx, "Scheduled", update.scheduled, update.podStartTime)
	m.printCondition(ctx, "Initialized", update.initialized, update.scheduled.LastTransitionTime.Time)
	m.printCondition(ctx, "Ready", update.ready, update.initialized.LastTransitionTime.Time)
}

func (m *PodMonitor) printCondition(ctx context.Context, name string, cond v1alpha1.PodCondition, startTime time.Time) {
	l := logger.Get(ctx).WithFields(logger.Fields{logger.FieldNameProgressID: name})

	indent := "     "
	duration := ""
	spacerMax := 16
	spacer := ""
	if len(name) > spacerMax {
		name = name[:spacerMax-1] + "…"
	} else {
		spacer = strings.Repeat(" ", spacerMax-len(name))
	}

	dur := cond.LastTransitionTime.Sub(startTime)
	if !startTime.IsZero() && !cond.LastTransitionTime.IsZero() {
		if dur == 0 {
			duration = "<1s"
		} else {
			duration = fmt.Sprint(dur.Truncate(time.Millisecond))
		}
	}

	if cond.Status == string(v1.ConditionTrue) {
		l.Infof("%s┊ %s%s- %s", indent, name, spacer, duration)
		return
	}

	message := cond.Message
	reason := cond.Reason
	if cond.Status == "" || reason == "" || message == "" {
		l.Infof("%s┊ %s%s- (…) Pending", indent, name, spacer)
		return
	}

	prefix := "Not "
	spacer = strings.Repeat(" ", spacerMax-len(name)-len(prefix))
	l.Infof("%s┃ %s%s%s- (%s): %s", indent, prefix, name, spacer, reason, message)
}

type podStatus struct {
	podID           k8s.PodID
	manifestName    model.ManifestName
	updateStartTime time.Time
	podStartTime    time.Time
	scheduled       v1alpha1.PodCondition
	initialized     v1alpha1.PodCondition
	ready           v1alpha1.PodCondition
}

func newPodStatus(pod v1alpha1.Pod, updateStartTime time.Time, manifestName model.ManifestName) podStatus {
	s := podStatus{
		podID:           k8s.PodID(pod.Name),
		manifestName:    manifestName,
		updateStartTime: updateStartTime,
		podStartTime:    pod.CreatedAt.Time,
	}
	for _, condition := range pod.Conditions {
		switch v1.PodConditionType(condition.Type) {
		case v1.PodScheduled:
			s.scheduled = condition
		case v1.PodInitialized:
			s.initialized = condition
		case v1.PodReady:
			s.ready = condition
		}
	}
	return s
}

var podStatusAllowUnexported = cmp.AllowUnexported(podStatus{})

func podStatusesEqual(a, b podStatus) bool {
	return cmp.Equal(a, b, podStatusAllowUnexported)
}

type podStatusWriter struct {
	store        store.RStore
	podID        k8s.PodID
	manifestName model.ManifestName
}

func (w podStatusWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(store.NewLogAction(w.manifestName, spanIDForPod(w.manifestName, w.podID), level, fields, p))
	return nil
}

func spanIDForPod(mn model.ManifestName, podID k8s.PodID) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("monitor:%s:%s", mn, podID))
}

var _ store.Subscriber = &PodMonitor{}

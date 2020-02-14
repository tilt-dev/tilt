package k8srollout

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type PodMonitor struct {
	pods            map[k8s.PodID]podStatus
	trackingStarted map[k8s.PodID]bool
}

func NewPodMonitor() *PodMonitor {
	return &PodMonitor{
		pods:            make(map[k8s.PodID]podStatus),
		trackingStarted: make(map[k8s.PodID]bool),
	}
}

func (m *PodMonitor) diff(st store.RStore) []podStatus {
	state := st.RLockState()
	defer st.RUnlockState()

	updates := make([]podStatus, 0)
	active := make(map[k8s.PodID]bool)

	for _, mt := range state.Targets() {
		ms := mt.State
		manifest := mt.Manifest
		pod := ms.MostRecentPod()
		podID := pod.PodID
		if podID == "" {
			continue
		}

		active[podID] = true

		currentStatus := newPodStatus(pod, manifest.Name)
		if !podStatusesEqual(currentStatus, m.pods[podID]) {
			updates = append(updates, currentStatus)
			m.pods[podID] = currentStatus
		}
	}

	for key := range m.pods {
		if !active[key] {
			delete(m.pods, key)
		}
	}

	return updates
}

func (m *PodMonitor) OnChange(ctx context.Context, st store.RStore) {
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
	if !m.trackingStarted[update.podID] {
		logger.Get(ctx).Infof("\nTracking new pod rollout (%s):", update.podID)
		m.trackingStarted[update.podID] = true
	}

	m.printCondition(ctx, "Scheduled", update.scheduled, update.startTime)
	m.printCondition(ctx, "Initialized", update.initialized, update.scheduled.LastTransitionTime.Time)
	m.printCondition(ctx, "Ready", update.ready, update.initialized.LastTransitionTime.Time)
}

func (m *PodMonitor) printCondition(ctx context.Context, name string, cond v1.PodCondition, startTime time.Time) {
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

	if cond.Status == v1.ConditionTrue {
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
	podID        k8s.PodID
	manifestName model.ManifestName
	startTime    time.Time
	scheduled    v1.PodCondition
	initialized  v1.PodCondition
	ready        v1.PodCondition
}

func newPodStatus(pod store.Pod, manifestName model.ManifestName) podStatus {
	s := podStatus{podID: pod.PodID, manifestName: manifestName, startTime: pod.StartedAt}
	for _, condition := range pod.Conditions {
		switch condition.Type {
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
	w.store.Dispatch(store.NewLogAction(w.manifestName, SpanIDForPod(w.podID), level, fields, p))
	return nil
}

func SpanIDForPod(podID k8s.PodID) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("monitor:%s", podID))
}

var _ store.Subscriber = &PodMonitor{}

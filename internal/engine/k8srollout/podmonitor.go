package k8srollout

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

		currentStatus := newPodStatus(pod, manifest.Name)
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

func (m *PodMonitor) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	updates := m.diff(st)
	for _, update := range updates {
		ctx := store.WithManifestLogHandler(ctx, st, update.manifestName, spanIDForPod(update.manifestName, update.podID))
		m.print(ctx, update)
	}

	return nil
}

func (m *PodMonitor) print(ctx context.Context, update podStatus) {
	key := podManifest{pod: update.podID, manifest: update.manifestName}
	if !m.trackingStarted[key] {
		logger.Get(ctx).Infof("\nTracking new pod rollout (%s):", update.podID)
		m.trackingStarted[key] = true
	}

	m.printCondition(ctx, "Scheduled", update.scheduled, update.startTime)
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

	// PodConditions unfortunately don't represent Jobs well:
	// 1) If a Job runs quickly enough, we might never observe the pod in a ready state (i.e., no updates have Type="Ready" and Status="True")
	// 2) After a Job finishes, we get a pod update with Type="Ready", Status="False", and LastTransitionTime=the job end time,
	//    meaning that we lose the time at which the pod actually transitioned to the ready state.
	// Rather than invest in more state to track these for the Job case, let's just replace "Ready" with "Completed"
	// and reconsider if/when a user cares.
	if cond.Type == "Ready" && cond.Reason == "PodCompleted" {
		name = "Completed"
		spacer = strings.Repeat(" ", spacerMax-len(name))
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
	scheduled    v1alpha1.PodCondition
	initialized  v1alpha1.PodCondition
	ready        v1alpha1.PodCondition
}

func newPodStatus(pod v1alpha1.Pod, manifestName model.ManifestName) podStatus {
	s := podStatus{podID: k8s.PodID(pod.Name), manifestName: manifestName, startTime: pod.CreatedAt.Time}
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

func spanIDForPod(mn model.ManifestName, podID k8s.PodID) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("monitor:%s:%s", mn, podID))
}

var _ store.Subscriber = &PodMonitor{}

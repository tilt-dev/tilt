package k8srollout

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/manifestutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// NOTE(han): set at runtime with:
// go test -ldflags="-X 'github.com/tilt-dev/tilt/internal/engine/k8srollout.PodmonitorWriteGoldenMaster=1'" ./internal/engine/k8srollout
var PodmonitorWriteGoldenMaster = "0"

func TestMonitorReady(t *testing.T) {
	f := newPMFixture(t)

	start := f.clock.Now()
	p := v1alpha1.Pod{
		Name:      "pod-id",
		CreatedAt: apis.NewTime(start),
		Conditions: []v1alpha1.PodCondition{
			{
				Type:               string(v1.PodScheduled),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(time.Second)),
			},
			{
				Type:               string(v1.PodInitialized),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(5 * time.Second)),
			},
			{
				Type:               string(v1.PodReady),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(10 * time.Second)),
			},
		},
	}

	state := store.NewState()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)

	_ = f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	assertSnapshot(t, f.out.String())
}

func TestAttachExisting(t *testing.T) {
	f := newPMFixture(t)

	start := f.clock.Now()
	p := v1alpha1.Pod{
		Name:      "pod-id",
		CreatedAt: apis.NewTime(start.Add(-10 * time.Second)),
		Conditions: []v1alpha1.PodCondition{
			{
				Type:               string(v1.PodScheduled),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(-10 * time.Second)),
			},
			{
				Type:               string(v1.PodInitialized),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(-10 * time.Second)),
			},
			{
				Type:               string(v1.PodReady),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(-10 * time.Second)),
			},
		},
	}

	state := store.NewState()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)

	_ = f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	assertSnapshot(t, f.out.String())
}

// https://github.com/tilt-dev/tilt/issues/3513
func TestJobCompleted(t *testing.T) {
	f := newPMFixture(t)

	start := f.clock.Now()
	p := v1alpha1.Pod{
		Name:      "pod-id",
		CreatedAt: apis.NewTime(start),
		Conditions: []v1alpha1.PodCondition{
			{
				Type:               string(v1.PodScheduled),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(time.Second)),
				Reason:             "PodCompleted",
			},
			{
				Type:               string(v1.PodInitialized),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(5 * time.Second)),
				Reason:             "PodCompleted",
			},
			{
				Type:               string(v1.PodReady),
				Status:             string(v1.ConditionFalse),
				LastTransitionTime: apis.NewTime(start.Add(10 * time.Second)),
				Reason:             "PodCompleted",
			},
		},
	}

	state := store.NewState()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)

	_ = f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	assertSnapshot(t, f.out.String())
}

func TestJobCompletedAfterReady(t *testing.T) {
	f := newPMFixture(t)

	start := f.clock.Now()
	p := v1alpha1.Pod{
		Name:      "pod-id",
		CreatedAt: apis.NewTime(start),
		Conditions: []v1alpha1.PodCondition{
			{
				Type:               string(v1.PodScheduled),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(time.Second)),
			},
			{
				Type:               string(v1.PodInitialized),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(5 * time.Second)),
			},
			{
				Type:               string(v1.PodReady),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(10 * time.Second)),
			},
		},
	}

	state := store.NewState()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)
	_ = f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	p.Conditions[2].Status = string(v1.ConditionFalse)
	p.Conditions[2] = v1alpha1.PodCondition{
		Type:               string(v1.PodReady),
		Status:             string(v1.ConditionFalse),
		LastTransitionTime: apis.NewTime(start.Add(20 * time.Second)),
		Reason:             "PodCompleted",
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)
	_ = f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	assertSnapshot(t, f.out.String())
}

func TestMonitorChangeSummaryFiltering(t *testing.T) {
	changedResource := store.NewChangeSet(types.NamespacedName{Name: "server"})
	tests := []struct {
		name          string
		summary       store.ChangeSummary
		wantProcessed bool
	}{
		{
			name:          "exact log only skips",
			summary:       store.ChangeSummary{Log: true},
			wantProcessed: false,
		},
		{
			name:          "log and legacy processes",
			summary:       store.ChangeSummary{Log: true, Legacy: true},
			wantProcessed: true,
		},
		{
			name:          "log and UI resource processes",
			summary:       store.ChangeSummary{Log: true, UIResources: changedResource},
			wantProcessed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newPMFixture(t)
			key := podManifest{pod: "cached-pod", manifest: "cached-manifest"}
			cached := podStatus{podID: key.pod, manifestName: key.manifest}
			f.pm.pods[key] = cached

			err := f.pm.OnChange(f.ctx, f.store, tt.summary)
			assert.NoError(t, err)

			_, found := f.pm.pods[key]
			assert.Equal(t, !tt.wantProcessed, found)
		})
	}
}

func TestMonitorMixedLogSummaryPrintsRealPodChange(t *testing.T) {
	f := newPMFixture(t)
	start := f.clock.Now()
	p := v1alpha1.Pod{
		Name:      "pod-id",
		CreatedAt: apis.NewTime(start),
		Conditions: []v1alpha1.PodCondition{
			{
				Type:               string(v1.PodScheduled),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(time.Second)),
			},
		},
	}
	state := store.NewState()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)

	err := f.pm.OnChange(f.ctx, f.store, store.ChangeSummary{
		Log:         true,
		UIResources: store.NewChangeSet(types.NamespacedName{Name: "server"}),
	})

	assert.NoError(t, err)
	assert.Contains(t, f.out.String(), "Tracking new pod rollout (pod-id)")
}

func TestPodStatusesEqual(t *testing.T) {
	start := time.Date(2026, time.July, 10, 1, 2, 3, 4, time.UTC)
	condition := func(conditionType, status, reason, message string, transition time.Time) v1alpha1.PodCondition {
		return v1alpha1.PodCondition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: apis.NewTime(transition),
			Reason:             reason,
			Message:            message,
		}
	}
	base := podStatus{
		podID:        "pod-id",
		manifestName: "server",
		startTime:    start,
		scheduled:    condition("PodScheduled", "True", "Scheduled", "scheduled", start.Add(time.Second)),
		initialized:  condition("Initialized", "True", "Initialized", "initialized", start.Add(2*time.Second)),
		ready:        condition("Ready", "False", "Starting", "starting", start.Add(3*time.Second)),
	}

	tests := []struct {
		name   string
		mutate func(*podStatus)
		want   bool
	}{
		{name: "equal", mutate: func(*podStatus) {}, want: true},
		{
			name: "equal instants in different locations",
			mutate: func(s *podStatus) {
				location := time.FixedZone("offset", 2*60*60)
				s.startTime = s.startTime.In(location)
				s.scheduled.LastTransitionTime = apis.NewTime(s.scheduled.LastTransitionTime.Time.In(location))
				s.initialized.LastTransitionTime = apis.NewTime(s.initialized.LastTransitionTime.Time.In(location))
				s.ready.LastTransitionTime = apis.NewTime(s.ready.LastTransitionTime.Time.In(location))
			},
			want: true,
		},
		{name: "pod ID", mutate: func(s *podStatus) { s.podID = "other" }, want: false},
		{name: "manifest name", mutate: func(s *podStatus) { s.manifestName = "other" }, want: false},
		{name: "start time", mutate: func(s *podStatus) { s.startTime = s.startTime.Add(time.Second) }, want: false},
		{name: "scheduled type", mutate: func(s *podStatus) { s.scheduled.Type = "Other" }, want: false},
		{name: "scheduled status", mutate: func(s *podStatus) { s.scheduled.Status = "False" }, want: false},
		{name: "scheduled time", mutate: func(s *podStatus) {
			s.scheduled.LastTransitionTime = apis.NewTime(s.scheduled.LastTransitionTime.Add(time.Second))
		}, want: false},
		{name: "scheduled reason", mutate: func(s *podStatus) { s.scheduled.Reason = "Other" }, want: false},
		{name: "scheduled message", mutate: func(s *podStatus) { s.scheduled.Message = "other" }, want: false},
		{name: "initialized type", mutate: func(s *podStatus) { s.initialized.Type = "Other" }, want: false},
		{name: "initialized status", mutate: func(s *podStatus) { s.initialized.Status = "False" }, want: false},
		{name: "initialized time", mutate: func(s *podStatus) {
			s.initialized.LastTransitionTime = apis.NewTime(s.initialized.LastTransitionTime.Add(time.Second))
		}, want: false},
		{name: "initialized reason", mutate: func(s *podStatus) { s.initialized.Reason = "Other" }, want: false},
		{name: "initialized message", mutate: func(s *podStatus) { s.initialized.Message = "other" }, want: false},
		{name: "ready type", mutate: func(s *podStatus) { s.ready.Type = "Other" }, want: false},
		{name: "ready status", mutate: func(s *podStatus) { s.ready.Status = "True" }, want: false},
		{name: "ready time", mutate: func(s *podStatus) {
			s.ready.LastTransitionTime = apis.NewTime(s.ready.LastTransitionTime.Add(time.Second))
		}, want: false},
		{name: "ready reason", mutate: func(s *podStatus) { s.ready.Reason = "Other" }, want: false},
		{name: "ready message", mutate: func(s *podStatus) { s.ready.Message = "other" }, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := base
			tt.mutate(&candidate)
			assert.Equal(t, tt.want, podStatusesEqual(base, candidate))
		})
	}
}

func TestPodStatusFieldCoverageGuard(t *testing.T) {
	// podStatusesEqual and podConditionsEqual enumerate every field by hand
	// instead of using reflection. If either count changes, update the
	// comparison (and TestPodStatusesEqual) before updating the count, or the
	// new field is silently ignored when deciding whether to log a rollout
	// update.
	assert.Equal(t, 6, reflect.TypeOf(podStatus{}).NumField())
	assert.Equal(t, 5, reflect.TypeOf(v1alpha1.PodCondition{}).NumField())
}

func BenchmarkPodMonitorOnLogOnly(b *testing.B) {
	clock := clockwork.NewFakeClock()
	pm := NewPodMonitor(clock)
	st := store.NewTestingStore()
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(io.Discard))
	state := store.NewState()
	start := clock.Now()
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("server-%03d", i)
		pod := v1alpha1.Pod{Name: name, CreatedAt: apis.NewTime(start)}
		state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
			model.Manifest{Name: model.ManifestName(name)}, pod))
	}
	st.SetState(*state)

	// Populate the cache before timing so this measures the quiet steady-state
	// work caused by a log-only notification.
	_ = pm.OnChange(ctx, st, store.LegacyChangeSummary())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pm.OnChange(ctx, st, store.ChangeSummary{Log: true})
	}
}

func BenchmarkPodStatusesEqual(b *testing.B) {
	status := podStatus{
		podID:        "pod-id",
		manifestName: "server",
		startTime:    time.Unix(1, 2),
		ready: v1alpha1.PodCondition{
			Type:               string(v1.PodReady),
			Status:             string(v1.ConditionTrue),
			LastTransitionTime: apis.NewTime(time.Unix(3, 4)),
		},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = podStatusesEqual(status, status)
	}
}

type pmFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	pm     *PodMonitor
	cancel func()
	out    *bufsync.ThreadSafeBuffer
	store  *testStore
	clock  *clockwork.FakeClock
}

func newPMFixture(t *testing.T) *pmFixture {
	f := tempdir.NewTempDirFixture(t)

	out := bufsync.NewThreadSafeBuffer()
	st := NewTestingStore(out)
	clock := clockwork.NewFakeClock()
	pm := NewPodMonitor(clock)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(out))

	ret := &pmFixture{
		TempDirFixture: f,
		pm:             pm,
		ctx:            ctx,
		cancel:         cancel,
		out:            out,
		store:          st,
		clock:          clock,
	}
	clock.Advance(time.Second)

	t.Cleanup(ret.TearDown)

	return ret
}

func (f *pmFixture) TearDown() {
	f.cancel()
}

type testStore struct {
	*store.TestingStore
	out io.Writer
}

func NewTestingStore(out io.Writer) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	logAction, ok := action.(store.LogAction)
	if ok {
		_, _ = s.out.Write(logAction.Message())
	}
}

func assertSnapshot(t *testing.T, output string) {
	d1 := []byte(output)
	gmPath := fmt.Sprintf("testdata/%s_master", t.Name())
	if PodmonitorWriteGoldenMaster == "1" {
		err := os.WriteFile(gmPath, d1, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(gmPath)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, normalize(string(expected)), normalize(output))
}

func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

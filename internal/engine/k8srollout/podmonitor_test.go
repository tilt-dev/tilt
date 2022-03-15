package k8srollout

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

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

type pmFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	pm     *PodMonitor
	cancel func()
	out    *bufsync.ThreadSafeBuffer
	store  *testStore
	clock  clockwork.FakeClock
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
		err := ioutil.WriteFile(gmPath, d1, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected, err := ioutil.ReadFile(gmPath)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, normalize(string(expected)), normalize(output))
}

func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

package k8srollout

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/manifestutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestMonitorReady(t *testing.T) {
	f := newPMFixture(t)
	defer f.TearDown()

	start := time.Now()
	p := v1alpha1.Pod{
		Name:      "pod-id",
		CreatedAt: apis.NewTime(start.Add(5 * time.Second)),
		Conditions: []v1alpha1.PodCondition{
			{
				Type:               string(v1.PodScheduled),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(6 * time.Second)),
			},
			{
				Type:               string(v1.PodInitialized),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(10 * time.Second)),
			},
			{
				Type:               string(v1.PodReady),
				Status:             string(v1.ConditionTrue),
				LastTransitionTime: apis.NewTime(start.Add(15 * time.Second)),
			},
		},
	}

	state := store.NewState()
	mt := manifestutils.NewManifestTargetWithPod(model.Manifest{Name: "server"}, p)
	mt.State.BuildHistory = []model.BuildRecord{{StartTime: start}}
	state.UpsertManifestTarget(mt)

	f.onChange(*state)

	expectedLines := []string{
		"Tracking new pod rollout (pod-id):",
		"     ┊ Scheduled       - 1s",
		"     ┊ Initialized     - 4s",
		"     ┊ Ready           - 5s",
	}
	actualLines := strings.Split(strings.TrimSpace(f.out.String()), "\n")

	require.Equal(t, expectedLines, actualLines)
}

func TestAttachToExistingPod(t *testing.T) {
	f := newPMFixture(t)
	defer f.TearDown()

	start := time.Now()
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
	mt := manifestutils.NewManifestTargetWithPod(model.Manifest{Name: "server"}, p)
	mt.State.BuildHistory = []model.BuildRecord{{StartTime: start.Add(20 * time.Second)}}
	state.UpsertManifestTarget(mt)

	f.onChange(*state)

	// make sure we log every time a build finishes
	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: start.Add(30 * time.Second)})
	state.UpsertManifestTarget(mt)
	f.onChange(*state)

	// two builds, two logs
	msg := "Existing pod still matches build (pod-id)\n\nExisting pod still matches build (pod-id)"
	require.Equal(t, msg, strings.TrimSpace(f.out.String()))
}

func TestAttachToExistingPodDuringActiveBuild(t *testing.T) {
	f := newPMFixture(t)
	defer f.TearDown()

	start := time.Now()
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

	// the manifest knows about the pod, and there is a build in progress
	mt := manifestutils.NewManifestTargetWithPod(model.Manifest{Name: "server"}, p)
	mt.State.BuildHistory = []model.BuildRecord{{StartTime: start}}
	mt.State.CurrentBuild = model.BuildRecord{StartTime: start.Add(15 * time.Second)}
	state.UpsertManifestTarget(mt)
	f.onChange(*state)

	// nothing should be logged while a build is in progress
	require.Equal(t, "", f.out.String())

	mt.State.AddCompletedBuild(mt.State.CurrentBuild)
	mt.State.CurrentBuild = model.BuildRecord{}
	state.UpsertManifestTarget(mt)
	f.onChange(*state)

	// now that the build has finished, we should recognize the pod
	msg := "Existing pod still matches build (pod-id)"
	require.Equal(t, msg, strings.TrimSpace(f.out.String()))
}

type pmFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	pm     *PodMonitor
	cancel func()
	out    *bufsync.ThreadSafeBuffer
	store  *testStore
}

func newPMFixture(t *testing.T) *pmFixture {
	f := tempdir.NewTempDirFixture(t)

	out := bufsync.NewThreadSafeBuffer()
	st := NewTestingStore(out)
	pm := NewPodMonitor()

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)

	return &pmFixture{
		TempDirFixture: f,
		pm:             pm,
		ctx:            ctx,
		cancel:         cancel,
		out:            out,
		store:          st,
	}
}

func (f *pmFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
}

func (f *pmFixture) onChange(state store.EngineState) {
	f.store.SetState(state)
	f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
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

func normalize(s string) string {
	return strings.Replace(s, "\r\n", "\n", -1)
}

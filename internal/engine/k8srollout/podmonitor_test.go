package k8srollout

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)

	f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	assertSnapshot(t, f.out.String())
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

	setBuildFinishTime := func(ft time.Time) {
		mt := manifestutils.NewManifestTargetWithPod(
			model.Manifest{Name: "server"}, p)
		mt.State.BuildHistory = []model.BuildRecord{{FinishTime: ft}}
		state.UpsertManifestTarget(mt)
		f.store.SetState(*state)
	}

	setBuildFinishTime(start.Add(20 * time.Second))
	f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	// a previous implementation of this wouldn't re-log pod info on subsequent builds, so
	// make sure if we attach to an existing pod twice, we get a log each time
	setBuildFinishTime(start.Add(30 * time.Second))
	f.pm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	// two builds, two logs
	msg := "Using existing pod that matches spec (pod-id)\n\nUsing existing pod that matches spec (pod-id)"
	require.Contains(t, f.out.String(), msg)
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
	return strings.Replace(s, "\r\n", "\n", -1)
}

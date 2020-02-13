package k8srollout

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/testutils/manifestutils"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

// NOTE(han): set at runtime with:
// go test -ldflags="-X 'github.com/windmilleng/tilt/internal/engine/k8srollout.PodmonitorWriteGoldenMaster=1'" ./internal/engine/k8srollout
var PodmonitorWriteGoldenMaster = "0"

func TestMonitorReady(t *testing.T) {
	f := newPMFixture(t)
	defer f.TearDown()

	start := time.Now()
	p := store.Pod{
		PodID:     "pod-id",
		StartedAt: start,
		Conditions: []v1.PodCondition{
			v1.PodCondition{
				Type:               v1.PodScheduled,
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: start.Add(time.Second)},
			},
			v1.PodCondition{
				Type:               v1.PodInitialized,
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: start.Add(5 * time.Second)},
			},
			v1.PodCondition{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: start.Add(10 * time.Second)},
			},
		},
	}

	state := store.NewState()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.SetState(*state)

	f.pm.OnChange(f.ctx, f.store)

	assertSnapshot(t, f.out.String())
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

	if string(expected) != output {
		t.Errorf("Expected: %s != Output: %s", expected, output)
	}
}

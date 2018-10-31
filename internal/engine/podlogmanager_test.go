package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"k8s.io/api/core/v1"
)

func TestLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.PodLogs = "hello world!"

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.ManifestStates["server"] = &store.ManifestState{
		Manifest: model.Manifest{Name: "server"},
		Pod: store.Pod{
			PodID:         "pod-id",
			ContainerName: "cname",
			ContainerID:   "cid",
			Phase:         v1.PodRunning,
		},
	}
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	err := f.out.WaitUntilContains("hello world!", time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogActions(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.PodLogs = "hello world!\ngoodbye world!\n"

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.ManifestStates["server"] = &store.ManifestState{
		Manifest: model.Manifest{Name: "server"},
		Pod: store.Pod{
			PodID:         "pod-id",
			ContainerName: "cname",
			ContainerID:   "cid",
			Phase:         v1.PodRunning,
		},
	}
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	f.ConsumeLogActionsUntil("hello world!")
}

func TestLogsFailed(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.ContainerLogsError = fmt.Errorf("my-error")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.ManifestStates["server"] = &store.ManifestState{
		Manifest: model.Manifest{Name: "server"},
		Pod: store.Pod{
			PodID:         "pod-id",
			ContainerName: "cname",
			ContainerID:   "cid",
			Phase:         v1.PodRunning,
		},
	}
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	err := f.out.WaitUntilContains("Error streaming server logs", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, f.out.String(), "my-error")
}

func TestLogsCanceledUnexpectedly(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.PodLogs = "hello world!\n"

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.ManifestStates["server"] = &store.ManifestState{
		Manifest: model.Manifest{Name: "server"},
		Pod: store.Pod{
			PodID:         "pod-id",
			ContainerName: "cname",
			ContainerID:   "cid",
			Phase:         v1.PodRunning,
		},
	}
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	err := f.out.WaitUntilContains("hello world!\n", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	f.kClient.PodLogs = "goodbye world!\n"
	f.plm.OnChange(f.ctx, f.store)
	err = f.out.WaitUntilContains("goodbye world!\n", time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

type plmFixture struct {
	*tempdir.TempDirFixture
	ctx     context.Context
	kClient *k8s.FakeK8sClient
	plm     *PodLogManager
	cancel  func()
	out     *bufsync.ThreadSafeBuffer
	store   *store.Store
}

func newPLMFixture(t *testing.T) *plmFixture {
	f := tempdir.NewTempDirFixture(t)
	kClient := k8s.NewFakeK8sClient()

	out := bufsync.NewThreadSafeBuffer()
	reducer := func(ctx context.Context, state *store.EngineState, action store.Action) {
		podLog, ok := action.(PodLogAction)
		if !ok {
			t.Errorf("Expected action type PodLogAction. Actual: %T", action)
		}
		out.Write(podLog.Log)
	}

	st := store.NewStore(store.Reducer(reducer))
	plm := NewPodLogManager(kClient)

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)
	go st.Loop(ctx)

	return &plmFixture{
		TempDirFixture: f,
		kClient:        kClient,
		plm:            plm,
		ctx:            ctx,
		cancel:         cancel,
		out:            out,
		store:          st,
	}
}

func (f *plmFixture) ConsumeLogActionsUntil(expected string) {
	start := time.Now()
	for time.Since(start) < time.Second {
		f.store.RLockState()
		done := strings.Contains(f.out.String(), expected)
		f.store.RUnlockState()

		if done {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	f.T().Fatalf("Timeout. Collected output: %s", f.out.String())
}

func (f *plmFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
}

package engine

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"k8s.io/api/core/v1"
)

var podID = k8s.PodID("pod-id")
var cName = container.Name("cname")
var cID = container.ID("cid")

func TestLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
			ContainerName: cName,
			ContainerID:   cID,
			Phase:         v1.PodRunning,
		}))
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

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
			ContainerName: cName,
			ContainerID:   cID,
			Phase:         v1.PodRunning,
		}))
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
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
			ContainerName: cName,
			ContainerID:   cID,
			Phase:         v1.PodRunning,
		}))
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

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\n")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
			ContainerName: cName,
			ContainerID:   cID,
			Phase:         v1.PodRunning,
		}))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	err := f.out.WaitUntilContains("hello world!\n", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	f.kClient.SetLogsForPodContainer(podID, cName, "goodbye world!\n")
	f.plm.OnChange(f.ctx, f.store)
	err = f.out.WaitUntilContains("goodbye world!\n", time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogsTruncatedWhenCanceled(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	logs := bytes.NewBuffer(nil)
	f.kClient.PodLogsByPodAndContainer[k8s.PodAndCName{PID: podID, CName: cName}] = k8s.BufferCloser{Buffer: logs}

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
			ContainerName: cName,
			ContainerID:   cID,
			Phase:         v1.PodRunning,
		}))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	logs.Write([]byte("hello world!\n"))
	err := f.out.WaitUntilContains("hello world!\n", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	state = f.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
			ContainerName: cName,
			ContainerID:   "",
			Phase:         v1.PodRunning,
		}))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)

	logs.Write([]byte("goodbye world!\n"))
	time.Sleep(10 * time.Millisecond)

	assert.NotContains(t, f.out.String(), "goodbye")
}

func TestMultiContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	pID := k8s.PodID("the-pod")
	f.kClient.SetLogsForPodContainer(pID, "cont1", "hello world!")
	f.kClient.SetLogsForPodContainer(pID, "cont2", "goodbye world!")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         pID,
			ContainerName: "cont1",
			ContainerID:   "cid1",
			Phase:         v1.PodRunning,
			ContainerInfos: []store.ContainerInfo{
				store.ContainerInfo{ID: "cid1", Name: "cont1"},
				store.ContainerInfo{ID: "cid2", Name: "cont2"},
			},
		}))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	err := f.out.WaitUntilContains("hello world!", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = f.out.WaitUntilContains("goodbye world!", time.Second)
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

	st := store.NewStore(store.Reducer(reducer), store.LogActionsFlag(false))
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

func newManifestTargetWithPod(m model.Manifest, pod store.Pod) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	mt.State.PodSet = store.NewPodSet(pod)
	return mt
}

package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
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
	f.AssertOutputContains("hello world!")
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
	f.AssertOutputContains("Error streaming server logs")
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
	f.AssertOutputContains("hello world!\n")

	// Previous log stream has finished, so the first pod watch has been canceled,
	// but not cleaned up; check that we start a new watch .OnChange
	f.kClient.SetLogsForPodContainer(podID, cName, "goodbye world!\n")
	f.plm.OnChange(f.ctx, f.store)
	f.AssertOutputContains("goodbye world!\n")
}

func TestMultiContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")
	f.kClient.SetLogsForPodContainer(podID, "cont2", "goodbye world!")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:         podID,
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
	f.AssertOutputContains("hello world!")
	f.AssertOutputContains("goodbye world!")
}

func TestContainerPrefixes(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	pID1 := k8s.PodID("pod1")
	cNamePrefix1 := container.Name("yes-prefix-1")
	cNamePrefix2 := container.Name("yes-prefix-2")
	f.kClient.SetLogsForPodContainer(pID1, cNamePrefix1, "hello world!")
	f.kClient.SetLogsForPodContainer(pID1, cNamePrefix2, "goodbye world!")

	pID2 := k8s.PodID("pod2")
	cNameNoPrefix := container.Name("no-prefix")
	f.kClient.SetLogsForPodContainer(pID2, cNameNoPrefix, "hello jupiter!")

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "multiContainer"},
		// Pod with multiple containers -- logs should be prefixed with container name
		store.Pod{
			PodID:         pID1,
			ContainerName: cNamePrefix1,
			ContainerID:   "cid1",
			Phase:         v1.PodRunning,
			ContainerInfos: []store.ContainerInfo{
				store.ContainerInfo{ID: "cid1", Name: cNamePrefix1},
				store.ContainerInfo{ID: "cid2", Name: cNamePrefix2},
			},
		}))
	state.UpsertManifestTarget(newManifestTargetWithPod(
		model.Manifest{Name: "singleContainer"},
		// Pod with just one container -- logs should NOT be prefixed with container name
		store.Pod{
			PodID:         pID2,
			ContainerName: cNameNoPrefix,
			ContainerID:   "cid3",
			Phase:         v1.PodRunning,
		}))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)

	// Make sure we have expected logs
	f.AssertOutputContains("hello world!")
	f.AssertOutputContains("goodbye world!")
	f.AssertOutputContains("hello jupiter!")

	// Check for un/expected prefixes
	f.AssertOutputContains(cNamePrefix1.String())
	f.AssertOutputContains(cNamePrefix2.String())
	f.AssertOutputDoesNotContain(cNameNoPrefix.String())
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
		out.Write(podLog.logEvent.message)
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

func (f *plmFixture) AssertOutputContains(s string) {
	err := f.out.WaitUntilContains(s, time.Second)
	if err != nil {
		f.T().Fatal(err)
	}
}

func (f *plmFixture) AssertOutputDoesNotContain(s string) {
	assert.NotContains(f.T(), f.out.String(), s)
}

func newManifestTargetWithPod(m model.Manifest, pod store.Pod) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	mt.State.PodSet = store.NewPodSet(pod)
	return mt
}

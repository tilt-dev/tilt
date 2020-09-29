package runtimelog

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/internal/testutils"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils/manifestutils"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var podID = k8s.PodID("pod-id")
var cName = container.Name("cname")
var cID = container.ID("cid")

func TestLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	start := time.Now()
	state := f.store.LockMutableStateForTesting()
	state.TiltStartTime = start

	p := store.Pod{
		PodID:      podID,
		Containers: []store.Container{NewRunningContainer(cName, cID)},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	f.AssertOutputContains("hello world!")
	assert.Equal(t, start, f.kClient.LastPodLogStartTime)
}

func TestLogActions(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	state := f.store.LockMutableStateForTesting()

	p := store.Pod{
		PodID:      podID,
		Containers: []store.Container{NewRunningContainer(cName, cID)},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.UnlockMutableState()

	f.plm.OnChange(f.ctx, f.store)
	f.ConsumeLogActionsUntil("hello world!")
}

func TestLogsFailed(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.ContainerLogsError = fmt.Errorf("my-error")

	state := f.store.LockMutableStateForTesting()

	p := store.Pod{
		PodID:      podID,
		Containers: []store.Container{NewRunningContainer(cName, cID)},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
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

	p := store.Pod{
		PodID:      podID,
		Containers: []store.Container{NewRunningContainer(cName, cID)},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
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

	p := store.Pod{
		PodID: podID,
		Containers: []store.Container{
			NewRunningContainer("cont1", "cid1"),
			NewRunningContainer("cont2", "cid2"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
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

	podMultiC := store.Pod{
		PodID: pID1,
		Containers: []store.Container{
			// Pod with multiple containers -- logs should be prefixed with container name
			NewRunningContainer(cNamePrefix1, "cid1"),
			NewRunningContainer(cNamePrefix2, "cid2"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "multiContainer"}, podMultiC))

	podSingleC := store.Pod{
		PodID: pID2,
		Containers: []store.Container{
			// Pod with just one container -- logs should NOT be prefixed with container name
			NewRunningContainer(cNameNoPrefix, "cid3"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "singleContainer"},
		podSingleC))
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

func TestTerminatedContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()

	cName := container.Name("cName")
	p := store.Pod{
		PodID: podID,
		Containers: []store.Container{
			NewTerminatedContainer(cName, "cID"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.UnlockMutableState()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	// Fire OnChange twice, because we used to have a bug where
	// we'd immediately teardown the log watch on the terminated container.
	f.plm.OnChange(f.ctx, f.store)
	f.plm.OnChange(f.ctx, f.store)

	f.AssertOutputContains("hello world!")

	// Make sure that we don't try to re-stream after the terminated container
	// closes the log stream.
	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	f.plm.OnChange(f.ctx, f.store)
	f.AssertOutputContains("hello world!")
	f.AssertOutputDoesNotContain("goodbye world!")
}

func TestInitContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")

	state := f.store.LockMutableStateForTesting()

	cNameInit := container.Name("cNameInit")
	cNameNormal := container.Name("cNameNormal")
	p := store.Pod{
		PodID: podID,
		InitContainers: []store.Container{
			NewTerminatedContainer(cNameInit, "cID-init"),
		},
		Containers: []store.Container{
			NewRunningContainer(cNameNormal, "cID-normal"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.UnlockMutableState()

	f.kClient.SetLogsForPodContainer(podID, cNameInit, "init world!")
	f.kClient.SetLogsForPodContainer(podID, cNameNormal, "hello world!")

	f.plm.OnChange(f.ctx, f.store)

	f.AssertOutputContains(cNameInit.String())
	f.AssertOutputContains("init world!")
	f.AssertOutputDoesNotContain(cNameNormal.String())
	f.AssertOutputContains("hello world!")
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
		event, ok := action.(store.LogAction)
		if !ok {
			t.Errorf("Expected action type LogAction. Actual: %T", action)
		}
		_, err := out.Write(event.Message())
		if err != nil {
			fmt.Printf("error writing event: %v\n", err)
		}
	}

	st := store.NewStore(store.Reducer(reducer), store.LogActionsFlag(false))
	plm := NewPodLogManager(kClient)

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)
	go func() {
		err := st.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()

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
	f.kClient.TearDown()
	f.TempDirFixture.TearDown()
}

func (f *plmFixture) AssertOutputContains(s string) {
	err := f.out.WaitUntilContains(s, time.Second)
	if err != nil {
		f.T().Fatal(err)
	}
}

func (f *plmFixture) AssertOutputDoesNotContain(s string) {
	time.Sleep(10 * time.Millisecond)
	assert.NotContains(f.T(), f.out.String(), s)
}

func NewRunningContainer(name container.Name, id container.ID) store.Container {
	return store.Container{Name: name, ID: id, Running: true}
}
func NewTerminatedContainer(name container.Name, id container.ID) store.Container {
	return store.Container{Name: name, ID: id, Terminated: true}
}

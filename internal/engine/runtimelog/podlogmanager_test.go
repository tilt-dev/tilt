package runtimelog

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

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

	f.onChange(podID)
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

	f.onChange(podID)
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

	f.onChange(podID)
	f.AssertOutputContains("Error streaming pod-id logs")
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

	f.onChange(podID)
	f.AssertOutputContains("hello world!\n")

	// Previous log stream has finished, so the first pod watch has been canceled,
	// but not cleaned up; check that we start a new watch .OnChange
	f.kClient.SetLogsForPodContainer(podID, cName, "goodbye world!\n")
	f.onChange(podID)
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

	f.onChange(podID)
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

	f.onChange(podID)

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
	f.onChange(podID)
	f.onChange(podID)

	f.AssertOutputContains("hello world!")

	// Make sure that we don't try to re-stream after the terminated container
	// closes the log stream.
	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	f.onChange(podID)
	f.AssertOutputContains("hello world!")
	f.AssertOutputDoesNotContain("goodbye world!")
}

// https://github.com/tilt-dev/tilt/issues/3908
func TestLogReconnection(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()

	cName := container.Name("cName")
	p := store.Pod{
		PodID: podID,
		Containers: []store.Container{
			NewRunningContainer(cName, "cID"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.UnlockMutableState()

	reader, writer := io.Pipe()
	defer writer.Close()
	f.kClient.SetLogReaderForPodContainer(podID, cName, reader)

	// Set up fake time
	startTime := time.Now()
	currentTime := startTime.Add(5 * time.Second)
	timeCh := make(chan time.Time)
	ticker := time.Ticker{C: timeCh}
	f.plsc.now = func() time.Time { return currentTime }
	f.plsc.since = func(t time.Time) time.Duration { return currentTime.Sub(t) }
	f.plsc.newTicker = func(d time.Duration) *time.Ticker { return &ticker }

	f.store.WithState(func(state *store.EngineState) {
		state.TiltStartTime = startTime
	})

	f.onChange(podID)

	_, _ = writer.Write([]byte("hello world!"))
	f.AssertOutputContains("hello world!")
	assert.Equal(t, startTime, f.kClient.LastPodLogStartTime)

	currentTime = currentTime.Add(20 * time.Second)
	lastRead := currentTime
	_, _ = writer.Write([]byte("hello world2!"))
	f.AssertOutputContains("hello world2!")

	// Simulate Kubernetes rotating the logs by creating a new pipe.
	reader2, writer2 := io.Pipe()
	defer writer2.Close()
	f.kClient.SetLogReaderForPodContainer(podID, cName, reader2)
	go func() {
		_, _ = writer2.Write([]byte("goodbye world!"))
	}()
	f.AssertOutputDoesNotContain("goodbye world!")

	currentTime = currentTime.Add(5 * time.Second)
	timeCh <- currentTime
	f.AssertOutputDoesNotContain("goodbye world!")

	currentTime = currentTime.Add(5 * time.Second)
	timeCh <- currentTime
	f.AssertOutputDoesNotContain("goodbye world!")
	assert.Equal(t, startTime, f.kClient.LastPodLogStartTime)

	// simulate 15s since we last read a log; this triggers a reconnect
	currentTime = currentTime.Add(5 * time.Second)
	timeCh <- currentTime
	time.Sleep(20 * time.Millisecond)
	assert.Error(t, f.kClient.LastPodLogContext.Err())
	writer.Close()

	f.AssertOutputContains("goodbye world!")

	// Make sure the start time was adjusted for when the last read happened.
	assert.Equal(t, lastRead.Add(podLogReconnectGap), f.kClient.LastPodLogStartTime)
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

	f.onChange(podID)

	f.AssertOutputContains(cNameInit.String())
	f.AssertOutputContains("init world!")
	f.AssertOutputDoesNotContain(cNameNormal.String())
	f.AssertOutputContains("hello world!")
}

func TestIstioContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")

	state := f.store.LockMutableStateForTesting()

	istioInit := IstioInitContainerName
	istioSidecar := IstioSidecarContainerName
	cNormal := container.Name("cNameNormal")
	p := store.Pod{
		PodID: podID,
		InitContainers: []store.Container{
			NewTerminatedContainer(istioInit, "cID-init"),
		},
		Containers: []store.Container{
			NewRunningContainer(istioSidecar, "cID-sidecar"),
			NewRunningContainer(cNormal, "cID-normal"),
		},
	}
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, p))
	f.store.UnlockMutableState()

	f.kClient.SetLogsForPodContainer(podID, istioInit, "init istio!")
	f.kClient.SetLogsForPodContainer(podID, istioSidecar, "hello istio!")
	f.kClient.SetLogsForPodContainer(podID, cNormal, "hello world!")

	f.onChange(podID)

	f.AssertOutputDoesNotContain("istio")
	f.AssertOutputContains("hello world!")
}

type plmStore struct {
	t *testing.T
	*store.TestingStore
	out *bufsync.ThreadSafeBuffer

	mu      sync.Mutex
	streams map[string]*PodLogStream
	summary store.ChangeSummary
}

func newPLMStore(t *testing.T, out *bufsync.ThreadSafeBuffer) *plmStore {
	return &plmStore{
		t:            t,
		TestingStore: store.NewTestingStore(),
		out:          out,
		streams:      make(map[string]*PodLogStream),
	}
}

func (s *plmStore) getSummary() store.ChangeSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.summary
}

func (s *plmStore) clearSummary() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = store.ChangeSummary{}
}

func (s *plmStore) Dispatch(action store.Action) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.LockMutableStateForTesting()
	defer s.UnlockMutableState()

	switch action := action.(type) {
	case PodLogStreamCreateAction:
		state.PodLogStreams[action.PodLogStream.Name] = action.PodLogStream
		action.Summarize(&s.summary)
		return
	case PodLogStreamDeleteAction:
		delete(state.PodLogStreams, action.Name)
		action.Summarize(&s.summary)
		return
	}

	event, ok := action.(store.LogAction)
	if !ok {
		s.t.Errorf("Expected action type LogAction. Actual: %T", action)
	}

	_, err := s.out.Write(event.Message())
	if err != nil {
		fmt.Printf("error writing event: %v\n", err)
	}
}

type plmFixture struct {
	*tempdir.TempDirFixture
	ctx     context.Context
	kClient *k8s.FakeK8sClient
	plm     *PodLogManager
	plsc    *PodLogStreamController
	cancel  func()
	out     *bufsync.ThreadSafeBuffer
	store   *plmStore
}

func newPLMFixture(t *testing.T) *plmFixture {
	f := tempdir.NewTempDirFixture(t)
	kClient := k8s.NewFakeK8sClient()

	out := bufsync.NewThreadSafeBuffer()
	st := newPLMStore(t, out)
	plm := NewPodLogManager()
	plsc := NewPodLogStreamController(st, kClient)

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)

	return &plmFixture{
		TempDirFixture: f,
		kClient:        kClient,
		plm:            plm,
		plsc:           plsc,
		ctx:            ctx,
		cancel:         cancel,
		out:            out,
		store:          st,
	}
}

func (f *plmFixture) onChange(podID k8s.PodID) {
	f.plm.OnChange(f.ctx, f.store, store.ChangeSummary{
		Pods: store.NewChangeSet(types.NamespacedName{Name: string(podID)}),
	})
	f.plsc.OnChange(f.ctx, f.store, f.store.getSummary())
	f.store.clearSummary()
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

package podlogstream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/internal/engine/k8swatch"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/manifestutils"
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

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.onChange("server", podID)
	f.AssertOutputContains("hello world!")
	f.AssertLogStartTime(start)

	// Check to make sure that we're enqueuing pod changes as Reconcile() calls.
	podNN := types.NamespacedName{Name: string(podID), Namespace: "default"}
	streamNN := types.NamespacedName{Name: fmt.Sprintf("default-%s", podID)}
	assert.Equal(t, []reconcile.Request{
		reconcile.Request{NamespacedName: streamNN},
	}, f.plsc.podSource.indexer.EnqueueKey(indexer.Key{Name: podNN, GVK: podGVK}))
}

func TestLogActions(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	state := f.store.LockMutableStateForTesting()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.onChange("server", podID)
	f.ConsumeLogActionsUntil("hello world!")
}

func TestLogsFailed(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.ContainerLogsError = fmt.Errorf("my-error")

	state := f.store.LockMutableStateForTesting()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.onChange("server", podID)
	f.AssertOutputContains("Error streaming pod-id logs")
	assert.Contains(t, f.out.String(), "my-error")

	// Check to make sure the status has an error.
	stream := f.getPodLogStream(podID)
	assert.Equal(t, stream.Status, PodLogStreamStatus{
		ContainerStatuses: []ContainerLogStreamStatus{
			ContainerLogStreamStatus{
				Name:  "cname",
				Error: "my-error",
			},
		},
	})
}

func TestLogsCanceledUnexpectedly(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\n")

	state := f.store.LockMutableStateForTesting()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.onChange("server", podID)
	f.AssertOutputContains("hello world!\n")

	// Wait until the previous log stream finishes.
	assert.Eventually(f.T(), func() bool {
		stream := f.getPodLogStream(podID)
		statuses := stream.Status.ContainerStatuses
		if len(statuses) != 1 {
			return false
		}
		return !statuses[0].Active
	}, time.Second, 5*time.Millisecond)

	// Set new logs, as if the pod restarted.
	f.kClient.SetLogsForPodContainer(podID, cName, "goodbye world!\n")
	f.onChange("server", podID)
	f.AssertOutputContains("goodbye world!\n")
}

func TestMultiContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")
	f.kClient.SetLogsForPodContainer(podID, "cont2", "goodbye world!")

	state := f.store.LockMutableStateForTesting()

	pb := newPodBuilder(podID).
		addRunningContainer("cont1", "cid1").
		addRunningContainer("cont2", "cid2")
	f.kClient.UpsertPod(pb.toPod())
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.onChange("server", podID)
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

	pbMultiC := newPodBuilder(pID1).
		// Pod with multiple containers -- logs should be prefixed with container name
		addRunningContainer(cNamePrefix1, "cid1").
		addRunningContainer(cNamePrefix2, "cid2")
	f.kClient.UpsertPod(pbMultiC.toPod())

	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "multiContainer"}, pbMultiC.toStorePod(f.ctx)))

	pbSingleC := newPodBuilder(pID2).
		// Pod with just one container -- logs should NOT be prefixed with container name
		addRunningContainer(cNameNoPrefix, "cid3")
	f.kClient.UpsertPod(pbSingleC.toPod())

	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "singleContainer"},
		pbSingleC.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.onChange("singleContainer", pID1)
	f.onChange("singleContainer", pID2)

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
	pb := newPodBuilder(podID).addTerminatedContainer(cName, "cID")
	f.kClient.UpsertPod(pb.toPod())
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	// Fire OnChange twice, because we used to have a bug where
	// we'd immediately teardown the log watch on the terminated container.
	f.onChange("server", podID)
	f.onChange("server", podID)

	f.AssertOutputContains("hello world!")

	// Make sure that we don't try to re-stream after the terminated container
	// closes the log stream.
	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	f.onChange("server", podID)
	f.AssertOutputContains("hello world!")
	f.AssertOutputDoesNotContain("goodbye world!")
}

// https://github.com/tilt-dev/tilt/issues/3908
func TestLogReconnection(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()

	cName := container.Name("cName")
	pb := newPodBuilder(podID).addRunningContainer(cName, "cID")
	f.kClient.UpsertPod(pb.toPod())
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
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

	f.onChange("server", podID)

	_, _ = writer.Write([]byte("hello world!"))
	f.AssertOutputContains("hello world!")
	f.AssertLogStartTime(startTime)

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
	f.AssertLogStartTime(startTime)

	// simulate 15s since we last read a log; this triggers a reconnect
	currentTime = currentTime.Add(5 * time.Second)
	timeCh <- currentTime
	time.Sleep(20 * time.Millisecond)
	assert.Error(t, f.kClient.LastPodLogContext.Err())
	writer.Close()

	f.AssertOutputContains("goodbye world!")

	// Make sure the start time was adjusted for when the last read happened.
	f.AssertLogStartTime(lastRead.Add(podLogReconnectGap))
}

func TestInitContainerLogs(t *testing.T) {
	f := newPLMFixture(t)
	defer f.TearDown()

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")

	state := f.store.LockMutableStateForTesting()

	cNameInit := container.Name("cNameInit")
	cNameNormal := container.Name("cNameNormal")
	pb := newPodBuilder(podID).
		addTerminatedInitContainer(cNameInit, "cID-init").
		addRunningContainer(cNameNormal, "cID-normal")
	f.kClient.UpsertPod(pb.toPod())

	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.kClient.SetLogsForPodContainer(podID, cNameInit, "init world!")
	f.kClient.SetLogsForPodContainer(podID, cNameNormal, "hello world!")

	f.onChange("server", podID)

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

	istioInit := runtimelog.IstioInitContainerName
	istioSidecar := runtimelog.IstioSidecarContainerName
	cNormal := container.Name("cNameNormal")
	pb := newPodBuilder(podID).
		addTerminatedInitContainer(istioInit, "cID-init").
		addRunningContainer(istioSidecar, "cID-sidecar").
		addRunningContainer(cNormal, "cID-normal")
	f.kClient.UpsertPod(pb.toPod())

	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"}, pb.toStorePod(f.ctx)))
	f.store.UnlockMutableState()

	f.kClient.SetLogsForPodContainer(podID, istioInit, "init istio!")
	f.kClient.SetLogsForPodContainer(podID, istioSidecar, "hello istio!")
	f.kClient.SetLogsForPodContainer(podID, cNormal, "hello world!")

	f.onChange("server", podID)

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
	case runtimelog.PodLogStreamCreateAction:
		state.PodLogStreams[action.PodLogStream.Name] = action.PodLogStream
		action.Summarize(&s.summary)
		return
	case runtimelog.PodLogStreamDeleteAction:
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
	client  ctrlclient.Client
	kClient *k8s.FakeK8sClient
	plm     *runtimelog.PodLogManager
	plsc    *Controller
	cancel  func()
	out     *bufsync.ThreadSafeBuffer
	store   *plmStore
}

func newPLMFixture(t *testing.T) *plmFixture {
	f := tempdir.NewTempDirFixture(t)
	kClient := k8s.NewFakeK8sClient(t)

	out := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(out))

	st := newPLMStore(t, out)
	fc := fake.NewFakeTiltClient()
	plm := runtimelog.NewPodLogManager(fc)
	podSource := NewPodSource(ctx, kClient, v1alpha1.NewScheme())
	plsc := NewController(ctx, fc, st, kClient, podSource)

	return &plmFixture{
		TempDirFixture: f,
		kClient:        kClient,
		client:         fc,
		plm:            plm,
		plsc:           plsc,
		ctx:            ctx,
		cancel:         cancel,
		out:            out,
		store:          st,
	}
}

func (f *plmFixture) onChange(mn model.ManifestName, podID k8s.PodID) {
	podNN := types.NamespacedName{Name: string(podID), Namespace: "default"}
	kdNN := k8swatch.KeyForManifest(mn)
	_ = f.plm.OnChange(f.ctx, f.store, store.ChangeSummary{
		KubernetesDiscoveries: store.NewChangeSet(kdNN),
	})

	// Reconcile any PodLogStreams
	summary := f.store.getSummary()
	for streamName := range summary.PodLogStreams.Changes {
		_, err := f.plsc.Reconcile(f.ctx, reconcile.Request{NamespacedName: streamName})
		assert.NoError(f.T(), err)
	}

	reqs := f.plsc.podSource.indexer.EnqueueKey(indexer.Key{Name: podNN, GVK: podGVK})
	for _, req := range reqs {
		_, err := f.plsc.Reconcile(f.ctx, req)
		assert.NoError(f.T(), err)
	}

	f.store.clearSummary()
}

func (f *plmFixture) getPodLogStream(id k8s.PodID) *PodLogStream {
	stream := &PodLogStream{}
	streamNN := types.NamespacedName{Name: fmt.Sprintf("default-%s", id)}
	err := f.client.Get(f.ctx, streamNN, stream)
	require.NoError(f.T(), err)
	return stream
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
	f.T().Helper()
	err := f.out.WaitUntilContains(s, time.Second)
	if err != nil {
		f.T().Fatal(err)
	}
}

func (f *plmFixture) AssertOutputDoesNotContain(s string) {
	time.Sleep(10 * time.Millisecond)
	assert.NotContains(f.T(), f.out.String(), s)
}

func (f *plmFixture) AssertLogStartTime(t time.Time) {
	f.T().Helper()

	// Truncate the time to match the behavior of metav1.Time
	assert.Equal(f.T(), t.Truncate(time.Second), f.kClient.LastPodLogStartTime)
}

type podBuilder v1.Pod

func newPodBuilder(id k8s.PodID) *podBuilder {
	return (*podBuilder)(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(id),
			Namespace: "default",
		},
	})
}

func (pb *podBuilder) addRunningContainer(name container.Name, id container.ID) *podBuilder {
	pb.Spec.Containers = append(pb.Spec.Containers, v1.Container{
		Name: string(name),
	})
	pb.Status.ContainerStatuses = append(pb.Status.ContainerStatuses, v1.ContainerStatus{
		Name:        string(name),
		ContainerID: fmt.Sprintf("containerd://%s", id),
		Image:       fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		ImageID:     fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		Ready:       true,
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: metav1.Now(),
			},
		},
	})
	return pb
}

func (pb *podBuilder) addRunningInitContainer(name container.Name, id container.ID) *podBuilder {
	pb.Spec.InitContainers = append(pb.Spec.InitContainers, v1.Container{
		Name: string(name),
	})
	pb.Status.InitContainerStatuses = append(pb.Status.InitContainerStatuses, v1.ContainerStatus{
		Name:        string(name),
		ContainerID: fmt.Sprintf("containerd://%s", id),
		Image:       fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		ImageID:     fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		Ready:       true,
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: metav1.Now(),
			},
		},
	})
	return pb
}

func (pb *podBuilder) addTerminatedContainer(name container.Name, id container.ID) *podBuilder {
	pb.addRunningContainer(name, id)
	statuses := pb.Status.ContainerStatuses
	statuses[len(statuses)-1].State.Running = nil
	statuses[len(statuses)-1].State.Terminated = &v1.ContainerStateTerminated{
		StartedAt: metav1.Now(),
	}
	return pb
}

func (pb *podBuilder) addTerminatedInitContainer(name container.Name, id container.ID) *podBuilder {
	pb.addRunningInitContainer(name, id)
	statuses := pb.Status.InitContainerStatuses
	statuses[len(statuses)-1].State.Running = nil
	statuses[len(statuses)-1].State.Terminated = &v1.ContainerStateTerminated{
		StartedAt: metav1.Now(),
	}
	return pb
}

func (pb *podBuilder) toPod() *v1.Pod {
	return (*v1.Pod)(pb)
}

func (pb *podBuilder) toStorePod(ctx context.Context) v1alpha1.Pod {
	pod := (*v1.Pod)(pb)
	return v1alpha1.Pod{
		Name:           pb.Name,
		Namespace:      pb.Namespace,
		InitContainers: k8sconv.PodContainers(ctx, pod, pod.Status.InitContainerStatuses),
		Containers:     k8sconv.PodContainers(ctx, pod, pod.Status.ContainerStatuses),
	}
}

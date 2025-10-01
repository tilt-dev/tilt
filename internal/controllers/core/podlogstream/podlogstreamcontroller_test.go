package podlogstream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var podID = k8s.PodID("pod-id")
var cName = container.Name("cname")
var cID = container.ID("cid")

func TestLogs(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	start := f.clock.Now()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	pls := plsFromPod("server", pb, start)
	f.Create(pls)

	f.triggerPodEvent(podID)
	f.AssertOutputContains("hello world!")
	f.AssertLogStartTime(start)

	// Check to make sure that we're enqueuing pod changes as Reconcile() calls.
	podNN := types.NamespacedName{Name: string(podID), Namespace: "default"}
	streamNN := types.NamespacedName{Name: fmt.Sprintf("default-%s", podID)}
	assert.Equal(t, []reconcile.Request{
		{NamespacedName: streamNN},
	}, f.plsc.podSource.indexer.EnqueueKey(indexer.Key{Name: podNN, GVK: podGVK}))
}

func TestLogCleanup(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	start := f.clock.Now()
	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	pls := plsFromPod("server", pb, start)
	f.Create(pls)

	f.triggerPodEvent(podID)
	f.AssertOutputContains("hello world!")

	f.Delete(pls)
	assert.Len(t, f.plsc.watches, 0)

	// TODO(nick): Currently, namespace watches are never cleanedup,
	// because the user might restart them again.
	assert.Len(t, f.plsc.podSource.watchesByNamespace, 1)
}

func TestLogActions(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	f.Create(plsFromPod("server", pb, time.Time{}))

	f.triggerPodEvent(podID)
	f.ConsumeLogActionsUntil("hello world!")
}

func TestLogsFailed(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.ContainerLogsError = fmt.Errorf("my-error")

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	pls := plsFromPod("server", pb, time.Time{})
	f.Create(pls)

	f.AssertOutputContains("Error streaming pod-id logs")
	assert.Contains(t, f.out.String(), "my-error")

	require.Eventually(t,
		func() bool {
			// Check to make sure the status has an error.
			f.MustGet(f.KeyForObject(pls), pls)
			return apicmp.DeepEqual(pls.Status,
				PodLogStreamStatus{
					ContainerStatuses: []ContainerLogStreamStatus{
						{
							Name:  "cname",
							Error: "my-error",
						},
					},
				})
		},
		time.Second, 10*time.Millisecond,
		"Expected error not present on PodLogStreamStatus: %v", pls,
	)

	result := f.MustReconcile(f.KeyForObject(pls))
	assert.Equal(t, 2*time.Second, result.RequeueAfter)

	f.clock.Advance(2 * time.Second)

	assert.Eventually(f.t, func() bool {
		result = f.MustReconcile(f.KeyForObject(pls))
		return result.RequeueAfter == 4*time.Second
	}, time.Second, 5*time.Millisecond, "should re-stream and backoff again")
}

func TestLogsCanceledUnexpectedly(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\n")

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())
	pls := plsFromPod("server", pb, time.Time{})
	f.Create(pls)

	f.AssertOutputContains("hello world!\n")

	// Wait until the previous log stream finishes.
	assert.Eventually(f.t, func() bool {
		f.MustGet(f.KeyForObject(pls), pls)
		statuses := pls.Status.ContainerStatuses
		if len(statuses) != 1 {
			return false
		}
		return !statuses[0].Active
	}, time.Second, 5*time.Millisecond)

	// Set new logs, as if the pod restarted.
	f.kClient.SetLogsForPodContainer(podID, cName, "goodbye world!\n")
	f.triggerPodEvent(podID)
	f.clock.Advance(10 * time.Second)
	f.MustReconcile(types.NamespacedName{Name: pls.Name})
	f.AssertOutputContains("goodbye world!\n")
}

func TestMultiContainerLogs(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")
	f.kClient.SetLogsForPodContainer(podID, "cont2", "goodbye world!")

	pb := newPodBuilder(podID).
		addRunningContainer("cont1", "cid1").
		addRunningContainer("cont2", "cid2")
	f.kClient.UpsertPod(pb.toPod())
	f.Create(plsFromPod("server", pb, time.Time{}))

	f.AssertOutputContains("hello world!")
	f.AssertOutputContains("goodbye world!")
}

func TestContainerPrefixes(t *testing.T) {
	f := newPLMFixture(t)

	pID1 := k8s.PodID("pod1")
	cNamePrefix1 := container.Name("yes-prefix-1")
	cNamePrefix2 := container.Name("yes-prefix-2")
	f.kClient.SetLogsForPodContainer(pID1, cNamePrefix1, "hello world!")
	f.kClient.SetLogsForPodContainer(pID1, cNamePrefix2, "goodbye world!")

	pID2 := k8s.PodID("pod2")
	cNameNoPrefix := container.Name("no-prefix")
	f.kClient.SetLogsForPodContainer(pID2, cNameNoPrefix, "hello jupiter!")

	pbMultiC := newPodBuilder(pID1).
		// Pod with multiple containers -- logs should be prefixed with container name
		addRunningContainer(cNamePrefix1, "cid1").
		addRunningContainer(cNamePrefix2, "cid2")
	f.kClient.UpsertPod(pbMultiC.toPod())

	f.Create(plsFromPod("multiContainer", pbMultiC, time.Time{}))

	pbSingleC := newPodBuilder(pID2).
		// Pod with just one container -- logs should NOT be prefixed with container name
		addRunningContainer(cNameNoPrefix, "cid3")
	f.kClient.UpsertPod(pbSingleC.toPod())

	f.Create(plsFromPod("singleContainer", pbSingleC, time.Time{}))

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

	cName := container.Name("cName")
	pb := newPodBuilder(podID).addTerminatedContainer(cName, "cID")
	f.kClient.UpsertPod(pb.toPod())

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	f.Create(plsFromPod("server", pb, time.Time{}))

	// Fire OnChange twice, because we used to have a bug where
	// we'd immediately teardown the log watch on the terminated container.
	f.triggerPodEvent(podID)
	f.triggerPodEvent(podID)

	f.AssertOutputContains("hello world!")

	// Make sure that we don't try to re-stream after the terminated container
	// closes the log stream.
	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	f.triggerPodEvent(podID)
	f.AssertOutputContains("hello world!")
	f.AssertOutputDoesNotContain("goodbye world!")
}

// https://github.com/tilt-dev/tilt/issues/3908
func TestLogReconnection(t *testing.T) {
	f := newPLMFixture(t)
	cName := container.Name("cName")
	pb := newPodBuilder(podID).addRunningContainer(cName, "cID")
	f.kClient.UpsertPod(pb.toPod())

	reader, writer := io.Pipe()
	defer func() {
		require.NoError(t, writer.Close())
	}()
	f.kClient.SetLogReaderForPodContainer(podID, cName, reader)

	// Set up fake time
	startTime := f.clock.Now()
	f.Create(plsFromPod("server", pb, startTime))

	_, err := writer.Write([]byte("hello world!"))
	require.NoError(t, err)
	f.AssertOutputContains("hello world!")
	f.AssertLogStartTime(startTime)

	f.clock.Advance(20 * time.Second)
	lastRead := f.clock.Now()
	_, _ = writer.Write([]byte("hello world2!"))
	f.AssertOutputContains("hello world2!")

	// Simulate Kubernetes rotating the logs by creating a new pipe.
	reader2, writer2 := io.Pipe()
	defer func() {
		require.NoError(t, writer2.Close())
	}()
	f.kClient.SetLogReaderForPodContainer(podID, cName, reader2)
	go func() {
		_, _ = writer2.Write([]byte("goodbye world!"))
	}()
	f.AssertOutputDoesNotContain("goodbye world!")

	f.clock.Advance(5 * time.Second)
	f.AssertOutputDoesNotContain("goodbye world!")

	f.clock.Advance(5 * time.Second)
	f.AssertOutputDoesNotContain("goodbye world!")
	f.AssertLogStartTime(startTime)

	// simulate 15s since we last read a log; this triggers a reconnect
	f.clock.Advance(15 * time.Second)
	assert.Eventually(t, func() bool {
		return f.kClient.LastPodLogContext.Err() != nil
	}, time.Second, time.Millisecond)
	require.NoError(t, writer.Close())

	f.AssertOutputContains("goodbye world!")

	// Make sure the start time was adjusted for when the last read happened.
	f.AssertLogStartTime(lastRead.Add(podLogReconnectGap))
}

func TestInitContainerLogs(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")

	cNameInit := container.Name("cNameInit")
	cNameNormal := container.Name("cNameNormal")
	pb := newPodBuilder(podID).
		addTerminatedInitContainer(cNameInit, "cID-init").
		addRunningContainer(cNameNormal, "cID-normal")
	f.kClient.UpsertPod(pb.toPod())

	f.kClient.SetLogsForPodContainer(podID, cNameInit, "init world!")
	f.kClient.SetLogsForPodContainer(podID, cNameNormal, "hello world!")

	f.Create(plsFromPod("server", pb, time.Time{}))

	f.AssertOutputContains(cNameInit.String())
	f.AssertOutputContains("init world!")
	f.AssertOutputDoesNotContain(cNameNormal.String())
	f.AssertOutputContains("hello world!")
}

func TestIgnoredContainerLogs(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")

	istioInit := container.IstioInitContainerName
	istioSidecar := container.IstioSidecarContainerName
	cNormal := container.Name("cNameNormal")
	pb := newPodBuilder(podID).
		addTerminatedInitContainer(istioInit, "cID-init").
		addRunningContainer(istioSidecar, "cID-sidecar").
		addRunningContainer(cNormal, "cID-normal")
	f.kClient.UpsertPod(pb.toPod())

	f.kClient.SetLogsForPodContainer(podID, istioInit, "init istio!")
	f.kClient.SetLogsForPodContainer(podID, istioSidecar, "hello istio!")
	f.kClient.SetLogsForPodContainer(podID, cNormal, "hello world!")

	pls := plsFromPod("server", pb, time.Time{})
	pls.Spec.IgnoreContainers = []string{string(istioInit), string(istioSidecar)}
	f.Create(pls)

	f.AssertOutputDoesNotContain("istio")
	f.AssertOutputContains("hello world!")
}

// Our old Fake Kubernetes client used to interact badly
// with the pod log stream reconciler, leading to an infinite
// loop in tests.
func TestInfiniteLoop(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")

	pb := newPodBuilder(podID).
		addRunningContainer("cNameNormal", "cID-normal")
	f.kClient.UpsertPod(pb.toPod())

	pls := plsFromPod("server", pb, time.Time{})
	f.Create(pls)

	nn := types.NamespacedName{Name: pls.Name}
	f.MustReconcile(nn)

	// Make sure this goes into an active state and stays there.
	assert.Eventually(t, func() bool {
		var pls v1alpha1.PodLogStream
		f.MustGet(nn, &pls)
		return len(pls.Status.ContainerStatuses) > 0 && pls.Status.ContainerStatuses[0].Active
	}, 200*time.Millisecond, 10*time.Millisecond)

	assert.Never(t, func() bool {
		var pls v1alpha1.PodLogStream
		f.MustGet(nn, &pls)
		return len(pls.Status.ContainerStatuses) == 0 || !pls.Status.ContainerStatuses[0].Active
	}, 200*time.Millisecond, 10*time.Millisecond)

	_ = f.kClient.LastPodLogPipeWriter.CloseWithError(fmt.Errorf("manually closed"))

	assert.Eventually(t, func() bool {
		var pls v1alpha1.PodLogStream
		f.MustGet(nn, &pls)
		if len(pls.Status.ContainerStatuses) == 0 {
			return false
		}
		cst := pls.Status.ContainerStatuses[0]
		return !cst.Active && strings.Contains(cst.Error, "manually closed")
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func TestReconcilerIndexing(t *testing.T) {
	f := newPLMFixture(t)

	pls := plsFromPod("server", newPodBuilder(podID), f.clock.Now())
	pls.Namespace = "some-ns"
	pls.Spec.Cluster = "my-cluster"
	f.Create(pls)

	ctx := context.Background()
	reqs := f.plsc.indexer.Enqueue(ctx, &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "my-cluster"},
	})
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: "some-ns", Name: "default-pod-id"}},
	}, reqs)
}

func TestDeletionTimestamp(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	start := f.clock.Now()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID).addDeletionTimestamp()
	f.kClient.UpsertPod(pb.toPod())

	pls := plsFromPod("server", pb, start)
	f.Create(pls)

	f.triggerPodEvent(podID)

	nn := types.NamespacedName{Name: pls.Name}
	f.MustReconcile(nn)

	f.AssertOutputContains("hello world!")

	assert.Eventually(f.t, func() bool {
		f.Get(nn, pls)
		return len(pls.Status.ContainerStatuses) == 1 && !pls.Status.ContainerStatuses[0].Active
	}, time.Second, 5*time.Millisecond, "should stream then stop")

	// No log streams should be active.
	assert.Equal(t, pls.Status, v1alpha1.PodLogStreamStatus{
		ContainerStatuses: []v1alpha1.ContainerLogStreamStatus{
			v1alpha1.ContainerLogStreamStatus{Name: "cname"},
		},
	})

	// The cname stream is closed forever.
	assert.Len(t, f.plsc.hasClosedStream, 1)
}

func TestMissingPod(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!")

	start := f.clock.Now()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	pls := plsFromPod("server", pb, start)
	nn := types.NamespacedName{Name: pls.Name}
	result := f.Create(pls)
	assert.Equal(t, time.Second, result.RequeueAfter)

	result = f.MustReconcile(nn)
	assert.Equal(t, 2*time.Second, result.RequeueAfter)

	f.Get(nn, pls)
	assert.Equal(t, "pod not found: default/pod-id", pls.Status.Error)

	f.kClient.UpsertPod(pb.toPod())

	result = f.MustReconcile(nn)
	assert.Equal(t, time.Duration(0), result.RequeueAfter)

	f.AssertOutputContains("hello world!")
	f.AssertLogStartTime(start)
}

func TestFailedToCreateLogWatcher(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName,
		"listening on 8080\nfailed to create fsnotify watcher: too many open files")

	start := f.clock.Now()

	pb := newPodBuilder(podID).addRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.toPod())

	pls := plsFromPod("server", pb, start)
	f.Create(pls)

	f.triggerPodEvent(podID)
	f.AssertOutputContains(`listening on 8080
failed to create fsnotify watcher: too many open files
Error streaming pod-id logs: failed to create fsnotify watcher: too many open files. Consider adjusting inotify limits: https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
`)
}

type plmStore struct {
	t testing.TB
	*store.TestingStore
	out *bufsync.ThreadSafeBuffer
}

func newPLMStore(t testing.TB, out *bufsync.ThreadSafeBuffer) *plmStore {
	return &plmStore{
		t:            t,
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *plmStore) Dispatch(action store.Action) {
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
	*fake.ControllerFixture
	t       testing.TB
	ctx     context.Context
	kClient *k8s.FakeK8sClient
	plsc    *Controller
	out     *bufsync.ThreadSafeBuffer
	store   *plmStore
	clock   *clockwork.FakeClock
}

func newPLMFixture(t testing.TB) *plmFixture {
	kClient := k8s.NewFakeK8sClient(t)

	out := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(out))

	cfb := fake.NewControllerFixtureBuilder(t)

	clock := clockwork.NewFakeClock()
	st := newPLMStore(t, out)
	podSource := NewPodSource(ctx, kClient, cfb.Client.Scheme(), clock)
	plsc := NewController(ctx, cfb.Client, cfb.Scheme(), st, kClient, podSource, clock)

	return &plmFixture{
		t:                 t,
		ControllerFixture: cfb.WithRequeuer(plsc.podSource).Build(plsc),
		kClient:           kClient,
		plsc:              plsc,
		ctx:               ctx,
		out:               out,
		store:             st,
		clock:             clock,
	}
}

func (f *plmFixture) triggerPodEvent(podID k8s.PodID) {
	podNN := types.NamespacedName{Name: string(podID), Namespace: "default"}
	reqs := f.plsc.podSource.indexer.EnqueueKey(indexer.Key{Name: podNN, GVK: podGVK})
	for _, req := range reqs {
		_, err := f.plsc.Reconcile(f.ctx, req)
		assert.NoError(f.t, err)
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

	f.t.Fatalf("Timeout. Collected output: %s", f.out.String())
}

func (f *plmFixture) AssertOutputContains(s string) {
	f.t.Helper()
	f.out.AssertEventuallyContains(f.t, s, time.Second)
}

func (f *plmFixture) AssertOutputDoesNotContain(s string) {
	time.Sleep(10 * time.Millisecond)
	assert.NotContains(f.t, f.out.String(), s)
}

func (f *plmFixture) AssertLogStartTime(t time.Time) {
	f.t.Helper()

	// Truncate the time to match the behavior of metav1.Time
	timecmp.AssertTimeEqual(f.t, t.Truncate(time.Second), f.kClient.LastPodLogStartTime)
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

func (pb *podBuilder) addDeletionTimestamp() *podBuilder {
	now := metav1.Now()
	pb.ObjectMeta.DeletionTimestamp = &now
	return pb
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

func plsFromPod(mn model.ManifestName, pb *podBuilder, start time.Time) *v1alpha1.PodLogStream {
	var sinceTime *metav1.Time
	if !start.IsZero() {
		t := apis.NewTime(start)
		sinceTime = &t
	}
	return &v1alpha1.PodLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", pb.Namespace, pb.Name),
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: string(mn),
				v1alpha1.AnnotationSpanID:   string(k8sconv.SpanIDForPod(mn, k8s.PodID(pb.Name))),
			},
		},
		Spec: PodLogStreamSpec{
			Namespace: pb.Namespace,
			Pod:       pb.Name,
			SinceTime: sinceTime,
		},
	}
}

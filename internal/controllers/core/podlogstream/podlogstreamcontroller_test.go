package podlogstream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
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

	start := time.Now()

	pb := fake.NewPodBuilder(podID).AddRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.ToPod())

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

func TestLogActions(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\ngoodbye world!\n")

	pb := fake.NewPodBuilder(podID).AddRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.ToPod())

	f.Create(plsFromPod("server", pb, time.Time{}))

	f.triggerPodEvent(podID)
	f.ConsumeLogActionsUntil("hello world!")
}

func TestLogsFailed(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.ContainerLogsError = fmt.Errorf("my-error")

	pb := fake.NewPodBuilder(podID).AddRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.ToPod())

	pls := plsFromPod("server", pb, time.Time{})
	f.Create(pls)

	f.AssertOutputContains("Error streaming pod-id logs")
	assert.Contains(t, f.out.String(), "my-error")

	// Check to make sure the status has an error.
	f.MustGet(f.KeyForObject(pls), pls)
	assert.Equal(t, pls.Status, PodLogStreamStatus{
		ContainerStatuses: []ContainerLogStreamStatus{
			{
				Name:  "cname",
				Error: "my-error",
			},
		},
	})
}

func TestLogsCanceledUnexpectedly(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, cName, "hello world!\n")

	pb := fake.NewPodBuilder(podID).AddRunningContainer(cName, cID)
	f.kClient.UpsertPod(pb.ToPod())
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
	f.AssertOutputContains("goodbye world!\n")
}

func TestMultiContainerLogs(t *testing.T) {
	f := newPLMFixture(t)

	f.kClient.SetLogsForPodContainer(podID, "cont1", "hello world!")
	f.kClient.SetLogsForPodContainer(podID, "cont2", "goodbye world!")

	pb := fake.NewPodBuilder(podID).
		AddRunningContainer("cont1", "cid1").
		AddRunningContainer("cont2", "cid2")
	f.kClient.UpsertPod(pb.ToPod())
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

	pbMultiC := fake.NewPodBuilder(pID1).
		// Pod with multiple containers -- logs should be prefixed with container name
		AddRunningContainer(cNamePrefix1, "cid1").
		AddRunningContainer(cNamePrefix2, "cid2")
	f.kClient.UpsertPod(pbMultiC.ToPod())

	f.Create(plsFromPod("multiContainer", pbMultiC, time.Time{}))

	pbSingleC := fake.NewPodBuilder(pID2).
		// Pod with just one container -- logs should NOT be prefixed with container name
		AddRunningContainer(cNameNoPrefix, "cid3")
	f.kClient.UpsertPod(pbSingleC.ToPod())

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
	pb := fake.NewPodBuilder(podID).AddTerminatedContainer(cName, "cID")
	f.kClient.UpsertPod(pb.ToPod())

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
	pb := fake.NewPodBuilder(podID).AddRunningContainer(cName, "cID")
	f.kClient.UpsertPod(pb.ToPod())

	reader, writer := io.Pipe()
	defer func() {
		require.NoError(t, writer.Close())
	}()
	f.kClient.SetLogReaderForPodContainer(podID, cName, reader)

	// Set up fake time
	startTime := time.Now()
	currentTime := startTime.Add(5 * time.Second)
	timeCh := make(chan time.Time)
	ticker := time.Ticker{C: timeCh}
	f.plsc.now = func() time.Time { return currentTime }
	f.plsc.since = func(t time.Time) time.Duration { return currentTime.Sub(t) }
	f.plsc.newTicker = func(d time.Duration) *time.Ticker { return &ticker }

	f.Create(plsFromPod("server", pb, startTime))

	_, err := writer.Write([]byte("hello world!"))
	require.NoError(t, err)
	f.AssertOutputContains("hello world!")
	f.AssertLogStartTime(startTime)

	currentTime = currentTime.Add(20 * time.Second)
	lastRead := currentTime
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
	pb := fake.NewPodBuilder(podID).
		AddTerminatedInitContainer(cNameInit, "cID-init").
		AddRunningContainer(cNameNormal, "cID-normal")
	f.kClient.UpsertPod(pb.ToPod())

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

	istioInit := runtimelog.IstioInitContainerName
	istioSidecar := runtimelog.IstioSidecarContainerName
	cNormal := container.Name("cNameNormal")
	pb := fake.NewPodBuilder(podID).
		AddTerminatedInitContainer(istioInit, "cID-init").
		AddRunningContainer(istioSidecar, "cID-sidecar").
		AddRunningContainer(cNormal, "cID-normal")
	f.kClient.UpsertPod(pb.ToPod())

	f.kClient.SetLogsForPodContainer(podID, istioInit, "init istio!")
	f.kClient.SetLogsForPodContainer(podID, istioSidecar, "hello istio!")
	f.kClient.SetLogsForPodContainer(podID, cNormal, "hello world!")

	pls := plsFromPod("server", pb, time.Time{})
	pls.Spec.IgnoreContainers = []string{string(istioInit), string(istioSidecar)}
	f.Create(pls)

	f.AssertOutputDoesNotContain("istio")
	f.AssertOutputContains("hello world!")
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
	plm     *runtimelog.PodLogManager
	plsc    *Controller
	out     *bufsync.ThreadSafeBuffer
	store   *plmStore
}

func newPLMFixture(t testing.TB) *plmFixture {
	kClient := k8s.NewFakeK8sClient(t)
	t.Cleanup(kClient.TearDown)

	out := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(out))

	cfb := fake.NewControllerFixtureBuilder(t)

	st := newPLMStore(t, out)
	plm := runtimelog.NewPodLogManager(cfb.Client)
	podSource := NewPodSource(ctx, kClient, cfb.Client.Scheme())
	plsc := NewController(ctx, cfb.Client, st, kClient, podSource)

	return &plmFixture{
		t:                 t,
		ControllerFixture: cfb.Build(plsc),
		kClient:           kClient,
		plm:               plm,
		plsc:              plsc,
		ctx:               ctx,
		out:               out,
		store:             st,
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
	err := f.out.WaitUntilContains(s, time.Second)
	if err != nil {
		f.t.Fatal(err)
	}
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

func plsFromPod(mn model.ManifestName, pb *fake.PodBuilder, start time.Time) *v1alpha1.PodLogStream {
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

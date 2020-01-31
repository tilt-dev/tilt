package k8s

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dfake "k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/windmilleng/tilt/internal/testutils"
)

func TestTiltPatch(t *testing.T) {
	assert.True(t, cache.IsTiltPatchApplied())
}

func TestK8sClient_WatchPods(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	pod1 := fakePod(PodID("abcd"), "efgh")
	pod2 := fakePod(PodID("1234"), "hieruyge")
	pod3 := fakePod(PodID("754"), "efgh")
	pods := []runtime.Object{pod1, pod2, pod3}
	tf.runPods(pods, pods)
}

func TestK8sClient_WatchPodDeletion(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	podID := PodID("pod1")
	pod := fakePod(podID, "image1")
	ch := tf.watchPods()
	tf.addObjects(pod)

	select {
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for pod update")
	case obj := <-ch:
		asPod, _ := obj.AsPod()
		assert.Equal(t, podID, PodIDFromPod(asPod))
	}

	err := tf.tracker.Delete(PodGVR, "default", "pod1")
	assert.NoError(t, err)

	select {
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for pod delete")
	case obj := <-ch:
		ns, name, _ := obj.AsDeletedKey()
		assert.Equal(t, "pod1", name)
		assert.Equal(t, Namespace("default"), ns)
	}
}

func TestK8sClient_WatchPodsFilterNonPods(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	pod := fakePod(PodID("abcd"), "efgh")
	pods := []runtime.Object{pod}

	deployment := &appsv1.Deployment{}
	input := []runtime.Object{deployment, pod}
	tf.runPods(input, pods)
}

func TestK8sClient_WatchPodsLabelsPassed(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	ls := labels.Set{"foo": "bar", "baz": "quu"}
	tf.testPodLabels(ls, ls)
}

func TestK8sClient_WatchServices(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	svc1 := fakeService("svc1")
	svc2 := fakeService("svc2")
	svc3 := fakeService("svc3")
	svcs := []runtime.Object{svc1, svc2, svc3}
	tf.runServices(svcs, svcs)
}

func TestK8sClient_WatchServicesFilterNonServices(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	svc := fakeService("svc1")
	svcs := []runtime.Object{svc}

	deployment := &appsv1.Deployment{}
	input := []runtime.Object{deployment, svc}
	tf.runServices(input, svcs)
}

func TestK8sClient_WatchServicesLabelsPassed(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	lps := labels.Set{"foo": "bar", "baz": "quu"}
	tf.testServiceLabels(lps, lps)
}

func TestK8sClient_WatchPodsError(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.watchErr = newForbiddenError()
	_, err := tf.kCli.WatchPods(tf.ctx, labels.Everything())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Forbidden")
	}
}

func TestK8sClient_WatchPodsWithNamespaceRestriction(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.nsRestriction = "sandbox"
	tf.kCli.configNamespace = "sandbox"

	pod1 := fakePod(PodID("pod1"), "image1")
	pod1.Namespace = "sandbox"

	input := []runtime.Object{pod1}
	expected := []runtime.Object{pod1}
	tf.runPods(input, expected)
}

func TestK8sClient_WatchPodsBlockedByNamespaceRestriction(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.nsRestriction = "sandbox"
	tf.kCli.configNamespace = ""

	_, err := tf.kCli.WatchPods(tf.ctx, labels.Everything())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Code: 403")
	}
}

func TestK8sClient_WatchServicesWithNamespaceRestriction(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.nsRestriction = "sandbox"
	tf.kCli.configNamespace = "sandbox"

	svc1 := fakeService("svc1")
	svc1.Namespace = "sandbox"

	input := []runtime.Object{svc1}
	expected := []runtime.Object{svc1}
	tf.runServices(input, expected)
}

func TestK8sClient_WatchServicesBlockedByNamespaceRestriction(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.nsRestriction = "sandbox"
	tf.kCli.configNamespace = ""

	_, err := tf.kCli.WatchServices(tf.ctx, labels.Everything())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Code: 403")
	}
}

func TestK8sClient_WatchEvents(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	event1 := fakeEvent("event1", "hello1", 1)
	event2 := fakeEvent("event2", "hello2", 2)
	event3 := fakeEvent("event3", "hello3", 3)
	events := []runtime.Object{event1, event2, event3}
	tf.runEvents(events, events)
}

func TestK8sClient_WatchEventsNamespaced(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.kCli.configNamespace = "default"

	event1 := fakeEvent("event1", "hello1", 1)
	event1.Namespace = "sandbox"

	events := []runtime.Object{event1}
	tf.runEvents(events, events)
}

func TestK8sClient_WatchEventsUpdate(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	event1 := fakeEvent("event1", "hello1", 1)
	event2 := fakeEvent("event2", "hello2", 1)
	event1b := fakeEvent("event1", "hello1", 1)
	event3 := fakeEvent("event3", "hello3", 1)
	event2b := fakeEvent("event2", "hello2", 2)

	ch := tf.watchEvents()

	gvr := schema.GroupVersionResource{Version: "v1", Resource: "events"}
	tf.addObjects(event1, event2)
	tf.assertEvents([]runtime.Object{event1, event2}, ch)

	err := tf.tracker.Update(gvr, event1b, "")
	require.NoError(t, err)
	tf.assertEvents([]runtime.Object{}, ch)

	err = tf.tracker.Add(event3)
	require.NoError(t, err)
	err = tf.tracker.Update(gvr, event2b, "")
	require.NoError(t, err)
	tf.assertEvents([]runtime.Object{event3, event2b}, ch)
}

func TestWatchPodsAfterAdding(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	pod1 := fakePod(PodID("abcd"), "efgh")
	tf.addObjects(pod1)
	ch := tf.watchPods()
	tf.assertPods([]runtime.Object{pod1}, ch)
}

func TestWatchServicesAfterAdding(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	svc := fakeService("svc1")
	tf.addObjects(svc)
	ch := tf.watchServices()
	tf.assertServices([]runtime.Object{svc}, ch)
}

func TestWatchEventsAfterAdding(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	event := fakeEvent("event1", "hello1", 1)
	tf.addObjects(event)
	ch := tf.watchEvents()
	tf.assertEvents([]runtime.Object{event}, ch)
}

type watchTestFixture struct {
	t    *testing.T
	kCli K8sClient

	tracker           ktesting.ObjectTracker
	watchRestrictions ktesting.WatchRestrictions
	ctx               context.Context
	watchErr          error
	nsRestriction     Namespace
	cancel            context.CancelFunc
}

func newWatchTestFixture(t *testing.T) *watchTestFixture {
	ret := &watchTestFixture{t: t}

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ret.ctx, ret.cancel = context.WithCancel(ctx)

	tracker := ktesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	ret.tracker = tracker

	wr := func(action ktesting.Action) (handled bool, wi watch.Interface, err error) {
		wa := action.(ktesting.WatchAction)
		nsRestriction := ret.nsRestriction
		if !nsRestriction.Empty() && wa.GetNamespace() != nsRestriction.String() {
			return true, nil, &apiErrors.StatusError{
				ErrStatus: metav1.Status{Code: http.StatusForbidden},
			}
		}

		ret.watchRestrictions = wa.GetWatchRestrictions()
		if ret.watchErr != nil {
			return true, nil, ret.watchErr
		}

		// Fake watcher implementation based on objects added to the tracker.
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := tracker.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}

		return true, watch, nil
	}

	cs := kfake.NewSimpleClientset()
	cs.PrependReactor("*", "*", ktesting.ObjectReaction(tracker))
	cs.PrependWatchReactor("*", wr)

	dcs := dfake.NewSimpleDynamicClient(scheme.Scheme)
	dcs.PrependReactor("*", "*", ktesting.ObjectReaction(tracker))
	dcs.PrependWatchReactor("*", wr)

	ret.kCli = K8sClient{
		env:       EnvUnknown,
		dynamic:   dcs,
		clientset: cs,
		core:      cs.CoreV1(),
	}

	return ret
}

func (tf *watchTestFixture) TearDown() {
	tf.cancel()
}

func (tf *watchTestFixture) watchPods() <-chan ObjectUpdate {
	ch, err := tf.kCli.WatchPods(tf.ctx, labels.Everything())
	if err != nil {
		tf.t.Fatalf("watchPods: %v", err)
	}
	return ch
}

func (tf *watchTestFixture) watchServices() <-chan *v1.Service {
	ch, err := tf.kCli.WatchServices(tf.ctx, labels.Everything())
	if err != nil {
		tf.t.Fatalf("watchServices: %v", err)
	}
	return ch
}

func (tf *watchTestFixture) watchEvents() <-chan *v1.Event {
	ch, err := tf.kCli.WatchEvents(tf.ctx)
	if err != nil {
		tf.t.Fatalf("watchEvents: %v", err)
	}
	return ch
}

func (tf *watchTestFixture) addObjects(inputs ...runtime.Object) {
	for _, o := range inputs {
		err := tf.tracker.Add(o)
		if err != nil {
			tf.t.Fatalf("addObjects: %v", err)
		}
	}
}

func (tf *watchTestFixture) runPods(input []runtime.Object, expected []runtime.Object) {
	ch := tf.watchPods()
	tf.addObjects(input...)
	tf.assertPods(expected, ch)
}

func (tf *watchTestFixture) assertPods(expectedOutput []runtime.Object, ch <-chan ObjectUpdate) {
	var observedPods []runtime.Object

	done := false
	for !done {
		select {
		case obj, ok := <-ch:
			if !ok {
				done = true
				continue
			}

			pod, ok := obj.AsPod()
			if ok {
				observedPods = append(observedPods, pod)
			}
		case <-time.After(10 * time.Millisecond):
			// if we haven't seen any events for 10ms, assume we're done
			done = true
		}
	}

	assert.Equal(tf.t, expectedOutput, observedPods)
}

func (tf *watchTestFixture) runServices(input []runtime.Object, expected []runtime.Object) {
	ch := tf.watchServices()
	tf.addObjects(input...)
	tf.assertServices(expected, ch)
}

func (tf *watchTestFixture) assertServices(expectedOutput []runtime.Object, ch <-chan *v1.Service) {
	var observedServices []runtime.Object

	done := false
	for !done {
		select {
		case pod, ok := <-ch:
			if !ok {
				done = true
			} else {
				observedServices = append(observedServices, pod)
			}
		case <-time.After(10 * time.Millisecond):
			// if we haven't seen any events for 10ms, assume we're done
			done = true
		}
	}

	assert.Equal(tf.t, expectedOutput, observedServices)
}

func (tf *watchTestFixture) runEvents(input []runtime.Object, expectedOutput []runtime.Object) {
	ch := tf.watchEvents()
	tf.addObjects(input...)
	tf.assertEvents(expectedOutput, ch)
}

func (tf *watchTestFixture) assertEvents(expectedOutput []runtime.Object, ch <-chan *v1.Event) {
	var observedEvents []runtime.Object

	done := false
	for !done {
		select {
		case event, ok := <-ch:
			if !ok {
				done = true
			} else {
				observedEvents = append(observedEvents, event)
			}
		case <-time.After(10 * time.Millisecond):
			// if we haven't seen any events for 10ms, assume we're done
			// ideally we'd always be exiting from ch closing, but it's not currently clear how to do that via informer
			done = true
		}
	}

	// TODO(matt) - using ElementsMatch instead of Equal because, for some reason, events do not always come out in the
	// same order we feed them in. I'm punting on figuring out why for now.
	assert.ElementsMatch(tf.t, expectedOutput, observedEvents)
}

func (tf *watchTestFixture) testPodLabels(input labels.Set, expectedLabels labels.Set) {
	_, err := tf.kCli.WatchPods(tf.ctx, input.AsSelector())
	if !assert.NoError(tf.t, err) {
		return
	}

	assert.Equal(tf.t, expectedLabels.String(), input.String())
}

func (tf *watchTestFixture) testServiceLabels(input labels.Set, expectedLabels labels.Set) {
	_, err := tf.kCli.WatchServices(tf.ctx, input.AsSelector())
	if !assert.NoError(tf.t, err) {
		return
	}

	assert.Equal(tf.t, fields.Everything(), tf.watchRestrictions.Fields)

	expectedLabelSelector := expectedLabels.AsSelector()
	assert.Equal(tf.t, expectedLabelSelector, tf.watchRestrictions.Labels)
}

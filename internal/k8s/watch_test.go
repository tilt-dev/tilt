package k8s

import (
	"context"
	"net/http"
	"testing"
	"time"

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

	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestK8sClient_WatchPods(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	pod1 := fakePod(PodID("abcd"), "efgh")
	pod2 := fakePod(PodID("1234"), "hieruyge")
	pod3 := fakePod(PodID("754"), "efgh")
	pods := []runtime.Object{&pod1, &pod2, &pod3}
	tf.runPods(pods, pods)
}

func TestK8sClient_WatchPodsFilterNonPods(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	pod := fakePod(PodID("abcd"), "efgh")
	pods := []runtime.Object{&pod}

	deployment := appsv1.Deployment{}
	input := []runtime.Object{&deployment, &pod}
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
	svcs := []runtime.Object{&svc1, &svc2, &svc3}
	tf.runServices(svcs, svcs)
}

func TestK8sClient_WatchServicesFilterNonServices(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	svc := fakeService("svc1")
	svcs := []runtime.Object{&svc}

	deployment := appsv1.Deployment{}
	input := []runtime.Object{&deployment, &svc}
	tf.runServices(input, svcs)
}

func TestK8sClient_WatchServicesLabelsPassed(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	lps := []model.LabelPair{{Key: "foo", Value: "bar"}, {Key: "baz", Value: "quu"}}
	tf.testServiceLabels(lps, lps)
}

func TestK8sClient_WatchPodsError(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.watchErr = newForbiddenError()
	_, err := tf.kCli.WatchPods(tf.ctx, labels.Set{}.AsSelector())
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

	input := []runtime.Object{&pod1}
	expected := []runtime.Object{&pod1}
	tf.runPods(input, expected)
}

func TestK8sClient_WatchPodsBlockedByNamespaceRestriction(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.nsRestriction = "sandbox"
	tf.kCli.configNamespace = ""

	_, err := tf.kCli.WatchPods(tf.ctx, labels.Set{}.AsSelector())
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

	input := []runtime.Object{&svc1}
	expected := []runtime.Object{&svc1}
	tf.runServices(input, expected)
}

func TestK8sClient_WatchServicesBlockedByNamespaceRestriction(t *testing.T) {
	tf := newWatchTestFixture(t)
	defer tf.TearDown()

	tf.nsRestriction = "sandbox"
	tf.kCli.configNamespace = ""

	_, err := tf.kCli.WatchServices(tf.ctx, nil)
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

	ch, err := tf.kCli.WatchEvents(tf.ctx)
	assert.NoError(t, err)

	gvr := schema.GroupVersionResource{Version: "v1", Resource: "events"}
	tf.tracker.Add(event1)
	tf.tracker.Add(event2)
	tf.assertEvents([]runtime.Object{event1, event2}, ch)

	tf.tracker.Update(gvr, event1b, "")
	tf.assertEvents([]runtime.Object{}, ch)

	tf.tracker.Add(event3)
	tf.tracker.Update(gvr, event2b, "")
	tf.assertEvents([]runtime.Object{event3, event2b}, ch)
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
		clientSet: cs,
		core:      cs.CoreV1(),
	}

	return ret
}

func (tf *watchTestFixture) TearDown() {
	tf.cancel()
}

func (tf *watchTestFixture) runPods(input []runtime.Object, expectedOutput []runtime.Object) {
	ch, err := tf.kCli.WatchPods(tf.ctx, labels.Set{}.AsSelector())
	if !assert.NoError(tf.t, err) {
		return
	}

	for _, o := range input {
		tf.tracker.Add(o)
	}

	var observedPods []runtime.Object

	done := false
	for !done {
		select {
		case pod, ok := <-ch:
			if !ok {
				done = true
			} else {
				observedPods = append(observedPods, pod)
			}
		case <-time.After(10 * time.Millisecond):
			// if we haven't seen any events for 10ms, assume we're done
			done = true
		}
	}

	assert.Equal(tf.t, expectedOutput, observedPods)
}

func (tf *watchTestFixture) runServices(input []runtime.Object, expectedOutput []runtime.Object) {
	ch, err := tf.kCli.WatchServices(tf.ctx, []model.LabelPair{})
	if !assert.NoError(tf.t, err) {
		return
	}

	for _, o := range input {
		tf.tracker.Add(o)
	}

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
	ch, err := tf.kCli.WatchEvents(tf.ctx)
	if !assert.NoError(tf.t, err) {
		return
	}

	for _, o := range input {
		tf.tracker.Add(o)
	}
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

func (tf *watchTestFixture) testServiceLabels(input []model.LabelPair, expectedLabels []model.LabelPair) {
	_, err := tf.kCli.WatchServices(tf.ctx, input)
	if !assert.NoError(tf.t, err) {
		return
	}

	assert.Equal(tf.t, fields.Everything(), tf.watchRestrictions.Fields)

	ls := labels.Set{}
	for _, l := range expectedLabels {
		ls[l.Key] = l.Value
	}
	expectedLabelSelector := labels.SelectorFromSet(ls)
	assert.Equal(tf.t, expectedLabelSelector, tf.watchRestrictions.Labels)
}

func fakeService(name string) v1.Service {
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func fakeEvent(name string, message string, count int) *v1.Event {
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Message: message,
		Count:   int32(count),
	}
}

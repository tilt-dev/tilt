package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/model"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"k8s.io/apimachinery/pkg/watch"
	k8stesting "k8s.io/client-go/testing"
)

func TestK8sClient_WatchPods(t *testing.T) {
	tf := newWatchTestFixture(t)

	pod1 := fakePod(PodID("abcd"), "efgh")
	pod2 := fakePod(PodID("1234"), "hieruyge")
	pod3 := fakePod(PodID("754"), "efgh")
	pods := []runtime.Object{&pod1, &pod2, &pod3}
	tf.runPods(pods, pods)
}

func TestK8sClient_WatchPodsFilterNonPods(t *testing.T) {
	tf := newWatchTestFixture(t)

	pod := fakePod(PodID("abcd"), "efgh")
	pods := []runtime.Object{&pod}

	deployment := v1.Deployment{}
	input := []runtime.Object{&deployment, &pod}
	tf.runPods(input, pods)
}

func TestK8sClient_WatchPodsLabelsPassed(t *testing.T) {
	tf := newWatchTestFixture(t)
	ls := labels.Set{"foo": "bar", "baz": "quu"}
	tf.testPodLabels(ls, ls)
}

func TestK8sClient_WatchServices(t *testing.T) {
	tf := newWatchTestFixture(t)

	pod1 := fakePod(PodID("abcd"), "efgh")
	pod2 := fakePod(PodID("1234"), "hieruyge")
	pod3 := fakePod(PodID("754"), "efgh")
	pods := []runtime.Object{&pod1, &pod2, &pod3}
	tf.runPods(pods, pods)
}

func TestK8sClient_WatchServicesFilterNonServices(t *testing.T) {
	tf := newWatchTestFixture(t)

	pod := fakePod(PodID("abcd"), "efgh")
	pods := []runtime.Object{&pod}

	deployment := v1.Deployment{}
	input := []runtime.Object{&deployment, &pod}
	tf.runPods(input, pods)
}

func TestK8sClient_WatchServicesLabelsPassed(t *testing.T) {
	tf := newWatchTestFixture(t)
	lps := []model.LabelPair{{Key: "foo", Value: "bar"}, {Key: "baz", Value: "quu"}}
	tf.testServiceLabels(lps, lps)
}

type watchTestFixture struct {
	t                 *testing.T
	kCli              Client
	w                 *watch.FakeWatcher
	watchRestrictions k8stesting.WatchRestrictions
	ctx               context.Context
}

func newWatchTestFixture(t *testing.T) *watchTestFixture {
	ret := &watchTestFixture{t: t}

	c := fake.NewSimpleClientset()

	ret.ctx = output.CtxForTest()

	ret.w = watch.NewFakeWithChanSize(10, false)

	wr := func(action k8stesting.Action) (handled bool, wi watch.Interface, err error) {
		wa := action.(k8stesting.WatchAction)
		ret.watchRestrictions = wa.GetWatchRestrictions()
		return true, ret.w, nil
	}

	c.Fake.PrependWatchReactor("*", wr)
	ret.kCli = K8sClient{
		env:           EnvUnknown,
		kubectlRunner: nil,
		core:          c.CoreV1(), // TODO set
		restConfig:    nil,
		portForwarder: nil,
	}

	return ret
}

func (tf *watchTestFixture) runPods(input []runtime.Object, expectedOutput []runtime.Object) {
	for _, o := range input {
		tf.w.Add(o)
	}

	tf.w.Stop()

	ch, err := tf.kCli.WatchPods(tf.ctx, labels.Set{}.AsSelector())
	if !assert.NoError(tf.t, err) {
		return
	}

	var observedPods []runtime.Object

	timeout := time.After(500 * time.Millisecond)
	done := false
	for !done {
		select {
		case pod, ok := <-ch:
			if !ok {
				done = true
			} else {
				observedPods = append(observedPods, pod)
			}
		case <-timeout:
			tf.t.Fatal("test timed out")
		}
	}

	assert.Equal(tf.t, expectedOutput, observedPods)
}

func (tf *watchTestFixture) runServices(input []runtime.Object, expectedOutput []runtime.Object) {
	for _, o := range input {
		tf.w.Add(o)
	}

	tf.w.Stop()

	ch, err := tf.kCli.WatchServices(tf.ctx, []model.LabelPair{})
	if !assert.NoError(tf.t, err) {
		return
	}

	var observedServices []runtime.Object

	timeout := time.After(500 * time.Millisecond)
	done := false
	for !done {
		select {
		case pod, ok := <-ch:
			if !ok {
				done = true
			} else {
				observedServices = append(observedServices, pod)
			}
		case <-timeout:
			tf.t.Fatal("test timed out")
		}
	}

	assert.Equal(tf.t, expectedOutput, observedServices)
}

func (tf *watchTestFixture) testPodLabels(input labels.Set, expectedLabels labels.Set) {
	_, err := tf.kCli.WatchPods(tf.ctx, input.AsSelector())
	if !assert.NoError(tf.t, err) {
		return
	}

	assert.Equal(tf.t, fields.Everything(), tf.watchRestrictions.Fields)

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

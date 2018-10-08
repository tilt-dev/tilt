package k8s

import (
	"context"
	"testing"
	"time"

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
	tf := newWatchPodsTestFixture(t)

	pod1 := fakePod(PodID("abcd"), "efgh")
	pod2 := fakePod(PodID("1234"), "hieruyge")
	pod3 := fakePod(PodID("754"), "efgh")
	pods := []runtime.Object{&pod1, &pod2, &pod3}
	tf.run(pods, pods)
}

func TestK8sClient_WatchPodsFilterNonPods(t *testing.T) {
	tf := newWatchPodsTestFixture(t)

	pod := fakePod(PodID("abcd"), "efgh")
	pods := []runtime.Object{&pod}

	deployment := v1.Deployment{}
	input := []runtime.Object{&deployment, &pod}
	tf.run(input, pods)
}

func TestK8sClient_WatchPodsLabelsPassed(t *testing.T) {
	tf := newWatchPodsTestFixture(t)
	lps := []LabelPair{{"foo", "bar"}, {"baz", "quu"}}
	tf.testLabels(lps, lps)
}

type watchPodsTestFixture struct {
	t                 *testing.T
	kCli              Client
	w                 *watch.FakeWatcher
	watchRestrictions k8stesting.WatchRestrictions
	ctx               context.Context
}

func newWatchPodsTestFixture(t *testing.T) *watchPodsTestFixture {
	ret := &watchPodsTestFixture{t: t}

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

func (tf *watchPodsTestFixture) run(input []runtime.Object, expectedOutput []runtime.Object) {
	for _, o := range input {
		tf.w.Add(o)
	}

	ch, err := tf.kCli.WatchPods(tf.ctx, Namespace("default"), []LabelPair{})
	if !assert.NoError(tf.t, err) {
		return
	}

	var observedPods []runtime.Object

	timeout := time.After(time.Millisecond * 500)
	done := false
	for !done {
		select {
		case observedPod := <-ch:
			observedPods = append(observedPods, observedPod)
		case <-timeout:
			done = true
		}
	}

	assert.Equal(tf.t, expectedOutput, observedPods)
}

func (tf *watchPodsTestFixture) testLabels(input []LabelPair, expectedLabels []LabelPair) {
	_, err := tf.kCli.WatchPods(tf.ctx, Namespace("default"), input)
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

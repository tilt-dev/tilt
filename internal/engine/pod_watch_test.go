package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils/output"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestPodWatch(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	f.addManifestWithSelectors("server")

	f.pw.OnChange(f.ctx, f.store)

	ls := k8s.TiltRunSelector()
	p := podNamed("hello")
	f.kClient.EmitPod(ls, p)

	f.assertWatchedSelectors(ls)

	f.assertObservedPods(p)
}

func TestPodWatchExtraSelectors(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	ls := k8s.TiltRunSelector()
	ls1 := labels.Set{"foo": "bar"}.AsSelector()
	ls2 := labels.Set{"baz": "quu"}.AsSelector()
	f.addManifestWithSelectors("server", ls1, ls2)

	f.pw.OnChange(f.ctx, f.store)

	f.assertWatchedSelectors(ls, ls1, ls2)

	p := podNamed("pod1")
	f.kClient.EmitPod(ls1, p)

	f.assertObservedPods(p)
}

func TestPodWatchHandleSelectorChange(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	ls := k8s.TiltRunSelector()
	ls1 := labels.Set{"foo": "bar"}.AsSelector()
	f.addManifestWithSelectors("server1", ls1)

	f.pw.OnChange(f.ctx, f.store)

	f.assertWatchedSelectors(ls, ls1)

	p := podNamed("pod1")
	f.kClient.EmitPod(ls1, p)

	f.assertObservedPods(p)
	f.clearPods()

	ls2 := labels.Set{"baz": "quu"}.AsSelector()
	f.addManifestWithSelectors("server2", ls2)
	f.removeManifest("server1")

	f.pw.OnChange(f.ctx, f.store)

	f.assertWatchedSelectors(ls, ls2)

	p2 := podNamed("pod2")
	f.kClient.EmitPod(ls, p2)
	f.assertObservedPods(p2)
	f.clearPods()

	f.kClient.EmitPod(ls1, podNamed("pod3"))
	p4 := podNamed("pod4")
	f.kClient.EmitPod(ls2, p4)
	p5 := podNamed("pod5")
	f.kClient.EmitPod(ls, p5)

	f.assertObservedPods(p4, p5)

}

func podNamed(name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func (f *pwFixture) addManifestWithSelectors(manifestName string, ls ...labels.Selector) {
	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	mt, err := newManifestTargetWithSelectors(model.Manifest{Name: model.ManifestName(manifestName)}, ls)
	if err != nil {
		f.t.Fatalf("error creating manifest target with selectors: %+v", err)
	}
	state.UpsertManifestTarget(mt)
	f.store.UnlockMutableState()
}

func (f *pwFixture) removeManifest(manifestName string) {
	mn := model.ManifestName(manifestName)
	state := f.store.LockMutableStateForTesting()
	delete(state.ManifestTargets, model.ManifestName(mn))
	var newDefOrder []model.ManifestName
	for _, e := range state.ManifestDefinitionOrder {
		if mn != e {
			newDefOrder = append(newDefOrder, e)
		}
	}
	state.ManifestDefinitionOrder = newDefOrder
	f.store.UnlockMutableState()
}

type pwFixture struct {
	t       *testing.T
	kClient *k8s.FakeK8sClient
	pw      *PodWatcher
	ctx     context.Context
	cancel  func()
	store   *store.Store
	pods    []*corev1.Pod
}

func (pw *pwFixture) reducer(ctx context.Context, state *store.EngineState, action store.Action) {
	a, ok := action.(PodChangeAction)
	if !ok {
		pw.t.Errorf("Expected action type PodLogAction. Actual: %T", action)
	}
	pw.pods = append(pw.pods, a.Pod)
}

func newPWFixture(t *testing.T) *pwFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx := output.CtxForTest()
	ctx, cancel := context.WithCancel(ctx)

	ret := &pwFixture{
		kClient: kClient,
		pw:      NewPodWatcher(kClient),
		ctx:     ctx,
		cancel:  cancel,
		t:       t,
	}

	st := store.NewStore(store.Reducer(ret.reducer), store.LogActionsFlag(false))
	go st.Loop(ctx)

	ret.store = st

	return ret
}

func (f *pwFixture) TearDown() {
	f.cancel()
}

func newManifestTargetWithSelectors(m model.Manifest, selectors []labels.Selector) (*store.ManifestTarget, error) {
	dt, err := k8s.NewTarget(model.TargetName(m.Name), nil, nil, selectors, nil)
	if err != nil {
		return nil, err
	}
	m = m.WithDeployTarget(dt)
	return store.NewManifestTarget(m), nil
}

func (f *pwFixture) assertObservedPods(pods ...*corev1.Pod) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		if len(pods) == len(f.pods) {
			break
		}
	}

	if !assert.ElementsMatch(f.t, pods, f.pods) {
		f.t.FailNow()
	}
}

func (f *pwFixture) assertWatchedSelectors(ls ...labels.Selector) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		if len(ls) == len(f.kClient.WatchedSelectors()) {
			break
		}
	}

	if !assert.ElementsMatch(f.t, ls, f.kClient.WatchedSelectors()) {
		f.t.FailNow()
	}
}

func (f *pwFixture) clearPods() {
	f.pods = nil
}

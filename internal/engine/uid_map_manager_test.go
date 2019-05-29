package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestUIDMapManager(t *testing.T) {
	f := newUMMFixture(t)
	defer f.TearDown()

	es, err := k8s.ParseYAMLFromString(testyaml.DoggosDeploymentYaml)
	if err != nil {
		t.Fatalf("error parsing doggos yaml: %v", err)
	}

	e := es[0]
	e, err = k8s.InjectLabels(e, []model.LabelPair{{
		Key:   k8s.ManifestNameLabel,
		Value: e.Name(),
	}})
	if err != nil {
		t.Fatalf("error injecting manifest label: %v", err)
	}

	k8s.SetUIDForTest(t, &e, "foobar")

	f.addManifest(e.Name())

	f.umm.OnChange(f.ctx, f.store)

	ls := k8s.TiltRunSelector()

	f.kClient.EmitEverything(ls, watch.Event{
		Type:   watch.Added,
		Object: e.Obj,
	})

	expectedAction := UIDUpdateAction{
		UID:          k8s.UID("foobar"),
		EventType:    watch.Added,
		ManifestName: "doggos",
		Entity:       k8s.K8sEntity{Obj: e.Obj, Kind: e.Kind},
	}

	f.assertObservedUIDUpdateActions(expectedAction)
}

func (f *ummFixture) addManifest(manifestName string) {
	state := f.store.LockMutableStateForTesting()
	state.WatchFiles = true
	dt := model.K8sTarget{Name: model.TargetName(manifestName)}
	m := model.Manifest{Name: model.ManifestName(manifestName)}.WithDeployTarget(dt)
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.store.UnlockMutableState()
}

type ummFixture struct {
	t          *testing.T
	kClient    *k8s.FakeK8sClient
	umm        *UIDMapManager
	ctx        context.Context
	cancel     func()
	store      *store.Store
	getActions func() []store.Action

	// old value of k8sEventsFeatureFlag env var, for teardown
	// TODO(maia): remove this when we remove the feature flag
	oldFeatureFlagVal string
}

func newUMMFixture(t *testing.T) *ummFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx := output.CtxForTest()
	ctx, cancel := context.WithCancel(ctx)

	ret := &ummFixture{
		kClient:           kClient,
		umm:               NewUIDMapManager(kClient),
		ctx:               ctx,
		cancel:            cancel,
		t:                 t,
		oldFeatureFlagVal: os.Getenv(k8sEventsFeatureFlag),
	}

	os.Setenv(k8sEventsFeatureFlag, "true")

	ret.store, ret.getActions = store.NewStoreForTesting()
	go ret.store.Loop(ctx)

	return ret
}

func (f *ummFixture) TearDown() {
	_ = os.Setenv(k8sEventsFeatureFlag, f.oldFeatureFlagVal)
	f.cancel()
}

func (f *ummFixture) assertObservedUIDUpdateActions(expectedActions ...UIDUpdateAction) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		actions := f.getActions()
		if len(actions) == len(expectedActions) {
			break
		}
	}

	var observedActions []UIDUpdateAction
	for _, a := range f.getActions() {
		sca, ok := a.(UIDUpdateAction)
		if !ok {
			f.t.Fatalf("got non-%T: %v", UIDUpdateAction{}, a)
		}
		observedActions = append(observedActions, sca)
	}
	if !assert.Equal(f.t, expectedActions, observedActions) {
		f.t.FailNow()
	}
}

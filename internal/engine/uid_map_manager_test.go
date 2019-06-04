package engine

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestUIDMapManager_needsWatchNoK8s(t *testing.T) {
	f := newUMMFixture(t)
	defer f.TearDown()

	e := entityWithUID(t, testyaml.DoggosDeploymentYaml, "foobar")

	// No k8s manifests on the state, so OnChange shouldn't do anything --
	// when we emit an event, we do NOT expect to see an action dispatched,
	// since no watch should have been started.
	f.umm.OnChange(f.ctx, f.store)

	ls := k8s.TiltRunSelector()
	f.kClient.EmitEverything(ls, watch.Event{
		Type:   watch.Added,
		Object: e.Obj,
	})

	f.assertNoActions()
}

func TestUIDMapManager_dispatchesAction(t *testing.T) {
	UID := "foobar"
	eWithManifestLabels := entityWithUID(t, testyaml.DoggosDeploymentYaml, UID)
	eNoManifestLabels := entityWithUIDAndMaybeManifestLabel(t, testyaml.DoggosDeploymentYaml, UID, false)

	for _, test := range []struct {
		name           string
		watchType      watch.EventType
		entity         k8s.K8sEntity
		eventHasObject bool
		expectAction   bool
	}{
		{
			name:           "type = Added",
			watchType:      watch.Added,
			entity:         eWithManifestLabels,
			eventHasObject: true,
			expectAction:   true,
		},
		{
			name:           "no action for type = Modified",
			watchType:      watch.Modified,
			entity:         eWithManifestLabels,
			eventHasObject: true,
			expectAction:   false,
		},
		{
			name:           "no action for nil object",
			watchType:      watch.Added,
			entity:         eWithManifestLabels,
			eventHasObject: false,
			expectAction:   false,
		},
		{
			name:           "no action for entity without tilt manifest label",
			watchType:      watch.Added,
			entity:         eNoManifestLabels,
			eventHasObject: false,
			expectAction:   false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUMMFixture(t)
			defer f.TearDown()

			f.addManifest(test.entity.Name())

			f.umm.OnChange(f.ctx, f.store)

			ls := k8s.TiltRunSelector()

			evt := &watch.Event{Type: test.watchType}
			if test.eventHasObject {
				evt.Object = test.entity.Obj
			}
			f.kClient.EmitEverything(ls, *evt)

			if test.expectAction {
				expectedAction := UIDUpdateAction{
					UID:          k8s.UID(UID),
					EventType:    test.watchType,
					ManifestName: model.ManifestName(test.entity.Name()),
					Entity:       k8s.K8sEntity{Obj: eWithManifestLabels.Obj, Kind: eWithManifestLabels.Kind},
				}

				f.assertActions(expectedAction)
			} else {
				f.assertNoActions()
			}
		})
	}
}

func TestUIDMapManager_watchError(t *testing.T) {
	f := newUMMFixture(t)
	defer f.TearDown()

	err := fmt.Errorf("oh noes")
	f.kClient.EverythingWatchErr = err
	f.addManifest("someK8sEntity")

	f.umm.OnChange(f.ctx, f.store)

	expectedErr := errors.Wrap(err, "Error watching for uids\n")
	expected := store.ErrorAction{Error: expectedErr}
	f.assertActions(expected)
}

func entityWithUID(t *testing.T, yaml string, uid string) k8s.K8sEntity {
	return entityWithUIDAndMaybeManifestLabel(t, yaml, uid, true)
}

func entityWithUIDAndMaybeManifestLabel(t *testing.T, yaml string, uid string, withManifestLabel bool) k8s.K8sEntity {
	es, err := k8s.ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatalf("error parsing yaml: %v", err)
	}

	if len(es) != 1 {
		t.Fatalf("expected exactly 1 k8s entity from yaml, got %d", len(es))
	}

	e := es[0]
	if withManifestLabel {
		e, err = k8s.InjectLabels(e, []model.LabelPair{{
			Key:   k8s.ManifestNameLabel,
			Value: e.Name(),
		}})
		if err != nil {
			t.Fatalf("error injecting manifest label: %v", err)
		}
	}

	k8s.SetUIDForTest(t, &e, uid)

	return e
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

func (f *ummFixture) assertNoActions() {
	f.assertActions()
}

func (f *ummFixture) assertActions(expected ...store.Action) {
	if len(expected) == 0 {
		// assert no actions -- sleep briefly
		// to give any actions a chance to get into the queue
		time.Sleep(10 * time.Millisecond)
	}

	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		actions := f.getActions()
		if len(actions) >= len(expected) {
			break
		}
	}

	// NOTE(maia): this test will break if this the code ever returns other
	// correct-but-incidental-to-this-test actions, but for now it's fine.
	actual := f.getActions()
	if !assert.Len(f.t, actual, len(expected)) {
		f.t.FailNow()
	}

	for i, a := range actual {
		switch exp := expected[i].(type) {
		case store.ErrorAction:
			// Special case -- we can't just assert.Equal b/c pointer equality stuff
			act, ok := a.(store.ErrorAction)
			if !ok {
				f.t.Fatalf("got non-%T: %v", store.ErrorAction{}, a)
			}
			assert.Equal(f.t, exp.Error.Error(), act.Error.Error())
		default:
			assert.Equal(f.t, expected[i], a)
		}
	}
}

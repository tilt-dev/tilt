package uiresource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r := f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "(Tiltfile)", r.ObjectMeta.Name)
	assert.Equal(t, "1", r.ObjectMeta.ResourceVersion)

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	r = f.resource("(Tiltfile)")
	assert.Equal(t, "1", r.ObjectMeta.ResourceVersion)
}

func TestUpdateTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	r := f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "(Tiltfile)", r.ObjectMeta.Name)
	assert.Equal(t, "1", r.ObjectMeta.ResourceVersion)

	f.store.WithState(func(es *store.EngineState) {
		es.TiltfileState.CurrentBuild.StartTime = time.Now()
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r = f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "2", r.ObjectMeta.ResourceVersion)
}

func TestDeleteManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SanchoYAML).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, "(Tiltfile)", f.resource("(Tiltfile)").ObjectMeta.Name)
	assert.Equal(t, "fe", f.resource("fe").ObjectMeta.Name)

	f.store.WithState(func(state *store.EngineState) {
		state.RemoveManifestTarget("fe")
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Nil(t, f.resource("fe"))
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	store *store.TestingStore
	tc    ctrlclient.Client
	sub   *Subscriber
}

func newFixture(t *testing.T) *fixture {
	tc := fake.NewTiltClient()
	return &fixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		ctx:            context.Background(),
		tc:             tc,
		sub:            NewSubscriber(tc),
		store:          store.NewTestingStore(),
	}
}

func (f *fixture) resource(name string) *v1alpha1.UIResource {
	r := &v1alpha1.UIResource{}
	err := f.tc.Get(f.ctx, types.NamespacedName{Name: name}, r)
	if apierrors.IsNotFound(err) {
		return nil
	}

	require.NoError(f.T(), err)
	return r
}

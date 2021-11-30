package uiresource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestUpdateTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	r := &v1alpha1.UIResource{ObjectMeta: metav1.ObjectMeta{Name: "(Tiltfile)"}}
	err := f.tc.Create(f.ctx, r)
	require.NoError(t, err)

	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	r = f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "(Tiltfile)", r.ObjectMeta.Name)
	assert.Equal(t, "2", r.ObjectMeta.ResourceVersion)

	f.store.WithState(func(es *store.EngineState) {
		es.TiltfileStates[model.MainTiltfileManifestName].CurrentBuild.StartTime = time.Now()
	})

	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r = f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "3", r.ObjectMeta.ResourceVersion)
	assert.Equal(t, "False", string(readyCondition(r).Status))

	// Make sure OnChange is idempotent.
	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r = f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "3", r.ObjectMeta.ResourceVersion)

	f.store.WithState(func(es *store.EngineState) {
		ms := es.TiltfileStates[model.MainTiltfileManifestName]
		b := ms.CurrentBuild
		b.FinishTime = time.Now()
		ms.AddCompletedBuild(b)
		ms.CurrentBuild = model.BuildRecord{}
	})

	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	r = f.resource("(Tiltfile)")
	require.NotNil(t, r)
	assert.Equal(t, "4", r.ObjectMeta.ResourceVersion)
	assert.Equal(t, "True", string(readyCondition(r).Status))
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	store *store.TestingStore
	tc    ctrlclient.Client
	sub   *Subscriber
}

func newFixture(t *testing.T) *fixture {
	tc := fake.NewFakeTiltClient()
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

func readyCondition(r *v1alpha1.UIResource) *v1alpha1.UIResourceCondition {
	for _, c := range r.Status.Conditions {
		if c.Type == v1alpha1.UIResourceReady {
			return &c
		}
	}
	return nil
}

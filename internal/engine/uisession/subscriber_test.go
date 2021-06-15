package uisession

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreate(t *testing.T) {
	f := newFixture(t)

	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r := f.session("Tiltfile")
	require.NotNil(t, r)
	assert.Equal(t, "Tiltfile", r.ObjectMeta.Name)
	assert.Equal(t, "1", r.ObjectMeta.ResourceVersion)

	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	r = f.session("Tiltfile")
	assert.Equal(t, "1", r.ObjectMeta.ResourceVersion)
}

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r := f.session("Tiltfile")
	require.NotNil(t, r)
	assert.Equal(t, "Tiltfile", r.ObjectMeta.Name)
	assert.Equal(t, "1", r.ObjectMeta.ResourceVersion)

	f.store.WithState(func(es *store.EngineState) {
		es.CloudStatus.Username = "sparkle"
	})

	_ = f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	r = f.session("Tiltfile")
	require.NotNil(t, r)
	assert.Equal(t, "2", r.ObjectMeta.ResourceVersion)
	assert.Equal(t, "sparkle", r.Status.TiltCloudUsername)
}

type fixture struct {
	ctx   context.Context
	t     *testing.T
	store *store.TestingStore
	sub   *Subscriber
	tc    ctrlclient.Client
}

func newFixture(t *testing.T) *fixture {
	tc := fake.NewTiltClient()
	return &fixture{
		t:     t,
		ctx:   context.Background(),
		sub:   NewSubscriber(tc),
		tc:    tc,
		store: store.NewTestingStore(),
	}
}

func (f *fixture) session(name string) *v1alpha1.UISession {
	r := &v1alpha1.UISession{}
	err := f.tc.Get(f.ctx, types.NamespacedName{Name: name}, r)
	if apierrors.IsNotFound(err) {
		return nil
	}

	require.NoError(f.t, err)
	return r
}

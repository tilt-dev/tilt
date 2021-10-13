package uiresource

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var uirName = types.NamespacedName{Name: "test-uiresource"}

func TestReconcileDisableStatus(t *testing.T) {
	f := newFixture(t)

	var disableSources []v1alpha1.DisableSource
	for i := 0; i < 9; i++ {
		val := "false"
		if i%3 == 0 {
			val = "true"
		}
		cm := &v1alpha1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("cm-%d", i)},
			Data: map[string]string{
				"disabled": val,
			},
		}

		f.Create(cm)

		disableSources = append(disableSources, v1alpha1.DisableSource{
			ConfigMap: &v1alpha1.ConfigMapDisableSource{
				Name: cm.Name,
				Key:  "disabled",
			},
		})
	}

	uir := &v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{Name: uirName.Name},
	}
	f.Create(uir)

	uir.Status.DisableStatus.Sources = disableSources
	err := f.Client.Status().Update(f.ctx, uir)
	require.NoError(t, err)

	f.MustReconcile(uirName)

	uir = f.uiResource()
	require.Equal(t, 3, int(uir.Status.DisableStatus.DisabledCount))
	require.Equal(t, 6, int(uir.Status.DisableStatus.EnabledCount))
}

type fixture struct {
	*fake.ControllerFixture
	t   *testing.T
	st  *store.TestingStore
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	st := store.NewTestingStore()
	wsl := server.NewWebsocketList()

	r := NewReconciler(cfb.Client, wsl, st)
	return &fixture{
		ControllerFixture: cfb.Build(r),
		t:                 t,
		ctx:               context.Background(),
		st:                st,
	}
}

func (f *fixture) uiResource() *v1alpha1.UIResource {
	result := &v1alpha1.UIResource{}
	f.Get(uirName, result)
	return result
}

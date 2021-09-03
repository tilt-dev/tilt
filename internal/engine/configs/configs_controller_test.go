package configs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestCreateTiltfile(t *testing.T) {
	st := store.NewTestingStore()
	st.WithState(func(s *store.EngineState) {
		s.DesiredTiltfilePath = "./fake-tiltfile-path"
		s.UserConfigState = model.NewUserConfigState([]string{"arg1", "arg2"})
	})
	ctx := context.Background()
	client := fake.NewFakeTiltClient()
	cc := NewConfigsController(client)
	require.NoError(t, cc.OnChange(ctx, st, store.ChangeSummary{}))

	var tf v1alpha1.Tiltfile
	require.NoError(t, client.Get(ctx, types.NamespacedName{Name: model.MainTiltfileManifestName.String()}, &tf))
	assert.Equal(t, tf.Spec, v1alpha1.TiltfileSpec{
		Path: "./fake-tiltfile-path",
		Args: []string{"arg1", "arg2"},
		RestartOn: &v1alpha1.RestartOnSpec{
			FileWatches: []string{"configs:(Tiltfile)"},
		},
	})
}

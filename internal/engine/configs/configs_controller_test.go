package configs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/controllers/apis/uibutton"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/feature"
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
		Path: tiltfile.ResolveFilename("fake-tiltfile-path"),
		Args: []string{"arg1", "arg2"},
		RestartOn: &v1alpha1.RestartOnSpec{
			FileWatches: []string{"configs:(Tiltfile)"},
		},
		StopOn: &v1alpha1.StopOnSpec{
			UIButtons: []string{"(Tiltfile)-cancel"},
		},
	})
}

// TODO(matt) - replace TestCreateTiltfile above with this when removing feature.CancelBuild
func TestCreateTiltfileCancelEnabled(t *testing.T) {
	st := store.NewTestingStore()
	st.WithState(func(s *store.EngineState) {
		s.DesiredTiltfilePath = "./fake-tiltfile-path"
		s.UserConfigState = model.NewUserConfigState([]string{"arg1", "arg2"})
		s.Features = map[string]bool{feature.CancelBuild: true}
	})
	ctx := context.Background()
	client := fake.NewFakeTiltClient()
	cc := NewConfigsController(client)
	require.NoError(t, cc.OnChange(ctx, st, store.ChangeSummary{}))

	var tf v1alpha1.Tiltfile
	require.NoError(t, client.Get(ctx, types.NamespacedName{Name: model.MainTiltfileManifestName.String()}, &tf))
	expectedTfSpec := v1alpha1.TiltfileSpec{
		Path: tiltfile.ResolveFilename("fake-tiltfile-path"),
		Args: []string{"arg1", "arg2"},
		RestartOn: &v1alpha1.RestartOnSpec{
			FileWatches: []string{"configs:(Tiltfile)"},
		},
		StopOn: &v1alpha1.StopOnSpec{
			UIButtons: []string{"(Tiltfile)-cancel"},
		},
	}
	assert.Equal(t, expectedTfSpec, tf.Spec)

	var actualButton v1alpha1.UIButton
	name := types.NamespacedName{Name: uibutton.CancelButtonName(model.MainTiltfileManifestName.String())}
	err := client.Get(ctx, name, &actualButton)
	require.NoError(t, err)
	expectedButton := uibutton.CancelButton(model.MainTiltfileManifestName.String())
	assert.Equal(t, expectedButton.Spec, actualButton.Spec)
}

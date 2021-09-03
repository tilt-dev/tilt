package configs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestTriggerQueue(t *testing.T) {
	st := store.NewTestingStore()
	st.WithState(func(s *store.EngineState) {
		s.UpsertManifestTarget(store.NewManifestTarget(model.Manifest{Name: "a"}))
		s.UpsertManifestTarget(store.NewManifestTarget(model.Manifest{Name: "b"}))
		s.UpsertManifestTarget(store.NewManifestTarget(model.Manifest{Name: "c"}))
		s.AppendToTriggerQueue("a", model.BuildReasonFlagTriggerCLI)
		s.AppendToTriggerQueue("b", model.BuildReasonFlagTriggerWeb)
	})

	ctx := context.Background()
	client := fake.NewFakeTiltClient()
	cm, err := configmap.TriggerQueue(ctx, client)
	require.NoError(t, err)

	nnA := types.NamespacedName{Name: "a"}
	nnB := types.NamespacedName{Name: "b"}
	nnC := types.NamespacedName{Name: "c"}
	assert.False(t, configmap.InTriggerQueue(cm, nnA))

	tqs := NewTriggerQueueSubscriber(client)
	require.NoError(t, tqs.OnChange(ctx, st, store.ChangeSummary{}))

	cm, err = configmap.TriggerQueue(ctx, client)
	require.NoError(t, err)

	assert.True(t, configmap.InTriggerQueue(cm, nnA))
	assert.True(t, configmap.InTriggerQueue(cm, nnB))
	assert.False(t, configmap.InTriggerQueue(cm, nnC))

	assert.Equal(t, model.BuildReasonFlagTriggerCLI, configmap.TriggerQueueReason(cm, nnA))
	assert.Equal(t, model.BuildReasonFlagTriggerWeb, configmap.TriggerQueueReason(cm, nnB))
	assert.Equal(t, model.BuildReasonNone, configmap.TriggerQueueReason(cm, nnC))
}

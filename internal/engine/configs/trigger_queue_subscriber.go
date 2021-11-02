package configs

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Replicates the TriggerQueue back to the API server.
type TriggerQueueSubscriber struct {
	client     ctrlclient.Client
	lastUpdate *v1alpha1.ConfigMap
}

func NewTriggerQueueSubscriber(client ctrlclient.Client) *TriggerQueueSubscriber {
	return &TriggerQueueSubscriber{client: client}
}

func (s *TriggerQueueSubscriber) fromState(st store.RStore) *v1alpha1.ConfigMap {
	state := st.RLockState()
	defer st.RUnlockState()

	cm := &v1alpha1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configmap.TriggerQueueName,
		},
		Data: make(map[string]string, len(state.TriggerQueue)),
	}

	for i, v := range state.TriggerQueue {
		cm.Data[fmt.Sprintf("%d-name", i)] = v.String()

		ms, ok := state.ManifestState(v)
		if !ok {
			continue
		}
		cm.Data[fmt.Sprintf("%d-reason-code", i)] = fmt.Sprintf("%d", ms.TriggerReason)
	}
	return cm
}

func (s *TriggerQueueSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	cm := s.fromState(st)
	if s.lastUpdate != nil && apicmp.DeepEqual(cm.Data, s.lastUpdate.Data) {
		return nil
	}

	obj := v1alpha1.ConfigMap{
		ObjectMeta: cm.ObjectMeta,
	}
	_, err := controllerutil.CreateOrUpdate(ctx, s.client, &obj, func() error {
		obj.Data = cm.Data
		return nil
	})
	if err != nil {
		return err
	}
	s.lastUpdate = &obj
	return nil
}

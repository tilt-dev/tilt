package configs

import (
	"context"

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

	var entries []configmap.TriggerQueueEntry
	for _, mn := range state.TriggerQueue {
		entry := configmap.TriggerQueueEntry{
			Name: mn,
		}

		ms, ok := state.ManifestState(mn)
		if !ok {
			continue
		}
		entry.Reason = ms.TriggerReason

		entries = append(entries, entry)
	}

	result := configmap.TriggerQueueCreate(entries)
	return &result
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

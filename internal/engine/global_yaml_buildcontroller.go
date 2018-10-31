package engine

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"k8s.io/api/core/v1"
)

type GlobalYAMLBuildController struct {
	disabledForTesting     bool
	lastGlobalYAMLManifest model.YAMLManifest
	k8sClient              k8s.Client
}

func NewGlobalYAMLBuildController(k8sClient k8s.Client) *GlobalYAMLBuildController {
	return &GlobalYAMLBuildController{
		k8sClient: k8sClient,
	}
}

func (c *GlobalYAMLBuildController) OnChange(ctx context.Context, st *store.Store) {
	if c.disabledForTesting {
		return
	}

	state := st.RLockState()
	m := state.GlobalYAML
	st.RUnlockState()

	newK8sEntities := []k8s.K8sEntity{}
	if m.K8sYAML() != c.lastGlobalYAMLManifest.K8sYAML() {
		logger.Get(ctx).Debugf("Gotta rebuild/apply the global YAML!")
		entities, err := k8s.ParseYAMLFromString(m.K8sYAML())
		if err != nil {
			st.Dispatch(NewErrorAction(err))
			return
		}

		for _, e := range entities {
			e, err = k8s.InjectLabels(e, []k8s.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: m.ManifestName().String()}})
			if err != nil {
				st.Dispatch(NewErrorAction(err))
				return
			}

			// For development, image pull policy should never be set to "Always",
			// even if it might make sense to use "Always" in prod. People who
			// set "Always" for development are shooting their own feet.
			e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
			if err != nil {
				st.Dispatch(NewErrorAction(err))
				return
			}

			newK8sEntities = append(newK8sEntities, e)
		}

		err = c.k8sClient.Upsert(ctx, newK8sEntities)
		if err != nil {
			st.Dispatch(NewErrorAction(fmt.Errorf("Unable to upsert global YAML: %v", err)))
		} else {
			c.lastGlobalYAMLManifest = m
		}
	}
}

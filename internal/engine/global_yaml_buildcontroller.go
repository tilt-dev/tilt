package engine

import (
	"context"

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

func (c *GlobalYAMLBuildController) OnChange(ctx context.Context, dsr store.DispatchingStateReader) {
	if c.disabledForTesting {
		return
	}

	state := dsr.RLockState()
	m := state.GlobalYAML
	dsr.RUnlockState()

	newK8sEntities := []k8s.K8sEntity{}
	if m.K8sYAML() != c.lastGlobalYAMLManifest.K8sYAML() {
		entities, err := k8s.ParseYAMLFromString(m.K8sYAML())
		if err != nil {
			logger.Get(ctx).Infof("Error parsing global_yaml: %v", err)
			c.lastGlobalYAMLManifest = model.YAMLManifest{}
			dsr.Dispatch(GlobalYAMLApplyError{Error: err})
			return
		}
		dsr.Dispatch(GlobalYAMLApplyStartedAction{})

		for _, e := range entities {
			e, err = k8s.InjectLabels(e, []k8s.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: m.ManifestName().String()}})
			if err != nil {
				logger.Get(ctx).Infof("Error injecting labels in to global_yaml: %v", err)
				c.lastGlobalYAMLManifest = model.YAMLManifest{}
				dsr.Dispatch(GlobalYAMLApplyError{Error: err})
				return
			}

			// For development, image pull policy should never be set to "Always",
			// even if it might make sense to use "Always" in prod. People who
			// set "Always" for development are shooting their own feet.
			e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
			if err != nil {
				logger.Get(ctx).Infof("Error injecting image pull policy in to global_yaml: %v", err)
				dsr.Dispatch(GlobalYAMLApplyError{Error: err})
				c.lastGlobalYAMLManifest = model.YAMLManifest{}
				return
			}

			newK8sEntities = append(newK8sEntities, e)
		}

		err = c.k8sClient.Upsert(ctx, newK8sEntities)
		if err != nil {
			logger.Get(ctx).Infof("Error upserting global_yaml: %v", err)
			c.lastGlobalYAMLManifest = model.YAMLManifest{}
			dsr.Dispatch(GlobalYAMLApplyError{Error: err})
		} else {
			c.lastGlobalYAMLManifest = m
			dsr.Dispatch(GlobalYAMLApplyCompleteAction{})
		}
	}
}

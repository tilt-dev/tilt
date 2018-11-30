package engine

import (
	"context"

	"github.com/pkg/errors"
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

func (c *GlobalYAMLBuildController) OnChange(ctx context.Context, st store.RStore) {
	if c.disabledForTesting {
		return
	}

	state := st.RLockState()
	m := state.GlobalYAML
	st.RUnlockState()

	if m.K8sYAML() != c.lastGlobalYAMLManifest.K8sYAML() {
		c.lastGlobalYAMLManifest = m
		st.Dispatch(GlobalYAMLApplyStartedAction{})

		err := handleGlobalYamlChange(ctx, m, c.k8sClient)

		if err != nil {
			logger.Get(ctx).Infof(err.Error())
		}

		st.Dispatch(GlobalYAMLApplyCompleteAction{Error: err})
	}
}

func handleGlobalYamlChange(ctx context.Context, m model.YAMLManifest, kCli k8s.Client) error {
	entities, err := k8s.ParseYAMLFromString(m.K8sYAML())
	if err != nil {
		return errors.Wrap(err, "Error parsing global_yaml")
	}

	newK8sEntities := []k8s.K8sEntity{}

	for _, e := range entities {
		e, err = k8s.InjectLabels(e, []k8s.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: m.ManifestName().String()}})
		if err != nil {
			return errors.Wrap(err, "Error injecting labels in to global_yaml")
		}

		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return errors.Wrap(err, "Error injecting image pull policy in to global_yaml")
		}

		newK8sEntities = append(newK8sEntities, e)
	}

	err = kCli.Upsert(ctx, newK8sEntities)
	if err != nil {
		return errors.Wrap(err, "Error upserting global_yaml")
	}

	return nil
}

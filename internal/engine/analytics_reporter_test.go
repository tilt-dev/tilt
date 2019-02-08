package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func TestAnalyticsReporter_Everything(t *testing.T) {
	reducer := func(ctx context.Context, engineState *store.EngineState, action store.Action) {}
	ar := AnalyticsReporter{
		a:       analytics.NewMemoryAnalytics(),
		store:   store.NewStore(reducer, store.LogActionsFlag(false)),
		started: false,
	}

	manifestCount := 0
	nextManifest := func() model.Manifest {
		manifestCount++
		return model.Manifest{Name: model.ManifestName(fmt.Sprintf("manifest%d", manifestCount))}
	}

	manifests := []model.Manifest{
		nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.FastBuild{}}),
		nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.StaticBuild{}}),
		nextManifest().WithDeployTarget(model.K8sTarget{}),
		nextManifest().WithDeployTarget(model.K8sTarget{}),
		nextManifest().WithDeployTarget(model.K8sTarget{}),
		nextManifest().WithDeployTarget(model.DockerComposeTarget{}),
		nextManifest().WithDeployTarget(model.DockerComposeTarget{}),
		nextManifest().WithDeployTarget(model.DockerComposeTarget{}),
		nextManifest().WithDeployTarget(model.DockerComposeTarget{}),
	}

	state := ar.store.LockMutableStateForTesting()
	for _, m := range manifests {
		state.UpsertManifestTarget(store.NewManifestTarget(m))
	}
	state.TiltStartTime = time.Now()

	state.CompletedBuildCount = 3

	ar.store.UnlockMutableState()

	ar.report()

	ar.a.Flush(500 * time.Second)

	ma := ar.a.(*analytics.MemoryAnalytics)

	expectedTags := map[string]string{
		"builds.completed_count":       "3",
		"resource.count":               "9",
		"resource.dockercompose.count": "4",
		"resource.fastbuild.count":     "1",
		"resource.k8s.count":           "3",
		"tiltfile.error":               "false",
		"up.starttime":                 state.TiltStartTime.Format(time.RFC3339),
	}

	assert.Equal(t, []analytics.CountEvent{{
		Name: "up.running",
		Tags: expectedTags,
		N:    1,
	}}, ma.Counts)
}

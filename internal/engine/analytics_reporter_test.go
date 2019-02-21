package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func TestAnalyticsReporter_Everything(t *testing.T) {
	tf := newAnalyticsReporterTestFixture()

	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.FastBuild{}}))   // k8s, fastbuild
	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.StaticBuild{}})) // k8s
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))                                   // k8s, unbuilt
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))                                   // k8s, unbuilt
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))                                   // k8s, unbuilt
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))                         // dc
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))                         // dc
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))                         // dc
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))                         // dc

	state := tf.ar.store.LockMutableStateForTesting()
	state.TiltStartTime = time.Now()

	state.CompletedBuildCount = 3

	tf.ar.store.UnlockMutableState()

	tf.run()

	expectedTags := map[string]string{
		"builds.completed_count":          "3",
		"resource.count":                  "9",
		"resource.dockercompose.count":    "4",
		"resource.unbuiltresources.count": "3",
		"resource.fastbuild.count":        "1",
		"resource.k8s.count":              "3",
		"tiltfile.error":                  "false",
		"up.starttime":                    state.TiltStartTime.Format(time.RFC3339),
	}

	tf.assertStats(t, expectedTags)
}

func TestAnalyticsReporter_TiltfileError(t *testing.T) {
	tf := newAnalyticsReporterTestFixture()

	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.FastBuild{}}))
	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.StaticBuild{}}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))

	state := tf.ar.store.LockMutableStateForTesting()
	state.TiltStartTime = time.Now()

	state.CompletedBuildCount = 3

	state.LastTiltfileBuild = model.BuildRecord{Error: errors.New("foo")}

	tf.ar.store.UnlockMutableState()

	tf.run()

	expectedTags := map[string]string{
		"builds.completed_count": "3",
		"tiltfile.error":         "true",
		"up.starttime":           state.TiltStartTime.Format(time.RFC3339),
	}

	tf.assertStats(t, expectedTags)
}

type analyticsReporterTestFixture struct {
	manifestCount int
	ar            AnalyticsReporter
}

func newAnalyticsReporterTestFixture() *analyticsReporterTestFixture {
	reducer := func(ctx context.Context, engineState *store.EngineState, action store.Action) {}
	ar := AnalyticsReporter{
		a:       analytics.NewMemoryAnalytics(),
		store:   store.NewStore(reducer, store.LogActionsFlag(false)),
		started: false,
	}

	return &analyticsReporterTestFixture{
		manifestCount: 0,
		ar:            ar,
	}
}

func (artf *analyticsReporterTestFixture) addManifest(m model.Manifest) {
	state := artf.ar.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	artf.ar.store.UnlockMutableState()
}

func (artf *analyticsReporterTestFixture) nextManifest() model.Manifest {
	artf.manifestCount++
	return model.Manifest{Name: model.ManifestName(fmt.Sprintf("manifest%d", artf.manifestCount))}
}

func (artf *analyticsReporterTestFixture) run() {
	artf.ar.report()

	artf.ar.a.Flush(500 * time.Second)
}

func (artf *analyticsReporterTestFixture) assertStats(t *testing.T, expectedTags map[string]string) {
	ma := artf.ar.a.(*analytics.MemoryAnalytics)

	expectedCounts := []analytics.CountEvent{{Name: "up.running", N: 1, Tags: expectedTags}}
	assert.Equal(t, expectedCounts, ma.Counts)
}

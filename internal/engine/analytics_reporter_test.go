package engine

import (
	"errors"
	"fmt"
	"testing"
	"time"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

var (
	fb = model.FastBuild{HotReload: true}                                            // non-empty FastBuild
	lu = model.LiveUpdate{Steps: []model.LiveUpdateStep{model.LiveUpdateSyncStep{}}} // non-empty LiveUpdate

	imgTargDB       = model.ImageTarget{BuildDetails: model.DockerBuild{}}
	imgTargFB       = model.ImageTarget{BuildDetails: fb}
	imgTargDBWithFB = model.ImageTarget{BuildDetails: model.DockerBuild{FastBuild: fb}}
	imgTargDBWithLU = model.ImageTarget{BuildDetails: model.DockerBuild{LiveUpdate: lu}}

	kTarg = model.K8sTarget{}
	dTarg = model.DockerComposeTarget{}
)

func TestAnalyticsReporter_Everything(t *testing.T) {
	tf := newAnalyticsReporterTestFixture()

	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargFB))                               // fastbuild
	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargDB).WithDeployTarget(kTarg))       // k8s
	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargDBWithFB))                         // anyfastbuild
	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargDBWithLU))                         // liveupdate
	tf.addManifest(tf.nextManifest().WithDeployTarget(kTarg))                                  // k8s, unbuilt
	tf.addManifest(tf.nextManifest().WithDeployTarget(kTarg))                                  // k8s, unbuilt
	tf.addManifest(tf.nextManifest().WithDeployTarget(kTarg))                                  // k8s, unbuilt
	tf.addManifest(tf.nextManifest().WithDeployTarget(dTarg))                                  // dc
	tf.addManifest(tf.nextManifest().WithDeployTarget(dTarg))                                  // dc
	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargDBWithLU).WithDeployTarget(dTarg)) // dc, liveupdate

	state := tf.ar.store.LockMutableStateForTesting()
	state.TiltStartTime = time.Now()

	state.CompletedBuildCount = 3

	tf.ar.store.UnlockMutableState()

	tf.run()

	expectedTags := map[string]string{
		"builds.completed_count":          "3",
		"resource.count":                  "10",
		"resource.dockercompose.count":    "3",
		"resource.unbuiltresources.count": "3",
		"resource.fastbuild.count":        "1",
		"resource.anyfastbuild.count":     "2",
		"resource.liveupdate.count":       "2",
		"resource.k8s.count":              "4",
		"tiltfile.error":                  "false",
		"up.starttime":                    state.TiltStartTime.Format(time.RFC3339),
	}

	tf.assertStats(t, expectedTags)
}

func TestAnalyticsReporter_TiltfileError(t *testing.T) {
	tf := newAnalyticsReporterTestFixture()

	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.FastBuild{}}))
	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.DockerBuild{}}))
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
	ma            *analytics.MemoryAnalytics
}

func newAnalyticsReporterTestFixture() *analyticsReporterTestFixture {
	st, _ := store.NewStoreForTesting()
	ma, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(tiltanalytics.NullOpter{})
	ar := AnalyticsReporter{
		a:       a,
		store:   st,
		started: false,
	}

	return &analyticsReporterTestFixture{
		manifestCount: 0,
		ar:            ar,
		ma:            ma,
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
	expectedCounts := []analytics.CountEvent{{Name: "up.running", N: 1, Tags: expectedTags}}
	assert.Equal(t, expectedCounts, artf.ma.Counts)
}

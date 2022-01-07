package analytics

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var (
	lu = v1alpha1.LiveUpdateSpec{Syncs: []v1alpha1.LiveUpdateSync{{ContainerPath: "/"}}} // non-empty LiveUpdate

	imgTargDB       = model.ImageTarget{BuildDetails: model.DockerBuild{}}
	imgTargDBWithLU = model.ImageTarget{LiveUpdateSpec: lu, BuildDetails: model.DockerBuild{}}

	kTarg = model.K8sTarget{}
	dTarg = model.DockerComposeTarget{}
)

var (
	r1 = "gcr.io/some-project-162817/one"
	r2 = "gcr.io/some-project-162817/two"
	r3 = "gcr.io/some-project-162817/three"
	r4 = "gcr.io/some-project-162817/four"

	iTargWithRef1     = iTargetForRef(r1).WithLiveUpdateSpec("one", lu).WithBuildDetails(model.DockerBuild{})
	iTargWithRef2     = iTargetForRef(r2).WithLiveUpdateSpec("two", lu).WithBuildDetails(model.DockerBuild{})
	iTargWithRef3     = iTargetForRef(r3).WithLiveUpdateSpec("three", lu).WithBuildDetails(model.DockerBuild{})
	iTargWithRef4NoLU = iTargetForRef(r4)
)

func TestAnalyticsReporter_Everything(t *testing.T) {
	tf := newAnalyticsReporterTestFixture(t)

	tf.addManifest(
		tf.nextManifest().
			WithLabels(map[string]string{"k8s": "k8s"}).
			WithImageTarget(imgTargDB).
			WithDeployTarget(kTarg)) // k8s

	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargDBWithLU)) // liveupdate

	tf.addManifest(
		tf.nextManifest().
			WithLabels(map[string]string{"k8s": "k8s1"}).
			WithDeployTarget(kTarg)) // k8s, unbuilt
	tf.addManifest(
		tf.nextManifest().
			WithLabels(map[string]string{"k8s": "k8s2"}).
			WithDeployTarget(kTarg)) // k8s, unbuilt
	tf.addManifest(
		tf.nextManifest().
			WithLabels(map[string]string{"k8s": "k8s3"}).
			WithDeployTarget(kTarg)) // k8s, unbuilt

	tf.addManifest(
		tf.nextManifest().
			WithLabels(map[string]string{"dc": "dc1"}).
			WithDeployTarget(dTarg)) // dc
	tf.addManifest(
		tf.nextManifest().
			WithLabels(map[string]string{"dc": "dc2"}).
			WithDeployTarget(dTarg)) // dc

	tf.addManifest(tf.nextManifest().WithImageTarget(imgTargDBWithLU).WithDeployTarget(dTarg)) // dc, liveupdate
	tf.addManifest(tf.nextManifest().WithImageTargets(
		[]model.ImageTarget{imgTargDBWithLU, imgTargDBWithLU})) // liveupdate, multipleimageliveupdate

	state := tf.st.LockMutableStateForTesting()
	state.TiltStartTime = time.Now()

	state.CompletedBuildCount = 3

	tf.st.UnlockMutableState()
	tf.kClient.Registry, _ = container.NewRegistryWithHostFromCluster("localhost:5000", "registry:5000")

	tf.run()

	expectedTags := map[string]string{
		"builds.completed_count":                              "3",
		"resource.count":                                      "9",
		"resource.dockercompose.count":                        "3",
		"resource.unbuiltresources.count":                     "3",
		"resource.liveupdate.count":                           "3",
		"resource.k8s.count":                                  "4",
		"resource.sameimagemultiplecontainerliveupdate.count": "0", // tests for this below
		"resource.multipleimageliveupdate.count":              "1",
		"tiltfile.error":                                      "false",
		"up.starttime":                                        state.TiltStartTime.Format(time.RFC3339),
		"env":                                                 string(k8s.EnvDockerDesktop),
		"term_mode":                                           "0",
		"k8s.runtime":                                         "docker",
		"k8s.registry.host":                                   "1",
		"k8s.registry.hostFromCluster":                        "1",
		"label.count":                                         "2",
		"feature.testflag_enabled.enabled":                    "true",
	}

	tf.assertStats(t, expectedTags)
}

func TestAnalyticsReporter_SameImageMultiContainer(t *testing.T) {
	tf := newAnalyticsReporterTestFixture(t)

	injectCountsA := map[string]int{
		r1: 1,
		r2: 2,
	}
	k8sTargA := kTarg.WithRefInjectCounts(injectCountsA)
	tf.addManifest(tf.nextManifest().
		WithImageTarget(iTargWithRef1).
		WithImageTarget(iTargWithRef2).
		WithDeployTarget(k8sTargA))

	injectCountsB := map[string]int{
		r2: 2,
		r3: 3,
	}
	k8sTargB := kTarg.WithRefInjectCounts(injectCountsB)
	tf.addManifest(tf.nextManifest().
		WithImageTarget(iTargWithRef2).
		WithImageTarget(iTargWithRef3).
		WithDeployTarget(k8sTargB))

	tf.run()

	assert.Equal(t, "2", tf.ma.Counts[0].Tags["resource.sameimagemultiplecontainerliveupdate.count"])
}

func TestAnalyticsReporter_SameImageMultiContainer_NoIncr(t *testing.T) {
	tf := newAnalyticsReporterTestFixture(t)

	injectCounts := map[string]int{
		r1: 1,
		r4: 2,
	}
	k8sTarg := kTarg.WithRefInjectCounts(injectCounts)
	tf.addManifest(tf.nextManifest().
		WithImageTarget(iTargWithRef1).
		WithImageTarget(iTargWithRef4NoLU). // injects multiple times, but no LU so won't record stat for it
		WithDeployTarget(k8sTarg))

	tf.run()

	assert.Equal(t, "0", tf.ma.Counts[0].Tags["resource.sameimagemultiplecontainerliveupdate.count"])
}

func TestAnalyticsReporter_TiltfileError(t *testing.T) {
	tf := newAnalyticsReporterTestFixture(t)

	tf.addManifest(tf.nextManifest().WithImageTarget(model.ImageTarget{BuildDetails: model.DockerBuild{}}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.K8sTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))
	tf.addManifest(tf.nextManifest().WithDeployTarget(model.DockerComposeTarget{}))

	state := tf.st.LockMutableStateForTesting()
	state.TiltStartTime = time.Now()

	state.CompletedBuildCount = 3

	state.TiltfileStates[model.MainTiltfileManifestName].AddCompletedBuild(model.BuildRecord{Error: errors.New("foo")})

	tf.st.UnlockMutableState()

	tf.run()

	expectedTags := map[string]string{
		"builds.completed_count":           "3",
		"tiltfile.error":                   "true",
		"up.starttime":                     state.TiltStartTime.Format(time.RFC3339),
		"env":                              string(k8s.EnvDockerDesktop),
		"term_mode":                        "0",
		"k8s.runtime":                      "docker",
		"feature.testflag_enabled.enabled": "true",
	}

	tf.assertStats(t, expectedTags)
}

type analyticsReporterTestFixture struct {
	manifestCount int
	ar            *AnalyticsReporter
	ma            *analytics.MemoryAnalytics
	kClient       *k8s.FakeK8sClient
	st            *store.TestingStore
}

func newAnalyticsReporterTestFixture(t testing.TB) *analyticsReporterTestFixture {
	st := store.NewTestingStore()
	opter := tiltanalytics.NewFakeOpter(analytics.OptIn)
	ma, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(opter)
	kClient := k8s.NewFakeK8sClient(t)
	features := feature.Defaults{
		"testflag_disabled":     feature.Value{Enabled: false},
		"testflag_enabled":      feature.Value{Enabled: true},
		"obsoleteflag_enabled":  feature.Value{Status: feature.Obsolete, Enabled: true},
		"obsoleteflag_disabled": feature.Value{Status: feature.Obsolete, Enabled: false},
	}

	state := st.LockMutableStateForTesting()
	state.Features = feature.FromDefaults(features).ToEnabled()
	st.UnlockMutableState()

	ar := ProvideAnalyticsReporter(a, st, kClient, k8s.EnvDockerDesktop, features)
	return &analyticsReporterTestFixture{
		manifestCount: 0,
		ar:            ar,
		ma:            ma,
		kClient:       kClient,
		st:            st,
	}
}

func (artf *analyticsReporterTestFixture) addManifest(m model.Manifest) {
	state := artf.st.LockMutableStateForTesting()
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	artf.st.UnlockMutableState()
}

func (artf *analyticsReporterTestFixture) nextManifest() model.Manifest {
	artf.manifestCount++
	return model.Manifest{Name: model.ManifestName(fmt.Sprintf("manifest%d", artf.manifestCount))}
}

func (artf *analyticsReporterTestFixture) run() {
	artf.ar.report(context.Background())

	artf.ar.a.Flush(500 * time.Second)
}

func (artf *analyticsReporterTestFixture) assertStats(t *testing.T, expectedTags map[string]string) {
	expectedCounts := []analytics.CountEvent{{Name: "up.running", N: 1, Tags: expectedTags}}
	assert.Equal(t, expectedCounts, artf.ma.Counts)
}

func iTargetForRef(ref string) model.ImageTarget {
	named := container.MustParseNamed(ref)
	selector := container.NameSelector(named)
	return model.MustNewImageTarget(selector)
}

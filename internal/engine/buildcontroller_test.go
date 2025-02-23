package engine

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/uibutton"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/configmap"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestBuildControllerLocalResource(t *testing.T) {
	f := newTestFixture(t)

	dep := f.JoinPath("stuff.json")
	manifest := manifestbuilder.New(f, "local").
		WithLocalResource("echo beep boop", []string{dep}).
		Build()
	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete()
	lt := manifest.LocalTarget()
	assert.Equal(t, lt, call.local())

	f.fsWatcher.Events <- watch.NewFileEvent(dep)

	call = f.nextCallComplete()
	assert.Equal(t, lt, call.local())

	f.WaitUntilManifestState("local target manifest state not updated", "local", func(ms store.ManifestState) bool {
		lrs := ms.RuntimeState.(store.LocalRuntimeState)
		return !lrs.LastReadyOrSucceededTime.IsZero() && lrs.RuntimeStatus() == v1alpha1.RuntimeStatusNotApplicable
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerManualTriggerBuildReasonInit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		triggerMode model.TriggerMode
	}{
		{"fully manual", model.TriggerModeManual},
		{"manual with auto init", model.TriggerModeManualWithAutoInit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixture(t)
			mName := model.ManifestName("foobar")
			manifest := f.newManifest(mName.String()).WithTriggerMode(tc.triggerMode)
			manifests := []model.Manifest{manifest}
			f.Start(manifests)

			// make sure there's a first build
			if !manifest.TriggerMode.AutoInitial() {
				f.store.Dispatch(store.AppendToTriggerQueueAction{Name: mName})
			}

			f.nextCallComplete()

			f.withManifestState(mName, func(ms store.ManifestState) {
				require.Equal(t, tc.triggerMode.AutoInitial(), ms.LastBuild().Reason.Has(model.BuildReasonFlagInit))
			})
		})
	}
}

func TestTriggerModes(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		triggerMode                model.TriggerMode
		expectInitialBuild         bool
		expectBuildWhenFilesChange bool
	}{
		{name: "fully auto", triggerMode: model.TriggerModeAuto, expectInitialBuild: true, expectBuildWhenFilesChange: true},
		{name: "auto with manual init", triggerMode: model.TriggerModeAutoWithManualInit, expectInitialBuild: false, expectBuildWhenFilesChange: true},
		{name: "manual with auto init", triggerMode: model.TriggerModeManualWithAutoInit, expectInitialBuild: true, expectBuildWhenFilesChange: false},
		{name: "fully manual", triggerMode: model.TriggerModeManual, expectInitialBuild: false, expectBuildWhenFilesChange: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixture(t)

			manifest := f.simpleManifestWithTriggerMode("foobar", tc.triggerMode)
			manifests := []model.Manifest{manifest}
			f.Start(manifests)

			// basic check of trigger mode properties
			assert.Equal(t, tc.expectInitialBuild, tc.triggerMode.AutoInitial())
			assert.Equal(t, tc.expectBuildWhenFilesChange, tc.triggerMode.AutoOnChange())

			// if we expect an initial build from the manifest, wait for it to complete
			if tc.expectInitialBuild {
				f.nextCallComplete("initial build")
			}

			f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))
			f.WaitUntil("pending change appears", func(st store.EngineState) bool {
				return st.BuildStatus(manifest.ImageTargetAt(0).ID()).CountPendingFileChanges() >= 1
			})

			if !tc.expectBuildWhenFilesChange {
				f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")
				return
			}

			call := f.nextCallComplete("build after file change")
			state := call.oneImageState()
			assert.Equal(t, []string{f.JoinPath("main.go")}, state.FilesChanged())
		})
	}
}

func TestBuildControllerImageBuildTrigger(t *testing.T) {
	for _, tc := range []struct {
		name               string
		triggerMode        model.TriggerMode
		filesChanged       bool
		expectedImageBuild bool
	}{
		{name: "fully manual with change", triggerMode: model.TriggerModeManual, filesChanged: true, expectedImageBuild: false},
		{name: "manual with auto init with change", triggerMode: model.TriggerModeManualWithAutoInit, filesChanged: true, expectedImageBuild: false},
		{name: "fully manual without change", triggerMode: model.TriggerModeManual, filesChanged: false, expectedImageBuild: true},
		{name: "manual with auto init without change", triggerMode: model.TriggerModeManualWithAutoInit, filesChanged: false, expectedImageBuild: true},
		{name: "fully auto without change", triggerMode: model.TriggerModeAuto, filesChanged: false, expectedImageBuild: true},
		{name: "auto with manual init without change", triggerMode: model.TriggerModeAutoWithManualInit, filesChanged: false, expectedImageBuild: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixture(t)
			mName := model.ManifestName("foobar")

			manifest := f.simpleManifestWithTriggerMode(mName, tc.triggerMode)
			manifests := []model.Manifest{manifest}
			f.Start(manifests)

			// if we expect an initial build from the manifest, wait for it to complete
			if manifest.TriggerMode.AutoInitial() {
				f.nextCallComplete()
			}

			expectedFiles := []string{}
			if tc.filesChanged {
				expectedFiles = append(expectedFiles, f.JoinPath("main.go"))
				f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))
			}
			f.WaitUntil("pending change appears", func(st store.EngineState) bool {
				return st.BuildStatus(manifest.ImageTargetAt(0).ID()).CountPendingFileChanges() >= len(expectedFiles)
			})

			if manifest.TriggerMode.AutoOnChange() {
				f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")
			}

			f.store.Dispatch(store.AppendToTriggerQueueAction{Name: mName})
			call := f.nextCallComplete()
			state := call.oneImageState()
			assert.Equal(t, expectedFiles, state.FilesChanged())
			assert.Equal(t, tc.expectedImageBuild, state.FullBuildTriggered)

			f.WaitUntil("manifest removed from queue", func(st store.EngineState) bool {
				for _, mn := range st.TriggerQueue {
					if mn == mName {
						return false
					}
				}
				return true
			})
		})
	}
}

func TestBuildQueueOrdering(t *testing.T) {
	f := newTestFixture(t)

	m1 := f.newManifestWithRef("manifest1", container.MustParseNamed("manifest1")).
		WithTriggerMode(model.TriggerModeManualWithAutoInit)
	m2 := f.newManifestWithRef("manifest2", container.MustParseNamed("manifest2")).
		WithTriggerMode(model.TriggerModeManualWithAutoInit)
	m3 := f.newManifestWithRef("manifest3", container.MustParseNamed("manifest3")).
		WithTriggerMode(model.TriggerModeManual)
	m4 := f.newManifestWithRef("manifest4", container.MustParseNamed("manifest4")).
		WithTriggerMode(model.TriggerModeManual)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m4, m2, m3, m1}
	f.Start(manifests)

	expectedInitialBuildCount := 0
	for _, m := range manifests {
		if m.TriggerMode.AutoInitial() {
			expectedInitialBuildCount++
			f.nextCall()
		}
	}

	f.waitForCompletedBuildCount(expectedInitialBuildCount)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return st.BuildStatus(m1.ImageTargetAt(0).ID()).HasPendingFileChanges() &&
			st.BuildStatus(m2.ImageTargetAt(0).ID()).HasPendingFileChanges() &&
			st.BuildStatus(m3.ImageTargetAt(0).ID()).HasPendingFileChanges() &&
			st.BuildStatus(m4.ImageTargetAt(0).ID()).HasPendingFileChanges()
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest1"})
	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest2"})
	time.Sleep(10 * time.Millisecond)
	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest4"})

	for i := range manifests {
		expName := fmt.Sprintf("manifest%d", i+1)
		call := f.nextCall()
		imgID := call.firstImgTarg().ID().String()
		if assert.True(t, strings.HasSuffix(imgID, expName),
			"expected to get manifest '%s' but instead got: '%s' (checking suffix for manifest name)", expName, imgID) {
			assert.Equal(t, []string{f.JoinPath("main.go")}, call.oneImageState().FilesChanged(),
				"for manifest '%s", expName)
		}
	}
	f.waitForCompletedBuildCount(expectedInitialBuildCount + len(manifests))
}

func TestBuildQueueAndAutobuildOrdering(t *testing.T) {
	f := newTestFixture(t)

	// changes to this dir. will register with our manual manifests
	dirManual := f.JoinPath("dirManual/")
	// changes to this dir. will register with our automatic manifests
	dirAuto := f.JoinPath("dirAuto/")

	m1 := f.newDockerBuildManifestWithBuildPath("manifest1", dirManual).WithTriggerMode(model.TriggerModeManualWithAutoInit)
	m2 := f.newDockerBuildManifestWithBuildPath("manifest2", dirManual).WithTriggerMode(model.TriggerModeManualWithAutoInit)
	m3 := f.newDockerBuildManifestWithBuildPath("manifest3", dirManual).WithTriggerMode(model.TriggerModeManual)
	m4 := f.newDockerBuildManifestWithBuildPath("manifest4", dirManual).WithTriggerMode(model.TriggerModeManual)
	m5 := f.newDockerBuildManifestWithBuildPath("manifest5", dirAuto).WithTriggerMode(model.TriggerModeAuto)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m5, m4, m2, m3, m1}
	f.Start(manifests)

	expectedInitialBuildCount := 0
	for _, m := range manifests {
		if m.TriggerMode.AutoInitial() {
			expectedInitialBuildCount++
			f.nextCall()
		}
	}

	f.waitForCompletedBuildCount(expectedInitialBuildCount)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("dirManual/main.go"))
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return st.BuildStatus(m1.ImageTargetAt(0).ID()).HasPendingFileChanges() &&
			st.BuildStatus(m2.ImageTargetAt(0).ID()).HasPendingFileChanges() &&
			st.BuildStatus(m3.ImageTargetAt(0).ID()).HasPendingFileChanges() &&
			st.BuildStatus(m4.ImageTargetAt(0).ID()).HasPendingFileChanges()
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest1"})
	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest2"})
	// make our one auto-trigger manifest build - should be evaluated LAST, after
	// all the manual manifests waiting in the queue
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("dirAuto/main.go"))
	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: "manifest4"})

	for i := range manifests {
		call := f.nextCall()
		imgTargID := call.firstImgTarg().ID().String()
		expectSuffix := fmt.Sprintf("manifest%d", i+1)
		assert.True(t, strings.HasSuffix(imgTargID, expectSuffix), "expect this call to have image target ...%s (got: %s)", expectSuffix, imgTargID)

		if i < 4 {
			assert.Equal(t, []string{f.JoinPath("dirManual/main.go")}, call.oneImageState().FilesChanged(), "for manifest %d", i+1)
		} else {
			// the automatic manifest
			assert.Equal(t, []string{f.JoinPath("dirAuto/main.go")}, call.oneImageState().FilesChanged(), "for manifest %d", i+1)
		}
	}
	f.waitForCompletedBuildCount(len(manifests) + expectedInitialBuildCount)
}

// any manifests without image targets should be deployed before any manifests WITH image targets
func TestBuildControllerNoBuildManifestsFirst(t *testing.T) {
	f := newTestFixture(t)

	manifests := make([]model.Manifest, 10)
	for i := 0; i < 10; i++ {
		manifests[i] = f.newManifest(fmt.Sprintf("built%d", i+1))
	}

	for _, i := range []int{3, 7, 8} {
		manifests[i] = manifestbuilder.New(f, model.ManifestName(fmt.Sprintf("unbuilt%d", i+1))).
			WithK8sYAML(SanchoYAML).
			Build()
	}
	f.Start(manifests)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
	}

	// throwing a bunch of elements at it to increase confidence we maintain order between built and unbuilt
	// this might miss bugs since we might just get these elements back in the right order via luck
	expectedBuildOrder := []string{
		"unbuilt4",
		"unbuilt8",
		"unbuilt9",
		"built1",
		"built2",
		"built3",
		"built5",
		"built6",
		"built7",
		"built10",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerUnresourcedYAMLFirst(t *testing.T) {
	f := newTestFixture(t)

	manifests := []model.Manifest{
		f.newManifest("built1"),
		f.newManifest("built2"),
		f.newManifest("built3"),
		f.newManifest("built4"),
	}

	manifests = append(manifests, manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).Build())
	f.Start(manifests)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
	}

	expectedBuildOrder := []string{
		model.UnresourcedYAMLManifestName.String(),
		"built1",
		"built2",
		"built3",
		"built4",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerRespectDockerComposeOrder(t *testing.T) {
	f := newTestFixture(t)

	sancho := NewSanchoLiveUpdateDCManifest(f)
	redis := manifestbuilder.New(f, "redis").WithDockerCompose().Build()
	donQuixote := manifestbuilder.New(f, "don-quixote").WithDockerCompose().Build()
	manifests := []model.Manifest{redis, sancho, donQuixote}
	f.Start(manifests)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		observedBuildOrder = append(observedBuildOrder, call.dc().Name.String())
	}

	// If these were Kubernetes resources, we would try to deploy don-quixote
	// before sancho, because it doesn't have an image build.
	//
	// But this would be wrong, because Docker Compose has stricter ordering requirements, see:
	// https://docs.docker.com/compose/startup-order/
	expectedBuildOrder := []string{
		"redis",
		"sancho",
		"don-quixote",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerLocalResourcesBeforeClusterResources(t *testing.T) {
	f := newTestFixture(t)

	manifests := []model.Manifest{
		f.newManifest("clusterBuilt1"),
		f.newManifest("clusterBuilt2"),
		manifestbuilder.New(f, "clusterUnbuilt").
			WithK8sYAML(SanchoYAML).Build(),
		manifestbuilder.New(f, "local1").
			WithLocalResource("echo local1", nil).Build(),
		f.newManifest("clusterBuilt3"),
		manifestbuilder.New(f, "local2").
			WithLocalResource("echo local2", nil).Build(),
	}

	manifests = append(manifests, manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).Build())
	f.Start(manifests)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		if !call.k8s().Empty() {
			observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
			continue
		}
		observedBuildOrder = append(observedBuildOrder, call.local().Name.String())
	}

	expectedBuildOrder := []string{
		"local1",
		"local2",
		model.UnresourcedYAMLManifestName.String(),
		"clusterUnbuilt",
		"clusterBuilt1",
		"clusterBuilt2",
		"clusterBuilt3",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerResourceDeps(t *testing.T) {
	f := newTestFixture(t)

	depGraph := map[string][]string{
		"a": {"e"},
		"b": {"e"},
		"c": {"d", "g"},
		"d": {},
		"e": {"d", "f"},
		"f": {"c"},
		"g": {},
	}

	var manifests []model.Manifest
	podBuilders := make(map[string]podbuilder.PodBuilder)
	for name, deps := range depGraph {
		m := f.newManifest(name)
		for _, dep := range deps {
			m.ResourceDependencies = append(m.ResourceDependencies, model.ManifestName(dep))
		}
		manifests = append(manifests, m)
		podBuilders[name] = f.registerForDeployer(m)
	}

	f.Start(manifests)

	var observedOrder []string
	for i := range manifests {
		call := f.nextCall("%dth build. have built: %v", i, observedOrder)
		name := call.k8s().Name.String()
		observedOrder = append(observedOrder, name)
		f.podEvent(podBuilders[name].WithContainerReady(true).Build())
	}

	var expectedManifests []string
	for name := range depGraph {
		expectedManifests = append(expectedManifests, name)
	}

	// make sure everything built
	require.ElementsMatch(t, expectedManifests, observedOrder)

	buildIndexes := make(map[string]int)
	for i, n := range observedOrder {
		buildIndexes[n] = i
	}

	// make sure it happened in an acceptable order
	for name, deps := range depGraph {
		for _, dep := range deps {
			require.Truef(t, buildIndexes[name] > buildIndexes[dep], "%s built before %s, contrary to resource deps", name, dep)
		}
	}
}

// normally, local builds go before k8s builds
// if the local build depends on the k8s build, the k8s build should go first
func TestBuildControllerResourceDepTrumpsLocalResourcePriority(t *testing.T) {
	f := newTestFixture(t)

	k8sManifest := f.newManifest("foo")
	pb := f.registerForDeployer(k8sManifest)
	localManifest := manifestbuilder.New(f, "bar").
		WithLocalResource("echo bar", nil).
		WithResourceDeps("foo").Build()
	manifests := []model.Manifest{localManifest, k8sManifest}
	f.Start(manifests)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		if !call.k8s().Empty() {
			observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
			pb = pb.WithContainerReady(true)
			f.podEvent(pb.Build())
			continue
		}
		observedBuildOrder = append(observedBuildOrder, call.local().Name.String())
	}

	expectedBuildOrder := []string{"foo", "bar"}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

// bar depends on foo, we build foo three times before marking it ready, and make sure bar waits
func TestBuildControllerResourceDepTrumpsInitialBuild(t *testing.T) {
	f := newTestFixture(t)

	foo := manifestbuilder.New(f, "foo").
		WithLocalResource("foo cmd", []string{f.JoinPath("foo")}).
		Build()
	bar := manifestbuilder.New(f, "bar").
		WithLocalResource("bar cmd", []string{f.JoinPath("bar")}).
		WithResourceDeps("foo").
		Build()
	manifests := []model.Manifest{foo, bar}
	f.SetNextBuildError(errors.New("failure"))
	f.Start(manifests)

	call := f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	f.SetNextBuildError(errors.New("failure"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	// now that the foo build has succeeded, bar should get queued
	call = f.nextCall()
	require.Equal(t, "bar", call.local().Name.String())
}

// bar depends on foo. make sure bar waits on foo even as foo fails
func TestBuildControllerResourceDepTrumpsPendingBuild(t *testing.T) {
	f := newTestFixture(t)

	foo := manifestbuilder.New(f, "foo").
		WithLocalResource("foo cmd", []string{f.JoinPath("foo")}).
		Build()
	bar := manifestbuilder.New(f, "bar").
		WithLocalResource("bar cmd", []string{f.JoinPath("bar")}).
		WithResourceDeps("foo").
		Build()

	manifests := []model.Manifest{bar, foo}
	f.SetNextBuildError(errors.New("failure"))
	f.Start(manifests)

	// trigger a change for bar so that it would try to build if not for its resource dep
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("bar", "main.go"))

	call := f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	// since the foo build succeeded, bar should now queue
	call = f.nextCall()
	require.Equal(t, "bar", call.local().Name.String())
}

func TestBuildControllerWontBuildManifestIfNoSlotsAvailable(t *testing.T) {
	f := newTestFixture(t)
	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(2)

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	manC := f.newDockerBuildManifestWithBuildPath("manC", f.JoinPath("c"))
	f.Start([]model.Manifest{manA, manB, manC})
	f.completeAndCheckBuildsForManifests(manA, manB, manC)

	// start builds for all manifests (we only have 2 build slots)
	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")
	f.editFileAndWaitForManifestBuilding("manB", "b/main.go")
	f.editFileAndAssertManifestNotBuilding("manC", "c/main.go")

	// Complete one build...
	f.completeBuildForManifest(manA)
	call := f.nextCall("expect manA build complete")
	f.assertCallIsForManifestAndFiles(call, manA, "a/main.go")

	// ...and now there's a free build slot for 'manC'
	f.waitUntilManifestBuilding("manC")

	// complete the rest (can't guarantee order)
	f.completeAndCheckBuildsForManifests(manB, manC)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

// It should be legal for a user to change maxParallelUpdates while builds
// are in progress (e.g. if there are 5 builds in progress and user sets
// maxParallelUpdates=3, nothing should explode.)
func TestCurrentlyBuildingMayExceedMaxParallelUpdates(t *testing.T) {
	f := newTestFixture(t)
	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(3)

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	manC := f.newDockerBuildManifestWithBuildPath("manC", f.JoinPath("c"))
	f.Start([]model.Manifest{manA, manB, manC})
	f.completeAndCheckBuildsForManifests(manA, manB, manC)

	// start builds for all manifests
	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")
	f.editFileAndWaitForManifestBuilding("manB", "b/main.go")
	f.editFileAndWaitForManifestBuilding("manC", "c/main.go")
	f.waitUntilNumBuildSlots(0)

	// decrease maxParallelUpdates (now less than the number of current builds, but this is okay)
	f.setMaxParallelUpdates(2)
	f.waitUntilNumBuildSlots(0)

	// another file change for manB -- will try to start another build as soon as possible
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("b/other.go"))

	f.completeBuildForManifest(manB)
	call := f.nextCall("expect manB build complete")
	f.assertCallIsForManifestAndFiles(call, manB, "b/main.go")

	// we should NOT see another build for manB, even though it has a pending file change,
	// b/c we don't have enough slots (since we decreased maxParallelUpdates)
	f.waitUntilNumBuildSlots(0)
	f.waitUntilManifestNotBuilding("manB")

	// complete another build...
	f.completeBuildForManifest(manA)
	call = f.nextCall("expect manA build complete")
	f.assertCallIsForManifestAndFiles(call, manA, "a/main.go")

	// ...now that we have an available slots again, manB will rebuild
	f.waitUntilManifestBuilding("manB")

	f.completeBuildForManifest(manB)
	call = f.nextCall("expect manB build complete (second build)")
	f.assertCallIsForManifestAndFiles(call, manB, "b/other.go")

	f.completeBuildForManifest(manC)
	call = f.nextCall("expect manC build complete")
	f.assertCallIsForManifestAndFiles(call, manC, "c/main.go")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestDontStartBuildIfControllerAndEngineUnsynced(t *testing.T) {
	f := newTestFixture(t)

	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(3)

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	f.Start([]model.Manifest{manA, manB})
	f.completeAndCheckBuildsForManifests(manA, manB)

	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")

	// deliberately de-sync engine state and build controller
	st := f.store.LockMutableStateForTesting()
	st.BuildControllerStartCount--
	f.store.UnlockMutableState()

	// this build won't start while state and build controller are out of sync
	f.editFileAndAssertManifestNotBuilding("manB", "b/main.go")

	// resync the two counts...
	st = f.store.LockMutableStateForTesting()
	st.BuildControllerStartCount++
	f.store.UnlockMutableState()

	// ...and manB build will start as expected
	f.waitUntilManifestBuilding("manB")

	// complete all builds (can't guarantee order)
	f.completeAndCheckBuildsForManifests(manA, manB)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestErrorHandlingWithMultipleBuilds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): fix this")
	}
	f := newTestFixture(t)
	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(2)

	errA := fmt.Errorf("errA")
	errB := fmt.Errorf("errB")

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	manC := f.newDockerBuildManifestWithBuildPath("manC", f.JoinPath("c"))
	f.Start([]model.Manifest{manA, manB, manC})
	f.completeAndCheckBuildsForManifests(manA, manB, manC)

	// start builds for all manifests (we only have 2 build slots)
	f.SetNextBuildError(errA)
	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")
	f.SetNextBuildError(errB)
	f.editFileAndWaitForManifestBuilding("manB", "b/main.go")
	f.editFileAndAssertManifestNotBuilding("manC", "c/main.go")

	// Complete one build...
	f.completeBuildForManifest(manA)
	call := f.nextCall("expect manA build complete")
	f.assertCallIsForManifestAndFiles(call, manA, "a/main.go")
	f.WaitUntilManifestState("last manA build reflects expected error", "manA", func(ms store.ManifestState) bool {
		return ms.LastBuild().Error == errA
	})

	// ...'manC' should start building, even though the manA build ended with an error
	f.waitUntilManifestBuilding("manC")

	// complete the rest
	f.completeAndCheckBuildsForManifests(manB, manC)
	f.WaitUntilManifestState("last manB build reflects expected error", "manB", func(ms store.ManifestState) bool {
		return ms.LastBuild().Error == errB
	})
	f.WaitUntilManifestState("last manC build recorded and has no error", "manC", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) == 2 && ms.LastBuild().Error == nil
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestManifestsWithSameTwoImages(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithSameTwoImages(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	aPath := f.JoinPath("common", "a.txt")
	f.fsWatcher.Events <- watch.NewFileEvent(aPath)

	f.waitForCompletedBuildCount(4)

	// Make sure that both builds are triggered, and that they
	// are triggered in a particular order.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	state := call.state[m1.ImageTargets[0].ID()]
	assert.Equal(t, map[string]bool{aPath: true}, state.FilesChangedSet)

	// Make sure that when the second build is triggered, we did the bookkeeping
	// correctly around marking the first and second image built and only deploying
	// the k8s resources.
	call = f.nextCall("m2 build2")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	id := m2.ImageTargets[0].ID()
	result := f.b.resultsByID[id]
	assert.Equal(t, result, call.state[id].LastResult)
	assert.Equal(t, 0, len(call.state[id].FilesChangedSet))

	id = m2.ImageTargets[1].ID()
	result = f.b.resultsByID[id]
	assert.Equal(t, result, call.state[id].LastResult)
	assert.Equal(t, 0, len(call.state[id].FilesChangedSet))

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestManifestsWithTwoCommonAncestors(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithTwoCommonAncestors(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	aPath := f.JoinPath("base", "a.txt")
	f.fsWatcher.Events <- watch.NewFileEvent(aPath)

	f.waitForCompletedBuildCount(4)

	// Make sure that both builds are triggered, and that they
	// are triggered in a particular order.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	state := call.state[m1.ImageTargets[0].ID()]
	assert.Equal(t, map[string]bool{aPath: true}, state.FilesChangedSet)

	// Make sure that when the second build is triggered, we did the bookkeeping
	// correctly around marking the first and second image built, and only
	// rebuilding the third image and k8s deploy.
	call = f.nextCall("m2 build2")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	id := m2.ImageTargets[0].ID()
	result := f.b.resultsByID[id]
	assert.Equal(t, result, call.state[id].LastResult)
	assert.Equal(t, 0, len(call.state[id].FilesChangedSet))

	id = m2.ImageTargets[1].ID()
	result = f.b.resultsByID[id]
	assert.Equal(t, result, call.state[id].LastResult)
	assert.Equal(t, 0, len(call.state[id].FilesChangedSet))

	id = m2.ImageTargets[2].ID()
	result = f.b.resultsByID[id]

	// Assert the 3rd image was not reused from the previous build.
	assert.NotEqual(t, result, call.state[id].LastResult)
	assert.Equal(t,
		map[model.TargetID]bool{m2.ImageTargets[1].ID(): true},
		call.state[id].DepsChangedSet)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestLocalDependsOnNonWorkloadK8s(t *testing.T) {
	f := newTestFixture(t)

	local1 := manifestbuilder.New(f, "local").
		WithLocalResource("exec-local", nil).
		WithResourceDeps("k8s1").
		Build()
	k8s1 := manifestbuilder.New(f, "k8s1").
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodReadiness(model.PodReadinessIgnore).
		Build()
	f.Start([]model.Manifest{local1, k8s1})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("k8s1 build")
	assert.Equal(t, k8s1.K8sTarget(), call.k8s())

	call = f.nextCall("local build")
	assert.Equal(t, local1.LocalTarget(), call.local())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestManifestsWithCommonAncestorAndTrigger(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithCommonAncestor(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	f.store.Dispatch(store.AppendToTriggerQueueAction{Name: m1.Name})
	f.waitForCompletedBuildCount(3)

	// Make sure that only one build was triggered.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	f.assertNoCall("m2 should not be rebuilt")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestDisablingCancelsBuild(t *testing.T) {
	f := newTestFixture(t)
	manifest := manifestbuilder.New(f, "local").
		WithLocalResource("sleep 10000", nil).
		Build()
	f.b.completeBuildsManually = true

	f.Start([]model.Manifest{manifest})
	f.waitUntilManifestBuilding("local")

	ds := manifest.DeployTarget.(model.LocalTarget).ServeCmdDisableSource
	err := configmap.UpsertDisableConfigMap(f.ctx, f.ctrlClient, ds.ConfigMap.Name, ds.ConfigMap.Key, true)
	require.NoError(t, err)

	f.waitForCompletedBuildCount(1)

	f.withManifestState("local", func(ms store.ManifestState) {
		require.EqualError(t, ms.LastBuild().Error, "build canceled")
	})

	err = f.Stop()
	require.NoError(t, err)
}

func TestCancelButton(t *testing.T) {
	f := newTestFixture(t)
	f.b.completeBuildsManually = true
	f.useRealTiltfileLoader()
	f.WriteFile("Tiltfile", `
local_resource('local', 'sleep 10000')
`)
	f.loadAndStart()
	f.waitUntilManifestBuilding("local")

	var cancelButton v1alpha1.UIButton
	err := f.ctrlClient.Get(f.ctx, types.NamespacedName{Name: uibutton.StopBuildButtonName("local")}, &cancelButton)
	require.NoError(t, err)
	cancelButton.Status.LastClickedAt = metav1.NowMicro()
	err = f.ctrlClient.Status().Update(f.ctx, &cancelButton)
	require.NoError(t, err)

	f.waitForCompletedBuildCount(1)

	f.withManifestState("local", func(ms store.ManifestState) {
		require.EqualError(t, ms.LastBuild().Error, "build canceled")
	})

	err = f.Stop()
	require.NoError(t, err)
}

func TestCancelButtonClickedBeforeBuild(t *testing.T) {
	f := newTestFixture(t)
	f.b.completeBuildsManually = true
	f.useRealTiltfileLoader()
	f.WriteFile("Tiltfile", `
local_resource('local', 'sleep 10000')
`)
	// grab a timestamp now to represent clicking the button before the build started
	ts := metav1.NowMicro()

	f.loadAndStart()
	f.waitUntilManifestBuilding("local")

	var cancelButton v1alpha1.UIButton
	err := f.ctrlClient.Get(f.ctx, types.NamespacedName{Name: uibutton.StopBuildButtonName("local")}, &cancelButton)
	require.NoError(t, err)
	cancelButton.Status.LastClickedAt = ts
	err = f.ctrlClient.Status().Update(f.ctx, &cancelButton)
	require.NoError(t, err)

	// give the build controller a little time to process the button click
	require.Never(t, func() bool {
		state := f.store.RLockState()
		defer f.store.RUnlockState()
		return state.CompletedBuildCount > 0
	}, 20*time.Millisecond, 2*time.Millisecond, "build finished on its own even though manual build completion is enabled")

	f.b.completeBuild("local:local")

	f.waitForCompletedBuildCount(1)

	f.withManifestState("local", func(ms store.ManifestState) {
		require.NoError(t, ms.LastBuild().Error)
	})

	err = f.Stop()
	require.NoError(t, err)
}

func TestBuildControllerK8sFileDependencies(t *testing.T) {
	f := newTestFixture(t)

	kt := k8s.MustTarget("fe", testyaml.SanchoYAML).
		WithPathDependencies([]string{f.JoinPath("k8s-dep")}).
		WithIgnores([]v1alpha1.IgnoreDef{
			{BasePath: f.JoinPath("k8s-dep", ".git")},
			{
				BasePath: f.JoinPath("k8s-dep"),
				Patterns: []string{"ignore-me"},
			},
		})
	m := model.Manifest{Name: "fe"}.WithDeployTarget(kt)

	f.Start([]model.Manifest{m})

	call := f.nextCall()
	assert.Empty(t, call.k8sState().FilesChanged())

	// path dependency is on ./k8s-dep/** with a local repo of ./k8s-dep/.git/** (ignored)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("k8s-dep", "ignore-me"))
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("k8s-dep", ".git", "file"))
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("k8s-dep", "file"))

	call = f.nextCall()
	assert.Equal(t, []string{f.JoinPath("k8s-dep", "file")}, call.k8sState().FilesChanged())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func (f *testFixture) waitUntilManifestBuilding(name model.ManifestName) {
	f.t.Helper()
	msg := fmt.Sprintf("manifest %q is building", name)
	f.WaitUntilManifestState(msg, name, func(ms store.ManifestState) bool {
		return ms.IsBuilding()
	})

	f.withState(func(st store.EngineState) {
		ok := st.CurrentBuildSet[name]
		require.True(f.t, ok, "expected EngineState to reflect that %q is currently building", name)
	})
}

func (f *testFixture) waitUntilManifestNotBuilding(name model.ManifestName) {
	msg := fmt.Sprintf("manifest %q is NOT building", name)
	f.WaitUntilManifestState(msg, name, func(ms store.ManifestState) bool {
		return !ms.IsBuilding()
	})

	f.withState(func(st store.EngineState) {
		ok := st.CurrentBuildSet[name]
		require.False(f.t, ok, "expected EngineState to reflect that %q is NOT currently building", name)
	})
}

func (f *testFixture) waitUntilNumBuildSlots(expected int) {
	msg := fmt.Sprintf("%d build slots available", expected)
	f.WaitUntil(msg, func(st store.EngineState) bool {
		return expected == st.AvailableBuildSlots()
	})
}

func (f *testFixture) editFileAndWaitForManifestBuilding(name model.ManifestName, path string) {
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(path))
	f.waitUntilManifestBuilding(name)
}

func (f *testFixture) editFileAndAssertManifestNotBuilding(name model.ManifestName, path string) {
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(path))
	f.waitUntilManifestNotBuilding(name)
}

func (f *testFixture) assertCallIsForManifestAndFiles(call buildAndDeployCall, m model.Manifest, files ...string) {
	assert.Equal(f.t, m.ImageTargetAt(0).ID(), call.firstImgTarg().ID())
	assert.Equal(f.t, f.JoinPaths(files), call.oneImageState().FilesChanged())
}

func (f *testFixture) completeAndCheckBuildsForManifests(manifests ...model.Manifest) {
	for _, m := range manifests {
		f.completeBuildForManifest(m)
	}

	expectedImageTargets := make([][]model.ImageTarget, len(manifests))
	var actualImageTargets [][]model.ImageTarget
	for i, m := range manifests {
		expectedImageTargets[i] = m.ImageTargets

		call := f.nextCall("timed out waiting for call %d/%d", i+1, len(manifests))
		actualImageTargets = append(actualImageTargets, call.imageTargets())
	}
	require.ElementsMatch(f.t, expectedImageTargets, actualImageTargets)

	for _, m := range manifests {
		f.waitUntilManifestNotBuilding(m.Name)
	}
}

func (f *testFixture) simpleManifestWithTriggerMode(name model.ManifestName, tm model.TriggerMode) model.Manifest {
	return manifestbuilder.New(f, name).WithTriggerMode(tm).
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithK8sYAML(SanchoYAML).Build()
}

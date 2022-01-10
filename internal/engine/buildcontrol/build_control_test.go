package buildcontrol

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestNextTargetToBuildDoesntReturnCurrentlyBuildingTarget(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mt := f.manifestNeedingCrashRebuild()
	f.st.UpsertManifestTarget(mt)

	// Verify this target is normally next-to-build
	f.assertNextTargetToBuild(mt.Manifest.Name)

	// If target is currently building, should NOT be next-to-build
	mt.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
}

func TestCurrentlyBuildingK8sResourceDisablesLocalScheduling(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	k8s1 := f.upsertK8sManifest("k8s1")
	k8s2 := f.upsertK8sManifest("k8s2")
	f.upsertLocalManifest("local1")

	f.assertNextTargetToBuild("local1")

	k8s1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s2")
	f.assertHold("local1", store.HoldReasonIsUnparallelizableTarget)

	k8s2.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
}

func TestCurrentlyBuildingK8sResourceDoesNotCreateHoldIfResourceNotPending(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	k8s1 := f.upsertK8sManifest("k8s1")
	k8s2 := f.upsertK8sManifest("k8s2")
	f.upsertLocalManifest("local1", func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithTriggerMode(model.TriggerModeManual)
	})

	f.assertHold("local1", store.HoldReasonNone)

	k8s1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s2")
	f.assertHold("local1", store.HoldReasonNone)

	k8s2.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
	f.assertHold("local1", store.HoldReasonNone)
}

func TestCurrentlyBuildingUncategorizedDisablesOtherK8sTargets(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	_ = f.upsertK8sManifest("k8s1")
	k8sUnresourced := f.upsertK8sManifest(model.UnresourcedYAMLManifestName)
	_ = f.upsertK8sManifest("k8s2")

	f.assertNextTargetToBuild(model.UnresourcedYAMLManifestName)
	k8sUnresourced.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
	for _, mn := range []model.ManifestName{"k8s1", "k8s2"} {
		f.assertHold(mn, store.HoldReasonWaitingForUncategorized, model.ManifestName("uncategorized").TargetID())
	}
}

func TestK8sDependsOnLocal(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	k8s1 := f.upsertK8sManifest("k8s1", withResourceDeps("local1"))
	k8s2 := f.upsertK8sManifest("k8s2")
	local1 := f.upsertLocalManifest("local1")

	f.assertNextTargetToBuild("local1")

	f.assertHold("k8s1", store.HoldReasonWaitingForDep, model.ManifestName("local1").TargetID())
	f.assertHold("k8s2", store.HoldReasonNone)

	local1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	lrs := local1.State.LocalRuntimeState()
	lrs.LastReadyOrSucceededTime = time.Now()
	local1.State.RuntimeState = lrs

	f.assertNextTargetToBuild("k8s1")
	k8s1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s2")

	_ = k8s2
}

func TestLocalDependsOnNonWorkloadK8s(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	local1 := f.upsertLocalManifest("local1", withResourceDeps("k8s1"))
	k8s1 := f.upsertK8sManifest("k8s1", withK8sPodReadiness(model.PodReadinessIgnore))
	k8s2 := f.upsertK8sManifest("k8s2", withK8sPodReadiness(model.PodReadinessIgnore))

	f.assertNextTargetToBuild("k8s1")
	f.assertHold("local1", store.HoldReasonWaitingForDep, model.ManifestName("k8s1").TargetID())
	f.assertHold("k8s1", store.HoldReasonNone)
	f.assertHold("k8s2", store.HoldReasonNone)

	k8s1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	k8s1.State.RuntimeState = store.K8sRuntimeState{
		PodReadinessMode:            model.PodReadinessIgnore,
		HasEverDeployedSuccessfully: true,
	}

	f.assertNextTargetToBuild("local1")
	local1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	f.assertNextTargetToBuild("k8s2")

	_ = k8s2
}

func TestK8sDependsOnCluster(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.st.Clusters["default"].Status.Error = "connection error"

	_ = f.upsertK8sManifest("k8s1")
	f.assertNoTargetNextToBuild()
	f.assertHoldOnRefs("k8s1", store.HoldReasonCluster, v1alpha1.UIResourceStateWaitingOnRef{
		Group:      "tilt.dev",
		APIVersion: "v1alpha1",
		Kind:       "Cluster",
		Name:       "default",
	})

	f.st.Clusters["default"].Status.Error = ""
	f.assertNextTargetToBuild("k8s1")
}

func TestCurrentlyBuildingLocalResourceDisablesK8sScheduling(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.upsertK8sManifest("k8s1")
	f.upsertK8sManifest("k8s2")
	local1 := f.upsertLocalManifest("local1")
	f.upsertLocalManifest("local2")

	f.assertNextTargetToBuild("local1")
	local1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
	for _, mn := range []model.ManifestName{"k8s1", "k8s2", "local2"} {
		f.assertHold(mn, store.HoldReasonWaitingForUnparallelizableTarget, model.ManifestName("local1").TargetID())
	}
}

func TestCurrentlyBuildingParallelLocalResource(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.upsertK8sManifest("k8s1")
	local1 := f.upsertLocalManifest("local1", func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithLocalAllowParallel(true)
	})
	local2 := f.upsertLocalManifest("local2", func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithLocalAllowParallel(true)
	})

	f.assertNextTargetToBuild("local1")

	local1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("local2")

	local2.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s1")
}

func TestTriggerIneligibleResource(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	// local1 has a build in progress
	local1 := f.upsertLocalManifest("local1", func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithLocalAllowParallel(true)
	})
	local1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}

	// local2 is not parallelizable
	local2 := f.upsertLocalManifest("local2")

	f.st.AppendToTriggerQueue(local1.Manifest.Name, model.BuildReasonFlagTriggerCLI)
	f.st.AppendToTriggerQueue(local2.Manifest.Name, model.BuildReasonFlagTriggerCLI)
	f.assertNoTargetNextToBuild()
}

func TestTwoK8sTargetsWithBaseImage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	baseImage := newDockerImageTarget("sancho-base")
	sanchoOneImage := newDockerImageTarget("sancho-one").
		WithImageMapDeps([]string{baseImage.ImageMapName()})
	sanchoTwoImage := newDockerImageTarget("sancho-two").
		WithImageMapDeps([]string{baseImage.ImageMapName()})

	sanchoOne := f.upsertManifest(manifestbuilder.New(f, "sancho-one").
		WithImageTargets(baseImage, sanchoOneImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())
	f.upsertManifest(manifestbuilder.New(f, "sancho-two").
		WithImageTargets(baseImage, sanchoTwoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())

	f.assertNextTargetToBuild("sancho-one")

	sanchoOne.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}

	f.assertNoTargetNextToBuild()
	f.assertHold("sancho-two", store.HoldReasonBuildingComponent, baseImage.ID())

	sanchoOne.State.CurrentBuild = model.BuildRecord{}
	sanchoOne.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})

	f.assertNextTargetToBuild("sancho-two")
}

func TestTwoK8sTargetsWithBaseImagePrebuilt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	baseImage := newDockerImageTarget("sancho-base")
	sanchoOneImage := newDockerImageTarget("sancho-one").
		WithImageMapDeps([]string{baseImage.ImageMapName()})
	sanchoTwoImage := newDockerImageTarget("sancho-two").
		WithImageMapDeps([]string{baseImage.ImageMapName()})

	sanchoOne := f.upsertManifest(manifestbuilder.New(f, "sancho-one").
		WithImageTargets(baseImage, sanchoOneImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())
	sanchoTwo := f.upsertManifest(manifestbuilder.New(f, "sancho-two").
		WithImageTargets(baseImage, sanchoTwoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())

	sanchoOne.State.MutableBuildStatus(baseImage.ID()).LastResult = store.ImageBuildResult{}
	sanchoTwo.State.MutableBuildStatus(baseImage.ID()).LastResult = store.ImageBuildResult{}

	f.assertNextTargetToBuild("sancho-one")

	sanchoOne.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}

	// Make sure sancho-two can start while sanchoOne is still pending.
	f.assertNextTargetToBuild("sancho-two")
}

func TestHoldForDeploy(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	srcFile := f.JoinPath("src", "a.txt")
	objFile := f.JoinPath("obj", "a.out")
	fallbackFile := f.JoinPath("src", "package.json")
	f.WriteFile(srcFile, "hello")
	f.WriteFile(objFile, "hello")
	f.WriteFile(fallbackFile, "hello")

	luSpec := v1alpha1.LiveUpdateSpec{
		BasePath:  f.Path(),
		StopPaths: []string{filepath.Join("src", "package.json")},
		Syncs:     []v1alpha1.LiveUpdateSync{{LocalPath: "src", ContainerPath: "/src"}},
	}
	sanchoImage := newDockerImageTarget("sancho").
		WithLiveUpdateSpec("sancho", luSpec).
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.Path()})
	sancho := f.upsertManifest(manifestbuilder.New(f, "sancho").
		WithImageTargets(sanchoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		WithLiveUpdateBAD().
		Build())

	f.assertNextTargetToBuild("sancho")

	sancho.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	f.assertNoTargetNextToBuild()

	status := sancho.State.MutableBuildStatus(sanchoImage.ID())

	status.PendingFileChanges[objFile] = time.Now()
	f.assertNextTargetToBuild("sancho")
	delete(status.PendingFileChanges, objFile)

	status.PendingFileChanges[fallbackFile] = time.Now()
	f.assertNextTargetToBuild("sancho")
	delete(status.PendingFileChanges, fallbackFile)

	status.PendingFileChanges[srcFile] = time.Now()
	f.assertNoTargetNextToBuild()
	f.assertHold("sancho", store.HoldReasonWaitingForDeploy)

	resource := &k8sconv.KubernetesResource{
		FilteredPods: []v1alpha1.Pod{},
	}
	f.st.KubernetesResources["sancho"] = resource

	resource.FilteredPods = append(resource.FilteredPods, *readyPod("pod-1", sanchoImage.Refs.ClusterRef()))
	f.assertNextTargetToBuild("sancho")

	resource.FilteredPods[0] = *crashingPod("pod-1", sanchoImage.Refs.ClusterRef())
	f.assertNextTargetToBuild("sancho")

	resource.FilteredPods[0] = *crashedInThePastPod("pod-1", sanchoImage.Refs.ClusterRef())
	f.assertNextTargetToBuild("sancho")

	resource.FilteredPods[0] = *sidecarCrashedPod("pod-1", sanchoImage.Refs.ClusterRef())
	f.assertNextTargetToBuild("sancho")

	resource.FilteredPods[0] = *completedPod("pod-1", sanchoImage.Refs.ClusterRef())
	f.assertNextTargetToBuild("sancho")
}

func TestHoldDisabled(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.upsertLocalManifest("local")
	f.st.UIResources = map[string]*v1alpha1.UIResource{
		"local": {
			Status: v1alpha1.UIResourceStatus{
				DisableStatus: v1alpha1.DisableResourceStatus{
					DisabledCount: 5,
				},
			},
		},
	}
	f.assertHold("local", store.HoldReasonDisabled)
	f.assertNoTargetNextToBuild()
}

func readyPod(podID k8s.PodID, ref reference.Named) *v1alpha1.Pod {
	return &v1alpha1.Pod{
		Name:   podID.String(),
		Phase:  string(v1.PodRunning),
		Status: "Running",
		Containers: []v1alpha1.Container{
			{
				ID:    string(podID + "-container"),
				Name:  "c",
				Ready: true,
				Image: ref.String(),
				State: v1alpha1.ContainerState{
					Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
			},
		},
	}
}

func crashingPod(podID k8s.PodID, ref reference.Named) *v1alpha1.Pod {
	return &v1alpha1.Pod{
		Name:   podID.String(),
		Phase:  string(v1.PodRunning),
		Status: "CrashLoopBackOff",
		Containers: []v1alpha1.Container{
			{
				ID:       string(podID + "-container"),
				Name:     "c",
				Ready:    false,
				Image:    ref.String(),
				Restarts: 1,
				State: v1alpha1.ContainerState{
					Terminated: &v1alpha1.ContainerStateTerminated{
						StartedAt:  metav1.Now(),
						FinishedAt: metav1.Now(),
						Reason:     "Error",
						ExitCode:   127,
					}},
			},
		},
	}
}

func crashedInThePastPod(podID k8s.PodID, ref reference.Named) *v1alpha1.Pod {
	return &v1alpha1.Pod{
		Name:   podID.String(),
		Phase:  string(v1.PodRunning),
		Status: "Ready",
		Containers: []v1alpha1.Container{
			{
				ID:       string(podID + "-container"),
				Name:     "c",
				Ready:    true,
				Image:    ref.String(),
				Restarts: 1,
				State: v1alpha1.ContainerState{
					Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
			},
		},
	}
}

func sidecarCrashedPod(podID k8s.PodID, ref reference.Named) *v1alpha1.Pod {
	return &v1alpha1.Pod{
		Name:   podID.String(),
		Phase:  string(v1.PodRunning),
		Status: "Ready",
		Containers: []v1alpha1.Container{
			{
				ID:       string(podID + "-container"),
				Name:     "c",
				Ready:    true,
				Image:    ref.String(),
				Restarts: 0,
				State: v1alpha1.ContainerState{
					Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
			},
			{
				ID:       string(podID + "-sidecar"),
				Name:     "s",
				Ready:    false,
				Image:    container.MustParseNamed("sidecar").String(),
				Restarts: 1,
				State: v1alpha1.ContainerState{
					Terminated: &v1alpha1.ContainerStateTerminated{
						StartedAt:  metav1.Now(),
						FinishedAt: metav1.Now(),
						Reason:     "Error",
						ExitCode:   127,
					}},
			},
		},
	}
}

func completedPod(podID k8s.PodID, ref reference.Named) *v1alpha1.Pod {
	return &v1alpha1.Pod{
		Name:   podID.String(),
		Phase:  string(v1.PodSucceeded),
		Status: "Completed",
		Containers: []v1alpha1.Container{
			{
				ID:       string(podID + "-container"),
				Name:     "c",
				Ready:    false,
				Image:    ref.String(),
				Restarts: 0,
				State: v1alpha1.ContainerState{
					Terminated: &v1alpha1.ContainerStateTerminated{
						StartedAt:  metav1.Now(),
						FinishedAt: metav1.Now(),
						Reason:     "Succcess!",
						ExitCode:   0,
					}},
			},
		},
	}
}

type testFixture struct {
	*tempdir.TempDirFixture
	t  *testing.T
	st *store.EngineState
}

func newTestFixture(t *testing.T) testFixture {
	f := tempdir.NewTempDirFixture(t)
	st := store.NewState()
	st.Clusters["default"] = &v1alpha1.Cluster{
		Status: v1alpha1.ClusterStatus{
			Arch: "amd64",
		},
	}
	return testFixture{
		TempDirFixture: f,
		t:              t,
		st:             st,
	}
}

func (f *testFixture) assertHold(m model.ManifestName, reason store.HoldReason, holdOn ...model.TargetID) {
	f.T().Helper()
	_, hs := NextTargetToBuild(*f.st)
	hold := store.Hold{
		Reason: reason,
		HoldOn: holdOn,
	}
	assert.Equal(f.t, hold, hs[m])
}

func (f *testFixture) assertHoldOnRefs(m model.ManifestName, reason store.HoldReason, onRefs ...v1alpha1.UIResourceStateWaitingOnRef) {
	f.T().Helper()
	_, hs := NextTargetToBuild(*f.st)
	hold := store.Hold{
		Reason: reason,
		OnRefs: onRefs,
	}
	assert.Equal(f.t, hold, hs[m])
}

func (f *testFixture) assertNextTargetToBuild(expected model.ManifestName) {
	f.T().Helper()
	next, _ := NextTargetToBuild(*f.st)
	require.NotNil(f.t, next, "expected next target %s but got: nil", expected)
	actual := next.Manifest.Name
	assert.Equal(f.t, expected, actual, "expected next target to be %s but got %s", expected, actual)
}

func (f *testFixture) assertNoTargetNextToBuild() {
	f.T().Helper()
	next, _ := NextTargetToBuild(*f.st)
	if next != nil {
		f.t.Fatalf("expected no next target to build, but got %s", next.Manifest.Name)
	}
}

func (f *testFixture) upsertManifest(m model.Manifest) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	f.st.UpsertManifestTarget(mt)
	return mt
}

func (f *testFixture) upsertK8sManifest(name model.ManifestName, opts ...manifestOption) *store.ManifestTarget {
	b := manifestbuilder.New(f, name)
	for _, o := range opts {
		b = o(b)
	}
	return f.upsertManifest(b.WithK8sYAML(testyaml.SanchoYAML).Build())
}

func (f *testFixture) upsertLocalManifest(name model.ManifestName, opts ...manifestOption) *store.ManifestTarget {
	b := manifestbuilder.New(f, name)
	for _, o := range opts {
		b = o(b)
	}
	return f.upsertManifest(b.WithLocalResource(fmt.Sprintf("exec-%s", name), nil).Build())
}

func (f *testFixture) manifestNeedingCrashRebuild() *store.ManifestTarget {
	m := manifestbuilder.New(f, "needs-crash-rebuild").
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	mt := store.NewManifestTarget(m)
	mt.State.BuildHistory = []model.BuildRecord{
		model.BuildRecord{
			StartTime:  time.Now().Add(-5 * time.Second),
			FinishTime: time.Now(),
		},
	}
	mt.State.NeedsRebuildFromCrash = true
	return mt
}

type manifestOption func(manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder

func withResourceDeps(deps ...string) manifestOption {
	return manifestOption(func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithResourceDeps(deps...)
	})
}
func withK8sPodReadiness(pr model.PodReadinessMode) manifestOption {
	return manifestOption(func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithK8sPodReadiness(pr)
	})
}

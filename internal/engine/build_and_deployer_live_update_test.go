package engine

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/util/exec"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"

	"github.com/tilt-dev/tilt/internal/docker"

	"github.com/tilt-dev/tilt/internal/store"

	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var userFailureErrDocker = docker.ExitError{ExitCode: 123}
var userFailureErrExec = exec.CodeExitError{
	Err:  fmt.Errorf("welcome to zombocom"),
	Code: 123,
}

// intRange is used for cases where results are non-deterministic
type intRange struct {
	min int
	max int
}

func (ir *intRange) AssertIn(t assert.TestingT, i int, msgAndArgs ...interface{}) {
	assert.GreaterOrEqual(t, i, ir.min, msgAndArgs...)
	assert.LessOrEqual(t, i, ir.max, msgAndArgs...)
}

type testCase struct {
	manifest model.Manifest

	changedFiles              []string                          // leave empty for from-scratch build
	runningContainersByTarget map[model.TargetID][]container.ID // if empty, use default container

	// expect that these targets (and only these targets) will be updated (i.e. will have build
	// results, will have acted upon the associated containers). Must be used in conjunction with
	// runningContainersByTarget. If blank, we check that all targets in runningContainersByTarget
	// have been updated; if THAT is blank, we check that the last image target on the manifest
	// has been updated
	expectUpdatedTargets []model.TargetID

	// Expect the BuildAndDeploy call to fail with an error containing this string
	expectErrorContains string

	// Docker actions
	expectDockerBuildRange   intRange
	expectDockerPushRange    intRange
	expectDockerCopyRange    intRange
	expectDockerExecRange    intRange
	expectDockerRestartCount int

	// k8s/deploy actions
	expectK8sExecCount int
	expectK8sDeploy    bool

	// logs checks
	logsContain     []string
	logsDontContain []string
}

func runTestCase(t *testing.T, f *bdFixture, tCase testCase) {
	if len(tCase.changedFiles) == 0 && len(tCase.runningContainersByTarget) != 0 {
		t.Fatal("can't specify both empty changedFiles (implies from-scratch " +
			"build) and non-empty running containers (implies there's an existing build " +
			"to LiveUpdate on top of).")
	}

	if len(tCase.expectUpdatedTargets) > 0 && len(tCase.runningContainersByTarget) == 0 {
		t.Fatal("can only specify expectUpdatedTargets with runningContainersByTarget" +
			"(the former only makes sense in a multi-target scenario)")
	}

	manifest := tCase.manifest
	bs := f.createBuildStateSet(manifest, tCase.changedFiles)

	// Assume that the last image target is the deployed one.
	iTargIdx := len(manifest.ImageTargets) - 1
	iTarg := manifest.ImageTargetAt(iTargIdx)
	require.True(t, manifest.IsImageDeployed(iTarg))

	for targID, cIDs := range tCase.runningContainersByTarget {
		containers := make([]liveupdates.Container, len(cIDs))
		for i, id := range cIDs {
			containers[i] = liveupdates.Container{
				PodID:         testPodID,
				ContainerID:   id,
				ContainerName: container.Name(fmt.Sprintf("container %s", id)),
			}
		}
		imageName := string(targID.Name)
		bs[targID] = liveupdates.WithFakeK8sContainers(bs[targID], imageName, containers)
	}

	targets := buildcontrol.BuildTargets(manifest)

	result, err := f.BuildAndDeploy(targets, bs)
	if tCase.expectErrorContains != "" {
		require.NotNil(t, err, "expected error containing '%s' but got no error", tCase.expectErrorContains)
		require.Contains(t, err.Error(), tCase.expectErrorContains,
			"expected BuildAndDeploy error to contain string")
	} else if err != nil {
		t.Fatal(err)
	}

	tCase.expectDockerBuildRange.AssertIn(t, f.docker.BuildCount, f.docker.BuildCount, "docker build")
	tCase.expectDockerPushRange.AssertIn(t, f.docker.PushCount, f.docker.PushCount, "docker push")
	tCase.expectDockerCopyRange.AssertIn(t, f.docker.CopyCount, f.docker.CopyCount, "docker copy")
	tCase.expectDockerExecRange.AssertIn(t, len(f.docker.ExecCalls), len(f.docker.ExecCalls), "docker exec")
	if len(tCase.runningContainersByTarget) > 0 {
		f.assertTotalContainerRestarts(tCase.expectDockerRestartCount)
	} else {
		f.assertContainerRestarts(tCase.expectDockerRestartCount)
	}

	assert.Equal(t, tCase.expectK8sExecCount, len(f.k8s.ExecCalls), "# k8s exec calls")

	logsStr := f.logs.String()
	if len(tCase.logsContain) > 0 {
		for _, s := range tCase.logsContain {
			assert.Contains(t, logsStr, s, "checking that logs contain expected string")
		}
	}
	if len(tCase.logsDontContain) > 0 {
		for _, s := range tCase.logsDontContain {
			assert.NotContains(t, logsStr, s, "checking that logs do NOT contain string")
		}
	}

	if tCase.expectErrorContains != "" {
		return
	}

	if tCase.expectK8sDeploy {
		expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
		if !strings.Contains(f.k8s.Yaml, expectedYaml) {
			t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
		}
		return
	} else {
		require.Empty(t, f.k8s.Yaml, "expected no k8s deploy, but we deployed YAML: %s", f.k8s.Yaml)
	}

	expectUpdatedTargs := make(map[model.TargetID]bool)
	if len(tCase.expectUpdatedTargets) > 0 {
		expectUpdatedTargs = model.TargetIDSet(tCase.expectUpdatedTargets)
	} else if len(tCase.runningContainersByTarget) > 0 {
		for targID := range tCase.runningContainersByTarget {
			expectUpdatedTargs[targID] = true
		}
	} else {
		// if no other info provided, assume that the last image target is the deployed one
		expectUpdatedTargs[manifest.ImageTargetAt(iTargIdx).ID()] = true
	}

	for tid, res := range result {
		if !expectUpdatedTargs[tid] && res != nil {
			t.Fatalf("got non empty result for target %s when didn't expect one. Result: %v", tid, res)
		}
		// mark this target as seen
		delete(expectUpdatedTargs, tid)

		if len(tCase.runningContainersByTarget) > 0 {
			// We expect to have operated on the containers that the test specified
			lRes := res.(store.LiveUpdateBuildResult)
			assert.ElementsMatch(t, lRes.LiveUpdatedContainerIDs, tCase.runningContainersByTarget[tid])
		} else {
			// We set up the test with RunningContainer = DefaultContainer; expect to have operated on that.
			assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
		}
	}
	require.Empty(t, expectUpdatedTargs, "didn't find results for these expected targets")
}

func TestLiveUpdateDockerBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	tCase := testCase{
		manifest:                 m,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 0, max: 0},
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 1, max: 1},
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildLocalContainerSameImgMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest:                  m,
		runningContainersByTarget: map[model.TargetID][]container.ID{m.ImageTargetAt(0).ID(): cIDs},
		changedFiles:              []string{"a.txt"},
		expectDockerBuildRange:    intRange{min: 0, max: 0},
		expectDockerPushRange:     intRange{min: 0, max: 0},

		// one of each operation per container
		expectDockerCopyRange:    intRange{min: 3, max: 3},
		expectDockerExecRange:    intRange{min: 3, max: 3},
		expectDockerRestartCount: 3,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildExecSameImgMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeCrio)

	iTarg := NewSanchoDockerBuildImageTarget(f)
	lu := assembleLiveUpdate(SanchoSyncSteps(f), nil, false, []string{"i/match/nothing"}, f)
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(iTarg).
			WithLiveUpdate(lu).
			Build(),
		runningContainersByTarget: map[model.TargetID][]container.ID{iTarg.ID(): cIDs},
		changedFiles:              []string{"a.txt"},
		expectDockerBuildRange:    intRange{min: 0, max: 0},
		expectDockerPushRange:     intRange{min: 0, max: 0},

		// 1 per container (tar archive) x 3 containers
		expectK8sExecCount: 3,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildLocalContainerDiffImgMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	sanchoTarg := NewSanchoLiveUpdateImageTarget(f)
	sidecarTarg := NewSanchoSidecarLiveUpdateImageTarget(f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sanchoWithSidecar").
			WithK8sYAML(testyaml.SanchoSidecarYAML).
			WithLiveUpdateBAD().
			WithImageTargets(sanchoTarg, sidecarTarg).
			Build(),
		runningContainersByTarget: map[model.TargetID][]container.ID{
			sanchoTarg.ID():  []container.ID{"c1"},
			sidecarTarg.ID(): []container.ID{"c2"},
		},
		changedFiles:           []string{"a.txt"},
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectDockerPushRange:  intRange{min: 0, max: 0},

		// one of each operation per container
		expectDockerCopyRange:    intRange{min: 2, max: 2},
		expectDockerExecRange:    intRange{min: 2, max: 2},
		expectDockerRestartCount: 2,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildExecDiffImgMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeCrio)

	sanchoLU := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	sidecarLU := assembleLiveUpdate(SyncStepsForApp("sidecar", f), RunStepsForApp("sidecar"),
		false, nil, f)
	sanchoTarg := NewSanchoDockerBuildImageTarget(f)
	sidecarTarg := NewSanchoSidecarDockerBuildImageTarget(f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sanchoWithSidecar").
			WithK8sYAML(testyaml.SanchoSidecarYAML).
			WithLiveUpdateBAD().
			WithImageTargets(sanchoTarg, sidecarTarg).
			WithLiveUpdateAtIndex(sanchoLU, 0).
			WithLiveUpdateAtIndex(sidecarLU, 1).
			Build(),
		runningContainersByTarget: map[model.TargetID][]container.ID{
			sanchoTarg.ID():  []container.ID{"c1"},
			sidecarTarg.ID(): []container.ID{"c2"},
		},
		changedFiles:           []string{"a.txt"},
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectDockerPushRange:  intRange{min: 0, max: 0},

		// two (tar archive + run step) per container
		expectK8sExecCount: 4,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDiffImgMultipleContainersOnlySomeSyncsMatch(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeCrio)

	sanchoPath := f.JoinPath("sancho")
	sanchoSyncs := SanchoSyncSteps(f)
	sanchoSyncs[0].LocalPath = sanchoPath

	sidecarPath := f.JoinPath("sidecar")
	sidecarSyncs := SyncStepsForApp("sidecar", f)
	sidecarSyncs[0].LocalPath = sidecarPath

	sanchoLU := assembleLiveUpdate(sanchoSyncs, SanchoRunSteps, false, nil, f)
	sidecarLU := assembleLiveUpdate(sidecarSyncs, RunStepsForApp("sidecar"),
		false, nil, f)
	sanchoTarg := model.MustNewImageTarget(SanchoRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: SanchoDockerfile,
			Context:            sanchoPath,
		})
	sidecarTarg := model.MustNewImageTarget(SanchoSidecarRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: SanchoDockerfile,
			Context:            sidecarPath,
		})

	tCase := testCase{
		manifest: manifestbuilder.New(f, "sanchoWithSidecar").
			WithK8sYAML(testyaml.SanchoSidecarYAML).
			WithLiveUpdateBAD().
			WithImageTargets(sanchoTarg, sidecarTarg).
			WithLiveUpdateAtIndex(sanchoLU, 0).
			WithLiveUpdateAtIndex(sidecarLU, 1).
			Build(),
		runningContainersByTarget: map[model.TargetID][]container.ID{
			sanchoTarg.ID():  []container.ID{"c1"},
			sidecarTarg.ID(): []container.ID{"c2"},
		},
		changedFiles:           []string{"sidecar/a.txt"},          // nothing matching Sancho's syncs
		expectUpdatedTargets:   []model.TargetID{sidecarTarg.ID()}, // we should only update the Sidecar target
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectDockerPushRange:  intRange{min: 0, max: 0},

		// two (tar archive + run step) per container
		// only the sidecar should be updated, so expect 2 calls
		expectK8sExecCount: 2,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDiffImgMultipleContainersSameContextOnlyOneLiveUpdate(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeCrio)

	buildContext := f.Path()
	sanchoSyncs := SanchoSyncSteps(f)
	sanchoSyncs[0].LocalPath = buildContext

	sanchoLU := assembleLiveUpdate(sanchoSyncs, SanchoRunSteps, false, nil, f)
	sanchoTarg := model.MustNewImageTarget(SanchoRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: SanchoDockerfile,
			Context:            buildContext,
		})
	sidecarTarg := model.MustNewImageTarget(SanchoSidecarRef).
		WithDockerImage(v1alpha1.DockerImageSpec{
			DockerfileContents: SanchoDockerfile,
			Context:            buildContext,
		})

	tCase := testCase{
		manifest: manifestbuilder.New(f, "sanchoWithSidecar").
			WithK8sYAML(testyaml.SanchoSidecarYAML).
			WithLiveUpdateBAD().
			WithImageTargets(sanchoTarg, sidecarTarg).
			WithLiveUpdateAtIndex(sanchoLU, 0).
			Build(),
		runningContainersByTarget: map[model.TargetID][]container.ID{
			sanchoTarg.ID():  []container.ID{"c1"},
			sidecarTarg.ID(): []container.ID{"c2"},
		},
		changedFiles:           []string{"sancho/a.txt"},
		expectDockerBuildRange: intRange{min: 2, max: 2},
		expectDockerPushRange:  intRange{min: 2, max: 2},
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDiffImgMultipleContainersFallBackIfFilesDoesntMatchAnySyncs(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeCrio)

	sanchoSyncs := SanchoSyncSteps(f)
	sanchoSyncs[0].LocalPath = f.JoinPath("sancho")
	sidecarSyncs := SyncStepsForApp("sidecar", f)
	sidecarSyncs[0].LocalPath = f.JoinPath("sidecar")

	sanchoLU := assembleLiveUpdate(sanchoSyncs, SanchoRunSteps, false, nil, f)
	sidecarLU := assembleLiveUpdate(sidecarSyncs, RunStepsForApp("sidecar"),
		false, nil, f)
	sanchoTarg := NewSanchoDockerBuildImageTarget(f)
	sidecarTarg := NewSanchoSidecarDockerBuildImageTarget(f)

	tCase := testCase{
		manifest: manifestbuilder.New(f, "sanchoWithSidecar").
			WithK8sYAML(testyaml.SanchoSidecarYAML).
			WithLiveUpdateBAD().
			WithImageTargets(sanchoTarg, sidecarTarg).
			WithLiveUpdateAtIndex(sanchoLU, 0).
			WithLiveUpdateAtIndex(sidecarLU, 1).
			Build(),
		runningContainersByTarget: map[model.TargetID][]container.ID{
			sanchoTarg.ID():  []container.ID{"c1"},
			sidecarTarg.ID(): []container.ID{"c2"},
		},
		changedFiles: []string{"sidecar/matches_sync.txt", "./doesnt_match.txt"},

		// expect to fall back to image build b/c one file matches NO syncs
		expectDockerBuildRange: intRange{min: 2, max: 2},
		expectDockerPushRange:  intRange{min: 2, max: 2},
		expectK8sDeploy:        true,

		// no container update, so expect no k8s exec calls
		expectK8sExecCount: 0,

		// fallback message for 1+ files not matching a sync
		logsContain: []string{"Found file(s) not matching any sync",
			"doesnt_match.txt"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerContainerUserRunFailureDoesntFallBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	f.docker.SetExecError(userFailureErrDocker)
	tCase := testCase{
		manifest:     m,
		changedFiles: []string{"a.txt"},

		// BuildAndDeploy call will ultimately fail with this error,
		// b/c we DON'T fall back to an image build
		expectErrorContains: "failed with exit code: 123",

		// called copy and exec before hitting error
		// (so, did not restart)
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 1, max: 1},
		expectDockerRestartCount: 0,

		// DO NOT fall back to image build
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectK8sDeploy:        false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateExecUserRunFailureDoesntFallBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeCrio)

	f.k8s.ExecErrors = []error{nil, userFailureErrExec}

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, []string{"i/match/nothing"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles: []string{"a.txt"},

		// BuildAndDeploy call will ultimately fail with this error,
		// b/c we DON'T fall back to an image build
		expectErrorContains: "failed with exit code: 123",

		// called exec twice (tar archive, run command) before hitting error
		expectK8sExecCount: 2,

		// DO NOT fall back to image build
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectK8sDeploy:        false,
	}
	runTestCase(t, f, tCase)
}

// If any container updates fail with a non-UserRunFailure, fall back to image build.
func TestLiveUpdateMultipleContainersFallsBackForFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	f.docker.SetExecError(fmt.Errorf("egads"))

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest:                  m,
		runningContainersByTarget: map[model.TargetID][]container.ID{m.ImageTargetAt(0).ID(): cIDs},
		changedFiles:              []string{"a.txt"},

		// attempted container update; called copy and exec before hitting error
		expectDockerCopyRange: intRange{min: 1, max: 3},
		expectDockerExecRange: intRange{min: 1, max: 3},

		// fell back to image build
		expectDockerBuildRange: intRange{min: 1, max: 1},
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

// Even if the first container update succeeds, if any subsequent container updates
// fail with a non-UserRunFailure, fall back to image build.
func TestLiveUpdateMultipleContainersFallsBackForFailureAfterSuccess(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	// First call = no error, second call = error
	f.docker.ExecErrorsToThrow = []error{nil, fmt.Errorf("egads")}

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest:                  m,
		runningContainersByTarget: map[model.TargetID][]container.ID{m.ImageTargetAt(0).ID(): cIDs},
		changedFiles:              []string{"a.txt"},

		// one successful update (copy, exec, restart);
		// one truncated update (copy, exec) before hitting error
		expectDockerCopyRange:    intRange{min: 1, max: 3},
		expectDockerExecRange:    intRange{min: 1, max: 3},
		expectDockerRestartCount: 1,

		// fell back to image build
		expectDockerBuildRange: intRange{min: 1, max: 1},
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

// If one container update fails with UserRunFailure, continue running updates on
// all containers. If ALL the updates fail with a UserRunFailure, don't fall back.
func TestLiveUpdateMultipleContainersUpdatesAllForUserRunFailuresAndDoesntFallBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	// Same UserRunFailure on all three exec calls
	f.docker.ExecErrorsToThrow = []error{userFailureErrDocker, userFailureErrDocker, userFailureErrDocker}

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest:                  m,
		runningContainersByTarget: map[model.TargetID][]container.ID{m.ImageTargetAt(0).ID(): cIDs},
		changedFiles:              []string{"a.txt"},

		// BuildAndDeploy call will ultimately fail with this error,
		// b/c we DON'T fall back to an image build
		expectErrorContains: "failed with exit code: 123",

		// attempted update for each container;
		// for each, called copy and exec before hitting error
		// (so did not call restart)
		expectDockerCopyRange:    intRange{min: 3, max: 3},
		expectDockerExecRange:    intRange{min: 3, max: 3},
		expectDockerRestartCount: 0,

		// DO NOT fall back to image build
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectK8sDeploy:        false,
	}
	runTestCase(t, f, tCase)
}

// If only SOME container updates fail with a UserRunFailure (1+ succeeds, or 1+ fails
// with a non-UserRunFailure), fall back to an image build.
func TestLiveUpdateMultipleContainersFallsBackForSomeUserRunFailuresSomeSuccess(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	f.docker.ExecErrorsToThrow = []error{userFailureErrDocker, nil, userFailureErrDocker}

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest:                  m,
		runningContainersByTarget: map[model.TargetID][]container.ID{m.ImageTargetAt(0).ID(): cIDs},
		changedFiles:              []string{"a.txt"},

		// one truncated update (copy and exec before hitting error)
		// one successful update (copy, exec, restart)
		// fall back before attempting third update
		expectDockerCopyRange:    intRange{min: 2, max: 2},
		expectDockerExecRange:    intRange{min: 2, max: 2},
		expectDockerRestartCount: 1,

		// fell back to image build
		expectDockerBuildRange: intRange{min: 1, max: 1},
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateMultipleContainersFallsBackForSomeUserRunFailuresSomeNonUserFailures(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	f.docker.ExecErrorsToThrow = []error{
		userFailureErrDocker,
		fmt.Errorf("not a user failure"),
		userFailureErrDocker,
	}

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		manifest:                  m,
		runningContainersByTarget: map[model.TargetID][]container.ID{m.ImageTargetAt(0).ID(): cIDs},
		changedFiles:              []string{"a.txt"},

		// two truncated updates (copy and exec before hitting error)
		// fall back before attempting third update
		expectDockerCopyRange:    intRange{min: 3, max: 3},
		expectDockerExecRange:    intRange{min: 3, max: 3},
		expectDockerRestartCount: 0,

		// fell back to image build
		expectDockerBuildRange: intRange{min: 1, max: 1},
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoCustomBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"app/a.txt"},
		expectDockerBuildRange:   intRange{min: 0, max: 0},
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 1, max: 1},
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 0, max: 0},
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 1, max: 1},
		expectDockerRestartCount: 0,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateRunTriggerLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	runs := []v1alpha1.LiveUpdateExec{
		{Args: model.ToUnixCmd("echo hello").Argv},
		{Args: model.ToUnixCmd("echo a").Argv, TriggerPaths: []string{"a.txt"}}, // matches changed file
		{Args: model.ToUnixCmd("echo b").Argv, TriggerPaths: []string{"b.txt"}}, // does NOT match changed file
	}
	lu := assembleLiveUpdate(SanchoSyncSteps(f), runs, true, nil, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 0, max: 0},
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 2, max: 2}, // one run's triggers don't match -- should only exec the other two.
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateRunTriggerExec(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	runs := []v1alpha1.LiveUpdateExec{
		{Args: model.ToUnixCmd("echo hello").Argv},
		{Args: model.ToUnixCmd("echo a").Argv, TriggerPaths: []string{"a.txt"}}, // matches changed file
		{Args: model.ToUnixCmd("echo b").Argv, TriggerPaths: []string{"b.txt"}}, // does NOT match changed file
	}
	lu := assembleLiveUpdate(SanchoSyncSteps(f), runs, false, nil, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 0, max: 0},
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0}, // one run's triggers don't match -- should only exec the other two.
		expectDockerRestartCount: 0,
		expectK8sExecCount:       3, // one copy, two runs (third run's triggers don't match so don't exec it)
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildExec(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoCustomBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"app/a.txt"},
		expectDockerBuildRange:   intRange{min: 0, max: 0},
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sExecCount:       2,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateExecDoesNotSupportRestart(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeContainerd)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 1, max: 1}, // expect Docker push on GKE
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy
		logsContain:              []string{"unexpected error", "ExecUpdater does not support `restart_container()` step"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildExec(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeContainerd)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:           []string{"a.txt"},
		expectDockerBuildRange: intRange{min: 0, max: 0},
		expectDockerPushRange:  intRange{min: 0, max: 0},
		expectK8sExecCount:     2, // one tar archive, one run cmd
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateLocalContainerFallBackOn(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"a.txt"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy
		logsContain:              []string{"Detected change to fall_back_on file", "a.txt"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateExecFallBackOn(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	lu := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, []string{"a.txt"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles:             []string{"a.txt"},
		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 1, max: 1},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // because we fell back to image builder, we also did a k8s deploy
		logsContain:              []string{"Detected change to fall_back_on file", "a.txt"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateLocalContainerChangedFileNotMatchingSyncFallsBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     filepath.Join("specific", "directory"),
		ContainerPath: "/go/src/github.com/tilt-dev/sancho",
	}}

	lu := assembleLiveUpdate(steps, SanchoRunSteps, true, []string{"a.txt"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles: []string{f.JoinPath("a.txt")}, // matches context but not sync'd directory

		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy

		logsContain:     []string{"Found file(s) not matching any sync", "a.txt"},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateExecChangedFileNotMatchingSyncFallsBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     filepath.Join("specific", "directory"),
		ContainerPath: "/go/src/github.com/tilt-dev/sancho",
	}}

	lu := assembleLiveUpdate(steps, SanchoRunSteps, false, []string{"a.txt"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles: []string{f.JoinPath("a.txt")}, // matches context but not sync'd directory

		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 1, max: 1},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // because we fell back to image builder, we also did a k8s deploy

		logsContain:     []string{"Found file(s) not matching any sync", "a.txt"},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateManyFilesNotMatching(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     filepath.Join("specific", "directory"),
		ContainerPath: "/go/src/github.com/tilt-dev/sancho",
	}}

	changedFiles := []string{}
	for i := 0; i < 100; i++ {
		changedFiles = append(changedFiles, f.JoinPath(fmt.Sprintf("a%d.txt", i)))
	}

	expectedList := fmt.Sprintf("%s %s %s %s %s ...",
		f.JoinPath("a0.txt"),
		f.JoinPath("a1.txt"),
		f.JoinPath("a10.txt"),
		f.JoinPath("a11.txt"),
		f.JoinPath("a12.txt"))

	lu := assembleLiveUpdate(steps, SanchoRunSteps, false, []string{"a.txt"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		changedFiles: changedFiles,

		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 1, max: 1},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // because we fell back to image builder, we also did a k8s deploy

		logsContain:     []string{"Found file(s) not matching any sync", expectedList},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateSomeFilesMatchSyncSomeDontFallsBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     filepath.Join("specific", "directory"),
		ContainerPath: "/go/src/github.com/tilt-dev/sancho",
	}}

	lu := assembleLiveUpdate(steps, SanchoRunSteps, true, []string{"a.txt"}, f)
	tCase := testCase{
		manifest: manifestbuilder.New(f, "sancho").
			WithK8sYAML(SanchoYAML).
			WithLiveUpdateBAD().
			WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
			WithLiveUpdate(lu).
			Build(),
		// One file matches a sync, one does not -- we should still fall back.
		changedFiles: f.JoinPaths([]string{"specific/directory/i_match", "a.txt"}),

		expectDockerBuildRange:   intRange{min: 1, max: 1}, // we did a Docker build instead of an in-place update!
		expectDockerPushRange:    intRange{min: 0, max: 0},
		expectDockerCopyRange:    intRange{min: 0, max: 0},
		expectDockerExecRange:    intRange{min: 0, max: 0},
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy

		logsContain:     []string{"Found file(s) not matching any sync", "a.txt"},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateInFirstImageOfImageDependency(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     "sancho-base",
		ContainerPath: "/go/src/github.com/tilt-dev/sancho-base",
	}}

	lu := assembleLiveUpdate(steps, SanchoRunSteps, true, nil, f)
	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTargets(NewSanchoMultiStageImages(f)...).
		WithLiveUpdateAtIndex(lu, 1).
		Build()
	tCase := testCase{
		manifest:                 m,
		changedFiles:             []string{"sancho-base/a.txt"},
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 1, max: 1},
		expectDockerRestartCount: 1,
		logsContain:              []string{f.JoinPath("sancho-base/a.txt")},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateInFirstImageOfImageDependencyWithoutSync(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     "sancho",
		ContainerPath: "/go/src/github.com/tilt-dev/sancho",
	}}

	lu := assembleLiveUpdate(steps, SanchoRunSteps, true, nil, f)
	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTargets(NewSanchoMultiStageImages(f)...).
		WithLiveUpdateAtIndex(lu, 1).
		Build()
	tCase := testCase{
		manifest:               m,
		changedFiles:           []string{"sancho-base/a.txt"},
		expectDockerBuildRange: intRange{min: 2, max: 2},
		expectK8sDeploy:        true,
		logsContain:            []string{f.JoinPath("sancho-base/a.txt")},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateInSecondImageOfImageDependency(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)

	steps := []v1alpha1.LiveUpdateSync{{
		LocalPath:     "sancho",
		ContainerPath: "/go/src/github.com/tilt-dev/sancho",
	}}

	lu := assembleLiveUpdate(steps, SanchoRunSteps, true, nil, f)
	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTargets(NewSanchoMultiStageImages(f)...).
		WithLiveUpdateAtIndex(lu, 1).
		Build()
	tCase := testCase{
		manifest:                 m,
		changedFiles:             []string{"sancho/a.txt"},
		expectDockerCopyRange:    intRange{min: 1, max: 1},
		expectDockerExecRange:    intRange{min: 1, max: 1},
		expectDockerRestartCount: 1,
		logsContain:              []string{f.JoinPath("sancho/a.txt")},
	}
	runTestCase(t, f, tCase)
}

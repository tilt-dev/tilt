package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/docker"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
)

var userFailureErr = docker.ExitError{ExitCode: 123}

type testCase struct {
	baseManifest model.Manifest
	liveUpdate   model.LiveUpdate

	changedFiles        []string       // leave empty for from-scratch build
	runningContainerIDs []container.ID // if empty, use default container

	// Expect the BuildAndDeploy call to fail with an error containing this string
	expectErrorContains string

	// Docker actions
	expectDockerBuildCount   int
	expectDockerPushCount    int
	expectDockerCopyCount    int
	expectDockerExecCount    int
	expectDockerRestartCount int

	// Synclet actions
	expectSyncletUpdateContainerCount int
	expectSyncletCommandCount         int
	expectSyncletHotReload            bool

	// k8s/deploy actions
	expectK8sExecCount  int
	expectK8sDeploy     bool
	expectSyncletDeploy bool

	// logs checks
	logsContain     []string
	logsDontContain []string
}

func runTestCase(t *testing.T, f *bdFixture, tCase testCase) {
	if len(tCase.changedFiles) == 0 && len(tCase.runningContainerIDs) != 0 {
		t.Fatal("can't specify both empty changedFiles (implies from-scratch " +
			"build) and non-empty running containers (implies there's an existing build " +
			"to LiveUpdate on top of).")
	}

	manifest := tCase.baseManifest
	bs := f.createBuildStateSet(manifest, tCase.changedFiles)

	// Assume that the last image target is the deployed one.
	iTargIdx := len(manifest.ImageTargets) - 1
	iTarg := manifest.ImageTargetAt(iTargIdx)
	require.True(t, manifest.IsImageDeployed(iTarg))

	if len(tCase.runningContainerIDs) > 0 {
		cInfos := make([]store.ContainerInfo, len(tCase.runningContainerIDs))
		for i, id := range tCase.runningContainerIDs {
			cInfos[i] = store.ContainerInfo{
				PodID:         testPodID,
				ContainerID:   id,
				ContainerName: container.Name(fmt.Sprintf("container %s", id)),
			}
		}
		bs[iTarg.ID()] = bs[iTarg.ID()].WithRunningContainers(cInfos)
	}

	db := iTarg.DockerBuildInfo()
	db.LiveUpdate = tCase.liveUpdate
	manifest.ImageTargets[iTargIdx] = iTarg.WithBuildDetails(db)
	targets := buildTargets(manifest)

	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if tCase.expectErrorContains != "" {
		require.NotNil(t, err, "expected error containing '%s' but got no error", tCase.expectErrorContains)
		require.Contains(t, err.Error(), tCase.expectErrorContains,
			"expected BuildAndDeploy error to contain string")
	} else if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, tCase.expectDockerBuildCount, f.docker.BuildCount, "docker build")
	assert.Equal(t, tCase.expectDockerPushCount, f.docker.PushCount, "docker push")
	assert.Equal(t, tCase.expectDockerCopyCount, f.docker.CopyCount, "docker copy")
	assert.Equal(t, tCase.expectDockerExecCount, len(f.docker.ExecCalls), "docker exec")
	if len(tCase.runningContainerIDs) > 0 {
		f.assertTotalContainerRestarts(tCase.expectDockerRestartCount)
	} else {
		f.assertContainerRestarts(tCase.expectDockerRestartCount)
	}

	assert.Equal(t, tCase.expectSyncletUpdateContainerCount, f.sCli.UpdateContainerCount, "synclet update container")
	assert.Equal(t, tCase.expectSyncletCommandCount, f.sCli.CommandsRunCount, "synclet commands run")
	assert.Equal(t, tCase.expectSyncletHotReload, f.sCli.LastHotReload, "synclet hot reload")

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

	id := manifest.ImageTargetAt(iTargIdx).ID()
	imgRes, hasResult := result[id]
	if tCase.expectErrorContains != "" {
		return
	}

	require.True(t, hasResult, "expect build result for image")
	if !tCase.expectK8sDeploy {
		assert.Empty(t, f.k8s.Yaml, "expected no k8s deploy, but we deployed YAML: %s", f.k8s.Yaml)

		if len(tCase.runningContainerIDs) > 0 {
			// We expect to have operated on the number of containers that the user specified
			assert.Equal(t, imgRes.LiveUpdatedContainerIDs, tCase.runningContainerIDs)
		} else {
			// We did a container build, so we expect result to have the container ID we operated on
			assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyLiveUpdatedContainerID().String())
		}
	} else {
		expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
		if !strings.Contains(f.k8s.Yaml, expectedYaml) {
			t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
		}
		assert.Equal(t, tCase.expectSyncletDeploy, strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName), "expected synclet-deploy = %t (deployed yaml was: %s)", tCase.expectSyncletDeploy, f.k8s.Yaml)
	}
}

func TestLiveUpdateDockerBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildLocalContainerOnMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildManifest(f),
		runningContainerIDs:    cIDs,
		liveUpdate:             lu,
		changedFiles:           []string{"a.txt"},
		expectDockerBuildCount: 0,
		expectDockerPushCount:  0,

		// one of each operation per container
		expectDockerCopyCount:    3,
		expectDockerExecCount:    3,
		expectDockerRestartCount: 3,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildSyncletOnMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildManifest(f),
		runningContainerIDs:    cIDs,
		liveUpdate:             lu,
		changedFiles:           []string{"a.txt"},
		expectDockerBuildCount: 0,
		expectDockerPushCount:  0,

		// one of each operation per container
		expectSyncletUpdateContainerCount: 3,
		expectSyncletCommandCount:         3,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildExecOnMultipleContainers(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeCrio)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), nil, false, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildManifest(f),
		runningContainerIDs:    cIDs,
		liveUpdate:             lu,
		changedFiles:           []string{"a.txt"},
		expectDockerBuildCount: 0,
		expectDockerPushCount:  0,

		// 1 per container (tar archive) x 3 containers
		expectK8sExecCount: 3,
	}
	runTestCase(t, f, tCase)
}

// If any container updates fail with a non-UserRunFailure, fall back to image build.
func TestLiveUpdateMultipleContainersFallsBackForFailure(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	f.docker.SetExecError(fmt.Errorf("egads"))

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:        NewSanchoDockerBuildManifest(f),
		runningContainerIDs: cIDs,
		liveUpdate:          lu,
		changedFiles:        []string{"a.txt"},

		// attempted container update; called copy and exec before hitting error
		expectDockerCopyCount: 1,
		expectDockerExecCount: 1,

		// fell back to image build
		expectDockerBuildCount: 1,
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

// Even if the first container update succeeds, if any subsequent container updates
// fail with a non-UserRunFailure, fall back to image build.
func TestLiveUpdateMultipleContainersFallsBackForFailureAfterSuccess(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	// First call = no error, second call = error
	f.docker.ExecErrorsToThrow = []error{nil, fmt.Errorf("egads")}

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:        NewSanchoDockerBuildManifest(f),
		runningContainerIDs: cIDs,
		liveUpdate:          lu,
		changedFiles:        []string{"a.txt"},

		// one successful update (copy, exec, restart);
		// one truncated update (copy, exec) before hitting error
		expectDockerCopyCount:    2,
		expectDockerExecCount:    2,
		expectDockerRestartCount: 1,

		// fell back to image build
		expectDockerBuildCount: 1,
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

// If one container update fails with UserRunFailure, continue running updates on
// all containers. If ALL the updates fail with a UserRunFailure, don't fall back.
func TestLiveUpdateMultipleContainersUpdatesAllForUserRunFailuresAndDoesntFallBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	// Same UserRunFailure on all three exec calls
	f.docker.ExecErrorsToThrow = []error{userFailureErr, userFailureErr, userFailureErr}

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:        NewSanchoDockerBuildManifest(f),
		runningContainerIDs: cIDs,
		liveUpdate:          lu,
		changedFiles:        []string{"a.txt"},

		// BuildAndDeploy call will ultimately fail with this error,
		// b/c we DON'T fall back to an image build
		expectErrorContains: "failed with exit code: 123",

		// attempted update for each container;
		// for each, called copy and exec before hitting error
		// (so did not call restart)
		expectDockerCopyCount:    3,
		expectDockerExecCount:    3,
		expectDockerRestartCount: 0,

		// DO NOT fall back to image build
		expectDockerBuildCount: 0,
		expectK8sDeploy:        false,
	}
	runTestCase(t, f, tCase)
}

// If only SOME container updates fail with a UserRunFailure (1+ succeeds, or 1+ fails
// with a non-UserRunFailure), fall back to an image build.
func TestLiveUpdateMultipleContainersFallsBackForSomeUserRunFailuresSomeSuccess(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	f.docker.ExecErrorsToThrow = []error{userFailureErr, nil, userFailureErr}

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:        NewSanchoDockerBuildManifest(f),
		runningContainerIDs: cIDs,
		liveUpdate:          lu,
		changedFiles:        []string{"a.txt"},

		// one truncated update (copy and exec before hitting error)
		// one successful update (copy, exec, restart)
		// fall back before attempting third update
		expectDockerCopyCount:    2,
		expectDockerExecCount:    2,
		expectDockerRestartCount: 1,

		// fell back to image build
		expectDockerBuildCount: 1,
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateMultipleContainersFallsBackForSomeUserRunFailuresSomeNonUserFailures(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	f.docker.ExecErrorsToThrow = []error{
		userFailureErr,
		fmt.Errorf("not a user failure"),
		userFailureErr,
	}

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	cIDs := []container.ID{"c1", "c2", "c3"}
	tCase := testCase{
		baseManifest:        NewSanchoDockerBuildManifest(f),
		runningContainerIDs: cIDs,
		liveUpdate:          lu,
		changedFiles:        []string{"a.txt"},

		// two truncated updates (copy and exec before hitting error)
		// fall back before attempting third update
		expectDockerCopyCount:    2,
		expectDockerExecCount:    2,
		expectDockerRestartCount: 0,

		// fell back to image build
		expectDockerBuildCount: 1,
		expectK8sDeploy:        true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoCustomBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"app/a.txt"},
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 0,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateRunTriggerLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	runs := []model.LiveUpdateRunStep{
		model.LiveUpdateRunStep{Command: model.ToShellCmd("echo hello")},
		model.LiveUpdateRunStep{Command: model.ToShellCmd("echo a"), Triggers: f.NewPathSet("a.txt")}, // matches changed file
		model.LiveUpdateRunStep{Command: model.ToShellCmd("echo b"), Triggers: f.NewPathSet("b.txt")}, // does NOT match changed file
	}
	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), runs, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    2, // one run's triggers don't match -- should only exec the other two.
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateRunTriggerSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	runs := []model.LiveUpdateRunStep{
		model.LiveUpdateRunStep{Command: model.ToShellCmd("echo hello")},
		model.LiveUpdateRunStep{Command: model.ToShellCmd("echo a"), Triggers: f.NewPathSet("a.txt")}, // matches changed file
		model.LiveUpdateRunStep{Command: model.ToShellCmd("echo b"), Triggers: f.NewPathSet("b.txt")}, // does NOT match changed file
	}
	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), runs, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:                      NewSanchoDockerBuildManifest(f),
		liveUpdate:                        lu,
		changedFiles:                      []string{"a.txt"},
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletCommandCount:         2, // one run's triggers don't match -- should only exec the other two.
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:                      NewSanchoDockerBuildManifest(f),
		liveUpdate:                        lu,
		changedFiles:                      []string{"a.txt"},
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletCommandCount:         1,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:                      NewSanchoCustomBuildManifest(f),
		liveUpdate:                        lu,
		changedFiles:                      []string{"app/a.txt"},
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletCommandCount:         1,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:                      NewSanchoDockerBuildManifest(f),
		liveUpdate:                        lu,
		changedFiles:                      []string{"a.txt"},
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletCommandCount:         1,
		expectSyncletHotReload:            true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateExecDoesNotSupportRestart(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeContainerd)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildCount:   1, // we did a Docker build instead of an in-place update!
		expectDockerPushCount:    1, // expect Docker push on GKE
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy
		logsContain:              []string{"unexpected error", "ExecUpdater does not support `restart_container()` step"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildExec(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeContainerd)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildManifest(f),
		liveUpdate:             lu,
		changedFiles:           []string{"a.txt"},
		expectDockerBuildCount: 0,
		expectDockerPushCount:  0,
		expectK8sExecCount:     2, // one tar archive, one run cmd
	}
	runTestCase(t, f, tCase)
}

func TestDockerBuildDoesNotDeploySynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildManifest(f),
		changedFiles:           nil, // will use an empty BuildResultSet, i.e. treat this as first build
		expectDockerBuildCount: 1,
		expectDockerPushCount:  1,
		expectK8sDeploy:        true,
		expectSyncletDeploy:    false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildDeploysSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}

	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildManifest(f),
		liveUpdate:             lu,
		changedFiles:           nil, // will use an empty BuildResultSet, i.e. treat this as first build
		expectDockerBuildCount: 1,
		expectDockerPushCount:  1,
		expectK8sDeploy:        true,
		expectSyncletDeploy:    true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateLocalContainerFallBackOn(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildCount:   1, // we did a Docker build instead of an in-place update!
		expectDockerPushCount:    0,
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy
		logsContain:              []string{"detected change to fall_back_on file", f.JoinPath("a.txt")},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateSyncletFallBackOn(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"a.txt"},
		expectDockerBuildCount:   1, // we did a Docker build instead of an in-place update!
		expectDockerPushCount:    1,
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // because we fell back to image builder, we also did a k8s deploy
		expectSyncletDeploy:      true, // (and expect that yaml to have contained the synclet)
		logsContain:              []string{"detected change to fall_back_on file", f.JoinPath("a.txt")},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateLocalContainerChangedFileNotMatchingSyncFallsBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	steps := []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: f.JoinPath("specific/directory"),
		Dest:   "/go/src/github.com/windmilleng/sancho",
	}}

	lu, err := assembleLiveUpdate(steps, SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest: NewSanchoDockerBuildManifest(f),
		liveUpdate:   lu,
		changedFiles: []string{f.JoinPath("a.txt")}, // matches context but not sync'd directory

		expectDockerBuildCount:   1, // we did a Docker build instead of an in-place update!
		expectDockerPushCount:    0,
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy

		logsContain:     []string{f.JoinPath("a.txt"), "doesn't match a LiveUpdate sync"},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateSyncletChangedFileNotMatchingSyncFallsBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE, container.RuntimeDocker)
	defer f.TearDown()

	steps := []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: f.JoinPath("specific/directory"),
		Dest:   "/go/src/github.com/windmilleng/sancho",
	}}

	lu, err := assembleLiveUpdate(steps, SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest: NewSanchoDockerBuildManifest(f),
		liveUpdate:   lu,
		changedFiles: []string{f.JoinPath("a.txt")}, // matches context but not sync'd directory

		expectDockerBuildCount:   1, // we did a Docker build instead of an in-place update!
		expectDockerPushCount:    1,
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // because we fell back to image builder, we also did a k8s deploy
		expectSyncletDeploy:      true, // (and expect that yaml to have contained the synclet)

		logsContain:     []string{f.JoinPath("a.txt"), "doesn't match a LiveUpdate sync"},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateSomeFilesMatchSyncSomeDontFallsBack(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	steps := []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: f.JoinPath("specific/directory"),
		Dest:   "/go/src/github.com/windmilleng/sancho",
	}}

	lu, err := assembleLiveUpdate(steps, SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		baseManifest: NewSanchoDockerBuildManifest(f),
		liveUpdate:   lu,
		// One file matches a sync, one does not -- we should still fall back.
		changedFiles: f.JoinPaths([]string{"specific/directory/i_match", "a.txt"}),

		expectDockerBuildCount:   1, // we did a Docker build instead of an in-place update!
		expectDockerPushCount:    0,
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true, // Because we fell back to image builder, we also did a k8s deploy

		logsContain:     []string{f.JoinPath("a.txt"), "doesn't match a LiveUpdate sync"},
		logsDontContain: []string{"unexpected error"},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateInFirstImageOfImageDependency(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	steps := []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: f.JoinPath("sancho-base"),
		Dest:   "/go/src/github.com/windmilleng/sancho-base",
	}}

	lu, err := assembleLiveUpdate(steps, SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}

	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildMultiStageManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"sancho-base/a.txt"},
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 1,
		logsContain:              []string{f.JoinPath("sancho-base/a.txt")},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateInFirstImageOfImageDependencyWithoutSync(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	steps := []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: f.JoinPath("sancho"),
		Dest:   "/go/src/github.com/windmilleng/sancho",
	}}

	lu, err := assembleLiveUpdate(steps, SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}

	tCase := testCase{
		baseManifest:           NewSanchoDockerBuildMultiStageManifest(f),
		liveUpdate:             lu,
		changedFiles:           []string{"sancho-base/a.txt"},
		expectDockerBuildCount: 2,
		expectK8sDeploy:        true,
		logsContain:            []string{f.JoinPath("sancho-base/a.txt")},
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateInSecondImageOfImageDependency(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop, container.RuntimeDocker)
	defer f.TearDown()

	steps := []model.LiveUpdateSyncStep{model.LiveUpdateSyncStep{
		Source: f.JoinPath("sancho"),
		Dest:   "/go/src/github.com/windmilleng/sancho",
	}}

	lu, err := assembleLiveUpdate(steps, SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}

	tCase := testCase{
		baseManifest:             NewSanchoDockerBuildMultiStageManifest(f),
		liveUpdate:               lu,
		changedFiles:             []string{"sancho/a.txt"},
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 1,
		logsContain:              []string{f.JoinPath("sancho/a.txt")},
	}
	runTestCase(t, f, tCase)
}

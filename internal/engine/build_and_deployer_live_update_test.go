package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
)

type testCase struct {
	env k8s.Env

	baseManifest model.Manifest
	liveUpdate   model.LiveUpdate

	changedFile string // leave empty for no changed files

	// Docker actions
	expectDockerBuildCount   int
	expectDockerPushCount    int
	expectDockerCopyCount    int
	expectDockerExecCount    int
	expectDockerRestartCount int

	// Synclet actions
	expectSyncletUpdateContainerCount int
	expectSyncletHotReload            bool

	// k8s/deploy actions
	expectK8sDeploy     bool
	expectSyncletDeploy bool
}

func runTestCase(t *testing.T, f *bdFixture, tCase testCase) {
	var bs store.BuildStateSet
	if tCase.changedFile != "" {
		changed := f.WriteFile(tCase.changedFile, "blah")
		bs = resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	}

	manifest := tCase.baseManifest
	iTarg := manifest.ImageTargetAt(0)
	db := iTarg.DockerBuildInfo()
	db.LiveUpdate = &tCase.liveUpdate
	manifest = manifest.WithImageTarget(iTarg.WithBuildDetails(db))
	targets := buildTargets(manifest)

	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, tCase.expectDockerBuildCount, f.docker.BuildCount, "docker build")
	assert.Equal(t, tCase.expectDockerPushCount, f.docker.PushCount, "docker push")
	assert.Equal(t, tCase.expectDockerCopyCount, f.docker.CopyCount, "docker copy")
	assert.Equal(t, tCase.expectDockerExecCount, len(f.docker.ExecCalls), "docker exec")
	assert.Equal(t, tCase.expectSyncletUpdateContainerCount, f.sCli.UpdateContainerCount, "synclet update container")
	f.assertContainerRestarts(tCase.expectDockerRestartCount)
	assert.Equal(t, tCase.expectSyncletHotReload, f.sCli.UpdateContainerHotReload, "synclet hot reload")

	id := manifest.ImageTargetAt(0).ID()
	_, hasResult := result[id]
	assert.True(t, hasResult)

	if !tCase.expectK8sDeploy {
		assert.Empty(t, f.k8s.Yaml, "expected no k8s deploy, but we deployed YAML: %s", f.k8s.Yaml)

		// We did a container build, so we expect result to have the container ID we operated on
		assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())
	} else {
		expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
		if !strings.Contains(f.k8s.Yaml, expectedYaml) {
			t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
		}
		assert.Equal(t, tCase.expectSyncletDeploy, strings.Contains(f.k8s.Yaml, sidecar.SyncletImageName), "expected synclet-deploy = %t (deployed yaml was: %s)", tCase.expectSyncletDeploy, f.k8s.Yaml)
	}

}

func TestLiveUpdateDockerBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"})
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoDockerBuildManifest(),
		liveUpdate:               lu,
		changedFile:              "a.txt",
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"})
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoCustomBuildManifest(f),
		liveUpdate:               lu,
		changedFile:              "a.txt",
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, false, nil)
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoDockerBuildManifest(),
		liveUpdate:               lu,
		changedFile:              "a.txt",
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    1,
		expectDockerRestartCount: 0,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateRunTriggerLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	runs := []model.LiveUpdateRunStep{{
		Command:  model.ToShellCmd("echo hi"),
		Triggers: []string{"b.txt"}, // does NOT match changed file
	}}
	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), runs, true, nil)
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoDockerBuildManifest(),
		liveUpdate:               lu,
		changedFile:              "a.txt",
		expectDockerBuildCount:   0,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    1,
		expectDockerExecCount:    0, // Run doesn't match changed file, so shouldn't exec
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, true, nil)
	tCase := testCase{
		env:                               k8s.EnvGKE,
		baseManifest:                      NewSanchoDockerBuildManifest(),
		liveUpdate:                        lu,
		changedFile:                       "a.txt",
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, true, nil)
	tCase := testCase{
		env:                               k8s.EnvGKE,
		baseManifest:                      NewSanchoCustomBuildManifest(f),
		liveUpdate:                        lu,
		changedFile:                       "a.txt",
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, false, nil)
	tCase := testCase{
		env:                               k8s.EnvGKE,
		baseManifest:                      NewSanchoDockerBuildManifest(),
		liveUpdate:                        lu,
		changedFile:                       "a.txt",
		expectDockerBuildCount:            0,
		expectDockerPushCount:             0,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletHotReload:            true,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildDeploysSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()
	tCase := testCase{
		env:                    k8s.EnvGKE,
		baseManifest:           NewSanchoDockerBuildManifest(),
		changedFile:            "", // will use an empty BuildResultSet, i.e. treat this as first build
		expectDockerBuildCount: 1,
		expectDockerPushCount:  1,
		expectK8sDeploy:        true,
		expectSyncletDeploy:    true,
	}
	runTestCase(t, f, tCase)
}

func _TestLiveUpdateLocalContainerFullBuildTrigger(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu := assembleLiveUpdate(t, SanchoSyncSteps(f), SanchoRunSteps, true, []string{"a.txt"})
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoDockerBuildManifest(),
		liveUpdate:               lu,
		changedFile:              "a.txt",
		expectDockerBuildCount:   1,
		expectDockerPushCount:    0,
		expectDockerCopyCount:    0,
		expectDockerExecCount:    0,
		expectDockerRestartCount: 0,
		expectK8sDeploy:          true,
	}
	runTestCase(t, f, tCase)
}

func _TestLiveUpdateSyncletFullBuildTrigger(t *testing.T) {}

func assembleLiveUpdate(t *testing.T, syncs []model.LiveUpdateSyncStep, runs []model.LiveUpdateRunStep, shouldRestart bool, fullRebuildTriggers []string) model.LiveUpdate {
	var steps []model.LiveUpdateStep
	for _, sync := range syncs {
		steps = append(steps, sync)
	}
	for _, run := range runs {
		steps = append(steps, run)
	}
	if shouldRestart {
		steps = append(steps, model.LiveUpdateRestartContainerStep{})
	}
	lu, err := model.NewLiveUpdate(steps, fullRebuildTriggers)
	if err != nil {
		t.Fatal(err)
	}
	return lu
}

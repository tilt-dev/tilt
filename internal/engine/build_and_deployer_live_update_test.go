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

	baseManifest        model.Manifest
	liveUpdSyncs        []model.LiveUpdateSyncStep
	liveUpdRuns         []model.LiveUpdateRunStep
	liveUpdRestart      bool
	fullRebuildTriggers []string

	changedFile string // leave empty for no changed files

	// Docker actions
	expectBuildCount         int
	expectPushCount          int
	expectCopyCount          int
	expectExecCount          int
	expectDockerRestartCount int

	// Synclet actions
	expectSyncletUpdateContainerCount int
	expectSyncletHotReload            bool
	expectK8sDeploy                   bool
	expectSyncletDeploy               bool
}

func runTestCase(t *testing.T, f *bdFixture, tCase testCase) {
	var bs store.BuildStateSet
	if tCase.changedFile != "" {
		changed := f.WriteFile(tCase.changedFile, "blah")
		bs = resultToStateSet(alreadyBuiltSet, []string{changed}, f.deployInfo())
	}

	var steps []model.LiveUpdateStep
	for _, sync := range tCase.liveUpdSyncs {
		steps = append(steps, sync)
	}
	for _, run := range tCase.liveUpdRuns {
		steps = append(steps, run)
	}
	if tCase.liveUpdRestart {
		steps = append(steps, model.LiveUpdateRestartContainerStep{})
	}
	lu := model.MustNewLiveUpdate(steps, tCase.fullRebuildTriggers)

	manifest := tCase.baseManifest
	iTarg := manifest.ImageTargetAt(0)
	db := iTarg.DockerBuildInfo()
	db.LiveUpdate = &lu
	manifest = manifest.WithImageTarget(iTarg.WithBuildDetails(db))
	targets := buildTargets(manifest)

	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, tCase.expectBuildCount, f.docker.BuildCount, "docker build")
	assert.Equal(t, tCase.expectPushCount, f.docker.PushCount, "docker push")
	assert.Equal(t, tCase.expectCopyCount, f.docker.CopyCount, "docker copy")
	assert.Equal(t, tCase.expectExecCount, len(f.docker.ExecCalls), "docker exec")
	assert.Equal(t, tCase.expectSyncletUpdateContainerCount, f.sCli.UpdateContainerCount, "synclet update container")
	f.assertContainerRestarts(tCase.expectDockerRestartCount)
	assert.Equal(t, tCase.expectSyncletHotReload, f.sCli.UpdateContainerHotReload)

	id := manifest.ImageTargetAt(0).ID()
	_, hasResult := result[id]
	assert.True(t, hasResult)
	assert.Equal(t, k8s.MagicTestContainerID, result.OneAndOnlyContainerID().String())

	if tCase.expectK8sDeploy {
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
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoDockerBuildManifest(),
		liveUpdSyncs:             SanchoSyncSteps(f),
		liveUpdRuns:              SanchoRunSteps,
		liveUpdRestart:           true,
		changedFile:              "a.txt",
		expectBuildCount:         0,
		expectPushCount:          0,
		expectCopyCount:          1,
		expectExecCount:          1,
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoCustomBuildManifest(f),
		liveUpdSyncs:             SanchoSyncSteps(f),
		liveUpdRuns:              SanchoRunSteps,
		liveUpdRestart:           true,
		changedFile:              "a.txt",
		expectBuildCount:         0,
		expectPushCount:          0,
		expectCopyCount:          1,
		expectExecCount:          1,
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
		baseManifest:             NewSanchoDockerBuildManifest(),
		liveUpdSyncs:             SanchoSyncSteps(f),
		liveUpdRuns:              SanchoRunSteps,
		liveUpdRestart:           false,
		changedFile:              "a.txt",
		expectBuildCount:         0,
		expectPushCount:          0,
		expectCopyCount:          1,
		expectExecCount:          1,
		expectDockerRestartCount: 0,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateRunTriggerLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()
	tCase := testCase{
		env:          k8s.EnvDockerDesktop,
		baseManifest: NewSanchoDockerBuildManifest(),
		liveUpdSyncs: SanchoSyncSteps(f),
		liveUpdRuns: []model.LiveUpdateRunStep{
			{
				Command: model.ToCmd("echo", "hi"),
				Trigger: "b.txt", // does NOT match changed file
			},
		},
		liveUpdRestart:           true,
		changedFile:              "a.txt",
		expectBuildCount:         0,
		expectPushCount:          0,
		expectCopyCount:          1,
		expectExecCount:          0, // Run doesn't match changed file, so shouldn't exec
		expectDockerRestartCount: 1,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateDockerBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()
	tCase := testCase{
		env:                               k8s.EnvGKE,
		baseManifest:                      NewSanchoDockerBuildManifest(),
		liveUpdSyncs:                      SanchoSyncSteps(f),
		liveUpdRuns:                       SanchoRunSteps,
		liveUpdRestart:                    false,
		changedFile:                       "a.txt",
		expectBuildCount:                  0,
		expectPushCount:                   0,
		expectCopyCount:                   1,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateCustomBuildSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()
	tCase := testCase{
		env:                               k8s.EnvGKE,
		baseManifest:                      NewSanchoCustomBuildManifest(f),
		liveUpdSyncs:                      SanchoSyncSteps(f),
		liveUpdRuns:                       SanchoRunSteps,
		liveUpdRestart:                    false,
		changedFile:                       "a.txt",
		expectBuildCount:                  0,
		expectPushCount:                   0,
		expectCopyCount:                   1,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletHotReload:            false,
	}
	runTestCase(t, f, tCase)
}

func TestLiveUpdateHotReloadSynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()
	tCase := testCase{
		env:                               k8s.EnvGKE,
		baseManifest:                      NewSanchoDockerBuildManifest(),
		liveUpdSyncs:                      SanchoSyncSteps(f),
		liveUpdRuns:                       SanchoRunSteps,
		liveUpdRestart:                    true,
		changedFile:                       "a.txt",
		expectBuildCount:                  0,
		expectPushCount:                   0,
		expectCopyCount:                   1,
		expectSyncletUpdateContainerCount: 1,
		expectSyncletHotReload:            true,
	}
	runTestCase(t, f, tCase)
}

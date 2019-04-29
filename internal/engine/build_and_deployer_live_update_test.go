package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
)

type testCase struct {
	env k8s.Env

	baseManifest model.Manifest
	liveUpdate   model.LiveUpdate

	changedFiles []string // leave empty for no changed files

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
	expectK8sDeploy     bool
	expectSyncletDeploy bool

	// logs checks
	logsContain     []string
	logsDontContain []string
}

func runTestCase(t *testing.T, f *bdFixture, tCase testCase) {
	bs := f.createBuildStateSet(tCase.baseManifest, tCase.changedFiles)
	manifest := tCase.baseManifest

	// Assume that the last image target is the deployed one.
	iTargIdx := len(manifest.ImageTargets) - 1
	iTarg := manifest.ImageTargetAt(iTargIdx)
	assert.True(t, manifest.IsImageDeployed(iTarg))

	db := iTarg.DockerBuildInfo()
	db.LiveUpdate = tCase.liveUpdate
	manifest.ImageTargets[iTargIdx] = iTarg.WithBuildDetails(db)
	targets := buildTargets(manifest)

	result, err := f.bd.BuildAndDeploy(f.ctx, f.st, targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, tCase.expectDockerBuildCount, f.docker.BuildCount, "docker build")
	assert.Equal(t, tCase.expectDockerPushCount, f.docker.PushCount, "docker push")
	assert.Equal(t, tCase.expectDockerCopyCount, f.docker.CopyCount, "docker copy")
	assert.Equal(t, tCase.expectDockerExecCount, len(f.docker.ExecCalls), "docker exec")
	f.assertContainerRestarts(tCase.expectDockerRestartCount)

	assert.Equal(t, tCase.expectSyncletUpdateContainerCount, f.sCli.UpdateContainerCount, "synclet update container")
	assert.Equal(t, tCase.expectSyncletCommandCount, f.sCli.CommandsRunCount, "synclet commands run")
	assert.Equal(t, tCase.expectSyncletHotReload, f.sCli.UpdateContainerHotReload, "synclet hot reload")

	id := manifest.ImageTargetAt(iTargIdx).ID()
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
}

func TestLiveUpdateDockerBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
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

func TestLiveUpdateCustomBuildLocalContainer(t *testing.T) {
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"i/match/nothing"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
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
		env:                      k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvGKE)
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
		env:                               k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                               k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                               k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                               k8s.EnvGKE,
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

func TestDockerBuildDoesNotDeploySynclet(t *testing.T) {
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	tCase := testCase{
		env:                    k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, false, nil, f)
	if err != nil {
		t.Fatal(err)
	}

	tCase := testCase{
		env:                    k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                      k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvGKE)
	defer f.TearDown()

	lu, err := assembleLiveUpdate(SanchoSyncSteps(f), SanchoRunSteps, true, []string{"a.txt"}, f)
	if err != nil {
		t.Fatal(err)
	}
	tCase := testCase{
		env:                      k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
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
		env:          k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvGKE)
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
		env:          k8s.EnvGKE,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
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
		env:          k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
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
		env:                      k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
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
		env:                    k8s.EnvDockerDesktop,
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
	f := newBDFixture(t, k8s.EnvDockerDesktop)
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
		env:                      k8s.EnvDockerDesktop,
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

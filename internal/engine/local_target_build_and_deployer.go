package engine

import (
	"context"
	"os/exec"
	"time"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

var _ BuildAndDeployer = &LocalTargetBuildAndDeployer{}

// TODO(maia): CommandRunner interface for testability
type LocalTargetBuildAndDeployer struct {
	clock build.Clock
}

func NewLocalTargetBuildAndDeployer(c build.Clock) *LocalTargetBuildAndDeployer {
	return &LocalTargetBuildAndDeployer{clock: c}
}

func (bd *LocalTargetBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	targets := bd.extract(specs)
	if len(targets) != 1 {
		return store.BuildResultSet{}, buildcontrol.SilentRedirectToNextBuilderf(
			"LocalTargetBuildAndDeployer requires exactly one LocalTarget (got %d)", len(targets))
	}

	targ := targets[0]
	err = bd.run(ctx, targ.UpdateCmd, targ.Workdir)
	if err != nil {
		// (Never fall back from the LocalTargetBaD, none of our other BaDs can handle this target)
		return store.BuildResultSet{}, buildcontrol.DontFallBackErrorf("Command %q failed: %v", targ.UpdateCmd.String(), err)
	}

	if state := stateSet[targ.ID()]; state.IsEmpty() {
		// HACK(maia) If target A generates file X and target B depends on file X, it was common that on Tilt startup,
		// targets A and B would both be queued for their initial build, A would run, modify X, and then B would start
		// running before Tilt processed the change to X, so we'd end up with this:
		// 1. A starts building
		// 2. A writes X
		// 3. A finishes building
		// 4. B starts building
		// 5. Tilt observes change to X
		// 6. B finishes building
		// 7. B is dirty because there was a change to X since the last time it started building, so it starts building again
		// Empirically, this sleep generally suffices to ensure that step (5) precedes step (4), which eliminates step (7)
		// It has been observed to fail to suffice when inotify is under load.
		// At the moment (2020-01-31), local_resources will not build in parallel with other resources, so this works fine
		// If/when we reenable parallel builds for local_resources, it will still help if the Tiltfile specifies
		// A as a resource dependency of B (NB: both the problem and resource_dep only apply to initial builds).
		time.Sleep(250 * time.Millisecond)
	}

	return bd.successfulBuildResult(targ), nil
}

// Extract the targets we can apply -- i.e. LocalTargets
func (bd *LocalTargetBuildAndDeployer) extract(specs []model.TargetSpec) []model.LocalTarget {
	var targs []model.LocalTarget
	for _, s := range specs {
		switch s := s.(type) {
		case model.LocalTarget:
			targs = append(targs, s)
		}
	}
	return targs
}

func (bd *LocalTargetBuildAndDeployer) run(ctx context.Context, c model.Cmd, wd string) error {
	if len(c.Argv) == 0 {
		return nil
	}

	l := logger.Get(ctx)
	writer := l.Writer(logger.InfoLvl)
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Stdout = writer
	cmd.Stderr = writer
	cmd.Dir = wd

	ps := build.NewPipelineState(ctx, 1, bd.clock)
	ps.StartPipelineStep(ctx, "Running command: %v (in %q)", c.Argv, wd)
	defer ps.EndPipelineStep(ctx)
	err := cmd.Run()
	defer func() { ps.End(ctx, err) }()
	if err != nil {
		// TODO(maia): any point in checking if it's an ExitError,
		//   pulling out the error code, etc.?
		return err
	}

	return nil
}

func (bd *LocalTargetBuildAndDeployer) successfulBuildResult(t model.LocalTarget) store.BuildResultSet {
	br := store.NewLocalBuildResult(t.ID())
	return store.BuildResultSet{t.ID(): br}
}

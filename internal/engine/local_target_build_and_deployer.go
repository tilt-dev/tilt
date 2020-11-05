package engine

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
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

	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.local", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	targ := targets[0]
	err = bd.run(ctx, targ.UpdateCmd, targ.Workdir)
	if err != nil {
		// (Never fall back from the LocalTargetBaD, none of our other BaDs can handle this target)
		return store.BuildResultSet{}, buildcontrol.DontFallBackErrorf("Command %q failed: %v", targ.UpdateCmd.String(), err)
	}

	// HACK(maia) Suppose target A modifies file X and target B depends on file X.
	//
	// Consider this sequence:
	//
	// 1. A starts
	// 2. A modifies X at time T1
	// 3. A modifies X at time T2
	// 4. A finishes
	// 5. B starts, caused by change at T1
	// 6. Tilt observes change at T2
	// 7. B finishes building
	// 8. B builds again, because the change at T2 was observed after the first build started.
	//
	// Empirically, this sleep ensures that any local file changes are processed
	// before the next build starts.
	//
	// At the moment (2020-01-31), local_resources will not build in parallel with
	// other resources by default, so this works fine.
	//
	// Possible approaches for a better system:
	//
	// - Use mtimes rather than our own internal modification tracking
	//   for determining dirtiness. Here is some good discussion of this approach:
	//   https://github.com/ninja-build/ninja/blob/master/src/deps_log.h#L29
	//   https://apenwarr.ca/log/20181113
	//   which has a lot of caveats, but you can boil it down to "using mtimes can
	//   make things a lot more efficient, but be careful how you use them"
	//
	// - Make a "dummy" change to the file system and make sure it propagates
	//   through the watch system before we start the next build (like fsync() does
	//   in our watch tests).
	time.Sleep(250 * time.Millisecond)

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

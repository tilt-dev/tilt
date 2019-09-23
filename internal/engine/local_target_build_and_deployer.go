package engine

import (
	"context"
	"os/exec"

	"github.com/windmilleng/tilt/internal/build"
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
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf(
			"LocalTargetBuildAndDeployer requires exactly one LocalTarget (got %d)", len(targets))
	}

	targ := targets[0]
	err = bd.run(ctx, targ.Cmd)
	if err != nil {
		// (Never fall back from the LocalTargetBaD, none of our other BaDs can handle this target)
		return store.BuildResultSet{}, DontFallBackErrorf("Command %q failed: %v", targ.Cmd.String(), err)
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

func (bd *LocalTargetBuildAndDeployer) run(ctx context.Context, c model.Cmd) error {
	if len(c.Argv) == 0 {
		panic("LocalTargetBuildAndDeployer tried to run empty command " +
			"(should have been caught by Validate() )")
	}

	l := logger.Get(ctx)
	writer := l.Writer(logger.InfoLvl)
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Stdout = writer
	cmd.Stderr = writer

	ps := build.NewPipelineState(ctx, 1, bd.clock)
	ps.StartPipelineStep(ctx, "Running command: %v", c.Argv)
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
	br := store.BuildResult{TargetID: t.ID()}
	return store.BuildResultSet{t.ID(): br}
}

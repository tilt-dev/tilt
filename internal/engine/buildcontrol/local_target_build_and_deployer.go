package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ BuildAndDeployer = &LocalTargetBuildAndDeployer{}

// TODO(maia): CommandRunner interface for testability
type LocalTargetBuildAndDeployer struct {
	clock      build.Clock
	ctrlClient ctrlclient.Client
	cmds       *cmd.Controller
}

func NewLocalTargetBuildAndDeployer(
	c build.Clock,
	ctrlClient ctrlclient.Client,
	cmds *cmd.Controller) *LocalTargetBuildAndDeployer {
	return &LocalTargetBuildAndDeployer{
		clock:      c,
		ctrlClient: ctrlClient,
		cmds:       cmds,
	}
}

func (bd *LocalTargetBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	targets := bd.extract(specs)
	if len(targets) != 1 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf(
			"LocalTargetBuildAndDeployer requires exactly one LocalTarget (got %d)", len(targets))
	}

	targ := targets[0]
	if targ.UpdateCmdSpec == nil {
		// Even if a LocalResource has no update command, we push it through the build-and-deploy
		// pipeline so that it gets all the appropriate logs.
		return bd.successfulBuildResult(targ), nil
	}

	startTime := time.Now()
	defer func() {
		analytics.Get(ctx).Timer("build.local", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	var cmd v1alpha1.Cmd
	err = bd.ctrlClient.Get(ctx, types.NamespacedName{Name: targ.UpdateCmdName()}, &cmd)
	if err != nil {
		return store.BuildResultSet{}, DontFallBackErrorf("Loading command: %v", err)
	}

	status, err := bd.cmds.ForceRun(ctx, &cmd)
	if err != nil {
		// (Never fall back from the LocalTargetBaD, none of our other BaDs can handle this target)
		return store.BuildResultSet{}, DontFallBackErrorf("Command %q failed: %v",
			model.ArgListToString(cmd.Spec.Args), err)
	} else if status.Terminated == nil {
		return store.BuildResultSet{}, DontFallBackErrorf("Command didn't terminate")
	} else if status.Terminated.ExitCode != 0 {
		return store.BuildResultSet{}, DontFallBackErrorf("Command %q failed: %v",
			model.ArgListToString(cmd.Spec.Args), status.Terminated.Reason)
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
		if s, ok := s.(model.LocalTarget); ok {
			targs = append(targs, s)
		}
	}
	return targs
}

func (bd *LocalTargetBuildAndDeployer) successfulBuildResult(t model.LocalTarget) store.BuildResultSet {
	br := store.NewLocalBuildResult(t.ID())
	return store.BuildResultSet{t.ID(): br}
}

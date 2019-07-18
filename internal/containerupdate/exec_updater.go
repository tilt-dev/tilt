package containerupdate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/k8s"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ExecUpdater struct {
	kCli k8s.Client
}

var _ ContainerUpdater = &ExecUpdater{}

func NewExecUpdater(kCli k8s.Client) ContainerUpdater {
	return &ExecUpdater{kCli: kCli}
}

func (cu *ExecUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ExecUpdater-UpdateContainer")
	defer span.Finish()

	// TODO(maia): put this in ValidUpdate() or whatever.
	if !hotReload {
		// If we're using kubectl exec syncing, it implies a non-Docker runtime,
		// which probably doesn't support container restart. User will have to use
		// our wrapper script to hack around it.
		return fmt.Errorf("your container runtime does not support the `restart_container()` step. " +
			"For a workaround, see https://github.com/windmilleng/rerun-process-wrapper")
	}

	l := logger.Get(ctx)
	w := l.Writer(logger.InfoLvl)

	if len(filesToDelete) > 0 {
		err := cu.kCli.Exec(ctx,
			deployInfo.PodID, deployInfo.ContainerName, deployInfo.Namespace,
			append([]string{"rm", "-rf"}, filesToDelete...), nil, w, w)
		if err != nil {
			return err
		}
	}

	err := cu.kCli.Exec(ctx, deployInfo.PodID, deployInfo.ContainerName, deployInfo.Namespace,
		[]string{"tar", "-C", "/", "-x", "-v", "-f", "-"}, archiveToCopy, w, w)
	if err != nil {
		return err
	}

	for i, c := range cmds {
		l.Infof("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		err := cu.kCli.Exec(ctx, deployInfo.PodID, deployInfo.ContainerName, deployInfo.Namespace,
			c.Argv, nil, w, w)
		if err != nil {
			return err
		}

	}

	return nil
}

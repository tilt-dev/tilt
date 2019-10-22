package containerupdate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type ExecUpdater struct {
	kCli k8s.Client
}

var _ ContainerUpdater = &ExecUpdater{}

func NewExecUpdater(kCli k8s.Client) *ExecUpdater {
	return &ExecUpdater{kCli: kCli}
}

func (cu *ExecUpdater) UpdateContainer(ctx context.Context, cInfo store.ContainerInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ExecUpdater-UpdateContainer")
	defer span.Finish()

	if !hotReload {
		return fmt.Errorf("ExecUpdater does not support `restart_container()` step. If you ran Tilt " +
			"with `--updateMode=exec`, omit this flag. If you are using a non-Docker container runtime, " +
			"see https://github.com/windmilleng/rerun-process-wrapper for a workaround")
	}

	l := logger.Get(ctx)
	w := logger.Get(ctx).Writer(logger.InfoLvl)

	if len(filesToDelete) > 0 {
		err := cu.kCli.Exec(ctx,
			cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
			append([]string{"rm", "-rf"}, filesToDelete...), nil, w, w)
		if err != nil {
			return err
		}
	}

	err := cu.kCli.Exec(ctx, cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
		[]string{"tar", "-C", "/", "-x", "-f", "-"}, archiveToCopy, w, w)
	if err != nil {
		return err
	}

	for i, c := range cmds {
		l.Infof("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		err := cu.kCli.Exec(ctx, cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
			c.Argv, nil, w, w)
		if err != nil {
			return build.WrapCodeExitError(err, cInfo.ContainerID, c)
		}

	}

	return nil
}

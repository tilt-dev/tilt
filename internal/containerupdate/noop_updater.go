package containerupdate

import (
	"context"
	"io"

	"github.com/opentracing/opentracing-go"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type NoopUpdater struct{}

var _ ContainerUpdater = NoopUpdater{}

func NewNoopUpdater() ContainerUpdater {
	return NoopUpdater{}
}

func (cu NoopUpdater) CanUpdateSpecs(specs []model.TargetSpec, env k8s.Env) (canUpd bool, msg string, silent bool) {
	// TODO(maia): implement
	// remember: if you get docker compose specs, error, we can't handle them -- should run with UpdateMode: Container
	return true, "", false
}

func (cu NoopUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ExecUpdater-UpdateContainer")
	defer span.Finish()
	return nil
}

package engine

import (
	"context"
	"errors"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"

	"io"
)

func UpService(ctx context.Context, buildAndDeployer BuildAndDeployer, service model.Service, watchMounts bool, stdout io.Writer, stderr io.Writer) error {
	bad, err := NewLocalBuildAndDeployer()
	if err != nil {
		return err
	}

	buildToken, err := bad.BuildAndDeploy(ctx, service, nil)
	if err != nil {
		return err
	}

	if watchMounts {
		watcher, err := watch.NewWatcher()
		defer func() {
			err := watcher.Close()
			if err != nil {
				logger.Get(ctx).Info(err.Error())
			}
		}()

		if err != nil {
			return err
		}

		if len(service.Mounts) == 0 {
			return errors.New("service has 0 repos - nothing to watch")
		}

		for _, mount := range service.Mounts {
			watcher.Add(mount.Repo.LocalPath)
		}

		for {
			select {
			case err := <-watcher.Errors():
				return err
			case <-watcher.Events():
				logger.Get(ctx).Verbose("file changed, rebuilding %v", service.Name)
				buildToken, err = bad.BuildAndDeploy(ctx, service, buildToken)
				if err != nil {
					return err
				}
			}
		}
	}
	return err
}

package engine

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
	"github.com/windmilleng/tilt/internal/watch"
)

type Upper struct {
	b            BuildAndDeployer
	watcherMaker func() (watch.Notify, error)
}

func NewUpper(manager service.Manager) (Upper, error) {
	b, err := NewLocalBuildAndDeployer(manager)
	if err != nil {
		return Upper{}, err
	}
	watcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}
	return Upper{b, watcherMaker}, nil
}

func (u Upper) Up(ctx context.Context, service model.Service, watchMounts bool, stdout io.Writer, stderr io.Writer) error {
	buildToken, err := u.b.BuildAndDeploy(ctx, service, nil, nil)
	if err != nil {
		return err
	}

	if watchMounts {
		watcher, err := u.watcherMaker()
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
			// TODO(matt) buffer events a bit so that we're not triggering 10 builds when you change branches
			select {
			case err := <-watcher.Errors():
				return err
			case event := <-watcher.Events():
				logger.Get(ctx).Info("file changed, rebuilding %v", service.Name)
				path, err := filepath.Abs(event.Name)
				if err != nil {
					return err
				}
				buildToken, err = u.b.BuildAndDeploy(ctx, service, buildToken, []string{path})
				if err != nil {
					logger.Get(ctx).Info("build failed: %v", err.Error())
				}
			}
		}
	}
	return err
}

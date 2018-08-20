package engine

import (
	"context"
	"errors"
	"io"
	"log"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/git"
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

		var repoRoots []string

		for _, mount := range service.Mounts {
			repoRoots = append(repoRoots, mount.Repo.LocalPath)
			err = watcher.Add(mount.Repo.LocalPath)
			if err != nil {
				return err
			}
		}

		eventFilter, err := git.NewMultiRepoIgnoreTester(repoRoots)
		if err != nil {
			return err
		}

		for {
			// TODO(matt) buffer events a bit so that we're not triggering 10 builds when you change branches
			select {
			case err := <-watcher.Errors():
				return err
			case event := <-watcher.Events():
				log.Printf("observed filechange. kicking off rebuild.")
				path, err := filepath.Abs(event.Name)
				if err != nil {
					return err
				}
				isIgnored, err := eventFilter.IsIgnored(path, false)
				if err != nil {
					return err
				}
				if !isIgnored {
					logger.Get(ctx).Info("file changed, rebuilding %v", service.Name)
					buildToken, err = u.b.BuildAndDeploy(ctx, service, buildToken, []string{path})
					if err != nil {
						logger.Get(ctx).Info("build failed: %v", err.Error())
					}
				}
			}
		}
	}
	return err
}

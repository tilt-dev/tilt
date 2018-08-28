package containerized

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// AtomicImageUpdate will perform an update of the given container with the new image
// and verify success via the provided healthcheckFn.  If the healthcheck fails, the
// container will be reverted to the prior image
func AtomicImageUpdate(ctx context.Context, container containerd.Container, image containerd.Image, healthcheckFn func() error) error {
	updateCompleted := false
	err := pauseAndRun(ctx, container, func() error {
		if err := container.Update(ctx, WithUpgrade(image)); err != nil {
			return errors.Wrap(err, "failed to update to new image")
		}
		updateCompleted = true
		task, err := container.Task(ctx, nil)
		if err != nil {
			if errdefs.IsNotFound(err) {
				return nil
			}
			return errors.Wrap(err, "failed to lookup task")
		}
		return task.Kill(ctx, sigTERM)
	})
	if err != nil {
		if updateCompleted {
			logrus.WithError(err).Error("failed to update, rolling back")
			return rollBack(ctx, container)
		}
		return err
	}
	if err := healthcheckFn(); err != nil {
		logrus.WithError(err).Error("failed health check, rolling back")
		return rollBack(ctx, container)
	}
	return nil
}

func rollBack(ctx context.Context, container containerd.Container) error {
	return pauseAndRun(ctx, container, func() error {
		if err := container.Update(ctx, WithRollback); err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			if errdefs.IsNotFound(err) {
				return nil
			}
			return errors.Wrap(err, "failed to lookup task")
		}
		return task.Kill(ctx, sigTERM)
	})
}

func pauseAndRun(ctx context.Context, container containerd.Container, fn func() error) error {
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return fn()
		}
		return errors.Wrap(err, "failed to lookup task")
	}
	if err := task.Pause(ctx); err != nil {
		return errors.Wrap(err, "failed to pause task")
	}
	defer task.Resume(ctx)
	return fn()
}

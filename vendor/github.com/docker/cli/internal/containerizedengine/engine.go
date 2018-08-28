package containerizedengine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime/restart"
	"github.com/docker/cli/internal/pkg/containerized"
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

// InitEngine is the main entrypoint for `docker engine init`
func (c baseClient) InitEngine(ctx context.Context, opts EngineInitOptions, out OutStream,
	authConfig *types.AuthConfig, healthfn func(context.Context) error) error {

	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	// Verify engine isn't already running
	_, err := c.GetEngine(ctx)
	if err == nil {
		return ErrEngineAlreadyPresent
	} else if err != ErrEngineNotPresent {
		return err
	}

	imageName := fmt.Sprintf("%s/%s:%s", opts.RegistryPrefix, opts.EngineImage, opts.EngineVersion)
	// Look for desired image
	_, err = c.cclient.GetImage(ctx, imageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			_, err = c.pullWithAuth(ctx, imageName, out, authConfig)
			if err != nil {
				return errors.Wrapf(err, "unable to pull image %s", imageName)
			}
		} else {
			return errors.Wrapf(err, "unable to check for image %s", imageName)
		}
	}

	// Spin up the engine
	err = c.startEngineOnContainerd(ctx, imageName, opts.ConfigFile)
	if err != nil {
		return errors.Wrap(err, "failed to create docker daemon")
	}

	// Wait for the daemon to start, verify it's responsive
	fmt.Fprintf(out, "Waiting for engine to start... ")
	ctx, cancel := context.WithTimeout(ctx, engineWaitTimeout)
	defer cancel()
	if err := c.waitForEngine(ctx, out, healthfn); err != nil {
		// TODO once we have the logging strategy sorted out
		// this should likely gather the last few lines of logs to report
		// why the daemon failed to initialize
		return errors.Wrap(err, "failed to start docker daemon")
	}
	fmt.Fprintf(out, "Success!  The docker engine is now running.\n")

	return nil

}

// GetEngine will return the containerd container running the engine (or error)
func (c baseClient) GetEngine(ctx context.Context) (containerd.Container, error) {
	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	containers, err := c.cclient.Containers(ctx, "id=="+engineContainerName)
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, ErrEngineNotPresent
	}
	return containers[0], nil
}

// getEngineImage will return the current image used by the engine
func (c baseClient) getEngineImage(engine containerd.Container) (string, error) {
	ctx := namespaces.WithNamespace(context.Background(), engineNamespace)
	image, err := engine.Image(ctx)
	if err != nil {
		return "", err
	}
	return image.Name(), nil
}

// getEngineConfigFilePath will extract the config file location from the engine flags
func (c baseClient) getEngineConfigFilePath(ctx context.Context, engine containerd.Container) (string, error) {
	spec, err := engine.Spec(ctx)
	configFile := ""
	if err != nil {
		return configFile, err
	}
	for i := 0; i < len(spec.Process.Args); i++ {
		arg := spec.Process.Args[i]
		if strings.HasPrefix(arg, "--config-file") {
			if strings.Contains(arg, "=") {
				split := strings.SplitN(arg, "=", 2)
				configFile = split[1]
			} else {
				if i+1 >= len(spec.Process.Args) {
					return configFile, ErrMalformedConfigFileParam
				}
				configFile = spec.Process.Args[i+1]
			}
		}
	}

	if configFile == "" {
		// TODO - any more diagnostics to offer?
		return configFile, ErrEngineConfigLookupFailure
	}
	return configFile, nil
}

var (
	engineWaitInterval = 500 * time.Millisecond
	engineWaitTimeout  = 60 * time.Second
)

// waitForEngine will wait for the engine to start
func (c baseClient) waitForEngine(ctx context.Context, out io.Writer, healthfn func(context.Context) error) error {
	ticker := time.NewTicker(engineWaitInterval)
	defer ticker.Stop()
	defer func() {
		fmt.Fprintf(out, "\n")
	}()

	err := c.waitForEngineContainer(ctx, ticker)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "waiting for engine to be responsive... ")
	for {
		select {
		case <-ticker.C:
			err = healthfn(ctx)
			if err == nil {
				fmt.Fprintf(out, "engine is online.")
				return nil
			}
		case <-ctx.Done():
			return errors.Wrap(err, "timeout waiting for engine to be responsive")
		}
	}
}

func (c baseClient) waitForEngineContainer(ctx context.Context, ticker *time.Ticker) error {
	var ret error
	for {
		select {
		case <-ticker.C:
			engine, err := c.GetEngine(ctx)
			if engine != nil {
				return nil
			}
			ret = err
		case <-ctx.Done():
			return errors.Wrap(ret, "timeout waiting for engine to be responsive")
		}
	}
}

// RemoveEngine gracefully unwinds the current engine
func (c baseClient) RemoveEngine(ctx context.Context, engine containerd.Container) error {
	ctx = namespaces.WithNamespace(ctx, engineNamespace)

	// Make sure the container isn't being restarted while we unwind it
	stopLabel := map[string]string{}
	stopLabel[restart.StatusLabel] = string(containerd.Stopped)
	engine.SetLabels(ctx, stopLabel)

	// Wind down the existing engine
	task, err := engine.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
	} else {
		status, err := task.Status(ctx)
		if err != nil {
			return err
		}
		if status.Status == containerd.Running {
			// It's running, so kill it
			err := task.Kill(ctx, syscall.SIGTERM, []containerd.KillOpts{}...)
			if err != nil {
				return errors.Wrap(err, "task kill error")
			}

			ch, err := task.Wait(ctx)
			if err != nil {
				return err
			}
			timeout := time.NewTimer(engineWaitTimeout)
			select {
			case <-timeout.C:
				// TODO - consider a force flag in the future to allow a more aggressive
				// kill of the engine via
				// task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll)
				return ErrEngineShutdownTimeout
			case <-ch:
			}
		}
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
	}
	deleteOpts := []containerd.DeleteOpts{containerd.WithSnapshotCleanup}
	err = engine.Delete(ctx, deleteOpts...)
	if err != nil && errdefs.IsNotFound(err) {
		return nil
	}
	return errors.Wrap(err, "failed to remove existing engine container")
}

// startEngineOnContainerd creates a new docker engine running on containerd
func (c baseClient) startEngineOnContainerd(ctx context.Context, imageName, configFile string) error {
	ctx = namespaces.WithNamespace(ctx, engineNamespace)
	image, err := c.cclient.GetImage(ctx, imageName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return fmt.Errorf("engine image missing: %s", imageName)
		}
		return errors.Wrap(err, "failed to check for engine image")
	}

	// Make sure we have a valid config file
	err = c.verifyDockerConfig(configFile)
	if err != nil {
		return err
	}

	engineSpec.Process.Args = append(engineSpec.Process.Args,
		"--config-file", configFile,
	)

	cOpts := []containerd.NewContainerOpts{
		containerized.WithNewSnapshot(image),
		restart.WithStatus(containerd.Running),
		restart.WithLogPath("/var/log/engine.log"), // TODO - better!
		genSpec(),
		containerd.WithRuntime("io.containerd.runtime.process.v1", nil),
	}

	_, err = c.cclient.NewContainer(
		ctx,
		engineContainerName,
		cOpts...,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create engine container")
	}

	return nil
}

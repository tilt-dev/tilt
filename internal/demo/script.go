package demo

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type RepoBranch string

// Runs the demo script
type Script struct {
	hud     hud.HeadsUpDisplay
	upper   engine.Upper
	store   *store.Store
	env     k8s.Env
	kClient k8s.Client
	branch  RepoBranch
	runtime container.Runtime
	tfl     tiltfile.TiltfileLoader

	readTiltfileCh chan string
	podMonitor     *podMonitor
}

func NewScript(upper engine.Upper, hud hud.HeadsUpDisplay, kClient k8s.Client,
	env k8s.Env, st *store.Store, branch RepoBranch, runtime container.Runtime,
	tfl tiltfile.TiltfileLoader) Script {
	s := Script{
		upper:          upper,
		hud:            hud,
		env:            env,
		kClient:        kClient,
		branch:         branch,
		readTiltfileCh: make(chan string),
		podMonitor:     &podMonitor{},
		store:          st,
		runtime:        runtime,
		tfl:            tfl,
	}
	st.AddSubscriber(s.podMonitor)
	return s
}

type podMonitor struct {
	hasBuildError bool
	hasPodRestart bool
	healthy       bool
	mu            sync.Mutex
}

func (m *podMonitor) OnChange(ctx context.Context, st store.RStore) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := st.RLockState()
	defer st.RUnlockState()

	m.hasPodRestart = false
	m.hasBuildError = false
	m.healthy = true

	if len(state.ManifestTargets) == 0 {
		m.healthy = false
	}

	if state.CurrentlyBuilding != "" {
		m.healthy = false
	}

	for _, ms := range state.ManifestStates() {
		pod := ms.MostRecentPod()
		if pod.Phase != v1.PodRunning {
			m.healthy = false
		}

		if pod.ContainerRestarts > 0 {
			m.hasPodRestart = true
			m.healthy = false
		}

		if ms.LastBuild().Error != nil {
			m.hasBuildError = true
			m.healthy = false
		}

		for _, status := range ms.BuildStatuses {
			if len(status.PendingFileChanges) > 0 {
				m.healthy = false
			}
		}
	}

}

func (m *podMonitor) waitUntilPodsReady(ctx context.Context) error {
	return m.waitUntilCond(ctx, func() bool {
		return m.healthy
	})
}

func (m *podMonitor) waitUntilBuildError(ctx context.Context) error {
	return m.waitUntilCond(ctx, func() bool {
		return m.hasBuildError
	})
}

func (m *podMonitor) waitUntilPodRestart(ctx context.Context) error {
	return m.waitUntilCond(ctx, func() bool {
		return m.hasPodRestart
	})
}

func (m *podMonitor) waitUntilCond(ctx context.Context, f func() bool) error {
	for {
		m.mu.Lock()
		cond := f()
		m.mu.Unlock()
		if cond {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s Script) Run(ctx context.Context) error {
	if !s.env.IsLocalCluster() {
		_, _ = fmt.Fprintf(os.Stderr, "tilt demo mode only supports Docker For Mac, Minikube, and MicroK8s\n")
		_, _ = fmt.Fprintf(os.Stderr, "check your current cluster with:\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nkubectl config get-contexts\n\n")
		return nil
	} else if s.runtime != container.RuntimeDocker {
		_, _ = fmt.Fprintf(os.Stderr, "tilt demo mode only supports clusters configured with docker\n")
		_, _ = fmt.Fprintf(os.Stderr, "Current container runtime: %s\n", s.runtime)
		return nil
	}

	l := engine.NewLogActionLogger(ctx, s.store.Dispatch)
	out := l.Writer(logger.InfoLvl)
	ctx = logger.WithLogger(ctx, l)
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer cancel()
		return s.hud.Run(ctx, s.upper.Dispatch, hud.DefaultRefreshInterval)
	})

	g.Go(func() error {
		defer cancel()
		return s.runSteps(ctx, out)
	})

	g.Go(func() error {
		defer cancel()
		var dir string
		select {
		case dir = <-s.readTiltfileCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		tfPath := filepath.Join(dir, tiltfile.FileName)
		// TODO(dmiller): not this?
		tlr, err := s.tfl.Load(ctx, tfPath, nil)
		if err != nil {
			return err
		}

		defer s.cleanUp(newBackgroundContext(ctx), tlr.Manifests)

		initAction := engine.InitAction{
			WatchMounts:  true,
			Manifests:    tlr.Manifests,
			TiltfilePath: tfPath,
			Warnings:     tlr.Warnings,
		}
		return s.upper.Init(ctx, initAction)
	})

	return g.Wait()
}

func newBackgroundContext(ctx context.Context) context.Context {
	l := logger.Get(ctx)
	return logger.WithLogger(context.Background(), l)
}

func (s Script) cleanUp(ctx context.Context, manifests []model.Manifest) {
	if manifests == nil {
		return
	}

	entities, err := engine.ParseYAMLFromManifests(manifests...)
	if err != nil {
		logger.Get(ctx).Infof("Parsing entities: %v", err)
		return
	}

	err = s.kClient.Delete(ctx, entities)
	if err != nil {
		logger.Get(ctx).Infof("Deleting entities: %v", err)
	}
}

func (s Script) runSteps(ctx context.Context, out io.Writer) error {
	tmpDir, err := ioutil.TempDir("", "tiltdemo")
	if err != nil {
		return errors.Wrap(err, "demo.runSteps")
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	for _, step := range steps {
		if step.ChangeBranch && s.branch == "" {
			continue
		}

		err := s.hud.SetNarrationMessage(ctx, step.Narration)
		if err != nil {
			return err
		}

		if step.Command != "" {
			cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)
			cmd.Stdout = out
			cmd.Stderr = out
			cmd.Dir = tmpDir
			err := cmd.Run()
			if err != nil {
				return errors.Wrap(err, "demo.runSteps")
			}
		} else if step.CreateManifests {
			s.readTiltfileCh <- tmpDir
		} else if step.ChangeBranch {
			cmd := exec.CommandContext(ctx, "git", "checkout", string(s.branch))
			cmd.Stdout = out
			cmd.Stderr = out
			cmd.Dir = tmpDir
			err := cmd.Run()
			if err != nil {
				return errors.Wrap(err, "demo.runSteps")
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(Pause):
		}

		if step.WaitForHealthy {
			_ = s.podMonitor.waitUntilPodsReady(ctx)
			continue
		} else if step.WaitForBuildError {
			_ = s.podMonitor.waitUntilBuildError(ctx)
			continue
		} else if step.WaitForPodRestart {
			_ = s.podMonitor.waitUntilPodRestart(ctx)
			continue
		}
	}

	return nil
}

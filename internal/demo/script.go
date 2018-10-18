package demo

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/client"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"golang.org/x/sync/errgroup"
	"k8s.io/api/core/v1"
)

type RepoBranch string

// Runs the demo script
type Script struct {
	hud    hud.HeadsUpDisplay
	upper  engine.Upper
	store  *store.Store
	env    k8s.Env
	branch RepoBranch

	readTiltfileCh chan string
	podMonitor     *podMonitor
}

func NewScript(upper engine.Upper, hud hud.HeadsUpDisplay, env k8s.Env, st *store.Store, branch RepoBranch) Script {
	s := Script{
		upper:          upper,
		hud:            hud,
		env:            env,
		branch:         branch,
		readTiltfileCh: make(chan string),
		podMonitor:     &podMonitor{podsReadyCh: make(chan bool)},
		store:          st,
	}
	st.AddSubscriber(s.podMonitor)
	return s
}

type podMonitor struct {
	podsReady   bool
	podsReadyCh chan bool
}

func (m *podMonitor) arePodsReady(ctx context.Context, store *store.Store) bool {
	state := store.RLockState()
	defer store.RUnlockState()
	hasPods := false
	allPodsReady := true
	for _, ms := range state.ManifestStates {
		if ms.Pod.PodID != "" {
			hasPods = true
		}

		if ms.Pod.Phase != v1.PodRunning {
			allPodsReady = false
		}
	}
	return hasPods && allPodsReady
}

func (m *podMonitor) OnChange(ctx context.Context, store *store.Store) {
	podsReady := m.arePodsReady(ctx, store)
	if podsReady != m.podsReady {
		m.podsReady = podsReady
		m.podsReadyCh <- podsReady
	}
}

func (m *podMonitor) waitUntilPodsReady(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ready := <-m.podsReadyCh:
			if ready {
				return nil
			}
		}
	}
}

func (s Script) Run(ctx context.Context) error {
	if !s.env.IsLocalCluster() {
		_, _ = fmt.Fprintf(os.Stderr, "tilt demo mode only supports Docker For Mac or Minikube\n")
		_, _ = fmt.Fprintf(os.Stderr, "check your current cluster with:\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nkubectl config get-contexts\n\n")
		return nil
	}

	// Discard all the logs. Uncomment the line below to make debugging easier.
	out := ioutil.Discard
	//out, _ = os.OpenFile("log.txt", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.FileMode(0644))

	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = output.WithOutputter(
		logger.WithLogger(ctx, l),
		output.NewOutputter(l))
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer cancel()
		return s.hud.Run(ctx, s.store, hud.DefaultRefreshInterval)
	})

	g.Go(func() error {
		defer cancel()
		return client.ConnectHud(ctx)
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

		file := filepath.Join(dir, tiltfile.FileName)
		tf, err := tiltfile.Load(file, out)
		if err != nil {
			return err
		}

		manifests, err := tf.GetManifestConfigs("tiltdemo")
		if err != nil {
			return err
		}

		return s.upper.CreateManifests(ctx, manifests, true)
	})

	return g.Wait()
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

		s.hud.SetNarrationMessage(ctx, step.Narration)

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
			_ = s.podMonitor.waitUntilPodsReady(ctx)
			continue
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
	}

	return nil
}

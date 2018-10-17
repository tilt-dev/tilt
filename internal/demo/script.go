package demo

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/client"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"golang.org/x/sync/errgroup"
)

// Runs the demo script
type Script struct {
	hud   hud.HeadsUpDisplay
	upper engine.Upper
	store *store.Store
	env   k8s.Env

	ready chan bool
}

func NewScript(upper engine.Upper, hud hud.HeadsUpDisplay, env k8s.Env, st *store.Store) Script {
	s := Script{
		upper: upper,
		hud:   hud,
		env:   env,
		ready: make(chan bool),
		store: st,
	}
	st.AddSubscriber(s)
	return s
}

func (s Script) OnChange(ctx context.Context, store *store.Store) {
}

func (s Script) Run(ctx context.Context) error {
	if !s.env.IsLocalCluster() {
		_, _ = fmt.Fprintf(os.Stderr, "tilt demo mode only supports docker-for-desktop or Minikube\n")
		_, _ = fmt.Fprintf(os.Stderr, "check your current cluster with:\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nkubectl config get-contexts\n\n")
		return nil
	}

	// Discard all the logs
	l := logger.NewLogger(logger.DebugLvl, ioutil.Discard)
	ctx = output.WithOutputter(
		logger.WithLogger(ctx, l),
		output.NewOutputter(l))
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	s.hud.SetNarrationMessage(ctx, "\t🚀  Launching demo... ")

	g.Go(func() error {
		defer cancel()
		return s.hud.Run(ctx, s.store, hud.DefaultRefreshInterval)
	})

	g.Go(func() error {
		defer cancel()
		return client.ConnectHud(ctx)
	})

	return g.Wait()
}

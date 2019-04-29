package server

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/store"
)

type HeadsUpServerController struct {
	port        model.WebPort
	hudServer   HeadsUpServer
	assetServer assets.Server
	initDone    bool
}

func ProvideHeadsUpServerController(port model.WebPort, hudServer HeadsUpServer, assetServer assets.Server) *HeadsUpServerController {
	return &HeadsUpServerController{
		port:        port,
		hudServer:   hudServer,
		assetServer: assetServer,
	}
}

func (s *HeadsUpServerController) Teardown(ctx context.Context) {
	s.assetServer.Teardown(ctx)
}

func (s *HeadsUpServerController) OnChange(ctx context.Context, st store.RStore) {
	defer func() {
		s.initDone = true
	}()

	if s.initDone || s.port == 0 {
		return
	}

	err := network.IsBindAddrFree(network.LocalhostBindAddr(int(s.port)))
	if err != nil {
		st.Dispatch(
			store.NewErrorAction(
				errors.Wrapf(err, "Cannot start Tilt. Maybe another process is already running on port %d? Use --port to set a custom port", s.port)))
		return
	}

	httpServer := &http.Server{
		Addr:    network.LocalhostBindAddr(int(s.port)),
		Handler: http.DefaultServeMux,
	}
	http.Handle("/", s.hudServer.Router())

	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()

	go func() {
		err := s.assetServer.Serve(ctx)
		if err != nil && ctx.Err() == nil {
			st.Dispatch(store.NewErrorAction(err))
		}
	}()

	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed && ctx.Err() == nil {
			st.Dispatch(store.NewErrorAction(err))
		}
	}()
}

var _ store.SubscriberLifecycle = &HeadsUpServerController{}

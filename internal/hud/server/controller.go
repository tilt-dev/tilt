package server

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/store"
)

type HeadsUpServerController struct {
	port        model.WebPort
	hudServer   *HeadsUpServer
	assetServer assets.Server
	webURL      model.WebURL
	webLoadDone bool
	initDone    bool
}

func ProvideHeadsUpServerController(port model.WebPort, hudServer *HeadsUpServer, assetServer assets.Server, webURL model.WebURL) *HeadsUpServerController {
	return &HeadsUpServerController{
		port:        port,
		hudServer:   hudServer,
		assetServer: assetServer,
		webURL:      webURL,
	}
}

func (s *HeadsUpServerController) TearDown(ctx context.Context) {
	s.assetServer.TearDown(ctx)
}

func (s *HeadsUpServerController) maybeOpenBrowser(st store.RStore) {
	if s.webURL.Empty() || s.webLoadDone {
		return
	}

	connCount := atomic.LoadInt32(&(s.hudServer.numWebsocketConns))
	if connCount > 0 {
		// Don't auto-open the web view. It's already opened.
		s.webLoadDone = true
		return
	}

	state := st.RLockState()
	tiltfileCompleted := state.FirstTiltfileBuildCompleted
	st.RUnlockState()

	if tiltfileCompleted {
		// We should probably dependency-inject a browser opener.
		//
		// It might also make sense to wait until the asset server is ready?
		_ = browser.OpenURL(s.webURL.String())
		s.webLoadDone = true
	}
}

func (s *HeadsUpServerController) OnChange(ctx context.Context, st store.RStore) {
	s.maybeOpenBrowser(st)

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

var _ store.TearDowner = &HeadsUpServerController{}

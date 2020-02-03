package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/assets"
	"github.com/windmilleng/tilt/pkg/model"
)

// The amount of time to wait for a reconnection before restarting the browser
// window.
const reconnectDur = 2 * time.Second

type HeadsUpServerController struct {
	host        model.WebHost
	port        model.WebPort
	hudServer   *HeadsUpServer
	assetServer assets.Server
	webURL      model.WebURL
	webLoadDone bool
	initDone    bool
	noBrowser   model.NoBrowser
}

func ProvideHeadsUpServerController(host model.WebHost, port model.WebPort, hudServer *HeadsUpServer, assetServer assets.Server, webURL model.WebURL, noBrowser model.NoBrowser) *HeadsUpServerController {
	return &HeadsUpServerController{
		host:        host,
		port:        port,
		hudServer:   hudServer,
		assetServer: assetServer,
		webURL:      webURL,
		noBrowser:   noBrowser,
	}
}

func (s *HeadsUpServerController) TearDown(ctx context.Context) {
	s.assetServer.TearDown(ctx)
}

func (s *HeadsUpServerController) isWebsocketConnected() bool {
	connCount := atomic.LoadInt32(&(s.hudServer.numWebsocketConns))
	return connCount > 0
}

func (s *HeadsUpServerController) maybeOpenBrowser(st store.RStore) {
	if s.webURL.Empty() || s.webLoadDone || (bool)(s.noBrowser) {
		return
	}

	if s.isWebsocketConnected() {
		// Don't auto-open the web view. It's already opened.
		s.webLoadDone = true
		return
	}

	state := st.RLockState()
	tiltfileCompleted := !state.TiltfileState.LastBuild().Empty()
	startTime := state.TiltStartTime
	st.RUnlockState()

	// Only open the webview if the Tiltfile has completed.
	if tiltfileCompleted {
		s.webLoadDone = true

		// Make sure we wait at least `reconnectDur` before opening the browser, to
		// give any open pages time to reconnect. Do this on a goroutine so we don't
		// hold the lock.
		go func() {
			runDur := time.Since(startTime)
			if runDur < reconnectDur {
				time.Sleep(reconnectDur - runDur)
			}

			if s.isWebsocketConnected() {
				return
			}

			// We should probably dependency-inject a browser opener.
			//
			// It might also make sense to wait until the asset server is ready?
			_ = browser.OpenURL(s.webURL.String())
		}()
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

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", string(s.host), int(s.port)))
	if err != nil {
		st.Dispatch(
			store.NewErrorAction(
				errors.Wrapf(err, "Cannot start Tilt. Maybe another process is already running on port %d? Use --port to set a custom port", s.port)))
		return
	}

	httpServer := &http.Server{
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
		err := httpServer.Serve(l)
		if err != nil && err != http.ErrServerClosed && ctx.Err() == nil {
			st.Dispatch(store.NewErrorAction(err))
		}
	}()
}

var _ store.TearDowner = &HeadsUpServerController{}

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
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
	initDone    bool
}

func ProvideHeadsUpServerController(host model.WebHost, port model.WebPort, hudServer *HeadsUpServer, assetServer assets.Server, webURL model.WebURL) *HeadsUpServerController {
	return &HeadsUpServerController{
		host:        host,
		port:        port,
		hudServer:   hudServer,
		assetServer: assetServer,
		webURL:      webURL,
	}
}

func (s *HeadsUpServerController) TearDown(ctx context.Context) {
	s.assetServer.TearDown(ctx)
}

func (s *HeadsUpServerController) isWebsocketConnected() bool {
	connCount := atomic.LoadInt32(&(s.hudServer.numWebsocketConns))
	return connCount > 0
}

func (s *HeadsUpServerController) OnChange(ctx context.Context, st store.RStore) {
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

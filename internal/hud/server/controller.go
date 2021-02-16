package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

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
				fmt.Errorf("Tilt cannot start because you already have another process on port %d\n"+
					"If you want to run multiple Tilt instances simultaneously,\n"+
					"use the --port flag or TILT_PORT env variable to set a custom port\nOriginal error: %v",
					s.port, err)))
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

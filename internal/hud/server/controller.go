package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type HeadsUpServerController struct {
	port        model.WebPort
	hudServer   HeadsUpServer
	assetServer AssetServer
	initDone    bool
}

func ProvideHeadsUpServerController(port model.WebPort, hudServer HeadsUpServer, assetServer AssetServer) *HeadsUpServerController {
	return &HeadsUpServerController{
		port:        port,
		hudServer:   hudServer,
		assetServer: assetServer,
	}
}

func (s *HeadsUpServerController) OnChange(ctx context.Context, st store.RStore) {
	defer func() {
		s.initDone = true
	}()

	if s.initDone || s.port == 0 {
		return
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
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

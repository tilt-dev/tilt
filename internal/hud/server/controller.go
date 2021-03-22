package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

type HeadsUpServerController struct {
	port            model.WebPort
	hudServer       *HeadsUpServer
	assetServer     assets.Server
	apiServer       *http.Server
	webURL          model.WebURL
	apiServerConfig *APIServerConfig

	shutdown func()
}

func ProvideHeadsUpServerController(
	port model.WebPort,
	apiServerConfig *APIServerConfig,
	hudServer *HeadsUpServer,
	assetServer assets.Server,
	webURL model.WebURL) *HeadsUpServerController {

	emptyCh := make(chan struct{})
	close(emptyCh)

	return &HeadsUpServerController{
		port:            port,
		hudServer:       hudServer,
		assetServer:     assetServer,
		webURL:          webURL,
		apiServerConfig: apiServerConfig,
		shutdown:        func() {},
	}
}

func (s *HeadsUpServerController) TearDown(ctx context.Context) {
	s.shutdown()
	s.assetServer.TearDown(ctx)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	_ = s.apiServer.Shutdown(ctx)
}

func (s *HeadsUpServerController) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) {
}

// Merge the APIServer and the Tilt Web server into a single handler,
// and attach them both to the public listener.
func (s *HeadsUpServerController) SetUp(ctx context.Context, st store.RStore) error {
	ctx, cancel := context.WithCancel(ctx)
	s.shutdown = cancel

	err := s.setUpHelper(ctx, st)
	if err != nil {
		return fmt.Errorf("Cannot start the tilt-apiserver: %v", err)
	}
	return nil
}

func (s *HeadsUpServerController) setUpHelper(ctx context.Context, st store.RStore) error {
	stopCh := ctx.Done()
	config := s.apiServerConfig
	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	err = server.GenericAPIServer.AddPostStartHook("start-tilt-server-informers", func(context genericapiserver.PostStartHookContext) error {
		if config.GenericConfig.SharedInformerFactory != nil {
			config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		}
		return nil
	})
	if err != nil {
		return err
	}

	prepared := server.GenericAPIServer.PrepareRun()
	apiserverHandler := prepared.Handler
	serving := config.ExtraConfig.ServingInfo

	r := mux.NewRouter()
	r.Path("/api").Handler(http.NotFoundHandler())
	r.PathPrefix("/apis").Handler(apiserverHandler)
	r.PathPrefix("/healthz").Handler(apiserverHandler)
	r.PathPrefix("/livez").Handler(apiserverHandler)
	r.PathPrefix("/metrics").Handler(apiserverHandler)
	r.PathPrefix("/openapi").Handler(apiserverHandler)
	r.PathPrefix("/readyz").Handler(apiserverHandler)
	r.PathPrefix("/swagger").Handler(apiserverHandler)
	r.PathPrefix("/version").Handler(apiserverHandler)
	r.PathPrefix("/debug").Handler(http.DefaultServeMux) // for /debug/pprof
	r.PathPrefix("/").Handler(s.hudServer.Router())

	s.apiServer = &http.Server{
		Addr:           serving.Listener.Addr().String(),
		Handler:        r,
		MaxHeaderBytes: 1 << 20,
	}
	runServer(ctx, s.apiServer, serving.Listener)
	server.GenericAPIServer.RunPostStartHooks(stopCh)

	go func() {
		err := s.assetServer.Serve(ctx)
		if err != nil && ctx.Err() == nil {
			st.Dispatch(store.NewErrorAction(err))
		}
	}()

	return nil
}

var _ store.SetUpper = &HeadsUpServerController{}
var _ store.TearDowner = &HeadsUpServerController{}

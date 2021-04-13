package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

type HeadsUpServerController struct {
	host            model.WebHost
	port            model.WebPort
	hudServer       *HeadsUpServer
	assetServer     assets.Server
	apiServer       *http.Server
	webServer       *http.Server
	webURL          model.WebURL
	apiServerConfig *APIServerConfig

	shutdown func()
}

func ProvideHeadsUpServerController(
	host model.WebHost,
	port model.WebPort,
	apiServerConfig *APIServerConfig,
	hudServer *HeadsUpServer,
	assetServer assets.Server,
	webURL model.WebURL) *HeadsUpServerController {

	emptyCh := make(chan struct{})
	close(emptyCh)

	return &HeadsUpServerController{
		host:            host,
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

	// Close all active connections immediately.
	// Tilt is deleting all its state, so there's no good
	// reason to handle graceful shutdown.
	_ = s.webServer.Close()
	_ = s.apiServer.Close()
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

	apiRouter := mux.NewRouter()
	apiRouter.Path("/api").Handler(http.NotFoundHandler())
	apiRouter.PathPrefix("/apis").Handler(apiserverHandler)
	apiRouter.PathPrefix("/healthz").Handler(apiserverHandler)
	apiRouter.PathPrefix("/livez").Handler(apiserverHandler)
	apiRouter.PathPrefix("/metrics").Handler(apiserverHandler)
	apiRouter.PathPrefix("/openapi").Handler(apiserverHandler)
	apiRouter.PathPrefix("/readyz").Handler(apiserverHandler)
	apiRouter.PathPrefix("/swagger").Handler(apiserverHandler)
	apiRouter.PathPrefix("/version").Handler(apiserverHandler)
	apiRouter.PathPrefix("/debug").Handler(http.DefaultServeMux) // for /debug/pprof

	apiTLSConfig, err := start.TLSConfig(serving)
	if err != nil {
		return fmt.Errorf("Starting apiserver: %v", err)
	}

	webListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", string(s.host), int(s.port)))
	if err != nil {
		return fmt.Errorf("Tilt cannot start because you already have another process on port %d\n"+
			"If you want to run multiple Tilt instances simultaneously,\n"+
			"use the --port flag or TILT_PORT env variable to set a custom port\nOriginal error: %v",
			s.port, err)
	}

	webRouter := mux.NewRouter()
	webRouter.PathPrefix("/debug").Handler(http.DefaultServeMux) // for /debug/pprof
	webRouter.PathPrefix("/").Handler(s.hudServer.Router())

	s.webServer = &http.Server{
		Addr:    webListener.Addr().String(),
		Handler: webRouter,
	}
	runServer(ctx, s.webServer, webListener)

	s.apiServer = &http.Server{
		Addr:           serving.Listener.Addr().String(),
		Handler:        apiRouter,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      apiTLSConfig,
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

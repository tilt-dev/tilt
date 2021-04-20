package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
)

type HeadsUpServerController struct {
	configAccess    clientcmd.ConfigAccess
	apiServerName   model.APIServerName
	webListener     WebListener
	hudServer       *HeadsUpServer
	assetServer     assets.Server
	apiServer       *http.Server
	webServer       *http.Server
	webURL          model.WebURL
	apiServerConfig *APIServerConfig

	shutdown func()
}

func ProvideHeadsUpServerController(
	configAccess clientcmd.ConfigAccess,
	apiServerName model.APIServerName,
	webListener WebListener,
	apiServerConfig *APIServerConfig,
	hudServer *HeadsUpServer,
	assetServer assets.Server,
	webURL model.WebURL) *HeadsUpServerController {

	emptyCh := make(chan struct{})
	close(emptyCh)

	return &HeadsUpServerController{
		configAccess:    configAccess,
		apiServerName:   apiServerName,
		webListener:     webListener,
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

	_ = s.removeFromAPIServerConfig()
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
	err = s.addToAPIServerConfig()
	if err != nil {
		return fmt.Errorf("writing tilt api configs: %v", err)
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

	webRouter := mux.NewRouter()
	webRouter.PathPrefix("/debug").Handler(http.DefaultServeMux) // for /debug/pprof
	webRouter.PathPrefix("/").Handler(s.hudServer.Router())

	webListener := net.Listener(s.webListener)
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

// Write the API server configs into the user settings directory.
//
// Usually shows up as ~/.windmill/config or ~/.tilt-dev/config.
func (s *HeadsUpServerController) addToAPIServerConfig() error {
	newConfig, err := s.configAccess.GetStartingConfig()
	if err != nil {
		return err
	}
	newConfig = newConfig.DeepCopy()

	clientConfig := s.apiServerConfig.GenericConfig.LoopbackClientConfig
	if err := model.ValidateAPIServerName(s.apiServerName); err != nil {
		return err
	}

	name := string(s.apiServerName)
	newConfig.Contexts[name] = &clientcmdapi.Context{
		Cluster:  name,
		AuthInfo: name,
	}
	newConfig.AuthInfos[name] = &clientcmdapi.AuthInfo{
		Token: clientConfig.BearerToken,
	}

	newConfig.Clusters[name] = &clientcmdapi.Cluster{
		Server:                   clientConfig.Host,
		CertificateAuthorityData: clientConfig.TLSClientConfig.CAData,
	}

	return clientcmd.ModifyConfig(s.configAccess, *newConfig, true)
}

// Remove this API server's configs into the user settings directory.
//
// Usually shows up as ~/.windmill/config or ~/.tilt-dev/config.
func (s *HeadsUpServerController) removeFromAPIServerConfig() error {
	newConfig, err := s.configAccess.GetStartingConfig()
	if err != nil {
		return err
	}
	newConfig = newConfig.DeepCopy()
	if err := model.ValidateAPIServerName(s.apiServerName); err != nil {
		return err
	}

	name := string(s.apiServerName)
	delete(newConfig.Contexts, name)
	delete(newConfig.AuthInfos, name)
	delete(newConfig.Clusters, name)

	return clientcmd.ModifyConfig(s.configAccess, *newConfig, true)
}

var _ store.SetUpper = &HeadsUpServerController{}
var _ store.TearDowner = &HeadsUpServerController{}

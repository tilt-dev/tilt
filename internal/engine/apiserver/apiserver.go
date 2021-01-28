package apiserver

import (
	"context"
	"io/ioutil"

	"github.com/google/go-cmp/cmp"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/clientset/tiltapi"
	"github.com/tilt-dev/tilt/pkg/openapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The port where we'll serve the API endpoints.
// TODO(nick): It might make more sense to serve this on the same port
// as the other Tilt webserver, if we can reconcile them.
const Port = 10351

type TiltClientset = *tiltapi.Clientset

// Configure the Tilt API server.
func NewTiltServerOptions(ctx context.Context, tiltBuild model.TiltBuild) (*start.TiltServerOptions, error) {
	dir, err := ioutil.TempDir("", "tilt-data")
	if err != nil {
		return nil, err
	}

	w := logger.Get(ctx).Writer(logger.DebugLvl)
	builder := builder.APIServer.
		WithResourceFileStorage(&v1alpha1.Manifest{}, dir).
		WithOpenAPIDefinitions("tilt", tiltBuild.Version, openapi.GetOpenAPIDefinitions)
	codec, err := builder.BuildCodec()
	if err != nil {
		return nil, err
	}

	o := start.NewTiltServerOptions(w, w, codec)
	o.ServingOptions.BindPort = Port
	err = o.Complete()
	if err != nil {
		return nil, err
	}
	err = o.Validate(nil)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Configure an HTTP client that talks to the API server on loopback.
func NewTiltClientset(o *start.TiltServerOptions) (TiltClientset, error) {
	info := &genericapiserver.DeprecatedInsecureServingInfo{}
	err := o.ServingOptions.ApplyTo(&info)
	if err != nil {
		return nil, err
	}
	loopbackConfig, err := info.NewLoopbackClientConfig()
	if err != nil {
		return nil, err
	}
	return tiltapi.NewForConfig(loopbackConfig)
}

type Controller struct {
	client        TiltClientset
	serverOptions *start.TiltServerOptions

	shutdown   func()
	shutdownCh <-chan struct{}
	names      []model.ManifestName
}

func NewController(client TiltClientset, serverOptions *start.TiltServerOptions) *Controller {
	return &Controller{
		client:        client,
		serverOptions: serverOptions,
	}
}

func (c *Controller) SetUp(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	shutdownCh, err := c.serverOptions.RunTiltServer(ctx.Done())
	if err != nil {
		panic(err)
	}
	c.shutdown = cancel
	c.shutdownCh = shutdownCh
}

func (c *Controller) TearDown(ctx context.Context) {
	c.shutdown()
	<-c.shutdownCh
}

// Grab all the manifest names from the current engine.
func (c *Controller) manifestNames(st store.RStore) []model.ManifestName {
	state := st.RLockState()
	defer st.RUnlockState()

	return append([]model.ManifestName{}, state.ManifestDefinitionOrder...)
}

// Every time the engine state changes, sync all the manifests to the apiserver.
func (c *Controller) OnChange(ctx context.Context, st store.RStore) {
	manifestNames := c.manifestNames(st)

	if cmp.Equal(manifestNames, c.names) {
		return
	}

	c.names = manifestNames
	for _, name := range manifestNames {
		_, err := c.client.CoreV1alpha1().Manifests().Create(ctx, &v1alpha1.Manifest{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(name),
			},
		}, metav1.CreateOptions{})
		if err != nil {
			logger.Get(ctx).Errorf("ERROR: %v", err)
		}
	}
}

var _ store.Subscriber = &Controller{}
var _ store.SetUpper = &Controller{}
var _ store.TearDowner = &Controller{}
